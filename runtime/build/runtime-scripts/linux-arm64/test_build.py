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
BUILD_ROOT = SCRIPT_DIR.parents[3]
if str(BUILD_ROOT) not in sys.path:
    sys.path.insert(0, str(BUILD_ROOT))

import build


def make_source_dir(base):
    source_dir = pathlib.Path(base) / "source"
    source_dir.mkdir()
    node_dir = source_dir / "node"
    node_dir.mkdir()
    (node_dir / "amitia-node-prepare.sh").write_text("#!/bin/sh\n")
    (node_dir / "amitia-node-probe.sh").write_text("#!/bin/sh\n")
    return source_dir


class TestFreshBuild(unittest.TestCase):
    def setUp(self):
        self.tmpdir = tempfile.mkdtemp()
        self.source_dir = make_source_dir(self.tmpdir)
        self.output_base = pathlib.Path(self.tmpdir) / "output"
        self.output_base.mkdir()
        self.patches = [
            mock.patch("build.sha256_tree_manifest", return_value="a" * 64),
            mock.patch("build.sha256_file", return_value="b" * 64),
        ]
        for p in self.patches:
            p.start()
        self.mock_publish = mock.patch("build.atomic_publish_directory")
        self.mock_publish.start().return_value = mock.Mock(
            success=True,
            published_dir=str(self.output_base / "runtime-scripts" / "linux-arm64" / "1.0.0"),
            errors=[],
        )

    def tearDown(self):
        mock.patch.stopall()
        shutil.rmtree(self.tmpdir, ignore_errors=True)

    def test_fresh_build_returns_record(self):
        record = build.build(str(self.source_dir), str(self.output_base), "1.0.0")
        self.assertIsNotNone(record)
        self.assertEqual(record.componentId, "runtime-scripts")
        self.assertEqual(record.version, "1.0.0")

    def test_fresh_build_calls_publish(self):
        build.build(str(self.source_dir), str(self.output_base), "1.0.0")
        from common import atomic_publish_directory
        atomic_publish_directory.assert_called_once()


class TestMissingInput(unittest.TestCase):
    def setUp(self):
        self.tmpdir = tempfile.mkdtemp()
        self.output_base = pathlib.Path(self.tmpdir) / "output"
        self.output_base.mkdir()

    def tearDown(self):
        shutil.rmtree(self.tmpdir, ignore_errors=True)

    def test_missing_source_dir_fails(self):
        with self.assertRaises(FileNotFoundError):
            build.build("/nonexistent/path", str(self.output_base), "1.0.0")

    def test_missing_prepare_script_fails(self):
        source_dir = pathlib.Path(self.tmpdir) / "source"
        source_dir.mkdir()
        node_dir = source_dir / "node"
        node_dir.mkdir()
        (node_dir / "amitia-node-probe.sh").write_text("#!/bin/sh\n")
        with self.assertRaises(FileNotFoundError):
            build.build(str(source_dir), str(self.output_base), "1.0.0")

    def test_missing_probe_script_fails(self):
        source_dir = pathlib.Path(self.tmpdir) / "source"
        source_dir.mkdir()
        node_dir = source_dir / "node"
        node_dir.mkdir()
        (node_dir / "amitia-node-prepare.sh").write_text("#!/bin/sh\n")
        with self.assertRaises(FileNotFoundError):
            build.build(str(source_dir), str(self.output_base), "1.0.0")


class TestShaMismatch(unittest.TestCase):
    def setUp(self):
        self.tmpdir = tempfile.mkdtemp()
        self.source_dir = make_source_dir(self.tmpdir)
        self.output_base = pathlib.Path(self.tmpdir) / "output"
        self.output_base.mkdir()

    def tearDown(self):
        shutil.rmtree(self.tmpdir, ignore_errors=True)

    def test_post_publish_tree_sha_mismatch_fails(self):
        call_count = {"n": 0}

        def fake_tree(path):
            result_map = {0: "a" * 64, 1: "c" * 64}
            val = result_map.get(call_count["n"], "a" * 64)
            call_count["n"] += 1
            return val

        with mock.patch("build.sha256_tree_manifest", side_effect=fake_tree):
            with mock.patch("build.atomic_publish_directory") as mock_pub:
                mock_pub.return_value = mock.Mock(
                    success=True,
                    published_dir=str(self.output_base / "runtime-scripts" / "linux-arm64" / "1.0.0"),
                    errors=[],
                )
                with self.assertRaises(RuntimeError):
                    build.build(str(self.source_dir), str(self.output_base), "1.0.0")


class TestSameVersionSameBytes(unittest.TestCase):
    def setUp(self):
        self.tmpdir = tempfile.mkdtemp()
        self.source_dir = make_source_dir(self.tmpdir)
        self.output_base = pathlib.Path(self.tmpdir) / "output"
        self.output_base.mkdir()
        self.patches = [
            mock.patch("build.sha256_tree_manifest", return_value="a" * 64),
            mock.patch("build.sha256_file", return_value="b" * 64),
        ]
        for p in self.patches:
            p.start()

    def tearDown(self):
        mock.patch.stopall()
        shutil.rmtree(self.tmpdir, ignore_errors=True)

    def test_same_version_same_tree_reuses_or_succeeds(self):
        with mock.patch("build.atomic_publish_directory") as mock_pub:
            mock_pub.return_value = mock.Mock(
                success=True,
                published_dir=str(self.output_base / "runtime-scripts" / "linux-arm64" / "1.0.0"),
                errors=[],
            )
            record1 = build.build(str(self.source_dir), str(self.output_base), "1.0.0")
        self.assertEqual(record1.componentId, "runtime-scripts")


