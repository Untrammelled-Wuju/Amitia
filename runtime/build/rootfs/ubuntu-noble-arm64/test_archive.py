import hashlib
import io
import json
import lzma
import os
import pathlib
import stat
import sys
import tarfile
import tempfile
import unittest

SCRIPT_DIR = pathlib.Path(__file__).resolve().parent
sys.path.insert(0, str(SCRIPT_DIR))

from archive import (
    sha256_file,
    verify_archive_security,
    verify_no_extra_rootfs,
    create_deterministic_tar,
    write_file_manifest,
)


class TestArchiveSecurity(unittest.TestCase):
    def create_test_archive(self, members_data):
        buffer = io.BytesIO()
        with lzma.open(buffer, "wb", preset=5) as xz:
            with tarfile.open(fileobj=xz, mode="w") as tf:
                for name, content, mtype in members_data:
                    info = tarfile.TarInfo(name=name)
                    info.uid = 0
                    info.gid = 0
                    info.uname = "root"
                    info.gname = "root"
                    info.mtime = 0
                    if mtype == "dir":
                        info.type = tarfile.DIRTYPE
                        info.mode = 0o755
                        tf.addfile(info)
                    elif mtype == "symlink":
                        info.type = tarfile.SYMTYPE
                        info.linkname = content
                        tf.addfile(info)
                    elif mtype == "file":
                        info.type = tarfile.REGTYPE
                        info.mode = 0o644
                        info.size = len(content)
                        tf.addfile(info, io.BytesIO(content.encode()))
        return buffer.getvalue()

    def test_valid_archive(self):
        members = [
            ("bin/", b"", "dir"),
            ("bin/bash", "#!/bin/bash\necho hello\n", "file"),
            ("bin/sh", "bash", "symlink"),
        ]
        archive_data = self.create_test_archive(members)
        with tempfile.NamedTemporaryFile(suffix=".tar.xz", delete=False) as f:
            f.write(archive_data)
            archive_path = pathlib.Path(f.name)
        try:
            issues = verify_archive_security(archive_path)
            self.assertEqual(len(issues), 0, f"安全检查应为空: {issues}")
        finally:
            os.unlink(archive_path)

    def test_absolute_path_rejected(self):
        members = [
            ("/etc/passwd", "root:x:0:0\n", "file"),
        ]
        archive_data = self.create_test_archive(members)
        with tempfile.NamedTemporaryFile(suffix=".tar.xz", delete=False) as f:
            f.write(archive_data)
            archive_path = pathlib.Path(f.name)
        try:
            issues = verify_archive_security(archive_path)
            self.assertTrue(any("绝对路径" in i for i in issues))
        finally:
            os.unlink(archive_path)

    def test_traversal_rejected(self):
        members = [
            ("../../../etc/passwd", "malicious\n", "file"),
        ]
        archive_data = self.create_test_archive(members)
        with tempfile.NamedTemporaryFile(suffix=".tar.xz", delete=False) as f:
            f.write(archive_data)
            archive_path = pathlib.Path(f.name)
        try:
            issues = verify_archive_security(archive_path)
            self.assertTrue(any("路径穿越" in i for i in issues))
        finally:
            os.unlink(archive_path)

    def test_win_drive_rejected(self):
        members = [
            ("C:/windows/system32/config", "malicious\n", "file"),
        ]
        archive_data = self.create_test_archive(members)
        with tempfile.NamedTemporaryFile(suffix=".tar.xz", delete=False) as f:
            f.write(archive_data)
            archive_path = pathlib.Path(f.name)
        try:
            issues = verify_archive_security(archive_path)
            self.assertTrue(any("Windows" in i or "盘符" in i for i in issues))
        finally:
            os.unlink(archive_path)

    def test_absolute_symlink_rejected(self):
        members = [
            ("bin/sh", "/bin/bash", "symlink"),
        ]
        archive_data = self.create_test_archive(members)
        with tempfile.NamedTemporaryFile(suffix=".tar.xz", delete=False) as f:
            f.write(archive_data)
            archive_path = pathlib.Path(f.name)
        try:
            issues = verify_archive_security(archive_path)
            self.assertTrue(any("绝对符号链接" in i for i in issues))
        finally:
            os.unlink(archive_path)

    def test_no_extra_rootfs_layer(self):
        members = [
            ("rootfs/", b"", "dir"),
            ("rootfs/bin/bash", "#!/bin/bash\n", "file"),
        ]
        archive_data = self.create_test_archive(members)
        with tempfile.NamedTemporaryFile(suffix=".tar.xz", delete=False) as f:
            f.write(archive_data)
            archive_path = pathlib.Path(f.name)
        try:
            self.assertTrue(verify_no_extra_rootfs(archive_path))
        finally:
            os.unlink(archive_path)

    def test_no_rootfs_layer_pass(self):
        members = [
            ("bin/", b"", "dir"),
            ("bin/bash", "#!/bin/bash\n", "file"),
        ]
        archive_data = self.create_test_archive(members)
        with tempfile.NamedTemporaryFile(suffix=".tar.xz", delete=False) as f:
            f.write(archive_data)
            archive_path = pathlib.Path(f.name)
        try:
            self.assertFalse(verify_no_extra_rootfs(archive_path))
        finally:
            os.unlink(archive_path)


