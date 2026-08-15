import os
import shutil
import stat
import sys
import tempfile
import unittest

from common.tree_manifest import generate_tree_manifest
from common.hashing import sha256_file, sha256_tree_manifest


class TestGenerateTreeManifest(unittest.TestCase):
    def setUp(self):
        self.tmpdir = tempfile.mkdtemp()

    def tearDown(self):
        shutil.rmtree(self.tmpdir, ignore_errors=True)

    def _create_file(self, rel_path, content="data"):
        full = os.path.join(self.tmpdir, rel_path)
        os.makedirs(os.path.dirname(full), exist_ok=True)
        with open(full, "w") as f:
            f.write(content)
        return full

    def _create_dir(self, rel_path):
        full = os.path.join(self.tmpdir, rel_path)
        os.makedirs(full, exist_ok=True)
        return full

    def _create_symlink(self, rel_path, target):
        full = os.path.join(self.tmpdir, rel_path)
        os.makedirs(os.path.dirname(full), exist_ok=True)
        os.symlink(target, full)
        return full

    def test_file_entry_type(self):
        self._create_file("test.txt", "hello")
        lines = generate_tree_manifest(self.tmpdir)
        file_lines = [l for l in lines if l.startswith("F ")]
        self.assertTrue(len(file_lines) >= 1)
        self.assertIn("test.txt", file_lines[0])

    def test_directory_entry_type(self):
        self._create_dir("mydir")
        self._create_file("mydir/file.txt", "x")
        lines = generate_tree_manifest(self.tmpdir)
        dir_lines = [l for l in lines if l.startswith("D ")]
        self.assertTrue(len(dir_lines) >= 1)
        self.assertTrue(any("mydir" in l for l in dir_lines))

    def test_symlink_entry_type(self):
        if sys.platform == "win32":
            self._create_file("target.txt", "content")
            try:
                self._create_symlink("link.txt", "target.txt")
            except OSError:
                self.skipTest("symlink requires admin privileges on Windows")
                return
        else:
            self._create_file("target.txt", "content")
            self._create_symlink("link.txt", "target.txt")
        lines = generate_tree_manifest(self.tmpdir)
        link_lines = [l for l in lines if l.startswith("L ")]
        self.assertTrue(len(link_lines) >= 1)
        self.assertIn("link.txt", link_lines[0])
        self.assertIn("target.txt", link_lines[0])

    def test_file_entry_contains_sha256(self):
        fpath = self._create_file("hashtest.txt", "specific content")
        expected_hash = sha256_file(fpath)
        lines = generate_tree_manifest(self.tmpdir)
        file_lines = [l for l in lines if l.startswith("F ") and "hashtest.txt" in l]
        self.assertEqual(len(file_lines), 1)
        self.assertIn(expected_hash, file_lines[0])

    def test_directory_entry_format(self):
        self._create_dir("formatdir")
        self._create_file("formatdir/x.txt", "x")
        lines = generate_tree_manifest(self.tmpdir)
        dir_lines = [l for l in lines if l.startswith("D ") and "formatdir" in l]
        self.assertEqual(len(dir_lines), 1)
        parts = dir_lines[0].split()
        self.assertEqual(parts[0], "D")
        self.assertEqual(parts[2], "-")

    def test_tree_sha256_computation(self):
        self._create_file("a.txt", "aaa")
        self._create_file("b.txt", "bbb")
        lines = generate_tree_manifest(self.tmpdir)
        tree_hash = sha256_tree_manifest(lines)
        self.assertEqual(len(tree_hash), 64)
        self.assertTrue(all(c in "0123456789abcdef" for c in tree_hash))

    def test_tree_sha256_deterministic(self):
        self._create_file("x.txt", "xxx")
        self._create_file("y.txt", "yyy")
        lines1 = generate_tree_manifest(self.tmpdir)
        lines2 = generate_tree_manifest(self.tmpdir)
        hash1 = sha256_tree_manifest(lines1)
        hash2 = sha256_tree_manifest(lines2)
        self.assertEqual(hash1, hash2)

    def test_nonexistent_root_raises(self):
        with self.assertRaises(FileNotFoundError):
            generate_tree_manifest("/nonexistent/path/for/test")

    def test_nested_directory_structure(self):
        self._create_file("top.txt", "top")
        self._create_file("dir1/mid.txt", "mid")
        self._create_file("dir1/dir2/deep.txt", "deep")
        lines = generate_tree_manifest(self.tmpdir)
        joined = "\n".join(lines)
        self.assertIn("top.txt", joined)
        self.assertIn("dir1/mid.txt", joined)
        self.assertIn("dir1/dir2/deep.txt", joined)

    def test_mode_present_in_file_entry(self):
        fpath = self._create_file("mode.txt", "content")
        lines = generate_tree_manifest(self.tmpdir)
        file_lines = [l for l in lines if l.startswith("F ") and "mode.txt" in l]
        self.assertEqual(len(file_lines), 1)
        parts = file_lines[0].split()
        self.assertEqual(parts[0], "F")
        self.assertTrue(len(parts[1]) == 4 and parts[1].startswith("o"))


if __name__ == "__main__":
    unittest.main()
