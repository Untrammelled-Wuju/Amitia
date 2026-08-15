import hashlib
import io
import json
import os
import pathlib
import shutil
import struct
import sys
import tarfile
import tempfile
import unittest
from unittest import mock

SCRIPT_DIR = pathlib.Path(__file__).resolve().parent
sys.path.insert(0, str(SCRIPT_DIR))

import build


def make_node_archive(base, root_name="node-v24.19.0-linux-arm64"):
    buf = io.BytesIO()
    with tarfile.open(fileobj=buf, mode="w") as tf:
        def add_dir(name):
            info = tarfile.TarInfo(name=name + "/")
            info.type = tarfile.DIRTYPE
            info.mode = 0o755
            info.uid = 0
            info.gid = 0
            info.mtime = 0
            tf.addfile(info)

        def add_file(name, content):
            info = tarfile.TarInfo(name=name)
            info.type = tarfile.REGTYPE
            info.mode = 0o644
            info.uid = 0
            info.gid = 0
            info.mtime = 0
            info.size = len(content)
            tf.addfile(info, io.BytesIO(content))

        def add_exec(name, content):
            info = tarfile.TarInfo(name=name)
            info.type = tarfile.REGTYPE
            info.mode = 0o755
            info.uid = 0
            info.gid = 0
            info.mtime = 0
            info.size = len(content)
            tf.addfile(info, io.BytesIO(content))

        add_dir(root_name)
        add_dir(root_name + "/bin")
        elf_magic = b"\x7fELF\x02\x01\x01"
        elf_rest = b"\x00" * 14
        elf_machine = struct.pack("<H", 183)
        elf_content = elf_magic + elf_rest + elf_machine + b"\x00" * 40
        add_exec(root_name + "/bin/node", elf_content)
    data = buf.getvalue()
    archive_path = pathlib.Path(base) / "node.tar.xz"
    archive_path.write_bytes(data)
    sha = hashlib.sha256(data).hexdigest()
    return archive_path, sha


def make_lock(base, sha):
    lock_path = pathlib.Path(base) / "node.lock.json"
    lock_data = {
        "schemaVersion": 1,
        "sourceSha256": sha,
    }
    with open(lock_path, "w", encoding="utf-8") as f:
        json.dump(lock_data, f)
    return lock_path


class TestFreshBuild(unittest.TestCase):
    def setUp(self):
        self.tmpdir = tempfile.mkdtemp()
        self.archive, self.sha = make_node_archive(self.tmpdir)
        self.output_base = pathlib.Path(self.tmpdir) / "output"
        self.output_base.mkdir()

    def tearDown(self):
        shutil.rmtree(self.tmpdir, ignore_errors=True)

    def test_fresh_build_returns_record(self):
        lock_path = make_lock(self.tmpdir, self.sha)
        with mock.patch("build.sha256_tree_manifest", return_value="a" * 64):
            with mock.patch("build.atomic_publish_directory") as mock_pub:
                mock_pub.return_value = mock.Mock(
                    success=True,
                    published_dir=str(self.output_base / "node" / "linux-arm64" / "1.0.0"),
                    errors=[],
                )
                record = build.build(str(self.archive), str(self.output_base), "1.0.0", str(lock_path))
        self.assertIsNotNone(record)
        self.assertEqual(record.componentId, "node")
        self.assertEqual(record.version, "1.0.0")

    def test_fresh_build_saves_record(self):
        lock_path = make_lock(self.tmpdir, self.sha)
        with mock.patch("build.sha256_tree_manifest", return_value="a" * 64):
            with mock.patch("build.atomic_publish_directory") as mock_pub:
                mock_pub.return_value = mock.Mock(
                    success=True,
                    published_dir=str(self.output_base / "node" / "linux-arm64" / "1.0.0"),
                    errors=[],
                )
                build.build(str(self.archive), str(self.output_base), "1.0.0", str(lock_path))
        record_path = self.output_base / "node" / "linux-arm64" / "1.0.0" / "build-record.json"
        self.assertTrue(record_path.exists())


