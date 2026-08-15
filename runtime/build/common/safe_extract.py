import os
import tarfile
import zipfile
from dataclasses import dataclass
from typing import List


@dataclass
class SafeExtractResult:
    success: bool = False
    errors: List[str] = None

    def __post_init__(self):
        if self.errors is None:
            self.errors = []


def _is_path_safe(member: str, base: str) -> bool:
    target = os.path.realpath(os.path.join(base, member))
    base_real = os.path.realpath(base)
    return target.startswith(base_real + os.sep) or target == base_real


def safe_extract_archive(archive_path: str, dest: str) -> SafeExtractResult:
    result = SafeExtractResult()
    os.makedirs(dest, exist_ok=True)

    if archive_path.endswith(".zip"):
        try:
            with zipfile.ZipFile(archive_path, "r") as zf:
                for member in zf.namelist():
                    if not _is_path_safe(member, dest):
                        result.errors.append(f"Path traversal detected: {member}")
                        return result
                zf.extractall(dest)
        except Exception as e:
            result.errors.append(f"ZIP extraction failed: {e}")
            return result
    elif archive_path.endswith((".tar.gz", ".tgz")):
        try:
            with tarfile.open(archive_path, "r:gz") as tf:
                for member in tf.getmembers():
                    if not _is_path_safe(member.name, dest):
                        result.errors.append(f"Path traversal detected: {member.name}")
                        return result
                tf.extractall(dest)
        except Exception as e:
            result.errors.append(f"tar.gz extraction failed: {e}")
            return result
    elif archive_path.endswith(".tar"):
        try:
            with tarfile.open(archive_path, "r:") as tf:
                for member in tf.getmembers():
                    if not _is_path_safe(member.name, dest):
                        result.errors.append(f"Path traversal detected: {member.name}")
                        return result
                tf.extractall(dest)
        except Exception as e:
            result.errors.append(f"tar extraction failed: {e}")
            return result
    else:
        result.errors.append(f"Unsupported archive format: {archive_path}")
        return result

    result.success = True
    return result
