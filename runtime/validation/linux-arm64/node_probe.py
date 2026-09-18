import json
import os
import re
from dataclasses import dataclass, field
from pathlib import Path
from typing import Dict, List, Optional

try:
    from . import process_runner
    from .errors import ValidationErrorCode
except ImportError:
    import process_runner
    from errors import ValidationErrorCode


EXPECTED_NODE_VERSION = "24.19.0"
EXPECTED_NPM_VERSION = "11.17.0"
EXPECTED_NODE_SCRIPT_VERSION = "1"

NODE_PREPARE_OUTPUT_PATHS = [
    "node/home",
    "node/prefix",
    "npm",
]

NODE_FORBIDDEN_PATHS = ["/root/.npm", "/opt/amitia/node/.npm"]


@dataclass
class NodeProbeResult:
    version: str = ""
    npmVersion: str = ""
    npxVersion: str = ""
    prepareOk: bool = False
    probeOk: bool = False
    noPathPollution: bool = False
    systemNodeNotUsed: bool = False
    pluginHostOk: bool = False
    taskHostOk: bool = False
    variableDirectoriesOk: bool = False
    executable: str = ""
    errors: List[dict] = field(default_factory=list)
    warnings: List[str] = field(default_factory=list)

    def to_safe_dict(self) -> dict:
        return {
            "version": self.version,
            "npmVersion": self.npmVersion,
            "npxVersion": self.npxVersion,
            "prepareOk": self.prepareOk,
            "probeOk": self.probeOk,
            "noPathPollution": self.noPathPollution,
            "systemNodeNotUsed": self.systemNodeNotUsed,
            "pluginHostOk": self.pluginHostOk,
            "taskHostOk": self.taskHostOk,
            "variableDirectoriesOk": self.variableDirectoriesOk,
            "executable": self.executable,
            "errors": self.errors,
            "warnings": self.warnings,
        }


def load_node_lock(lock_path: str) -> dict:
    if not os.path.exists(lock_path):
        return {}
    with open(lock_path, "r", encoding="utf-8") as f:
        data = json.load(f)
    return data if isinstance(data, dict) else {}


def validate_node_elf(runtime_root: str) -> List[str]:
    errors = []
    node_path = os.path.join(runtime_root, "node/bin/node")
    if not os.path.exists(node_path):
        errors.append(f"Node executable not found: {node_path}")
        return errors
    with open(node_path, "rb") as f:
        header = f.read(64)
    if len(header) < 24:
        errors.append("Node ELF header too short")
        return errors
    magic = header[1:4]
    if magic != b"ELF":
        errors.append("Node is not an ELF executable")
        return errors
    ei_class = header[4]
    if ei_class != 2:
        errors.append(f"Node is not 64-bit (ei_class={ei_class})")
    ei_data = header[5]
    if ei_data != 1:
        errors.append("Node is not little-endian")
    e_machine = int.from_bytes(header[18:20], "little")
    if e_machine != 183:
        errors.append(f"Node is not AArch64 (e_machine={e_machine})")
    return errors


def get_node_version(runtime_root: str) -> Optional[str]:
    node_path = os.path.join(runtime_root, "node/bin/node")
    if not os.path.exists(node_path):
        return None
    result = process_runner.run_process([node_path, "--version"], timeout=10)
    if result.returncode != 0:
        return None
    output = (result.stdout or result.stderr or "").strip()
    return output.replace("v", "") if output else None


def get_npm_version(runtime_root: str) -> Optional[str]:
    node_path = os.path.join(runtime_root, "node/bin/node")
    npm_cli = os.path.join(runtime_root, "node/lib/node_modules/npm/bin/npm-cli.js")
    if not os.path.exists(node_path) or not os.path.exists(npm_cli):
        return None
    result = process_runner.run_process([node_path, npm_cli, "--version"], timeout=10)
    if result.returncode != 0:
        return None
    return (result.stdout or result.stderr or "").strip() or None


def get_npx_version(runtime_root: str) -> Optional[str]:
    node_path = os.path.join(runtime_root, "node/bin/node")
    npx_cli = os.path.join(runtime_root, "node/lib/node_modules/npm/bin/npx-cli.js")
    if not os.path.exists(node_path) or not os.path.exists(npx_cli):
        return None
    result = process_runner.run_process([node_path, npx_cli, "--version"], timeout=10)
    if result.returncode != 0:
        return None
    return (result.stdout or result.stderr or "").strip() or None


