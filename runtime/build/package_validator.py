import os
import json
from typing import List, Dict, Optional, Tuple
from dataclasses import dataclass, field

from common.hashing import sha256_file, sha256_bytes
from common.errors import ValidationError


@dataclass
class PackageValidationResult:
    valid: bool = False
    errors: List[str] = field(default_factory=list)
    warnings: List[str] = field(default_factory=list)
    payload_integrity: Dict[str, bool] = field(default_factory=dict)
    metadata_integrity: Dict[str, bool] = field(default_factory=dict)


def _validate_payload_integrity(
    package_dir: str,
    component_lock: dict,
    result: PackageValidationResult,
) -> None:
    comp_map = {}
    for entry in component_lock.get("components", []):
        rel = entry.get("artifactRelativePath", "")
        sha = entry.get("artifactSha256", "")
        if rel and sha:
            comp_map[rel] = sha

    payload_dir = os.path.join(package_dir, "payload")
    if not os.path.isdir(payload_dir):
        result.errors.append("payload directory missing")
        return

    for root, dirs, files in os.walk(payload_dir):
        dirs.sort()
        for fname in sorted(files):
            full_path = os.path.join(root, fname)
            rel_path = os.path.relpath(full_path, package_dir).replace(os.sep, "/")
            actual_sha = sha256_file(full_path)
            if rel_path in comp_map:
                expected_sha = comp_map[rel_path]
                if actual_sha == expected_sha:
                    result.payload_integrity[rel_path] = True
                else:
                    result.payload_integrity[rel_path] = False
                    result.errors.append(
                        f"Payload SHA-256 mismatch for {rel_path}: "
                        f"expected={expected_sha}, actual={actual_sha}"
                    )
            else:
                result.payload_integrity[rel_path] = False
                result.warnings.append(
                    f"Payload file {rel_path} not found in component-lock"
                )


def _validate_metadata_integrity(
    package_dir: str,
    package_index: dict,
    result: PackageValidationResult,
) -> None:
    meta_map = {}
    for entry in package_index.get("metadata", []):
        rel = entry.get("path", "")
        sha = entry.get("sha256", "")
        if rel and sha:
            meta_map[rel] = sha

    for rel_path, expected_sha in meta_map.items():
        full_path = os.path.join(package_dir, rel_path)
        if not os.path.isfile(full_path):
            result.metadata_integrity[rel_path] = False
            result.errors.append(f"Metadata file missing: {rel_path}")
            continue
        actual_sha = sha256_file(full_path)
        if actual_sha == expected_sha:
            result.metadata_integrity[rel_path] = True
        else:
            result.metadata_integrity[rel_path] = False
            result.errors.append(
                f"Metadata SHA-256 mismatch for {rel_path}: "
                f"expected={expected_sha}, actual={actual_sha}"
            )


def _validate_no_nested_archives(
    package_dir: str,
    result: PackageValidationResult,
) -> None:
    archive_extensions = {".tar", ".tar.gz", ".tgz", ".tar.xz", ".txz", ".tar.bz2", ".zip"}
    payload_dir = os.path.join(package_dir, "payload")
    if not os.path.isdir(payload_dir):
        return

    for root, dirs, files in os.walk(payload_dir):
        dirs.sort()
        for fname in sorted(files):
            full_path = os.path.join(root, fname)
            rel_path = os.path.relpath(full_path, package_dir).replace(os.sep, "/")
            lower = fname.lower()
            for ext in archive_extensions:
                if lower.endswith(ext) and rel_path.count(".tar.") <= 0:
                    break
                if lower.endswith(ext):
                    nested = rel_path not in {
                        "payload/rootfs/rootfs.tar.xz",
                        "payload/runtime/runtime.tar.xz",
                    }
                    if nested:
                        result.errors.append(
                            f"Nested component archive detected in program tree: {rel_path}"
                        )
                    break


def _validate_guest_layout_dirs(
    package_dir: str,
    result: PackageValidationResult,
) -> None:
    layout_path = os.path.join(package_dir, "metadata", "guest-layout.json")
    if not os.path.isfile(layout_path):
        result.errors.append("guest-layout.json not found in metadata")
        return

    with open(layout_path, "r", encoding="utf-8") as f:
        layout = json.load(f)

    required_dirs = layout.get("requiredDirectories", []) if isinstance(layout, dict) else []
    for dir_path in required_dirs:
        norm = dir_path.replace("\\", "/").lstrip("/")
        if norm.startswith("metadata/") or norm.startswith("payload/"):
            continue
        full_path = os.path.join(package_dir, norm)
        if not os.path.isdir(full_path):
            result.errors.append(
                f"Guest layout directory missing after extraction: {norm}"
            )


