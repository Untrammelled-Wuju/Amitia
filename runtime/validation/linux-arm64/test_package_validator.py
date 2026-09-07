import hashlib
import io
import json
import os
import tarfile
import tempfile
import unittest
import zipfile
import sys

sys.path.insert(0, os.path.dirname(os.path.abspath(__file__)))


def _build_index(version, package_id, rootfs_sha, rootfs_size, runtime_sha, runtime_size,
                 guest_layout_sha, guest_layout_size, mount_contract_sha, mount_contract_size,
                 sha256sums_sha, sha256sums_size, source_revision="abc1234567890def"):
    return {
        "schemaVersion": 1,
        "packageFormatVersion": 1,
        "runtimeVersion": version,
        "packageId": package_id,
        "sourceRevision": source_revision,
        "target": {
            "hostPlatform": "android",
            "hostAbi": "arm64-v8a",
            "runtimeKind": "embedded-proot",
            "guestPlatform": "linux",
            "guestArchitecture": "arm64",
        },
        "payloads": [
            {
                "role": "rootfs",
                "path": "payload/rootfs/rootfs.tar.xz",
                "sha256": rootfs_sha,
                "size": rootfs_size,
            },
            {
                "role": "runtime",
                "path": "payload/runtime/runtime.tar.xz",
                "sha256": runtime_sha,
                "size": runtime_size,
            },
        ],
        "metadata": [
            {
                "role": "guest-layout",
                "path": "metadata/guest-layout.json",
                "sha256": guest_layout_sha,
                "size": guest_layout_size,
            },
            {
                "role": "mount-contract",
                "path": "metadata/mount-contract.json",
                "sha256": mount_contract_sha,
                "size": mount_contract_size,
            },
            {
                "role": "sha256sums",
                "path": "metadata/SHA256SUMS",
                "sha256": sha256sums_sha,
                "size": sha256sums_size,
            },
        ],
    }


def _build_guest_layout():
    return {
        "root": "/opt/amitia",
        "directories": [
            "/opt/amitia/backend",
            "/opt/amitia/node",
            "/opt/amitia/qdrant",
            "/etc/amitia",
            "/var/lib/amitia",
            "/var/cache/amitia",
            "/var/log/amitia",
            "/run/amitia",
            "/home/amitia",
            "/tmp",
        ],
    }


def _build_mount_contract():
    return {
        "binds": [
            {"source": "/data/local/tmp/program", "target": "/opt/amitia", "readOnly": True},
            {"source": "/data/local/tmp/config", "target": "/etc/amitia", "readOnly": False},
            {"source": "/data/local/tmp/data", "target": "/var/lib/amitia", "readOnly": False},
            {"source": "/data/local/tmp/cache", "target": "/var/cache/amitia", "readOnly": False},
            {"source": "/data/local/tmp/logs", "target": "/var/log/amitia", "readOnly": False},
            {"source": "/data/local/tmp/run", "target": "/run/amitia", "readOnly": False},
            {"source": "/data/local/tmp/home", "target": "/home/amitia", "readOnly": False},
        ],
    }


def _build_component_lock(version, package_id):
    return {
        "runtimeVersion": version,
        "packageId": package_id,
        "components": [
            {"id": "backend", "version": "1.0.0", "architecture": "arm64", "path": "payload/runtime/runtime.tar.xz", "sha256": "a" * 64},
            {"id": "node", "version": "24.19.0", "architecture": "arm64", "path": "payload/runtime/runtime.tar.xz", "sha256": "b" * 64},
            {"id": "qdrant", "version": "1.9.0", "architecture": "arm64", "path": "payload/runtime/runtime.tar.xz", "sha256": "c" * 64},
        ],
    }


def _build_sums(rootfs_sha, runtime_sha, guest_layout_sha, mount_contract_sha):
    lines = [
        f"{rootfs_sha}  payload/rootfs/rootfs.tar.xz",
        f"{runtime_sha}  payload/runtime/runtime.tar.xz",
        f"{guest_layout_sha}  metadata/guest-layout.json",
        f"{mount_contract_sha}  metadata/mount-contract.json",
    ]
    return "\n".join(lines) + "\n"