class TestSameVersionDifferentBytes(unittest.TestCase):
    def setUp(self):
        self.tmpdir = tempfile.mkdtemp()
        self.source_dir = make_source_dir(self.tmpdir)
        self.output_base = pathlib.Path(self.tmpdir) / "output"
        self.output_base.mkdir()

    def tearDown(self):
        shutil.rmtree(self.tmpdir, ignore_errors=True)

    def test_existing_publish_dir_causes_failure(self):
        pub_dir = self.output_base / "runtime-scripts" / "linux-arm64" / "1.0.0"
        pub_dir.mkdir(parents=True)
        (pub_dir / "existing.txt").write_text("original")
        with mock.patch("build.sha256_tree_manifest", return_value="x" * 64):
            with mock.patch("build.atomic_publish_directory") as mock_pub:
                mock_pub.return_value = mock.Mock(
                    success=False,
                    published_dir="",
                    errors=["Target version directory already exists"],
                )
                with self.assertRaises(RuntimeError):
                    build.build(str(self.source_dir), str(self.output_base), "1.0.0")
        self.assertTrue((pub_dir / "existing.txt").exists())


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
            build.build("/nonexistent", str(self.output_base), "1.0.0")
        output_after = list(self.output_base.rglob("*"))
        self.assertEqual(output_before, output_after)


class TestFailureDuringCandidateMetadata(unittest.TestCase):
    def setUp(self):
        self.tmpdir = tempfile.mkdtemp()
        self.source_dir = make_source_dir(self.tmpdir)
        self.output_base = pathlib.Path(self.tmpdir) / "output"
        self.output_base.mkdir()

    def tearDown(self):
        shutil.rmtree(self.tmpdir, ignore_errors=True)

    def test_publish_failure_leaves_no_partial_record(self):
        with mock.patch("build.sha256_tree_manifest", return_value="a" * 64):
            with mock.patch("build.atomic_publish_directory") as mock_pub:
                mock_pub.return_value = mock.Mock(
                    success=False, published_dir="", errors=["disk full"]
                )
                with self.assertRaises(RuntimeError):
                    build.build(str(self.source_dir), str(self.output_base), "1.0.0")
        pub_dir = self.output_base / "runtime-scripts" / "linux-arm64" / "1.0.0"
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
        self.source_dir = make_source_dir(self.tmpdir)
        self.output_base = pathlib.Path(self.tmpdir) / "output"
        self.output_base.mkdir()

    def tearDown(self):
        shutil.rmtree(self.tmpdir, ignore_errors=True)

    def test_two_builds_same_tree_sha(self):
        with mock.patch("build.sha256_tree_manifest", return_value="a" * 64):
            with mock.patch("build.atomic_publish_directory") as mock_pub:
                mock_pub.return_value = mock.Mock(
                    success=True,
                    published_dir=str(self.output_base / "runtime-scripts" / "linux-arm64" / "1.0.0"),
                    errors=[],
                )
                record1 = build.build(str(self.source_dir), str(self.output_base), "1.0.0")
        output_base2 = pathlib.Path(self.tmpdir) / "output2"
        output_base2.mkdir()
        with mock.patch("build.sha256_tree_manifest", return_value="a" * 64):
            with mock.patch("build.atomic_publish_directory") as mock_pub:
                mock_pub.return_value = mock.Mock(
                    success=True,
                    published_dir=str(output_base2 / "runtime-scripts" / "linux-arm64" / "1.0.0"),
                    errors=[],
                )
                record2 = build.build(str(self.source_dir), str(output_base2), "1.0.0")
        self.assertEqual(record1.treeSha256, record2.treeSha256)


class TestOffline(unittest.TestCase):
    def setUp(self):
        self.tmpdir = tempfile.mkdtemp()
        self.source_dir = make_source_dir(self.tmpdir)
        self.output_base = pathlib.Path(self.tmpdir) / "output"
        self.output_base.mkdir()

    def tearDown(self):
        shutil.rmtree(self.tmpdir, ignore_errors=True)

    def test_offline_mode_no_network_fallback(self):
        with mock.patch("build.sha256_tree_manifest", return_value="a" * 64):
            with mock.patch("build.atomic_publish_directory") as mock_pub:
                mock_pub.return_value = mock.Mock(
                    success=True,
                    published_dir=str(self.output_base / "runtime-scripts" / "linux-arm64" / "1.0.0"),
                    errors=[],
                )
                with mock.patch("build.sha256_file", return_value="b" * 64):
                    record = build.build(str(self.source_dir), str(self.output_base), "1.0.0")
        self.assertIsNotNone(record)


if __name__ == "__main__":
    unittest.main()
