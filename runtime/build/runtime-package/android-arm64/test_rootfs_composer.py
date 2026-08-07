import hashlib
import io
import os
import pathlib
import shutil
import struct
import sys
import tarfile
import tempfile
import unittest

sys.path.insert(0, os.path.dirname(os.path.abspath(__file__)))
import rootfs_composer as rc


def make_tar_with_files(files_map):
    tmp = tempfile.NamedTemporaryFile(suffix=".tar", delete=False)
    tmp.close()
    with tarfile.open(tmp.name, "w:") as tf:
        for name, data in files_map.items():
            info = tarfile.TarInfo(name=name)
            info.size = len(data)
            info.mode = 0o644
            info.uid = 0
            info.gid = 0
            info.mtime = 0
            info.type = tarfile.REGTYPE
            tf.addfile(info, io.BytesIO(data.encode() if isinstance(data, str) else data))
    return tmp.name


class TestRootfsComposer(unittest.TestCase):
    def setUp(self):
        self.tmpdir = tempfile.mkdtemp(prefix="rc_test_")

    def tearDown(self):
        shutil.rmtree(self.tmpdir, ignore_errors=True)

    def test_base_plus_overlay(self):
        base_files = {
            "bin/sh": "#!/bin/sh",
            "usr/bin/env": "#!/bin/sh\nexit 0",
            "opt/amitia": "",
        }
        overlay_files = {
            "opt/amitia/manifest/guest-layout.json": '{"version": "1"}',
            "opt/amitia/manifest/mount-contract.json": '{"mounts": []}',
        }
        base_tar = make_tar_with_files(base_files)
        overlay_tar = make_tar_with_files(overlay_files)
        out_path = os.path.join(self.tmpdir, "rootfs.tar.xz")
        try:
            result_path, result_sha = rc.compose_rootfs(base_tar, overlay_tar, out_path)
            self.assertEqual(result_path, out_path)
            self.assertIsNotNone(result_sha)
            with tarfile.open(out_path, "r:xz") as tf:
                names = {m.name for m in tf.getmembers()}
            self.assertIn("opt/amitia/manifest/guest-layout.json", names)
            self.assertIn("bin/sh", names)
        finally:
            for p in [base_tar, overlay_tar]:
                if os.path.exists(p):
                    os.unlink(p)

    def test_overlay_no_program_files(self):
        base_files = {"opt/amitia/backend": ""}
        overlay_files = {
            "opt/amitia/backend/amitia-server": "FAKEBINARYDATA",
        }
        base_tar = make_tar_with_files(base_files)
        overlay_tar = make_tar_with_files(overlay_files)
        out_path = os.path.join(self.tmpdir, "rootfs.tar.xz")
        try:
            with self.assertRaises(RuntimeError):
                rc.compose_rootfs(base_tar, overlay_tar, out_path)
        finally:
            for p in [base_tar, overlay_tar]:
                if os.path.exists(p):
                    os.unlink(p)

    def test_deterministic_output(self):
        base_files = {
            "bin/sh": "#!/bin/sh",
            "etc/passwd": "root:x:0:0:root:/root:/bin/sh",
        }
        overlay_files = {
            "opt/amitia/manifest/guest-layout.json": '{"version": "1"}',
            "opt/amitia/manifest/mount-contract.json": '{"mounts": []}',
        }
        base_tar = make_tar_with_files(base_files)
        overlay_tar = make_tar_with_files(overlay_files)
        out1 = os.path.join(self.tmpdir, "rootfs1.tar.xz")
        out2 = os.path.join(self.tmpdir, "rootfs2.tar.xz")
        try:
            p1, s1 = rc.compose_rootfs(base_tar, overlay_tar, out1)
            p2, s2 = rc.compose_rootfs(base_tar, overlay_tar, out2)
            self.assertEqual(s1, s2)
        finally:
            for p in [base_tar, overlay_tar]:
                if os.path.exists(p):
                    os.unlink(p)

    def test_overlay_reject_system_file_override(self):
        base_files = {"etc/passwd": "root:x:0:0:root:/root:/bin/sh"}
        overlay_files = {"etc/passwd": "evil:x:0:0::/:/bin/sh"}
        base_tar = make_tar_with_files(base_files)
        overlay_tar = make_tar_with_files(overlay_files)
        out_path = os.path.join(self.tmpdir, "rootfs.tar.xz")
        try:
            with self.assertRaises(RuntimeError):
                rc.compose_rootfs(base_tar, overlay_tar, out_path)
        finally:
            for p in [base_tar, overlay_tar]:
                if os.path.exists(p):
                    os.unlink(p)

    def test_empty_rootfs_with_only_manifest(self):
        base_files = {}
        overlay_files = {
            "opt/amitia/manifest/guest-layout.json": "{}",
            "opt/amitia/manifest/mount-contract.json": "{}",
        }
        base_tar = make_tar_with_files(base_files)
        overlay_tar = make_tar_with_files(overlay_files)
        out_path = os.path.join(self.tmpdir, "rootfs.tar.xz")
        try:
            result_path, result_sha = rc.compose_rootfs(base_tar, overlay_tar, out_path)
            self.assertTrue(os.path.exists(result_path))
            self.assertEqual(len(result_sha), 64)
        finally:
            for p in [base_tar, overlay_tar]:
                if os.path.exists(p):
                    os.unlink(p)


if __name__ == "__main__":
    unittest.main()