class TestPackageValidator(unittest.TestCase):
    def setUp(self):
        self.tmpdir = tempfile.mkdtemp()

    def tearDown(self):
        import shutil
        shutil.rmtree(self.tmpdir, ignore_errors=True)

    def _make_simple_xz(self, data: bytes) -> bytes:
        import lzma
        import io
        import tarfile
        tar_buffer = io.BytesIO()
        with tarfile.open(fileobj=tar_buffer, mode='w') as tf:
            tar_info = tarfile.TarInfo(name="content")
            tar_info.size = len(data)
            tf.addfile(tar_info, io.BytesIO(data))
        return lzma.compress(tar_buffer.getvalue())

    def _make_runtime_xz(self) -> bytes:
        import lzma
        import io
        import tarfile
        tar_buffer = io.BytesIO()
        with tarfile.open(fileobj=tar_buffer, mode='w') as tf:
            required_files = [
                ("backend/amitia-server", b"#!/bin/sh\n# backend", 0o755),
                ("node/bin/node", b"#!/bin/sh\n# node", 0o755),
                ("node/lib/node_modules/npm/bin/npm-cli.js", b"#!/usr/bin/env node\n// npm", 0o644),
                ("node/lib/node_modules/npm/bin/npx-cli.js", b"#!/usr/bin/env node\n// npx", 0o644),
                ("qdrant/bin/qdrant", b"#!/bin/sh\n# qdrant", 0o755),
                ("plugin-host/dist/index.js", b"// plugin-host", 0o644),
                ("task-host/dist/index.js", b"// task-host", 0o644),
                ("scripts/node/amitia-node-prepare.sh", b"#!/bin/sh\n# prepare", 0o755),
                ("scripts/node/amitia-node-probe.sh", b"#!/bin/sh\n# probe", 0o755),
                ("manifest/guest-layout.json", b'{"root":"/opt/amitia"}', 0o644),
                ("manifest/mount-contract.json", b'{"binds":[]}', 0o644),
            ]
            for path, content, mode in required_files:
                tar_info = tarfile.TarInfo(name=path)
                tar_info.size = len(content)
                tar_info.mode = mode
                tf.addfile(tar_info, io.BytesIO(content))
        return lzma.compress(tar_buffer.getvalue())

    def _build_test_package(self, dest_path: str, version="0.1.0", package_id="abc1234"):
        rootfs_payload = self._make_simple_xz(b"rootfs-content")
        runtime_payload = self._make_runtime_xz()

        rootfs_sha = hashlib.sha256(rootfs_payload).hexdigest()
        runtime_sha = hashlib.sha256(runtime_payload).hexdigest()

        guest_layout = _build_guest_layout()
        guest_layout_raw = json.dumps(guest_layout, separators=(",", ":"), ensure_ascii=False).encode("utf-8")
        guest_layout_sha = hashlib.sha256(guest_layout_raw).hexdigest()

        mount_contract = _build_mount_contract()
        mount_contract_raw = json.dumps(mount_contract, separators=(",", ":"), ensure_ascii=False).encode("utf-8")
        mount_contract_sha = hashlib.sha256(mount_contract_raw).hexdigest()

        sums_raw = _build_sums(rootfs_sha, runtime_sha, guest_layout_sha, mount_contract_sha).encode("utf-8")
        sums_sha = hashlib.sha256(sums_raw).hexdigest()

        index = _build_index(
            version=version,
            package_id=package_id,
            rootfs_sha=rootfs_sha,
            rootfs_size=len(rootfs_payload),
            runtime_sha=runtime_sha,
            runtime_size=len(runtime_payload),
            guest_layout_sha=guest_layout_sha,
            guest_layout_size=len(guest_layout_raw),
            mount_contract_sha=mount_contract_sha,
            mount_contract_size=len(mount_contract_raw),
            sha256sums_sha=sums_sha,
            sha256sums_size=len(sums_raw),
        )

        comp_lock = _build_component_lock(version, package_id)

        with zipfile.ZipFile(dest_path, "w", zipfile.ZIP_DEFLATED) as zf:
            zf.writestr("metadata/package-index.json", json.dumps(index, indent=2))
            zf.writestr("metadata/component-lock.json", json.dumps(comp_lock, indent=2))
            zf.writestr("metadata/guest-layout.json", guest_layout_raw.decode("utf-8"))
            zf.writestr("metadata/mount-contract.json", mount_contract_raw.decode("utf-8"))
            zf.writestr("metadata/SHA256SUMS", sums_raw.decode("utf-8"))
            zf.writestr("payload/rootfs/rootfs.tar.xz", rootfs_payload)
            zf.writestr("payload/runtime/runtime.tar.xz", runtime_payload)

    def _build_from_src(self, src_dir: str, dest_path: str):
        with zipfile.ZipFile(dest_path, "w", zipfile.ZIP_DEFLATED) as zf:
            for dirpath, dirnames, filenames in os.walk(src_dir):
                for name in filenames:
                    full = os.path.join(dirpath, name)
                    rel = os.path.relpath(full, src_dir)
                    zf.write(full, rel)

    def test_valid_package(self):
        from package_validator import validate_package
        pkg_path = os.path.join(self.tmpdir, "test.zip")
        self._build_test_package(pkg_path, "0.1.0", "abc1234")
        result = validate_package(pkg_path, "0.1.0", "abc1234")
        self.assertTrue(result.valid, msg=f"Errors: {result.errors}")
        self.assertEqual(len(result.errors), 0)
        self.assertIsNotNone(result.manifest)
        self.assertEqual(result.manifest.runtimeVersion, "0.1.0")
        self.assertEqual(result.manifest.packageId, "abc1234")
        self.assertEqual(result.manifest.hostPlatform, "android")
        self.assertEqual(result.manifest.hostAbi, "arm64-v8a")
        self.assertEqual(result.manifest.guestPlatform, "linux")
        self.assertEqual(result.manifest.guestArchitecture, "arm64")

    def test_version_mismatch(self):
        from package_validator import validate_package
        pkg_path = os.path.join(self.tmpdir, "test.zip")
        self._build_test_package(pkg_path, "0.1.0", "abc1234")
        result = validate_package(pkg_path, "0.2.0", "abc1234")
        self.assertFalse(result.valid)
        self.assertTrue(any("VERSION" in err.get("code", "") for err in result.errors))

    def test_commit_mismatch(self):
        from package_validator import validate_package
        pkg_path = os.path.join(self.tmpdir, "test.zip")
        self._build_test_package(pkg_path, "0.1.0", "abc1234")
        result = validate_package(pkg_path, "0.1.0", "badpackageid")
        self.assertFalse(result.valid)
        self.assertTrue(any("PACKAGE_ID_MISMATCH" in err.get("code", "") for err in result.errors))

    def test_missing_package(self):
        from package_validator import validate_package
        result = validate_package("/nonexistent/path.zip", "0.1.0", "abc1234")
        self.assertFalse(result.valid)

    def test_sha_mismatch(self):
        from package_validator import validate_package, sha256_file
        pkg_path = os.path.join(self.tmpdir, "test.zip")
        self._build_test_package(pkg_path, "0.1.0", "abc1234")
        actual_sha = sha256_file(pkg_path)
        result = validate_package(pkg_path, "0.1.0", "abc1234",
                                  expected_sha256="wrongsha")
        self.assertFalse(result.valid)
        self.assertTrue(any("SHA_MISMATCH" in err.get("code", "") or "PACKAGE_SHA" in err.get("code", "") for err in result.errors))

    def test_host_target_invalid(self):
        from package_validator import validate_package
        with tempfile.TemporaryDirectory() as src:
            os.makedirs(os.path.join(src, "payload", "rootfs"), exist_ok=True)
            os.makedirs(os.path.join(src, "payload", "runtime"), exist_ok=True)
            os.makedirs(os.path.join(src, "metadata"), exist_ok=True)

            rootfs_data = b"rootfs"
            with open(os.path.join(src, "payload", "rootfs", "rootfs.tar.xz"), "wb") as f:
                f.write(rootfs_data)
            runtime_data = b"runtime"
            with open(os.path.join(src, "payload", "runtime", "runtime.tar.xz"), "wb") as f:
                f.write(runtime_data)

            gl_data = json.dumps({"root": "/opt/amitia", "directories": ["/opt/amitia/backend"]})
            gl_sha = hashlib.sha256(gl_data.encode("utf-8")).hexdigest()
            with open(os.path.join(src, "metadata", "guest-layout.json"), "w", encoding="utf-8") as f:
                f.write(gl_data)
            mc_data = json.dumps({"binds": [{"source": "/host", "target": "/opt/amitia", "readOnly": False}]})
            mc_sha = hashlib.sha256(mc_data.encode("utf-8")).hexdigest()
            with open(os.path.join(src, "metadata", "mount-contract.json"), "w", encoding="utf-8") as f:
                f.write(mc_data)

            rootfs_sha = hashlib.sha256(rootfs_data).hexdigest()
            runtime_sha = hashlib.sha256(runtime_data).hexdigest()
            sums_raw = f"{rootfs_sha}  payload/rootfs/rootfs.tar.xz\n{runtime_sha}  payload/runtime/runtime.tar.xz\n{gl_sha}  metadata/guest-layout.json\n{mc_sha}  metadata/mount-contract.json\n"
            sums_sha = hashlib.sha256(sums_raw.encode("utf-8")).hexdigest()
            with open(os.path.join(src, "metadata", "SHA256SUMS"), "w", encoding="utf-8") as f:
                f.write(sums_raw)

            lock_data = {"runtimeVersion": "0.1.0", "packageId": "abc", "components": [{"id": "x", "sha256": "b" * 64, "version": "1.0", "architecture": "arm64", "path": "p"}]}
            with open(os.path.join(src, "metadata", "component-lock.json"), "w", encoding="utf-8") as f:
                json.dump(lock_data, f)

            index = {
                "schemaVersion": 1,
                "packageFormatVersion": 1,
                "runtimeVersion": "0.1.0",
                "packageId": "abc",
                "sourceRevision": "abc1234567890def",
                "target": {
                    "hostPlatform": "windows",
                    "hostAbi": "arm64-v8a",
                    "runtimeKind": "embedded-proot",
                    "guestPlatform": "linux",
                    "guestArchitecture": "arm64",
                },
                "payloads": [
                    {"role": "rootfs", "path": "payload/rootfs/rootfs.tar.xz", "sha256": rootfs_sha, "size": len(rootfs_data)},
                    {"role": "runtime", "path": "payload/runtime/runtime.tar.xz", "sha256": runtime_sha, "size": len(runtime_data)},
                ],
                "metadata": [
                    {"role": "guest-layout", "path": "metadata/guest-layout.json", "sha256": gl_sha, "size": len(gl_data.encode("utf-8"))},
                    {"role": "mount-contract", "path": "metadata/mount-contract.json", "sha256": mc_sha, "size": len(mc_data.encode("utf-8"))},
                    {"role": "sha256sums", "path": "metadata/SHA256SUMS", "sha256": sums_sha, "size": len(sums_raw.encode("utf-8"))},
                ],
            }
            with open(os.path.join(src, "metadata", "package-index.json"), "w", encoding="utf-8") as f:
                json.dump(index, f)

            pkg_path = os.path.join(self.tmpdir, "bad-host.zip")
            self._build_from_src(src, pkg_path)
            result = validate_package(pkg_path, "0.1.0", "abc")
            self.assertFalse(result.valid)

    def test_guest_arch_invalid(self):
        from package_validator import validate_package
        with tempfile.TemporaryDirectory() as src:
            os.makedirs(os.path.join(src, "payload", "rootfs"), exist_ok=True)
            os.makedirs(os.path.join(src, "payload", "runtime"), exist_ok=True)
            os.makedirs(os.path.join(src, "metadata"), exist_ok=True)

            rootfs_data = b"rootfs"
            with open(os.path.join(src, "payload", "rootfs", "rootfs.tar.xz"), "wb") as f:
                f.write(rootfs_data)
            runtime_data = b"runtime"
            with open(os.path.join(src, "payload", "runtime", "runtime.tar.xz"), "wb") as f:
                f.write(runtime_data)

            gl_data = json.dumps({"root": "/opt/amitia", "directories": ["/opt/amitia/backend"]})
            gl_sha = hashlib.sha256(gl_data.encode("utf-8")).hexdigest()
            with open(os.path.join(src, "metadata", "guest-layout.json"), "w", encoding="utf-8") as f:
                f.write(gl_data)
            mc_data = json.dumps({"binds": [{"source": "/host", "target": "/opt/amitia", "readOnly": False}]})
            mc_sha = hashlib.sha256(mc_data.encode("utf-8")).hexdigest()
            with open(os.path.join(src, "metadata", "mount-contract.json"), "w", encoding="utf-8") as f:
                f.write(mc_data)

            rootfs_sha = hashlib.sha256(rootfs_data).hexdigest()
            runtime_sha = hashlib.sha256(runtime_data).hexdigest()
            sums_raw = f"{rootfs_sha}  payload/rootfs/rootfs.tar.xz\n{runtime_sha}  payload/runtime/runtime.tar.xz\n{gl_sha}  metadata/guest-layout.json\n{mc_sha}  metadata/mount-contract.json\n"
            sums_sha = hashlib.sha256(sums_raw.encode("utf-8")).hexdigest()
            with open(os.path.join(src, "metadata", "SHA256SUMS"), "w", encoding="utf-8") as f:
                f.write(sums_raw)

            lock_data = {"runtimeVersion": "0.1.0", "packageId": "abc", "components": [{"id": "x", "sha256": "b" * 64, "version": "1.0", "architecture": "arm64", "path": "p"}]}
            with open(os.path.join(src, "metadata", "component-lock.json"), "w", encoding="utf-8") as f:
                json.dump(lock_data, f)

            index = {
                "schemaVersion": 1,
                "packageFormatVersion": 1,
                "runtimeVersion": "0.1.0",
                "packageId": "abc",
                "sourceRevision": "abc1234567890def",
                "target": {
                    "hostPlatform": "android",
                    "hostAbi": "arm64-v8a",
                    "runtimeKind": "embedded-proot",
                    "guestPlatform": "linux",
                    "guestArchitecture": "x86_64",
                },
                "payloads": [
                    {"role": "rootfs", "path": "payload/rootfs/rootfs.tar.xz", "sha256": rootfs_sha, "size": len(rootfs_data)},
                    {"role": "runtime", "path": "payload/runtime/runtime.tar.xz", "sha256": runtime_sha, "size": len(runtime_data)},
                ],
                "metadata": [
                    {"role": "guest-layout", "path": "metadata/guest-layout.json", "sha256": gl_sha, "size": len(gl_data.encode("utf-8"))},
                    {"role": "mount-contract", "path": "metadata/mount-contract.json", "sha256": mc_sha, "size": len(mc_data.encode("utf-8"))},
                    {"role": "sha256sums", "path": "metadata/SHA256SUMS", "sha256": sums_sha, "size": len(sums_raw.encode("utf-8"))},
                ],
            }
            with open(os.path.join(src, "metadata", "package-index.json"), "w", encoding="utf-8") as f:
                json.dump(index, f)

            pkg_path = os.path.join(self.tmpdir, "bad-arch.zip")
            self._build_from_src(src, pkg_path)
            result = validate_package(pkg_path, "0.1.0", "abc")
            self.assertFalse(result.valid)

    def test_rootfs_missing(self):
        from package_validator import validate_package
        with tempfile.TemporaryDirectory() as src:
            os.makedirs(os.path.join(src, "payload", "runtime"), exist_ok=True)
            os.makedirs(os.path.join(src, "metadata"), exist_ok=True)

            runtime_data = b"runtime"
            with open(os.path.join(src, "payload", "runtime", "runtime.tar.xz"), "wb") as f:
                f.write(runtime_data)

            gl_data = json.dumps({"root": "/opt/amitia", "directories": ["/opt/amitia/backend"]})
            gl_sha = hashlib.sha256(gl_data.encode("utf-8")).hexdigest()
            with open(os.path.join(src, "metadata", "guest-layout.json"), "w", encoding="utf-8") as f:
                f.write(gl_data)
            mc_data = json.dumps({"binds": [{"source": "/host", "target": "/opt/amitia", "readOnly": False}]})
            mc_sha = hashlib.sha256(mc_data.encode("utf-8")).hexdigest()
            with open(os.path.join(src, "metadata", "mount-contract.json"), "w", encoding="utf-8") as f:
                f.write(mc_data)

            runtime_sha = hashlib.sha256(runtime_data).hexdigest()
            sums_raw = f"{runtime_sha}  payload/runtime/runtime.tar.xz\n{gl_sha}  metadata/guest-layout.json\n{mc_sha}  metadata/mount-contract.json\n"
            sums_sha = hashlib.sha256(sums_raw.encode("utf-8")).hexdigest()
            with open(os.path.join(src, "metadata", "SHA256SUMS"), "w", encoding="utf-8") as f:
                f.write(sums_raw)

            lock_data = {"runtimeVersion": "0.1.0", "packageId": "abc1234", "components": [{"id": "x", "sha256": "b" * 64, "version": "1.0", "architecture": "arm64", "path": "p"}]}
            with open(os.path.join(src, "metadata", "component-lock.json"), "w", encoding="utf-8") as f:
                json.dump(lock_data, f)

            index = {
                "schemaVersion": 1,
                "packageFormatVersion": 1,
                "runtimeVersion": "0.1.0",
                "packageId": "abc1234",
                "sourceRevision": "abc1234567890def",
                "target": {
                    "hostPlatform": "android",
                    "hostAbi": "arm64-v8a",
                    "runtimeKind": "embedded-proot",
                    "guestPlatform": "linux",
                    "guestArchitecture": "arm64",
                },
                "payloads": [
                    {"role": "rootfs", "path": "payload/rootfs/rootfs.tar.xz", "sha256": "a" * 64, "size": 100},
                    {"role": "runtime", "path": "payload/runtime/runtime.tar.xz", "sha256": runtime_sha, "size": len(runtime_data)},
                ],
                "metadata": [
                    {"role": "guest-layout", "path": "metadata/guest-layout.json", "sha256": gl_sha, "size": len(gl_data.encode("utf-8"))},
                    {"role": "mount-contract", "path": "metadata/mount-contract.json", "sha256": mc_sha, "size": len(mc_data.encode("utf-8"))},
                    {"role": "sha256sums", "path": "metadata/SHA256SUMS", "sha256": sums_sha, "size": len(sums_raw.encode("utf-8"))},
                ],
            }
            with open(os.path.join(src, "metadata", "package-index.json"), "w", encoding="utf-8") as f:
                json.dump(index, f)

            pkg_path = os.path.join(self.tmpdir, "missing-rootfs.zip")
            with zipfile.ZipFile(pkg_path, "w", zipfile.ZIP_DEFLATED) as zf:
                for dirpath, dirnames, filenames in os.walk(src):
                    for name in filenames:
                        full = os.path.join(dirpath, name)
                        rel = os.path.relpath(full, src)
                        zf.write(full, rel)

            result = validate_package(pkg_path, "0.1.0", "abc1234")
            self.assertFalse(result.valid)
            self.assertTrue(any("ROOTFS" in err.get("code", "") for err in result.errors))

    def test_component_lock_invalid(self):
        from package_validator import validate_package
        with tempfile.TemporaryDirectory() as src:
            os.makedirs(os.path.join(src, "payload", "rootfs"), exist_ok=True)
            os.makedirs(os.path.join(src, "payload", "runtime"), exist_ok=True)
            os.makedirs(os.path.join(src, "metadata"), exist_ok=True)

            rootfs_data = b"rootfs"
            with open(os.path.join(src, "payload", "rootfs", "rootfs.tar.xz"), "wb") as f:
                f.write(rootfs_data)
            runtime_data = b"runtime"
            with open(os.path.join(src, "payload", "runtime", "runtime.tar.xz"), "wb") as f:
                f.write(runtime_data)

            gl_data = json.dumps({"root": "/opt/amitia", "directories": ["/opt/amitia/backend"]})
            with open(os.path.join(src, "metadata", "guest-layout.json"), "w", encoding="utf-8") as f:
                f.write(gl_data)

            mc_data = json.dumps({"binds": [{"source": "/host", "target": "/opt/amitia", "readOnly": False}]})
            with open(os.path.join(src, "metadata", "mount-contract.json"), "w", encoding="utf-8") as f:
                f.write(mc_data)

            rootfs_sha = hashlib.sha256(rootfs_data).hexdigest()
            runtime_sha = hashlib.sha256(runtime_data).hexdigest()
            gl_sha = hashlib.sha256(gl_data.encode("utf-8")).hexdigest()
            mc_sha = hashlib.sha256(mc_data.encode("utf-8")).hexdigest()
            sums_raw = f"{rootfs_sha}  payload/rootfs/rootfs.tar.xz\n{runtime_sha}  payload/runtime/runtime.tar.xz\n{gl_sha}  metadata/guest-layout.json\n{mc_sha}  metadata/mount-contract.json\n"
            sums_sha = hashlib.sha256(sums_raw.encode("utf-8")).hexdigest()
            with open(os.path.join(src, "metadata", "SHA256SUMS"), "w", encoding="utf-8") as f:
                f.write(sums_raw)

            lock_data = {"runtimeVersion": "0.1.0", "packageId": "abc1234", "components": []}
            with open(os.path.join(src, "metadata", "component-lock.json"), "w", encoding="utf-8") as f:
                json.dump(lock_data, f)

            index = {
                "schemaVersion": 1,
                "packageFormatVersion": 1,
                "runtimeVersion": "0.1.0",
                "packageId": "abc1234",
                "sourceRevision": "abc1234567890def",
                "target": {
                    "hostPlatform": "android",
                    "hostAbi": "arm64-v8a",
                    "runtimeKind": "embedded-proot",
                    "guestPlatform": "linux",
                    "guestArchitecture": "arm64",
                },
                "payloads": [
                    {"role": "rootfs", "path": "payload/rootfs/rootfs.tar.xz", "sha256": rootfs_sha, "size": len(rootfs_data)},
                    {"role": "runtime", "path": "payload/runtime/runtime.tar.xz", "sha256": runtime_sha, "size": len(runtime_data)},
                ],
                "metadata": [
                    {"role": "guest-layout", "path": "metadata/guest-layout.json", "sha256": gl_sha, "size": len(gl_data)},
                    {"role": "mount-contract", "path": "metadata/mount-contract.json", "sha256": mc_sha, "size": len(mc_data)},
                    {"role": "sha256sums", "path": "metadata/SHA256SUMS", "sha256": sums_sha, "size": len(sums_raw.encode("utf-8"))},
                ],
            }
            with open(os.path.join(src, "metadata", "package-index.json"), "w", encoding="utf-8") as f:
                json.dump(index, f)

            pkg_path = os.path.join(self.tmpdir, "empty-lock.zip")
            with zipfile.ZipFile(pkg_path, "w", zipfile.ZIP_DEFLATED) as zf:
                for dirpath, dirnames, filenames in os.walk(src):
                    for name in filenames:
                        full = os.path.join(dirpath, name)
                        rel = os.path.relpath(full, src)
                        zf.write(full, rel)

            result = validate_package(pkg_path, "0.1.0", "abc1234")
            self.assertFalse(result.valid)
            self.assertTrue(any("COMPONENT_LOCK" in err.get("code", "") for err in result.errors))

    def test_safe_path_validation(self):
        from package_validator import _is_path_safe
        self.assertTrue(_is_path_safe("file.txt", "/tmp/test"))
        self.assertTrue(_is_path_safe("subdir/file.txt", "/tmp/test"))
        self.assertFalse(_is_path_safe("../etc/passwd", "/tmp/test"))

    def test_sha256_file(self):
        from package_validator import sha256_file
        test_file = os.path.join(self.tmpdir, "sha_test.txt")
        with open(test_file, "w", encoding="utf-8") as f:
            f.write("test content")
        digest = sha256_file(test_file)
        expected = hashlib.sha256(b"test content").hexdigest()
        self.assertEqual(digest, expected)

    def test_guest_layout_missing(self):
        from package_validator import validate_package
        with tempfile.TemporaryDirectory() as src:
            os.makedirs(os.path.join(src, "payload", "rootfs"), exist_ok=True)
            os.makedirs(os.path.join(src, "payload", "runtime"), exist_ok=True)
            os.makedirs(os.path.join(src, "metadata"), exist_ok=True)

            rootfs_data = b"rootfs"
            with open(os.path.join(src, "payload", "rootfs", "rootfs.tar.xz"), "wb") as f:
                f.write(rootfs_data)
            runtime_data = b"runtime"
            with open(os.path.join(src, "payload", "runtime", "runtime.tar.xz"), "wb") as f:
                f.write(runtime_data)

            mc_data = json.dumps({"binds": [{"source": "/host", "target": "/opt/amitia", "readOnly": False}]})
            mc_sha = hashlib.sha256(mc_data.encode("utf-8")).hexdigest()
            with open(os.path.join(src, "metadata", "mount-contract.json"), "w", encoding="utf-8") as f:
                f.write(mc_data)

            rootfs_sha = hashlib.sha256(rootfs_data).hexdigest()
            runtime_sha = hashlib.sha256(runtime_data).hexdigest()
            sums_raw = f"{rootfs_sha}  payload/rootfs/rootfs.tar.xz\n{runtime_sha}  payload/runtime/runtime.tar.xz\n{mc_sha}  metadata/mount-contract.json\n"
            sums_sha = hashlib.sha256(sums_raw.encode("utf-8")).hexdigest()
            with open(os.path.join(src, "metadata", "SHA256SUMS"), "w", encoding="utf-8") as f:
                f.write(sums_raw)

            lock_data = {"runtimeVersion": "0.1.0", "packageId": "abc1234", "components": [{"id": "x", "sha256": "b" * 64, "version": "1.0", "architecture": "arm64", "path": "p"}]}
            with open(os.path.join(src, "metadata", "component-lock.json"), "w", encoding="utf-8") as f:
                json.dump(lock_data, f)

            index = {
                "schemaVersion": 1,
                "packageFormatVersion": 1,
                "runtimeVersion": "0.1.0",
                "packageId": "abc1234",
                "sourceRevision": "abc1234567890def",
                "target": {
                    "hostPlatform": "android",
                    "hostAbi": "arm64-v8a",
                    "runtimeKind": "embedded-proot",
                    "guestPlatform": "linux",
                    "guestArchitecture": "arm64",
                },
                "payloads": [
                    {"role": "rootfs", "path": "payload/rootfs/rootfs.tar.xz", "sha256": rootfs_sha, "size": len(rootfs_data)},
                    {"role": "runtime", "path": "payload/runtime/runtime.tar.xz", "sha256": runtime_sha, "size": len(runtime_data)},
                ],
                "metadata": [
                    {"role": "mount-contract", "path": "metadata/mount-contract.json", "sha256": mc_sha, "size": len(mc_data)},
                    {"role": "sha256sums", "path": "metadata/SHA256SUMS", "sha256": sums_sha, "size": len(sums_raw.encode("utf-8"))},
                ],
            }
            with open(os.path.join(src, "metadata", "package-index.json"), "w", encoding="utf-8") as f:
                json.dump(index, f)

            pkg_path = os.path.join(self.tmpdir, "no-gl.zip")
            with zipfile.ZipFile(pkg_path, "w", zipfile.ZIP_DEFLATED) as zf:
                for dirpath, dirnames, filenames in os.walk(src):
                    for name in filenames:
                        full = os.path.join(dirpath, name)
                        rel = os.path.relpath(full, src)
                        zf.write(full, rel)

            result = validate_package(pkg_path, "0.1.0", "abc1234")
            self.assertFalse(result.valid)

    def test_unknown_entry_rejected(self):
        from package_validator import validate_package
        with tempfile.TemporaryDirectory() as src:
            os.makedirs(os.path.join(src, "payload", "rootfs"), exist_ok=True)
            os.makedirs(os.path.join(src, "payload", "runtime"), exist_ok=True)
            os.makedirs(os.path.join(src, "metadata"), exist_ok=True)

            rootfs_data = b"rootfs"
            with open(os.path.join(src, "payload", "rootfs", "rootfs.tar.xz"), "wb") as f:
                f.write(rootfs_data)
            runtime_data = b"runtime"
            with open(os.path.join(src, "payload", "runtime", "runtime.tar.xz"), "wb") as f:
                f.write(runtime_data)

            gl_data = json.dumps({"root": "/opt/amitia", "directories": ["/opt/amitia/backend"]})
            gl_sha = hashlib.sha256(gl_data.encode("utf-8")).hexdigest()
            with open(os.path.join(src, "metadata", "guest-layout.json"), "w", encoding="utf-8") as f:
                f.write(gl_data)
            mc_data = json.dumps({"binds": [{"source": "/h", "target": "/opt/amitia", "readOnly": False}]})
            mc_sha = hashlib.sha256(mc_data.encode("utf-8")).hexdigest()
            with open(os.path.join(src, "metadata", "mount-contract.json"), "w", encoding="utf-8") as f:
                f.write(mc_data)

            rootfs_sha = hashlib.sha256(rootfs_data).hexdigest()
            runtime_sha = hashlib.sha256(runtime_data).hexdigest()
            sums_raw = f"{rootfs_sha}  payload/rootfs/rootfs.tar.xz\n{runtime_sha}  payload/runtime/runtime.tar.xz\n{gl_sha}  metadata/guest-layout.json\n{mc_sha}  metadata/mount-contract.json\n"
            sums_sha = hashlib.sha256(sums_raw.encode("utf-8")).hexdigest()
            with open(os.path.join(src, "metadata", "SHA256SUMS"), "w", encoding="utf-8") as f:
                f.write(sums_raw)
            lock_data = {"runtimeVersion": "0.1.0", "packageId": "abc1234", "components": [{"id": "x", "sha256": "b" * 64, "version": "1.0", "architecture": "arm64", "path": "p"}]}
            with open(os.path.join(src, "metadata", "component-lock.json"), "w", encoding="utf-8") as f:
                json.dump(lock_data, f)

            index = {
                "schemaVersion": 1,
                "packageFormatVersion": 1,
                "runtimeVersion": "0.1.0",
                "packageId": "abc1234",
                "sourceRevision": "abc1234567890def",
                "target": {
                    "hostPlatform": "android",
                    "hostAbi": "arm64-v8a",
                    "runtimeKind": "embedded-proot",
                    "guestPlatform": "linux",
                    "guestArchitecture": "arm64",
                },
                "payloads": [
                    {"role": "rootfs", "path": "payload/rootfs/rootfs.tar.xz", "sha256": rootfs_sha, "size": len(rootfs_data)},
                    {"role": "runtime", "path": "payload/runtime/runtime.tar.xz", "sha256": runtime_sha, "size": len(runtime_data)},
                ],
                "metadata": [
                    {"role": "guest-layout", "path": "metadata/guest-layout.json", "sha256": gl_sha, "size": len(gl_data)},
                    {"role": "mount-contract", "path": "metadata/mount-contract.json", "sha256": mc_sha, "size": len(mc_data)},
                    {"role": "sha256sums", "path": "metadata/SHA256SUMS", "sha256": sums_sha, "size": len(sums_raw.encode("utf-8"))},
                ],
            }
            with open(os.path.join(src, "metadata", "package-index.json"), "w", encoding="utf-8") as f:
                json.dump(index, f)
            with open(os.path.join(src, "evil.txt"), "w", encoding="utf-8") as f:
                f.write("secret")

            pkg_path = os.path.join(self.tmpdir, "unknown-entry.zip")
            with zipfile.ZipFile(pkg_path, "w", zipfile.ZIP_DEFLATED) as zf:
                for dirpath, dirnames, filenames in os.walk(src):
                    for name in filenames:
                        full = os.path.join(dirpath, name)
                        rel = os.path.relpath(full, src)
                        zf.write(full, rel)

            result = validate_package(pkg_path, "0.1.0", "abc1234")
            self.assertFalse(result.valid)
            self.assertTrue(any("UNKNOWN" in err.get("code", "") for err in result.errors))

    def test_placeholder_rejected(self):
        from package_validator import validate_package, _has_placeholder
        self.assertTrue(_has_placeholder("placeholder"))
        self.assertTrue(_has_placeholder("TODO_SHA"))
        self.assertTrue(_has_placeholder("TBD"))
        self.assertFalse(_has_placeholder("abc1234567890"))
        self.assertTrue(_has_placeholder(""))

        with tempfile.TemporaryDirectory() as src:
            os.makedirs(os.path.join(src, "payload", "rootfs"), exist_ok=True)
            os.makedirs(os.path.join(src, "payload", "runtime"), exist_ok=True)
            os.makedirs(os.path.join(src, "metadata"), exist_ok=True)

            rootfs_data = b"rootfs"
            with open(os.path.join(src, "payload", "rootfs", "rootfs.tar.xz"), "wb") as f:
                f.write(rootfs_data)
            runtime_data = b"runtime"
            with open(os.path.join(src, "payload", "runtime", "runtime.tar.xz"), "wb") as f:
                f.write(runtime_data)

            gl_data = json.dumps({"root": "/opt/amitia", "directories": ["/opt/amitia/backend"]})
            gl_sha = hashlib.sha256(gl_data.encode("utf-8")).hexdigest()
            with open(os.path.join(src, "metadata", "guest-layout.json"), "w", encoding="utf-8") as f:
                f.write(gl_data)
            mc_data = json.dumps({"binds": [{"source": "/h", "target": "/opt/amitia", "readOnly": False}]})
            mc_sha = hashlib.sha256(mc_data.encode("utf-8")).hexdigest()
            with open(os.path.join(src, "metadata", "mount-contract.json"), "w", encoding="utf-8") as f:
                f.write(mc_data)

            rootfs_sha = hashlib.sha256(rootfs_data).hexdigest()
            runtime_sha = hashlib.sha256(runtime_data).hexdigest()
            sums_raw = f"{rootfs_sha}  payload/rootfs/rootfs.tar.xz\n{runtime_sha}  payload/runtime/runtime.tar.xz\n{gl_sha}  metadata/guest-layout.json\n{mc_sha}  metadata/mount-contract.json\n"
            sums_sha = hashlib.sha256(sums_raw.encode("utf-8")).hexdigest()
            with open(os.path.join(src, "metadata", "SHA256SUMS"), "w", encoding="utf-8") as f:
                f.write(sums_raw)
            lock_data = {"runtimeVersion": "0.1.0", "packageId": "abc1234", "components": [{"id": "x", "sha256": "b" * 64, "version": "1.0", "architecture": "arm64", "path": "p"}]}
            with open(os.path.join(src, "metadata", "component-lock.json"), "w", encoding="utf-8") as f:
                json.dump(lock_data, f)

            index = {
                "schemaVersion": 1,
                "packageFormatVersion": 1,
                "runtimeVersion": "0.1.0",
                "packageId": "abc1234",
                "sourceRevision": "abc1234567890def",
                "target": {
                    "hostPlatform": "android",
                    "hostAbi": "arm64-v8a",
                    "runtimeKind": "embedded-proot",
                    "guestPlatform": "linux",
                    "guestArchitecture": "arm64",
                },
                "payloads": [
                    {"role": "rootfs", "path": "payload/rootfs/rootfs.tar.xz", "sha256": "placeholder", "size": len(rootfs_data)},
                    {"role": "runtime", "path": "payload/runtime/runtime.tar.xz", "sha256": runtime_sha, "size": len(runtime_data)},
                ],
                "metadata": [
                    {"role": "guest-layout", "path": "metadata/guest-layout.json", "sha256": gl_sha, "size": len(gl_data)},
                    {"role": "mount-contract", "path": "metadata/mount-contract.json", "sha256": mc_sha, "size": len(mc_data)},
                    {"role": "sha256sums", "path": "metadata/SHA256SUMS", "sha256": sums_sha, "size": len(sums_raw.encode("utf-8"))},
                ],
            }
            with open(os.path.join(src, "metadata", "package-index.json"), "w", encoding="utf-8") as f:
                json.dump(index, f)

            pkg_path = os.path.join(self.tmpdir, "placeholder.zip")
            with zipfile.ZipFile(pkg_path, "w", zipfile.ZIP_DEFLATED) as zf:
                for dirpath, dirnames, filenames in os.walk(src):
                    for name in filenames:
                        full = os.path.join(dirpath, name)
                        rel = os.path.relpath(full, src)
                        zf.write(full, rel)

            result = validate_package(pkg_path, "0.1.0", "abc1234")
            self.assertFalse(result.valid)


if __name__ == "__main__":
    unittest.main()
