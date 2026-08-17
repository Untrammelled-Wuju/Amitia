import json
import os
import re
from dataclasses import dataclass, field, asdict
from datetime import datetime, timezone
from typing import List

SCHEMA_VERSION = 1

VALID_PLATFORMS = {"linux", "android"}
VALID_ARCHITECTURES = {"arm64", "arm64-v8a", "x86_64"}
VALID_ARTIFACT_TYPES = {"executable", "archive", "tree"}
VALID_BUILD_MODES = {"release", "debug", "offline"}

SHA256_PATTERN = re.compile(r"^[a-fA-F0-9]{64}$")


@dataclass(frozen=True)
class FrozenArtifactRecord:
    componentId: str
    version: str
    platform: str
    architecture: str
    artifactType: str
    artifactRelativePath: str
    artifactSha256: str
    treeSha256: str
    sourceRevision: str
    buildMode: str
    schemaVersion: int = SCHEMA_VERSION
    createdAt: str = field(default_factory=lambda: datetime.now(timezone.utc).isoformat())
    _record_path: str = field(default="", repr=False, compare=False)

    def to_dict(self) -> dict:
        return asdict(self)

    @classmethod
    def from_dict(cls, data: dict) -> "FrozenArtifactRecord":
        return cls(
            schemaVersion=data.get("schemaVersion", SCHEMA_VERSION),
            componentId=data["componentId"],
            version=data["version"],
            platform=data["platform"],
            architecture=data["architecture"],
            artifactType=data["artifactType"],
            artifactRelativePath=data["artifactRelativePath"],
            artifactSha256=data["artifactSha256"],
            treeSha256=data["treeSha256"],
            sourceRevision=data["sourceRevision"],
            buildMode=data["buildMode"],
            createdAt=data.get("createdAt", datetime.now(timezone.utc).isoformat()),
        )

    def write(self, path: str) -> None:
        write_canonical_json(self, path)

    @classmethod
    def load(cls, path: str) -> "FrozenArtifactRecord":
        return load(path)


def validate(record: FrozenArtifactRecord) -> List[str]:
    errors = []
    if record.schemaVersion < 1:
        errors.append(f"schemaVersion must be >= 1, got {record.schemaVersion}")
    if not record.componentId or not isinstance(record.componentId, str):
        errors.append("componentId is required and must be a non-empty string")
    if not record.version or not isinstance(record.version, str):
        errors.append("version is required and must be a non-empty string")
    if record.platform not in VALID_PLATFORMS:
        errors.append(f"platform '{record.platform}' is not valid. Allowed: {VALID_PLATFORMS}")
    if record.architecture not in VALID_ARCHITECTURES:
        errors.append(f"architecture '{record.architecture}' is not valid. Allowed: {VALID_ARCHITECTURES}")
    if record.artifactType not in VALID_ARTIFACT_TYPES:
        errors.append(f"artifactType '{record.artifactType}' is not valid. Allowed: {VALID_ARTIFACT_TYPES}")
    if not record.artifactRelativePath:
        errors.append("artifactRelativePath is required")
    else:
        if record.artifactRelativePath.startswith("/"):
            errors.append(f"artifactRelativePath must be relative, got absolute path: {record.artifactRelativePath}")
        if ".." in record.artifactRelativePath.split("/"):
            errors.append(f"artifactRelativePath must not contain '..': {record.artifactRelativePath}")
    if not SHA256_PATTERN.match(record.artifactSha256):
        errors.append(f"artifactSha256 must be 64 hex chars, got: {record.artifactSha256}")
    if not SHA256_PATTERN.match(record.treeSha256):
        errors.append(f"treeSha256 must be 64 hex chars, got: {record.treeSha256}")
    if not record.sourceRevision:
        errors.append("sourceRevision is required")
    if record.buildMode not in VALID_BUILD_MODES:
        errors.append(f"buildMode '{record.buildMode}' is not valid. Allowed: {VALID_BUILD_MODES}")
    return errors


def load(path: str) -> FrozenArtifactRecord:
    with open(path, "r", encoding="utf-8") as f:
        data = json.load(f)
    record = FrozenArtifactRecord(
        schemaVersion=data.get("schemaVersion", SCHEMA_VERSION),
        componentId=data["componentId"],
        version=data["version"],
        platform=data["platform"],
        architecture=data["architecture"],
        artifactType=data["artifactType"],
        artifactRelativePath=data["artifactRelativePath"],
        artifactSha256=data["artifactSha256"],
        treeSha256=data["treeSha256"],
        sourceRevision=data["sourceRevision"],
        buildMode=data["buildMode"],
        createdAt=data.get("createdAt", datetime.now(timezone.utc).isoformat()),
    )
    object.__setattr__(record, '_record_path', path)
    return record


def write_canonical_json(record: FrozenArtifactRecord, path: str) -> None:
    data = asdict(record)
    with open(path, "w", encoding="utf-8") as f:
        json.dump(data, f, indent=2, sort_keys=True)
        f.write("\n")


def record_sha256(path: str) -> str:
    import hashlib
    digest = hashlib.sha256()
    with open(path, "rb") as f:
        for chunk in iter(lambda: f.read(65536), b""):
            digest.update(chunk)
    return digest.hexdigest()


def resolve_artifact_path(record: FrozenArtifactRecord, base_dir: str) -> str:
    return os.path.normpath(os.path.join(base_dir, record.artifactRelativePath))