class TestMissingInput(unittest.TestCase):
    def setUp(self):
        self.tmpdir = tempfile.mkdtemp()
        self.output_base = pathlib.Path(self.tmpdir) / "output"
        self.output_base.mkdir()

    def tearDown(self):
        shutil.rmtree(self.tmpdir, ignore_errors=True)

    def test_missing_archive_fails(self):
        lock_path = make_lock(self.tmpdir, "a" * 64)
        with self.assertRaises(Exception):
            build.build("/nonexistent.tar.xz", str(self.output_base), "1.0.0", str(lock_path))

    def test_missing_lock_fails(self):
        archive, sha = make_node_archive(self.tmpdir)
        with self.assertRaises(Exception):
            build.build(str(archive), str(self.output_base), "1.0.0", "/nonexistent.lock.json")


class TestShaMismatch(unittest.TestCase):
    def setUp(self):
        self.tmpdir = tempfile.mkdtemp()
        self.archive, self.sha = make_node_archive(self.tmpdir)
        self.output_base = pathlib.Path(self.tmpdir) / "output"
        self.output_base.mkdir()

    def tearDown(self):
        shutil.rmtree(self.tmpdir, ignore_errors=True)

    def test_sha_mismatch_fails(self):
        lock_path = make_lock(self.tmpdir, "f" * 64)
        with self.assertRaises(ValueError):
            build.build(str(self.archive), str(self.output_base), "1.0.0", str(lock_path))


class TestSameVersionSameBytes(unittest.TestCase):
    def setUp(self):
        self.tmpdir = tempfile.mkdtemp()
        self.archive, self.sha = make_node_archive(self.tmpdir)
        self.output_base = pathlib.Path(self.tmpdir) / "output"
        self.output_base.mkdir()

    def tearDown(self):
        shutil.rmtree(self.tmpdir, ignore_errors=True)

    def test_same_archive_same_sha_succeeds(self):
        lock_path = make_lock(self.tmpdir, self.sha)
        with mock.patch("build.sha256_tree_manifest", return_value="a" * 64):
            with mock.patch("build.atomic_publish_directory") as mock_pub:
                mock_pub.return_value = mock.Mock(
                    success=True,
                    published_dir=str(self.output_base / "node" / "linux-arm64" / "1.0.0"),
                    errors=[],
                )
                record = build.build(str(self.archive), str(self.output_base), "1.0.0", str(lock_path))
        self.assertEqual(record.componentId, "node")


class TestSameVersionDifferentBytes(unittest.TestCase):
    def setUp(self):
        self.tmpdir = tempfile.mkdtemp()
        self.archive, self.sha = make_node_archive(self.tmpdir)
        self.output_base = pathlib.Path(self.tmpdir) / "output"
        self.output_base.mkdir()

    def tearDown(self):
        shutil.rmtree(self.tmpdir, ignore_errors=True)

    def test_different_archive_sha_fails(self):
        lock_path = make_lock(self.tmpdir, self.sha)
        other_archive, other_sha = make_node_archive(self.tmpdir, root_name="node-v24.20.0-linux-arm64")
        with self.assertRaises(ValueError):
            build.build(str(other_archive), str(self.output_base), "1.0.0", str(lock_path))


class TestFailureBeforePublish(unittest.TestCase):
    def setUp(self):
        self.tmpdir = tempfile.mkdtemp()
        self.output_base = pathlib.Path(self.tmpdir) / "output"
        self.output_base.mkdir()

    def tearDown(self):
        shutil.rmtree(self.tmpdir, ignore_errors=True)

    def test_failure_before_publish_leaves_output_intact(self):
        output_before = list(self.output_base.rglob("*"))
        with self.assertRaises(Exception):
            build.build("/nonexistent", str(self.output_base), "1.0.0", "/nonexistent.lock.json")
        output_after = list(self.output_base.rglob("*"))
        self.assertEqual(output_before, output_after)


