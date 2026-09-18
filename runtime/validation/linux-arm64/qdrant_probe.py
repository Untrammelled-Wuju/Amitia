import json
import os
import time
from dataclasses import dataclass, field
from pathlib import Path
from typing import Dict, List, Optional, Tuple

try:
    from . import http_probe, process_runner
    from .errors import ValidationErrorCode, ValidationException
except ImportError:
    import http_probe, process_runner
    from errors import ValidationErrorCode, ValidationException


QDRANT_INTERNAL_PORT = 19178
DEFAULT_PROFILES = ["desktop-default", "mobile-compact", "mobile-balanced", "mobile-performance"]
MOBILE_PROFILES = ["mobile-compact", "mobile-balanced", "mobile-performance"]

PROFILE_DEFAULTS = {
    "desktop-default": {
        "service": {"host": "127.0.0.1", "http_port": 19178, "grpc_port": 19179},
        "cluster": {"enabled": False},
        "telemetry_disabled": True,
    },
    "mobile-compact": {
        "service": {"host": "127.0.0.1", "http_port": 19178, "grpc_port": 19179},
        "max_workers": 1,
        "max_search_threads": 1,
        "optimizer_cpu_budget": 0,
        "max_optimization_threads": 1,
        "max_indexing_threads": 1,
        "cluster": {"enabled": False},
        "telemetry_disabled": True,
    },
    "mobile-balanced": {
        "service": {"host": "127.0.0.1", "http_port": 19178, "grpc_port": 19179},
        "max_workers": 2,
        "max_search_threads": 2,
        "optimizer_cpu_budget": 1,
        "max_optimization_threads": 1,
        "max_indexing_threads": 2,
        "cluster": {"enabled": False},
        "telemetry_disabled": True,
    },
    "mobile-performance": {
        "service": {"host": "127.0.0.1", "http_port": 19178, "grpc_port": 19179},
        "max_workers": 4,
        "max_search_threads": 4,
        "optimizer_cpu_budget": 2,
        "max_optimization_threads": 2,
        "max_indexing_threads": 4,
        "cluster": {"enabled": False},
        "telemetry_disabled": True,
    },
}


@dataclass
class QdrantIdentity:
    title: str = ""
    version: str = ""
    raw: dict = field(default_factory=dict)

    def to_safe_dict(self) -> dict:
        return {
            "title": self.title,
            "version": self.version,
        }


@dataclass
class QdrantProbeResult:
    profile: str = ""
    started: bool = False
    ready: bool = False
    live: bool = False
    healthy: bool = False
    identity: Optional[QdrantIdentity] = None
    processStartMs: int = 0
    readyMs: int = 0
    peakRssKb: int = 0
    portConflictProtected: bool = False
    duplicateStartRejected: bool = False
    ownershipProtected: bool = False
    pid: int = 0
    errors: List[dict] = field(default_factory=list)
    warnings: List[str] = field(default_factory=list)

    def to_safe_dict(self) -> dict:
        return {
            "profile": self.profile,
            "started": self.started,
            "ready": self.ready,
            "live": self.live,
            "healthy": self.healthy,
            "identity": self.identity.to_safe_dict() if self.identity else None,
            "processStartMs": self.processStartMs,
            "readyMs": self.readyMs,
            "peakRssKb": self.peakRssKb,
            "portConflictProtected": self.portConflictProtected,
            "duplicateStartRejected": self.duplicateStartRejected,
            "ownershipProtected": self.ownershipProtected,
            "pid": self.pid,
            "errors": self.errors,
            "warnings": self.warnings,
        }


def load_qdrant_lock(lock_path: str) -> dict:
    if not os.path.exists(lock_path):
        return {}
    with open(lock_path, "r", encoding="utf-8") as f:
        data = json.load(f)
    return data if isinstance(data, dict) else {}


def render_qdrant_config(profile: str, runtime_root: str, data_root: str, config_path: str) -> dict:
    if profile not in PROFILE_DEFAULTS:
        raise ValidationException(
            ValidationErrorCode.QDRANT_PROFILE_INVALID,
            f"Unknown profile: {profile}",
        )
    params = PROFILE_DEFAULTS[profile]
    config = {
        "service": {
            "host": params["service"]["host"],
            "http_port": params["service"]["http_port"],
            "grpc_port": params["service"]["grpc_port"],
        },
        "storage": {
            "storage_path": os.path.join(data_root, "providers/qdrant/storage"),
            "snapshots_path": os.path.join(data_root, "providers/qdrant/snapshots"),
        },
        "cluster": params["cluster"],
        "telemetry_disabled": params["telemetry_disabled"],
    }
    if "max_workers" in params:
        config["max_workers"] = params["max_workers"]
    if "max_search_threads" in params:
        config["max_search_threads"] = params["max_search_threads"]
    if "optimizer_cpu_budget" in params:
        config["optimizer_cpu_budget"] = params["optimizer_cpu_budget"]
    if "max_optimization_threads" in params:
        config["max_optimization_threads"] = params["max_optimization_threads"]
    if "max_indexing_threads" in params:
        config["max_indexing_threads"] = params["max_indexing_threads"]

    os.makedirs(os.path.dirname(config_path), exist_ok=True)
    with open(config_path, "w", encoding="utf-8") as f:
        json.dump(config, f, indent=2, sort_keys=True)
    return config


