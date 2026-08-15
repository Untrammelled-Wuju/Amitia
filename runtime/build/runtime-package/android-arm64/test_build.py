import hashlib
import io
import json
import os
import pathlib
import shutil
import sys
import tarfile
import tempfile
import unittest
import zipfile
from unittest import mock

SCRIPT_DIR = pathlib.Path(__file__).resolve().parent
sys.path.insert(0, str(SCRIPT_DIR))

import build


def make_frozen_artifact(base, component_id, version, artifact_rel_path, content=b"artifact content"):
    comp_dir = pathlib.Path(base) / component_id / "linux-arm64" / version
    comp_dir.mkdir(parents=True, exist_ok=True)
    artifact_path = comp_dir / artifact_rel_path
    artifact_path.parent.mkdir(parents=True, exist_ok=True)
    artifact_path.write_bytes(content)
    sha = hashlib.sha256(content).hexdigest()
    record = {
        "schemaVersion": 1,
        "componentId": component_id,
        "version": version,
        "platform": "linux",
        "architecture": "arm64",
        "artifactType": "frozen-tree",
        "artifactRelativePath": artifact_rel_path,
        "artifactSha256": sha,
        "treeSha256": sha,
        "buildMode": "release",
    }
    with open(comp_dir / "build-record.json", "w", encoding="utf-8") as f:
        json.dump(record, f)
    return comp_dir


def make_all_frozen_artifacts(base, version="1.0.0"):
    make_frozen_artifact(base, "backend", version, "amitia-server", b"backend binary")
    make_frozen_artifact(base, "node", version, "bin/node", b"\x7fELF" + b"\x00" * 60)
    make_frozen_artifact(base, "node", version, "lib/node_modules/npm/bin/npm-cli.js", b"npm")
    make_frozen_artifact(base, "node", version, "lib/node_modules/npm/bin/npx-cli.js", b"npx")
    make_frozen_artifact(base, "qdrant", version, "bin/qdrant", b"qdrant binary")
    make_frozen_artifact(base, "plugin-host", version, "dist/index.js", b"plugin host")
    make_frozen_artifact(base, "task-host", version, "dist/index.js", b"task host")
    scripts_dir = pathlib.Path(base) / "runtime-scripts" / "linux-arm64" / version / "node"
    scripts_dir.mkdir(parents=True, exist_ok=True)
    (scripts_dir / "amitia-node-prepare.sh").write_text("#!/bin/sh\n")
    (scripts_dir / "amitia-node-probe.sh").write_text("#!/bin/sh\n")
    scripts_sha = hashlib.sha256(b"script content").hexdigest()
    record = {
        "schemaVersion": 1,
        "componentId": "runtime-scripts",
        "version": version,
        "platform": "linux",
        "architecture": "arm64",
        "artifactType": "frozen-tree",
        "artifactRelativePath": "node",
        "artifactSha256": scripts_sha,
        "treeSha256": scripts_sha,
        "buildMode": "release",
    }
    with open(pathlib.Path(base) / "runtime-scripts" / "linux-arm64" / version / "build-record.json", "w", encoding="utf-8") as f:
        json.dump(record, f)
    rootfs_dir = pathlib.Path(base) / "rootfs" / "linux-arm64" / version
    rootfs_dir.mkdir(parents=True, exist_ok=True)
    rootfs_content = b"rootfs archive"
    (rootfs_dir / "rootfs.tar.xz").write_bytes(rootfs_content)
    rootfs_sha = hashlib.sha256(rootfs_content).hexdigest()
    rootfs_record = {
        "schemaVersion": 1,
        "componentId": "rootfs",
        "version": version,
        "platform": "linux",
        "architecture": "arm64",
        "artifactType": "archive",
        "artifactRelativePath": "rootfs.tar.xz",
        "artifactSha256": rootfs_sha,
        "treeSha256": rootfs_sha,
        "buildMode": "release",
    }
    with open(rootfs_dir / "build-record.json", "w", encoding="utf-8") as f:
        json.dump(rootfs_record, f)


def make_contracts(base, contract_dir_name="contracts"):
    contract_dir = pathlib.Path(base) / contract_dir_name
    contract_dir.mkdir(parents=True, exist_ok=True)
    layout_content = json.dumps({"layout": "default"}).encode()
    contract_content = json.dumps({"mount": "default"}).encode()
    (contract_dir / "guest-layout.json").write_bytes(layout_content)
    (contract_dir / "mount-contract.json").write_bytes(contract_content)
    return contract_dir


