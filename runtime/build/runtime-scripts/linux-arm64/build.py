#!/usr/bin/env python3
"""Canonical Runtime Scripts Builder for Linux ARM64."""
import argparse
import hashlib
import json
import os
import shutil
import sys

SCRIPT_DIR = os.path.dirname(os.path.abspath(__file__))
BUILD_COMMON = os.path.join(SCRIPT_DIR, "..", "..", "common")
sys.path.insert(0, BUILD_COMMON)

from artifact_record import FrozenArtifactRecord, validate
from errors import BuildError
from atomic_publish import atomic_publish_dir
from tree_manifest import compute_tree_manifest, write_tree_manifest
from hashing import sha256_file

SCRIPTS_ROOT = os.path.join(SCRIPT_DIR, "..", "..", "..", "..", "scripts")
NODE_SCRIPTS_DIR = os.path.join(SCRIPTS_ROOT, "node")

OUTPUT_DIR_NAME = "scripts"
FROZEN_RECORD_NAME = "runtime-scripts-frozen-record.json"
NODE_DIR_NAME = "node"

REQUIRED_SCRIPTS = [
    "node/amitia-node-prepare.sh",
    "node/amitia-node-probe.sh",
]


def sha256_bytes(data):
    return hashlib.sha256(data).hexdigest()


def build_runtime_scripts(input_dir, output_root):
    if not os.path.isdir(NODE_SCRIPTS_DIR):
        raise BuildError(f"Node scripts directory not found: {NODE_SCRIPTS_DIR}")

    for script in REQUIRED_SCRIPTS:
        script_path = os.path.join(SCRIPTS_ROOT, script)
        if not os.path.isfile(script_path):
            raise BuildError(f"Required script not found: {script_path}")

    staging = os.path.join(output_root, ".staging", OUTPUT_DIR_NAME)
    if os.path.exists(staging):
        shutil.rmtree(staging)
    os.makedirs(staging, exist_ok=True)

    try:
        node_staging = os.path.join(staging, NODE_DIR_NAME)
        os.makedirs(node_staging, exist_ok=True)

        for script in REQUIRED_SCRIPTS:
            script_path = os.path.join(SCRIPTS_ROOT, script)
            script_basename = os.path.basename(script_path)
            dest = os.path.join(node_staging, script_basename)
            shutil.copy2(script_path, dest)
            os.chmod(dest, 0o755)

        tree_sha = compute_tree_manifest(staging)
        artifact_sha = tree_sha

        record = FrozenArtifactRecord(
            componentId="runtime-scripts",
            version="1.0.0",
            platform="linux",
            architecture="arm64",
            artifactType="tree",
            artifactRelativePath=OUTPUT_DIR_NAME,
            artifactSha256=artifact_sha,
            treeSha256=tree_sha,
            sourceRevision="unknown",
            buildMode="release",
        )
        validation_errors = validate(record)
        if validation_errors:
            raise BuildError(f"Runtime scripts record validation failed: {'; '.join(validation_errors)}")

        frozen_path = os.path.join(staging, FROZEN_RECORD_NAME)
        record.write(frozen_path)

        manifest_path = os.path.join(staging, "runtime-scripts-files.sha256")
        write_tree_manifest(staging, manifest_path)

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
    parser = argparse.ArgumentParser(description="Runtime Scripts Linux ARM64 Builder")
    parser.add_argument("--input", default=None, help="Input directory")
    parser.add_argument("--output", required=True, help="Output directory")
    args = parser.parse_args()

    if not args.output:
        print("Runtime scripts builder: ERROR - --output directory required", file=sys.stderr)
        return 1

    try:
        record = build_runtime_scripts(args.input, args.output)
        print(f"Runtime scripts builder: completed. Version: {record.version}")
        return 0
    except BuildError as e:
        print(f"Runtime scripts builder: BUILD ERROR - {e}", file=sys.stderr)
        return 1
    except Exception as e:
        print(f"Runtime scripts builder: UNEXPECTED ERROR - {e}", file=sys.stderr)
        return 2


if __name__ == "__main__":
    sys.exit(main())
