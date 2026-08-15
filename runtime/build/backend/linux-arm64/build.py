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
from common.frozen_artifact_resolver import FrozenArtifactResolver


def build(
    source_binary: str,
    source_artifact_json: str,
    output_base: str,
    version: str,
) -> ArtifactRecord:
    if not os.path.isfile(source_binary):
        raise FileNotFoundError(f"Backend binary not found: {source_binary}")

    if os.path.getsize(source_binary) == 0:
        raise ValueError(f"Backend binary is empty: {source_binary}")

    existing_record = None
    if os.path.isfile(source_artifact_json):
        with open(source_artifact_json, "r", encoding="utf-8") as f:
            existing_record = json.load(f)

    binary_sha = sha256_file(source_binary)

    if existing_record and existing_record.get("artifactSha256") == binary_sha:
        return ArtifactRecord(**{k: v for k, v in existing_record.items() if k in ArtifactRecord.__dataclass_fields__})

    component_dir = os.path.join(output_base, "backend", "linux-arm64", version)
    os.makedirs(component_dir, exist_ok=True)

    dest_binary = os.path.join(component_dir, "amitia-server")
    shutil.copy2(source_binary, dest_binary)

    record = (
        BuildRecordBuilder("backend", version)
        .set_target("linux", "arm64")
        .set_artifact_type("binary")
        .set_artifact_path(
            f"backend/linux-arm64/{version}/amitia-server",
            dest_binary,
        )
        .set_tree_manifest(binary_sha)
        .set_build_mode("release")
        .build()
    )

    record_path = os.path.join(component_dir, "build-record.json")
    record.save(record_path)

    return record


def main():
    parser = argparse.ArgumentParser(description="Backend linux-arm64 Frozen Adapter")
    parser.add_argument("--source-binary", required=True)
    parser.add_argument("--source-artifact-json", default="")
    parser.add_argument("--output-base", required=True)
    parser.add_argument("--version", required=True)
    args = parser.parse_args()

    record = build(args.source_binary, args.source_artifact_json, args.output_base, args.version)
    print(json.dumps(record.to_dict(), indent=2))


if __name__ == "__main__":
    main()
