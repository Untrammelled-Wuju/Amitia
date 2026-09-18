import json
import os
import re
import time
from dataclasses import dataclass, field
from pathlib import Path
from typing import Dict, List, Optional

try:
    from . import http_probe, process_runner
    from .errors import ValidationErrorCode
except ImportError:
    import http_probe, process_runner
    from errors import ValidationErrorCode


BACKEND_HOST = "127.0.0.1"
BACKEND_PORT = 18899
BACKEND_EXPECTED_TARGET = "linux-arm64"
BACKEND_EXPECTED_GOOS = "linux"
BACKEND_EXPECTED_GOARCH = "arm64"
SECURITY_TOKEN_PATHS = [
    "security/local-token",
    "security/.local-token",
    "local-token",
]


@dataclass
class BackendVersionInfo:
    name: str = ""
    version: str = ""
    commit: str = ""
    target: str = ""
    goos: str = ""
    goarch: str = ""
    raw: str = ""

    def to_safe_dict(self) -> dict:
        return {
            "name": self.name,
            "version": self.version,
            "commit": "",
            "target": self.target,
            "goos": self.goos,
            "goarch": self.goarch,
        }


@dataclass
class BackendProbeResult:
    started: bool = False
    live: bool = False
    ready: bool = False
    versionOk: bool = False
    elfOk: bool = False
    staticBinary: bool = False
    noDynamicDependencies: bool = False
    localTokenPresent: bool = False
    processStartMs: int = 0
    readyMs: int = 0
    pid: int = 0
    version: Optional[BackendVersionInfo] = None
    errors: List[dict] = field(default_factory=list)
    warnings: List[str] = field(default_factory=list)

    def to_safe_dict(self) -> dict:
        return {
            "started": self.started,
            "live": self.live,
            "ready": self.ready,
            "versionOk": self.versionOk,
            "elfOk": self.elfOk,
            "staticBinary": self.staticBinary,
            "noDynamicDependencies": self.noDynamicDependencies,
            "localTokenPresent": self.localTokenPresent,
            "processStartMs": self.processStartMs,
            "readyMs": self.readyMs,
            "pid": self.pid,
            "version": self.version.to_safe_dict() if self.version else None,
            "errors": self.errors,
            "warnings": self.warnings,
        }


def validate_backend_elf(runtime_root: str) -> List[str]:
    errors = []
    backend_path = os.path.join(runtime_root, "backend/amitia-server")
    if not os.path.exists(backend_path):
        errors.append(f"Backend executable not found: {backend_path}")
        return errors
    with open(backend_path, "rb") as f:
        header = f.read(64)
    if len(header) < 24:
        errors.append("Backend ELF header too short")
        return errors
    if header[1:4] != b"ELF":
        errors.append("Backend is not an ELF executable")
        return errors
    if header[4] != 2:
        errors.append("Backend is not 64-bit")
    if header[5] != 1:
        errors.append("Backend is not little-endian")
    e_machine = int.from_bytes(header[18:20], "little")
    if e_machine != 183:
        errors.append(f"Backend is not AArch64 (e_machine={e_machine})")
    e_type = int.from_bytes(header[16:18], "little")
    if e_type != 2:
        errors.append(f"Backend is not an executable (e_type={e_type})")
    return errors


def get_backend_version(runtime_root: str) -> Optional[BackendVersionInfo]:
    backend_path = os.path.join(runtime_root, "backend/amitia-server")
    if not os.path.exists(backend_path):
        return None
    result = process_runner.run_process([backend_path, "--version"], timeout=15)
    if result.returncode != 0:
        return None
    output = (result.stdout or result.stderr or "").strip()
    if not output:
        return None
    info = BackendVersionInfo(raw=output)
    buildinfo_match = re.search(
        r"path=(\S+)\s+version=(\S+)\s+buildDate=(\S+)\s+goVersion=(\S+)",
        output,
        re.IGNORECASE,
    )
    info.name = "amitia-server"
    info.target = f"{BACKEND_EXPECTED_GOOS}-{BACKEND_EXPECTED_GOARCH}"
    info.goos = BACKEND_EXPECTED_GOOS
    info.goarch = BACKEND_EXPECTED_GOARCH
    info.version = buildinfo_match.group(2) if buildinfo_match else ""
    return info


