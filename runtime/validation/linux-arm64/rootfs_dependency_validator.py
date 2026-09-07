import argparse
import hashlib
import json
import os
import re
import struct
import sys
from pathlib import Path
from typing import Dict, List, Optional, Set, Tuple


ELF_MAGIC = b"\x7fELF"
ELFCLASS64 = 2
ELF_MACHINE_AARCH64 = 183
ET_DYN = 3
PT_INTERP = 3
PT_DYNAMIC = 2
DT_NEEDED = 1
DT_STRTAB = 5
DT_STRSZ = 10


class ElfInfo:
    def __init__(self):
        self.is_valid_elf = False
        self.is_aarch64 = False
        self.is_dynamic = False
        self.interpreter: Optional[str] = None
        self.needed_libraries: List[str] = []
        self.error: Optional[str] = None


def parse_elf_header(filepath: str) -> ElfInfo:
    info = ElfInfo()
    try:
        with open(filepath, "rb") as f:
            data = f.read(256)
        if len(data) < 24 or data[:4] != ELF_MAGIC:
            info.error = "Not an ELF file"
            return info
        info.is_valid_elf = True
        ei_class = data[4]
        if ei_class != ELFCLASS64:
            info.error = f"ELF class {ei_class} is not 64-bit"
            return info
        e_type = struct.unpack_from("<H", data, 16)[0]
        e_machine = struct.unpack_from("<H", data, 18)[0]
        if e_machine != ELF_MACHINE_AARCH64:
            info.error = f"ELF machine {e_machine} is not AArch64 ({ELF_MACHINE_AARCH64})"
            return info
        info.is_aarch64 = True
        if e_type == ET_DYN:
            info.is_dynamic = True
    except OSError as e:
        info.error = f"Cannot read file: {e}"
    return info


def parse_elf_dependencies(filepath: str) -> ElfInfo:
    info = parse_elf_header(filepath)
    if not info.is_valid_elf or not info.is_aarch64:
        return info
    try:
        with open(filepath, "rb") as f:
            data = f.read()
        if len(data) < 64:
            info.error = "ELF file too small"
            return info
        e_phoff = struct.unpack_from("<Q", data, 32)[0]
        e_phentsize = struct.unpack_from("<H", data, 54)[0]
        e_phnum = struct.unpack_from("<H", data, 56)[0]
        if e_phentsize < 56 or e_phnum == 0:
            return info
        strtab_vaddr = 0
        strsz = 0
        needed_offsets = []
        for i in range(e_phnum):
            ph_offset = e_phoff + i * e_phentsize
            if ph_offset + 56 > len(data):
                break
            p_type = struct.unpack_from("<I", data, ph_offset)[0]
            if p_type == PT_INTERP:
                p_offset = struct.unpack_from("<Q", data, ph_offset + 8)[0]
                p_filesz = struct.unpack_from("<Q", data, ph_offset + 32)[0]
                end = p_offset + p_filesz
                if end <= len(data):
                    interp_data = data[p_offset:end]
                    interp_str = interp_data.split(b"\x00")[0].decode("utf-8", errors="replace")
                    info.interpreter = interp_str
            elif p_type == PT_DYNAMIC:
                p_offset = struct.unpack_from("<Q", data, ph_offset + 8)[0]
                p_filesz = struct.unpack_from("<Q", data, ph_offset + 32)[0]
                dyn_end = p_offset + p_filesz
                j = p_offset
                while j + 16 <= dyn_end and j + 16 <= len(data):
                    d_tag = struct.unpack_from("<q", data, j)[0]
                    d_val = struct.unpack_from("<Q", data, j + 8)[0]
                    if d_tag == DT_NEEDED:
                        needed_offsets.append(d_val)
                    elif d_tag == DT_STRTAB:
                        strtab_vaddr = d_val
                    elif d_tag == DT_STRSZ:
                        strsz = d_val
                    elif d_tag == 0:
                        break
                    j += 16
        if strtab_vaddr > 0 and strsz > 0 and needed_offsets:
            file_size = os.path.getsize(filepath)
            offset_in_file = strtab_vaddr
            if offset_in_file < file_size:
                remaining = file_size - offset_in_file
                read_size = min(strsz, remaining)
                with open(filepath, "rb") as f:
                    f.seek(offset_in_file)
                    strtab_data = f.read(read_size)
                for off in needed_offsets:
                    if off < len(strtab_data):
                        entry = strtab_data[off:].split(b"\x00")[0]
                        lib_name = entry.decode("utf-8", errors="replace")
                        if lib_name:
                            info.needed_libraries.append(lib_name)
    except OSError as e:
        info.error = f"Cannot parse ELF dependencies: {e}"
    return info


def scan_rootfs_libraries(rootfs_path: str) -> Dict[str, List[str]]:
    root = Path(rootfs_path)
    lib_dirs = [
        root / "lib",
        root / "usr" / "lib",
        root / "usr" / "local" / "lib",
    ]
    libraries: Dict[str, List[str]] = {}
    for lib_dir in lib_dirs:
        if not lib_dir.exists():
            continue
        for item in lib_dir.rglob("*.so*"):
            if item.is_symlink() or not item.is_file():
                continue
            rel_path = str(item.relative_to(root))
            libraries[rel_path] = []
            try:
                with open(item, "rb") as f:
                    magic = f.read(4)
                if magic == ELF_MAGIC:
                    elf_info = parse_elf_header(str(item))
                    if elf_info.is_valid_elf:
                        libraries[rel_path] = []
            except OSError:
                pass
    return libraries


