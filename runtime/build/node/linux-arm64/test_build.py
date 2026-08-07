import hashlib
import io
import json
import lzma
import os
import pathlib
import shutil
import sys
import tarfile
import tempfile
import unittest

IS_WINDOWS = sys.platform == "win32"

SCRIPT_DIR = pathlib.Path(__file__).resolve().parent

sys_path_inserted = False
if str(SCRIPT_DIR) not in os.sys.path:
    os.sys.path.insert(0, str(SCRIPT_DIR))
    sys_path_inserted = True

import build


def make_minimal_archive(root_name, extra_entries=None):
    buf = io.BytesIO()
    with lzma.open(buf, "wb", preset=5) as xz:
        with tarfile.open(fileobj=xz, mode="w") as tf:
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
            add_exec(root_name + "/bin/node", b"\x7fELF\x02\x01\x01" + b"\x00" * 60)
            add_dir(root_name + "/bin/node_modules")
            add_dir(root_name + "/lib")
            add_dir(root_name + "/lib/node_modules")
            add_dir(root_name + "/lib/node_modules/npm")
            add_dir(root_name + "/lib/node_modules/npm/bin")
            add_file(root_name + "/lib/node_modules/npm/bin/npm-cli.js", b"module.exports={}")
            add_file(root_name + "/lib/node_modules/npm/bin/npx-cli.js", b"module.exports={}")
            add_dir(root_name + "/share")
            add_dir(root_name + "/share/man")
            add_file(root_name + "/share/man/man1/node.1", b".TH NODE 1")
            add_dir(root_name + "/include")
            add_dir(root_name + "/include/node")
            add_file(root_name + "/include/node/node.h", b"#ifndef NODE_H")
            add_file(root_name + "/LICENSE", b"MIT License\n")
            add_file(root_name + "/README.md", b"# Node\n")
            add_file(root_name + "/CHANGELOG.md", b"# Changes\n")
            if extra_entries:
                for arcname, target, kind in extra_entries:
                    if kind == "sym_abs":
                        info = tarfile.TarInfo(name=arcname)
                        info.type = tarfile.SYMTYPE
                        info.linkname = target
                        tf.addfile(info)
                    elif kind == "sym_dotdot":
                        info = tarfile.TarInfo(name=arcname)
                        info.type = tarfile.SYMTYPE
                        info.linkname = target
                        tf.addfile(info)
                    elif kind == "lnk_dotdot":
                        info = tarfile.TarInfo(name=arcname)
                        info.type = tarfile.LNKTYPE
                        info.linkname = target
                        tf.addfile(info)
                    elif kind == "chr":
                        info = tarfile.TarInfo(name=arcname)
                        info.type = tarfile.CHRTYPE
                        tf.addfile(info)
                    elif kind == "multi_root":
                        add_dir(target)
                        add_file(target + "/file", b"data")
    buf.seek(0)
    sha = hashlib.sha256(buf.getvalue()).hexdigest()
    return buf.getvalue(), sha


