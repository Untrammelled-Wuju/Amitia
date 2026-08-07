import os
import sys
import shutil
import tarfile
from pathlib import Path

SCRIPT_DIR = Path(__file__).parent
BUILD_DIR = SCRIPT_DIR.parent.parent / "out" / "proot" / "android-arm64"
SOURCE_DIR = BUILD_DIR / "source"
CACHE_DIR = BUILD_DIR / "cache"


def clean():
    if SOURCE_DIR.exists():
        shutil.rmtree(SOURCE_DIR, ignore_errors=True)
    if CACHE_DIR.exists():
        shutil.rmtree(CACHE_DIR, ignore_errors=True)


def download_upstream(url: str, dest: Path):
    import urllib.request
    print(f"[prepare_source] Downloading: {url}")
    urllib.request.urlretrieve(url, dest)


def extract_source(archive_path: Path, dest: Path):
    print(f"[prepare_source] Extracting: {archive_path}")
    with tarfile.open(archive_path, "r:gz") as tar:
        tar.extractall(dest)


def apply_patches(source_dir: Path, patch_dir: Path):
    import subprocess
    patches = sorted(patch_dir.glob("*.patch"))
    for patch in patches:
        print(f"[prepare_source] Applying: {patch.name}")
        result = subprocess.run(
            ["git", "apply", "--check", str(patch)],
            cwd=source_dir,
            capture_output=True
        )
        if result.returncode != 0:
            print(f"[prepare_source] Patch check failed: {patch.name}")
            return False
        subprocess.run(
            ["git", "apply", str(patch)],
            cwd=source_dir,
            check=True
        )
    return True


def main():
    offline = "--offline" in sys.argv
    clean_only = "--clean" in sys.argv
    source_override = None
    cache_override = None

    for arg in sys.argv[1:]:
        if arg.startswith("--source-dir="):
            source_override = Path(arg.split("=", 1)[1])
        if arg.startswith("--cache-dir="):
            cache_override = Path(arg.split("=", 1)[1])

    src = source_override or SOURCE_DIR
    cache = cache_override or CACHE_DIR

    if clean_only:
        clean()
        print("[prepare_source] Cleaned source and cache")
        return

    if offline:
        if not src.exists():
            print("[prepare_source] Offline mode: source dir missing")
            sys.exit(1)
        print("[prepare_source] Offline mode: using existing source")
        return


if __name__ == "__main__":
    main()
