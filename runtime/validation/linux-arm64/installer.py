import os
import shutil
import tarfile
from dataclasses import dataclass, field
from pathlib import Path
from typing import List, Optional


@dataclass
class InstallResult:
    success: bool = False
    runtimeRoot: str = ""
    errors: List[str] = field(default_factory=list)

    def to_safe_dict(self) -> dict:
        return {
            "success": self.success,
            "runtimeRoot": self.runtimeRoot,
            "errors": self.errors,
        }


def _is_safe_path(member: str, base: str) -> bool:
    target = os.path.realpath(os.path.join(base, member))
    base_real = os.path.realpath(base)
    return target.startswith(base_real + os.sep) or target == base_real


def extract_runtime_payload(archive_path: str, dest: str) -> bool:
    os.makedirs(dest, exist_ok=True)
    mode = "r:gz" if archive_path.endswith((".tar.gz", ".tgz")) else "r:"
    try:
        with tarfile.open(archive_path, mode) as tf:
            for member in tf.getmembers():
                if not _is_safe_path(member.name, dest):
                    return False
            tf.extractall(dest)
        return True
    except (tarfile.TarError, OSError):
        return False


def install_package_to_work_dir(
    package_path: str,
    work_dir: str,
) -> InstallResult:
    result = InstallResult()
    if not os.path.exists(package_path):
        result.errors.append(f"Package not found: {package_path}")
        return result

    runtime_root = os.path.join(work_dir, "runtime")
    if os.path.exists(runtime_root):
        shutil.rmtree(runtime_root, ignore_errors=True)
    os.makedirs(runtime_root, exist_ok=True)

    try:
        import zipfile
        with zipfile.ZipFile(package_path, "r") as zf:
            for member in zf.namelist():
                if not _is_safe_path(member, work_dir):
                    result.errors.append(f"Path traversal in package: {member}")
                    return result
            zf.extractall(work_dir)
    except (zipfile.BadZipFile, OSError) as e:
        result.errors.append(f"Extract failed: {str(e)}")
        return result

    runtime_payload = os.path.join(work_dir, "runtime", "runtime-root.tar.gz")
    if not os.path.exists(runtime_payload):
        result.errors.append("runtime-root.tar.gz missing from package")
        return result

    if not extract_runtime_payload(runtime_payload, runtime_root):
        result.errors.append("Failed to extract runtime payload")
        return result

    result.success = True
    result.runtimeRoot = runtime_root
    return result


def install_rootfs_payload(
    package_path: str,
    work_dir: str,
) -> bool:
    rootfs_path = os.path.join(work_dir, "rootfs", "ubuntu-arm64-rootfs.tar.gz")
    if not os.path.exists(rootfs_path):
        return False
    dest = os.path.join(work_dir, "guest")
    os.makedirs(dest, exist_ok=True)
    return extract_runtime_payload(rootfs_path, dest)


def copy_runtime_to_system(work_runtime_root: str, system_runtime_root: str) -> bool:
    if not os.path.exists(work_runtime_root):
        return False
    if os.path.exists(system_runtime_root):
        shutil.rmtree(system_runtime_root, ignore_errors=True)
    shutil.copytree(work_runtime_root, system_runtime_root, symlinks=True)
    return os.path.exists(system_runtime_root)


def remove_runtime_from_system(system_runtime_root: str) -> bool:
    if os.path.exists(system_runtime_root):
        shutil.rmtree(system_runtime_root, ignore_errors=True)
    return not os.path.exists(system_runtime_root)