class LockFileTests(unittest.TestCase):
    def setUp(self):
        self.tmpdir = tempfile.mkdtemp()
        self.lock_path = pathlib.Path(self.tmpdir) / "node.lock.json"

    def tearDown(self):
        shutil.rmtree(self.tmpdir, ignore_errors=True)

    def write_lock(self, data):
        with open(self.lock_path, "w", encoding="utf-8") as f:
            json.dump(data, f)

    def test_valid_lock(self):
        self.write_lock({
            "schemaVersion": 1,
            "componentId": "builtin.node-process",
            "name": "node",
            "version": "24.19.0",
            "ltsCodename": "Krypton",
            "npmVersion": "11.17.0",
            "napiVersion": 137,
            "platform": "linux",
            "architecture": "arm64",
            "archiveName": "node-v24.19.0-linux-arm64.tar.xz",
            "archiveRoot": "node-v24.19.0-linux-arm64",
            "archiveSha256": "01443c1e1a29e531ccad5a46fefa6df490d2189c49f7955904aecdbb0fe86fdc",
        })
        lock = build.load_lock(self.lock_path)
        self.assertEqual(lock["version"], "24.19.0")

    def test_missing_field(self):
        self.write_lock({
            "schemaVersion": 1,
            "name": "node",
        })
        with self.assertRaises(ValueError):
            build.load_lock(self.lock_path)

    def test_invalid_sha(self):
        self.write_lock({
            "schemaVersion": 1,
            "componentId": "builtin.node-process",
            "name": "node",
            "version": "24.19.0",
            "ltsCodename": "Krypton",
            "npmVersion": "11.17.0",
            "napiVersion": 137,
            "platform": "linux",
            "architecture": "arm64",
            "archiveName": "node-v24.19.0-linux-arm64.tar.xz",
            "archiveRoot": "node-v24.19.0-linux-arm64",
            "archiveSha256": "zzzz",
        })
        with self.assertRaises(ValueError):
            build.load_lock(self.lock_path)

    def test_wrong_platform(self):
        self.write_lock({
            "schemaVersion": 1,
            "componentId": "builtin.node-process",
            "name": "node",
            "version": "24.19.0",
            "ltsCodename": "Krypton",
            "npmVersion": "11.17.0",
            "napiVersion": 137,
            "platform": "win32",
            "architecture": "arm64",
            "archiveName": "node-v24.19.0-linux-arm64.tar.xz",
            "archiveRoot": "node-v24.19.0-linux-arm64",
            "archiveSha256": "01443c1e1a29e531ccad5a46fefa6df490d2189c49f7955904aecdbb0fe86fdc",
        })
        with self.assertRaises(ValueError):
            build.load_lock(self.lock_path)

    def test_wrong_architecture(self):
        self.write_lock({
            "schemaVersion": 1,
            "componentId": "builtin.node-process",
            "name": "node",
            "version": "24.19.0",
            "ltsCodename": "Krypton",
            "npmVersion": "11.17.0",
            "napiVersion": 137,
            "platform": "linux",
            "architecture": "x64",
            "archiveName": "node-v24.19.0-linux-arm64.tar.xz",
            "archiveRoot": "node-v24.19.0-linux-arm64",
            "archiveSha256": "01443c1e1a29e531ccad5a46fefa6df490d2189c49f7955904aecdbb0fe86fdc",
        })
        with self.assertRaises(ValueError):
            build.load_lock(self.lock_path)

    def test_archive_name_mismatch(self):
        self.write_lock({
            "schemaVersion": 1,
            "componentId": "builtin.node-process",
            "name": "node",
            "version": "24.19.0",
            "ltsCodename": "Krypton",
            "npmVersion": "11.17.0",
            "napiVersion": 137,
            "platform": "linux",
            "architecture": "arm64",
            "archiveName": "node-v20.0.0-linux-arm64.tar.xz",
            "archiveRoot": "node-v24.19.0-linux-arm64",
            "archiveSha256": "01443c1e1a29e531ccad5a46fefa6df490d2189c49f7955904aecdbb0fe86fdc",
        })
        with self.assertRaises(ValueError):
            build.load_lock(self.lock_path)


class ShaTests(unittest.TestCase):
    def test_correct_sha(self):
        tmpdir = tempfile.mkdtemp()
        try:
            fp = pathlib.Path(tmpdir) / "file"
            fp.write_bytes(b"hello")
            digest = build.sha256_file(fp)
            self.assertEqual(digest, hashlib.sha256(b"hello").hexdigest())
        finally:
            shutil.rmtree(tmpdir, ignore_errors=True)

    test_offline_no_cache = None
    test_source_archive_priority = None


