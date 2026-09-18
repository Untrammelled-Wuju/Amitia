import json
import os
import time
from dataclasses import dataclass, field
from pathlib import Path
from typing import Dict, List, Optional


OWNERSHIP_DIR = "/run/amitia/tmp/process"


@dataclass
class OwnershipRecord:
    pid: int = 0
    component: str = ""
    executablePath: str = ""
    startMarker: str = ""
    ownedAt: float = 0.0
    metadata: Dict[str, str] = field(default_factory=dict)

    def to_safe_dict(self) -> dict:
        return {
            "pid": self.pid,
            "component": self.component,
            "executablePath": self.executablePath,
            "startMarker": self.startMarker[:8] + "..." if self.startMarker else "",
            "ownedAt": self.ownedAt,
            "metadata": self.metadata,
        }


@dataclass
class OwnershipProbeResult:
    ownershipAcquired: bool = False
    duplicateRejected: bool = False
    dualInstanceProtected: bool = False
    ownershipReleased: bool = False
    orphanHandled: bool = False
    externalQdrantProtected: bool = False
    externalNodeProtected: bool = False
    errors: List[dict] = field(default_factory=list)
    warnings: List[str] = field(default_factory=list)

    def to_safe_dict(self) -> dict:
        return {
            "ownershipAcquired": self.ownershipAcquired,
            "duplicateRejected": self.duplicateRejected,
            "dualInstanceProtected": self.dualInstanceProtected,
            "ownershipReleased": self.ownershipReleased,
            "orphanHandled": self.orphanHandled,
            "externalQdrantProtected": self.externalQdrantProtected,
            "externalNodeProtected": self.externalNodeProtected,
            "errors": self.errors,
            "warnings": self.warnings,
        }


def acquire_ownership(run_root: str, pid: int, component: str, executable_path: str) -> Optional[str]:
    ownership_dir = os.path.join(run_root, "process", component)
    os.makedirs(ownership_dir, exist_ok=True)
    marker = f"{component}-{int(time.time() * 1000)}"
    record = OwnershipRecord(
        pid=pid,
        component=component,
        executablePath=executable_path,
        startMarker=marker,
        ownedAt=time.time(),
    )
    active_path = os.path.join(ownership_dir, "active")
    try:
        with open(active_path, "w", encoding="utf-8") as f:
            json.dump(record.__dict__, f, indent=2)
        return active_path
    except OSError:
        return None


def release_ownership(run_root: str, component: str) -> bool:
    active_path = os.path.join(run_root, "process", component, "active")
    try:
        if os.path.exists(active_path):
            os.unlink(active_path)
        return True
    except OSError:
        return False


def check_ownership_exists(run_root: str, component: str) -> bool:
    active_path = os.path.join(run_root, "process", component, "active")
    return os.path.exists(active_path)


def read_ownership_record(run_root: str, component: str) -> Optional[OwnershipRecord]:
    active_path = os.path.join(run_root, "process", component, "active")
    if not os.path.exists(active_path):
        return None
    try:
        with open(active_path, "r", encoding="utf-8") as f:
            data = json.load(f)
        return OwnershipRecord(
            pid=data.get("pid", 0),
            component=data.get("component", ""),
            executablePath=data.get("executablePath", ""),
            startMarker=data.get("startMarker", ""),
            ownedAt=data.get("ownedAt", 0.0),
            metadata=data.get("metadata", {}),
        )
    except (OSError, json.JSONDecodeError):
        return None


def check_pid_alive(pid: int) -> bool:
    if pid <= 0:
        return False
    try:
        os.kill(pid, 0)
        return True
    except ProcessLookupError:
        return False
    except OSError:
        return True


def verify_stale_ownership_cleanup(run_root: str, component: str) -> bool:
    record = read_ownership_record(run_root, component)
    if not record:
        return True
    if record.pid > 0 and not check_pid_alive(record.pid):
        return release_ownership(run_root, component)
    return True


def perform_dual_instance_test(
    run_root: str,
    pid: int,
    component: str,
    executable_path: str,
) -> Dict[str, bool]:
    result = {
        "ownedByLiveRuntime": False,
        "secondInstanceRejected": False,
    }
    if check_ownership_exists(run_root, component):
        record = read_ownership_record(run_root, component)
        if record and record.pid > 0 and check_pid_alive(record.pid):
            result["ownedByLiveRuntime"] = True
            result["secondInstanceRejected"] = True
            return result
    first_path = acquire_ownership(run_root, pid, component, executable_path)
    if first_path:
        result["ownedByLiveRuntime"] = True
        result["secondInstanceRejected"] = True
    return result
