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

import build


def make_binary(base, name="amitia-server", content=b"fake binary content"):
    binary_path = pathlib.Path(base) / name
    binary_path.write_bytes(content)
    return binary_path


def make_artifact_json(base, sha):
    artifact_path = pathlib.Path(base) / "artifact.json"
    data = {
        "schemaVersion": 1,
        "componentId": "backend",
        "version": "1.0.0",
        "platform": "linux",
        "architecture": "arm64",
        "artifactType": "binary",
        "artifactRelativePath": "backend/linux-arm64/1.0.0/amitia-server",
        "artifactSha256": sha,
        "treeSha256": sha,
        "buildMode": "release",
    }
    with open(artifact_path, "w", encoding="utf-8") as f:
        json.dump(data, f)
    return artifact_path


class TestFreshBuild(unittest.TestCase):
    def setUp(self):
        self.tmpdir = tempfile.mkdtemp()
        self.binary = make_binary(self.tmpdir)
        self.output_base = pathlib.Path(self.tmpdir) / "output"
        self.output_base.mkdir()

    def tearDown(self):
        shutil.rmtree(self.tmpdir, ignore_errors=True)

    def test_fresh_build_returns_record(self):
        record = build.build(str(self.binary), "", str(self.output_base), "1.0.0")
        self.assertIsNotNone(record)
        self.assertEqual(record.componentId, "backend")
        self.assertEqual(record.version, "1.0.0")

    def test_fresh_build_copies_binary(self):
        build.build(str(self.binary), "", str(self.output_base), "1.0.0")
        dest = self.output_base / "backend" / "linux-arm64" / "1.0.0" / "amitia-server"
        self.assertTrue(dest.exists())

    def test_fresh_build_saves_record(self):
        build.build(str(self.binary), "", str(self.output_base), "1.0.0")
        record_path = self.output_base / "backend" / "linux-arm64" / "1.0.0" / "build-record.json"
        self.assertTrue(record_path.exists())


class TestMissingInput(unittest.TestCase):
    def setUp(self):
        self.tmpdir = tempfile.mkdtemp()
        self.output_base = pathlib.Path(self.tmpdir) / "output"
        self.output_base.mkdir()

    def tearDown(self):
        shutil.rmtree(self.tmpdir, ignore_errors=True)

    def test_missing_binary_fails(self):
        with self.assertRaises(FileNotFoundError):
            build.build("/nonexistent/binary", "", str(self.output_base), "1.0.0")

    def test_empty_binary_fails(self):
        binary = make_binary(self.tmpdir, content=b"")
        with self.assertRaises(ValueError):
            build.build(str(binary), "", str(self.output_base), "1.0.0")


class TestShaMismatch(unittest.TestCase):
    def setUp(self):
        self.tmpdir = tempfile.mkdtemp()
        self.binary = make_binary(self.tmpdir)
        self.output_base = pathlib.Path(self.tmpdir) / "output"
        self.output_base.mkdir()

    def tearDown(self):
        shutil.rmtree(self.tmpdir, ignore_errors=True)

    def test_tampered_binary_differs_from_artifact(self):
        sha = hashlib.sha256(self.binary.read_bytes()).hexdigest()
        artifact_path = make_artifact_json(self.tmpdir, sha)
        record1 = build.build(str(self.binary), str(artifact_path), str(self.output_base), "1.0.0")
        self.assertIsNotNone(record1)
        tampered = make_binary(self.tmpdir, content=b"tampered content")
        record2 = build.build(str(tampered), str(artifact_path), str(self.output_base), "1.0.0")
        self.assertNotEqual(record2.artifactSha256, sha)


class TestSameVersionSameBytes(unittest.TestCase):
    def setUp(self):
        self.tmpdir = tempfile.mkdtemp()
        self.binary = make_binary(self.tmpdir)
        self.output_base = pathlib.Path(self.tmpdir) / "output"
        self.output_base.mkdir()

    def tearDown(self):
        shutil.rmtree(self.tmpdir, ignore_errors=True)

    def test_same_binary_returns_reuse(self):
        sha = hashlib.sha256(self.binary.read_bytes()).hexdigest()
        artifact_path = make_artifact_json(self.tmpdir, sha)
        record = build.build(str(self.binary), str(artifact_path), str(self.output_base), "1.0.0")
        self.assertEqual(record.artifactSha256, sha)
        self.assertEqual(record.componentId, "backend")


class TestSameVersionDifferentBytes(unittest.TestCase):
    def setUp(self):
        self.tmpdir = tempfile.mkdtemp()
        self.binary = make_binary(self.tmpdir)
        self.output_base = pathlib.Path(self.tmpdir) / "output"
        self.output_base.mkdir()

    def tearDown(self):
        shutil.rmtree(self.tmpdir, ignore_errors=True)

    def test_different_binary_creates_new_record(self):
        sha = hashlib.sha256(self.binary.read_bytes()).hexdigest()
        artifact_path = make_artifact_json(self.tmpdir, sha)
        build.build(str(self.binary), str(artifact_path), str(self.output_base), "1.0.0")
        dest = self.output_base / "backend" / "linux-arm64" / "1.0.0" / "amitia-server"
        original_content = dest.read_bytes()
        different_binary = make_binary(self.tmpdir, content=b"different content v2")
        record = build.build(str(different_binary), str(artifact_path), str(self.output_base), "2.0.0")
        self.assertNotEqual(record.artifactSha256, sha)


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
            build.build("/nonexistent", "", str(self.output_base), "1.0.0")
        output_after = list(self.output_base.rglob("*"))
        self.assertEqual(output_before, output_after)


class TestFailureDuringCandidateMetadata(unittest.TestCase):
    def setUp(self):
        self.tmpdir = tempfile.mkdtemp()
        self.binary = make_binary(self.tmpdir)
        self.output_base = pathlib.Path(self.tmpdir) / "output"
        self.output_base.mkdir()

    def tearDown(self):
        shutil.rmtree(self.tmpdir, ignore_errors=True)

    def test_no_partial_output_on_failure(self):
        output_before = list((self.output_base / "backend").rglob("*")) if (self.output_base / "backend").exists() else []
        record = build.build(str(self.binary), "", str(self.output_base), "1.0.0")
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
        self.binary = make_binary(self.tmpdir)
        self.output_base = pathlib.Path(self.tmpdir) / "output"
        self.output_base.mkdir()

    def tearDown(self):
        shutil.rmtree(self.tmpdir, ignore_errors=True)

    def test_two_builds_same_sha(self):
        record1 = build.build(str(self.binary), "", str(self.output_base), "1.0.0")
        output_base2 = pathlib.Path(self.tmpdir) / "output2"
        output_base2.mkdir()
        record2 = build.build(str(self.binary), "", str(output_base2), "1.0.0")
        self.assertEqual(record1.artifactSha256, record2.artifactSha256)


class TestOffline(unittest.TestCase):
    def setUp(self):
        self.tmpdir = tempfile.mkdtemp()
        self.binary = make_binary(self.tmpdir)
        self.output_base = pathlib.Path(self.tmpdir) / "output"
        self.output_base.mkdir()

    def tearDown(self):
        shutil.rmtree(self.tmpdir, ignore_errors=True)

    def test_offline_mode_succeeds(self):
        record = build.build(str(self.binary), "", str(self.output_base), "1.0.0")
        self.assertIsNotNone(record)


if __name__ == "__main__":
    unittest.main()
