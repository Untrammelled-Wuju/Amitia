import json
import os
import pathlib
import struct
import sys

ELFMAG = b"\x7fELF"
ELFCLASS64 = 2
ELFDATA2LSB = 1
EM_AARCH64 = 183
ET_EXEC = 2
ET_DYN = 3
PT_NULL = 0
PT_LOAD = 1
PT_INTERP = 3
PT_DYNAMIC = 2
PT_PHDR = 6

EI_CLASS = 4
EI_DATA = 5
EI_NIDENT = 16

ELF64_EHDR_SIZE = 64
ELF64_PHDR_SIZE = 56


class ElfError(Exception):
    pass


def read_bytes(path, offset, size):
    with open(path, "rb") as f:
        f.seek(offset)
        data = f.read(size)
    if len(data) != size:
        raise ElfError("文件截断，无法读取预期数据")
    return data


def inspect(path):
    p = pathlib.Path(path)
    if not p.exists():
        raise ElfError(f"文件不存在: {path}")
    size = p.stat().st_size
    if size == 0:
        raise ElfError("文件为空")

    hdr = read_bytes(p, 0, ELF64_EHDR_SIZE)
    if hdr[:4] != ELFMAG:
        raise ElfError("非 ELF 文件")

    ei_class = hdr[EI_CLASS]
    if ei_class != ELFCLASS64:
        raise ElfError(f"不支持的 ELF Class: {ei_class}")

    ei_data = hdr[EI_DATA]
    endian = "little" if ei_data == ELFDATA2LSB else "big"
    prefix = "<" if endian == "little" else ">"

    fields = struct.unpack_from(prefix + "HHIQQQIHHHHHH", hdr, 16)
    e_type = fields[0]
    e_machine = fields[1]
    e_entry = fields[3]
    e_phoff = fields[4]
    e_phentsize = fields[8]
    e_phnum = fields[9]

    if e_machine != EM_AARCH64:
        raise ElfError(f"Machine 不是 AArch64: {e_machine}")

    if e_type == ET_EXEC:
        etype = "executable"
    elif e_type == ET_DYN:
        etype = "pie"
    else:
        raise ElfError(f"不支持的 ELF Type: {e_type}")

    interp = ""
    has_interp = False
    has_dynamic = False
    load_alignments = []
    has_load = False

    if e_phnum < 0 or e_phnum > 4096:
        raise ElfError(f"异常 Program Header 数量: {e_phnum}")

    if e_phnum > 0 and e_phentsize < ELF64_PHDR_SIZE:
        raise ElfError(f"Program Header Entry Size 过小: {e_phentsize}")

    for i in range(e_phnum):
        off = e_phoff + i * e_phentsize
        phdr = read_bytes(p, off, ELF64_PHDR_SIZE)
        p_type, p_flags, p_offset, p_vaddr, p_paddr, p_filesz, p_memsz, p_align = \
            struct.unpack(prefix + "IIQQQQQQ", phdr)

        if p_type == PT_INTERP:
            has_interp = True
            if p_filesz > 0 and p_filesz < 4096:
                raw = read_bytes(p, p_offset, p_filesz)
                interp = raw.rstrip(b"\x00").decode("ascii", errors="replace")
        elif p_type == PT_DYNAMIC:
            has_dynamic = True
        elif p_type == PT_LOAD:
            has_load = True
            load_alignments.append(p_align)

    if not has_load:
        raise ElfError("缺少 PT_LOAD Segment")

    return {
        "elfClass": 64,
        "endianness": endian,
        "machine": "aarch64",
        "type": etype,
        "hasInterpreter": has_interp,
        "interpreter": interp,
        "hasDynamicSegment": has_dynamic,
        "loadSegmentAlignments": sorted(set(load_alignments)),
    }


def main():
    if len(sys.argv) < 2:
        print("用法: python elf_inspector.py <path>")
        sys.exit(1)
    path = sys.argv[1]
    try:
        result = inspect(path)
        print(json.dumps(result, indent=2, ensure_ascii=False))
    except ElfError as e:
        print(f"[ELF错误] {e}", file=sys.stderr)
        sys.exit(1)
    except Exception as e:
        print(f"[错误] {e}", file=sys.stderr)
        sys.exit(1)


if __name__ == "__main__":
    main()
