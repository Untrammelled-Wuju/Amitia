import os
import time
import uuid
from dataclasses import dataclass, field
from typing import List, Optional


DATA_MARKER_FILENAME = ".amitia-validation-marker"


@dataclass
class DataProbeResult:
    dataCreated: bool = False
    dataPersistedAfterRestart: bool = False
    dataPersistedAfterCacheCleanup: bool = False
    dataPersistedAfterRunCleanup: bool = False
    sqliteInDataRoot: bool = False
    qdrantDataInDataRoot: bool = False
    nodeDataInDataRoot: bool = False
    logsNotInProgramDir: bool = False
    databasesNotInProgramDir: bool = False
    errors: List[dict] = field(default_factory=list)
    warnings: List[str] = field(default_factory=None)

    def __post_init__(self):
        if self.warnings is None:
            self.warnings = []

    def to_safe_dict(self) -> dict:
        return {
            "dataCreated": self.dataCreated,
            "dataPersistedAfterRestart": self.dataPersistedAfterRestart,
            "dataPersistedAfterCacheCleanup": self.dataPersistedAfterCacheCleanup,
            "dataPersistedAfterRunCleanup": self.dataPersistedAfterRunCleanup,
            "sqliteInDataRoot": self.sqliteInDataRoot,
            "qdrantDataInDataRoot": self.qdrantDataInDataRoot,
            "nodeDataInDataRoot": self.nodeDataInDataRoot,
            "logsNotInProgramDir": self.logsNotInProgramDir,
            "databasesNotInProgramDir": self.databasesNotInProgramDir,
            "errors": self.errors,
            "warnings": self.warnings,
        }


def create_data_marker(data_root: str) -> Optional[str]:
    os.makedirs(data_root, exist_ok=True)
    marker = str(uuid.uuid4())
    path = os.path.join(data_root, DATA_MARKER_FILENAME)
    try:
        with open(path, "w", encoding="utf-8") as f:
            f.write(marker)
        return marker
    except OSError:
        return None


def verify_data_marker(data_root: str, expected_marker: str) -> bool:
    path = os.path.join(data_root, DATA_MARKER_FILENAME)
    if not os.path.exists(path):
        return False
    try:
        with open(path, "r", encoding="utf-8") as f:
            content = f.read().strip()
        return content == expected_marker
    except OSError:
        return False


def scan_for_databases(runtime_root: str) -> List[str]:
    import pathlib
    findings = []
    root_path = pathlib.Path(runtime_root)
    if not root_path.exists():
        return findings
    for dirpath, dirnames, filenames in os.walk(runtime_root):
        for name in filenames:
            if name.endswith((".db", ".sqlite", ".sqlite3", ".wal", ".shm", ".db-wal")):
                findings.append(os.path.join(dirpath, name))
    return findings


def scan_for_logs(runtime_root: str) -> List[str]:
    findings = []
    for dirpath, dirnames, filenames in os.walk(runtime_root):
        for name in filenames:
            if name.endswith((".log", ".log.1", ".log.2")):
                findings.append(os.path.join(dirpath, name))
    return findings


def verify_sqlite_location(data_root: str) -> bool:
    expected_sqlite_paths = [
        os.path.join(data_root, "app.db"),
        os.path.join(data_root, "amitia.db"),
        os.path.join(data_root, "data", "app.db"),
        os.path.join(data_root, "data", "amitia.db"),
    ]
    for path in expected_sqlite_paths:
        if os.path.exists(path):
            return True
    for dirpath, dirnames, filenames in os.walk(data_root):
        for name in filenames:
            if name.endswith((".db", ".sqlite", ".sqlite3")):
                return True
    return False


def verify_qdrant_data_location(data_root: str) -> bool:
    expected_path = os.path.join(data_root, "providers/qdrant/storage")
    return os.path.isdir(expected_path)


def verify_node_data_location(data_root: str) -> bool:
    node_home = os.path.join(data_root, "node/home")
    node_prefix = os.path.join(data_root, "node/prefix")
    return os.path.isdir(node_home) and os.path.isdir(node_prefix)


def cleanup_test_data(data_root: str) -> None:
    marker_path = os.path.join(data_root, DATA_MARKER_FILENAME)
    try:
        if os.path.exists(marker_path):
            os.unlink(marker_path)
    except OSError:
        pass
