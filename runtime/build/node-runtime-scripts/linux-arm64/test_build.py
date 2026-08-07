import json
import lzma
import os
import pathlib
import platform
import shutil
import sys
import tarfile
import tempfile
import unittest

SCRIPT_DIR = pathlib.Path(__file__).resolve().parent
sys.path.insert(0, str(SCRIPT_DIR))

import build

IS_WINDOWS = platform.system() == "Windows"


class LockTests(unittest.TestCase):
    def setUp(self):
        self.tmpdir = tempfile.mkdtemp()
        self.lock_path = pathlib.Path(self.tmpdir) / "scripts.lock.json"

    def tearDown(self):
        shutil.rmtree(self.tmpdir, ignore_errors=True)

    def test_valid_lock(self):
        self.lock_path.write_text(json.dumps({
            "schemaVersion": 1,
            "componentId": "runtime.node-scripts",
            "version": "1",
            "platform": "linux",
            "architecture": "arm64",
            "nodeVersion": "24.19.0",
            "npmVersion": "11.17.0",
            "layout": {"scriptsRoot": "scripts/node"},
        }))
        lock = build.load_lock(self.lock_path)
        self.assertEqual(lock["nodeVersion"], "24.19.0")

    def test_wrong_platform(self):
        self.lock_path.write_text(json.dumps({
            "schemaVersion": 1,
            "componentId": "runtime.node-scripts",
            "version": "1",
            "platform": "win32",
            "architecture": "arm64",
            "nodeVersion": "24.19.0",
            "npmVersion": "11.17.0",
            "layout": {},
        }))
        with self.assertRaises(ValueError):
            build.load_lock(self.lock_path)

    def test_missing_field(self):
        self.lock_path.write_text(json.dumps({"schemaVersion": 1}))
        with self.assertRaises(ValueError):
            build.load_lock(self.lock_path)


class SourceValidationTests(unittest.TestCase):
    def test_required_scripts_exist(self):
        issue = build.verify_source_scripts()
        self.assertEqual(issue, [])

    def test_shebang_correct(self):
        sh_files = [p for p in build.SOURCE_DIR.glob("*.sh") if "/lib/" not in str(p)]
        self.assertTrue(len(sh_files) > 0, "至少需要一个 sh 文件")
        for fp in sh_files:
            data = fp.read_bytes()
            self.assertTrue(
                data.startswith(b"#!/bin/sh\n"),
                f"{fp.name} Shebang 错误",
            )
            self.assertNotIn(b"\r\n", data, f"{fp.name} 含 CRLF")
            self.assertTrue(data.endswith(b"\n"), f"{fp.name} 末尾无换行")

    def test_no_forbidden_commands(self):
        sh_files = [p for p in build.SOURCE_DIR.rglob("*.sh") if "/lib/" not in str(p)]
        for fp in sh_files:
            text = fp.read_text()
            self.assertNotIn("eval", text, f"{fp.name} 含 eval")
            self.assertNotIn("sh -c", text, f"{fp.name} 含 sh -c")
            self.assertNotIn("bash -c", text, f"{fp.name} 含 bash -c")
            self.assertNotIn("nohup", text, f"{fp.name} 含 nohup")
            self.assertNotIn("&", text.strip().split("\n")[-1] if text.strip() else "", fp.name)


class PermissionTests(unittest.TestCase):
    @unittest.skipIf(IS_WINDOWS, "Windows 不支持 POSIX 权限")
    def test_directory_perm_0755(self):
        tmp_root = pathlib.Path(tempfile.mkdtemp())
        try:
            (tmp_root / "lib").mkdir()
            (tmp_root / "test.sh").write_text("#!/bin/sh\necho ok\n")
            build.fix_permissions(tmp_root)
            self.assertEqual(oct((tmp_root / "lib").stat().st_mode & 0o777), "0o755")
            self.assertEqual(oct((tmp_root / "test.sh").stat().st_mode & 0o777), "0o755")
        finally:
            shutil.rmtree(tmp_root, ignore_errors=True)


class ManifestTests(unittest.TestCase):
    def test_manifest_sorted(self):
        tmp_root = pathlib.Path(tempfile.mkdtemp())
        try:
            (tmp_root / "b.txt").write_text("b")
            (tmp_root / "a.txt").write_text("a")
            manifest = build.build_file_manifest(tmp_root)
            paths = [e["path"] for e in manifest]
            self.assertEqual(paths, sorted(paths))
        finally:
            shutil.rmtree(tmp_root, ignore_errors=True)


class ReproducibleTests(unittest.TestCase):
    def test_two_builds_same_sha(self):
        tmp_root = pathlib.Path(tempfile.mkdtemp())
        out_root = pathlib.Path(tempfile.mkdtemp())
        try:
            (tmp_root / "scripts").mkdir()
            (tmp_root / "scripts" / "test.sh").write_text("#!/bin/sh\necho hello\n")
            (tmp_root / "scripts" / "lib").mkdir()
            (tmp_root / "scripts" / "lib" / "common.sh").write_text("# common\n")
            build.fix_permissions(tmp_root / "scripts")
            archive1 = out_root / "a.tar.xz"
            archive2 = out_root / "b.tar.xz"
            build.create_deterministic_tar(tmp_root, archive1)
            build.create_deterministic_tar(tmp_root, archive2)
            self.assertEqual(build.sha256_file(archive1), build.sha256_file(archive2))
        finally:
            shutil.rmtree(tmp_root, ignore_errors=True)
            shutil.rmtree(out_root, ignore_errors=True)

    def test_archive_fixed_mtime(self):
        tmp_root = pathlib.Path(tempfile.mkdtemp())
        out_root = pathlib.Path(tempfile.mkdtemp())
        try:
            (tmp_root / "test.sh").write_text("#!/bin/sh\necho hello\n")
            archive = out_root / "test.tar.xz"
            build.create_deterministic_tar(tmp_root, archive)
            with tarfile.open(archive, "r:xz") as tf:
                for m in tf.getmembers():
                    self.assertEqual(m.mtime, 0)
                    self.assertEqual(m.uid, 0)
                    self.assertEqual(m.gid, 0)
        finally:
            shutil.rmtree(tmp_root, ignore_errors=True)
            shutil.rmtree(out_root, ignore_errors=True)


if __name__ == "__main__":
    unittest.main()
