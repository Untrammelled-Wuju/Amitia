import json
import os
from pathlib import Path
from typing import Dict, List, Optional

from .artifact_record import ArtifactRecord


class FrozenArtifactResolver:
    def __init__(self, output_base: str):
        self._output_base = output_base
        self._cache: Dict[str, ArtifactRecord] = {}

    def resolve(self, component_id: str) -> ArtifactRecord:
        if component_id in self._cache:
            return self._cache[component_id]

        record = self._find_latest_build_record(component_id)
        if record is None:
            raise FileNotFoundError(
                f"No frozen build record found for component: {component_id}"
            )
        self._cache[component_id] = record
        return record

    def resolve_all(self, component_ids: List[str]) -> Dict[str, ArtifactRecord]:
        results = {}
        for cid in component_ids:
            results[cid] = self.resolve(cid)
        return results

    def verify_artifact(self, component_id: str) -> List[str]:
        errors = []
        try:
            record = self.resolve(component_id)
        except FileNotFoundError as e:
            return [str(e)]

        full_path = self._resolve_artifact_path(record)
        if not os.path.exists(full_path):
            errors.append(f"Artifact path does not exist: {full_path}")
        return errors

    def _find_latest_build_record(self, component_id: str) -> Optional[ArtifactRecord]:
        base_dir = os.path.join(self._output_base, component_id, "linux-arm64")
        if not os.path.exists(base_dir):
            return None

        build_record_path = None
        for entry in sorted(os.listdir(base_dir), reverse=True):
            candidate = os.path.join(base_dir, entry, "build-record.json")
            if os.path.isfile(candidate):
                build_record_path = candidate
                break

        if not build_record_path:
            return None

        return ArtifactRecord.load(build_record_path)

    def _resolve_artifact_path(self, record: ArtifactRecord) -> str:
        return os.path.join(self._output_base, record.artifactRelativePath)

    @staticmethod
    def register_adapter(
        output_base: str, component_id: str, record: ArtifactRecord
    ) -> str:
        component_dir = os.path.join(output_base, component_id, "linux-arm64", record.version)
        os.makedirs(component_dir, exist_ok=True)
        record_path = os.path.join(component_dir, "build-record.json")
        record.save(record_path)
        return record_path
