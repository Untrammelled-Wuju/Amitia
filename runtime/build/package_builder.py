import json
import os
import shutil
import tempfile
from pathlib import Path
from typing import Dict, List, Optional, Tuple

from .common.artifact_record import FrozenArtifactRecord, load as load_record
from .common.hashing import sha256_file, sha256_bytes
from .common.safe_extract import safe_extract
from .common.deterministic_archive import create_deterministic_tar_xz
from .common.tree_manifest import generate_tree_manifest
from .common.atomic_publish import publish_candidate
from .common.program_tree_validator import validate_program_tree
from .common.errors import BuildError, ValidationError
from .program_tree_assembler import (
    ProgramComponent,
    assemble_program_tree,
    copy_manifest_files,
)
from .package_index import (
    generate_package_index,
    validate_package_index,
    write_package_index,
)


METADATA_REQUIRED_ENTRIES = [
    {"role": "package-index", "path": "metadata/package-index.json"},
    {"role": "component-lock", "path": "metadata/component-lock.json"},
    {"role": "sha256sums", "path": "metadata/SHA256SUMS"},
    {"role": "guest-layout", "path": "metadata/guest-layout.json"},
    {"role": "mount-contract", "path": "metadata/mount-contract.json"},
]

PAYLOAD_REQUIRED_ENTRIES = [
    {"role": "rootfs", "path": "payload/rootfs/rootfs.tar.xz"},
    {"role": "runtime", "path": "payload/runtime/runtime.tar.xz"},
]


def _validate_artifact_record(record: FrozenArtifactRecord, base_dir: str) -> str:
    from .common.artifact_record import validate
    errors = validate(record)
    if errors:
        raise ValidationError(
            f"Invalid FrozenArtifactRecord for {record.componentId}: {'; '.join(errors)}"
        )

    artifact_path = os.path.join(base_dir, record.artifactRelativePath)
    if not os.path.exists(artifact_path):
        raise FileNotFoundError(
            f"Artifact not found for {record.componentId}: {artifact_path}"
        )

    actual_sha = sha256_file(artifact_path)
    if actual_sha != record.artifactSha256:
        raise ValidationError(
            f"SHA-256 mismatch for {record.componentId}: "
            f"expected={record.artifactSha256}, actual={actual_sha}"
        )

    return artifact_path


def _build_component_lock(components: List[ProgramComponent]) -> dict:
    comp_entries = []
    for comp in components:
        record = comp.artifact_record
        comp_entries.append({
            "id": record.componentId,
            "version": record.version,
            "platform": record.platform,
            "architecture": record.architecture,
            "artifactType": record.artifactType,
            "artifactRelativePath": record.artifactRelativePath,
            "artifactSha256": record.artifactSha256,
            "treeSha256": record.treeSha256,
            "sourceRevision": record.sourceRevision,
            "buildMode": record.buildMode,
        })
    return {
        "schemaVersion": 1,
        "components": comp_entries,
    }


def _generate_sha256sums(staging_dir: str) -> Tuple[str, dict]:
    sums = {}
    for root, dirs, files in os.walk(staging_dir):
        dirs.sort()
        for fname in sorted(files):
            full = os.path.join(root, fname)
            rel = os.path.relpath(full, staging_dir).replace(os.sep, "/")
            if rel == "metadata/SHA256SUMS":
                continue
            sums[rel] = sha256_file(full)

    content_lines = []
    for rel_path in sorted(sums.keys()):
        content_lines.append(f"{sums[rel_path]}  {rel_path}")
    content = "\n".join(content_lines)
    if content:
        content += "\n"

    content_bytes = content.encode("utf-8")
    return sha256_bytes(content_bytes), {
        "content": content,
        "content_bytes": content_bytes,
        "files": sums,
    }