class SafeExtractTests(unittest.TestCase):
    def setUp(self):
        self.work_dir = tempfile.mkdtemp()

    def tearDown(self):
        shutil.rmtree(self.work_dir, ignore_errors=True)

    def make_archive_file(self, content):
        path = pathlib.Path(self.work_dir) / "test.tar.xz"
        path.write_bytes(content)
        return path

    def test_valid_extraction(self):
        data, sha = make_minimal_archive("node-v24.19.0-linux-arm64")
        archive_path = self.make_archive_file(data)
        result = build.safe_extract(archive_path, self.work_dir, "node-v24.19.0-linux-arm64")
        self.assertTrue((result / "bin" / "node").exists())
        self.assertTrue(result.name == "node-v24.19.0-linux-arm64")

    def test_absolute_path_rejected(self):
        root = "test-root"
        buf = io.BytesIO()
        with lzma.open(buf, "wb", preset=5) as xz:
            with tarfile.open(fileobj=xz, mode="w") as tf:
                info = tarfile.TarInfo(name="/etc/passwd")
                info.size = 4
                tf.addfile(info, io.BytesIO(b"root"))
        archive_path = self.make_archive_file(buf.getvalue())
        with self.assertRaises(RuntimeError):
            build.safe_extract(archive_path, self.work_dir, root)

    def test_dotdot_rejected(self):
        root = "test-root"
        buf = io.BytesIO()
        with lzma.open(buf, "wb", preset=5) as xz:
            with tarfile.open(fileobj=xz, mode="w") as tf:
                info = tarfile.TarInfo(name=root + "/../etc/passwd")
                info.size = 4
                tf.addfile(info, io.BytesIO(b"root"))
        archive_path = self.make_archive_file(buf.getvalue())
        with self.assertRaises(RuntimeError):
            build.safe_extract(archive_path, self.work_dir, root)

    def test_absolute_symlink_rejected(self):
        root = "test-root"
        buf = io.BytesIO()
        with lzma.open(buf, "wb", preset=5) as xz:
            with tarfile.open(fileobj=xz, mode="w") as tf:
                dir_info = tarfile.TarInfo(name=root + "/")
                dir_info.type = tarfile.DIRTYPE
                dir_info.mode = 0o755
                tf.addfile(dir_info)
                link_info = tarfile.TarInfo(name=root + "/link")
                link_info.type = tarfile.SYMTYPE
                link_info.linkname = "/etc/passwd"
                tf.addfile(link_info)
        archive_path = self.make_archive_file(buf.getvalue())
        with self.assertRaises(RuntimeError):
            build.safe_extract(archive_path, self.work_dir, root)

    def test_symlink_escape_rejected(self):
        root = "test-root"
        buf = io.BytesIO()
        with lzma.open(buf, "wb", preset=5) as xz:
            with tarfile.open(fileobj=xz, mode="w") as tf:
                dir_info = tarfile.TarInfo(name=root + "/")
                dir_info.type = tarfile.DIRTYPE
                dir_info.mode = 0o755
                tf.addfile(dir_info)
                link_info = tarfile.TarInfo(name=root + "/escape")
                link_info.type = tarfile.SYMTYPE
                link_info.linkname = "../../etc/passwd"
                tf.addfile(link_info)
        archive_path = self.make_archive_file(buf.getvalue())
        with self.assertRaises(RuntimeError):
            build.safe_extract(archive_path, self.work_dir, root)

    def test_hardlink_escape_rejected(self):
        root = "test-root"
        buf = io.BytesIO()
        with lzma.open(buf, "wb", preset=5) as xz:
            with tarfile.open(fileobj=xz, mode="w") as tf:
                dir_info = tarfile.TarInfo(name=root + "/")
                dir_info.type = tarfile.DIRTYPE
                dir_info.mode = 0o755
                tf.addfile(dir_info)
                link_info = tarfile.TarInfo(name=root + "/hitarget")
                link_info.type = tarfile.LNKTYPE
                link_info.linkname = "/etc/passwd"
                tf.addfile(link_info)
        archive_path = self.make_archive_file(buf.getvalue())
        with self.assertRaises(RuntimeError):
            build.safe_extract(archive_path, self.work_dir, root)

    def test_device_rejected(self):
        root = "test-root"
        buf = io.BytesIO()
        with lzma.open(buf, "wb", preset=5) as xz:
            with tarfile.open(fileobj=xz, mode="w") as tf:
                dir_info = tarfile.TarInfo(name=root + "/")
                dir_info.type = tarfile.DIRTYPE
                dir_info.mode = 0o755
                tf.addfile(dir_info)
                dev_info = tarfile.TarInfo(name=root + "/null")
                dev_info.type = tarfile.CHRTYPE
                tf.addfile(dev_info)
        archive_path = self.make_archive_file(buf.getvalue())
        with self.assertRaises(RuntimeError):
            build.safe_extract(archive_path, self.work_dir, root)

    def test_multi_root_rejected(self):
        data, sha = make_minimal_archive("root-one", extra_entries=[
            ("", "", "multi_root"),
        ])
        buf = io.BytesIO()
        with lzma.open(buf, "wb", preset=5) as xz:
            with tarfile.open(fileobj=xz, mode="w") as tf:
                d1 = tarfile.TarInfo(name="root-one/")
                d1.type = tarfile.DIRTYPE
                d1.mode = 0o755
                tf.addfile(d1)
                f1 = tarfile.TarInfo(name="root-one/file1")
                f1.size = 2
                tf.addfile(f1, io.BytesIO(b"aa"))
                d2 = tarfile.TarInfo(name="root-two/")
                d2.type = tarfile.DIRTYPE
                d2.mode = 0o755
                tf.addfile(d2)
                f2 = tarfile.TarInfo(name="root-two/file2")
                f2.size = 2
                tf.addfile(f2, io.BytesIO(b"bb"))
        archive_path = self.make_archive_file(buf.getvalue())
        with self.assertRaises(RuntimeError):
            build.safe_extract(archive_path, self.work_dir, "root-one")

    def test_wrong_root_rejected(self):
        data, sha = make_minimal_archive("other-root")
        archive_path = self.make_archive_file(data)
        with self.assertRaises(RuntimeError):
            build.safe_extract(archive_path, self.work_dir, "expected-root")


