import hashlib
import io
import json
import lzma
import os
import pathlib
import shutil
import struct
import subprocess
import sys
import tarfile
import tempfile
import unittest

SCRIPT_DIR = pathlib.Path(__file__).resolve().parent
sys.path.insert(0, str(SCRIPT_DIR))

import elf_inspector


def make_valid_elf(path, machine=183, etype=3, load_alignments=(0x1000,), interp=False):
    with open(str(path), "wb") as f:
        num_ph = len(load_alignments) + (1 if interp else 0) + 1
        phoff_val = 64
        ehdr = bytearray(64)
        ehdr[0:4] = b"\x7fELF"
        ehdr[4] = 2
        ehdr[5] = 1
        total_ph_size = num_ph * 56
        interp_file_offset = phoff_val + total_ph_size
        struct.pack_into("<HHIQQQIHHHHHH", ehdr, 16,
                         etype, machine, 1, 0x400000, phoff_val,
                         interp_file_offset + 32, 0,
                         64, 56, num_ph, 0, 0, 0)
        f.write(ehdr)
        if interp:
            interp_path = b"/lib/ld-musl-aarch64.so.1\x00"
            ih = struct.pack("<IIQQQQQQ", 3, 4, interp_file_offset, 0, 0,
                             len(interp_path), len(interp_path), 1)
            f.write(ih)
        for align in load_alignments:
            lh = struct.pack("<IIQQQQQQ", 1, 7, 0, 0x400000, 0x400000,
                             0x1000, 0x1000, align)
            f.write(lh)
        dh = struct.pack("<IIQQQQQQ", 2, 6, 0, 0, 0, 0, 0, 8)
        f.write(dh)
        if interp:
            f.write(interp_path)


def make_fake_tarball(binary_content=b"X" * 100, asset_name="qdrant", include_qdrant=True):
    buf = io.BytesIO()
    with tarfile.open(fileobj=buf, mode="w:gz") as tf:
        if include_qdrant:
            info = tarfile.TarInfo(name=asset_name)
            info.size = len(binary_content)
            tf.addfile(info, io.BytesIO(binary_content))
    return buf.getvalue()


class TestElfInspector(unittest.TestCase):
    def setUp(self):
        self.tmp = tempfile.mkdtemp()

    def tearDown(self):
        shutil.rmtree(self.tmp, ignore_errors=True)

    def test_valid_aarch64_pie_4k(self):
        p = pathlib.Path(self.tmp) / "test"
        make_valid_elf(str(p), load_alignments=(0x1000,))
        info = elf_inspector.inspect(str(p))
        self.assertEqual(info["elfClass"], 64)
        self.assertEqual(info["endianness"], "little")
        self.assertEqual(info["machine"], "aarch64")
        self.assertEqual(info["type"], "pie")
        self.assertFalse(info["hasInterpreter"])
        self.assertIn(0x1000, info["loadSegmentAlignments"])

    def test_valid_aarch64_pie_16k(self):
        p = pathlib.Path(self.tmp) / "test"
        make_valid_elf(str(p), load_alignments=(0x4000,))
        info = elf_inspector.inspect(str(p))
        self.assertIn(0x4000, info["loadSegmentAlignments"])

    def test_with_interpreter(self):
        p = pathlib.Path(self.tmp) / "test"
        make_valid_elf(str(p), interp=True, load_alignments=(0x1000,))
        info = elf_inspector.inspect(str(p))
        self.assertTrue(info["hasInterpreter"])
        self.assertIn("musl", info["interpreter"])

    def test_invalid_magic(self):
        p = pathlib.Path(self.tmp) / "test"
        with open(str(p), "wb") as f:
            f.write(b"NOTELF" + b"\x00" * 100)
        with self.assertRaises(Exception):
            elf_inspector.inspect(str(p))

    def test_elf32_not_supported(self):
        p = pathlib.Path(self.tmp) / "test"
        with open(str(p), "wb") as f:
            f.write(b"\x7fELF")
            f.write(b"\x01" + b"\x00" * 63)
        with self.assertRaises(Exception):
            elf_inspector.inspect(str(p))

    def test_big_endian(self):
        p = pathlib.Path(self.tmp) / "test"
        with open(str(p), "wb") as f:
            hdr = bytearray(64)
            hdr[0:4] = b"\x7fELF"
            hdr[4] = 2
            hdr[5] = 2
            f.write(hdr)
        with self.assertRaises(Exception):
            elf_inspector.inspect(str(p))

    def test_x86_64_rejected(self):
        p = pathlib.Path(self.tmp) / "test"
        make_valid_elf(str(p), machine=62)
        with self.assertRaises(Exception):
            elf_inspector.inspect(str(p))

    def test_executable_type(self):
        p = pathlib.Path(self.tmp) / "test"
        make_valid_elf(str(p), etype=2, load_alignments=(0x1000,))
        info = elf_inspector.inspect(str(p))
        self.assertEqual(info["type"], "executable")

    def test_no_load_segment(self):
        p = pathlib.Path(self.tmp) / "test"
        with open(str(p), "wb") as f:
            ehdr = bytearray(64)
            ehdr[0:4] = b"\x7fELF"
            ehdr[4] = 2
            ehdr[5] = 1
            struct.pack_into("<HHIQQQIHHHHHH", ehdr, 16,
                             3, 183, 1, 0x400000, 0, 0, 0,
                             64, 56, 0, 0, 0, 0)
            f.write(ehdr)
        with self.assertRaises(Exception):
            elf_inspector.inspect(str(p))

    def test_dynamic_segment(self):
        p = pathlib.Path(self.tmp) / "test"
        make_valid_elf(str(p))
        info = elf_inspector.inspect(str(p))
        self.assertTrue(info["hasDynamicSegment"])

    def test_empty_file(self):
        p = pathlib.Path(self.tmp) / "test"
        with open(str(p), "wb") as f:
            pass
        with self.assertRaises(Exception):
            elf_inspector.inspect(str(p))


