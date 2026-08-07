import argparse
import json
import os
import shutil
import hashlib
import subprocess
import sys
import urllib.request
from pathlib import Path

SCRIPT_DIR = Path(__file__).parent
PROJECT_ROOT = SCRIPT_DIR.parent.parent.parent
ANDROID_MODULE_JNI_LIBS = PROJECT_ROOT / "mobile_app" / "android" / "amitia-runtime" / "src" / "main" / "jniLibs"
ANDROID_MODULE_RES_RAW = PROJECT_ROOT / "mobile_app" / "android" / "amitia-runtime" / "src" / "main" / "res" / "raw"
OUTPUT_DIR = SCRIPT_DIR.parent.parent / "out" / "proot" / "android-arm64"
SOURCE_DIR = OUTPUT_DIR / "source"
CACHE_DIR = OUTPUT_DIR / "cache"


def calculate_sha256(file_path: Path) -> str:
    h = hashlib.sha256()
    with open(file_path, "rb") as f:
        for chunk in iter(lambda: f.read(8192), b""):
            h.update(chunk)
    return h.hexdigest()


def load_lock() -> dict:
    lock_file = SCRIPT_DIR / "proot.lock.json"
    with open(lock_file, "r", encoding="utf-8") as f:
        return json.load(f)


def clean():
    if SOURCE_DIR.exists():
        shutil.rmtree(SOURCE_DIR, ignore_errors=True)
    if CACHE_DIR.exists():
        shutil.rmtree(CACHE_DIR, ignore_errors=True)
    for f in OUTPUT_DIR.glob("*"):
        if f.is_file() and f.name not in ("proot.lock.json", "update_lock.py", "prepare_source.py", "build.py", "verify.py", "test_build.py", "README.md", ".gitignore"):
            f.unlink()
        elif f.is_dir() and f.name not in ("patches", "licenses"):
            shutil.rmtree(f, ignore_errors=True)


def download_source(lock: dict, cache_dir: Path) -> Path:
    upstream = lock["upstream"]
    commit = upstream["commit"]
    url = f"https://github.com/proot-me/proot/archive/{commit}.tar.gz"
    archive_path = cache_dir / f"proot-upstream-{commit[:12]}.tar.gz"

    if archive_path.exists():
        actual_sha = calculate_sha256(archive_path)
        expected_sha = upstream.get("archiveSha256", "")
        if expected_sha and not expected_sha.startswith("placeholder") and actual_sha != expected_sha:
            print(f"[build] Archive SHA mismatch: expected {expected_sha}, got {actual_sha}")
            sys.exit(1)
        return archive_path

    print(f"[build] Downloading: {url}")
    cache_dir.mkdir(parents=True, exist_ok=True)
    try:
        urllib.request.urlretrieve(url, archive_path)
    except Exception as e:
        print(f"[build] Download failed: {e}")
        sys.exit(1)

    actual_sha = calculate_sha256(archive_path)
    expected_sha = upstream.get("archiveSha256", "")
    if expected_sha and not expected_sha.startswith("placeholder") and actual_sha != expected_sha:
        print(f"[build] Archive SHA mismatch")
        archive_path.unlink()
        sys.exit(1)

    return archive_path


def extract_and_patch(archive_path: Path, lock: dict) -> Path:
    import tarfile
    source_dir = SOURCE_DIR
    source_dir.mkdir(parents=True, exist_ok=True)

    print(f"[build] Extracting: {archive_path}")
    with tarfile.open(archive_path, "r:gz") as tar:
        members = tar.getmembers()
        root_dirs = set(m.name.split("/")[0] for m in members)
        if len(root_dirs) == 1:
            root = root_dirs.pop()
        else:
            root = None
        tar.extractall(source_dir)

    if root:
        actual_src = source_dir / root
        if actual_src.is_dst_dir():
            for item in actual_src.iterdir():
                target = source_dir / item.name
                if not target.exists():
                    item.rename(target)
            actual_src.rmdir()
            return source_dir
        return actual_src

    return source_dir


def build_with_ndk(source_dir: Path, ndk_path: Path, output_dir: Path, lock: dict) -> Path:
    target = lock["target"]
    abi = target["abi"]
    api = target["apiLevel"]

    clang = ndk_path / "toolchains" / "llvm" / "prebuilt" / get_ndk_host() / "bin" / "clang"
    if not clang.exists():
        print(f"[build] NDK clang not found: {clang}")
        sys.exit(1)

    output_elf = output_dir / "libamitia_proot.so"
    output_dir.mkdir(parents=True, exist_ok=True)

    source_files = ["src/proot.c", "src/path/path.c", "src/path/binding.c", "src/path/proc.c", "src/ptrace/ptrace.c", "ptrace/wait.c", "src/execve/execve.c", "src/execve/elf.c", "src/vfs/vfs.c"]
    existing_sources = [str(source_dir / sf) for sf in source_files if (source_dir / sf).exists()]

    if not existing_sources:
        print(f"[build] No PRoot source files found in {source_dir / 'src'}")
        sys.exit(1)

    cmd = [
        str(clang),
        f"--target=aarch64-linux-android{api}",
        f"--sysroot={ndk_path / 'toolchains' / 'llvm' / 'prebuilt' / get_ndk_host() / 'sysroot'}",
        "-fPIC",
        "-pie",
        "-fPIE",
        "-o", str(output_elf),
        "-static",
        "-DHAVE_LINUX_PTRACE_H=1",
    ] + existing_sources

    print(f"[build] Building PRoot for {abi}: {' '.join(cmd)}")
    result = subprocess.run(cmd, capture_output=True, text=True)
    if result.returncode != 0:
        print(f"[build] Build failed: {result.stderr}")
        sys.exit(1)

    return output_elf


