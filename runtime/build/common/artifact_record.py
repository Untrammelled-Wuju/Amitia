import hashlib
import json
from dataclasses import dataclass, field, asdict
from pathlib import Path
from typing import Dict, List, Optional


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
    metadata: Dict[str, str] = field(default_factory=dict)

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