class TestLockValidation(unittest.TestCase):
    def setUp(self):
        self.tmp = tempfile.mkdtemp()
        self.lock_path = pathlib.Path(self.tmp) / "qdrant.lock.json"

    def tearDown(self):
        shutil.rmtree(self.tmp, ignore_errors=True)

    def _write_lock(self, data):
        with open(str(self.lock_path), "w", encoding="utf-8") as f:
            json.dump(data, f)

    def _base_lock(self):
        return {
            "schemaVersion": 1,
            "componentId": "builtin.qdrant-process",
            "name": "qdrant",
            "version": "1.19.0",
            "releaseTag": "v1.19.0",
            "releaseCommit": "74f3e85b9473c62560006c043e13737ce6b48421",
            "releasePublishedAt": "2026-08-05T11:26:05Z",
            "platform": "linux",
            "architecture": "arm64",
            "rustTarget": "aarch64-unknown-linux-musl",
            "libc": "musl",
            "assetName": "qdrant-aarch64-unknown-linux-musl.tar.gz",
            "assetSize": 29972695,
            "assetContentType": "application/gzip",
            "assetSha256": "8986afbbff9ac32d6e2dbe5cabec80565f613f777126096a461ba066573d3245",
            "licenseFile": "LICENSE-QDRANT",
            "licenseSha256": "c71d239df91726fc519c6eb72d318ec65820627232b2f796219e87dcf35d0ab4",
        }

    def test_valid_lock(self):
        self._write_lock(self._base_lock())
        import build
        lock = build.load_lock(str(self.lock_path))
        self.assertEqual(lock["version"], "1.19.0")

    def test_missing_field(self):
        lock = self._base_lock()
        del lock["assetSha256"]
        self._write_lock(lock)
        import build
        with self.assertRaises(ValueError):
            build.load_lock(str(self.lock_path))

    def test_invalid_platform(self):
        lock = self._base_lock()
        lock["platform"] = "win32"
        self._write_lock(lock)
        import build
        with self.assertRaises(ValueError):
            build.load_lock(str(self.lock_path))

    def test_invalid_architecture(self):
        lock = self._base_lock()
        lock["architecture"] = "x86_64"
        self._write_lock(lock)
        import build
        with self.assertRaises(ValueError):
            build.load_lock(str(self.lock_path))

    def test_invalid_rust_target(self):
        lock = self._base_lock()
        lock["rustTarget"] = "x86_64-unknown-linux-gnu"
        self._write_lock(lock)
        import build
        with self.assertRaises(ValueError):
            build.load_lock(str(self.lock_path))

    def test_invalid_libc(self):
        lock = self._base_lock()
        lock["libc"] = "glibc"
        self._write_lock(lock)
        import build
        with self.assertRaises(ValueError):
            build.load_lock(str(self.lock_path))

    def test_invalid_sha_format(self):
        lock = self._base_lock()
        lock["assetSha256"] = "too-short"
        self._write_lock(lock)
        import build
        with self.assertRaises(ValueError):
            build.load_lock(str(self.lock_path))

    def test_zero_asset_size(self):
        lock = self._base_lock()
        lock["assetSize"] = 0
        self._write_lock(lock)
        import build
        with self.assertRaises(ValueError):
            build.load_lock(str(self.lock_path))

    def test_incomplete_commit(self):
        lock = self._base_lock()
        lock["releaseCommit"] = "abc123"
        self._write_lock(lock)
        import build
        with self.assertRaises(ValueError):
            build.load_lock(str(self.lock_path))

    def test_invalid_license_sha(self):
        lock = self._base_lock()
        lock["licenseSha256"] = "not-a-hex-string-long-enough-to-be-valid-sha256-value-1234567890abcdef"
        self._write_lock(lock)
        import build
        with self.assertRaises(ValueError):
            build.load_lock(str(self.lock_path))


