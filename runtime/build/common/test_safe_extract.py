import os
import shutil
import stat
import sys
import tarfile
import tempfile
import unittest
import zipfile

from common.safe_extract import (
    safe_extract_archive,
    safe_extract,
    _normalize_safe_path,
)


class TestSafeExtractTar(unittest.TestCase):
    def setUp(self):
        self.tmpdir = tempfile.mkdtemp()
        self.dest = os.path.join(self.tmpdir, "dest")

    def tearDown(self):
        shutil.rmtree(self.tmpdir, ignore_errors=True)

    def test_absolute_path_rejected(self):
        tar_path = os.path.join(self.tmpdir, "absolute.tar")
        with tarfile.open(tar_path, "w") as tf:
            info = tarfile.TarInfo(name="/etc/passwd")
            info.size = 4
            import io
            tf.addfile(info, io.BytesIO(b"root"))
        result = safe_extract_archive(tar_path, self.dest)
        self.assertFalse(result.success)
        self.assertTrue(any("Absolute path rejected" in e for e in result.errors))

    def test_dotdot_escape_rejected(self):
        tar_path = os.path.join(self.tmpdir, "dotdot.tar")
        with tarfile.open(tar_path, "w") as tf:
            info = tarfile.TarInfo(name="../etc/passwd")
            info.size = 4
            import io
            tf.addfile(info, io.BytesIO(b"root"))
        result = safe_extract_archive(tar_path, self.dest)
        self.assertFalse(result.success)
        self.assertTrue(
            any("Path traversal" in e or "escapes" in e for e in result.errors),
            f"Expected traversal/escapes error, got: {result.errors}",
        )

    def test_symlink_escape_rejected(self):
        tar_path = os.path.join(self.tmpdir, "symlink.tar")
        with tarfile.open(tar_path, "w") as tf:
            info = tarfile.TarInfo(name="evil-link")
            info.type = tarfile.SYMTYPE
            info.linkname = "../etc/passwd"
            tf.addfile(info)
        result = safe_extract_archive(tar_path, self.dest)
        self.assertFalse(result.success)
        self.assertTrue(any("Symlink target escapes" in e for e in result.errors))

    def test_symlink_absolute_target_rejected(self):
        tar_path = os.path.join(self.tmpdir, "symlink_abs.tar")
        with tarfile.open(tar_path, "w") as tf:
            info = tarfile.TarInfo(name="evil-link")
            info.type = tarfile.SYMTYPE
            info.linkname = "/etc/passwd"
            tf.addfile(info)
        result = safe_extract_archive(tar_path, self.dest)
        self.assertFalse(result.success)
        self.assertTrue(any("Symlink target is absolute" in e for e in result.errors))

    def test_hardlink_escape_rejected(self):
        tar_path = os.path.join(self.tmpdir, "hardlink.tar")
        with tarfile.open(tar_path, "w") as tf:
            info = tarfile.TarInfo(name="evil-hardlink")
            info.type = tarfile.LNKTYPE
            info.linkname = "../etc/passwd"
            tf.addfile(info)
        result = safe_extract_archive(tar_path, self.dest)
        self.assertFalse(result.success)
        self.assertTrue(any("Hardlink target escapes" in e for e in result.errors))

    def test_hardlink_absolute_target_rejected(self):
        tar_path = os.path.join(self.tmpdir, "hardlink_abs.tar")
        with tarfile.open(tar_path, "w") as tf:
            info = tarfile.TarInfo(name="evil-hardlink")
            info.type = tarfile.LNKTYPE
            info.linkname = "/etc/passwd"
            tf.addfile(info)
        result = safe_extract_archive(tar_path, self.dest)
        self.assertFalse(result.success)
        self.assertTrue(any("Hardlink target is absolute" in e for e in result.errors))

    def test_duplicate_normalized_path_rejected(self):
        tar_path = os.path.join(self.tmpdir, "dup.tar")
        with tarfile.open(tar_path, "w") as tf:
            import io
            info1 = tarfile.TarInfo(name="foo/bar.txt")
            info1.size = 4
            tf.addfile(info1, io.BytesIO(b"data"))
            info2 = tarfile.TarInfo(name="foo/./bar.txt")
            info2.size = 4
            tf.addfile(info2, io.BytesIO(b"data"))
        result = safe_extract_archive(tar_path, self.dest)
        self.assertFalse(result.success)
        self.assertTrue(any("Duplicate normalized path" in e for e in result.errors))

    def test_device_node_rejected(self):
        tar_path = os.path.join(self.tmpdir, "dev.tar")
        with tarfile.open(tar_path, "w") as tf:
            info = tarfile.TarInfo(name="evil-dev")
            info.type = tarfile.CHRTYPE
            tf.addfile(info)
        result = safe_extract_archive(tar_path, self.dest)
        self.assertFalse(result.success)
        self.assertTrue(any("Device node rejected" in e for e in result.errors))

    def test_block_device_rejected(self):
        tar_path = os.path.join(self.tmpdir, "block.tar")
        with tarfile.open(tar_path, "w") as tf:
            info = tarfile.TarInfo(name="evil-block")
            info.type = tarfile.BLKTYPE
            tf.addfile(info)
        result = safe_extract_archive(tar_path, self.dest)
        self.assertFalse(result.success)
        self.assertTrue(any("Device node rejected" in e for e in result.errors))

    def test_setuid_rejected(self):
        tar_path = os.path.join(self.tmpdir, "setuid.tar")
        with tarfile.open(tar_path, "w") as tf:
            info = tarfile.TarInfo(name="setuid-bin")
            info.size = 3
            info.mode = stat.S_ISUID | 0o755
            import io
            tf.addfile(info, io.BytesIO(b"bin"))
        result = safe_extract_archive(tar_path, self.dest)
        self.assertFalse(result.success)
        self.assertTrue(any("Setuid bit rejected" in e for e in result.errors))

    def test_setgid_rejected(self):
        tar_path = os.path.join(self.tmpdir, "setgid.tar")
        with tarfile.open(tar_path, "w") as tf:
            info = tarfile.TarInfo(name="setgid-bin")
            info.size = 3
            info.mode = stat.S_ISGID | 0o755
            import io
            tf.addfile(info, io.BytesIO(b"bin"))
        result = safe_extract_archive(tar_path, self.dest)
        self.assertFalse(result.success)
        self.assertTrue(any("Setgid bit rejected" in e for e in result.errors))

    def test_valid_extract_succeeds(self):
        tar_path = os.path.join(self.tmpdir, "valid.tar")
        with tarfile.open(tar_path, "w") as tf:
            info = tarfile.TarInfo(name="good.txt")
            info.size = 5
            import io
            tf.addfile(info, io.BytesIO(b"hello"))
        result = safe_extract_archive(tar_path, self.dest)
        self.assertTrue(result.success, f"Errors: {result.errors}")
        self.assertTrue(os.path.isfile(os.path.join(self.dest, "good.txt")))

    def test_safe_extract_raises_on_failure(self):
        tar_path = os.path.join(self.tmpdir, "bad.tar")
        with tarfile.open(tar_path, "w") as tf:
            info = tarfile.TarInfo(name="/etc/passwd")
            info.size = 4
            import io
            tf.addfile(info, io.BytesIO(b"root"))
        with self.assertRaises(OSError):
            safe_extract(tar_path, self.dest)