def verify_qdrant_config(config_path: str, expected_profile: str) -> List[str]:
    errors = []
    if not os.path.exists(config_path):
        errors.append(f"Config not found: {config_path}")
        return errors
    with open(config_path, "r", encoding="utf-8") as f:
        config = json.load(f)
    if expected_profile not in PROFILE_DEFAULTS:
        errors.append(f"Unknown profile: {expected_profile}")
        return errors
    expected = PROFILE_DEFAULTS[expected_profile]
    service = config.get("service", {})
    if service.get("host") != expected["service"]["host"]:
        errors.append(f"Expected host {expected['service']['host']}, got {service.get('host')}")
    for key, value in expected.items():
        if key in ("service", "cluster", "telemetry_disabled", "storage"):
            continue
        actual = config.get(key)
        if actual != value:
            errors.append(f"Profile {expected_profile}: expected {key}={value}, got {actual}")
    cluster = config.get("cluster", {})
    if cluster.get("enabled") != expected["cluster"]["enabled"]:
        errors.append(f"Expected cluster.enabled={expected['cluster']['enabled']}, got {cluster.get('enabled')}")
    telemetry_disabled = config.get("telemetry_disabled")
    if telemetry_disabled != expected["telemetry_disabled"]:
        errors.append(f"Expected telemetry_disabled={expected['telemetry_disabled']}, got {telemetry_disabled}")
    return errors


def query_qdrant_identity(host: str = "127.0.0.1", port: int = 19178) -> Optional[QdrantIdentity]:
    result = http_probe.http_get_json(f"http://{host}:{port}/")
    if not result.ok:
        return None
    try:
        payload = json.loads(result.body)
    except json.JSONDecodeError:
        payload = {}
    title = str(payload.get("title", ""))
    version = str(payload.get("version", ""))
    if not title:
        return None
    return QdrantIdentity(title=title, version=version, raw=payload)


def check_qdrant_live(host: str = "127.0.0.1", port: int = 19178) -> bool:
    result = http_probe.http_get_json(f"http://{host}:{port}/livez")
    return result.ok and result.status == 200


def check_qdrant_ready(host: str = "127.0.0.1", port: int = 19178) -> bool:
    result = http_probe.http_get_json(f"http://{host}:{port}/readyz")
    return result.ok and result.status == 200


def check_qdrant_healthy(host: str = "127.0.0.1", port: int = 19178) -> bool:
    result = http_probe.http_get_json(f"http://{host}:{port}/healthz")
    return result.ok and result.status == 200


def wait_for_qdrant_ready(timeout: float = 60.0, host: str = "127.0.0.1", port: int = 19178) -> bool:
    deadline = time.time() + timeout
    while time.time() < deadline:
        if check_qdrant_ready(host, port):
            return True
        if not process_alive_check(port):
            return False
        time.sleep(0.5)
    return check_qdrant_ready(host, port)


def process_alive_check(port: int) -> bool:
    return True


def scan_qdrant_logs(stderr_text: str, profile: str) -> List[str]:
    errors = []
    if not stderr_text:
        return errors
    lowered = stderr_text.lower()
    if "illegal instruction" in lowered:
        errors.append(ValidationErrorCode.ILLEGAL_INSTRUCTION)
    if "exec format error" in lowered:
        errors.append(ValidationErrorCode.EXEC_FORMAT_ERROR)
    if "segmentation fault" in lowered:
        errors.append("SEGFAULT")
    if "stack overflow" in lowered:
        errors.append("STACK_OVERFLOW")
    if "fatal runtime error" in lowered:
        errors.append("FATAL_RUNTIME_ERROR")
    return errors


def get_rss_kb(pid: int) -> int:
    try:
        with open(f"/proc/{pid}/status", "r", encoding="utf-8", errors="replace") as f:
            for line in f:
                if line.startswith("VmRSS:"):
                    parts = line.split()
                    if len(parts) >= 2:
                        return int(parts[1])
    except (OSError, ValueError):
        pass
    return 0
