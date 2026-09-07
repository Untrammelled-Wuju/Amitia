import os
import platform
import shutil
import socket
import stat
import sys
import tempfile
from dataclasses import dataclass, field
from pathlib import Path
from typing import List, Optional

try:
    from .errors import ValidationErrorCode, ValidationException
except ImportError:
    from errors import ValidationErrorCode, ValidationException


KERNEL_REQUIRED = "Linux"
ARCH_REQUIRED = "aarch64"
MIN_DISK_BYTES = 512 * 1024 * 1024
TARGET_DISTRO = "ubuntu"
TARGET_DISTRO_VERSION = "24.04.4"
SENSITIVE_ENV_KEYS = {
    "AMITIA_LOCAL_TOKEN",
    "AMITIA_TOKEN",
    "AMITIA_SECRET",
    "AMITIA_API_KEY",
    "AMITIA_PASSWORD",
    "AWS_SECRET_ACCESS_KEY",
    "AWS_ACCESS_KEY_ID",
    "GITHUB_TOKEN",
    "DOCKER_AUTH",
    "SSH_PRIVATE_KEY",
    "PRIVATE_KEY",
    "SECRET",
    "PASSWORD",
    "CREDENTIALS",
}


@dataclass
class EnvironmentReport:
    kernel: str = ""
    architecture: str = ""
    distribution: str = ""
    distributionVersion: str = ""
    isLinux: bool = False
    isAarch64: bool = False
    pythonVersion: str = ""
    diskFreeBytes: int = 0
    diskSufficient: bool = False
    symlinkSupported: bool = False
    executePermissionSupported: bool = False
    unixSocketSupported: bool = False
    loopbackSupported: bool = False
    systemNodePresent: bool = False
    systemNodeVersion: Optional[str] = None
    systemQdrantPresent: bool = False
    systemQdrantVersion: Optional[str] = None
    systemGoPresent: bool = False
    systemGoVersion: Optional[str] = None
    environmentType: str = "emulated-arm64"
    errors: List[dict] = field(default_factory=list)
    warnings: List[str] = field(default_factory=list)

    def to_safe_dict(self) -> dict:
        data = {
            "kernel": self.kernel,
            "architecture": self.architecture,
            "distribution": self.distribution,
            "distributionVersion": self.distributionVersion,
            "isLinux": self.isLinux,
            "isAarch64": self.isAarch64,
            "pythonVersion": self.pythonVersion,
            "diskFreeBytes": self.diskFreeBytes,
            "diskSufficient": self.diskSufficient,
            "symlinkSupported": self.symlinkSupported,
            "executePermissionSupported": self.executePermissionSupported,
            "unixSocketSupported": self.unixSocketSupported,
            "loopbackSupported": self.loopbackSupported,
            "systemNodePresent": self.systemNodePresent,
            "systemNodeVersion": self.systemNodeVersion,
            "systemQdrantPresent": self.systemQdrantPresent,
            "systemQdrantVersion": self.systemQdrantVersion,
            "systemGoPresent": self.systemGoPresent,
            "systemGoVersion": self.systemGoVersion,
            "environmentType": self.environmentType,
            "errors": self.errors,
            "warnings": self.warnings,
        }
        return data


def _resolve_optional_binary(name: str) -> Optional[str]:
    return shutil.which(name)


def _detect_distribution() -> tuple:
    try:
        with open("/etc/os-release", "r", encoding="utf-8") as f:
            content = f.read()
    except (FileNotFoundError, PermissionError):
        return "", ""
    name = ""
    version = ""
    for line in content.splitlines():
        stripped = line.strip()
        if stripped.startswith("ID="):
            name = stripped.split("=", 1)[1].strip().strip('"').lower()
        elif stripped.startswith("VERSION_ID="):
            version = stripped.split("=", 1)[1].strip().strip('"')
    return name, version


def _get_disk_free(path: str) -> int:
    usage = shutil.disk_usage(path)
    return usage.free


def _test_symlink_support(base: str) -> bool:
    source = os.path.join(base, "symlink_source_file")
    link = os.path.join(base, "symlink_link_file")
    try:
        with open(source, "w", encoding="utf-8") as f:
            f.write("symlink-test")
        os.symlink(source, link)
        return os.path.islink(link)
    except ( OSError, AttributeError, NotImplementedError ):
        return False
    finally:
        for target in (link, source):
            try:
                os.unlink(target)
            except (FileNotFoundError, OSError):
                pass


def _test_execute_permission(base: str) -> bool:
    target = os.path.join(base, "exec_permission_test")
    try:
        with open(target, "w", encoding="utf-8") as f:
            f.write("#!/bin/sh\nexit 0\n")
        os.chmod(target, 0o755)
        mode = os.stat(target).st_mode
        return bool(mode & stat.S_IXUSR)
    except OSError:
        return False
    finally:
        try:
            os.unlink(target)
        except FileNotFoundError:
            pass