def check_node_ldd(runtime_root: str) -> List[str]:
    errors = []
    node_path = os.path.join(runtime_root, "node/bin/node")
    if not os.path.exists(node_path):
        return errors
    ldd_path = "/usr/bin/ldd"
    if not os.path.exists(ldd_path):
        ldd_path = "/bin/ldd"
    if not os.path.exists(ldd_path):
        return errors
    result = process_runner.run_process([ldd_path, node_path], timeout=15)
    combined = (result.stdout or "") + (result.stderr or "")
    if "not a dynamic executable" in combined.lower():
        return errors
    for line in combined.splitlines():
        if "not found" in line.lower():
            errors.append(f"Node missing dynamic dependency: {line.strip()}")
    return errors


def run_node_prepare(runtime_root: str, data_root: str, cache_root: str, run_root: str) -> bool:
    prepare_script = os.path.join(runtime_root, "scripts/node/amitia-node-prepare.sh")
    if not os.path.exists(prepare_script):
        return False
    node_path = os.path.join(runtime_root, "node/bin/node")
    env = {
        "AMITIA_RUNTIME_ROOT": runtime_root,
        "AMITIA_DATA_ROOT": data_root,
        "AMITIA_CACHE_ROOT": cache_root,
        "AMITIA_RUN_ROOT": run_root,
        "AMITIA_NODE_PATH": node_path,
        "PATH": "/usr/bin:/bin",
    }
    result = process_runner.run_process(["/bin/sh", prepare_script], timeout=60, env=env)
    return result.returncode == 0


def run_node_probe(runtime_root: str, data_root: str) -> bool:
    probe_script = os.path.join(runtime_root, "scripts/node/amitia-node-probe.sh")
    if not os.path.exists(probe_script):
        return False
    env = {
        "AMITIA_RUNTIME_ROOT": runtime_root,
        "AMITIA_DATA_ROOT": data_root,
        "PATH": "/usr/bin:/bin",
    }
    result = process_runner.run_process(["/bin/sh", probe_script], timeout=60, env=env)
    return result.returncode == 0


def verify_variable_directories(data_root: str, cache_root: str, run_root: str, runtime_root: str) -> bool:
    required_paths = [
        os.path.join(data_root, "node/home"),
        os.path.join(data_root, "node/prefix"),
        os.path.join(cache_root, "node/npm"),
        os.path.join(run_root, "tmp/node"),
    ]
    for path in required_paths:
        if not os.path.exists(path):
            return False
    forbidden_paths = [
        os.path.join("/root", ".npm"),
        os.path.join(runtime_root, "node", ".npm"),
    ]
    for path in forbidden_paths:
        if os.path.exists(path):
            return False
    return True


def probe_plugin_host(runtime_root: str) -> bool:
    plugin_host_script = os.path.join(runtime_root, "scripts/node/amitia-plugin-host.sh")
    if not os.path.exists(plugin_host_script):
        return False
    node_path = os.path.join(runtime_root, "node/bin/node")
    plugin_entry = os.path.join(runtime_root, "plugin-host/dist/index.js")
    if not os.path.exists(plugin_entry):
        return False
    env = {
        "AMITIA_RUNTIME_ROOT": runtime_root,
        "AMITIA_NODE_PATH": node_path,
        "PATH": "/usr/bin:/bin",
        "AMITIA_PLUGIN_HOST_MODE": "probe",
    }
    result = process_runner.run_process(["/bin/sh", plugin_host_script], timeout=30, env=env)
    return result.returncode == 0


def probe_task_host(runtime_root: str) -> bool:
    task_host_script = os.path.join(runtime_root, "scripts/node/amitia-task-host.sh")
    if not os.path.exists(task_host_script):
        return False
    node_path = os.path.join(runtime_root, "node/bin/node")
    task_entry = os.path.join(runtime_root, "task-host/dist/index.js")
    if not os.path.exists(task_entry):
        return False
    env = {
        "AMITIA_RUNTIME_ROOT": runtime_root,
        "AMITIA_NODE_PATH": node_path,
        "PATH": "/usr/bin:/bin",
        "AMITIA_TASK_HOST_MODE": "probe",
    }
    result = process_runner.run_process(["/bin/sh", task_host_script], timeout=30, env=env)
    return result.returncode == 0


def run_node_script_test(script_path: str, env: dict, timeout: float = 30) -> bool:
    if not os.path.exists(script_path):
        return False
    result = process_runner.run_process(["/bin/sh", script_path], timeout=timeout, env=env)
    return result.returncode == 0
