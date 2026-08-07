import hashlib
import json
import lzma
import os
import pathlib
import shutil
import tarfile
import tempfile
import unittest

SCRIPT_DIR = pathlib.Path(__file__).resolve().parent
TEMP_ROOT_MAX = 300

try:
    from build import (
        build_overlay,
        build_guest_layout_json,
        build_mount_contract_json,
        build_file_manifest,
        DEFAULT_OUTPUT_DIR,
        ARCHIVE_NAME,
        FILE_MANIFEST,
        SHA256SUMS_FILE,
        MANIFEST_GUEST_LAYOUT,
        MANIFEST_MOUNT_CONTRACT,
    )
    _HAS_BUILD = True
except ImportError:
    _HAS_BUILD = False

try:
    from verify import (
        validate_linux_path,
        validate_directories,
        validate_mount_contract,
        validate_lock_structure,
        validate_paths_section,
        ALLOWED_PERSISTENCE,
        ALLOWED_PURPOSE,
    )
    _HAS_VERIFY = True
except ImportError:
    _HAS_VERIFY = False


def load_json(path):
    with open(path, "r", encoding="utf-8") as f:
        return json.load(f)


@unittest.skipUnless(_HAS_BUILD and _HAS_VERIFY, "import failed")
class TestBuildOverlay(unittest.TestCase):
    def test_build_creates_all_expected_outputs(self):
        with tempfile.TemporaryDirectory() as tmpdir:
            output = pathlib.Path(tmpdir) / "out"
            result = build_overlay(output)
            self.assertTrue((output / "overlay").is_dir())
            self.assertTrue((output / MANIFEST_GUEST_LAYOUT).is_file())
            self.assertTrue((output / MANIFEST_MOUNT_CONTRACT).is_file())
            self.assertTrue((output / FILE_MANIFEST).is_file())
            self.assertTrue((output / ARCHIVE_NAME).is_file())
            self.assertTrue((output / SHA256SUMS_FILE).is_file())

    def test_manifest_dir_in_overlay(self):
        with tempfile.TemporaryDirectory() as tmpdir:
            output = pathlib.Path(tmpdir) / "out"
            build_overlay(output)
            manifest_dir = output / "overlay" / "opt" / "amitia" / "manifest"
            self.assertTrue(manifest_dir.is_dir())
            self.assertTrue((manifest_dir / "guest-layout.json").is_file())
            self.assertTrue((manifest_dir / "mount-contract.json").is_file())

    def test_guest_layout_valid(self):
        with tempfile.TemporaryDirectory() as tmpdir:
            output = pathlib.Path(tmpdir) / "out"
            build_overlay(output)
            gl = load_json(output / MANIFEST_GUEST_LAYOUT)
            self.assertEqual(gl["paths"]["runtimeRoot"], "/opt/amitia")
            self.assertEqual(gl["paths"]["backendBinary"], "/opt/amitia/backend/amitia-server")
            self.assertEqual(gl["paths"]["nodeBinary"], "/opt/amitia/node/bin/node")
            self.assertEqual(gl["paths"]["qdrantBinary"], "/opt/amitia/qdrant/bin/qdrant")
            self.assertEqual(gl["paths"]["pluginHostEntry"], "/opt/amitia/plugin-host/dist/index.js")
            self.assertEqual(gl["paths"]["taskHostEntry"], "/opt/amitia/task-host/dist/index.js")
            self.assertEqual(gl["environment"]["AMITIA_RUNTIME_ROOT"], "/opt/amitia")
            self.assertEqual(gl["environment"]["HOME"], "/home/amitia")

    def test_file_manifest_sorted(self):
        with tempfile.TemporaryDirectory() as tmpdir:
            output = pathlib.Path(tmpdir) / "out"
            build_overlay(output)
            fm = load_json(output / FILE_MANIFEST)
            paths = [e["path"] for e in fm]
            self.assertEqual(paths, sorted(paths))
            for entry in fm:
                self.assertIn("path", entry)
                self.assertIn("type", entry)
                self.assertEqual(entry["type"], "directory")
                self.assertIn("mode", entry)
                self.assertIn("uid", entry)
                self.assertIn("gid", entry)
                self.assertIn("persistence", entry)
                self.assertIn("purpose", entry)

    def test_sha256sums_matches_files(self):
        with tempfile.TemporaryDirectory() as tmpdir:
            output = pathlib.Path(tmpdir) / "out"
            build_overlay(output)
            sums_path = output / SHA256SUMS_FILE
            with open(sums_path, "r", encoding="utf-8") as f:
                lines = f.read().strip().splitlines()
            for line in lines:
                sha, name = line.split("  ", 1)
                target = output / name
                self.assertTrue(target.exists(), name)
                h = hashlib.sha256()
                with open(target, "rb") as tf:
                    h.update(tf.read())
                self.assertEqual(h.hexdigest(), sha, name)

    def test_archive_member_structure(self):
        with tempfile.TemporaryDirectory() as tmpdir:
            output = pathlib.Path(tmpdir) / "out"
            build_overlay(output)
            archive_path = output / ARCHIVE_NAME
            with lzma.open(archive_path, "rb") as xz:
                with tarfile.open(fileobj=xz, mode="r:") as tf:
                    members = tf.getmembers()
            firsts = set()
            for m in members:
                self.assertFalse(m.name.startswith("/"))
                self.assertFalse(m.name.startswith("overlay/"))
                self.assertFalse(m.name.startswith("guest-layout/"))
                parts = m.name.split("/")
                if parts and parts[0]:
                    firsts.add(parts[0])
            self.assertEqual(firsts, {"opt", "etc", "var", "run"})

    def test_archive_contains_manifests(self):
        with tempfile.TemporaryDirectory() as tmpdir:
            output = pathlib.Path(tmpdir) / "out"
            build_overlay(output)
            archive_path = output / ARCHIVE_NAME
            with lzma.open(archive_path, "rb") as xz:
                with tarfile.open(fileobj=xz, mode="r:") as tf:
                    names = [m.name for m in tf.getmembers()]
            self.assertIn("opt/amitia/manifest/guest-layout.json", names)
            self.assertIn("opt/amitia/manifest/mount-contract.json", names)

    def test_archive_excludes_forbidden_files(self):
        with tempfile.TemporaryDirectory() as tmpdir:
            output = pathlib.Path(tmpdir) / "out"
            build_overlay(output)
            archive_path = output / ARCHIVE_NAME
            with lzma.open(archive_path, "rb") as xz:
                with tarfile.open(fileobj=xz, mode="r:") as tf:
                    names = [m.name for m in tf.getmembers()]
            for n in names:
                basename = n.split("/")[-1]
                self.assertNotIn(basename, (".keep", ".gitkeep", "placeholder", "README"))

    def test_deterministic_build(self):
        with tempfile.TemporaryDirectory() as tmpdir:
            output1 = pathlib.Path(tmpdir) / "out1"
            output2 = pathlib.Path(tmpdir) / "out2"
            build_overlay(output1)
            build_overlay(output2)
            sha1 = hashlib.sha256((output1 / ARCHIVE_NAME).read_bytes()).hexdigest()
            sha2 = hashlib.sha256((output2 / ARCHIVE_NAME).read_bytes()).hexdigest()
            self.assertEqual(sha1, sha2)

    def test_no_placeholder_binaries(self):
        with tempfile.TemporaryDirectory() as tmpdir:
            output = pathlib.Path(tmpdir) / "out"
            build_overlay(output)
            archive_path = output / ARCHIVE_NAME
            with lzma.open(archive_path, "rb") as xz:
                with tarfile.open(fileobj=xz, mode="r:") as tf:
                    for m in tf.getmembers():
                        self.assertNotIn(m.name, (
                            "opt/amitia/backend/amitia-server",
                            "opt/amitia/node/bin/node",
                            "opt/amitia/qdrant/bin/qdrant",
                            "opt/amitia/plugin-host/dist/index.js",
                            "opt/amitia/task-host/dist/index.js",
                        ))

    def test_no_logs_or_runtime_state(self):
        with tempfile.TemporaryDirectory() as tmpdir:
            output = pathlib.Path(tmpdir) / "out"
            build_overlay(output)
            log_dir = output / "overlay" / "var" / "log" / "amitia"
            run_dir = output / "overlay" / "run" / "amitia"
            self.assertTrue(log_dir.is_dir())
            self.assertTrue(run_dir.is_dir())
            for d in (log_dir, run_dir):
                for entry in d.iterdir():
                    self.assertTrue(entry.is_dir(), f"{entry} should be a directory")

    def test_default_output_dir_constant(self):
        self.assertTrue("out" in str(DEFAULT_OUTPUT_DIR))
        self.assertNotIn("getcwd", str(DEFAULT_OUTPUT_DIR))
        self.assertNotIn("Path.cwd", str(DEFAULT_OUTPUT_DIR))


@unittest.skipUnless(_HAS_BUILD, "build import failed")
class TestDerivedManifests(unittest.TestCase):
    def test_guest_layout_from_lock(self):
        import build as b
        lock = load_json(SCRIPT_DIR / "guest-layout.lock.json")
        gl = b.build_guest_layout_json(lock)
        self.assertEqual(gl["version"], lock["version"])
        for k in ("runtimeRoot", "backendBinary", "nodeBinary", "qdrantBinary"):
            self.assertIn(k, gl["paths"])

    def test_mount_contract_from_lock(self):
        import build as b
        lock = load_json(SCRIPT_DIR / "guest-layout.lock.json")
        mc = b.build_mount_contract_json(lock)
        self.assertEqual(len(mc["mounts"]), 6)
        self.assertEqual(mc["order"], [m["id"] for m in mc["mounts"]])


if __name__ == "__main__":
    unittest.main()