class TestDeterministicArchive(unittest.TestCase):
    def test_same_input_same_sha(self):
        with tempfile.TemporaryDirectory() as tmpdir:
            root1 = pathlib.Path(tmpdir) / "root1"
            root1.mkdir()
            (root1 / "bin").mkdir()
            (root1 / "etc").mkdir()
            with open(root1 / "bin" / "bash", "w") as f:
                f.write("#!/bin/bash\necho hello\n")
            try:
                os.chmod(root1 / "bin" / "bash", 0o755)
            except OSError:
                pass
            (root1 / "etc" / "hostname").write_text("test\n")
            try:
                os.symlink("../bin/bash", root1 / "bin" / "sh")
            except (OSError, NotImplementedError):
                with open(root1 / "bin" / "sh", "w") as f:
                    f.write("#!/bin/sh\necho sh\n")
            archive_path1 = pathlib.Path(tmpdir) / "test1.tar.xz"
            create_deterministic_tar(root1, archive_path1)
            archive_path2 = pathlib.Path(tmpdir) / "test2.tar.xz"
            create_deterministic_tar(root1, archive_path2)
            sha1 = sha256_file(archive_path1)
            sha2 = sha256_file(archive_path2)
            self.assertEqual(sha1, sha2, "相同输入应产生相同 SHA")

    def test_fixed_mtime(self):
        with tempfile.TemporaryDirectory() as tmpdir:
            root = pathlib.Path(tmpdir) / "root"
            root.mkdir()
            (root / "bin").mkdir()
            test_file = root / "bin" / "test"
            test_file.write_text("test\n")
            archive_path = pathlib.Path(tmpdir) / "fixed.tar.xz"
            create_deterministic_tar(root, archive_path)
            with lzma.open(archive_path, "rb") as xz:
                with tarfile.open(fileobj=xz, mode="r") as tf:
                    for m in tf.getmembers():
                        self.assertEqual(m.mtime, 0, f"mtime 应为 0: {m.name}")

    def test_sorted_members(self):
        with tempfile.TemporaryDirectory() as tmpdir:
            root = pathlib.Path(tmpdir) / "root"
            root.mkdir()
            for name in ["xyz", "abc", "mno", "def"]:
                d = root / name
                d.mkdir()
                (d / "file.txt").write_text(f"{name}\n")
            archive_path = pathlib.Path(tmpdir) / "sorted.tar.xz"
            create_deterministic_tar(root, archive_path)
            member_names = []
            with lzma.open(archive_path, "rb") as xz:
                with tarfile.open(fileobj=xz, mode="r") as tf:
                    for m in tf.getmembers():
                        member_names.append(m.name)
            sorted_names = sorted(member_names)
            self.assertEqual(member_names, sorted_names, "成员应按 UTF-8 排序")

    def test_no_pax_headers(self):
        with tempfile.TemporaryDirectory() as tmpdir:
            root = pathlib.Path(tmpdir) / "root"
            root.mkdir()
            (root / "bin").mkdir()
            (root / "bin" / "test").write_text("test\n")
            archive_path = pathlib.Path(tmpdir) / "nopax.tar.xz"
            create_deterministic_tar(root, archive_path)
            with lzma.open(archive_path, "rb") as xz:
                with tarfile.open(fileobj=xz, mode="r") as tf:
                    for m in tf.getmembers():
                        self.assertEqual(len(m.pax_headers), 0, "不应有 PAX 头")

    def test_uid_gid_fixed(self):
        with tempfile.TemporaryDirectory() as tmpdir:
            root = pathlib.Path(tmpdir) / "root"
            root.mkdir()
            (root / "bin").mkdir()
            (root / "bin" / "test").write_text("test\n")
            archive_path = pathlib.Path(tmpdir) / "uid.tar.xz"
            create_deterministic_tar(root, archive_path)
            with lzma.open(archive_path, "rb") as xz:
                with tarfile.open(fileobj=xz, mode="r") as tf:
                    for m in tf.getmembers():
                        self.assertEqual(m.uid, 0)
                        self.assertEqual(m.gid, 0)


class TestFileManifest(unittest.TestCase):
    def test_manifest_structure(self):
        with tempfile.TemporaryDirectory() as tmpdir:
            root = pathlib.Path(tmpdir) / "root"
            root.mkdir()
            (root / "bin").mkdir()
            bin_bash = root / "bin" / "bash"
            bin_bash.write_text("#!/bin/bash\n")
            bin_bash.chmod(0o755)
            output_dir = pathlib.Path(tmpdir) / "out"
            output_dir.mkdir()
            manifest_path = write_file_manifest(output_dir, root)
            with open(manifest_path, "r", encoding="utf-8") as f:
                manifest = json.load(f)
            paths = [e["path"] for e in manifest]
            self.assertEqual(paths, sorted(paths), "清单应按路径排序")
            for entry in manifest:
                self.assertIn("path", entry)
                self.assertIn("type", entry)
                self.assertIn(entry["type"], ["file", "directory", "symlink", "dirsymlink"])
                if entry["type"] == "file":
                    self.assertIn("sha256", entry)
                    self.assertIn("size", entry)
                    self.assertEqual(len(entry["sha256"]), 64)
                if entry["type"] in ("symlink", "dirsymlink"):
                    self.assertIn("target", entry)
                self.assertIn("mode", entry)
                self.assertIn("uid", entry)
                self.assertIn("gid", entry)


if __name__ == "__main__":
    unittest.main()
