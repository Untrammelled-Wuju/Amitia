import json
from dataclasses import dataclass, field
from typing import Dict, List, Optional

try:
    from . import http_probe
except ImportError:
    import http_probe


READ_ONLY_ENDPOINTS = [
    {"method": "GET", "path": "/api/v1/characters", "description": "读取角色列表"},
    {"method": "GET", "path": "/api/v1/models", "description": "读取模型Provider配置"},
    {"method": "GET", "path": "/api/v1/runtime/info", "description": "读取Runtime信息"},
    {"method": "GET", "path": "/api/v1/health", "description": "读取健康状态"},
]


@dataclass
class BusinessProbeResult:
    success: bool = False
    endpointUsed: str = ""
    method: str = ""
    path: str = ""
    status: int = 0
    responseValid: bool = False
    tokenUsed: bool = False
    errors: List[dict] = field(default_factory=list)
    warnings: List[str] = field(default_factory=list)

    def to_safe_dict(self) -> dict:
        return {
            "success": self.success,
            "endpointUsed": self.endpointUsed,
            "method": self.method,
            "path": self.path,
            "status": self.status,
            "responseValid": self.responseValid,
            "tokenUsed": self.tokenUsed,
            "errors": self.errors,
            "warnings": self.warnings,
        }


def read_local_token(data_root: str) -> Optional[str]:
    try:
        from .backend_probe import find_local_token
    except ImportError:
        from backend_probe import find_local_token
    path = find_local_token(data_root)
    if not path:
        return None
    try:
        with open(path, "r", encoding="utf-8") as f:
            content = f.read().strip()
        return content if content else None
    except OSError:
        return None


def probe_readonly_api(
    host: str = "127.0.0.1",
    port: int = 18899,
    data_root: str = "/var/lib/amitia",
) -> BusinessProbeResult:
    result = BusinessProbeResult()
    token = read_local_token(data_root)
    headers = {"Accept": "application/json"}
    if token:
        headers["X-Amitia-Local-Token"] = token
        result.tokenUsed = True

    for endpoint in READ_ONLY_ENDPOINTS:
        url = f"http://{host}:{port}{endpoint['path']}"
        probe = http_probe.http_get_json(url=url, headers=headers, timeout=10.0)
        if probe.ok and probe.status == 200:
            parsed = None
            try:
                parsed = json.loads(probe.body)
            except json.JSONDecodeError:
                parsed = None
            result.success = True
            result.status = probe.status
            result.endpointUsed = endpoint.get("description", endpoint["path"])
            result.method = endpoint["method"]
            result.path = endpoint["path"]
            result.responseValid = parsed is not None or not probe.body
            return result
        elif probe.ok:
            result.warnings.append(f"Endpoint {endpoint['path']} returned status {probe.status}")
        elif probe.error:
            result.errors.append({"endpoint": endpoint["path"], "error": probe.error})

    if not result.success:
        result.errors.append({"code": "BUSINESS_PROBE_FAILED", "message": "All readonly endpoints failed"})
    return result


def verify_data_persisted(
    data_root: str,
    previous_marker: Optional[str] = None,
) -> bool:
    marker_path = f"{data_root}/.validation-marker"
    if previous_marker:
        if os.path.exists(marker_path):
            try:
                with open(marker_path, "r", encoding="utf-8") as f:
                    content = f.read().strip()
                return content == previous_marker
            except OSError:
                return False
        return False
    else:
        import uuid
        marker = str(uuid.uuid4())
        os.makedirs(data_root, exist_ok=True)
        try:
            with open(marker_path, "w", encoding="utf-8") as f:
                f.write(marker)
            return True
        except OSError:
            return False


import os
