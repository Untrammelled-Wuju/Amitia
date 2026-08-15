import argparse
import json
import os
import shutil
import subprocess
import sys
from pathlib import Path

sys.path.insert(0, str(Path(__file__).resolve().parents[3]))
from common import (
    ArtifactRecord,
    BuildRecordBuilder,
    atomic_publish_directory,
    sha256_file,
    sha256_tree_manifest,
)


def read_lock(lock_path: str) -> dict:
    with open(lock_path, "r", encoding="utf-8") as f:
        return json.load(f)


def validate_elf_aarch64(binary_path: str) -> list:
    errors = []
    try:
        with open(binary_path, "rb") as f:
            magic = f.read(4)
            if magic != b"\x7fELF":
                errors.append(f"Invalid ELF magic: {binary_path}")
                return result
            f.seek(18)
            e_machine = int.from_bytes(f.read(2), "little")
            if e_machine != 183:
                errors.append(f"Not AArch64 ELF (e_machine={e_machine}): {binary_path}")
    except Exception as e:
        errors.append(f"ELF validation failed: {binary_path}: {e}")
    return errors


def normalize_tree(node_dir: str) -> list:
    errors = []
    node_bin = os.path.join(node_dir, "bin", "node")
    if not os.path.exists(node_bin):
        errors.append(f"node binary not found: {node_bin}")
    return errors


def build(
    source_archive: str,
    output_base: str,
    version: str,
    lock_path: str,
) -> ArtifactRecord:
    lock = read_lock(lock_path)
    expected_source_sha = lock.get("sourceSha256", "")

    if expected_source_sha:
        actual_sha = sha256_file(source_archive)
        if actual_sha != expected_source_sha:
            raise ValueError(
                f"Source archive SHA mismatch: expected {expected_source_sha}, got {actual_sha}"
            )

    staging_dir = os.path.join(output_base, ".staging", version)
    if os.path.exists(staging_dir):
        shutil.rmtree(staging_dir, ignore_errors=True)
    os.makedirs(staging_dir, exist_ok=True)

    import tarfile
    with tarfile.open(source_archive, "r:*") as tf:
        tf.extractall(staging_dir)

    extracted_entries = os.listdir(staging_dir)
    if len(extracted_entries) == 1:
        inner = os.path.join(staging_dir, extracted_entries[0])
        if os.path.isdir(inner):
            staging_dir = inner

    norm_errors = normalize_tree(staging_dir)
    if norm_errors:
        raise ValueError(f"Tree normalization failed: {norm_errors}")

    elf_errors = validate_elf_aarch64(os.path.join(staging_dir, "bin", "node"))
    if elf_errors:
        raise ValueError(f"ELF validation failed: {elf_errors}")

    tree_sha = sha256_tree_manifest(staging_dir)

    publish_base = os.path.join(output_base, "node", "linux-arm64")
    pub_result = atomic_publish_directory(staging_dir, publish_base, version)
    if not pub_result.success:
        raise RuntimeError(f"Atomic publish failed: {pub_result.errors}")

    published_dir = pub_result.published_dir
    final_tree_sha = sha256_tree_manifest(published_dir)
    if final_tree_sha != tree_sha:
        raise RuntimeError("Tree SHA mismatch after publish")

    record = (
        BuildRecordBuilder("node", version)
        .set_target("linux", "arm64")
        .set_artifact_type("frozen-tree")
        .set_artifact_path(
            f"node/linux-arm64/{version}",
            published_dir,
        )
        .set_tree_manifest(tree_sha)
        .set_build_mode("release")
        .set_metadata("sourceArchive", source_archive)
        .build()
    )

    record_path = os.path.join(published_dir, "build-record.json")
    record.save(record_path)

    shutil.rmtree(os.path.join(output_base, ".staging"), ignore_errors=True)
    return record


def main():
    parser = argparse.ArgumentParser(description="Node linux-arm64 Frozen Builder")
    parser.add_argument("--source-archive", required=True)
    parser.add_argument("--output-base", required=True)
    parser.add_argument("--version", required=True)
    parser.add_argument("--lock", required=True)
    args = parser.parse_args()

    record = build(args.source_archive, args.output_base, args.version, args.lock)
    print(json.dumps(record.to_dict(), indent=2))


if __name__ == "__main__":
    main()