class TestFailureDuringCandidateMetadata(unittest.TestCase):
    def setUp(self):
        self.tmpdir = tempfile.mkdtemp()
        self.archive, self.sha = make_node_archive(self.tmpdir)
        self.output_base = pathlib.Path(self.tmpdir) / "output"
        self.output_base.mkdir()

    def tearDown(self):
        shutil.rmtree(self.tmpdir, ignore_errors=True)

    def test_publish_failure_leaves_no_partial(self):
        lock_path = make_lock(self.tmpdir, self.sha)
        with mock.patch("build.sha256_tree_manifest", return_value="a" * 64):
            with mock.patch("build.atomic_publish_directory") as mock_pub:
                mock_pub.return_value = mock.Mock(
                    success=False, published_dir="", errors=["disk full"]
                )
                with self.assertRaises(RuntimeError):
                    build.build(str(self.archive), str(self.output_base), "1.0.0", str(lock_path))
        pub_dir = self.output_base / "node" / "linux-arm64" / "1.0.0"
        self.assertFalse(pub_dir.exists())


class TestUnsafeArchive(unittest.TestCase):
    def setUp(self):
        self.tmpdir = tempfile.mkdtemp()

    def tearDown(self):
        shutil.rmtree(self.tmpdir, ignore_errors=True)

    def test_absolute_path_in_archive_rejected(self):
        buf = io.BytesIO()
        with tarfile.open(fileobj=buf, mode="w") as tf:
            info = tarfile.TarInfo(name="/etc/passwd")
            info.size = 4
            tf.addfile(info, io.BytesIO(b"root"))
        archive_path = pathlib.Path(self.tmpdir) / "unsafe.tar"
        archive_path.write_bytes(buf.getvalue())
        from common import safe_extract_archive
        result = safe_extract_archive(str(archive_path), self.tmpdir)
        self.assertFalse(result.success)

    def test_dotdot_in_archive_rejected(self):
        buf = io.BytesIO()
        with tarfile.open(fileobj=buf, mode="w") as tf:
            info = tarfile.TarInfo(name="../escape")
            info.size = 4
            tf.addfile(info, io.BytesIO(b"root"))
        archive_path = pathlib.Path(self.tmpdir) / "unsafe.tar"
        archive_path.write_bytes(buf.getvalue())
        from common import safe_extract_archive
        result = safe_extract_archive(str(archive_path), self.tmpdir)
        self.assertFalse(result.success)

    def test_absolute_symlink_rejected(self):
        buf = io.BytesIO()
        with tarfile.open(fileobj=buf, mode="w") as tf:
            info = tarfile.TarInfo(name="link")
            info.type = tarfile.SYMTYPE
            info.linkname = "/etc/passwd"
            tf.addfile(info)
        archive_path = pathlib.Path(self.tmpdir) / "unsafe.tar"
        archive_path.write_bytes(buf.getvalue())
        from common import safe_extract_archive
        result = safe_extract_archive(str(archive_path), self.tmpdir)
        self.assertFalse(result.success)

    def test_symlink_escape_rejected(self):
        buf = io.BytesIO()
        with tarfile.open(fileobj=buf, mode="w") as tf:
            d = tarfile.TarInfo(name="dir/")
            d.type = tarfile.DIRTYPE
            d.mode = 0o755
            tf.addfile(d)
            info = tarfile.TarInfo(name="dir/escape")
            info.type = tarfile.SYMTYPE
            info.linkname = "../../etc/passwd"
            tf.addfile(info)
        archive_path = pathlib.Path(self.tmpdir) / "unsafe.tar"
        archive_path.write_bytes(buf.getvalue())
        from common import safe_extract_archive
        result = safe_extract_archive(str(archive_path), self.tmpdir)
        self.assertFalse(result.success)

    def test_hardlink_escape_rejected(self):
        buf = io.BytesIO()
        with tarfile.open(fileobj=buf, mode="w") as tf:
            d = tarfile.TarInfo(name="dir/")
            d.type = tarfile.DIRTYPE
            d.mode = 0o755
            tf.addfile(d)
            info = tarfile.TarInfo(name="dir/hl")
            info.type = tarfile.LNKTYPE
            info.linkname = "/etc/passwd"
            tf.addfile(info)
        archive_path = pathlib.Path(self.tmpdir) / "unsafe.tar"
        archive_path.write_bytes(buf.getvalue())
        from common import safe_extract_archive
        result = safe_extract_archive(str(archive_path), self.tmpdir)
        self.assertFalse(result.success)

    def test_duplicate_normalized_path_rejected(self):
        buf = io.BytesIO()
        with tarfile.open(fileobj=buf, mode="w") as tf:
            info1 = tarfile.TarInfo(name="a/b")
            info1.size = 2
            tf.addfile(info1, io.BytesIO(b"x1"))
            info2 = tarfile.TarInfo(name="a//b")
            info2.size = 2
            tf.addfile(info2, io.BytesIO(b"x2"))
        archive_path = pathlib.Path(self.tmpdir) / "unsafe.tar"
        archive_path.write_bytes(buf.getvalue())
        from common import safe_extract_archive
        result = safe_extract_archive(str(archive_path), self.tmpdir)
        self.assertFalse(result.success)


