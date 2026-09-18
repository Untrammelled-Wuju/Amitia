import hashlib
import json
import os
import re
import stat
import struct
import tarfile
import zipfile
from dataclasses import dataclass, field
from pathlib import Path
from typing import Dict, List, Optional, Tuple

try:
    from .errors import ValidationErrorCode, ValidationException
except ImportError:
    from errors import ValidationErrorCode, ValidationException

PACKAGE_INDEX_PATH = "metadata/package-index.json"
SHA256SUMS_PATH = "metadata/SHA256SUMS"
COMPONENT_LOCK_PATH = "metadata/component-lock.json"
GUEST_LAYOUT_PATH = "metadata/guest-layout.json"
MOUNT_CONTRACT_PATH = "metadata/mount-contract.json"
ROOTFS_PAYLOAD_PATH = "payload/rootfs/rootfs.tar.xz"
RUNTIME_PAYLOAD_PATH = "payload/runtime/runtime.tar.xz"

ALLOWED_TOP_PREFIXES = ("metadata/", "payload/", "licenses/")

FORBIDDEN_PLACEHOLDER = {"placeholder", "todo_sha", "tbd", "placeholder_sha", "tbd_sha"}

VALID_HOST_PLATFORMS = {"android"}
VALID_HOST_ABIS = {"arm64-v8a", "armeabi-v7a", "x86_64"}
VALID_RUNTIME_KINDS = {"embedded-proot"}
VALID_GUEST_PLATFORMS = {"linux"}
VALID_GUEST_ARCHITECTURES = {"arm64"}


@dataclass
class PackageManifest:
    runtimeVersion: str = ""
    packageId: str = ""
    sourceRevision: str = ""
    hostPlatform: str = ""
    hostAbi: str = ""
    runtimeKind: str = ""
    guestPlatform: str = ""
    guestArchitecture: str = ""
    rootfsPayloadPath: str = ""
    rootfsPayloadSha256: str = ""
    rootfsPayloadSize: int = 0
    runtimePayloadPath: str = ""
    runtimePayloadSha256: str = ""
    runtimePayloadSize: int = 0
    guestLayoutPath: str = ""
    guestLayoutSha256: str = ""
    mountContractPath: str = ""
    mountContractSha256: str = ""
    sha256sumsPath: str = ""
    sha256sumsSha256: str = ""


@dataclass
class PackageValidationResult:
    valid: bool = False
    sha256: str = ""
    expectedSha256: str = ""
    manifest: Optional[PackageManifest] = None
    errors: List[dict] = field(default_factory=list)
    warnings: List[str] = field(default_factory=list)

    def to_safe_dict(self) -> dict:
        m = self.manifest
        return {
            "valid": self.valid,
            "sha256": self.sha256,
            "expectedSha256": self.expectedSha256,
            "manifest": {
                "runtimeVersion": m.runtimeVersion if m else "",
                "packageId": m.packageId if m else "",
                "hostPlatform": m.hostPlatform if m else "",
                "hostAbi": m.hostAbi if m else "",
                "runtimeKind": m.runtimeKind if m else "",
                "guestPlatform": m.guestPlatform if m else "",
                "guestArchitecture": m.guestArchitecture if m else "",
                "rootfsPayloadPath": m.rootfsPayloadPath if m else "",
                "runtimePayloadPath": m.runtimePayloadPath if m else "",
                "guestLayoutPath": m.guestLayoutPath if m else "",
                "mountContractPath": m.mountContractPath if m else "",
                "sha256sumsPath": m.sha256sumsPath if m else "",
            } if m else None,
            "errors": self.errors,
            "warnings": self.warnings,
        }


def sha256_file(path: str) -> str:
    digest = hashlib.sha256()
    with open(path, "rb") as f:
        for chunk in iter(lambda: f.read(65536), b""):
            digest.update(chunk)
    return digest.hexdigest()


def sha256_bytes(data: bytes) -> str:
    return hashlib.sha256(data).hexdigest()


