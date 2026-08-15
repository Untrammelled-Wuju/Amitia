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
    source_dir: str,
    output_base: str,
    version: str,
) -> ArtifactRecord:
    source_path = Path(source_dir)
    if not source_path.exists():
        raise FileNotFoundError(f"Source directory not found: {source_dir}")

    entrypoint = source_path / "dist" / "index.js"
    if not entrypoint.exists():
        raise FileNotFoundError(f"Plugin Host entrypoint not found: {entrypoint}")

    staging_dir = os.path.join(output_base, ".staging", version)
    if os.path.exists(staging_dir):
        shutil.rmtree(staging_dir, ignore_errors=True)
    os.makedirs(staging_dir, exist_ok=True)

    shutil.copytree(str(source_path), staging_dir, dirs_exist_ok=True)

    tree_sha = sha256_tree_manifest(staging_dir)

    publish_base = os.path.join(output_base, "plugin-host", "linux-arm64")
    pub_result = atomic_publish_directory(staging_dir, publish_base, version)
    if not pub_result.success:
        raise RuntimeError(f"Atomic publish failed: {pub_result.errors}")

    published_dir = pub_result.published_dir
    final_tree_sha = sha256_tree_manifest(published_dir)
    if final_tree_sha != tree_sha:
        raise RuntimeError("Tree SHA mismatch after publish")

    record = (
        BuildRecordBuilder("plugin-host", version)
        .set_target("linux", "arm64")
        .set_artifact_type("frozen-tree")
        .set_artifact_path(
            f"plugin-host/linux-arm64/{version}",
            published_dir,
        )
        .set_tree_manifest(tree_sha)
        .set_build_mode("release")
        .build()
    )

    record_path = os.path.join(published_dir, "build-record.json")
    record.save(record_path)

    shutil.rmtree(os.path.join(output_base, ".staging"), ignore_errors=True)
    return record


def main():
    parser = argparse.ArgumentParser(description="Plugin Host linux-arm64 Frozen Builder")
    parser.add_argument("--source-dir", required=True)
    parser.add_argument("--output-base", required=True)
    parser.add_argument("--version", required=True)
    args = parser.parse_args()

    record = build(args.source_dir, args.output_base, args.version)
    print(json.dumps(record.to_dict(), indent=2))


if __name__ == "__main__":
    main()
