import json
import os
from typing import List, Dict, Optional, Tuple

from common.hashing import sha256_file


def generate_package_index(
    runtime_version: str,
    package_id: str,
    source_revision: str,
    payloads: List[Dict],
    metadata: List[Dict],
    host_platform: str = "android",
    host_abi: str = "arm64-v8a",
    runtime_kind: str = "proot",
    guest_platform: str = "linux",
    guest_architecture: str = "arm64",
) -> dict:
    return {
        "schemaVersion": 1,
        "runtimeVersion": runtime_version,
        "packageId": package_id,
        "sourceRevision": source_revision,
        "target": {
            "hostPlatform": host_platform,
            "hostAbi": host_abi,
            "runtimeKind": runtime_kind,
            "guestPlatform": guest_platform,
            "guestArchitecture": guest_architecture,
        },
        "payloads": payloads,
        "metadata": metadata,
    }


def validate_package_index(index: dict) -> List[str]:
    errors = []

    if not isinstance(index.get("schemaVersion"), int) or index["schemaVersion"] < 1:
        errors.append("schemaVersion must be a positive integer")

    if not index.get("runtimeVersion"):
        errors.append("runtimeVersion is required")

    if not index.get("packageId"):
        errors.append("packageId is required")

    if not index.get("sourceRevision"):
        errors.append("sourceRevision is required")

    target = index.get("target", {})
    if not target:
        errors.append("target is required")
    else:
        if target.get("hostPlatform") != "android":
            errors.append(f"hostPlatform must be 'android', got '{target.get('hostPlatform')}'")
        if target.get("hostAbi") != "arm64-v8a":
            errors.append(f"hostAbi must be 'arm64-v8a', got '{target.get('hostAbi')}'")
        if target.get("runtimeKind") != "proot":
            errors.append(f"runtimeKind must be 'proot', got '{target.get('runtimeKind')}'")
        if target.get("guestPlatform") != "linux":
            errors.append(f"guestPlatform must be 'linux', got '{target.get('guestPlatform')}'")
        if target.get("guestArchitecture") != "arm64":
            errors.append(f"guestArchitecture must be 'arm64', got '{target.get('guestArchitecture')}'")

    payloads = index.get("payloads", [])
    if not payloads:
        errors.append("payloads must be a non-empty list")
    else:
        roles = set()
        for i, p in enumerate(payloads):
            if not p.get("path"):
                errors.append(f"payloads[{i}].path is required")
            if not p.get("sha256"):
                errors.append(f"payloads[{i}].sha256 is required")
            if "size" not in p:
                errors.append(f"payloads[{i}].size is required")
            elif not isinstance(p.get("size"), int):
                errors.append(f"payloads[{i}].size must be a JSON number (int), got {type(p['size']).__name__}")
            if p.get("path"):
                roles.add(p["path"])
        if "payload/rootfs/rootfs.tar.xz" not in roles:
            errors.append("payloads must contain 'payload/rootfs/rootfs.tar.xz'")
        if "payload/runtime/runtime.tar.xz" not in roles:
            errors.append("payloads must contain 'payload/runtime/runtime.tar.xz'")

    meta_entries = index.get("metadata", [])
    if not meta_entries:
        errors.append("metadata must be a non-empty list")
    else:
        meta_paths = set()
        for i, m in enumerate(meta_entries):
            if not m.get("path"):
                errors.append(f"metadata[{i}].path is required")
            if not m.get("sha256"):
                errors.append(f"metadata[{i}].sha256 is required")
            if "size" not in m:
                errors.append(f"metadata[{i}].size is required")
            elif not isinstance(m.get("size"), int):
                errors.append(f"metadata[{i}].size must be a JSON number (int), got {type(m['size']).__name__}")
            if m.get("path"):
                meta_paths.add(m["path"])
    required_meta = {
        "metadata/package-index.json",
        "metadata/component-lock.json",
        "metadata/SHA256SUMS",
        "metadata/guest-layout.json",
        "metadata/mount-contract.json",
    }
    missing_meta = required_meta - meta_paths
    if missing_meta:
        errors.append(f"metadata missing required entries: {sorted(missing_meta)}")

    return errors


def compute_package_index_sha256(index: dict) -> str:
    from .common.hashing import sha256_bytes
    canonical = json.dumps(index, sort_keys=True, ensure_ascii=False, separators=(",", ":"))
    return sha256_bytes(canonical.encode("utf-8"))


def write_package_index(index: dict, output_path: str) -> str:
    from common.hashing import sha256_file
    parent = os.path.dirname(output_path)
    if parent:
        os.makedirs(parent, exist_ok=True)
    content = json.dumps(index, indent=2, ensure_ascii=False)
    with open(output_path, "w", encoding="utf-8") as f:
        f.write(content)
        f.write("\n")
    return sha256_file(output_path)