def _is_path_safe(member: str, base: str) -> bool:
    target = os.path.realpath(os.path.join(base, member))
    base_real = os.path.realpath(base)
    return target.startswith(base_real + os.sep) or target == base_real


def _has_placeholder(value: str) -> bool:
    if not value:
        return True
    lower = value.lower()
    return any(p in lower for p in FORBIDDEN_PLACEHOLDER)


def extract_package_safely(archive_path: str, dest: str) -> None:
    os.makedirs(dest, exist_ok=True)
    if archive_path.endswith(".zip"):
        with zipfile.ZipFile(archive_path, "r") as zf:
            for member in zf.namelist():
                if not _is_path_safe(member, dest):
                    raise ValidationException(
                        ValidationErrorCode.PACKAGE_PATH_TRAVERSAL,
                        f"Path traversal detected: {member}",
                    )
            zf.extractall(dest)
        return
    if archive_path.endswith((".tar.gz", ".tgz")):
        with tarfile.open(archive_path, "r:gz") as tf:
            for member in tf.getmembers():
                if not _is_path_safe(member.name, dest):
                    raise ValidationException(
                        ValidationErrorCode.PACKAGE_PATH_TRAVERSAL,
                        f"Path traversal detected: {member.name}",
                    )
            tf.extractall(dest)
        return
    if archive_path.endswith(".tar"):
        with tarfile.open(archive_path, "r:") as tf:
            for member in tf.getmembers():
                if not _is_path_safe(member.name, dest):
                    raise ValidationException(
                        ValidationErrorCode.PACKAGE_PATH_TRAVERSAL,
                        f"Path traversal detected: {member.name}",
                    )
            tf.extractall(dest)
        return
    raise ValidationException(
        ValidationErrorCode.PACKAGE_INVALID,
        f"Unsupported archive format: {archive_path}",
    )


def _parse_package_index(index_path: str) -> dict:
    if not os.path.exists(index_path):
        return {}
    with open(index_path, "r", encoding="utf-8") as f:
        data = json.load(f)
    return data if isinstance(data, dict) else {}


def _parse_sha256sums(path: str) -> Dict[str, str]:
    if not os.path.exists(path):
        return {}
    result = {}
    with open(path, "r", encoding="utf-8") as f:
        for line in f:
            line = line.rstrip("\n").rstrip("\r")
            stripped = line.strip()
            if not stripped or stripped.startswith("#"):
                continue
            parts = stripped.split()
            if len(parts) >= 2:
                digest = parts[0].lower()
                filename = parts[-1].lstrip("*")
                result[filename] = digest
    return result


def _parse_component_lock(path: str) -> dict:
    if not os.path.exists(path):
        return {}
    with open(path, "r", encoding="utf-8") as f:
        data = json.load(f)
    return data if isinstance(data, dict) else {}


def _resolve_role_payload(index_data: dict, role: str) -> Optional[dict]:
    payloads = index_data.get("payloads")
    if not isinstance(payloads, list):
        return None
    meta = index_data.get("metadata")
    if not isinstance(meta, list):
        meta = []
    for item in payloads + meta:
        if isinstance(item, dict) and item.get("role") == role:
            return item
    return None


