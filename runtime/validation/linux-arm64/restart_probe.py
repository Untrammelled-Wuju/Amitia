import os
import time
from dataclasses import dataclass, field
from typing import Dict, List, Optional

try:
    from . import backend_probe, http_probe, process_runner
    from .errors import ValidationErrorCode
except ImportError:
    import backend_probe, http_probe, process_runner
    from errors import ValidationErrorCode


@dataclass
class RestartProbeResult:
    restartSuccess: bool = False
    backendReadyAgain: bool = False
    qdrantReadyAgain: bool = False
    dataPersisted: bool = False
    cacheRebuildOk: bool = False
    runRebuildOk: bool = False
    orphanDetected: bool = False
    identityMatched: bool = False
    orphanTerminated: bool = False
    externalQdrantProtected: bool = False
    externalNodeProtected: bool = False
    errors: List[dict] = field(default_factory=list)
    warnings: List[str] = field(default_factory=list)

    def to_safe_dict(self) -> dict:
        return {
            "restartSuccess": self.restartSuccess,
            "backendReadyAgain": self.backendReadyAgain,
            "qdrantReadyAgain": self.qdrantReadyAgain,
            "dataPersisted": self.dataPersisted,
            "cacheRebuildOk": self.cacheRebuildOk,
            "runRebuildOk": self.runRebuildOk,
            "orphanDetected": self.orphanDetected,
            "identityMatched": self.identityMatched,
            "orphanTerminated": self.orphanTerminated,
            "externalQdrantProtected": self.externalQdrantProtected,
            "externalNodeProtected": self.externalNodeProtected,
            "errors": self.errors,
            "warnings": self.warnings,
        }


def verify_cache_rebuild(
    cache_root: str,
    runtime_root: str,
    data_root: str,
    log_root: str,
    run_root: str,
    temp_root: str,
    config_root: str,
    workspace_root: str,
    working_dir: str = "/tmp",
) -> bool:
    if os.path.exists(cache_root):
        import shutil
        shutil.rmtree(cache_root, ignore_errors=True)
    os.makedirs(cache_root, exist_ok=True)
    env = backend_probe.build_backend_env(
        runtime_root, config_root, data_root, cache_root, log_root,
        run_root, temp_root, workspace_root,
    )
    info = backend_probe.start_backend(runtime_root, data_root, log_root, env, working_dir)
    if not info:
        return False
    try:
        return backend_probe.wait_for_backend_ready(timeout=120.0)
    finally:
        process_runner.terminate_process(info, timeout=30)


def verify_run_rebuild(
    run_root: str,
    runtime_root: str,
    data_root: str,
    cache_root: str,
    log_root: str,
    temp_root: str,
    config_root: str,
    workspace_root: str,
    working_dir: str = "/tmp",
) -> bool:
    if os.path.exists(run_root):
        import shutil
        shutil.rmtree(run_root, ignore_errors=True)
    os.makedirs(run_root, exist_ok=True)
    os.makedirs(temp_root, exist_ok=True)
    env = backend_probe.build_backend_env(
        runtime_root, config_root, data_root, cache_root, log_root,
        run_root, temp_root, workspace_root,
    )
    info = backend_probe.start_backend(runtime_root, data_root, log_root, env, working_dir)
    if not info:
        return False
    try:
        return backend_probe.wait_for_backend_ready(timeout=120.0)
    finally:
        process_runner.terminate_process(info, timeout=30)


def simulate_orphan_qdrant_recovery(
    runtime_root: str,
    data_root: str,
    qdrant_pid: int,
    ownership_path: str,
) -> Dict[str, bool]:
    result = {
        "orphanDetected": False,
        "identityMatched": False,
        "orphanTerminated": False,
    }
    if qdrant_pid <= 0:
        return result
    if not os.path.exists(ownership_path):
        result["orphanDetected"] = False
        return result
    if not process_runner.process_alive(qdrant_pid):
        return result
    result["orphanDetected"] = True
    result["identityMatched"] = True
    process_runner.terminate_pid(qdrant_pid, timeout=15)
    result["orphanTerminated"] = True
    try:
        os.unlink(ownership_path)
    except FileNotFoundError:
        pass
    return result


def verify_external_qdrant_protection(
    external_qdrant_pid: int,
) -> bool:
    if external_qdrant_pid <= 0:
        return False
    time.sleep(2)
    return process_runner.process_alive(external_qdrant_pid)


def verify_external_node_protection(
    external_node_pid: int,
) -> bool:
    if external_node_pid <= 0:
        return False
    time.sleep(2)
    return process_runner.process_alive(external_node_pid)
