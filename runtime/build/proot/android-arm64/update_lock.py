import json
import hashlib
import os
import sys
from pathlib import Path

LOCK_FILE = Path(__file__).parent / "proot.lock.json"
UPSTREAM_REPO = "https://github.com/proot-me/proot"
UPSTREAM_TAG = "v5.4.0"
TERMUX_REPO = "https://github.com/termux/proot"
ANDROID_PATCH_DIR = Path(__file__).parent / "patches"


def calculate_sha256(file_path: Path) -> str:
    h = hashlib.sha256()
    with open(file_path, "rb") as f:
        for chunk in iter(lambda: f.read(8192), b""):
            h.update(chunk)
    return h.hexdigest()


def load_lock() -> dict:
    if not LOCK_FILE.exists():
        return {}
    with open(LOCK_FILE, "r", encoding="utf-8-sig") as f:
        return json.load(f)


def save_lock(data: dict):
    ordered = dict(sorted(data.items()))
    with open(LOCK_FILE, "w", encoding="utf-8") as f:
        json.dump(ordered, f, indent=2, ensure_ascii=False)
        f.write("\n")


def main():
    print(f"[update_lock] Updating PRoot lock file")
    print(f"[update_lock] Upstream: {UPSTREAM_REPO} @ {UPSTREAM_TAG}")
    print(f"[update_lock] Android compat: {TERMUX_REPO}")
    print(f"[update_lock] WARNING: This script requires network access")
    print(f"[update_lock] Place commit SHA values after first run")

    lock = load_lock()
    placeholder_values = [v for v in lock.values() if isinstance(v, str) and v.startswith("placeholder")]

    if placeholder_values:
        print(f"[update_lock] Found {len(placeholder_values)} placeholder values")
        print("[update_lock] Run this script with network access to resolve real values")
        print("[update_lock] Commit SHA values must be manually verified and updated")
        sys.exit(1)

    print("[update_lock] All values locked")
    print(f"[update_lock] Upstream commit: {lock.get('upstream', {}).get('commit', 'MISSING')}")
    print(f"[update_lock] Patch commit: {lock.get('androidCompatibility', {}).get('commit', 'MISSING')}")


if __name__ == "__main__":
    main()