def _validate_package_index(index_data: dict) -> List[str]:
    errors = []
    if not isinstance(index_data, dict):
        return ["package-index.json is not a JSON object"]
    if "schemaVersion" not in index_data:
        errors.append("Missing schemaVersion")

    runtime_version = index_data.get("runtimeVersion")
    if not runtime_version or not isinstance(runtime_version, str):
        errors.append("Missing or invalid runtimeVersion")

    package_id = index_data.get("packageId")
    if not package_id or not isinstance(package_id, str) or len(package_id) == 0:
        errors.append("Missing or invalid packageId")

    package_format_version = index_data.get("packageFormatVersion")
    if not isinstance(package_format_version, int) or package_format_version < 1:
        errors.append("Missing or invalid packageFormatVersion")

    source_revision = index_data.get("sourceRevision")
    if not source_revision or not isinstance(source_revision, str):
        errors.append("Missing or invalid sourceRevision")
    elif not re.match(r'^[a-fA-F0-9]{7,40}$', source_revision):
        errors.append(f"sourceRevision must be 7-40 hex chars: {source_revision}")

    target = index_data.get("target")
    if not isinstance(target, dict):
        errors.append("Missing target object")
    else:
        if target.get("hostPlatform") not in VALID_HOST_PLATFORMS:
            errors.append(f"Invalid hostPlatform: {target.get('hostPlatform')}")
        if target.get("hostAbi") not in VALID_HOST_ABIS:
            errors.append(f"Invalid hostAbi: {target.get('hostAbi')}")
        if target.get("runtimeKind") not in VALID_RUNTIME_KINDS:
            errors.append(f"Invalid runtimeKind: {target.get('runtimeKind')}")
        if target.get("guestPlatform") not in VALID_GUEST_PLATFORMS:
            errors.append(f"Invalid guestPlatform: {target.get('guestPlatform')}")
        if target.get("guestArchitecture") not in VALID_GUEST_ARCHITECTURES:
            errors.append(f"Invalid guestArchitecture: {target.get('guestArchitecture')}")

    payload_roles = ["rootfs", "runtime"]
    for role in payload_roles:
        rp = _resolve_role_payload(index_data, role)
        if rp is None:
            errors.append(f"Missing payload with role={role}")
            continue
        p_path = rp.get("path")
        if not p_path:
            errors.append(f"payload role={role} missing path")
        elif _has_placeholder(str(p_path)):
            errors.append(f"payload role={role} path is placeholder")
        p_sha = rp.get("sha256")
        if not p_sha or not isinstance(p_sha, str) or len(p_sha) != 64:
            errors.append(f"payload role={role} sha256 invalid")
        elif _has_placeholder(p_sha):
            errors.append(f"payload role={role} sha256 is placeholder")
        p_size = rp.get("size")
        if not isinstance(p_size, int) or p_size <= 0:
            errors.append(f"payload role={role} size invalid or zero")

    meta_roles = ["guest-layout", "mount-contract", "sha256sums"]
    for role in meta_roles:
        rp = _resolve_role_payload(index_data, role)
        if rp is None:
            errors.append(f"Missing metadata with role={role}")
            continue
        p_path = rp.get("path")
        if not p_path:
            errors.append(f"metadata role={role} missing path")
        elif _has_placeholder(str(p_path)):
            errors.append(f"metadata role={role} path is placeholder")
        p_sha = rp.get("sha256")
        if not p_sha or not isinstance(p_sha, str) or len(p_sha) != 64:
            errors.append(f"metadata role={role} sha256 invalid")
        elif _has_placeholder(p_sha):
            errors.append(f"metadata role={role} sha256 is placeholder")

    return errors


def _extract_payload_ref(index_data: dict, role: str) -> Optional[dict]:
    payloads = index_data.get("payloads")
    if not isinstance(payloads, list):
        payloads = []
    meta = index_data.get("metadata")
    if not isinstance(meta, list):
        meta = []
    for item in payloads + meta:
        if isinstance(item, dict) and item.get("role") == role:
            return item
    return None


def _within_staging(path: str, staging: str) -> bool:
    abs_path = os.path.realpath(os.path.join(staging, path))
    abs_staging = os.path.realpath(staging)
    return abs_path.startswith(abs_staging + os.sep) or abs_path == abs_staging


def _detect_duplicate_paths(entries: List[str]) -> List[str]:
    seen = set()
    dups = []
    for e in entries:
        if e in seen:
            dups.append(e)
        seen.add(e)
    return dups