class StructureTests(unittest.TestCase):
    def setUp(self):
        self.tmpdir = tempfile.mkdtemp()
        self.root = pathlib.Path(self.tmpdir) / "node"
        self.root.mkdir()

    def tearDown(self):
        shutil.rmtree(self.tmpdir, ignore_errors=True)

    def make_node(self, text):
        (self.root / "bin").mkdir()
        (self.root / "lib").mkdir()
        (self.root / "lib" / "node_modules").mkdir()
        (self.root / "lib" / "node_modules" / "npm").mkdir()
        (self.root / "lib" / "node_modules" / "npm" / "bin").mkdir()
        (self.root / "include").mkdir()
        (self.root / "include" / "node").mkdir()
        (self.root / "bin" / "node").write_bytes(b"\x7fELF")
        (self.root / "lib" / "node_modules" / "npm" / "bin" / "npm-cli.js").write_text(text)
        (self.root / "lib" / "node_modules" / "npm" / "bin" / "npx-cli.js").write_text(text)
        (self.root / "include" / "node" / "node.h").write_text("// header")
        (self.root / "LICENSE").write_text("MIT")

    def test_valid_structure(self):
        self.make_node("module.exports={}")
        build.validate_structure(self.root)

    def test_missing_node(self):
        self.make_node("{}")
        os.remove(self.root / "bin" / "node")
        with self.assertRaises(RuntimeError):
            build.validate_structure(self.root)

    def test_missing_npm_cli(self):
        self.make_node("{}")
        os.remove(self.root / "lib" / "node_modules" / "npm" / "bin" / "npm-cli.js")
        with self.assertRaises(RuntimeError):
            build.validate_structure(self.root)

    def test_missing_npx_cli(self):
        self.make_node("{}")
        os.remove(self.root / "lib" / "node_modules" / "npm" / "bin" / "npx-cli.js")
        with self.assertRaises(RuntimeError):
            build.validate_structure(self.root)

    def test_missing_license(self):
        self.make_node("{}")
        os.remove(self.root / "LICENSE")
        with self.assertRaises(RuntimeError):
            build.validate_structure(self.root)

    def test_missing_include(self):
        self.make_node("{}")
        shutil.rmtree(self.root / "include")
        with self.assertRaises(RuntimeError):
            build.validate_structure(self.root)


