import json
import hashlib
import struct
import sys
from pathlib import Path

SCRIPT_DIR = Path(__file__).parent
OUTPUT_DIR = SCRIPT_DIR.parent.parent / "out" / "proot" / "android-arm64"
LICENSE_FILE = SCRIPT_DIR / "licenses" / "COPYING-PROOT"

ELF_MAGIC = b"\x7fELF"
ELF64 = 2
ELF_LITTLE_ENDIAN = 1
EM_AARCH64 = 183
ET_DYN = 3
ET_EXEC = 2


def calculate_sha256(file_path: Path) -> str:
    h = hashlib.sha256()
    with open(file_path, "rb") as f:
        for chunk in iter(lambda: f.read(8192), b""):
            h.update(chunk)
    return h.hexdigest()


def inspect_elf(file_path: Path) -> dict:
    with open(file_path, "rb") as f:
        header = f.read(64)
        if len(header) < 52:
            return {"valid": False, "reason": "file too short"}

        magic = header[:4]
        if magic != ELF_MAGIC:
            return {"valid": False, "reason": "not ELF"}

        ei_class = header[4]
        if ei_class != ELF64:
            return {"valid": False, "reason": f"not ELF64 (class={ei_class})"}

        ei_data = header[5]
        if ei_data != ELF_LITTLE_ENDIAN:
            return {"valid": False, "reason": "not little-endian"}

        machine = struct.unpack_from("<H", header, 16)[0]
        if machine != EM_AARCH64:
            return {"valid": False, "reason": f"wrong arch (machine={machine})"}

        e_type = struct.unpack_from("<H", header, 16)[0]
        e_entry = struct.unpack_from("<Q", header, 24)[0]

        if e_entry == 0:
            return {"valid": False, "reason": "entry point is zero"}

        return {
            "valid": True,
            "class": ei_class,
            "endian": "little" if ei_data == ELF_LITTLE_ENDIAN else "big",
            "machine": machine,
            "machine_name": "AArch64" if machine == EM_AARCH64 else f"unknown({machine})",
            "type": e_type,
            "entry": e_entry
        }


def verify_artifact(lock: dict) -> bool:
    artifact_name = lock["artifact"]["name"]
    expected_sha = lock["artifact"]["sha256"]
    artifact_path = OUTPUT_DIR / artifact_name

    if not artifact_path.exists():
        print(f"[verify] FAIL: {artifact_name} not found")
        return False

    actual_sha = calculate_sha256(artifact_path)
    if not expected_sha.startswith("placeholder") and actual_sha != expected_sha:
        print(f"[verify] FAIL: SHA mismatch")
        return False

    elf_info = inspect_elf(artifact_path)
    if not elf_info.get("valid"):
        print(f"[verify] FAIL: Invalid ELF: {elf_info.get('reason')}")
        return False

    if elf_info["machine"] != EM_AARCH64:
        print(f"[verify] FAIL: Wrong architecture")
        return False

    if elf_info["class"] != ELF64:
        print(f"[verify] FAIL: Not ELF64")
        return False

    return True


def main():
    lock_file = SCRIPT_DIR / "proot.lock.json"
    if not lock_file.exists():
        print("[verify] FAIL: proot.lock.json not found")
        sys.exit(1)

    with open(lock_file, "r", encoding="utf-8") as f:
        lock = json.load(f)

    if not verify_artifact(lock):
        print("[verify] FAILED")
        sys.exit(1)

    print("[verify] PASSED")


if __name__ == "__main__":
    main()
