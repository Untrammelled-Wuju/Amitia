import hashlib
import io
import json
import os
import pathlib
import shutil
import sys
import tarfile
import tempfile
import unittest
from unittest import mock

SCRIPT_DIR = pathlib.Path(__file__).resolve().parent
sys.path.insert(0, str(SCRIPT_DIR))
BUILD_ROOT = SCRIPT_DIR.parents[1]
if str(BUILD_ROOT) not in sys.path:
    sys.path.insert(0, str(BUILD_ROOT))

import importlib.util
BuildSpec = importlib.util.spec_from_file_location("rootfs_linux_arm64_build", str(SCRIPT_DIR / "build.py"))
BuildModule = importlib.util.module_from_spec(BuildSpec)
sys.modules["rootfs_linux_arm64_build"] = BuildModule
BuildSpec.loader.exec_module(BuildModule)
def make_archive(base, name="rootfs.tar.xz", content=b"fake rootfs content"):
    archive_path = pathlib.Path(base) / name
    archive_path.write_bytes(content)
    return archive_path


def make_lock(base, sha):
    lock_path = pathlib.Path(base) / "rootfs.lock.json"
    lock_data = {"schemaVersion": 1, "rootfsSha256": sha}
    with open(lock_path, "w", encoding="utf-8") as f:
        json.dump(lock_data, f)
    return lock_path


class TestFreshBuild(unittest.TestCase):
    def setUp(self):
        self.tmpdir = tempfile.mkdtemp()
        self.archive = make_archive(self.tmpdir)
        self.sha = hashlib.sha256(self.archive.read_bytes()).hexdigest()
        self.output_base = pathlib.Path(self.tmpdir) / "output"
        self.output_base.mkdir()

    def tearDown(self):
        shutil.rmtree(self.tmpdir, ignore_errors=True)

    def test_fresh_build_returns_record(self):
        lock_path = make_lock(self.tmpdir, self.sha)
        record = BuildModule.build(str(self.archive), str(self.output_base), "1.0.0", str(lock_path))
        self.assertIsNotNone(record)
        self.assertEqual(record.componentId, "rootfs")
        self.assertEqual(record.version, "1.0.0")

    def test_fresh_build_copies_archive(self):
        lock_path = make_lock(self.tmpdir, self.sha)
        BuildModule.build(str(self.archive), str(self.output_base), "1.0.0", str(lock_path))
        dest = self.output_base / "rootfs" / "linux-arm64" / "1.0.0" / "rootfs.tar.xz"
        self.assertTrue(dest.exists())

    def test_fresh_build_saves_record(self):
        lock_path = make_lock(self.tmpdir, self.sha)
        BuildModule.build(str(self.archive), str(self.output_base), "1.0.0", str(lock_path))
        record_path = self.output_base / "rootfs" / "linux-arm64" / "1.0.0" / "build-record.json"
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
            BuildModule.build("/nonexistent/archive.tar.xz", str(self.output_base), "1.0.0", str(lock_path))


class TestShaMismatch(unittest.TestCase):
    def setUp(self):
        self.tmpdir = tempfile.mkdtemp()
        self.archive = make_archive(self.tmpdir)
        self.output_base = pathlib.Path(self.tmpdir) / "output"
        self.output_base.mkdir()

    def tearDown(self):
        shutil.rmtree(self.tmpdir, ignore_errors=True)

    def test_sha_mismatch_fails(self):
        lock_path = make_lock(self.tmpdir, "f" * 64)
        with self.assertRaises(ValueError):
            BuildModule.build(str(self.archive), str(self.output_base), "1.0.0", str(lock_path))


