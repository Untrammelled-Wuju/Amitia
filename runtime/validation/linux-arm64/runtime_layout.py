import hashlib
import json
import os
import stat
from dataclasses import dataclass, field
from enum import Enum
from pathlib import Path
from typing import Dict, List, Optional, Set

try:
    from .errors import ValidationErrorCode
except ImportError:
    from errors import ValidationErrorCode


RUNTIME_ROOT = "/opt/amitia"
CONFIG_ROOT = "/etc/amitia"
DATA_ROOT = "/var/lib/amitia"
CACHE_ROOT = "/var/cache/amitia"
LOG_ROOT = "/var/log/amitia"
RUN_ROOT = "/run/amitia"
TEMP_ROOT = "/run/amitia/tmp"
WORKSPACE_ROOT = "/var/lib/amitia/workspaces"

RUNTIME_REQUIRED_ENTRIES = {
    "backend",
    "node",
    "qdrant",
    "plugin-host",
    "task-host",
    "scripts",
    "manifest",
    "licenses",
}

RUNTIME_FORBIDDEN_PATHS = {
    "config",
    "data",
    "log",
}

PROVIDER_QDRANT_DATA_PATH = "providers/qdrant/storage"
PROVIDER_QDRANT_SNAPSHOTS_PATH = "providers/qdrant/snapshots"
NODE_HOME_PATH = "node/home"
NODE_PREFIX_PATH = "node/prefix"
NODE_CACHE_PATH = "node"
SECURITY_TOKEN_WHITELIST = [
    "security/local-token",
    "security/.local-token",
    "local-token",
]


class DirectoryRole(str, Enum):
    PROGRAM = "program"
    CONFIG = "config"
    DATA = "data"
    CACHE = "cache"
    RUN = "run"
    LOG = "log"


@dataclass
class FileEntry:
    path: str
    type: str
    mode: int
    size: int
    sha256: Optional[str] = None

    def to_safe_dict(self) -> dict:
        return {
            "path": self.path,
            "type": self.type,
            "mode": self.mode,
            "size": self.size,
            "sha256": self.sha256,
        }


@dataclass
class TreeManifest:
    root: str
    entries: List[FileEntry] = field(default_factory=list)

    def compute_sha(self) -> str:
        hasher = hashlib.sha256()
        for entry in sorted(self.entries, key=lambda e: e.path):
            hasher.update(entry.path.encode("utf-8"))
            hasher.update(b"|")
            hasher.update(f"{entry.type}:{entry.mode}:{entry.size}".encode("utf-8"))
            if entry.sha256:
                hasher.update(b"|")
                hasher.update(entry.sha256.encode("utf-8"))
            hasher.update(b"\n")
        return hasher.hexdigest()

    def to_safe_dict(self) -> dict:
        return {
            "root": self.root,
            "sha": self.compute_sha(),
            "entries": [entry.to_safe_dict() for entry in self.entries],
        }


@dataclass
class GuestLayout:
    runtimeRoot: str = RUNTIME_ROOT
    configRoot: str = CONFIG_ROOT
    dataRoot: str = DATA_ROOT
    cacheRoot: str = CACHE_ROOT
    logRoot: str = LOG_ROOT
    runRoot: str = RUN_ROOT
    tempRoot: str = TEMP_ROOT
    workspaceRoot: str = WORKSPACE_ROOT


def ensure_directory_structure(layout: GuestLayout) -> None:
    roots = [
        layout.runtimeRoot,
        layout.configRoot,
        layout.dataRoot,
        layout.cacheRoot,
        layout.logRoot,
        layout.runRoot,
        layout.tempRoot,
        layout.workspaceRoot,
        os.path.join(layout.dataRoot, PROVIDER_QDRANT_DATA_PATH),
        os.path.join(layout.dataRoot, PROVIDER_QDRANT_SNAPSHOTS_PATH),
        os.path.join(layout.dataRoot, NODE_HOME_PATH),
        os.path.join(layout.dataRoot, NODE_PREFIX_PATH),
        os.path.join(layout.cacheRoot, NODE_CACHE_PATH),
        os.path.join(layout.runRoot, "process"),
    ]
    for path in roots:
        os.makedirs(path, exist_ok=True)