def _validate_mount_contract_targets(
    package_dir: str,
    result: PackageValidationResult,
) -> None:
    contract_path = os.path.join(package_dir, "metadata", "mount-contract.json")
    if not os.path.isfile(contract_path):
        result.errors.append("mount-contract.json not found in metadata")
        return

    with open(contract_path, "r", encoding="utf-8") as f:
        contract = json.load(f)

    binds = contract.get("binds", []) if isinstance(contract, dict) else []
    for bind in binds:
        target = bind.get("target", "") if isinstance(bind, dict) else ""
        if not target:
            continue
        norm = target.replace("\\", "/").lstrip("/")
        full_path = os.path.join(package_dir, norm)
        if not os.path.exists(full_path):
            result.errors.append(
                f"Mount contract bind target does not exist in tree: {norm}"
            )


def _validate_sha256sums_integrity(
    package_dir: str,
    package_index: dict,
    result: PackageValidationResult,
) -> None:
    sums_path = os.path.join(package_dir, "metadata", "SHA256SUMS")
    if not os.path.isfile(sums_path):
        result.errors.append("SHA256SUMS file missing in metadata")
        return

    meta_map = {}
    for entry in package_index.get("metadata", []):
        rel = entry.get("path", "")
        sha = entry.get("sha256", "")
        if rel and sha:
            meta_map[rel] = sha

    sha256sums_entry = None
    for entry in package_index.get("metadata", []):
        if entry.get("path") == "metadata/SHA256SUMS":
            sha256sums_entry = entry
            break

    if sha256sums_entry:
        expected_sums_sha = sha256sums_entry.get("sha256", "")
        actual_sums_sha = sha256_file(sums_path)
        if actual_sums_sha != expected_sums_sha:
            result.errors.append(
                f"SHA256SUMS file integrity check failed: "
                f"expected={expected_sums_sha}, actual={actual_sums_sha}"
            )

    with open(sums_path, "r", encoding="utf-8") as f:
        lines = f.readlines()

    verified_count = 0
    for line in lines:
        line = line.strip()
        if not line or "  " not in line:
            continue
        parts = line.split("  ", 1)
        if len(parts) != 2:
            continue
        expected_sha, file_path = parts[0].strip(), parts[1].strip()
        if file_path == "metadata/SHA256SUMS":
            continue
        full_path = os.path.join(package_dir, file_path)
        if not os.path.isfile(full_path):
            result.errors.append(
                f"SHA256SUMS references missing file: {file_path}"
            )
            continue
        actual_sha = sha256_file(full_path)
        if actual_sha != expected_sha:
            result.errors.append(
                f"SHA256SUMS content mismatch for {file_path}: "
                f"expected={expected_sha}, actual={actual_sha}"
            )
        else:
            verified_count += 1

    if verified_count == 0 and len(lines) > 0:
        result.warnings.append("SHA256SUMS content verified zero files")


def validate_package(
    package_dir: str,
    contract_path: str = None,
    strict: bool = True,
) -> PackageValidationResult:
    result = PackageValidationResult()

    if not os.path.isdir(package_dir):
        result.errors.append(f"Package directory does not exist: {package_dir}")
        return result

    index_path = os.path.join(package_dir, "metadata", "package-index.json")
    if not os.path.isfile(index_path):
        result.errors.append("package-index.json not found")
        return result

    with open(index_path, "r", encoding="utf-8") as f:
        package_index = json.load(f)

    from package_index import validate_package_index
    index_errors = validate_package_index(package_index)
    if index_errors:
        for err in index_errors:
            result.errors.append(f"package-index schema validation: {err}")

    comp_lock_path = os.path.join(package_dir, "metadata", "component-lock.json")
    if not os.path.isfile(comp_lock_path):
        result.errors.append("component-lock.json not found")
    else:
        with open(comp_lock_path, "r", encoding="utf-8") as f:
            component_lock = json.load(f)

        lock_schema_errors = []
        if not isinstance(component_lock.get("schemaVersion"), int):
            lock_schema_errors.append("schemaVersion must be an integer")
        if not isinstance(component_lock.get("components"), list):
            lock_schema_errors.append("components must be a list")
        elif len(component_lock.get("components", [])) == 0:
            lock_schema_errors.append("components must be non-empty")
        else:
            for i, comp in enumerate(component_lock["components"]):
                for key in ("id", "version", "artifactRelativePath", "artifactSha256"):
                    if not comp.get(key):
                        lock_schema_errors.append(f"components[{i}].{key} is required")

        for err in lock_schema_errors:
            result.errors.append(f"component-lock schema validation: {err}")

    _validate_payload_integrity(package_dir, component_lock, result)
    _validate_metadata_integrity(package_dir, package_index, result)
    _validate_no_nested_archives(package_dir, result)
    _validate_guest_layout_dirs(package_dir, result)
    _validate_mount_contract_targets(package_dir, result)
    _validate_sha256sums_integrity(package_dir, package_index, result)

    if strict and not result.errors:
        result.valid = True
    elif not strict and not result.errors:
        result.valid = True

    return result