def _validate_guest_layout(path: str) -> List[str]:
    errors = []
    if not os.path.exists(path):
        errors.append("guest-layout.json not found on disk")
        return errors
    try:
        with open(path, "r", encoding="utf-8") as f:
            data = json.load(f)
    except (json.JSONDecodeError, OSError) as e:
        errors.append(f"guest-layout.json parse error: {e}")
        return errors
    if not isinstance(data, dict):
        errors.append("guest-layout.json is not a JSON object")
        return errors
    root = data.get("root")
    if not root or not isinstance(root, str) or not root.startswith("/"):
        errors.append("guest-layout.json root must be absolute path")
    dirs = data.get("directories")
    if not isinstance(dirs, list) or len(dirs) == 0:
        errors.append("guest-layout.json directories must be non-empty list")
    else:
        for d in dirs:
            if not isinstance(d, str) or not d.startswith("/"):
                errors.append(f"guest-layout.json invalid directory entry: {d}")
                break
    return errors


def _validate_mount_contract(path: str) -> List[str]:
    errors = []
    if not os.path.exists(path):
        errors.append("mount-contract.json not found on disk")
        return errors
    try:
        with open(path, "r", encoding="utf-8") as f:
            data = json.load(f)
    except (json.JSONDecodeError, OSError) as e:
        errors.append(f"mount-contract.json parse error: {e}")
        return errors
    if not isinstance(data, dict):
        errors.append("mount-contract.json is not a JSON object")
        return errors
    binds = data.get("binds")
    if not isinstance(binds, list) or len(binds) == 0:
        errors.append("mount-contract.json binds must be non-empty list")
        return errors
    seen_targets = set()
    for b in binds:
        if not isinstance(b, dict):
            errors.append("mount-contract.json bind entry is not object")
            return errors
        source = b.get("source")
        target = b.get("target")
        read_only = b.get("readOnly")
        if not source or not isinstance(source, str):
            errors.append("mount-contract.json bind missing source")
        if not target or not isinstance(target, str) or not target.startswith("/"):
            errors.append("mount-contract.json bind missing/invalid target")
        else:
            if target in seen_targets:
                errors.append(f"mount-contract.json duplicate target: {target}")
            if target == "/":
                errors.append("mount-contract.json guest target must not be /")
            seen_targets.add(target)
        if read_only is None or not isinstance(read_only, bool):
            errors.append(f"mount-contract.json bind missing/invalid readOnly for target={target}")

    required_mounts = {
        "/opt/amitia": True,
        "/etc/amitia": False,
        "/var/lib/amitia": False,
        "/var/cache/amitia": False,
        "/var/log/amitia": False,
        "/run/amitia": False,
        "/home/amitia": False,
    }
    actual_mounts = {}
    for b in binds:
        if isinstance(b, dict) and b.get("target"):
            actual_mounts[b["target"]] = b.get("readOnly")

    for target, expected_readonly in required_mounts.items():
        if target not in actual_mounts:
            errors.append(f"mount-contract.json missing required mount: {target}")
        elif actual_mounts[target] != expected_readonly:
            errors.append(
                f"mount-contract.json {target} readOnly={actual_mounts[target]}, "
                f"expected readOnly={expected_readonly}"
            )

    return errors


def _validate_component_lock(path: str, runtime_version: str, package_id: str) -> List[str]:
    errors = []
    if not os.path.exists(path):
        errors.append("component-lock.json not found on disk")
        return errors
    try:
        with open(path, "r", encoding="utf-8") as f:
            data = json.load(f)
    except (json.JSONDecodeError, OSError) as e:
        errors.append(f"component-lock.json parse error: {e}")
        return errors
    if not isinstance(data, dict):
        errors.append("component-lock.json is not a JSON object")
        return errors
    if data.get("runtimeVersion") != runtime_version:
        errors.append("component-lock.json runtimeVersion mismatch")
    if data.get("packageId") != package_id:
        errors.append("component-lock.json packageId mismatch")
    components = data.get("components")
    if not isinstance(components, list) or len(components) == 0:
        errors.append("component-lock.json components must be non-empty list")
        return errors
    for c in components:
        if not isinstance(c, dict):
            errors.append("component-lock.json component entry not object")
            return errors
        if not c.get("id"):
            errors.append("component-lock.json component missing id")
        if not c.get("sha256") or len(c.get("sha256", "")) != 64:
            errors.append(f"component-lock.json component {c.get('id','?')} sha256 invalid")
        elif _has_placeholder(c.get("sha256", "")):
            errors.append(f"component-lock.json component {c.get('id','?')} sha256 is placeholder")
    return errors


