import os
import pathlib
import sys
import tempfile
import unittest

sys.path.insert(0, os.path.dirname(os.path.abspath(__file__)))
import archive_validator as av


class TestPathNormalization(unittest.TestCase):
    def test_normal_relative(self):
        self.assertEqual(av.normalize_path("opt/amitia/manifest/foo.json"), "opt/amitia/manifest/foo.json")

    def test_subpath_file(self):
        self.assertEqual(av.normalize_path("backend/amitia-server"), "backend/amitia-server")

    def test_empty_fails(self):
        with self.assertRaises(ValueError):
            av.normalize_path("")

    def test_absolute_fails(self):
        with self.assertRaises(ValueError):
            av.normalize_path("/etc/passwd")

    def test_dotdot_fails(self):
        with self.assertRaises(ValueError):
            av.normalize_path("../../etc/passwd")

    def test_backslash_fails(self):
        with self.assertRaises(ValueError):
            av.normalize_path("foo\\bar")

    def test_windows_drive_fails(self):
        with self.assertRaises(ValueError):
            av.normalize_path("C:/file")

    def test_starting_dotdot_fails(self):
        with self.assertRaises(ValueError):
            av.normalize_path("./../../escape")

    def test_nul_fails(self):
        with self.assertRaises(ValueError):
            av.normalize_path("foo\x00bar")

    def test_symlink_normal(self):
        av.validate_member("link", "symlink", linkname="target/file")

    def test_absolute_symlink_fails(self):
        with self.assertRaises(ValueError):
            av.validate_member("link", "symlink", linkname="/etc/passwd")

    def test_symlink_escape_fails(self):
        with self.assertRaises(ValueError):
            av.validate_member("link", "symlink", linkname="../../../etc/passwd")

    def test_block_device_fails(self):
        with self.assertRaises(ValueError):
            av.validate_member("dev", "block")

    def test_fifo_fails(self):
        with self.assertRaises(ValueError):
            av.validate_member("pipe", "fifo")

    def test_socket_fails(self):
        with self.assertRaises(ValueError):
            av.validate_member("sock", "socket")

    def test_dup_check(self):
        self.assertTrue(av.check_no_duplicates(["a", "b", "c"]))

    def test_dup_check_fails(self):
        with self.assertRaises(ValueError):
            av.check_no_duplicates(["a", "b", "a"])

    def test_runtime_absolute_symlink_fails(self):
        with self.assertRaises(ValueError):
            av.validate_runtime_member("link", "symlink", linkname="/opt/foo")

    def test_long_path_fails(self):
        with self.assertRaises(ValueError):
            av.normalize_path("a/" * 5000)


if __name__ == "__main__":
    unittest.main()
