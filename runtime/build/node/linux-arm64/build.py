#!/usr/bin/env python3
"""Canonical Node Builder for Linux ARM64."""
import argparse
import hashlib
import json
import os
import shutil
import struct
import sys
import tarfile
import tempfile

SCRIPT_DIR = os.path.dirname(os.path.abspath(__file__))
BUILD_COMMON = os.path.join(SCRIPT_DIR, "..", "..", "common")
sys.path.insert(0, BUILD_COMMON)

from artifact_record import FrozenArtifactRecord, validate
from errors import BuildError
from safe_extract import safe_extract
from atomic_publish import atomic_publish_dir
from tree_manifest import compute_tree_manifest, write_tree_manifest
from hashing import sha256_file
from version_policy import same_version_gate

DEFAULT_LOCK_FILE = os.path.join(SCRIPT_DIR, "..", "..", "..", "artifacts", "node", "linux-arm64", "node-runtime-lock.json")

NODE_EXECUTABLE_RELPATH = "node/bin/node"
NPM_CLI_RELPATH = "lib/node_modules/npm/bin/npm-cli.js"
NPX_CLI_RELPATH = "lib/node_modules/npm/bin/npx-cli.js"
NODE_FILES_MANIFEST = "node-files.sha256"
NODE_BUILD_RECORD = "node-build-record.json"


def sha256_bytes(data):
    return hashlib.sha256(data).hexdigest()


def probe_elf_aarch64(header):
    if len(header) < 20:
        return False
    if header[0:4] != b"\x7fELF":
        return False
    if header[4] != 2:
        return False
    if header[5] != 1:
        return False
    e_type = int.from_bytes(header[16:18], "little")
    e_machine = int.from_bytes(header[18:20], "little")
    if e_type != 2:
        return False
    if e_machine != 183:
        return False
    return True


def load_lock(lock_path):
    if not os.path.exists(lock_path):
        raise BuildError(f"Node runtime lock file not found: {lock_path}")
    with open(lock_path, "r", encoding="utf-8") as f:
        data = json.load(f)
    required_fields = ["version", "platform", "architecture", "archiveFileName", "sourceUrl", "sha256", "installSubdir"]
    for field in required_fields:
        if field not in data:
            raise BuildError(f"Lock file missing required field: {field}")
    if data["platform"] != "linux":
        raise BuildError(f"Invalid platform in lock: {data['platform']}, expected linux")
    if data["architecture"] != "arm64":
        raise BuildError(f"Invalid architecture in lock: {data['architecture']}, expected arm64")
    return data


def verify_node_elf(node_path):
    with open(node_path, "rb") as f:
        header = f.read(64)
    if not probe_elf_aarch64(header):
        raise BuildError(f"Node binary is not ELF AArch64: {node_path}")


def validate_node_tree(node_root):
    node_bin = os.path.join(node_root, NODE_EXECUTABLE_RELPATH)
    if not os.path.exists(node_bin):
        raise BuildError(f"Node executable not found at expected path: {NODE_EXECUTABLE_RELPATH}")

    verify_node_elf(node_bin)

    npm_cli = os.path.join(node_root, NPM_CLI_RELPATH)
    if not os.path.exists(npm_cli):
        raise BuildError(f"npm CLI not found at expected path: {NPM_CLI_RELPATH}")

    npx_cli = os.path.join(node_root, NPX_CLI_RELPATH)
    if not os.path.exists(npx_cli):
        raise BuildError(f"npx CLI not found at expected path: {NPX_CLI_RELPATH}")

    return True


