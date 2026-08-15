import os
import stat
import tarfile
import zipfile
from dataclasses import dataclass
from pathlib import Path
from typing import List, Set


@dataclass
class SafeExtractResult:
    success: bool = False
    errors: List[str] = None

    def __post_init__(self):
        if self.errors is None:
            self.errors = []


def _normalize_safe_path(member: str) -> str:
    path = member.replace("\\", "/")
    path = os.path.normpath(path)
    if path.startswith("/"):
        path = path[1:]
    parts = path.split("/")
    safe_parts = []
    for p in parts:
        if p == "..":
            return ""
        if p and p != ".":
            safe_parts.append(p)
    return "/".join(safe_parts)


def _is_safe_in_dest(normalized: str, dest_real: str) -> bool:
    if not normalized:
        return False
    full_path = os.path.join(dest_real, normalized)
    full_real = os.path.realpath(full_path)
    if not (full_real.startswith(dest_real + os.sep) or full_real == dest_real):
        return False
    return True


def safe_extract(archive_path: str, dest: str) -> None:
    result = safe_extract_archive(archive_path, dest)
    if not result.success:
        raise OSError(f"Safe extraction failed: {'; '.join(result.errors)}")


def safe_extract_archive(archive_path: str, dest: str) -> SafeExtractResult:
    result = SafeExtractResult()
    os.makedirs(dest, exist_ok=True)
    dest_real = os.path.realpath(dest)
    seen_paths: Set[str] = set()

    ext = "".join(Path(archive_path).suffixes).lower()
    if not ext:
        ext = Path(archive_path).suffix.lower()

    if archive_path.endswith(".zip"):
        try:
            _extract_zip(archive_path, dest, dest_real, seen_paths, result)
        except Exception as e:
            result.errors.append(f"ZIP extraction failed: {e}")
        return result
    elif archive_path.endswith((".tar.gz", ".tgz")) or ext in (".tar.gz", ".tgz"):
        try:
            _extract_tar(archive_path, "r:gz", dest, dest_real, seen_paths, result)
        except Exception as e:
            result.errors.append(f"tar.gz extraction failed: {e}")
        return result
    elif archive_path.endswith(".tar.xz") or ext in (".tar.xz",):
        try:
            _extract_tar(archive_path, "r:xz", dest, dest_real, seen_paths, result)
        except Exception as e:
            result.errors.append(f"tar.xz extraction failed: {e}")
        return result
    elif archive_path.endswith(".tar"):
        try:
            _extract_tar(archive_path, "r:", dest, dest_real, seen_paths, result)
        except Exception as e:
            result.errors.append(f"tar extraction failed: {e}")
        return result
    else:
        result.errors.append(f"Unsupported archive format: {archive_path}")
        return result


def _check_tar_member(member, dest_real: str, seen_paths: Set[str]) -> List[str]:
    errors = []

    if member.name.startswith("/") or member.name.startswith("\\"):
        errors.append(f"Absolute path rejected: {member.name}")
        return errors

    norm = _normalize_safe_path(member.name)
    if not norm:
        errors.append(f"Path traversal or empty path rejected: {member.name}")
        return errors

    if norm in seen_paths:
        errors.append(f"Duplicate normalized path rejected: {norm}")
    seen_paths.add(norm)

    if not _is_safe_in_dest(norm, dest_real):
        errors.append(f"Path escapes destination: {member.name}")
        return errors

    if member.issym():
        link_target = member.linkname
        if link_target.startswith("/"):
            errors.append(f"Symlink target is absolute: {member.name} -> {link_target}")
            return errors
        target_norm = _normalize_safe_path(os.path.join(os.path.dirname(norm), link_target))
        if not target_norm or not _is_safe_in_dest(target_norm, dest_real):
            errors.append(f"Symlink target escapes destination: {member.name} -> {link_target}")
            return errors

    if member.islnk():
        link_target = member.linkname
        if link_target.startswith("/"):
            errors.append(f"Hardlink target is absolute: {member.name} -> {link_target}")
            return errors
        target_norm = _normalize_safe_path(os.path.join(os.path.dirname(norm), link_target))
        if not target_norm or not _is_safe_in_dest(target_norm, dest_real):
            errors.append(f"Hardlink target escapes destination: {member.name} -> {link_target}")
            return errors

    if member.isdev():
        errors.append(f"Device node rejected: {member.name}")
        return errors

    mode = member.mode or 0
    if mode & stat.S_ISUID:
        errors.append(f"Setuid bit rejected: {member.name}")
    if mode & stat.S_ISGID:
        errors.append(f"Setgid bit rejected: {member.name}")

    return errors


def _extract_tar(archive_path: str, mode: str, dest: str, dest_real: str,
                 seen_paths: Set[str], result: SafeExtractResult) -> None:
    with tarfile.open(archive_path, mode) as tf:
        for member in tf.getmembers():
            member_errors = _check_tar_member(member, dest_real, seen_paths)
            if member_errors:
                result.errors.extend(member_errors)
                return

        def filtered_extract():
            for member in tf.getmembers():
                norm = _normalize_safe_path(member.name)
                member.name = norm
                yield member

        tf.extractall(dest, members=filtered_extract())
    result.success = True


def _check_zip_member(info: zipfile.ZipInfo, dest_real: str, seen_paths: Set[str]) -> List[str]:
    errors = []

    if info.filename.startswith("/") or info.filename.startswith("\\"):
        errors.append(f"Absolute path rejected: {info.filename}")
        return errors

    norm = _normalize_safe_path(info.filename)
    if not norm:
        errors.append(f"Path traversal or empty path rejected: {info.filename}")
        return errors

    if norm in seen_paths:
        errors.append(f"Duplicate normalized path rejected: {norm}")
    seen_paths.add(norm)

    if not _is_safe_in_dest(norm, dest_real):
        errors.append(f"Path escapes destination: {info.filename}")
        return errors

    return errors


def _extract_zip(archive_path: str, dest: str, dest_real: str,
                 seen_paths: Set[str], result: SafeExtractResult) -> None:
    with zipfile.ZipFile(archive_path, "r") as zf:
        for info in zf.infolist():
            member_errors = _check_zip_member(info, dest_real, seen_paths)
            if member_errors:
                result.errors.extend(member_errors)
                return

        for info in zf.infolist():
            norm = _normalize_safe_path(info.filename)
            info.filename = norm
            if info.external_attr & 0xFFFF:
                unix_attr = info.external_attr >> 16
                if unix_attr & stat.S_ISUID or unix_attr & stat.S_ISGID:
                    result.errors.append(f"Setuid/setgid bit rejected: {info.filename}")
                    return
            zf.extract(info, dest)

    result.success = True