class TestSafeExtract(unittest.TestCase):
    def setUp(self):
        self.tmp = tempfile.mkdtemp()

    def tearDown(self):
        shutil.rmtree(self.tmp, ignore_errors=True)

    def _make_tar(self, members):
        buf = io.BytesIO()
        with tarfile.open(fileobj=buf, mode="w:gz") as tf:
            for name, content in members:
                info = tarfile.TarInfo(name=name)
                info.size = len(content)
                tf.addfile(info, io.BytesIO(content))
        archive_path = pathlib.Path(self.tmp) / "test.tar.gz"
        with open(str(archive_path), "wb") as f:
            f.write(buf.getvalue())
        return archive_path

    def test_normal_extract(self):
        import build
        archive = self._make_tar([
            ("qdrant", b"X" * 100),
            ("config.yaml", b"test"),
        ])
        work = pathlib.Path(self.tmp) / "work"
        work.mkdir()
        result = build.safe_extract(str(archive), str(work))
        self.assertTrue((result / "qdrant").exists())

    def test_absolute_path_rejected(self):
        import build
        archive = self._make_tar([
            ("/etc/passwd", b"X" * 100),
        ])
        work = pathlib.Path(self.tmp) / "work"
        work.mkdir()
        with self.assertRaises(RuntimeError):
            build.safe_extract(str(archive), str(work))

    def test_path_traversal_rejected(self):
        import build
        archive = self._make_tar([
            ("../escape", b"X" * 100),
        ])
        work = pathlib.Path(self.tmp) / "work"
        work.mkdir()
        with self.assertRaises(RuntimeError):
            build.safe_extract(str(archive), str(work))

    def test_symlink_escapes_rejected(self):
        workdir = pathlib.Path(self.tmp) / "work"
        workdir.mkdir()
        buf = io.BytesIO()
        with tarfile.open(fileobj=buf, mode="w:gz") as tf:
            info = tarfile.TarInfo(name="link")
            info.type = tarfile.SYMTYPE
            info.linkname = "/etc/passwd"
            tf.addfile(info)
        archive_path = pathlib.Path(self.tmp) / "test.tar.gz"
        with open(str(archive_path), "wb") as f:
            f.write(buf.getvalue())
        with self.assertRaises(RuntimeError):
            import build
            build.safe_extract(str(archive_path), str(workdir))

    def test_fifo_rejected(self):
        workdir = pathlib.Path(self.tmp) / "work"
        workdir.mkdir()
        buf = io.BytesIO()
        with tarfile.open(fileobj=buf, mode="w:gz") as tf:
            info = tarfile.TarInfo(name="pipe")
            info.type = tarfile.FIFOTYPE
            tf.addfile(info)
        archive_path = pathlib.Path(self.tmp) / "test.tar.gz"
        with open(str(archive_path), "wb") as f:
            f.write(buf.getvalue())
        with self.assertRaises(RuntimeError):
            import build
            build.safe_extract(str(archive_path), str(workdir))