def get_ndk_host() -> str:
    import platform
    sys_name = platform.system().lower()
    if sys_name == "windows":
        return "windows-x86_64"
    if sys_name == "darwin":
        return "darwin-x86_64"
    return "linux-x86_64"


def copy_to_android_module(elf_path: Path, lock: dict):
    abi = lock["target"]["abi"]
    jni_dir = ANDROID_MODULE_JNI_LIBS / abi
    jni_dir.mkdir(parents=True, exist_ok=True)
    target = jni_dir / elf_path.name
    shutil.copy2(elf_path, target)
    print(f"[build] Installed: {target}")

    metadata = {
        "schemaVersion": 1,
        "componentId": lock["componentId"],
        "name": "proot",
        "version": lock["version"],
        "abi": abi,
        "architecture": lock["target"]["architecture"],
        "fileName": elf_path.name,
        "sha256": calculate_sha256(elf_path),
        "license": lock["license"]["spdx"],
        "source": {
            "upstreamTag": lock["upstream"]["tag"],
            "upstreamCommit": lock["upstream"]["commit"],
            "androidPatchCommit": lock["androidCompatibility"]["commit"]
        }
    }

    ANDROID_MODULE_RES_RAW.mkdir(parents=True, exist_ok=True)
    metadata_path = ANDROID_MODULE_RES_RAW / "proot_artifact.json"
    with open(metadata_path, "w", encoding="utf-8") as f:
        json.dump(metadata, f, indent=2)
    print(f"[build] Metadata: {metadata_path}")

    license_src = SCRIPT_DIR / "licenses" / "COPYING-PROOT"
    if license_src.exists():
        license_dst = ANDROID_MODULE_RES_RAW / "proot_copying.txt"
        shutil.copy2(license_src, license_dst)
        print(f"[build] License: {license_dst}")


def main():
    parser = argparse.ArgumentParser(description="Build PRoot for Android ARM64")
    parser.add_argument("--clean", action="store_true")
    parser.add_argument("--offline", action="store_true")
    parser.add_argument("--ndk", default=None)
    parser.add_argument("--source-dir", default=None)
    parser.add_argument("--cache-dir", default=None)
    parser.add_argument("--output-dir", default=None)
    parser.add_argument("--install-android-module", action="store_true")

    args = parser.parse_args()

    if args.clean:
        clean()
        print("[build] Clean completed")
        return

    lock = load_lock()

    if any(v.startswith("placeholder") for v in lock.get("upstream", {}).values() if isinstance(v, str)):
        print("[build] LOCK FILE CONTAINS PLACEHOLDERS")
        print("[build] Run update_lock.py first to populate real values")
        sys.exit(1)

    ndk_path = Path(args.ndk) if args.ndk else find_ndk()
    if not ndk_path:
        print("[build] NDK not found. Use --ndk <path>")
        sys.exit(1)

    OUTPUT_DIR.mkdir(parents=True, exist_ok=True)
    CACHE_DIR.mkdir(parents=True, exist_ok=True)

    print(f"[build] PRoot {lock['version']} for {lock['target']['abi']}")
    print(f"[build] NDK: {ndk_path}")

    archive = download_source(lock, CACHE_DIR)
    source = extract_and_patch(archive, lock)
    elf = build_with_ndk(source, ndk_path, OUTPUT_DIR, lock)

    print(f"[build] Built: {elf}")
    print(f"[build] SHA256: {calculate_sha256(elf)}")

    if args.install_android_module:
        copy_to_android_module(elf, lock)

        with open(SCRIPT_DIR / "proot.lock.json", "r+", encoding="utf-8") as f:
            final_lock = json.load(f)
            final_lock["artifact"]["sha256"] = calculate_sha256(elf)
            f.seek(0)
            json.dump(final_lock, f, indent=2, ensure_ascii=False)
            f.truncate()

    print("[build] SUCCESS")


def find_ndk() -> Path | None:
    ndk_home = os.environ.get("ANDROID_NDK_HOME") or os.environ.get("NDK_HOME")
    if ndk_home:
        return Path(ndk_home)
    sdk_home = os.environ.get("ANDROID_HOME") or os.environ.get("ANDROID_SDK_ROOT")
    if sdk_home:
        ndk_dir = Path(sdk_home) / "ndk"
        if ndk_dir.exists():
            versions = sorted(ndk_dir.iterdir(), reverse=True)
            for v in versions:
                if v.is_dir():
                    return v
    return None


if __name__ == "__main__":
    main()