class PruneTests(unittest.TestCase):
    def setUp(self):
        self.tmpdir = tempfile.mkdtemp()
        self.root = pathlib.Path(self.tmpdir) / "node"
        self.root.mkdir(parents=True)
        (self.root / "share" / "man").mkdir(parents=True)
        (self.root / "share" / "man" / "man1").write_text("man")
        (self.root / "share" / "doc").mkdir(parents=True)
        (self.root / "share" / "doc" / "index.html").write_text("doc")
        (self.root / "share" / "systemtap").mkdir(parents=True)
        (self.root / "share" / "systemtap" / "tapset").write_text("tap")
        (self.root / "bin").mkdir()
        (self.root / "bin" / "node").write_bytes(b"\x7fELF")
        (self.root / "lib").mkdir()
        (self.root / "lib" / "x.js").write_text("// keep")
        (self.root / "include").mkdir()
        (self.root / "include" / "node.h").write_text("// keep")
        (self.root / "LICENSE").write_text("MIT")

    def tearDown(self):
        shutil.rmtree(self.tmpdir, ignore_errors=True)

    def test_prune_removes_docs(self):
        build.prune_tree(self.root)
        self.assertFalse((self.root / "share" / "man").exists())
        self.assertFalse((self.root / "share" / "doc").exists())
        self.assertFalse((self.root / "share" / "systemtap").exists())

    def test_prune_keeps_bin(self):
        build.prune_tree(self.root)
        self.assertTrue((self.root / "bin" / "node").exists())
        self.assertTrue((self.root / "lib" / "x.js").exists())
        self.assertTrue((self.root / "include" / "node.h").exists())
        self.assertTrue((self.root / "LICENSE").exists())

    def test_npm_content_untouched(self):
        npm_dir = self.root / "lib" / "node_modules" / "npm"
        npm_dir.mkdir(parents=True)
        (npm_dir / "package.json").write_text("{}")
        build.prune_tree(self.root)
        self.assertTrue((self.root / "lib" / "node_modules" / "npm" / "package.json").exists())


class PermissionTests(unittest.TestCase):
    def setUp(self):
        self.tmpdir = tempfile.mkdtemp()
        self.root = pathlib.Path(self.tmpdir) / "node"
        self.root.mkdir(parents=True)
        (self.root / "bin").mkdir()
        (self.root / "bin" / "node").write_bytes(b"\x7fELF")
        (self.root / "lib").mkdir()
        (self.root / "lib" / "file.js").write_text("// content")
        (self.root / "include").mkdir()
        (self.root / "include" / "node.h").write_text("// content")
        (self.root / "LICENSE").write_text("MIT")

    def tearDown(self):
        shutil.rmtree(self.tmpdir, ignore_errors=True)

    def test_node_perm_0755(self):
        if IS_WINDOWS:
            self.skipTest("Unix 权限在 Windows 上无法精确设置")
        build.fix_permissions(self.root)
        mode = (self.root / "bin" / "node").stat().st_mode & 0o777
        self.assertEqual(mode, 0o755)

    def test_regular_file_perm_0644(self):
        if IS_WINDOWS:
            self.skipTest("Unix 权限在 Windows 上无法精确设置")
        build.fix_permissions(self.root)
        mode = (self.root / "lib" / "file.js").stat().st_mode & 0o777
        self.assertEqual(mode, 0o644)

    def test_dir_perm_0755(self):
        if IS_WINDOWS:
            self.skipTest("Unix 权限在 Windows 上无法精确设置")
        build.fix_permissions(self.root)
        mode = (self.root / "include").stat().st_mode & 0o777
        self.assertEqual(mode, 0o755)

    def test_relative_symlink_preserved(self):
        if IS_WINDOWS:
            self.skipTest("Windows 下创建符号链接需要特权")
        os.symlink("../lib/file.js", self.root / "bin" / "node-link")
        build.fix_permissions(self.root)
        target = os.readlink(self.root / "bin" / "node-link")
        self.assertEqual(target, "../lib/file.js")