class TestFindQdrantBinary(unittest.TestCase):
    def setUp(self):
        self.tmp = tempfile.mkdtemp()

    def tearDown(self):
        shutil.rmtree(self.tmp, ignore_errors=True)

    def test_binary_at_root(self):
        import build
        extract = pathlib.Path(self.tmp) / "extract"
        extract.mkdir()
        bin_path = extract / "qdrant"
        with open(str(bin_path), "wb") as f:
            f.write(b"X" * 100)
        result = build.find_qdrant_binary(str(extract))
        self.assertEqual(str(result), str(bin_path))

    def test_binary_in_subdir(self):
        import build
        extract = pathlib.Path(self.tmp) / "extract"
        extract.mkdir()
        sub = extract / "qdrant-x"
        sub.mkdir()
        bin_path = sub / "qdrant"
        with open(str(bin_path), "wb") as f:
            f.write(b"X" * 100)
        result = build.find_qdrant_binary(str(extract))
        self.assertEqual(str(result), str(bin_path))

    def test_no_binary(self):
        import build
        extract = pathlib.Path(self.tmp) / "extract"
        extract.mkdir()
        with self.assertRaises(RuntimeError):
            build.find_qdrant_binary(str(extract))

    def test_multiple_candidates(self):
        import build
        extract = pathlib.Path(self.tmp) / "extract"
        extract.mkdir()
        (extract / "qdrant").write_bytes(b"X" * 10)
        sub = extract / "sub"
        sub.mkdir()
        (sub / "qdrant").write_bytes(b"X" * 10)
        with self.assertRaises(RuntimeError):
            build.find_qdrant_binary(str(extract))


class TestFileManifest(unittest.TestCase):
    def setUp(self):
        self.tmp = tempfile.mkdtemp()

    def tearDown(self):
        shutil.rmtree(self.tmp, ignore_errors=True)

    def test_sorted_paths(self):
        import build
        dist = pathlib.Path(self.tmp) / "dist"
        (dist / "bin").mkdir(parents=True)
        (dist / "bin" / "qdrant").write_bytes(b"X" * 10)
        (dist / "LICENSE").write_bytes(b"license")
        manifest = build.build_file_manifest(str(dist))
        paths = [e["path"] for e in manifest]
        self.assertEqual(paths, sorted(paths))

    def test_sha256_for_files(self):
        import build
        dist = pathlib.Path(self.tmp) / "dist"
        dist.mkdir()
        (dist / "qdrant").write_bytes(b"X" * 100)
        manifest = build.build_file_manifest(str(dist))
        entry = next(e for e in manifest if e["path"] == "qdrant")
        self.assertEqual(entry["sha256"],
                         hashlib.sha256(b"X" * 100).hexdigest())


class TestDeterministicTar(unittest.TestCase):
    def setUp(self):
        self.tmp = tempfile.mkdtemp()

    def tearDown(self):
        shutil.rmtree(self.tmp, ignore_errors=True)

    def test_same_content_same_sha(self):
        import build
        dist = pathlib.Path(self.tmp) / "dist"
        (dist / "bin").mkdir(parents=True)
        (dist / "bin" / "qdrant").write_bytes(b"X" * 100)
        (dist / "LICENSE").write_bytes(b"license")
        out1 = pathlib.Path(self.tmp) / "1.tar.xz"
        out2 = pathlib.Path(self.tmp) / "2.tar.xz"
        build.create_deterministic_tar(str(dist), str(out1))
        build.create_deterministic_tar(str(dist), str(out2))
        h1 = hashlib.sha256(out1.read_bytes()).hexdigest()
        h2 = hashlib.sha256(out2.read_bytes()).hexdigest()
        self.assertEqual(h1, h2)

    def test_fixed_mtime(self):
        import build
        dist = pathlib.Path(self.tmp) / "dist"
        dist.mkdir()
        (dist / "test.txt").write_bytes(b"data")
        out = pathlib.Path(self.tmp) / "out.tar.xz"
        build.create_deterministic_tar(str(dist), str(out))
        with lzma.open(str(out), "rb") as xz:
            with tarfile.open(fileobj=xz, mode="r") as tf:
                for m in tf.getmembers():
                    self.assertEqual(m.mtime, 0)
                    self.assertEqual(m.uid, 0)
                    self.assertEqual(m.gid, 0)


if __name__ == "__main__":
    unittest.main()
