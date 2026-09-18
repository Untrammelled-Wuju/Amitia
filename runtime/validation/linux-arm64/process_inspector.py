import os
import re
from dataclasses import dataclass
from pathlib import Path
from typing import Dict, List, Optional


AMITIA_SERVER_PROC_NAMES = {"amitia-server", "AmitiaCore", "server.exe"}
QDRANT_PROC_NAMES = {"qdrant"}
NODE_SCRIPT_MARKERS = {"amitia-plugin-host", "amitia-task-host", "amitia-node", "sidecar"}


@dataclass
class ProcessIdentity:
    pid: int
    name: str
    exe: str = ""
    cmdline: List[str] = None
    isAmitiaComponent: bool = False

    def __post_init__(self):
        if self.cmdline is None:
            self.cmdline = []

    def to_safe_dict(self) -> dict:
        return {
            "pid": self.pid,
            "name": self.name,
            "exe": self.exe,
            "cmdlineLength": len(self.cmdline),
            "isAmitiaComponent": self.isAmitiaComponent,
        }


def list_amitia_processes() -> List[ProcessIdentity]:
    results = []
    proc_root = Path("/proc")
    if not proc_root.exists():
        return results
    for entry in proc_root.iterdir():
        if not entry.name.isdigit():
            continue
        pid = int(entry.name)
        try:
            exe_link = os.readlink(str(entry / "exe"))
        except (OSError, PermissionError):
            exe_link = ""
        try:
            with open(str(entry / "comm"), "r", encoding="utf-8", errors="replace") as f:
                name = f.read().strip()
        except (OSError, PermissionError):
            name = ""
        try:
            with open(str(entry / "cmdline"), "rb") as f:
                raw = f.read()
            cmdline = [part.decode("utf-8", errors="replace") for part in raw.split(b"\x00") if part]
        except (OSError, PermissionError):
            cmdline = []
        is_amitia = _is_amitia_process(name, cmdline, exe_link)
        results.append(
            ProcessIdentity(
                pid=pid,
                name=name,
                exe=exe_link,
                cmdline=cmdline,
                isAmitiaComponent=is_amitia,
            )
        )
    return results


def _is_amitia_process(name: str, cmdline: List[str], exe: str) -> bool:
    combined = " ".join(cmdline).lower() if cmdline else ""
    if any(marker in name.lower() for marker in AMITIA_SERVER_PROC_NAMES):
        return True
    if any(marker in exe.lower() for marker in AMITIA_SERVER_PROC_NAMES):
        return True
    if "qdrant" in name.lower() and "amitia" in combined:
        return True
    if "node" in name.lower():
        if any(marker in combined for marker in NODE_SCRIPT_MARKERS):
            return True
        if any(marker in exe for marker in NODE_SCRIPT_MARKERS):
            return True
    if "amitia" in combined:
        return True
    if "/opt/amitia" in exe:
        return True
    return False


def find_processes_by_pid(pids: List[int]) -> Dict[int, bool]:
    result = {}
    for pid in pids:
        try:
            os.kill(pid, 0)
            result[pid] = True
        except ProcessLookupError:
            result[pid] = False
        except OSError:
            result[pid] = True
    return result


def collect_system_node_processes() -> List[ProcessIdentity]:
    results = []
    proc_root = Path("/proc")
    if not proc_root.exists():
        return results
    for entry in proc_root.iterdir():
        if not entry.name.isdigit():
            continue
        pid = int(entry.name)
        try:
            with open(str(entry / "comm"), "r", encoding="utf-8", errors="replace") as f:
                name = f.read().strip()
        except (OSError, PermissionError):
            continue
        try:
            with open(str(entry / "cmdline"), "rb") as f:
                raw = f.read()
            cmdline = [part.decode("utf-8", errors="replace") for part in raw.split(b"\x00") if part]
        except (OSError, PermissionError):
            cmdline = []
        if name.lower() == "node" and not _is_amitia_process(name, cmdline, ""):
            results.append(
                ProcessIdentity(pid=pid, name=name, cmdline=cmdline, isAmitiaComponent=False)
            )
    return results


def collect_qdrant_processes() -> List[ProcessIdentity]:
    results = []
    proc_root = Path("/proc")
    if not proc_root.exists():
        return results
    for entry in proc_root.iterdir():
        if not entry.name.isdigit():
            continue
        pid = int(entry.name)
        try:
            with open(str(entry / "comm"), "r", encoding="utf-8", errors="replace") as f:
                name = f.read().strip()
        except (OSError, PermissionError):
            continue
        if name.lower() == "qdrant":
            results.append(ProcessIdentity(pid=pid, name=name, isAmitiaComponent=False))
    return results


def collect_node_processes() -> List[ProcessIdentity]:
    results = []
    proc_root = Path("/proc")
    if not proc_root.exists():
        return results
    for entry in proc_root.iterdir():
        if not entry.name.isdigit():
            continue
        pid = int(entry.name)
        try:
            with open(str(entry / "comm"), "r", encoding="utf-8", errors="replace") as f:
                name = f.read().strip()
        except (OSError, PermissionError):
            continue
        if name.lower() == "node":
            try:
                with open(str(entry / "cmdline"), "rb") as f:
                    raw = f.read()
                cmdline = [part.decode("utf-8", errors="replace") for part in raw.split(b"\x00") if part]
            except (OSError, PermissionError):
                cmdline = []
            results.append(
                ProcessIdentity(pid=pid, name=name, cmdline=cmdline, isAmitiaComponent=False)
            )
    return results
