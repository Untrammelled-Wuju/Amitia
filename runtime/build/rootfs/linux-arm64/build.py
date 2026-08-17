#!/usr/bin/env python3
"""Canonical Rootfs Builder for Linux ARM64."""
import argparse
import hashlib
import json
import os
import shutil
import subprocess
import sys

SCRIPT_DIR = os.path.dirname(os.path.abspath(__file__))
BUILD_COMMON = os.path.join(SCRIPT_DIR, "..", "..", "common")
sys.path.insert(0, BUILD_COMMON)

from artifact_record import FrozenArtifactRecord, validate
from errors import BuildError
from atomic_publish import atomic_publish_dir
from tree_manifest import compute_tree_manifest, write_tree_manifest
from hashing import sha256_file

ROOTFS_SCRIPTS_DIR = os.path.join(SCRIPT_DIR, "..", "..", "..", "scripts", "prepare")
PREPARE_SH = os.path.join(ROOTFS_SCRIPTS_DIR, "prepare-ubuntu-rootfs-arm64.sh")
PREPARE_PS1 = os.path.join(ROOTFS_SCRIPTS_DIR, "prepare-ubuntu-rootfs-arm64.ps1")

OUTPUT_DIR_NAME = "rootfs"
FROZEN_RECORD_NAME = "rootfs-frozen-record.json"
ROOTFS_TARBALL_NAME = "rootfs.tar.xz"


def sha256_bytes(data):
    return hashlib.sha256(data).hexdigest()


def find_wsl():
    try:
        result = subprocess.run(["wsl.exe", "--help"], capture_output=True, timeout=5)
        return result.returncode == 0
    except (FileNotFoundError, subprocess.TimeoutExpired):
        return False


def build_rootfs(input_dir, output_root, cache_dir=None, staging_dir=None, dev_mode=False, skip_verify=False):
    if not os.path.isfile(PREPARE_SH):
        raise BuildError(f"Canonical rootfs prepare script not found: {PREPARE_SH}")

    staging = os.path.join(output_root, ".staging", OUTPUT_DIR_NAME)
    if os.path.exists(staging):
        shutil.rmtree(staging)
    os.makedirs(staging, exist_ok=True)

    try:
        if sys.platform == "win32":
            if not find_wsl():
                raise BuildError("WSL is not available. Rootfs build requires WSL on Windows.")
            cmd = ["wsl.exe", "bash", PREPARE_SH]
        else:
            cmd = ["bash", PREPARE_SH]

        if dev_mode:
            cmd.append("--dev-mode")
        if skip_verify:
            cmd.append("--skip-verify")
        if cache_dir:
            cmd.extend(["--cache-dir", cache_dir])
        if staging_dir:
            cmd.extend(["--staging-dir", staging_dir])
        cmd.extend(["--output-dir", staging])

        env = os.environ.copy()
        result = subprocess.run(cmd, env=env, capture_output=True, text=True)
        if result.returncode != 0:
            raise BuildError(f"Rootfs prepare failed:\n{result.stderr}")

        rootfs_tarball = os.path.join(staging, ROOTFS_TARBALL_NAME)
        if not os.path.isfile(rootfs_tarball):
            raise BuildError(f"Rootfs tarball not found: {rootfs_tarball}")

        artifact_sha = sha256_file(rootfs_tarball)
        tree_sha = artifact_sha

        record = FrozenArtifactRecord(
            componentId="rootfs",
            version="1.0.0",
            platform="linux",
            architecture="arm64",
            artifactType="archive",
            artifactRelativePath=ROOTFS_TARBALL_NAME,
            artifactSha256=artifact_sha,
            treeSha256=tree_sha,
            sourceRevision="unknown",
            buildMode="release",
        )
        validation_errors = validate(record)
        if validation_errors:
            raise BuildError(f"Rootfs record validation failed: {'; '.join(validation_errors)}")

        frozen_path = os.path.join(staging, FROZEN_RECORD_NAME)
        record.write(frozen_path)

        final_output = os.path.join(output_root, OUTPUT_DIR_NAME)
        if os.path.exists(final_output):
            shutil.rmtree(final_output)
        atomic_publish_dir(staging, final_output)

        return record

    finally:
        staging_parent = os.path.join(output_root, ".staging")
        if os.path.exists(staging_parent):
            shutil.rmtree(staging_parent, ignore_errors=True)


def main():
    parser = argparse.ArgumentParser(description="Rootfs Linux ARM64 Builder")
    parser.add_argument("--input", default=None, help="Input directory")
    parser.add_argument("--output", required=True, help="Output directory")
    parser.add_argument("--cache-dir", default=None, help="Cache directory")
    parser.add_argument("--staging-dir", default=None, help="Staging directory")
    parser.add_argument("--dev-mode", action="store_true", help="Enable dev mode")
    parser.add_argument("--skip-verify", action="store_true", help="Skip verification")
    args = parser.parse_args()

    if not args.output:
        print("Rootfs builder: ERROR - --output directory required", file=sys.stderr)
        return 1

    try:
        record = build_rootfs(
            args.input,
            args.output,
            cache_dir=args.cache_dir,
            staging_dir=args.staging_dir,
            dev_mode=args.dev_mode,
            skip_verify=args.skip_verify,
        )
        print(f"Rootfs builder: completed. Version: {record.version}")
        return 0
    except BuildError as e:
        print(f"Rootfs builder: BUILD ERROR - {e}", file=sys.stderr)
        return 1
    except Exception as e:
        print(f"Rootfs builder: UNEXPECTED ERROR - {e}", file=sys.stderr)
        return 2


if __name__ == "__main__":
    sys.exit(main())
