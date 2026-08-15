import os
import shutil
import stat
import tarfile
import tempfile
import zipfile
import unittest

from common.deterministic_archive import (
    FIXED_MTIME,
    FIXED_UID,
    FIXED_GID,
    FIXED_UNAME,
    FIXED_GNAME,
    create_deterministic_tar_xz,
    create_deterministic_zip,
)


def _create_test_tree(root):
    subdir = os.path.join(root, "subdir")
    os.makedirs(subdir, exist_ok=True)
    with open(os.path.join(root, "file1.txt"), "w") as f:
        f.write("hello world")
    with open(os.path.join(subdir, "file2.txt"), "w") as f:
        f.write("nested content")
    with open(os.path.join(root, "empty.txt"), "w") as f:
        f.write("")


class TestDeterministicTarXz(unittest.TestCase):
    def setUp(self):
        self.src_dir = tempfile.mkdtemp()
        self.out_dir = tempfile.mkdtemp()
        _create_test_tree(self.src_dir)

    def tearDown(self):
        shutil.rmtree(self.src_dir, ignore_errors=True)
        shutil.rmtree(self.out_dir, ignore_errors=True)

    def test_byte_identical_on_repack(self):
        out1 = os.path.join(self.out_dir, "archive1.tar.xz")
        out2 = os.path.join(self.out_dir, "archive2.tar.xz")
        create_deterministic_tar_xz(self.src_dir, out1)
        create_deterministic_tar_xz(self.src_dir, out2)
        with open(out1, "rb") as f:
            data1 = f.read()
        with open(out2, "rb") as f:
            data2 = f.read()
        self.assertEqual(data1, data2)

    def test_fixed_mtime(self):
        out = os.path.join(self.out_dir, "archive.tar.xz")
        create_deterministic_tar_xz(self.src_dir, out)
        with tarfile.open(out, "r:xz") as tf:
            for member in tf.getmembers():
                self.assertEqual(member.mtime, FIXED_MTIME)

    def test_fixed_uid_gid(self):
        out = os.path.join(self.out_dir, "archive.tar.xz")
        create_deterministic_tar_xz(self.src_dir, out)
        with tarfile.open(out, "r:xz") as tf:
            for member in tf.getmembers():
                self.assertEqual(member.uid, FIXED_UID)
                self.assertEqual(member.gid, FIXED_GID)

    def test_fixed_uname_gname(self):
        out = os.path.join(self.out_dir, "archive.tar.xz")
        create_deterministic_tar_xz(self.src_dir, out)
        with tarfile.open(out, "r:xz") as tf:
            for member in tf.getmembers():
                self.assertEqual(member.uname, FIXED_UNAME)
                self.assertEqual(member.gname, FIXED_GNAME)


class TestDeterministicZip(unittest.TestCase):
    def setUp(self):
        self.src_dir = tempfile.mkdtemp()
        self.out_dir = tempfile.mkdtemp()
        _create_test_tree(self.src_dir)

    def tearDown(self):
        shutil.rmtree(self.src_dir, ignore_errors=True)
        shutil.rmtree(self.out_dir, ignore_errors=True)

    def test_byte_identical_on_repack(self):
        out1 = os.path.join(self.out_dir, "archive1.zip")
        out2 = os.path.join(self.out_dir, "archive2.zip")
        create_deterministic_zip(self.src_dir, out1)
        create_deterministic_zip(self.src_dir, out2)
        with open(out1, "rb") as f:
            data1 = f.read()
        with open(out2, "rb") as f:
            data2 = f.read()
        self.assertEqual(data1, data2)

    def test_fixed_time(self):
        import time
        out = os.path.join(self.out_dir, "archive.zip")
        create_deterministic_zip(self.src_dir, out)
        expected_time = time.localtime(FIXED_MTIME)[:6]
        with zipfile.ZipFile(out, "r") as zf:
            for info in zf.infolist():
                self.assertEqual(info.date_time, expected_time)


if __name__ == "__main__":
    unittest.main()
