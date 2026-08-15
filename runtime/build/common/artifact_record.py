import hashlib
import json
import re
import os
from dataclasses import dataclass, asdict
from datetime import datetime, timezone
from pathlib import Path
from typing import Dict, List, Optional


SHA256_HEX_PATTERN = re.compile(r"^[a-fA-F0-9]{64}$")


@dataclass
class ArtifactRecord:
    schemaVersion: int = 1
    componentId: str = ""
    version: str = ""
    platform: str = "linux"
    architecture: str = "arm64"
    artifactType: str = ""
    artifactRelativePath: str = ""
    artifactSha256: str = ""
    treeSha256: str = ""
    buildMode: str = "release"
    metadata: Dict[str, str] = None

    def __post_init__(self):
        if self.metadata is None:
            self.metadata = {}

    def to_dict(self) -> dict:
        return asdict(self)

    def to_json(self, indent: int = 2) -> str:
        return json.dumps(self.to_dict(), indent=indent, ensure_ascii=False)

    def save(self, path: str) -> None:
        Path(path).parent.mkdir(parents=True, exist_ok=True)
        with open(path, "w", encoding="utf-8") as f:
            f.write(self.to_json())
            f.write("\n")

    @classmethod
    def load(cls, path: str) -> "ArtifactRecord":
        with open(path, "r", encoding="utf-8") as f:
            data = json.load(f)
        return cls(**{k: v for k, v in data.items() if k in cls.__dataclass_fields__})


@dataclass
class FrozenArtifactRecord:
    schemaVersion: int = 1
    componentId: str = ""
    version: str = ""
    platform: str = "linux"
    architecture: str = "arm64"
    artifactType: str = ""
    artifactRelativePath: str = ""
    artifactSha256: str = ""
    treeSha256: str = ""
    sourceRevision: str = ""
    buildMode: str = "release"
    createdAt: str = ""

    def __post_init__(self):
        if not self.createdAt:
            self.createdAt = datetime.now(timezone.utc).strftime("%Y-%m-%dT%H:%M:%SZ")

    def to_dict(self) -> dict:
        d = asdict(self)
        return d

    def to_json(self, indent: int = 2) -> str:
        return json.dumps(self.to_dict(), indent=indent, ensure_ascii=False)

    def save(self, path: str) -> None:
        Path(path).parent.mkdir(parents=True, exist_ok=True)
        with open(path, "w", encoding="utf-8") as f:
            f.write(self.to_json())
            f.write("\n")

    @property
    def identity_key(self) -> str:
        return f"{self.componentId}:{self.platform}:{self.architecture}:{self.artifactType}"


def save(record: FrozenArtifactRecord, path: str) -> None:
    record.save(path)


def load(path: str) -> FrozenArtifactRecord:
    with open(path, "r", encoding="utf-8") as f:
        data = json.load(f)
    valid_fields = set(FrozenArtifactRecord.__dataclass_fields__.keys())
    filtered = {k: v for k, v in data.items() if k in valid_fields}
    return FrozenArtifactRecord(**filtered)


def validate(record: FrozenArtifactRecord) -> List[str]:
    errors = []

    if record.schemaVersion < 1:
        errors.append(f"schemaVersion must be >= 1, got {record.schemaVersion}")

    if not record.componentId:
        errors.append("componentId is required")

    if not record.version:
        errors.append("version is required")

    valid_platforms = ["linux", "android"]
    if record.platform not in valid_platforms:
        errors.append(f"platform must be one of {valid_platforms}, got '{record.platform}'")

    valid_archs = ["arm64", "arm64-v8a", "x86_64"]
    if record.architecture not in valid_archs:
        errors.append(f"architecture must be one of {valid_archs}, got '{record.architecture}'")

    valid_types = ["executable", "archive", "tree"]
    if record.artifactType not in valid_types:
        errors.append(f"artifactType must be one of {valid_types}, got '{record.artifactType}'")

    if not record.artifactRelativePath:
        errors.append("artifactRelativePath is required")

    if not record.artifactSha256 or not SHA256_HEX_PATTERN.match(record.artifactSha256):
        errors.append("artifactSha256 must be a valid 64-character hex SHA-256 digest")

    if not record.treeSha256 or not SHA256_HEX_PATTERN.match(record.treeSha256):
        errors.append("treeSha256 must be a valid 64-character hex SHA-256 digest")

    if not record.sourceRevision:
        errors.append("sourceRevision is required")

    valid_modes = ["release", "debug", "offline"]
    if record.buildMode not in valid_modes:
        errors.append(f"buildMode must be one of {valid_modes}, got '{record.buildMode}'")

    return errors


class BuildRecordBuilder:
    def __init__(self, component_id: str, version: str):
        self._record = ArtifactRecord(componentId=component_id, version=version)

    def set_target(self, platform: str, architecture: str) -> "BuildRecordBuilder":
        self._record.platform = platform
        self._record.architecture = architecture
        return self

    def set_artifact_type(self, artifact_type: str) -> "BuildRecordBuilder":
        self._record.artifactType = artifact_type
        return self

    def set_artifact_path(self, relative_path: str, full_path: str) -> "BuildRecordBuilder":
        self._record.artifactRelativePath = relative_path
        self._record.artifactSha256 = sha256_file(full_path)
        return self

    def set_tree_manifest(self, tree_sha256: str) -> "BuildRecordBuilder":
        self._record.treeSha256 = tree_sha256
        return self

    def set_build_mode(self, mode: str) -> "BuildRecordBuilder":
        self._record.buildMode = mode
        return self

    def set_metadata(self, key: str, value: str) -> "BuildRecordBuilder":
        self._record.metadata[key] = value
        return self

    def build(self) -> ArtifactRecord:
        if not self._record.componentId:
            raise ValueError("componentId is required")
        if not self._record.version:
            raise ValueError("version is required")
        if not self._record.artifactRelativePath:
            raise ValueError("artifactRelativePath is required")
        if not self._record.artifactSha256:
            raise ValueError("artifactSha256 is required")
        return self._record


def sha256_file(path: str) -> str:
    digest = hashlib.sha256()
    with open(path, "rb") as f:
        for chunk in iter(lambda: f.read(65536), b""):
            digest.update(chunk)
    return digest.hexdigest()


def validate_build_record(record: ArtifactRecord) -> List[str]:
    errors = []
    if record.schemaVersion != 1:
        errors.append(f"unsupported schemaVersion: {record.schemaVersion}")
    if not record.componentId:
        errors.append("componentId is required")
    if not record.version:
        errors.append("version is required")
    if not record.artifactRelativePath:
        errors.append("artifactRelativePath is required")
    if not record.artifactSha256 or len(record.artifactSha256) != 64:
        errors.append("artifactSha256 must be a valid SHA-256 hex digest")
    if record.treeSha256 and len(record.treeSha256) != 64:
        errors.append("treeSha256 must be a valid SHA-256 hex digest")
    return errors
