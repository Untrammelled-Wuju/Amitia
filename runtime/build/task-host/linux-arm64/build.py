#!/usr/bin/env python3
"""Canonical Task Host Builder for Linux ARM64."""
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
from version_policy import same_version_gate

TASK_HOST_ROOT = os.path.join(SCRIPT_DIR, "..", "..", "..", "task-host")
SRC_DIR = os.path.join(TASK_HOST_ROOT, "src")
TS_CONFIG = os.path.join(TASK_HOST_ROOT, "tsconfig.json")
PACKAGE_JSON = os.path.join(TASK_HOST_ROOT, "package.json")

OUTPUT_DIR_NAME = "task-host"
FROZEN_RECORD_NAME = "task-host-frozen-record.json"
NODE_MODULES_DIR = "node_modules"


def find_node_binary(output_root):
    node_candidate = os.path.join(output_root, "node", "bin", "node")
    if os.path.isfile(node_candidate):
        return node_candidate
    node_candidate = os.path.join(output_root, "..", "node", "linux-arm64", "node", "bin", "node")
    if os.path.isfile(node_candidate):
        return node_candidate
    return None


def find_tsc(node_bin, task_host_dir):
    if node_bin:
        node_dir = os.path.dirname(node_bin)
        local_tsc = os.path.join(node_dir, "..", "lib", "node_modules", "typescript", "bin", "tsc")
        if os.path.isfile(local_tsc):
            return local_tsc
    local_npm_modules = os.path.join(task_host_dir, NODE_MODULES_DIR, "typescript", "bin", "tsc")
    if os.path.isfile(local_npm_modules):
        return local_npm_modules
    return None


def build_task_host(input_dir, output_root, node_bin=None):
    if not os.path.isdir(SRC_DIR):
        raise BuildError(f"Task host source directory not found: {SRC_DIR}")
    if not os.path.isfile(TS_CONFIG):
        raise BuildError(f"tsconfig.json not found: {TS_CONFIG}")
    if not os.path.isfile(PACKAGE_JSON):
        raise BuildError(f"package.json not found: {PACKAGE_JSON}")

    staging = os.path.join(output_root, ".staging", OUTPUT_DIR_NAME)
    if os.path.exists(staging):
        shutil.rmtree(staging)
    os.makedirs(staging, exist_ok=True)

    try:
        for fn in os.listdir(SRC_DIR):
            src = os.path.join(SRC_DIR, fn)
            dst = os.path.join(staging, fn)
            if os.path.isfile(src):
                shutil.copy2(src, dst)

        shutil.copy2(TS_CONFIG, os.path.join(staging, "tsconfig.json"))
        shutil.copy2(PACKAGE_JSON, os.path.join(staging, "package.json"))

        if node_bin is None:
            node_bin = find_node_binary(output_root)
        if node_bin is None:
            raise BuildError("Node binary not found. Provide --node-bin or ensure node build is available.")

        tsc_path = find_tsc(node_bin, staging)
        if tsc_path is None:
            raise BuildError("TypeScript compiler (tsc) not found. Ensure typescript is installed.")

        env = os.environ.copy()
        env["PATH"] = os.path.dirname(node_bin) + os.pathsep + env.get("PATH", "")

        compile_cmd = [node_bin, tsc_path, "--project", os.path.join(staging, "tsconfig.json")]
        result = subprocess.run(compile_cmd, cwd=staging, env=env, capture_output=True, text=True)
        if result.returncode != 0:
            raise BuildError(f"TypeScript compilation failed:\n{result.stderr}")

        index_js = os.path.join(staging, "dist", "index.js")
        if not os.path.isfile(index_js):
            raise BuildError(f"Expected output not found: dist/index.js")

        tree_sha = compute_tree_manifest(staging)
        artifact_sha = sha256_file(index_js)

        record = FrozenArtifactRecord(
            componentId="task-host",
            version=_read_version(),
            platform="linux",
            architecture="arm64",
            artifactType="tree",
            artifactRelativePath=OUTPUT_DIR_NAME,
            artifactSha256=artifact_sha,
            treeSha256=tree_sha,
            sourceRevision=_read_source_revision(),
            buildMode="release",
        )
        validation_errors = validate(record)
        if validation_errors:
            raise BuildError(f"Task host record validation failed: {'; '.join(validation_errors)}")

        frozen_path = os.path.join(staging, FROZEN_RECORD_NAME)
        record.write(frozen_path)

        manifest_path = os.path.join(staging, "task-host-files.sha256")
        write_tree_manifest(staging, manifest_path)

        final_output = os.path.join(output_root, OUTPUT_DIR_NAME)
        same_version_gate(output_root, record.componentId, record.version, record.treeSha256, tree_sha)
        atomic_publish_dir(staging, final_output)

        return record

    finally:
        staging_parent = os.path.join(output_root, ".staging")
        if os.path.exists(staging_parent):
            shutil.rmtree(staging_parent, ignore_errors=True)


def _read_version():
    try:
        with open(PACKAGE_JSON, "r", encoding="utf-8") as f:
            pkg = json.load(f)
        return pkg.get("version", "0.0.0")
    except (OSError, json.JSONDecodeError):
        return "0.0.0"


def _read_source_revision():
    return "unknown"


def main():
    parser = argparse.ArgumentParser(description="Task Host Linux ARM64 Builder")
    parser.add_argument("--input", default=None, help="Input directory")
    parser.add_argument("--output", required=True, help="Output directory")
    parser.add_argument("--node-bin", default=None, help="Path to node binary")
    args = parser.parse_args()

    if not args.output:
        print("Task host builder: ERROR - --output directory required", file=sys.stderr)
        return 1

    try:
        record = build_task_host(args.input, args.output, node_bin=args.node_bin)
        print(f"Task host builder: completed. Version: {record.version}")
        return 0
    except BuildError as e:
        print(f"Task host builder: BUILD ERROR - {e}", file=sys.stderr)
        return 1
    except Exception as e:
        print(f"Task host builder: UNEXPECTED ERROR - {e}", file=sys.stderr)
        return 2


if __name__ == "__main__":
    sys.exit(main())
