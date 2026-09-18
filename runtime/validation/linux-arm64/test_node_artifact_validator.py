import hashlib
import json
import os
import pathlib
import shutil
import stat
import sys
import tempfile
import unittest

IS_WINDOWS = sys.platform == "win32"

SCRIPT_DIR = pathlib.Path(__file__).resolve().parent

sys_path_inserted = False
if str(SCRIPT_DIR) not in os.sys.path:
    os.sys.path.insert(0, str(SCRIPT_DIR))
    sys_path_inserted = True

import node_artifact_validator as validator


def make_elf_aarch64_header():
    header = bytearray(64)
    header[0:4] = b"\x7fELF"
    header[4] = 2
    header[5] = 1
    header[16:18] = (2).to_bytes(2, "little")
    header[18:20] = (183).to_bytes(2, "little")
    return bytes(header)


def write_minimal_node_tree(root):
    node_root = root / "node"
    node_root.mkdir(parents=True, exist_ok=True)
    (node_root / "bin").mkdir()
    (node_root / "bin" / "node").write_bytes(make_elf_aarch64_header() + b"\x00" * 100)
    os.chmod(node_root / "bin" / "node", 0o755)
    if not IS_WINDOWS:
        (node_root / "bin" / "npm").symlink_to("../lib/node_modules/npm/bin/npm")
        (node_root / "bin" / "npx").symlink_to("../lib/node_modules/npm/bin/npx")
    else:
        (node_root / "bin" / "npm").write_text("#!/usr/bin/env node\n")
        (node_root / "bin" / "npx").write_text("#!/usr/bin/env node\n")
    (node_root / "lib" / "node_modules" / "npm" / "bin").mkdir(parents=True)
    (node_root / "lib" / "node_modules" / "npm" / "bin" / "npm").write_text("#!/usr/bin/env node\n")
    (node_root / "lib" / "node_modules" / "npm" / "bin" / "npx").write_text("#!/usr/bin/env node\n")
    (node_root / "lib" / "node_modules" / "npm" / "bin" / "npm-cli.js").write_text("module.exports={}")
    (node_root / "lib" / "node_modules" / "npm" / "bin" / "npx-cli.js").write_text("module.exports={}")
    (node_root / "include" / "node").mkdir(parents=True)
    (node_root / "include" / "node" / "node.h").write_text("// node header")
    (node_root / "LICENSE").write_text("MIT License")
    return node_root


def compute_file_sha(path):
    h = hashlib.sha256()
    with open(path, "rb") as f:
        h.update(f.read())
    return h.hexdigest()


def write_tree_manifest(output_dir):
    node_root = output_dir / "node"
    files = []
    for dirpath, dirnames, filenames in os.walk(node_root, followlinks=False):
        dirnames.sort()
        dp = pathlib.Path(dirpath)
        for fn in sorted(filenames):
            fp = dp / fn
            if fp.is_symlink():
                continue
            rel = fp.relative_to(output_dir).as_posix()
            files.append((rel, fp))
    lines = []
    for rel, fp in sorted(files):
        digest = compute_file_sha(fp)
        lines.append(f"{digest}  {rel}")
    manifest_path = output_dir / "node-files.sha256"
    content_bytes = ("\n".join(lines) + "\n").encode("utf-8")
    manifest_path.write_bytes(content_bytes)
    h = hashlib.sha256()
    h.update(content_bytes)
    return h.hexdigest()


def write_build_record(output_dir, lock, tree_sha):
    record = {
        "schemaVersion": 1,
        "component": "node",
        "version": lock["version"],
        "platform": "linux",
        "architecture": "arm64",
        "source": {
            "url": lock["sourceUrl"],
            "archiveFileName": lock["archiveFileName"],
            "expectedSha256": lock["sha256"],
            "actualSha256": lock["sha256"],
        },
        "runtime": {
            "nodePath": "node/bin/node",
            "npmPath": "node/bin/npm",
            "npxPath": "node/bin/npx",
        },
        "validation": {
            "staticValidation": "PASS",
            "executionValidation": "NOT_EXECUTED",
        },
        "npmVersion": "10.9.0",
        "npxVersion": "10.9.0",
        "corepackIncluded": False,
        "treeSha256": tree_sha,
        "frozenRoot": "node",
    }
    (output_dir / "node-build-record.json").write_text(json.dumps(record, indent=2) + "\n")


LOCK_DATA = {
    "schemaVersion": 1,
    "component": "node",
    "version": "24.19.0",
    "platform": "linux",
    "architecture": "arm64",
    "archiveFileName": "node-v24.19.0-linux-arm64.tar.xz",
    "sourceUrl": "https://nodejs.org/dist/v24.19.0/node-v24.19.0-linux-arm64.tar.xz",
    "sha256": "01443c1e1a29e531ccad5a46fefa6df490d2189c49f7955904aecdbb0fe86fdc",
    "installSubdir": "node",
}


