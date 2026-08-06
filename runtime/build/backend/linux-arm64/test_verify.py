import io
import json
import lzma
import os
import pathlib
import shutil
import struct
import tarfile
import tempfile
import unittest

import inspect_elf


def make_valid_elf(path, machine=183, etype=2, load_alignments=(0x1000,), interp=False, entry=0x400000):
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
                         etype, machine, 1, entry, phoff_val,
                         interp_file_offset + 32 if interp else 0, 0,
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


class TestInspectElf(unittest.TestCase):
    def setUp(self):
        self.tmp = tempfile.mkdtemp()

    def tearDown(self):
        shutil.rmtree(self.tmp, ignore_errors=True)

    def test_valid_aarch64_elf64(self):
        p = pathlib.Path(self.tmp) / "test"
        make_valid_elf(str(p))
        info = inspect_elf.inspect(str(p))
        self.assertEqual(info["elfClass"], 64)
        self.assertEqual(info["endianness"], "little")
        self.assertEqual(info["machine"], "aarch64")
        self.assertEqual(info["type"], "executable")
        self.assertFalse(info["hasInterpreter"])
        self.assertTrue(info["static"])

    def test_valid_entry_point(self):
        p = pathlib.Path(self.tmp) / "test"
        make_valid_elf(str(p), entry=0x400100)
        info = inspect_elf.inspect(str(p))
        self.assertEqual(info["entryPoint"], 0x400100)

    def test_invalid_magic(self):
        p = pathlib.Path(self.tmp) / "test"
        with open(str(p), "wb") as f:
            f.write(b"NOTELF" + b"\x00" * 100)
        with self.assertRaises(inspect_elf.ElfError):
            inspect_elf.inspect(str(p))

    def test_elf32_rejected(self):
        p = pathlib.Path(self.tmp) / "test"
        with open(str(p), "wb") as f:
            f.write(b"\x7fELF")
            f.write(b"\x01" + b"\x00" * 63)
        with self.assertRaises(inspect_elf.ElfError):
            inspect_elf.inspect(str(p))

    def test_big_endian_rejected(self):
        p = pathlib.Path(self.tmp) / "test"
        with open(str(p), "wb") as f:
            hdr = bytearray(64)
            hdr[0:4] = b"\x7fELF"
            hdr[4] = 2
            hdr[5] = 2
            f.write(hdr)
        with self.assertRaises(inspect_elf.ElfError):
            inspect_elf.inspect(str(p))

    def test_x86_64_rejected(self):
        p = pathlib.Path(self.tmp) / "test"
        make_valid_elf(str(p), machine=62)
        with self.assertRaises(inspect_elf.ElfError):
            inspect_elf.inspect(str(p))

    def test_arm32_rejected(self):
        p = pathlib.Path(self.tmp) / "test"
        make_valid_elf(str(p), machine=40)
        with self.assertRaises(inspect_elf.ElfError):
            inspect_elf.inspect(str(p))

    def test_entry_point_zero_rejected(self):
        p = pathlib.Path(self.tmp) / "test"
        make_valid_elf(str(p), entry=0)
        with self.assertRaises(inspect_elf.ElfError):
            inspect_elf.inspect(str(p))

    def test_et_dyn_rejected(self):
        p = pathlib.Path(self.tmp) / "test"
        make_valid_elf(str(p), etype=3, entry=0x400000)
        with self.assertRaises(inspect_elf.ElfError):
            inspect_elf.inspect(str(p))

    def test_with_interp_rejected(self):
        p = pathlib.Path(self.tmp) / "test"
        make_valid_elf(str(p), interp=True, entry=0x400000)
        with self.assertRaises(inspect_elf.ElfError):
            inspect_elf.inspect(str(p))

    def test_no_load_segment_rejected(self):
        p = pathlib.Path(self.tmp) / "test"
        with open(str(p), "wb") as f:
            ehdr = bytearray(64)
            ehdr[0:4] = b"\x7fELF"
            ehdr[4] = 2
            ehdr[5] = 1
            struct.pack_into("<HHIQQQIHHHHHH", ehdr, 16,
                             2, 183, 1, 0x400000, 0, 0, 0,
                             64, 56, 0, 0, 0, 0)
            f.write(ehdr)
        with self.assertRaises(inspect_elf.ElfError):
            inspect_elf.inspect(str(p))

    def test_empty_file_rejected(self):
        p = pathlib.Path(self.tmp) / "test"
        with open(str(p), "wb") as f:
            pass
        with self.assertRaises(inspect_elf.ElfError):
            inspect_elf.inspect(str(p))

    def test_truncated_file_rejected(self):
        p = pathlib.Path(self.tmp) / "test"
        with open(str(p), "wb") as f:
            f.write(b"\x7fELF\x02\x01")
        with self.assertRaises(inspect_elf.ElfError):
            inspect_elf.inspect(str(p))

    def test_16k_alignment(self):
        p = pathlib.Path(self.tmp) / "test"
        make_valid_elf(str(p), load_alignments=(0x4000,))
        info = inspect_elf.inspect(str(p))
        self.assertIn(0x4000, info["loadSegmentAlignments"])


class TestInspectMinimal(unittest.TestCase):
    def setUp(self):
        self.tmp = tempfile.mkdtemp()

    def tearDown(self):
        shutil.rmtree(self.tmp, ignore_errors=True)

    def test_missing_file(self):
        result = inspect_elf.inspect_minimal("/nonexistent/path")
        self.assertFalse(result["exists"])

    def test_empty_file(self):
        p = pathlib.Path(self.tmp) / "empty"
        p.write_bytes(b"")
        result = inspect_elf.inspect_minimal(str(p))
        self.assertTrue(result["exists"])
        self.assertIn("error", result)


if __name__ == "__main__":
    unittest.main()
