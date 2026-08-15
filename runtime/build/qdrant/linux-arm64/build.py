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
    sha256_file,
)


def build(
    source_archive: str,
    output_base: str,
    version: str,
) -> ArtifactRecord:
    if not os.path.isfile(source_archive):
        raise FileNotFoundError(f"Qdrant archive not found: {source_archive}")

    if os.path.getsize(source_archive) == 0:
        raise ValueError(f"Qdrant archive is empty: {source_archive}")

    archive_sha = sha256_file(source_archive)

    component_dir = os.path.join(output_base, "qdrant", "linux-arm64", version)
    os.makedirs(component_dir, exist_ok=True)

    dest_archive = os.path.join(component_dir, "qdrant.zip")
    shutil.copy2(source_archive, dest_archive)

    record = (
        BuildRecordBuilder("qdrant", version)
        .set_target("linux", "arm64")
        .set_artifact_type("archive")
        .set_artifact_path(
            f"qdrant/linux-arm64/{version}/qdrant.zip",
            dest_archive,
        )
        .set_tree_manifest(archive_sha)
        .set_build_mode("release")
        .build()
    )

    record_path = os.path.join(component_dir, "build-record.json")
    record.save(record_path)

    return record


def main():
    parser = argparse.ArgumentParser(description="Qdrant linux-arm64 Frozen Adapter")
    parser.add_argument("--source-archive", required=True)
    parser.add_argument("--output-base", required=True)
    parser.add_argument("--version", required=True)
    args = parser.parse_args()

    record = build(args.source_archive, args.output_base, args.version)
    print(json.dumps(record.to_dict(), indent=2))


if __name__ == "__main__":
    main()
