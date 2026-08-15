import os
import shutil
import stat
import tempfile
from dataclasses import dataclass, field
from typing import List, Dict, Optional, Tuple

from common.artifact_record import FrozenArtifactRecord
from common.hashing import sha256_file, sha256_tree_manifest
from common.safe_extract import safe_extract
from common.tree_manifest import generate_tree_manifest
from common.errors import BuildError, ValidationError


FORBIDDEN_STAGING_ENTRIES = {
    "build-record.json",
    ".cache",
    ".work",
    ".git",
    ".gitignore",
    ".DS_Store",
    "Thumbs.db",
    "node_modules",
    "__pycache__",
    ".pytest_cache",
    "logs",
    "log",
    "tmp",
    "temp",
    "cache",
    "backup",
    "backups",
}

FORBIDDEN_PATH_PATTERNS = [
    "/.cache/",
    "/.work/",
    "/.git/",
    "/node_modules/",
    "/__pycache__/",
    "/logs/",
    "/log/",
    "/tmp/",
    "/temp/",
    "/cache/",
    "/backup/",
    "/backups/",
]

FORBIDDEN_RUNTIME_PATHS = {
    "runtime-manifest",
    "active-runtime",
    "host-absolute-path",
    "token",
    "secrets",
}

ALLOWED_PROGRAM_ROOTS = {
    "backend",
    "node",
    "qdrant",
    "sidecar",
    "qq-sidecar",
    "plugin-host",
    "task-host",
    "scripts",
    "manifest",
    "licenses",
}

REQUIRED_PROGRAM_PATHS = [
    "backend/amitia-server",
    "node/bin/node",
    "node/lib/node_modules/npm/bin/npm-cli.js",
    "node/lib/node_modules/npm/bin/npx-cli.js",
    "qdrant/bin/qdrant",
    "plugin-host/dist/index.js",
    "task-host/dist/index.js",
    "scripts/node/amitia-node-prepare.sh",
    "scripts/node/amitia-node-probe.sh",
    "manifest/guest-layout.json",
    "manifest/mount-contract.json",
]

COMPONENT_EXTRACT_MAP = {
    "backend": ("backend", "backend/amitia-server"),
    "qdrant": ("qdrant", "qdrant/bin/qdrant"),
}


@dataclass
class ProgramComponent:
    component_id: str
    artifact_record: FrozenArtifactRecord
    source_path: str
    target_subdir: str


@dataclass
class ProgramTreeManifest:
    components: List[ProgramComponent]
    tree_sha256: str
    manifest_lines: List[str]


def _is_forbidden_staging_entry(name: str) -> bool:
    if name in FORBIDDEN_STAGING_ENTRIES:
        return True
    if name.startswith(".") and name not in {".", ".."}:
        if name.startswith((".cache", ".work", ".git", ".pytest")):
            return True
    return False


def _contains_forbidden_path(rel_path: str) -> bool:
    norm = rel_path.replace(os.sep, "/")
    for pattern in FORBIDDEN_PATH_PATTERNS:
        if pattern in f"/{norm}/":
            return True
    for forbidden in FORBIDDEN_RUNTIME_PATHS:
        if norm.startswith(forbidden + "/") or norm == forbidden:
            return True
    return False


def _copy_tree_clean(src: str, dst: str) -> None:
    os.makedirs(dst, exist_ok=True)
    for root, dirs, files in os.walk(src):
        rel_root = os.path.relpath(root, src)
        if rel_root != ".":
            norm_rel = rel_root.replace(os.sep, "/")
            if _contains_forbidden_path(norm_rel):
                dirs.clear()
                continue
        dirs[:] = sorted([d for d in dirs if not _is_forbidden_staging_entry(d)])
        for d in dirs:
            src_dir = os.path.join(root, d)
            dst_dir = os.path.join(dst, rel_root, d) if rel_root != "." else os.path.join(dst, d)
            os.makedirs(dst_dir, exist_ok=True)
        for fname in sorted(files):
            if _is_forbidden_staging_entry(fname):
                continue
            src_file = os.path.join(root, fname)
            if rel_root != ".":
                norm_rel = rel_root.replace(os.sep, "/")
                if _contains_forbidden_path(norm_rel):
                    continue
                dst_file = os.path.join(dst, rel_root, fname)
            else:
                dst_file = os.path.join(dst, fname)
            shutil.copy2(src_file, dst_file)


def _extract_artifact_to(archive_path: str, target_dir: str, expected_prefix: Optional[str] = None) -> None:
    work_dir = tempfile.mkdtemp(prefix=".extract-")
    try:
        safe_extract(archive_path, work_dir)
        if expected_prefix:
            prefix_path = os.path.join(work_dir, expected_prefix)
            if os.path.isdir(prefix_path):
                src = prefix_path
            else:
                src = work_dir
        else:
            entries = sorted(os.listdir(work_dir))
            if len(entries) == 1 and os.path.isdir(os.path.join(work_dir, entries[0])):
                src = os.path.join(work_dir, entries[0])
            else:
                src = work_dir
        for item in os.listdir(src):
            s = os.path.join(src, item)
            d = os.path.join(target_dir, item)
            if os.path.isdir(s):
                shutil.copytree(s, d, dirs_exist_ok=True)
            else:
                shutil.copy2(s, d)
    finally:
        shutil.rmtree(work_dir, ignore_errors=True)