class LockTests(unittest.TestCase):
    def setUp(self):
        self.tmpdir = tempfile.mkdtemp()

    def tearDown(self):
        shutil.rmtree(self.tmpdir, ignore_errors=True)

    def test_valid_lock(self):
        lock_path = pathlib.Path(self.tmpdir) / "node-runtime-lock.json"
        lock_path.write_text(json.dumps(LOCK_DATA, indent=2))
        issues = []
        result = validator.check_lock(lock_path, issues)
        self.assertIsNotNone(result)
        self.assertEqual(result["version"], "24.19.0")
        self.assertEqual(len(issues), 0)

    def test_missing_lock(self):
        lock_path = pathlib.Path(self.tmpdir) / "nonexistent.json"
        issues = []
        result = validator.check_lock(lock_path, issues)
        self.assertIsNone(result)
        self.assertTrue(any("not found" in i for i in issues))

    def test_missing_field(self):
        bad_lock = dict(LOCK_DATA)
        del bad_lock["sha256"]
        lock_path = pathlib.Path(self.tmpdir) / "bad.json"
        lock_path.write_text(json.dumps(bad_lock))
        issues = []
        validator.check_lock(lock_path, issues)
        self.assertTrue(any("sha256" in i for i in issues))

    def test_wrong_platform(self):
        bad_lock = dict(LOCK_DATA)
        bad_lock["platform"] = "windows"
        lock_path = pathlib.Path(self.tmpdir) / "bad.json"
        lock_path.write_text(json.dumps(bad_lock))
        issues = []
        validator.check_lock(lock_path, issues)
        self.assertTrue(any("platform" in i for i in issues))

    def test_wrong_architecture(self):
        bad_lock = dict(LOCK_DATA)
        bad_lock["architecture"] = "x64"
        lock_path = pathlib.Path(self.tmpdir) / "bad.json"
        lock_path.write_text(json.dumps(bad_lock))
        issues = []
        validator.check_lock(lock_path, issues)
        self.assertTrue(any("arm64" in i for i in issues))


class BuildRecordTests(unittest.TestCase):
    def setUp(self):
        self.tmpdir = tempfile.mkdtemp()

    def tearDown(self):
        shutil.rmtree(self.tmpdir, ignore_errors=True)

    def test_valid_record(self):
        output_dir = pathlib.Path(self.tmpdir) / "output"
        output_dir.mkdir()
        write_minimal_node_tree(output_dir)
        tree_sha = write_tree_manifest(output_dir)
        write_build_record(output_dir, LOCK_DATA, tree_sha)
        issues = []
        record = validator.check_build_record(output_dir, LOCK_DATA, issues)
        self.assertIsNotNone(record)
        self.assertEqual(record["version"], "24.19.0")
        self.assertEqual(len(issues), 0)

    def test_missing_record(self):
        output_dir = pathlib.Path(self.tmpdir) / "output"
        output_dir.mkdir()
        issues = []
        result = validator.check_build_record(output_dir, LOCK_DATA, issues)
        self.assertIsNone(result)
        self.assertTrue(any("not found" in i for i in issues))

    def test_version_mismatch(self):
        output_dir = pathlib.Path(self.tmpdir) / "output"
        output_dir.mkdir()
        write_minimal_node_tree(output_dir)
        tree_sha = write_tree_manifest(output_dir)
        bad_lock = dict(LOCK_DATA)
        bad_lock["version"] = "22.0.0"
        write_build_record(output_dir, bad_lock, tree_sha)
        issues = []
        validator.check_build_record(output_dir, LOCK_DATA, issues)
        self.assertTrue(any("version mismatch" in i for i in issues))


class NodeExistsTests(unittest.TestCase):
    def setUp(self):
        self.tmpdir = tempfile.mkdtemp()

    def tearDown(self):
        shutil.rmtree(self.tmpdir, ignore_errors=True)

    def test_node_missing(self):
        output_dir = pathlib.Path(self.tmpdir) / "output"
        output_dir.mkdir()
        issues = []
        result = validator.check_node_exists(output_dir, None, issues)
        self.assertFalse(result)
        self.assertTrue(any("not found" in i for i in issues))

    def test_node_exists(self):
        output_dir = pathlib.Path(self.tmpdir) / "output"
        output_dir.mkdir()
        write_minimal_node_tree(output_dir)
        issues = []
        result = validator.check_node_exists(output_dir, None, issues)
        self.assertTrue(result)
        self.assertEqual(len(issues), 0)


