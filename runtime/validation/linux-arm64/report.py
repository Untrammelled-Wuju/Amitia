import json
import os
import uuid
from dataclasses import dataclass, field
from datetime import datetime, timezone
from enum import Enum
from typing import Any, Dict, List, Optional


class CheckStatus(str, Enum):
    PASSED = "passed"
    FAILED = "failed"
    SKIPPED = "skipped"
    NOT_APPLICABLE = "not_applicable"


SENSITIVE_PATTERNS = (
    "token",
    "secret",
    "password",
    "api_key",
    "apikey",
    "credential",
    "private",
    "auth",
    "key",
    "bearer",
)


@dataclass
class Check:
    id: str
    status: str
    durationMs: int = 0
    details: Optional[dict] = None
    errorMessage: Optional[str] = None

    def to_safe_dict(self) -> dict:
        safe_details = self._sanitize_dict(self.details) if self.details else None
        return {
            "id": self.id,
            "status": self.status,
            "durationMs": self.durationMs,
            "details": safe_details,
            "errorMessage": self.errorMessage,
        }

    def _sanitize_dict(self, data: Any) -> Any:
        if isinstance(data, dict):
            sanitized = {}
            for key, value in data.items():
                if self._is_sensitive_key(key):
                    sanitized[key] = "[REDACTED]"
                else:
                    sanitized[key] = self._sanitize_dict(value)
            return sanitized
        elif isinstance(data, list):
            return [self._sanitize_dict(item) for item in data]
        elif isinstance(data, str) and len(data) > 512:
            return data[:512] + "...[TRUNCATED]"
        return data

    def _is_sensitive_key(self, key: str) -> bool:
        low = key.lower()
        return any(pattern in low for pattern in SENSITIVE_PATTERNS)


@dataclass
class ValidationReport:
    schemaVersion: int = 1
    validationId: str = "runtime.linux-arm64"
    validationUuid: str = ""
    runtimeVersion: str = ""
    sourceCommit: str = ""
    result: str = "passed"
    startTimestamp: str = ""
    endTimestamp: str = ""
    environment: Dict[str, Any] = field(default_factory=dict)
    package: Dict[str, Any] = field(default_factory=dict)
    components: Dict[str, Any] = field(default_factory=dict)
    profiles: Dict[str, Any] = field(default_factory=dict)
    startup: Dict[str, Any] = field(default_factory=dict)
    business: Dict[str, Any] = field(default_factory=dict)
    restart: Dict[str, Any] = field(default_factory=dict)
    dataIsolation: Dict[str, Any] = field(default_factory=dict)
    cleanup: Dict[str, Any] = field(default_factory=dict)
    checks: List[Check] = field(default_factory=list)

    def __post_init__(self):
        if not self.validationUuid:
            self.validationUuid = str(uuid.uuid4())
        if not self.startTimestamp:
            self.startTimestamp = datetime.now(timezone.utc).isoformat()

    def add_check(self, check: Check) -> None:
        self.checks.append(check)
        if check.status == CheckStatus.FAILED.value:
            self.result = "failed"

    def finalize(self) -> None:
        self.endTimestamp = datetime.now(timezone.utc).isoformat()
        if any(check.status == CheckStatus.FAILED.value for check in self.checks):
            self.result = "failed"
        else:
            self.result = "passed"

    def to_safe_dict(self) -> dict:
        return {
            "schemaVersion": self.schemaVersion,
            "validationId": self.validationId,
            "validationUuid": self.validationUuid,
            "runtimeVersion": self.runtimeVersion,
            "sourceCommit": "",
            "result": self.result,
            "startTimestamp": self.startTimestamp,
            "endTimestamp": self.endTimestamp,
            "environment": self.environment,
            "package": self.package,
            "components": self.components,
            "profiles": self.profiles,
            "startup": self.startup,
            "business": self.business,
            "restart": self.restart,
            "dataIsolation": self.dataIsolation,
            "cleanup": self.cleanup,
            "checks": [check.to_safe_dict() for check in self.checks],
        }

    def write_json(self, path: str) -> None:
        os.makedirs(os.path.dirname(path) or ".", exist_ok=True)
        with open(path, "w", encoding="utf-8") as f:
            json.dump(self.to_safe_dict(), f, indent=2, sort_keys=False, ensure_ascii=False)

    def write_summary(self, path: str) -> None:
        os.makedirs(os.path.dirname(path) or ".", exist_ok=True)
        total = len(self.checks)
        passed = sum(1 for c in self.checks if c.status == CheckStatus.PASSED.value)
        failed = sum(1 for c in self.checks if c.status == CheckStatus.FAILED.value)
        skipped = sum(1 for c in self.checks if c.status == CheckStatus.SKIPPED.value)
        not_applicable = sum(1 for c in self.checks if c.status == CheckStatus.NOT_APPLICABLE.value)
        lines = [
            "============================================================",
            "Amitia Runtime Linux ARM64 Validation Summary",
            "============================================================",
            f"Validation ID: {self.validationId}",
            f"Runtime Version: {self.runtimeVersion}",
            f"Result: {self.result.upper()}",
            f"Started: {self.startTimestamp}",
            f"Finished: {self.endTimestamp}",
            "------------------------------------------------------------",
            f"Total Checks: {total}",
            f"  Passed: {passed}",
            f"  Failed: {failed}",
            f"  Skipped: {skipped}",
            f"  Not Applicable: {not_applicable}",
            "------------------------------------------------------------",
        ]
        if failed > 0:
            lines.append("Failed Checks:")
            for check in self.checks:
                if check.status == CheckStatus.FAILED.value:
                    lines.append(f"  - {check.id}")
                    if check.errorMessage:
                        lines.append(f"    Error: {check.errorMessage}")
        lines.append("============================================================")
        with open(path, "w", encoding="utf-8") as f:
            f.write("\n".join(lines))