def build_runtime_package(
    output_path: str,
    rootfs_artifact: FrozenArtifactRecord,
    runtime_program_dir: str,
    guest_layout: dict,
    mount_contract: dict,
    component_lock: dict,
    version: str,
    package_id: str,
    source_revision: str,
    mode: str = "release",
) -> str:
    rootfs_base_dir = os.path.dirname(rootfs_artifact.artifactRelativePath) if os.path.sep in rootfs_artifact.artifactRelativePath else ""
    rootfs_artifact_path = rootfs_artifact.artifactRelativePath

    components_raw = component_lock.get("components", [])
    if not components_raw:
        raise ValidationError("component_lock must contain at least one component")

    components: List[ProgramComponent] = []
    for entry in components_raw:
        comp_id = entry.get("id", "")
        comp_record = FrozenArtifactRecord(
            componentId=comp_id,
            version=entry.get("version", ""),
            platform=entry.get("platform", "linux"),
            architecture=entry.get("architecture", "arm64"),
            artifactType=entry.get("artifactType", ""),
            artifactRelativePath=entry.get("artifactRelativePath", ""),
            artifactSha256=entry.get("artifactSha256", ""),
            treeSha256=entry.get("treeSha256", ""),
            sourceRevision=entry.get("sourceRevision", ""),
            buildMode=entry.get("buildMode", mode),
        )
        comp_artifact_path = entry.get("artifactRelativePath", "")
        components.append(ProgramComponent(
            component_id=comp_id,
            artifact_record=comp_record,
            source_path=comp_artifact_path,
            target_subdir=comp_id,
        ))

    staging_dir = tempfile.mkdtemp(prefix=".package-staging-")
    try:
        program_staging = os.path.join(staging_dir, "program")
        os.makedirs(program_staging, exist_ok=True)

        manifest_guest_layout_path = guest_layout.get("path", "")
        manifest_mount_contract_path = mount_contract.get("path", "")

        if not manifest_guest_layout_path or not os.path.exists(manifest_guest_layout_path):
            raise FileNotFoundError(
                f"guest-layout.json path not found: {manifest_guest_layout_path}"
            )
        if not manifest_mount_contract_path or not os.path.exists(manifest_mount_contract_path):
            raise FileNotFoundError(
                f"mount-contract.json path not found: {manifest_mount_contract_path}"
            )

        manifest_result = assemble_program_tree(components, program_staging)

        copy_manifest_files(
            program_staging,
            manifest_guest_layout_path,
            manifest_mount_contract_path,
        )

        validation_result = validate_program_tree(program_staging)
        if not validation_result.valid:
            raise ValidationError(
                f"Program tree validation failed: {'; '.join(validation_result.errors)}"
            )

        metadata_dir = os.path.join(staging_dir, "metadata")
        payload_dir = os.path.join(staging_dir, "payload")
        payload_runtime_dir = os.path.join(payload_dir, "runtime")
        payload_rootfs_dir = os.path.join(payload_dir, "rootfs")
        os.makedirs(metadata_dir, exist_ok=True)
        os.makedirs(payload_runtime_dir, exist_ok=True)
        os.makedirs(payload_rootfs_dir, exist_ok=True)

        runtime_tar_xz_path = os.path.join(payload_runtime_dir, "runtime.tar.xz")
        create_deterministic_tar_xz(program_staging, runtime_tar_xz_path)
        runtime_tar_sha = sha256_file(runtime_tar_xz_path)
        runtime_tar_size = os.path.getsize(runtime_tar_xz_path)

        rootfs_dest = os.path.join(payload_rootfs_dir, "rootfs.tar.xz")
        if not os.path.exists(rootfs_artifact_path):
            raise FileNotFoundError(f"Rootfs artifact not found: {rootfs_artifact_path}")
        rootfs_actual_sha = sha256_file(rootfs_artifact_path)
        if rootfs_actual_sha != rootfs_artifact.artifactSha256:
            raise ValidationError(
                f"Rootfs SHA-256 mismatch: expected={rootfs_artifact.artifactSha256}, "
                f"actual={rootfs_actual_sha}"
            )
        shutil.copy2(rootfs_artifact_path, rootfs_dest)
        rootfs_sha = sha256_file(rootfs_dest)
        rootfs_size = os.path.getsize(rootfs_dest)

        component_lock_obj = _build_component_lock(components)
        component_lock_path = os.path.join(metadata_dir, "component-lock.json")
        with open(component_lock_path, "w", encoding="utf-8") as f:
            json.dump(component_lock_obj, f, indent=2, ensure_ascii=False)
            f.write("\n")
        component_lock_sha = sha256_file(component_lock_path)
        component_lock_size = os.path.getsize(component_lock_path)

        guest_layout_dest = os.path.join(metadata_dir, "guest-layout.json")
        shutil.copy2(manifest_guest_layout_path, guest_layout_dest)
        guest_layout_sha = sha256_file(guest_layout_dest)
        guest_layout_size = os.path.getsize(guest_layout_dest)

        mount_contract_dest = os.path.join(metadata_dir, "mount-contract.json")
        shutil.copy2(manifest_mount_contract_path, mount_contract_dest)
        mount_contract_sha = sha256_file(mount_contract_dest)
        mount_contract_size = os.path.getsize(mount_contract_dest)

        sums_sha, sums_info = _generate_sha256sums(staging_dir)
        sums_path = os.path.join(metadata_dir, "SHA256SUMS")
        with open(sums_path, "wb") as f:
            f.write(sums_info["content_bytes"])
        sums_size = len(sums_info["content_bytes"])

        payloads = [
            {
                "role": "rootfs",
                "path": "payload/rootfs/rootfs.tar.xz",
                "sha256": rootfs_sha,
                "size": rootfs_size,
            },
            {
                "role": "runtime",
                "path": "payload/runtime/runtime.tar.xz",
                "sha256": runtime_tar_sha,
                "size": runtime_tar_size,
            },
        ]

        metadata_entries = [
            {
                "role": "package-index",
                "path": "metadata/package-index.json",
                "sha256": "",
                "size": 0,
            },
            {
                "role": "component-lock",
                "path": "metadata/component-lock.json",
                "sha256": component_lock_sha,
                "size": component_lock_size,
            },
            {
                "role": "sha256sums",
                "path": "metadata/SHA256SUMS",
                "sha256": sums_sha,
                "size": sums_size,
            },
            {
                "role": "guest-layout",
                "path": "metadata/guest-layout.json",
                "sha256": guest_layout_sha,
                "size": guest_layout_size,
            },
            {
                "role": "mount-contract",
                "path": "metadata/mount-contract.json",
                "sha256": mount_contract_sha,
                "size": mount_contract_size,
            },
        ]

        package_index = generate_package_index(
            runtime_version=version,
            package_id=package_id,
            source_revision=source_revision,
            payloads=payloads,
            metadata=metadata_entries,
        )

        index_errors = validate_package_index(package_index)
        if index_errors:
            raise ValidationError(
                f"Package index validation failed: {'; '.join(index_errors)}"
            )

        index_sha = write_package_index(
            package_index,
            os.path.join(metadata_dir, "package-index.json"),
        )

        for entry in metadata_entries:
            if entry["path"] == "metadata/package-index.json":
                entry["sha256"] = index_sha
                entry["size"] = os.path.getsize(os.path.join(metadata_dir, "package-index.json"))
                break

        package_index = generate_package_index(
            runtime_version=version,
            package_id=package_id,
            source_revision=source_revision,
            payloads=payloads,
            metadata=metadata_entries,
        )
        write_package_index(
            package_index,
            os.path.join(metadata_dir, "package-index.json"),
        )

        final_sums_sha, final_sums_info = _generate_sha256sums(staging_dir)
        with open(sums_path, "wb") as f:
            f.write(final_sums_info["content_bytes"])

        sums_path_final = os.path.join(metadata_dir, "SHA256SUMS")
        final_sums_size = os.path.getsize(sums_path_final)
        for entry in metadata_entries:
            if entry["path"] == "metadata/SHA256SUMS":
                entry["sha256"] = final_sums_sha
                entry["size"] = final_sums_size
                break

        package_index = generate_package_index(
            runtime_version=version,
            package_id=package_id,
            source_revision=source_revision,
            payloads=payloads,
            metadata=metadata_entries,
        )
        index_errors = validate_package_index(package_index)
        if index_errors:
            raise ValidationError(
                f"Final package index validation failed: {'; '.join(index_errors)}"
            )
        write_package_index(
            package_index,
            os.path.join(metadata_dir, "package-index.json"),
        )

        final_validation = validate_program_tree(program_staging)
        if not final_validation.valid:
            raise ValidationError(
                f"Final program tree validation failed: {'; '.join(final_validation.errors)}"
            )

        candidate_dir = tempfile.mkdtemp(prefix=".candidate-")
        try:
            for item in os.listdir(staging_dir):
                s = os.path.join(staging_dir, item)
                d = os.path.join(candidate_dir, item)
                if os.path.isdir(s):
                    shutil.copytree(s, d)
                else:
                    shutil.copy2(s, d)

            build_record = {
                "schemaVersion": 1,
                "runtimeVersion": version,
                "packageId": package_id,
                "sourceRevision": source_revision,
                "treeSha256": manifest_result.tree_sha256,
                "runtimeTarSha256": runtime_tar_sha,
                "rootfsSha256": rootfs_sha,
                "guestLayoutSha256": guest_layout_sha,
                "mountContractSha256": mount_contract_sha,
                "componentLockSha256": component_lock_sha,
                "sha256sumsSha256": final_sums_sha,
                "buildMode": mode,
                "componentIds": [c.component_id for c in components],
            }
            build_record_path = os.path.join(candidate_dir, "build-record.json")
            with open(build_record_path, "w", encoding="utf-8") as f:
                json.dump(build_record, f, indent=2, ensure_ascii=False)
                f.write("\n")

            output_parent = os.path.dirname(output_path)
            if output_parent:
                os.makedirs(output_parent, exist_ok=True)

            if os.path.exists(output_path):
                if os.path.isdir(output_path):
                    shutil.rmtree(output_path, ignore_errors=True)
                else:
                    os.remove(output_path)

            final_tmp = tempfile.mkdtemp(prefix=".final-publish-")
            try:
                final_staging = os.path.join(final_tmp, "package")
                shutil.copytree(candidate_dir, final_staging)

                if os.path.exists(output_path):
                    if os.path.isdir(output_path):
                        shutil.rmtree(output_path, ignore_errors=True)
                    else:
                        os.remove(output_path)

                shutil.copytree(final_staging, output_path) if os.path.isdir(final_staging) else shutil.copy2(final_staging, output_path)
            finally:
                shutil.rmtree(final_tmp, ignore_errors=True)

        finally:
            shutil.rmtree(candidate_dir, ignore_errors=True)

    finally:
        shutil.rmtree(staging_dir, ignore_errors=True)

    return output_path
