#!/usr/bin/env python3
"""Verify the external mock plugin consumes the current canonical TS Game Plugin SDK.

The mock plugin intentionally installs from a checked-in tarball so the external
E2E proves a third-party package boundary. This gate prevents that tarball (or
its package-lock SRI) from silently drifting behind the canonical SDK source.
"""
from __future__ import annotations

import base64
import hashlib
import json
import sys
import tarfile
import tempfile
from pathlib import Path

ROOT = Path(__file__).resolve().parents[1]
CANONICAL = ROOT / "backend/pkg/gameplugin/sdk/game-plugin"
CANONICAL_TGZ = CANONICAL / "amitia-game-plugin-sdk-0.1.0.tgz"
PLUGIN = ROOT / "testplugins/mock-amitiax-game-plugin"
VENDOR_TGZ = PLUGIN / "vendor/amitia-game-plugin-sdk-0.1.0.tgz"
LOCK = PLUGIN / "package-lock.json"
PACKAGE_KEY = "node_modules/@amitia/game-plugin-sdk"


def fail(message: str) -> None:
    print(f"ERROR: {message}", file=sys.stderr)
    raise SystemExit(1)


def file_map(root: Path) -> dict[str, bytes]:
    return {
        path.relative_to(root).as_posix(): path.read_bytes()
        for path in sorted(root.rglob("*"))
        if path.is_file()
    }


def safe_extract(archive: Path, destination: Path) -> None:
    with tarfile.open(archive, "r:gz") as tf:
        for member in tf.getmembers():
            name = member.name.replace("\\", "/")
            if name.startswith("/") or name == ".." or name.startswith("../") or "/../" in name:
                fail(f"unsafe path in vendored SDK archive: {member.name}")
        tf.extractall(destination, filter="data")


def main() -> None:
    for path in (CANONICAL_TGZ, VENDOR_TGZ, LOCK, CANONICAL / "package.json", CANONICAL / "src"):
        if not path.exists():
            fail(f"required SDK sync input is missing: {path.relative_to(ROOT)}")

    if CANONICAL_TGZ.read_bytes() != VENDOR_TGZ.read_bytes():
        fail("mock plugin vendor SDK tarball differs from backend canonical SDK tarball")

    lock = json.loads(LOCK.read_text(encoding="utf-8"))
    package_record = lock.get("packages", {}).get(PACKAGE_KEY, {})
    expected_sri = str(package_record.get("integrity", "")).strip()
    actual_sri = "sha512-" + base64.b64encode(hashlib.sha512(VENDOR_TGZ.read_bytes()).digest()).decode("ascii")
    if expected_sri != actual_sri:
        fail(f"mock plugin package-lock SDK integrity drift: expected {actual_sri}, found {expected_sri or '<empty>'}")

    with tempfile.TemporaryDirectory(prefix="amitia-game-sdk-vendor-") as tmp:
        extract_root = Path(tmp)
        safe_extract(VENDOR_TGZ, extract_root)
        package_root = extract_root / "package"
        packaged_src = package_root / "src"
        packaged_dist = package_root / "dist"
        packaged_manifest = package_root / "package.json"
        packaged_readme = package_root / "README.md"
        canonical_dist = CANONICAL / "dist"
        canonical_readme = CANONICAL / "README.md"
        if not packaged_src.is_dir() or not packaged_dist.is_dir() or not packaged_manifest.is_file() or not packaged_readme.is_file():
            fail("vendored SDK tarball is missing package/src, package/dist, package/package.json, or package/README.md")
        if not canonical_dist.is_dir():
            fail("canonical SDK dist/ is missing; build the TypeScript SDK before verifying the vendored package")
        if not canonical_readme.is_file():
            fail("canonical TypeScript SDK README.md is missing")

        canonical_src = file_map(CANONICAL / "src")
        vendor_src = file_map(packaged_src)
        if canonical_src.keys() != vendor_src.keys():
            missing = sorted(canonical_src.keys() - vendor_src.keys())
            extra = sorted(vendor_src.keys() - canonical_src.keys())
            fail(f"vendored SDK source file set drift: missing={missing} extra={extra}")
        for rel, content in canonical_src.items():
            if vendor_src[rel] != content:
                fail(f"vendored SDK source differs from canonical source: src/{rel}")

        canonical_dist_files = file_map(canonical_dist)
        vendor_dist_files = file_map(packaged_dist)
        if canonical_dist_files.keys() != vendor_dist_files.keys():
            missing = sorted(canonical_dist_files.keys() - vendor_dist_files.keys())
            extra = sorted(vendor_dist_files.keys() - canonical_dist_files.keys())
            fail(f"vendored SDK dist file set drift: missing={missing} extra={extra}")
        for rel, content in canonical_dist_files.items():
            if vendor_dist_files[rel] != content:
                fail(f"vendored SDK compiled output differs from canonical build: dist/{rel}")

        if packaged_readme.read_bytes() != canonical_readme.read_bytes():
            fail("vendored SDK README.md differs from canonical README.md")

        canonical_package = json.loads((CANONICAL / "package.json").read_text(encoding="utf-8"))
        vendor_package = json.loads(packaged_manifest.read_text(encoding="utf-8"))
        if canonical_package != vendor_package:
            fail("vendored SDK package.json differs from canonical package.json")

    print("mock Game Plugin TypeScript SDK vendor is synchronized")


if __name__ == "__main__":
    main()
