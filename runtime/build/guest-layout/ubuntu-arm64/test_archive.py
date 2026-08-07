import hashlib
import lzma
import os
import pathlib
import tarfile
import tempfile
import unittest

SCRIPT_DIR = pathlib.Path(__file__).resolve().parent

try:
    from build import create_deterministic_tar, sha256_file
    _HAS_BUILD = True
except ImportError:
    _HAS_BUILD = False


@unittest.skipUnless(_HAS_BUILD, "build import failed")
class TestDeterministicArchive(unittest.TestCase):
    def _make_root(self, tmpdir):
        root = pathlib.Path(tmpdir) / "r"
        root.mkdir()
        (root / "opt").mkdir()
        (root / "etc").mkdir()
        (root / "var").mkdir()
        (root / "run").mkdir()
        (root / "opt" / "amitia").mkdir(parents=True)
        (root / "opt" / "amitia" / "manifest").mkdir(parents=True)
        (root / "opt" / "amitia" / "manifest" / "test.json").write_text("{}\n", encoding="utf-8")
        (root / "opt" / "amitia" / "empty").mkdir()
        return root

    def test_same_input_same_sha(self):
        with tempfile.TemporaryDirectory() as tmpdir:
            root1 = self._make_root(tmpdir)
            archive1 = pathlib.Path(tmpdir) / "a1.tar.xz"
            create_deterministic_tar(root1, archive1)
            archive2 = pathlib.Path(tmpdir) / "a2.tar.xz"
            create_deterministic_tar(root1, archive2)
            self.assertEqual(sha256_file(archive1), sha256_file(archive2))

    def test_sorted_members(self):
        with tempfile.TemporaryDirectory() as tmpdir:
            root = pathlib.Path(tmpdir) / "r"
            root.mkdir()
            (root / "opt").mkdir()
            for name in ("zebra", "alpha", "mango"):
                (root / "opt" / name).mkdir()
            archive = pathlib.Path(tmpdir) / "a.tar.xz"
            create_deterministic_tar(root, archive)
            with lzma.open(archive, "rb") as xz:
                with tarfile.open(fileobj=xz, mode="r:") as tf:
                    names = [m.name for m in tf.getmembers()]
            self.assertEqual(names, sorted(names))

    def test_top_level_no_overlay_root(self):
        with tempfile.TemporaryDirectory() as tmpdir:
            root = self._make_root(tmpdir)
            archive = pathlib.Path(tmpdir) / "a.tar.xz"
            create_deterministic_tar(root, archive)
            with lzma.open(archive, "rb") as xz:
                with tarfile.open(fileobj=xz, mode="r:") as tf:
                    for m in tf.getmembers():
                        self.assertFalse(m.name.startswith("overlay/"), m.name)
                        self.assertFalse(m.name.startswith("guest-layout/"), m.name)
                        self.assertFalse(m.name.startswith("rootfs/"), m.name)
                        self.assertFalse(m.name.startswith("/"), m.name)

    def test_directory_entries_preserved(self):
        with tempfile.TemporaryDirectory() as tmpdir:
            root = self._make_root(tmpdir)
            archive = pathlib.Path(tmpdir) / "a.tar.xz"
            create_deterministic_tar(root, archive)
            with lzma.open(archive, "rb") as xz:
                with tarfile.open(fileobj=xz, mode="r:") as tf:
                    names = [m.name for m in tf.getmembers() if m.isdir()]
            self.assertIn("opt/amitia/empty", names)

    def test_no_device_node(self):
        with tempfile.TemporaryDirectory() as tmpdir:
            root = self._make_root(tmpdir)
            archive = pathlib.Path(tmpdir) / "a.tar.xz"
            create_deterministic_tar(root, archive)
            with lzma.open(archive, "rb") as xz:
                with tarfile.open(fileobj=xz, mode="r:") as tf:
                    for m in tf.getmembers():
                        self.assertFalse(m.isdev(), m.name)

    def test_mtime_zero(self):
        with tempfile.TemporaryDirectory() as tmpdir:
            root = self._make_root(tmpdir)
            archive = pathlib.Path(tmpdir) / "a.tar.xz"
            create_deterministic_tar(root, archive)
            with lzma.open(archive, "rb") as xz:
                with tarfile.open(fileobj=xz, mode="r:") as tf:
                    for m in tf.getmembers():
                        self.assertEqual(m.mtime, 0)

    def test_uid_gid_zero(self):
        with tempfile.TemporaryDirectory() as tmpdir:
            root = self._make_root(tmpdir)
            archive = pathlib.Path(tmpdir) / "a.tar.xz"
            create_deterministic_tar(root, archive)
            with lzma.open(archive, "rb") as xz:
                with tarfile.open(fileobj=xz, mode="r:") as tf:
                    for m in tf.getmembers():
                        self.assertEqual(m.uid, 0)
                        self.assertEqual(m.gid, 0)

    def test_json_file_mode(self):
        with tempfile.TemporaryDirectory() as tmpdir:
            root = self._make_root(tmpdir)
            archive = pathlib.Path(tmpdir) / "a.tar.xz"
            create_deterministic_tar(root, archive)
            with lzma.open(archive, "rb") as xz:
                with tarfile.open(fileobj=xz, mode="r:") as tf:
                    members = [m for m in tf.getmembers() if m.isfile()]
                    self.assertTrue(len(members) > 0)
                    for m in members:
                        if m.name.endswith(".json"):
                            self.assertEqual(m.mode, 0o644)

    def test_no_xattr(self):
        with tempfile.TemporaryDirectory() as tmpdir:
            root = self._make_root(tmpdir)
            archive = pathlib.Path(tmpdir) / "a.tar.xz"
            create_deterministic_tar(root, archive)
            with lzma.open(archive, "rb") as xz:
                with tarfile.open(fileobj=xz, mode="r:") as tf:
                    for m in tf.getmembers():
                        attrs = getattr(m, 'xattrs', [])
                        self.assertEqual(len(attrs), 0)

    def test_no_pax_headers(self):
        with tempfile.TemporaryDirectory() as tmpdir:
            root = self._make_root(tmpdir)
            archive = pathlib.Path(tmpdir) / "a.tar.xz"
            create_deterministic_tar(root, archive)
            with lzma.open(archive, "rb") as xz:
                buf = xz.read()
            self.assertNotIn(b"PAX", buf)


if __name__ == "__main__":
    unittest.main()