def check_dependency_compatibility(
    rootfs_path: str,
    component_manifests: Optional[List[dict]] = None,
) -> dict:
    root = Path(rootfs_path)
    if not root.exists():
        return {"valid": False, "errors": [f"Rootfs not found: {rootfs_path}"]}
    errors = []
    warnings = []
    lib_dirs = [root / "lib", root / "usr" / "lib", root / "usr" / "local" / "lib"]
    available_libs: Set[str] = set()
    for lib_dir in lib_dirs:
        if not lib_dir.exists():
            continue
        for item in lib_dir.iterdir():
            if item.is_symlink() or not item.is_file():
                continue
            if ".so" in item.name:
                available_libs.add(item.name)
    rootfs_libraries = scan_rootfs_libraries(rootfs_path)
    loader_candidates = [
        root / "lib" / "ld-linux-aarch64.so.1",
        root / "lib" / "ld-linux-aarch64.so",
    ]
    loader_found = any(p.exists() for p in loader_candidates)
    if not loader_found:
        errors.append("Missing ARM64 glibc loader (ld-linux-aarch64.so.1)")
    glibc_present = any("libc.so" in lib for lib in available_libs)
    if not glibc_present:
        errors.append("Missing libc.so (glibc)")
    critical_libs = ["libpthread.so", "libdl.so", "librt.so", "libm.so"]
    for crit_lib in critical_libs:
        if not any(crit_lib in lib for lib in available_libs):
            warnings.append(f"Missing critical library: {crit_lib}")
    if component_manifests:
        for manifest in component_manifests:
            component_name = manifest.get("name", "unknown")
            required_libs = manifest.get("requiredLibraries", [])
            interpreter = manifest.get("interpreter")
            if interpreter:
                interp_path = root / interpreter.lstrip("/")
                if not interp_path.exists():
                    errors.append(
                        f"[{component_name}] Missing ELF interpreter: {interpreter}"
                    )
            for lib in required_libs:
                lib_found = any(lib in avail for avail in available_libs)
                if not lib_found:
                    lib_in_rootfs = any(
                        lib in path for path in rootfs_libraries.keys()
                    )
                    if not lib_in_rootfs:
                        errors.append(
                            f"[{component_name}] Missing required library: {lib}"
                        )
    return {
        "valid": len(errors) == 0,
        "errors": errors,
        "warnings": warnings,
        "availableLibraries": sorted(available_libs),
        "rootfsLibraryCount": len(rootfs_libraries),
    }


def validate_elf_in_rootfs(rootfs_path: str, component_path: str) -> List[str]:
    errors = []
    comp_path = Path(component_path)
    if not comp_path.exists():
        return [f"Component not found: {component_path}"]
    elf_files = []
    if comp_path.is_file():
        elf_files.append(comp_path)
    else:
        for item in comp_path.rglob("*"):
            if item.is_file() and not item.is_symlink():
                try:
                    with open(item, "rb") as f:
                        if f.read(4) == ELF_MAGIC:
                            elf_files.append(item)
                except OSError:
                    pass
    for elf_file in elf_files[:20]:
        elf_info = parse_elf_dependencies(str(elf_file))
        if not elf_info.is_valid_elf:
            continue
        if elf_info.error:
            errors.append(f"ELF parse error in {elf_file}: {elf_info.error}")
    return errors


def parse_args():
    parser = argparse.ArgumentParser(description="Validate rootfs library compatibility")
    parser.add_argument("--rootfs", required=True, help="Path to rootfs directory")
    parser.add_argument("--component-manifest", help="Path to component dependency manifest JSON")
    parser.add_argument("--output", help="Output path for report JSON")
    return parser.parse_args()


def main():
    args = parse_args()
    component_manifests = None
    if args.component_manifest and os.path.exists(args.component_manifest):
        with open(args.component_manifest, "r", encoding="utf-8") as f:
            component_manifests = json.load(f)
        if isinstance(component_manifests, dict):
            component_manifests = [component_manifests]
    result = check_dependency_compatibility(args.rootfs, component_manifests)
    if args.output:
        with open(args.output, "w", encoding="utf-8", newline="") as f:
            json.dump(result, f, indent=2, ensure_ascii=False)
            f.write("\n")
    if result["valid"]:
        print("[PASS] Rootfs dependency validation passed")
        print(f"  Available libraries: {len(result['availableLibraries'])}")
        print(f"  Rootfs library count: {result['rootfsLibraryCount']}")
        if result["warnings"]:
            for warn in result["warnings"]:
                print(f"  [WARN] {warn}")
        return 0
    else:
        print(f"[FAIL] Rootfs dependency validation failed with {len(result['errors'])} errors:")
        for err in result["errors"]:
            print(f"  - {err}")
        return 1


if __name__ == "__main__":
    sys.exit(main())
