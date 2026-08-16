import os
import tarfile
import zipfile
from typing import Optional

try:
    from .errors import ValidationError
except ImportError:
    from errors import ValidationError


def is_path_safe(member: str, base: str) -> bool:
    target = os.path.realpath(os.path.join(base, member))
    base_real = os.path.realpath(base)
    return target.startswith(base_real + os.sep) or target == base_real


def is_tarinfo_safe(member: tarfile.TarInfo, base: str) -> bool:
    target = os.path.realpath(os.path.join(base, member.name))
    base_real = os.path.realpath(base)
    if not (target.startswith(base_real + os.sep) or target == base_real):
        return False
    if member.issym() or member.islnk():
        link_target = os.path.realpath(os.path.join(base, os.path.dirname(member.name), member.linkname))
        if not (link_target.startswith(base_real + os.sep) or link_target == base_real):
            return False
    return True


def safe_extract_zip(archive_path: str, dest: str) -> None:
    os.makedirs(dest, exist_ok=True)
    with zipfile.ZipFile(archive_path, "r") as zf:
        for member in zf.namelist():
            if not is_path_safe(member, dest):
                raise ValidationError(f"Path traversal detected in zip: {member}")
        zf.extractall(dest)


def safe_extract_tar(archive_path: str, dest: str, mode: str = "r:*") -> None:
    os.makedirs(dest, exist_ok=True)
    with tarfile.open(archive_path, mode) as tf:
        for member in tf.getmembers():
            if not is_tarinfo_safe(member, dest):
                raise ValidationError(f"Path traversal detected in tar: {member.name}")
        tf.extractall(dest)


def safe_extract(archive_path: str, dest: str) -> None:
    if archive_path.endswith(".zip"):
        safe_extract_zip(archive_path, dest)
    elif archive_path.endswith((".tar.gz", ".tgz")):
        safe_extract_tar(archive_path, dest, "r:gz")
    elif archive_path.endswith(".tar.xz"):
        safe_extract_tar(archive_path, dest, "r:xz")
    elif archive_path.endswith(".tar"):
        safe_extract_tar(archive_path, dest, "r:")
    else:
        raise ValidationError(f"Unsupported archive format: {archive_path}")