def build_node(input_dir, output_dir, lock_path=None):
    lock_path = lock_path or DEFAULT_LOCK_FILE
    lock = load_lock(lock_path)

    version = lock["version"]
    archive_name = lock["archiveFileName"]
    expected_sha = lock["sha256"]
    install_subdir = lock["installSubdir"]

    archive_path = None
    if input_dir:
        candidate = os.path.join(input_dir, archive_name)
        if os.path.exists(candidate):
            archive_path = candidate

    if not archive_path:
        raise BuildError(f"Node archive not found: {archive_name}. Provide via --input directory.")

    actual_sha = sha256_file(archive_path)
    if actual_sha != expected_sha:
        raise BuildError(f"Node archive SHA mismatch: expected {expected_sha}, got {actual_sha}")

    staging = os.path.join(output_dir, ".staging")
    if os.path.exists(staging):
        shutil.rmtree(staging)
    os.makedirs(staging, exist_ok=True)

    try:
        safe_extract(archive_path, staging)

        node_root = os.path.join(staging, install_subdir)
        if not os.path.isdir(node_root):
            for name in os.listdir(staging):
                candidate = os.path.join(staging, name)
                if os.path.isdir(candidate) and os.path.exists(os.path.join(candidate, "bin", "node")):
                    node_root = candidate
                    break

        validate_node_tree(node_root)

        node_files_sha = compute_tree_manifest(node_root)

        record = {
            "schemaVersion": 1,
            "component": "node",
            "version": version,
            "platform": "linux",
            "architecture": "arm64",
            "source": {
                "url": lock["sourceUrl"],
                "archiveFileName": archive_name,
                "expectedSha256": expected_sha,
                "actualSha256": actual_sha,
            },
            "runtime": {
                "nodePath": NODE_EXECUTABLE_RELPATH,
                "npmPath": "node/bin/npm",
                "npxPath": "node/bin/npx",
            },
            "validation": {
                "staticValidation": "PASS",
                "executionValidation": "NOT_EXECUTED",
            },
            "treeSha256": node_files_sha,
            "frozenRoot": "node",
        }

        if os.path.isdir(node_root) and os.path.dirname(node_root) == staging:
            final_node_dir = os.path.join(staging, "node")
            if node_root != final_node_dir:
                shutil.move(node_root, final_node_dir)
                node_root = final_node_dir

        manifest_path = os.path.join(staging, NODE_FILES_MANIFEST)
        write_tree_manifest(node_root, manifest_path)

        record_path = os.path.join(staging, NODE_BUILD_RECORD)
        with open(record_path, "w", encoding="utf-8") as f:
            json.dump(record, f, indent=2, sort_keys=True)
            f.write("\n")

        tree_record = FrozenArtifactRecord(
            componentId="node",
            version=version,
            platform="linux",
            architecture="arm64",
            artifactType="tree",
            artifactRelativePath="node",
            artifactSha256=node_files_sha,
            treeSha256=node_files_sha,
            sourceRevision=lock.get("sourceUrl", ""),
            buildMode="release",
        )
        validation_errors = validate(tree_record)
        if validation_errors:
            raise BuildError(f"Node record validation failed: {'; '.join(validation_errors)}")
        frozen_path = os.path.join(staging, "node-frozen-record.json")
        tree_record.write(frozen_path)

        same_version_gate(output_dir, tree_record.componentId, tree_record.version, tree_record.treeSha256, node_files_sha)
        if os.path.exists(output_dir):
            shutil.rmtree(output_dir)
        atomic_publish_dir(staging, output_dir)

        return record

    finally:
        if os.path.exists(staging):
            shutil.rmtree(staging, ignore_errors=True)


def main():
    parser = argparse.ArgumentParser(description="Node Linux ARM64 Builder")
    parser.add_argument("--input", default=None, help="Input directory with Node archive")
    parser.add_argument("--output", required=True, help="Output directory")
    parser.add_argument("--lock", default=None, help="Path to node-runtime-lock.json")
    args = parser.parse_args()

    if not args.output:
        print("Node builder: ERROR - --output directory required", file=sys.stderr)
        return 1

    try:
        print(f"Node builder: building Node runtime")
        record = build_node(args.input, args.output, lock_path=args.lock)
        print(f"Node builder: completed. Version: {record['version']}")
        return 0
    except BuildError as e:
        print(f"Node builder: BUILD ERROR - {e}", file=sys.stderr)
        return 1
    except Exception as e:
        print(f"Node builder: UNEXPECTED ERROR - {e}", file=sys.stderr)
        return 2


if __name__ == "__main__":
    sys.exit(main())
