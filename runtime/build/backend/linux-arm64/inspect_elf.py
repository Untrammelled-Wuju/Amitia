import json
import os
import pathlib
import struct
import sys

ELFMAG = b"\x7fELF"
ELFCLASS64 = 2
ELFCLASS32 = 1
ELFDATA2LSB = 1
ELFDATA2MSB = 2
EM_AARCH64 = 183
EM_X86_64 = 62
EM_ARM = 40
ET_EXEC = 2
ET_DYN = 3
PT_NULL = 0
PT_LOAD = 1
PT_INTERP = 3
PT_DYNAMIC = 2
PT_PHDR = 6
DT_NEEDED = 1
DT_RPATH = 15
DT_RUNPATH = 29
DT_STRTAB = 5
DT_STRSZ = 10

EI_CLASS = 4
EI_DATA = 5
EI_NIDENT = 16

ELF64_EHDR_SIZE = 64
ELF64_PHDR_SIZE = 56
ELF64_DYN_ENT_SIZE = 16
ELF32_EHDR_SIZE = 52
ELF32_PHDR_SIZE = 32


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
    if ei_class == ELFCLASS32:
        raise ElfError("不支持的 ELF Class: 32")
    if ei_class != ELFCLASS64:
        raise ElfError(f"不支持的 ELF Class: {ei_class}")

    ei_data = hdr[EI_DATA]
    if ei_data == ELFDATA2LSB:
        endian = "little"
        prefix = "<"
    elif ei_data == ELFDATA2MSB:
        endian = "big"
        prefix = ">"
    else:
        raise ElfError(f"未知的 ELF 数据编码: {ei_data}")

    if endian != "little":
        raise ElfError(f"端序非 Little Endian: {endian}")

    fields = struct.unpack_from(prefix + "HHIQQQIHHHHHH", hdr, 16)
    e_type = fields[0]
    e_machine = fields[1]
    e_entry = fields[3]
    e_phoff = fields[4]
    e_phentsize = fields[8]
    e_phnum = fields[9]

    if e_machine != EM_AARCH64:
        if e_machine == EM_X86_64:
            raise ElfError("Machine 为 x86_64，期望 AArch64")
        if e_machine == EM_ARM:
            raise ElfError("Machine 为 ARM32，期望 AArch64")
        raise ElfError(f"Machine 不是 AArch64: {e_machine}")

    if e_type == ET_EXEC:
        etype = "executable"
    elif e_type == ET_DYN:
        etype = "pie/shared"
        raise ElfError("Go 静态可执行文件不应为 ET_DYN")
    else:
        raise ElfError(f"不支持的 ELF Type: {e_type}")

    if e_entry == 0:
        raise ElfError("Entry Point 为零")

    interp = ""
    has_interp = False
    has_dynamic = False
    needed_libraries = []
    rpath = None
    runpath = None
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
            strtab_addr = None
            strsz_val = 0
            dynamic_entries = []
            if p_filesz > 0:
                num_entries = p_filesz // ELF64_DYN_ENT_SIZE
                for j in range(num_entries):
                    dyn_off = p_offset + j * ELF64_DYN_ENT_SIZE
                    dyn = read_bytes(p, dyn_off, ELF64_DYN_ENT_SIZE)
                    d_tag, d_val = struct.unpack(prefix + "QQ", dyn)
                    if d_tag == 0:
                        break
                    dynamic_entries.append((d_tag, d_val))
                for d_tag, d_val in dynamic_entries:
                    if d_tag == DT_STRTAB:
                        strtab_addr = d_val
                    elif d_tag == DT_STRSZ:
                        strsz_val = d_val
                if strtab_addr is not None and strsz_val > 0 and strsz_val < 65536:
                    try:
                        strtab_data = read_bytes(p, strtab_addr, strsz_val)
                    except ElfError:
                        strtab_data = b""
                else:
                    strtab_data = b""
                for d_tag, d_val in dynamic_entries:
                    if d_tag == DT_NEEDED:
                        name = _read_string_from_buffer(strtab_data, d_val)
                        if name:
                            needed_libraries.append(name)
                    elif d_tag == DT_RPATH:
                        name = _read_string_from_buffer(strtab_data, d_val)
                        if name:
                            rpath = name
                    elif d_tag == DT_RUNPATH:
                        name = _read_string_from_buffer(strtab_data, d_val)
                        if name:
                            runpath = name
        elif p_type == PT_LOAD:
            has_load = True
            load_alignments.append(p_align)

    if not has_load:
        raise ElfError("缺少 PT_LOAD Segment")

    if has_interp:
        raise ElfError(f"存在 PT_INTERP，非静态可执行文件: {interp}")
    if needed_libraries:
        raise ElfError(f"存在动态依赖库: {needed_libraries}")
    if rpath:
        raise ElfError(f"存在 RPATH: {rpath}")
    if runpath:
        raise ElfError(f"存在 RUNPATH: {runpath}")

    return {
        "elfClass": 64,
        "endianness": "little",
        "machine": "aarch64",
        "type": etype,
        "entryPoint": e_entry,
        "hasInterpreter": False,
        "interpreter": None,
        "hasDynamicSegment": has_dynamic,
        "neededLibraries": [],
        "rpath": None,
        "runpath": None,
        "static": True,
        "loadSegmentAlignments": sorted(set(load_alignments)),
    }


def _read_string_from_buffer(buf, offset):
    if not buf or offset >= len(buf):
        return ""
    end = buf.find(b"\x00", offset)
    if end == -1:
        return ""
    return buf[offset:end].decode("ascii", errors="replace")


def inspect_minimal(path):
    p = pathlib.Path(path)
    if not p.exists():
        return {"exists": False, "error": "文件不存在"}
    size = p.stat().st_size
    if size == 0:
        return {"exists": True, "size": 0, "error": "文件为空"}
    try:
        result = inspect(p)
        result["exists"] = True
        result["size"] = size
        return result
    except ElfError as e:
        return {"exists": True, "size": size, "error": str(e)}


def main():
    if len(sys.argv) < 2:
        print("用法: python inspect_elf.py <path>", file=sys.stderr)
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
