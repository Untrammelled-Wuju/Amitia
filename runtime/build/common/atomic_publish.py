import os
import shutil
import tempfile
from dataclasses import dataclass
from pathlib import Path
from typing import List

from .artifact_record import FrozenArtifactRecord, validate
from .hashing import sha256_file, sha256_tree_manifest
from .tree_manifest import generate_tree_manifest


@dataclass
class AtomicPublishResult:
    success: bool = False
    published_dir: str = ""
    errors: List[str] = None

    def __post_init__(self):
        if self.errors is None:
            self.errors = []


def atomic_publish_directory(
    source_dir: str,
    target_base_dir: str,
    version_dir_name: str,
) -> AtomicPublishResult:
    result = AtomicPublishResult()
    source_path = Path(source_dir)
    target_base = Path(target_base_dir)
    final_dir = target_base / version_dir_name

    if not source_path.exists():
        result.errors.append(f"Source directory does not exist: {source_dir}")
        return result

    if not any(source_path.iterdir()):
        result.errors.append(f"Source directory is empty: {source_dir}")
        return result

    if final_dir.exists():
        result.errors.append(f"Target version directory already exists: {final_dir}")
        return result

    target_base.mkdir(parents=True, exist_ok=True)
    tmp_dir = None
    try:
        tmp_dir = tempfile.mkdtemp(prefix=".publish-", dir=str(target_base))
        tmp_path = Path(tmp_dir) / version_dir_name
        shutil.copytree(str(source_path), str(tmp_path))
        tmp_path.rename(str(final_dir))
        result.success = True
        result.published_dir = str(final_dir)
    except Exception as e:
        result.errors.append(f"Atomic publish failed: {e}")
        if final_dir.exists():
            try:
                shutil.rmtree(str(final_dir), ignore_errors=True)
            except Exception:
                pass
    finally:
        if tmp_dir and os.path.exists(tmp_dir):
            try:
                shutil.rmtree(tmp_dir, ignore_errors=True)
            except Exception:
                pass

    return result


def publish_candidate(candidate_dir: str, output_root: str, version: str) -> str:
    candidate_path = Path(candidate_dir)
    output_path = Path(output_root)

    if not candidate_path.exists() or not candidate_path.is_dir():
        raise FileNotFoundError(f"Candidate directory does not exist: {candidate_dir}")

    record_path = candidate_path / "build-record.json"
    if not record_path.exists():
        raise FileNotFoundError(f"build-record.json not found in candidate: {candidate_dir}")

    record = FrozenArtifactRecord.load(str(record_path))
    validation_errors = validate(record)
    if validation_errors:
        raise ValueError(f"Candidate validation failed: {'; '.join(validation_errors)}")

    artifact_path = candidate_path / record.artifactRelativePath
    if not artifact_path.exists():
        raise FileNotFoundError(f"Artifact not found: {artifact_path}")

    actual_artifact_hash = sha256_file(str(artifact_path))
    if actual_artifact_hash != record.artifactSha256:
        raise ValueError(
            f"Artifact SHA-256 mismatch: expected={record.artifactSha256}, actual={actual_artifact_hash}"
        )

    manifest_lines = generate_tree_manifest(str(candidate_path))
    manifest_hash = sha256_tree_manifest(manifest_lines)
    if manifest_hash != record.treeSha256:
        raise ValueError(
            f"Tree manifest SHA-256 mismatch: expected={record.treeSha256}, computed={manifest_hash}"
        )

    target_base = output_path / record.componentId / f"{record.platform}-{record.architecture}"
    final_dir = target_base / version

    if final_dir.exists():
        raise FileExistsError(f"Target version already exists: {final_dir}")

    output_path.mkdir(parents=True, exist_ok=True)
    target_base.mkdir(parents=True, exist_ok=True)
    staging_dir = None
    try:
        staging_dir = tempfile.mkdtemp(prefix=".publish-", dir=str(target_base))
        staging_path = Path(staging_dir) / version
        shutil.copytree(str(candidate_path), str(staging_path))
        staging_path.rename(str(final_dir))
    except Exception as e:
        if staging_dir and os.path.exists(staging_dir):
            try:
                shutil.rmtree(staging_dir, ignore_errors=True)
            except Exception:
                pass
        if final_dir.exists():
            try:
                shutil.rmtree(str(final_dir), ignore_errors=True)
            except Exception:
                pass
        raise RuntimeError(f"Publish candidate failed: {e}") from e

    return str(final_dir)
