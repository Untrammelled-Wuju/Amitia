import argparse
import json
import os
import shutil
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


def build(
    source_archive: str,
    output_base: str,
    version: str,
    lock_path: str,
) -> ArtifactRecord:
    if lock_path:
        with open(lock_path, "r", encoding="utf-8") as f:
            lock = json.load(f)
        expected_sha = lock.get("rootfsSha256", "")
        if expected_sha:
            actual_sha = sha256_file(source_archive)
            if actual_sha != expected_sha:
                raise ValueError(
                    f"Rootfs archive SHA mismatch: expected {expected_sha}, got {actual_sha}"
                )

    archive_path = Path(source_archive)
    if not archive_path.exists():
        raise FileNotFoundError(f"Source archive not found: {source_archive}")

    publish_base = os.path.join(output_base, "rootfs", "linux-arm64")
    os.makedirs(publish_base, exist_ok=True)

    dest_archive = os.path.join(publish_base, version, "rootfs.tar.xz")
    os.makedirs(os.path.dirname(dest_archive), exist_ok=True)
    shutil.copy2(source_archive, dest_archive)

    archive_sha = sha256_file(dest_archive)

    record = (
        BuildRecordBuilder("rootfs", version)
        .set_target("linux", "arm64")
        .set_artifact_type("archive")
        .set_artifact_path(
            f"rootfs/linux-arm64/{version}/rootfs.tar.xz",
            dest_archive,
        )
        .set_tree_manifest(archive_sha)
        .set_build_mode("release")
        .build()
    )

    record_path = os.path.join(os.path.dirname(dest_archive), "build-record.json")
    record.save(record_path)

    return record


def main():
    parser = argparse.ArgumentParser(description="Rootfs linux-arm64 Frozen Builder")
    parser.add_argument("--source-archive", required=True)
    parser.add_argument("--output-base", required=True)
    parser.add_argument("--version", required=True)
    parser.add_argument("--lock", default="")
    args = parser.parse_args()

    record = build(args.source_archive, args.output_base, args.version, args.lock)
    print(json.dumps(record.to_dict(), indent=2))


if __name__ == "__main__":
    main()
