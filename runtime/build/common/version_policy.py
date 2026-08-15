import json
import os
from dataclasses import dataclass
from enum import Enum
from pathlib import Path

from .artifact_record import FrozenArtifactRecord


class VersionResolutionKind(str, Enum):
    REUSE = "REUSE"
    FAIL = "FAIL"
    PUBLISH = "PUBLISH"


@dataclass
class VersionResolution:
    kind: VersionResolutionKind
    existing_dir: str = ""
    reason: str = ""


def resolve_version(existing_dir: str, new_record: FrozenArtifactRecord) -> VersionResolution:
    if not os.path.isdir(existing_dir):
        return VersionResolution(
            kind=VersionResolutionKind.PUBLISH,
            existing_dir=existing_dir,
            reason="no existing directory, first publish",
        )

    record_path = os.path.join(existing_dir, "build-record.json")
    if not os.path.isfile(record_path):
        return VersionResolution(
            kind=VersionResolutionKind.PUBLISH,
            existing_dir=existing_dir,
            reason="no build-record.json in existing directory, treat as new",
        )

    try:
        with open(record_path, "r", encoding="utf-8") as f:
            data = json.load(f)
    except (json.JSONDecodeError, OSError) as e:
        return VersionResolution(
            kind=VersionResolutionKind.PUBLISH,
            existing_dir=existing_dir,
            reason=f"failed to read existing build-record.json: {e}",
        )

    identity_fields = ["componentId", "platform", "architecture", "artifactType"]
    for field_name in identity_fields:
        existing_val = data.get(field_name, "")
        new_val = getattr(new_record, field_name, "")
        if existing_val != new_val:
            return VersionResolution(
                kind=VersionResolutionKind.FAIL,
                existing_dir=existing_dir,
                reason=f"identity mismatch: {field_name} existing={existing_val} new={new_val}",
            )

    existing_version = data.get("version", "")
    new_version = new_record.version

    if existing_version == new_version:
        return VersionResolution(
            kind=VersionResolutionKind.REUSE,
            existing_dir=existing_dir,
            reason="same version and same identity, reuse existing",
        )
    else:
        return VersionResolution(
            kind=VersionResolutionKind.PUBLISH,
            existing_dir=existing_dir,
            reason=f"version changed: {existing_version} -> {new_version}",
        )