def _verify_artifact_sha256(component: ProgramComponent) -> None:
    record = component.artifact_record
    if not os.path.exists(component.source_path):
        raise ValidationError(f"Source path does not exist: {component.source_path}")
    actual_sha = sha256_file(component.source_path)
    if actual_sha != record.artifactSha256:
        raise ValidationError(
            f"SHA-256 mismatch for {record.componentId}: "
            f"expected={record.artifactSha256}, actual={actual_sha}"
        )


def assemble_program_tree(
    components: List[ProgramComponent],
    target_dir: str,
) -> ProgramTreeManifest:
    os.makedirs(target_dir, exist_ok=True)

    manifest_dir = os.path.join(target_dir, "manifest")
    os.makedirs(manifest_dir, exist_ok=True)

    processed_components: List[ProgramComponent] = []

    for comp in components:
        record = comp.artifact_record
        dest_dir = os.path.join(target_dir, comp.target_subdir)
        os.makedirs(dest_dir, exist_ok=True)

        if not os.path.exists(comp.source_path):
            raise BuildError(f"Artifact source path missing: {comp.source_path}")

        _verify_artifact_sha256(comp)

        if record.componentId in COMPONENT_EXTRACT_MAP:
            prefix, binary_relative = COMPONENT_EXTRACT_MAP[record.componentId]
            staging = tempfile.mkdtemp(prefix=f".{record.componentId}-")
            try:
                _extract_artifact_to(comp.source_path, staging, expected_prefix=prefix)
                binary_path = os.path.join(staging, binary_relative)
                if not os.path.exists(binary_path):
                    raise BuildError(
                        f"Expected binary not found after extraction: {binary_relative} "
                        f"from {record.componentId}"
                    )
                final_binary = os.path.join(dest_dir, os.path.basename(binary_relative))
                if os.path.isdir(final_binary):
                    shutil.rmtree(final_binary, ignore_errors=True)
                elif os.path.exists(final_binary):
                    os.remove(final_binary)
                shutil.copy2(binary_path, final_binary)
                os.chmod(final_binary, 0o755)
            finally:
                shutil.rmtree(staging, ignore_errors=True)
        elif record.artifactType == "tree":
            _copy_tree_clean(comp.source_path, dest_dir)
        elif record.artifactType == "archive":
            staging = tempfile.mkdtemp(prefix=f".{record.componentId}-")
            try:
                _extract_artifact_to(comp.source_path, staging)
                for item in os.listdir(staging):
                    s = os.path.join(staging, item)
                    d = os.path.join(dest_dir, item)
                    if os.path.isdir(s):
                        shutil.copytree(s, d, dirs_exist_ok=True)
                    else:
                        shutil.copy2(s, d)
            finally:
                shutil.rmtree(staging, ignore_errors=True)
        elif record.artifactType == "executable":
            final_path = os.path.join(dest_dir, os.path.basename(comp.source_path))
            shutil.copy2(comp.source_path, final_path)
            os.chmod(final_path, 0o755)
        else:
            raise BuildError(f"Unknown artifactType: {record.artifactType}")

        processed_components.append(comp)

    actual_roots = set()
    for item in os.listdir(target_dir):
        if item == "manifest":
            continue
        actual_roots.add(item)

    illegal_roots = actual_roots - ALLOWED_PROGRAM_ROOTS
    if illegal_roots:
        raise ValidationError(
            f"Illegal program tree roots detected: {sorted(illegal_roots)}"
        )

    for rel_root, dirs, files in os.walk(target_dir):
        rel = os.path.relpath(rel_root, target_dir)
        if rel != ".":
            norm_rel = rel.replace(os.sep, "/")
            if _contains_forbidden_path(norm_rel):
                raise ValidationError(
                    f"Forbidden path detected in program tree: {norm_rel}"
                )
        for fname in files:
            if _is_forbidden_staging_entry(fname):
                raise ValidationError(
                    f"Forbidden file detected in program tree: {os.path.join(rel, fname)}"
                )

    for required in REQUIRED_PROGRAM_PATHS:
        full = os.path.join(target_dir, required)
        if not os.path.exists(full):
            raise ValidationError(f"Required program path missing: {required}")

    manifest_lines = generate_tree_manifest(target_dir)
    tree_sha = sha256_tree_manifest(manifest_lines)

    return ProgramTreeManifest(
        components=processed_components,
        tree_sha256=tree_sha,
        manifest_lines=manifest_lines,
    )


def copy_manifest_files(
    target_dir: str,
    guest_layout_path: str,
    mount_contract_path: str,
) -> Tuple[str, str]:
    manifest_dir = os.path.join(target_dir, "manifest")
    os.makedirs(manifest_dir, exist_ok=True)

    layout_dest = os.path.join(manifest_dir, "guest-layout.json")
    contract_dest = os.path.join(manifest_dir, "mount-contract.json")

    if os.path.exists(guest_layout_path):
        shutil.copy2(guest_layout_path, layout_dest)
    else:
        raise FileNotFoundError(f"guest-layout.json not found: {guest_layout_path}")

    if os.path.exists(mount_contract_path):
        shutil.copy2(mount_contract_path, contract_dest)
    else:
        raise FileNotFoundError(f"mount-contract.json not found: {mount_contract_path}")

    return layout_dest, contract_dest