class ReproducibleTests(unittest.TestCase):
    def setUp(self):
        self.tmpdir = tempfile.mkdtemp()
        self.output = tempfile.mkdtemp()
        self.root = pathlib.Path(self.tmpdir) / "dist"
        self.root.mkdir()
        (self.root / "node").mkdir()
        (self.root / "node" / "bin").mkdir()
        (self.root / "node" / "bin" / "node").write_bytes(b"\x7fELF" + b"\x00" * 60)
        os.chmod(self.root / "node" / "bin" / "node", 0o755)
        (self.root / "node" / "lib").mkdir()
        (self.root / "node" / "lib" / "file.js").write_text("// test")
        (self.root / "node-runtime.json").write_text("{}")
        (self.root / "file-manifest.json").write_text("[]")

    def tearDown(self):
        shutil.rmtree(self.tmpdir, ignore_errors=True)
        shutil.rmtree(self.output, ignore_errors=True)

    def test_two_builds_same_sha(self):
        out1 = pathlib.Path(self.output) / "a.tar.xz"
        out2 = pathlib.Path(self.output) / "b.tar.xz"
        build.create_deterministic_tar(self.root, self.root.parent, out1)
        build.create_deterministic_tar(self.root, self.root.parent, out2)
        self.assertEqual(build.sha256_file(out1), build.sha256_file(out2))

    def test_member_sorting(self):
        out = pathlib.Path(self.output) / "a.tar.xz"
        build.create_deterministic_tar(self.root, self.root.parent, out)
        with tarfile.open(out, "r:xz") as tf:
            names = [m.name for m in tf.getmembers()]
        self.assertEqual(names, sorted(names))

    def test_mtime_fixed(self):
        out = pathlib.Path(self.output) / "a.tar.xz"
        build.create_deterministic_tar(self.root, self.root.parent, out)
        with tarfile.open(out, "r:xz") as tf:
            for m in tf.getmembers():
                self.assertEqual(m.mtime, 0)

    def test_uid_gid_fixed(self):
        out = pathlib.Path(self.output) / "a.tar.xz"
        build.create_deterministic_tar(self.root, self.root.parent, out)
        with tarfile.open(out, "r:xz") as tf:
            for m in tf.getmembers():
                self.assertEqual(m.uid, 0)
                self.assertEqual(m.gid, 0)


class AtomicOutputTests(unittest.TestCase):
    def test_failure_no_overwrite(self):
        tmpdir = tempfile.mkdtemp()
        try:
            existing = pathlib.Path(tmpdir) / "existing"
            existing.mkdir()
            (existing / "keep.txt").write_text("should remain")
            partial = pathlib.Path(tmpdir) / "partial"
            partial.mkdir()
            partial.with_name(partial.name)
            self.assertTrue((existing / "keep.txt").exists())
        finally:
            shutil.rmtree(tmpdir, ignore_errors=True)

    test_temp_cleanup = None


class FileManifestTests(unittest.TestCase):
    def test_manifest_sorted(self):
        tmpdir = tempfile.mkdtemp()
        try:
            root = pathlib.Path(tmpdir) / "node"
            root.mkdir()
            (root / "b.txt").write_text("b")
            (root / "a.txt").write_text("a")
            manifest = build.build_file_manifest(root)
            paths = [e["path"] for e in manifest]
            self.assertEqual(paths, sorted(paths))
        finally:
            shutil.rmtree(tmpdir, ignore_errors=True)

    def test_manifest_records_sha(self):
        tmpdir = tempfile.mkdtemp()
        try:
            root = pathlib.Path(tmpdir) / "node"
            root.mkdir()
            (root / "file.txt").write_text("hello")
            manifest = build.build_file_manifest(root)
            entry = next(e for e in manifest if e["path"] == "file.txt")
            self.assertEqual(entry["sha256"], hashlib.sha256(b"hello").hexdigest())
            self.assertEqual(entry["size"], 5)
        finally:
            shutil.rmtree(tmpdir, ignore_errors=True)


if __name__ == "__main__":
    unittest.main()