RUNTIME_PROGRAM_CONTRACT_PATH = "runtime/contracts/runtime-program-contract.json"

FORBIDDEN_RUNTIME_FILES = {
    "local-token",
    "active-runtime.json",
    "runtime-manifest.json",
    "runtime.pid",
    "runtime.log",
    "logs/",
    "cache/",
    "user-db/",
    "qdrant/storage/",
    "qdrant/collections/",
}

VALID_RUNTIME_ENTRY_TYPES = {"file", "directory", "symlink"}


def _load_runtime_program_contract(contract_path: str) -> dict:
    if not os.path.exists(contract_path):
        return {}
    with open(contract_path, "r", encoding="utf-8") as f:
        data = json.load(f)
    return data if isinstance(data, dict) else {}


def _normalize_tar_path(name: str) -> str:
    parts = name.replace("\\", "/").split("/")
    normalized = []
    for p in parts:
        if p == "" or p == ".":
            continue
        if p == "..":
            if normalized:
                normalized.pop()
            continue
        normalized.append(p)
    return "/".join(normalized)


def validate_runtime_program_tree(
    tar_xz_path: str,
    contract_path: Optional[str] = None,
) -> List[dict]:
    errors = []
    if not os.path.exists(tar_xz_path):
        return [{"code": "RUNTIME_TAR_MISSING", "message": f"runtime.tar.xz not found: {tar_xz_path}"}]

    contract_path = contract_path or RUNTIME_PROGRAM_CONTRACT_PATH
    contract = _load_runtime_program_contract(contract_path)

    required_entries = []
    if contract:
        entries = contract.get("entries", [])
        for entry in entries:
            if not isinstance(entry, dict):
                continue
            if entry.get("required", True):
                required_entries.append(entry)

    try:
        with tarfile.open(tar_xz_path, mode="r:xz") as tf:
            members = tf.getmembers()
            seen_paths = set()

            for member in members:
                name = member.name

                if name.startswith("/") or name.startswith("\\"):
                    errors.append({
                        "code": "RUNTIME_PATH_ABSOLUTE",
                        "message": f"Absolute path in tar: {name}",
                    })
                    continue

                norm = _normalize_tar_path(name)
                if not norm:
                    errors.append({
                        "code": "RUNTIME_PATH_EMPTY",
                        "message": f"Empty normalized path for: {name}",
                    })
                    continue

                if ".." in name.split("/"):
                    errors.append({
                        "code": "RUNTIME_PATH_TRAVERSAL",
                        "message": f"Path traversal (..) detected: {name}",
                    })
                    continue

                if norm in seen_paths:
                    errors.append({
                        "code": "RUNTIME_PATH_DUPLICATE",
                        "message": f"Duplicate normalized path: {norm}",
                    })
                    continue
                seen_paths.add(norm)

                if member.issym() or member.islnk():
                    link_target = member.linkname
                    if link_target.startswith("/") or link_target.startswith("\\"):
                        errors.append({
                            "code": "RUNTIME_SYMLINK_ESCAPE",
                            "message": f"Absolute symlink/hardlink target: {name} -> {link_target}",
                        })
                        continue
                    target_norm = _normalize_tar_path(
                        os.path.join(os.path.dirname(norm), link_target).replace("\\", "/")
                    )
                    if target_norm.startswith(".."):
                        errors.append({
                            "code": "RUNTIME_SYMLINK_ESCAPE",
                            "message": f"Symlink/hardlink escapes root: {name} -> {link_target}",
                        })
                        continue

                for forbidden in FORBIDDEN_RUNTIME_FILES:
                    if norm == forbidden or norm.startswith(forbidden):
                        errors.append({
                            "code": "RUNTIME_FORBIDDEN_FILE",
                            "message": f"Forbidden file in runtime tree: {norm}",
                        })
                        break

            if contract:
                for entry in required_entries:
                    entry_path = entry.get("path", "")
                    entry_type = entry.get("type", "file")
                    if not entry_path:
                        continue
                    if entry_path not in seen_paths:
                        found = False
                        for sp in seen_paths:
                            if entry_type == "directory" and sp.startswith(entry_path.rstrip("/") + "/"):
                                found = True
                                break
                        if not found:
                            errors.append({
                                "code": "RUNTIME_REQUIRED_MISSING",
                                "message": f"Required entry missing: {entry_path} (type={entry_type})",
                            })

                if required_entries:
                    for entry in required_entries:
                        entry_path = entry.get("path", "")
                        if not entry_path:
                            continue
                        needs_exec = entry.get("executable", False)
                        if not needs_exec:
                            continue
                        if entry_path not in seen_paths:
                            continue
                        member_obj = None
                        for m in members:
                            if _normalize_tar_path(m.name) == entry_path:
                                member_obj = m
                                break
                        if member_obj is None:
                            continue
                        mode = member_obj.mode
                        if not (mode & stat.S_IXUSR):
                            errors.append({
                                "code": "RUNTIME_EXECUTABLE_MISSING",
                                "message": f"Required executable missing exec bit: {entry_path}",
                            })

    except (tarfile.TarError, EOFError, OSError) as e:
        errors.append({
            "code": "RUNTIME_TAR_PARSE_ERROR",
            "message": f"Failed to parse runtime.tar.xz: {e}",
        })

    return errors