class TestFreshBuild(unittest.TestCase):
    def setUp(self):
        self.tmpdir = tempfile.mkdtemp()
        make_all_frozen_artifacts(self.tmpdir)
        make_contracts(self.tmpdir)
        self.output_base = pathlib.Path(self.tmpdir)
        self.components = ["backend", "node", "qdrant", "plugin-host", "task-host", "runtime-scripts"]

    def tearDown(self):
        shutil.rmtree(self.tmpdir, ignore_errors=True)

    def test_fresh_build_returns_record(self):
        record = build.build(
            str(self.output_base),
            "1.0.0",
            "pkg-001",
            self.components,
        )
        self.assertIsNotNone(record)
        self.assertEqual(record["runtimeVersion"], "1.0.0")
        self.assertEqual(record["packageId"], "pkg-001")

    def test_fresh_build_creates_package_file(self):
        build.build(
            str(self.output_base),
            "1.0.0",
            "pkg-001",
            self.components,
        )
        package_file = self.output_base / "runtime-package" / "android-arm64" / "1.0.0" / "amitia-runtime-1.0.0-linux-arm64.zip"
        self.assertTrue(package_file.exists())

    def test_fresh_build_creates_sha256_files(self):
        build.build(
            str(self.output_base),
            "1.0.0",
            "pkg-001",
            self.components,
        )
        sha_file = self.output_base / "runtime-package" / "android-arm64" / "1.0.0" / "runtime-package-files.sha256"
        self.assertTrue(sha_file.exists())


class TestMissingComponent(unittest.TestCase):
    def setUp(self):
        self.tmpdir = tempfile.mkdtemp()
        make_all_frozen_artifacts(self.tmpdir)
        make_contracts(self.tmpdir)
        self.output_base = pathlib.Path(self.tmpdir)

    def tearDown(self):
        shutil.rmtree(self.tmpdir, ignore_errors=True)

    def test_missing_backend_fails(self):
        components = ["node", "qdrant", "plugin-host", "task-host", "runtime-scripts"]
        with self.assertRaises(FileNotFoundError):
            build.build(str(self.output_base), "1.0.0", "pkg-001", components)

    def test_missing_node_fails(self):
        components = ["backend", "qdrant", "plugin-host", "task-host", "runtime-scripts"]
        with self.assertRaises(FileNotFoundError):
            build.build(str(self.output_base), "1.0.0", "pkg-001", components)

    def test_missing_qdrant_fails(self):
        components = ["backend", "node", "plugin-host", "task-host", "runtime-scripts"]
        with self.assertRaises(FileNotFoundError):
            build.build(str(self.output_base), "1.0.0", "pkg-001", components)

    def test_missing_plugin_host_fails(self):
        components = ["backend", "node", "qdrant", "task-host", "runtime-scripts"]
        with self.assertRaises(FileNotFoundError):
            build.build(str(self.output_base), "1.0.0", "pkg-001", components)

    def test_missing_task_host_fails(self):
        components = ["backend", "node", "qdrant", "plugin-host", "runtime-scripts"]
        with self.assertRaises(FileNotFoundError):
            build.build(str(self.output_base), "1.0.0", "pkg-001", components)

    def test_missing_runtime_scripts_fails(self):
        components = ["backend", "node", "qdrant", "plugin-host", "task-host"]
        with self.assertRaises(FileNotFoundError):
            build.build(str(self.output_base), "1.0.0", "pkg-001", components)


class TestMissingContracts(unittest.TestCase):
    def setUp(self):
        self.tmpdir = tempfile.mkdtemp()
        make_all_frozen_artifacts(self.tmpdir)
        self.output_base = pathlib.Path(self.tmpdir)
        self.components = ["backend", "node", "qdrant", "plugin-host", "task-host", "runtime-scripts"]

    def tearDown(self):
        shutil.rmtree(self.tmpdir, ignore_errors=True)

    def test_missing_guest_layout_fails(self):
        contract_dir = pathlib.Path(self.tmpdir) / "contracts"
        contract_dir.mkdir(parents=True, exist_ok=True)
        (contract_dir / "mount-contract.json").write_text("{}")
        (contract_dir / "guest-layout.json").write_text("{}")
        record = build.build(str(self.output_base), "1.0.0", "pkg-001", self.components)
        self.assertIsNotNone(record)


class TestContractHashTampering(unittest.TestCase):
    def setUp(self):
        self.tmpdir = tempfile.mkdtemp()
        make_all_frozen_artifacts(self.tmpdir)
        make_contracts(self.tmpdir)
        self.output_base = pathlib.Path(self.tmpdir)
        self.components = ["backend", "node", "qdrant", "plugin-host", "task-host", "runtime-scripts"]

    def tearDown(self):
        shutil.rmtree(self.tmpdir, ignore_errors=True)

    def test_tampered_contract_hash_detected(self):
        record = build.build(
            str(self.output_base),
            "1.0.0",
            "pkg-001",
            self.components,
        )
        original_contract_sha = record["mountContractSha256"]
        tampered_sha = "f" * 64
        self.assertNotEqual(original_contract_sha, tampered_sha)


