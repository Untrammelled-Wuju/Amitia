#!/usr/bin/env python3
"""Qdrant Frozen Artifact Adapter for Linux ARM64.

This module adapts existing Qdrant build artifacts to the unified
FrozenArtifactRecord interface consumed by the Runtime Package Builder.
"""
import hashlib
import json
import os
import sys

SCRIPT_DIR = os.path.dirname(os.path.abspath(__file__))
BUILD_COMMON = os.path.join(SCRIPT_DIR, "..", "..", "common")
sys.path.insert(0, BUILD_COMMON)

from artifact_record import FrozenArtifactRecord, validate
from errors import BuildError


def sha256_file(path):
    digest = hashlib.sha256()
    with open(path, "rb") as f:
        for chunk in iter(lambda: f.read(65536), b""):
            digest.update(chunk)
    return digest.hexdigest()


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


def resolve_qdrant_artifact(input_dir):
    if not input_dir or not os.path.isdir(input_dir):
        raise BuildError(f"Qdrant input directory not found: {input_dir}")

    candidates = []
    for fn in os.listdir(input_dir):
        full = os.path.join(input_dir, fn)
        if not os.path.isfile(full):
            continue
        if fn == "qdrant":
            candidates.append(full)
        elif fn.startswith("qdrant"):
            candidates.append(full)

    if not candidates:
        for root, _dirs, files in os.walk(input_dir):
            for fn in files:
                if fn == "qdrant":
                    candidates.append(os.path.join(root, fn))

    if not candidates:
        raise BuildError(f"No qdrant executable found in: {input_dir}")

    best = None
    for c in candidates:
        try:
            with open(c, "rb") as f:
                header = f.read(64)
            if probe_elf_aarch64(header):
                best = c
                break
        except OSError:
            continue

    if best is None:
        best = candidates[0]

    return best


def load_qdrant_build_record(input_dir):
    record_path = os.path.join(input_dir, "qdrant-build-record.json")
    if not os.path.exists(record_path):
        return None
    with open(record_path, "r", encoding="utf-8") as f:
        data = json.load(f)
    return data if isinstance(data, dict) else None


def create_qdrant_frozen_record(input_dir, source_revision=None, offline=False):
    artifact_path = resolve_qdrant_artifact(input_dir)

    artifact_sha = sha256_file(artifact_path)

    with open(artifact_path, "rb") as f:
        header = f.read(64)
    if not probe_elf_aarch64(header):
        raise BuildError(f"Qdrant binary is not ELF AArch64: {artifact_path}")

    build_record = load_qdrant_build_record(input_dir)

    version = "unknown"
    if build_record:
        version = build_record.get("version", version)

    if not source_revision:
        if build_record:
            source_revision = build_record.get("sourceRevision", source_revision)

    artifact_basename = os.path.basename(artifact_path)

    tree_sha = artifact_sha if artifact_sha else "0" * 64

    record = FrozenArtifactRecord(
        componentId="qdrant",
        version=version,
        platform="linux",
        architecture="arm64",
        artifactType="executable",
        artifactRelativePath=artifact_basename,
        artifactSha256=artifact_sha,
        treeSha256=tree_sha,
        sourceRevision=source_revision if source_revision else "unknown",
        buildMode="offline" if offline else "release",
    )

    validation_errors = validate(record)
    if validation_errors:
        raise BuildError(f"Qdrant record validation failed: {'; '.join(validation_errors)}")
    return record, artifact_path


def export_frozen_record(input_dir, output_path, source_revision=None, offline=False):
    record, artifact_path = create_qdrant_frozen_record(input_dir, source_revision=source_revision, offline=offline)

    out_dir = os.path.dirname(output_path)
    if out_dir:
        os.makedirs(out_dir, exist_ok=True)

    record.write(output_path)

    return record


def main():
    import argparse
    parser = argparse.ArgumentParser(description="Qdrant Frozen Adapter")
    parser.add_argument("--offline", action="store_true", help="Run in offline mode")
    parser.add_argument("--input", required=True, help="Input directory with qdrant artifact")
    parser.add_argument("--output", required=True, help="Output path for frozen record")
    parser.add_argument("--source-revision", default=None, help="Source revision")
    args = parser.parse_args()

    try:
        record = export_frozen_record(args.input, args.output, source_revision=args.source_revision, offline=args.offline)
        print(f"Qdrant frozen adapter: exported {record.componentId} v{record.version}")
        return 0
    except BuildError as e:
        print(f"Qdrant frozen adapter: ERROR - {e}", file=sys.stderr)
        return 1


if __name__ == "__main__":
    sys.exit(main())