class TestReproducibility(unittest.TestCase):
    def setUp(self):
        self.tmpdir = tempfile.mkdtemp()
        self.archive, self.sha = make_node_archive(self.tmpdir)
        self.output_base = pathlib.Path(self.tmpdir) / "output"
        self.output_base.mkdir()

    def tearDown(self):
        shutil.rmtree(self.tmpdir, ignore_errors=True)

    def test_two_builds_same_tree_sha(self):
        lock_path = make_lock(self.tmpdir, self.sha)
        with mock.patch("build.sha256_tree_manifest", return_value="a" * 64):
            with mock.patch("build.atomic_publish_directory") as mock_pub:
                mock_pub.return_value = mock.Mock(
                    success=True,
                    published_dir=str(self.output_base / "node" / "linux-arm64" / "1.0.0"),
                    errors=[],
                )
                record1 = build.build(str(self.archive), str(self.output_base), "1.0.0", str(lock_path))
        output_base2 = pathlib.Path(self.tmpdir) / "output2"
        output_base2.mkdir()
        with mock.patch("build.sha256_tree_manifest", return_value="a" * 64):
            with mock.patch("build.atomic_publish_directory") as mock_pub:
                mock_pub.return_value = mock.Mock(
                    success=True,
                    published_dir=str(output_base2 / "node" / "linux-arm64" / "1.0.0"),
                    errors=[],
                )
                record2 = build.build(str(self.archive), str(output_base2), "1.0.0", str(lock_path))
        self.assertEqual(record1.treeSha256, record2.treeSha256)


class TestOffline(unittest.TestCase):
    def setUp(self):
        self.tmpdir = tempfile.mkdtemp()
        self.archive, self.sha = make_node_archive(self.tmpdir)
        self.output_base = pathlib.Path(self.tmpdir) / "output"
        self.output_base.mkdir()

    def tearDown(self):
        shutil.rmtree(self.tmpdir, ignore_errors=True)

    def test_offline_mode_succeeds_with_matching_lock(self):
        lock_path = make_lock(self.tmpdir, self.sha)
        with mock.patch("build.sha256_tree_manifest", return_value="a" * 64):
            with mock.patch("build.atomic_publish_directory") as mock_pub:
                mock_pub.return_value = mock.Mock(
                    success=True,
                    published_dir=str(self.output_base / "node" / "linux-arm64" / "1.0.0"),
                    errors=[],
                )
                record = build.build(str(self.archive), str(self.output_base), "1.0.0", str(lock_path))
        self.assertIsNotNone(record)


if __name__ == "__main__":
    unittest.main()