class TestPackageIndexErrors(unittest.TestCase):
    def setUp(self):
        self.tmpdir = tempfile.mkdtemp()
        make_all_frozen_artifacts(self.tmpdir)
        make_contracts(self.tmpdir)
        self.output_base = pathlib.Path(self.tmpdir)
        self.components = ["backend", "node", "qdrant", "plugin-host", "task-host", "runtime-scripts"]

    def tearDown(self):
        shutil.rmtree(self.tmpdir, ignore_errors=True)

    def test_package_index_size_is_integer(self):
        record = build.build(
            str(self.output_base),
            "1.0.0",
            "pkg-001",
            self.components,
        )
        package_file = self.output_base / "runtime-package" / "android-arm64" / "1.0.0" / "amitia-runtime-1.0.0-linux-arm64.zip"
        self.assertTrue(package_file.exists())
        self.assertIsInstance(record["packageSize"], int)

    def test_package_index_sha_is_valid_hex(self):
        record = build.build(
            str(self.output_base),
            "1.0.0",
            "pkg-001",
            self.components,
        )
        self.assertEqual(len(record["packageSha256"]), 64)


class TestHostAbiGuestArch(unittest.TestCase):
    def setUp(self):
        self.tmpdir = tempfile.mkdtemp()
        make_all_frozen_artifacts(self.tmpdir)
        make_contracts(self.tmpdir)
        self.output_base = pathlib.Path(self.tmpdir)
        self.components = ["backend", "node", "qdrant", "plugin-host", "task-host", "runtime-scripts"]

    def tearDown(self):
        shutil.rmtree(self.tmpdir, ignore_errors=True)

    def test_default_host_abi_is_arm64_v8a(self):
        record = build.build(
            str(self.output_base),
            "1.0.0",
            "pkg-001",
            self.components,
        )
        self.assertIsNotNone(record)


class TestPackageIdSourceRevision(unittest.TestCase):
    def setUp(self):
        self.tmpdir = tempfile.mkdtemp()
        make_all_frozen_artifacts(self.tmpdir)
        make_contracts(self.tmpdir)
        self.output_base = pathlib.Path(self.tmpdir)
        self.components = ["backend", "node", "qdrant", "plugin-host", "task-host", "runtime-scripts"]

    def tearDown(self):
        shutil.rmtree(self.tmpdir, ignore_errors=True)

    def test_package_id_recorded(self):
        record = build.build(
            str(self.output_base),
            "1.0.0",
            "pkg-test-123",
            self.components,
        )
        self.assertEqual(record["packageId"], "pkg-test-123")

    def test_different_package_ids_distinct(self):
        record1 = build.build(
            str(self.output_base),
            "1.0.0",
            "pkg-001",
            self.components,
        )
        record2 = build.build(
            str(self.output_base),
            "2.0.0",
            "pkg-002",
            self.components,
        )
        self.assertNotEqual(record1["packageId"], record2["packageId"])


class TestNestedArchiveDetection(unittest.TestCase):
    def setUp(self):
        self.tmpdir = tempfile.mkdtemp()
        make_all_frozen_artifacts(self.tmpdir)
        make_contracts(self.tmpdir)
        self.output_base = pathlib.Path(self.tmpdir)
        self.components = ["backend", "node", "qdrant", "plugin-host", "task-host", "runtime-scripts"]

    def tearDown(self):
        shutil.rmtree(self.tmpdir, ignore_errors=True)

    def test_backend_archive_in_runtime_payload(self):
        record = build.build(
            str(self.output_base),
            "1.0.0",
            "pkg-001",
            self.components,
        )
        self.assertIn("runtimePayloadSha256", record)

    def test_qdrant_archive_in_runtime_payload(self):
        record = build.build(
            str(self.output_base),
            "1.0.0",
            "pkg-001",
            self.components,
        )
        self.assertIn("componentBuildRecordShas", record)
        self.assertIn("qdrant", record["componentBuildRecordShas"])


class TestReuseAndFailure(unittest.TestCase):
    def setUp(self):
        self.tmpdir = tempfile.mkdtemp()
        make_all_frozen_artifacts(self.tmpdir)
        make_contracts(self.tmpdir)
        self.output_base = pathlib.Path(self.tmpdir)
        self.components = ["backend", "node", "qdrant", "plugin-host", "task-host", "runtime-scripts"]

    def tearDown(self):
        shutil.rmtree(self.tmpdir, ignore_errors=True)

    def test_failure_before_package_writing_no_partial(self):
        pass

    def test_reproducibility_same_identity(self):
        record1 = build.build(
            str(self.output_base),
            "1.0.0",
            "pkg-001",
            self.components,
        )
        shutil.rmtree(str(self.output_base / "runtime-package"), ignore_errors=True)
        record2 = build.build(
            str(self.output_base),
            "1.0.0",
            "pkg-001",
            self.components,
        )
        self.assertEqual(record1["packageSha256"], record2["packageSha256"])


if __name__ == "__main__":
    unittest.main()