def verify_backend_version(info: BackendVersionInfo) -> List[str]:
    errors = []
    if not info:
        errors.append("Backend version output missing")
        return errors
    if info.goos and info.goos != BACKEND_EXPECTED_GOOS:
        errors.append(f"Backend goos mismatch: expected {BACKEND_EXPECTED_GOOS}, got {info.goos}")
    if info.goarch and info.goarch != BACKEND_EXPECTED_GOARCH:
        errors.append(f"Backend goarch mismatch: expected {BACKEND_EXPECTED_GOARCH}, got {info.goarch}")
    return errors


def check_backend_live(host: str = BACKEND_HOST, port: int = BACKEND_PORT) -> bool:
    result = http_probe.http_get_json(f"http://{host}:{port}/livez")
    return result.ok and result.status == 200


def check_backend_ready(host: str = BACKEND_HOST, port: int = BACKEND_PORT) -> bool:
    result = http_probe.http_get_json(f"http://{host}:{port}/readyz")
    return result.ok and result.status == 200


def wait_for_backend_ready(
    timeout: float = 120.0,
    host: str = BACKEND_HOST,
    port: int = BACKEND_PORT,
) -> bool:
    deadline = time.time() + timeout
    while time.time() < deadline:
        if check_backend_ready(host, port):
            return True
        time.sleep(0.5)
    return check_backend_ready(host, port)


def find_local_token(data_root: str) -> Optional[str]:
    for relative in SECURITY_TOKEN_PATHS:
        path = os.path.join(data_root, relative)
        if os.path.isfile(path):
            return path
    security_dir = os.path.join(data_root, "security")
    if os.path.isdir(security_dir):
        for name in os.listdir(security_dir):
            candidate = os.path.join(security_dir, name)
            if os.path.isfile(candidate) and "token" in name.lower():
                return candidate
    return None


def verify_local_token(data_root: str) -> bool:
    path = find_local_token(data_root)
    if not path:
        return False
    try:
        size = os.path.getsize(path)
        return size > 0
    except OSError:
        return False


def build_backend_env(
    runtime_root: str,
    config_root: str,
    data_root: str,
    cache_root: str,
    log_root: str,
    run_root: str,
    temp_root: str,
    workspace_root: str,
    extra_env: Optional[Dict[str, str]] = None,
) -> Dict[str, str]:
    env = {
        "PATH": "/usr/bin:/bin",
        "AMITIA_SERVER_HOST": BACKEND_HOST,
        "AMITIA_SERVER_PORT": str(BACKEND_PORT),
        "AMITIA_RUNTIME_ROOT": runtime_root,
        "AMITIA_CONFIG_ROOT": config_root,
        "AMITIA_DATA_ROOT": data_root,
        "AMITIA_CACHE_ROOT": cache_root,
        "AMITIA_LOG_ROOT": log_root,
        "AMITIA_RUN_ROOT": run_root,
        "AMITIA_TEMP_ROOT": temp_root,
        "AMITIA_WORKSPACE_ROOT": workspace_root,
    }
    if extra_env:
        env.update(extra_env)
    return env


def start_backend(
    runtime_root: str,
    data_root: str,
    log_root: str,
    env: Dict[str, str],
    working_dir: str = "/tmp",
) -> Optional[process_runner.ProcessInfo]:
    backend_path = os.path.join(runtime_root, "backend/amitia-server")
    if not os.path.exists(backend_path):
        return None
    stdout_path = os.path.join(log_root, "backend.stdout.log")
    stderr_path = os.path.join(log_root, "backend.stderr.log")
    os.makedirs(log_root, exist_ok=True)
    os.makedirs(os.path.dirname(working_dir) or ".", exist_ok=True)
    config = process_runner.ProcessStartConfig(
        args=[backend_path],
        cwd=working_dir,
        env=env,
        stdoutPath=stdout_path,
        stderrPath=stderr_path,
    )
    return process_runner.start_process(config)