class ElfTests(unittest.TestCase):
    def setUp(self):
        self.tmpdir = tempfile.mkdtemp()

    def tearDown(self):
        shutil.rmtree(self.tmpdir, ignore_errors=True)

    def test_valid_elf_aarch64(self):
        output_dir = pathlib.Path(self.tmpdir) / "output"
        output_dir.mkdir()
        write_minimal_node_tree(output_dir)
        issues = []
        validator.check_elf_binary(output_dir, issues)
        self.assertEqual(len(issues), 0)

    def test_wrong_architecture(self):
        output_dir = pathlib.Path(self.tmpdir) / "output"
        output_dir.mkdir()
        node_root = write_minimal_node_tree(output_dir)
        bad_header = bytearray(64)
        bad_header[0:4] = b"\x7fELF"
        bad_header[4] = 2
        bad_header[5] = 1
        bad_header[18:20] = (62).to_bytes(2, "little")
        (output_dir / "node" / "bin" / "node").write_bytes(bytes(bad_header))
        issues = []
        validator.check_elf_binary(output_dir, issues)
        self.assertTrue(any("AArch64" in i for i in issues))

    def test_not_elf(self):
        output_dir = pathlib.Path(self.tmpdir) / "output"
        output_dir.mkdir()
        node_root = write_minimal_node_tree(output_dir)
        (output_dir / "node" / "bin" / "node").write_bytes(b"NOTELFBINARY" + b"\x00" * 60)
        issues = []
        validator.check_elf_binary(output_dir, issues)
        self.assertTrue(any("not ELF" in i for i in issues))


class SymlinkTests(unittest.TestCase):
    def setUp(self):
        self.tmpdir = tempfile.mkdtemp()

    def tearDown(self):
        shutil.rmtree(self.tmpdir, ignore_errors=True)

    def test_safe_relative_symlinks(self):
        if IS_WINDOWS:
            self.skipTest("Unix symlinks not reliably testable on Windows")
        output_dir = pathlib.Path(self.tmpdir) / "output"
        output_dir.mkdir()
        write_minimal_node_tree(output_dir)
        issues = []
        validator.check_symlink_safety(output_dir, issues)
        self.assertEqual(len(issues), 0)

    def test_absolute_symlink_rejected(self):
        if IS_WINDOWS:
            self.skipTest("Unix symlinks not reliably testable on Windows")
        output_dir = pathlib.Path(self.tmpdir) / "output"
        output_dir.mkdir()
        write_minimal_node_tree(output_dir)
        (output_dir / "node" / "bin" / "npm").unlink()
        (output_dir / "node" / "bin" / "npm").symlink_to("/usr/bin/npm")
        issues = []
        validator.check_symlink_safety(output_dir, issues)
        self.assertTrue(any("Absolute symlink" in i for i in issues))


class TreeManifestTests(unittest.TestCase):
    def setUp(self):
        self.tmpdir = tempfile.mkdtemp()

    def tearDown(self):
        shutil.rmtree(self.tmpdir, ignore_errors=True)

    def test_valid_manifest(self):
        output_dir = pathlib.Path(self.tmpdir) / "output"
        output_dir.mkdir()
        write_minimal_node_tree(output_dir)
        tree_sha = write_tree_manifest(output_dir)
        record = {"treeSha256": tree_sha}
        issues = []
        validator.check_tree_manifest(output_dir, record, issues)
        self.assertEqual(len(issues), 0)

    def test_missing_manifest(self):
        output_dir = pathlib.Path(self.tmpdir) / "output"
        output_dir.mkdir()
        write_minimal_node_tree(output_dir)
        issues = []
        validator.check_tree_manifest(output_dir, None, issues)
        self.assertTrue(any("not found" in i for i in issues))

    def test_unsorted_manifest(self):
        output_dir = pathlib.Path(self.tmpdir) / "output"
        output_dir.mkdir()
        write_minimal_node_tree(output_dir)
        sha1 = "b" * 64
        sha2 = "a" * 64
        manifest_path = output_dir / "node-files.sha256"
        manifest_path.write_text(f"{sha1}  node/lib/file.txt\n{sha2}  node/bin/node\n")
        issues = []
        validator.check_tree_manifest(output_dir, None, issues)
        self.assertTrue(any("not sorted" in i for i in issues))


class FullValidationTests(unittest.TestCase):
    def setUp(self):
        self.tmpdir = tempfile.mkdtemp()

    def tearDown(self):
        shutil.rmtree(self.tmpdir, ignore_errors=True)

    def test_happy_path(self):
        output_dir = pathlib.Path(self.tmpdir) / "output"
        output_dir.mkdir()
        lock_path = pathlib.Path(self.tmpdir) / "node-runtime-lock.json"
        lock_path.write_text(json.dumps(LOCK_DATA, indent=2))
        write_minimal_node_tree(output_dir)
        tree_sha = write_tree_manifest(output_dir)
        write_build_record(output_dir, LOCK_DATA, tree_sha)
        issues = validator.run_validation(output_dir, lock_path)
        self.assertEqual(len(issues), 0)

    def test_missing_everything(self):
        output_dir = pathlib.Path(self.tmpdir) / "output"
        output_dir.mkdir()
        lock_path = pathlib.Path(self.tmpdir) / "nonexistent.json"
        issues = validator.run_validation(output_dir, lock_path)
        self.assertTrue(len(issues) > 0)


if __name__ == "__main__":
    unittest.main()
