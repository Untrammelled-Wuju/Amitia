import hashlib
import os
import pathlib
import shutil
import sys
import tempfile
import unittest
import zipfile

sys.path.insert(0, os.path.dirname(os.path.abspath(__file__)))
import package_writer as pw


class TestPackageWriter(unittest.TestCase):
    def setUp(self):
        self.tmpdir = pathlib.Path(tempfile.mkdtemp(prefix="pw_test_"))

    def tearDown(self):
        shutil.rmtree(str(self.tmpdir), ignore_errors=True)

    def test_deterministic_zip(self):
        payload = [
            {"name": "a.json", "data": b'{"a":1}'},
            {"name": "b.txt", "data": b"hello"},
            {"name": "c.txt", "data": b"world"},
        ]
        out1 = self.tmpdir / "test1.zip"
        pw.write_zip(payload, str(out1))
        out2 = self.tmpdir / "test2.zip"
        pw.write_zip(payload, str(out2))
        sha1 = hashlib.sha256(out1.read_bytes()).hexdigest()
        sha2 = hashlib.sha256(out2.read_bytes()).hexdigest()
        self.assertEqual(sha1, sha2)

    def test_zip_stored_only(self):
        payload = [{"name": "test.txt", "data": b"content" * 100}]
        out = self.tmpdir / "test.zip"
        pw.write_zip(payload, str(out))
        with zipfile.ZipFile(str(out), "r") as zf:
            for info in zf.infolist():
                self.assertEqual(info.compress_type, zipfile.ZIP_STORED)
                self.assertEqual(info.date_time, (1980, 1, 1, 0, 0, 0))
                self.assertEqual(info.create_system, 3)

    def test_sorted_entries(self):
        payload = [
            {"name": "z.txt", "data": b"1"},
            {"name": "a.txt", "data": b"2"},
            {"name": "m.txt", "data": b"3"},
        ]
        out = self.tmpdir / "test.zip"
        pw.write_zip(payload, str(out))
        with zipfile.ZipFile(str(out), "r") as zf:
            names = zf.namelist()
        self.assertEqual(names, sorted(names))

    def test_duplicate_rejected(self):
        payload = [
            {"name": "a.txt", "data": b"1"},
            {"name": "a.txt", "data": b"2"},
        ]
        with self.assertRaises(RuntimeError):
            pw.write_zip(payload, str(self.tmpdir / "test.zip"))

    def test_absolute_path_rejected(self):
        payload = [{"name": "/etc/passwd", "data": b"x"}]
        with self.assertRaises(RuntimeError):
            pw.write_zip(payload, str(self.tmpdir / "test.zip"))

    def test_path_traversal_rejected(self):
        payload = [{"name": "../escape.txt", "data": b"x"}]
        with self.assertRaises(RuntimeError):
            pw.write_zip(payload, str(self.tmpdir / "test.zip"))

    def test_sha_file(self):
        sha = "abc123" * 8
        target = "test.zip"
        out = self.tmpdir / "test.zip.sha256"
        pw.write_sha_file(sha, target, str(out))
        content = out.read_text(encoding="utf-8").strip()
        self.assertEqual(content, f"{sha}  {target}")


def test_suite_loader():
    loader = unittest.TestLoader()
    suite = unittest.TestSuite()
    suite.addTests(loader.loadTestsFromTestCase(TestPackageWriter))
    return suite


if __name__ == "__main__":
    unittest.main()