def _test_unix_socket(base: str) -> bool:
    if sys.platform == "win32":
        return False
    path = os.path.join(base, "unix_socket_test.sock")
    try:
        server = socket.socket(socket.AF_UNIX, socket.SOCK_STREAM)
        try:
            server.bind(path)
            return os.path.exists(path)
        except OSError:
            return False
        finally:
            server.close()
            try:
                os.unlink(path)
            except FileNotFoundError:
                pass
    except AttributeError:
        return False


def _test_loopback() -> bool:
    server = socket.socket(socket.AF_INET, socket.SOCK_STREAM)
    try:
        server.bind(("127.0.0.1", 0))
        server.listen(1)
        port = server.getsockname()[1]
        client = socket.create_connection(("127.0.0.1", port), timeout=2.0)
        client.close()
        return True
    except OSError:
        return False
    finally:
        server.close()


def _probe_binary_version(path: str, version_args=None) -> Optional[str]:
    if not path:
        return None
    import subprocess
    args = [path] + (version_args or ["--version"])
    try:
        result = subprocess.run(
            args,
            capture_output=True,
            text=True,
            timeout=30,
            check=False,
        )
        output = (result.stdout or "") + (result.stderr or "")
        return output.strip().splitlines()[0] if output.strip() else None
    except (OSError, subprocess.TimeoutExpired):
        return None


def _collect_sanitized_env() -> dict:
    result = {}
    for key, value in sorted(os.environ.items()):
        upper = key.upper()
        if any(sensitive in upper for sensitive in SENSITIVE_ENV_KEYS):
            result[key] = "***REDACTED***"
        else:
            result[key] = value
    return result


def validate_environment(work_dir: str, expected_env_type: str = "vm-arm64") -> EnvironmentReport:
    report = EnvironmentReport()
    report.kernel = platform.system()
    report.architecture = platform.machine()
    report.pythonVersion = sys.version.split()[0]
    distro, version = _detect_distribution()
    report.distribution = distro
    report.distributionVersion = version
    report.isLinux = report.kernel == KERNEL_REQUIRED
    report.isAarch64 = report.architecture == ARCH_REQUIRED
    report.environmentType = expected_env_type

    root_path = work_dir if os.path.isabs(work_dir) else os.path.abspath(work_dir)
    os.makedirs(root_path, exist_ok=True)
    report.diskFreeBytes = _get_disk_free(root_path)
    report.diskSufficient = report.diskFreeBytes >= MIN_DISK_BYTES
    report.symlinkSupported = _test_symlink_support(root_path)
    report.executePermissionSupported = _test_execute_permission(root_path)
    report.unixSocketSupported = _test_unix_socket(root_path)
    report.loopbackSupported = _test_loopback()

    node_path = _resolve_optional_binary("node")
    report.systemNodePresent = node_path is not None
    report.systemNodeVersion = _probe_binary_version(node_path) if node_path else None

    qdrant_path = _resolve_optional_binary("qdrant")
    report.systemQdrantPresent = qdrant_path is not None
    report.systemQdrantVersion = _probe_binary_version(qdrant_path) if qdrant_path else None

    go_path = _resolve_optional_binary("go")
    report.systemGoPresent = go_path is not None
    report.systemGoVersion = _probe_binary_version(go_path, ["version"]) if go_path else None

    if not report.isLinux:
        report.errors.append(
            {
                "code": ValidationErrorCode.ENV_NOT_LINUX,
                "message": f"Kernel is {report.kernel}, expected {KERNEL_REQUIRED}",
            }
        )
    if not report.isAarch64:
        report.errors.append(
            {
                "code": ValidationErrorCode.ENV_NOT_AARCH64,
                "message": f"Architecture is {report.architecture}, expected {ARCH_REQUIRED}",
            }
        )
    if report.distribution and report.distribution != TARGET_DISTRO:
        report.errors.append(
            {
                "code": ValidationErrorCode.ENV_UNSUPPORTED_DISTRO,
                "message": f"Distribution is {report.distribution}, expected {TARGET_DISTRO}",
            }
        )
    if not report.diskSufficient:
        report.errors.append(
            {
                "code": ValidationErrorCode.ENV_DISK_SPACE,
                "message": f"Insufficient disk space: {report.diskFreeBytes} bytes",
            }
        )
    if not report.executePermissionSupported:
        report.errors.append(
            {
                "code": ValidationErrorCode.ENV_NO_EXECUTE_PERMISSION,
                "message": "Execute permission not supported",
            }
        )
    if not report.symlinkSupported:
        report.errors.append(
            {
                "code": ValidationErrorCode.ENV_NO_SYMLINK,
                "message": "Symlink not supported",
            }
        )
    if not report.unixSocketSupported:
        report.errors.append(
            {
                "code": ValidationErrorCode.ENV_NO_UNIX_SOCKET,
                "message": "Unix socket not supported",
            }
        )
    if not report.loopbackSupported:
        report.errors.append(
            {
                "code": ValidationErrorCode.ENV_NO_LOOPBACK,
                "message": "Loopback network not functional",
            }
        )
    return report