def scan_tree(root: str, compute_hash: bool = True) -> TreeManifest:
    manifest = TreeManifest(root=root)
    root_path = Path(root)
    if not root_path.exists():
        return manifest
    for dirpath, dirnames, filenames in os.walk(root):
        rel_dir = os.path.relpath(dirpath, root)
        if rel_dir == ".":
            rel_dir = ""
        for name in sorted(dirnames):
            full = os.path.join(dirpath, name)
            rel = os.path.join(rel_dir, name) if rel_dir else name
            try:
                st = os.lstat(full)
                entry_type = "symlink" if stat.S_ISLNK(st.st_mode) else "dir"
                manifest.entries.append(
                    FileEntry(
                        path=rel,
                        type=entry_type,
                        mode=stat.S_IMODE(st.st_mode),
                        size=0,
                    )
                )
            except OSError:
                continue
        for name in sorted(filenames):
            full = os.path.join(dirpath, name)
            rel = os.path.join(rel_dir, name) if rel_dir else name
            try:
                st = os.lstat(full)
                if stat.S_ISLNK(st.st_mode):
                    entry_type = "symlink"
                    sha = None
                elif stat.S_ISREG(st.st_mode):
                    entry_type = "file"
                    sha = _sha256_file(full) if compute_hash else None
                else:
                    entry_type = "other"
                    sha = None
                manifest.entries.append(
                    FileEntry(
                        path=rel,
                        type=entry_type,
                        mode=stat.S_IMODE(st.st_mode),
                        size=st.st_size,
                        sha256=sha,
                    )
                )
            except OSError:
                continue
    return manifest


def _sha256_file(path: str) -> str:
    digest = hashlib.sha256()
    with open(path, "rb") as f:
        for chunk in iter(lambda: f.read(65536), b""):
            digest.update(chunk)
    return digest.hexdigest()


def compare_tree_manifests(original: TreeManifest, current: TreeManifest) -> dict:
    original_map = {entry.path: entry for entry in original.entries}
    current_map = {entry.path: entry for entry in current.entries}
    added = []
    removed = []
    modified = []
    for path, entry in current_map.items():
        if path not in original_map:
            added.append(path)
        else:
            old_entry = original_map[path]
            if old_entry.sha256 and entry.sha256 and old_entry.sha256 != entry.sha256:
                modified.append(path)
            elif old_entry.mode != entry.mode or old_entry.size != entry.size:
                modified.append(path)
    for path in original_map:
        if path not in current_map:
            removed.append(path)
    changed = bool(added or removed or modified)
    return {
        "changed": changed,
        "added": added,
        "removed": removed,
        "modified": modified,
    }


def validate_runtime_root_contents() -> List[str]:
    errors = []
    runtime_path = Path(RUNTIME_ROOT)
    if not runtime_path.exists():
        errors.append(f"Runtime root not found: {RUNTIME_ROOT}")
        return errors
    present = {entry.name for entry in runtime_path.iterdir()}
    missing = RUNTIME_REQUIRED_ENTRIES - present
    if missing:
        errors.append(f"Missing runtime entries: {missing}")
    for forbidden in RUNTIME_FORBIDDEN_PATHS:
        if forbidden in present:
            errors.append(f"Forbidden path in runtime root: {forbidden}")
    return errors


def classify_path_against_layout(absolute_path: str, layout: GuestLayout) -> Optional[DirectoryRole]:
    targets = [
        (layout.runtimeRoot, DirectoryRole.PROGRAM),
        (layout.configRoot, DirectoryRole.CONFIG),
        (layout.dataRoot, DirectoryRole.DATA),
        (layout.cacheRoot, DirectoryRole.CACHE),
        (layout.logRoot, DirectoryRole.LOG),
        (layout.runRoot, DirectoryRole.RUN),
        (layout.tempRoot, DirectoryRole.RUN),
        (layout.workspaceRoot, DirectoryRole.DATA),
    ]
    for prefix, role in targets:
        if absolute_path == prefix or absolute_path.startswith(prefix + "/"):
            return role
    return None


def validate_data_isolation(layout: GuestLayout) -> List[str]:
    errors = []
    runtime_path = Path(RUNTIME_ROOT)
    if not runtime_path.exists():
        return errors
    for dirpath, dirnames, filenames in os.walk(str(runtime_path)):
        for name in filenames:
            if name.endswith((".db", ".sqlite", ".sqlite3", ".wal", ".shm")):
                errors.append(f"Database found in program directory: {os.path.join(dirpath, name)}")
    config_path = Path(layout.configRoot)
    if config_path.exists():
        for dirpath, dirnames, filenames in os.walk(str(config_path)):
            for name in filenames:
                if name.endswith((".db", ".sqlite", ".sqlite3", ".log", ".pid", ".sock")):
                    errors.append(f"Non-config file in config directory: {os.path.join(dirpath, name)}")
    return errors