class TestSameVersionSameBytes(unittest.TestCase):
    def setUp(self):
        self.tmpdir = tempfile.mkdtemp()
        self.archive = make_archive(self.tmpdir)
        self.sha = hashlib.sha256(self.archive.read_bytes()).hexdigest()
        self.output_base = pathlib.Path(self.tmpdir) / "output"
        self.output_base.mkdir()

    def tearDown(self):
        shutil.rmtree(self.tmpdir, ignore_errors=True)

    def test_same_version_same_bytes_produces_consistent_record(self):
        lock_path = make_lock(self.tmpdir, self.sha)
        record1 = BuildModule.build(str(self.archive), str(self.output_base), "1.0.0", str(lock_path))
        record2 = BuildModule.build(str(self.archive), str(self.output_base), "1.0.0", str(lock_path))
        self.assertEqual(record1.artifactSha256, record2.artifactSha256)


class TestSameVersionDifferentBytes(unittest.TestCase):
    def setUp(self):
        self.tmpdir = tempfile.mkdtemp()
        self.archive = make_archive(self.tmpdir, content=b"version1")
        self.sha = hashlib.sha256(self.archive.read_bytes()).hexdigest()
        self.output_base = pathlib.Path(self.tmpdir) / "output"
        self.output_base.mkdir()

    def tearDown(self):
        shutil.rmtree(self.tmpdir, ignore_errors=True)

    def test_different_archive_fails_sha_check(self):
        lock_path = make_lock(self.tmpdir, self.sha)
        different_archive = make_archive(self.tmpdir, name="other.tar.xz", content=b"version2")
        with self.assertRaises(ValueError):
            BuildModule.build(str(different_archive), str(self.output_base), "1.0.0", str(lock_path))


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
            BuildModule.build("/nonexistent", str(self.output_base), "1.0.0", "")
        output_after = list(self.output_base.rglob("*"))
        self.assertEqual(output_before, output_after)


class TestFailureDuringCandidateMetadata(unittest.TestCase):
    def setUp(self):
        self.tmpdir = tempfile.mkdtemp()
        self.archive = make_archive(self.tmpdir)
        self.output_base = pathlib.Path(self.tmpdir) / "output"
        self.output_base.mkdir()

    def tearDown(self):
        shutil.rmtree(self.tmpdir, ignore_errors=True)

    def test_no_lock_skips_sha_check(self):
        record = BuildModule.build(str(self.archive), str(self.output_base), "1.0.0", "")
        self.assertIsNotNone(record)


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
        self.archive = make_archive(self.tmpdir)
        self.sha = hashlib.sha256(self.archive.read_bytes()).hexdigest()
        self.output_base = pathlib.Path(self.tmpdir) / "output"
        self.output_base.mkdir()

    def tearDown(self):
        shutil.rmtree(self.tmpdir, ignore_errors=True)

    def test_two_builds_same_sha(self):
        lock_path = make_lock(self.tmpdir, self.sha)
        record1 = BuildModule.build(str(self.archive), str(self.output_base), "1.0.0", str(lock_path))
        output_base2 = pathlib.Path(self.tmpdir) / "output2"
        output_base2.mkdir()
        record2 = BuildModule.build(str(self.archive), str(output_base2), "1.0.0", str(lock_path))
        self.assertEqual(record1.artifactSha256, record2.artifactSha256)


class TestOffline(unittest.TestCase):
    def setUp(self):
        self.tmpdir = tempfile.mkdtemp()
        self.archive = make_archive(self.tmpdir)
        self.sha = hashlib.sha256(self.archive.read_bytes()).hexdigest()
        self.output_base = pathlib.Path(self.tmpdir) / "output"
        self.output_base.mkdir()

    def tearDown(self):
        shutil.rmtree(self.tmpdir, ignore_errors=True)

    def test_offline_mode_succeeds_with_lock(self):
        lock_path = make_lock(self.tmpdir, self.sha)
        record = BuildModule.build(str(self.archive), str(self.output_base), "1.0.0", str(lock_path))
        self.assertIsNotNone(record)

    def test_offline_mode_succeeds_without_lock(self):
        record = BuildModule.build(str(self.archive), str(self.output_base), "1.0.0", "")
        self.assertIsNotNone(record)


if __name__ == "__main__":
    unittest.main()