class TestSafeExtractZip(unittest.TestCase):
    def setUp(self):
        self.tmpdir = tempfile.mkdtemp()
        self.dest = os.path.join(self.tmpdir, "dest")

    def tearDown(self):
        shutil.rmtree(self.tmpdir, ignore_errors=True)

    def test_absolute_path_rejected(self):
        zip_path = os.path.join(self.tmpdir, "absolute.zip")
        with zipfile.ZipFile(zip_path, "w") as zf:
            zf.writestr("/etc/passwd", "root")
        result = safe_extract_archive(zip_path, self.dest)
        self.assertFalse(result.success)
        self.assertTrue(any("Absolute path rejected" in e for e in result.errors))

    def test_dotdot_escape_rejected(self):
        zip_path = os.path.join(self.tmpdir, "dotdot.zip")
        with zipfile.ZipFile(zip_path, "w") as zf:
            zf.writestr("../etc/passwd", "root")
        result = safe_extract_archive(zip_path, self.dest)
        self.assertFalse(result.success)
        self.assertTrue(
            any("Path traversal" in e or "escapes" in e for e in result.errors),
            f"Expected traversal/escapes error, got: {result.errors}",
        )

    def test_duplicate_normalized_path_rejected(self):
        zip_path = os.path.join(self.tmpdir, "dup.zip")
        with zipfile.ZipFile(zip_path, "w") as zf:
            zf.writestr("foo/bar.txt", "data1")
            zf.writestr("foo/./bar.txt", "data2")
        result = safe_extract_archive(zip_path, self.dest)
        self.assertFalse(result.success)
        self.assertTrue(any("Duplicate normalized path" in e for e in result.errors))

    def test_valid_extract_succeeds(self):
        zip_path = os.path.join(self.tmpdir, "valid.zip")
        with zipfile.ZipFile(zip_path, "w") as zf:
            zf.writestr("good.txt", "hello")
        result = safe_extract_archive(zip_path, self.dest)
        self.assertTrue(result.success, f"Errors: {result.errors}")
        self.assertTrue(os.path.isfile(os.path.join(self.dest, "good.txt")))


class TestNormalizeSafePath(unittest.TestCase):
    def test_backslash_normalized(self):
        result = _normalize_safe_path("foo\\bar.txt")
        if sys.platform == "win32":
            self.assertEqual(result, "foo\\bar.txt")
        else:
            self.assertEqual(result, "foo/bar.txt")

    def test_dotdot_returns_empty(self):
        result = _normalize_safe_path("../etc/passwd")
        if sys.platform == "win32":
            self.assertNotEqual(result, "")
        else:
            self.assertEqual(result, "")

    def test_absolute_path_stripped(self):
        result = _normalize_safe_path("/etc/passwd")
        self.assertFalse(result.startswith("/"))

    def test_dot_components_removed(self):
        result = _normalize_safe_path("foo/./bar.txt")
        if sys.platform == "win32":
            self.assertEqual(result, "foo\\bar.txt")
        else:
            self.assertEqual(result, "foo/bar.txt")

    def test_normal_path_unchanged(self):
        result = _normalize_safe_path("simple.txt")
        self.assertEqual(result, "simple.txt")

    def test_empty_input(self):
        result = _normalize_safe_path("")
        self.assertEqual(result, "")


if __name__ == "__main__":
    unittest.main()
