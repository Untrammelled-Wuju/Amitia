import hashlib
import io
import lzma
import os
import pathlib
import shutil
import tarfile
import tempfile
import unittest

import archive


class TestValidateMemberPath(unittest.TestCase):
    def test_valid_path(self):
        archive.validate_member_path("backend/amitia-server")

    def test_valid_nested_path(self):
        archive.validate_member_path("manifest/build-inputs.json")

    def test_absolute_path_rejected(self):
        with self.assertRaises(archive.ArchiveError):
            archive.validate_member_path("/etc/passwd")

    def test_path_traversal_rejected(self):
        with self.assertRaises(archive.ArchiveError):
            archive.validate_member_path("../escape")

    def test_path_traversal_in_middle_rejected(self):
        with self.assertRaises(archive.ArchiveError):
            archive.validate_member_path("foo/../../etc/passwd")

    def test_empty_path_rejected(self):
        with self.assertRaises(archive.ArchiveError):
            archive.validate_member_path("")


class TestSha256File(unittest.TestCase):
    def setUp(self):
        self.tmp = tempfile.mkdtemp()

    def tearDown(self):
        shutil.rmtree(self.tmp, ignore_errors=True)

    def test_known_sha(self):
        p = pathlib.Path(self.tmp) / "test.txt"
        content = b"hello world"
        p.write_bytes(content)
        expected = hashlib.sha256(content).hexdigest()
        self.assertEqual(archive.sha256_file(str(p)), expected)

    def test_empty_file(self):
        p = pathlib.Path(self.tmp) / "empty"
        p.write_bytes(b"")
        expected = hashlib.sha256(b"").hexdigest()
        self.assertEqual(archive.sha256_file(str(p)), expected)


class TestSha256Bytes(unittest.TestCase):
    def test_known_value(self):
        self.assertEqual(
            archive.sha256_bytes(b"test"),
            hashlib.sha256(b"test").hexdigest()
        )


class TestCreateArchive(unittest.TestCase):
    def setUp(self):
        self.tmp = tempfile.mkdtemp()

    def tearDown(self):
        shutil.rmtree(self.tmp, ignore_errors=True)

    def _create_structure(self):
        dist = pathlib.Path(self.tmp) / "dist"
        bin_dir = dist / "backend"
        bin_dir.mkdir(parents=True)
        (bin_dir / "amitia-server").write_bytes(b"X" * 100)
        meta_dir = dist / "manifest"
        meta_dir.mkdir(parents=True)
        (meta_dir / "test.json").write_bytes(b"{}")
        return dist

    def test_create_and_verify(self):
        dist = self._create_structure()
        out = pathlib.Path(self.tmp) / "out.tar.xz"
        sha = archive.create_archive(str(dist), str(out))
        self.assertEqual(sha, hashlib.sha256(out.read_bytes()).hexdigest())
        self.assertTrue(out.exists())

    def test_fixed_mtime(self):
        dist = self._create_structure()
        out = pathlib.Path(self.tmp) / "out.tar.xz"
        archive.create_archive(str(dist), str(out))
        with lzma.open(str(out), "rb") as xz:
            with tarfile.open(fileobj=xz, mode="r") as tf:
                for m in tf.getmembers():
                    self.assertEqual(m.mtime, 0)
                    self.assertEqual(m.uid, 0)
                    self.assertEqual(m.gid, 0)
                    self.assertEqual(m.uname, "")
                    self.assertEqual(m.gname, "")

    def test_sorted_members(self):
        dist = self._create_structure()
        out = pathlib.Path(self.tmp) / "out.tar.xz"
        archive.create_archive(str(dist), str(out))
        with lzma.open(str(out), "rb") as xz:
            with tarfile.open(fileobj=xz, mode="r") as tf:
                names = [m.name for m in tf.getmembers()]
                sorted_names = sorted(names, key=lambda x: tuple(p.lower() for p in pathlib.PurePosixPath(x).parts))
                self.assertEqual(names, sorted_names)

    def test_deterministic(self):
        dist1 = pathlib.Path(self.tmp) / "dist1"
        dist2 = pathlib.Path(self.tmp) / "dist2"
        for dist in [dist1, dist2]:
            bin_dir = dist / "backend"
            bin_dir.mkdir(parents=True)
            (bin_dir / "amitia-server").write_bytes(b"X" * 100)
            meta_dir = dist / "manifest"
            meta_dir.mkdir(parents=True)
            (meta_dir / "test.json").write_bytes(b"{}")
        out1 = pathlib.Path(self.tmp) / "1.tar.xz"
        out2 = pathlib.Path(self.tmp) / "2.tar.xz"
        archive.create_archive(str(dist1), str(out1))
        archive.create_archive(str(dist2), str(out2))
        sha1 = hashlib.sha256(out1.read_bytes()).hexdigest()
        sha2 = hashlib.sha256(out2.read_bytes()).hexdigest()
        self.assertEqual(sha1, sha2)

    def test_archive_preserves_all_files(self):
        dist = self._create_structure()
        out = pathlib.Path(self.tmp) / "out.tar.xz"
        archive.create_archive(str(dist), str(out))
        with lzma.open(str(out), "rb") as xz:
            with tarfile.open(fileobj=xz, mode="r") as tf:
                names = [m.name for m in tf.getmembers()]
                self.assertTrue(len(names) > 0)
                for n in names:
                    self.assertFalse(n.startswith("/"))
                    self.assertNotIn("..", pathlib.PurePosixPath(n).parts)

    def test_verify_archive_pass(self):
        dist = self._create_structure()
        out = pathlib.Path(self.tmp) / "out.tar.xz"
        archive.create_archive(str(dist), str(out))
        issues, errors = archive.verify_archive(str(out))
        self.assertEqual(len(errors), 0)
        self.assertEqual(len(issues), 0)

    def test_verify_archive_missing(self):
        issues, errors = archive.verify_archive("/nonexistent/path.tar.xz")
        self.assertTrue(len(errors) > 0 or len(issues) > 0)

    def test_binary_permission(self):
        dist = self._create_structure()
        out = pathlib.Path(self.tmp) / "out.tar.xz"
        archive.create_archive(str(dist), str(out))
        with lzma.open(str(out), "rb") as xz:
            with tarfile.open(fileobj=xz, mode="r") as tf:
                for m in tf.getmembers():
                    if m.name.endswith("amitia-server"):
                        self.assertEqual(m.mode, 0o755)
                    elif not m.isdir():
                        self.assertEqual(m.mode, 0o644)


class TestComputeSha256sums(unittest.TestCase):
    def setUp(self):
        self.tmp = tempfile.mkdtemp()

    def tearDown(self):
        shutil.rmtree(self.tmp, ignore_errors=True)

    def test_sums_order_and_format(self):
        root = pathlib.Path(self.tmp)
        for name in ["a.txt", "b.txt", "c.txt"]:
            (root / name).write_bytes(b"data")
        files = ["a.txt", "b.txt", "c.txt"]
        content = archive.compute_sha256sums(str(root), files)
        lines = content.strip().split("\n")
        self.assertEqual(len(lines), 3)
        for line in lines:
            self.assertRegex(line, r'^[0-9a-f]{64}  .+$')
        names = [line.split("  ", 1)[1] for line in lines]
        self.assertEqual(names, sorted(names))


if __name__ == "__main__":
    unittest.main()