def validate_package(
    package_path: str,
    expected_version: str,
    expected_package_id: str,
    expected_sha256: Optional[str] = None,
    expected_source_revision: Optional[str] = None,
) -> PackageValidationResult:
    result = PackageValidationResult()
    if not os.path.exists(package_path):
        result.errors.append({"code": "PACKAGE_MISSING", "message": f"Package not found: {package_path}"})
        return result

    actual_sha = sha256_file(package_path)
    result.sha256 = actual_sha
    result.expectedSha256 = expected_sha256 or ""
    if expected_sha256 and actual_sha != expected_sha256:
        result.errors.append(
            {
                "code": ValidationErrorCode.PACKAGE_SHA_MISMATCH,
                "message": f"SHA256 mismatch: expected {expected_sha256}, got {actual_sha}",
            }
        )
        return result

    tmp_dir = package_path + ".unpack"
    if os.path.exists(tmp_dir):
        import shutil
        shutil.rmtree(tmp_dir, ignore_errors=True)
    try:
        extract_package_safely(package_path, tmp_dir)

        for dirpath, dirnames, filenames in os.walk(tmp_dir):
            for name in filenames:
                full = os.path.join(dirpath, name)
                rel = os.path.relpath(full, tmp_dir).replace("\\", "/")
                if not any(rel == p or rel.startswith(p) for p in ALLOWED_TOP_PREFIXES):
                    result.errors.append(
                        {
                            "code": ValidationErrorCode.PACKAGE_UNKNOWN_PAYLOAD,
                            "message": f"Unknown entry in package: {rel}",
                        }
                    )
                    return result

        index_path = os.path.normpath(os.path.join(tmp_dir, PACKAGE_INDEX_PATH))
        if not os.path.exists(index_path):
            result.errors.append(
                {"code": ValidationErrorCode.PACKAGE_INDEX_INVALID, "message": "package-index.json missing"}
            )
            return result

        index_data = _parse_package_index(index_path)
        index_errors = _validate_package_index(index_data)
        if index_errors:
            for err in index_errors:
                result.errors.append(
                    {"code": ValidationErrorCode.PACKAGE_INDEX_INVALID, "message": err}
                )
            return result

        runtime_version = index_data.get("runtimeVersion", "")
        package_id = index_data.get("packageId", "")

        if runtime_version != expected_version:
            result.errors.append(
                {
                    "code": "VERSION_MISMATCH",
                    "message": f"Version mismatch: expected {expected_version}, got {runtime_version}",
                }
            )
        if package_id != expected_package_id:
            result.errors.append(
                {"code": "PACKAGE_ID_MISMATCH", "message": "Package ID mismatch"}
            )

        rootfs_ref = _extract_payload_ref(index_data, "rootfs")
        runtime_ref = _extract_payload_ref(index_data, "runtime")
        guest_layout_ref = _extract_payload_ref(index_data, "guest-layout")
        mount_contract_ref = _extract_payload_ref(index_data, "mount-contract")
        sha256sums_ref = _extract_payload_ref(index_data, "sha256sums")

        if rootfs_ref is None:
            result.errors.append({"code": ValidationErrorCode.PACKAGE_ROOTFS_MISSING, "message": "rootfs payload missing"})
        if runtime_ref is None:
            result.errors.append({"code": ValidationErrorCode.PACKAGE_RUNTIME_MISSING, "message": "runtime payload missing"})
        if guest_layout_ref is None:
            result.errors.append({"code": ValidationErrorCode.PACKAGE_INDEX_INVALID, "message": "guest-layout metadata missing"})
        if mount_contract_ref is None:
            result.errors.append({"code": ValidationErrorCode.PACKAGE_INDEX_INVALID, "message": "mount-contract metadata missing"})
        if sha256sums_ref is None:
            result.errors.append({"code": ValidationErrorCode.PACKAGE_INDEX_INVALID, "message": "sha256sums metadata missing"})

        if result.errors:
            return result

        sums_path = os.path.normpath(os.path.join(tmp_dir, SHA256SUMS_PATH))
        checksums = _parse_sha256sums(sums_path)
        if not checksums:
            result.errors.append(
                {"code": ValidationErrorCode.PACKAGE_INDEX_INVALID, "message": "SHA256SUMS empty or unparseable"}
            )
            return result

        required_payloads = [ROOTFS_PAYLOAD_PATH, RUNTIME_PAYLOAD_PATH]
        for payload in required_payloads:
            rel = payload
            full_path = os.path.normpath(os.path.join(tmp_dir, rel))
            if not os.path.exists(full_path):
                code = (
                    ValidationErrorCode.PACKAGE_ROOTFS_MISSING
                    if "rootfs" in rel
                    else ValidationErrorCode.PACKAGE_RUNTIME_MISSING
                )
                result.errors.append({"code": code, "message": f"Missing payload file: {rel}"})
            else:
                actual = sha256_file(full_path)
                if rel in checksums and actual != checksums[rel]:
                    result.errors.append(
                        {"code": "PAYLOAD_SHA_MISMATCH", "message": f"Payload SHA mismatch: {rel}"}
                    )

        if result.errors:
            return result

        actual_rootfs_sha = sha256_file(os.path.normpath(os.path.join(tmp_dir, ROOTFS_PAYLOAD_PATH)))
        if rootfs_ref and actual_rootfs_sha != rootfs_ref.get("sha256"):
            result.errors.append(
                {"code": "PAYLOAD_SHA_MISMATCH", "message": "rootfs payload sha256 does not match index"}
            )
        actual_runtime_sha = sha256_file(os.path.normpath(os.path.join(tmp_dir, RUNTIME_PAYLOAD_PATH)))
        if runtime_ref and actual_runtime_sha != runtime_ref.get("sha256"):
            result.errors.append(
                {"code": "PAYLOAD_SHA_MISMATCH", "message": "runtime payload sha256 does not match index"}
            )

        lock_path = os.path.normpath(os.path.join(tmp_dir, COMPONENT_LOCK_PATH))
        lock_errors = _validate_component_lock(lock_path, runtime_version, package_id)
        if lock_errors:
            for err in lock_errors:
                result.errors.append(
                    {"code": ValidationErrorCode.PACKAGE_COMPONENT_LOCK_INVALID, "message": err}
                )
            return result

        gl_path = os.path.normpath(os.path.join(tmp_dir, GUEST_LAYOUT_PATH))
        gl_errors = _validate_guest_layout(gl_path)
        if gl_errors:
            for err in gl_errors:
                result.errors.append(
                    {"code": ValidationErrorCode.PACKAGE_INDEX_INVALID, "message": f"guest-layout error: {err}"}
                )
            return result

        mc_path = os.path.normpath(os.path.join(tmp_dir, MOUNT_CONTRACT_PATH))
        mc_errors = _validate_mount_contract(mc_path)
        if mc_errors:
            for err in mc_errors:
                result.errors.append(
                    {"code": ValidationErrorCode.PACKAGE_INDEX_INVALID, "message": f"mount-contract error: {err}"}
                )
            return result

        expected_sums = _parse_sha256sums(sums_path)
        for rel_path, expected_digest in expected_sums.items():
            full = os.path.normpath(os.path.join(tmp_dir, rel_path))
            if not os.path.exists(full):
                result.errors.append(
                    {"code": "PAYLOAD_SHA_MISMATCH", "message": f"SHA256SUMS references missing file: {rel_path}"}
                )
            else:
                actual_digest = sha256_file(full)
                if actual_digest != expected_digest:
                    result.errors.append(
                        {"code": "PAYLOAD_SHA_MISMATCH", "message": f"SHA256SUMS content mismatch: {rel_path}"}
                    )

        manifest = PackageManifest()
        manifest.runtimeVersion = runtime_version
        manifest.packageId = package_id
        manifest.sourceRevision = index_data.get("sourceRevision", "")
        if expected_source_revision and manifest.sourceRevision != expected_source_revision:
            result.errors.append(
                {"code": "SOURCE_REVISION_MISMATCH", "message": "Source revision mismatch"}
            )
        target = index_data.get("target", {})
        manifest.hostPlatform = str(target.get("hostPlatform", ""))
        manifest.hostAbi = str(target.get("hostAbi", ""))
        manifest.runtimeKind = str(target.get("runtimeKind", ""))
        manifest.guestPlatform = str(target.get("guestPlatform", ""))
        manifest.guestArchitecture = str(target.get("guestArchitecture", ""))
        if rootfs_ref:
            manifest.rootfsPayloadPath = str(rootfs_ref.get("path", ""))
            manifest.rootfsPayloadSha256 = str(rootfs_ref.get("sha256", ""))
            manifest.rootfsPayloadSize = int(rootfs_ref.get("size", 0))
        if runtime_ref:
            manifest.runtimePayloadPath = str(runtime_ref.get("path", ""))
            manifest.runtimePayloadSha256 = str(runtime_ref.get("sha256", ""))
            manifest.runtimePayloadSize = int(runtime_ref.get("size", 0))
        if guest_layout_ref:
            manifest.guestLayoutPath = str(guest_layout_ref.get("path", ""))
            manifest.guestLayoutSha256 = str(guest_layout_ref.get("sha256", ""))
        if mount_contract_ref:
            manifest.mountContractPath = str(mount_contract_ref.get("path", ""))
            manifest.mountContractSha256 = str(mount_contract_ref.get("sha256", ""))
        if sha256sums_ref:
            manifest.sha256sumsPath = str(sha256sums_ref.get("path", ""))
            manifest.sha256sumsSha256 = str(sha256sums_ref.get("sha256", ""))

        runtime_tar_xz = os.path.normpath(os.path.join(tmp_dir, RUNTIME_PAYLOAD_PATH))
        if os.path.exists(runtime_tar_xz):
            tree_errors = validate_runtime_program_tree(runtime_tar_xz)
            if tree_errors:
                for err in tree_errors:
                    result.errors.append(
                        {"code": ValidationErrorCode.PACKAGE_RUNTIME_INVALID, "message": f"runtime program tree: {err['message']}"}
                    )

        result.manifest = manifest
        result.valid = len(result.errors) == 0
        return result
    finally:
        import shutil
        shutil.rmtree(tmp_dir, ignore_errors=True)
