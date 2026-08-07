#!/usr/bin/env python3
import argparse
import hashlib
import json
import os
import pathlib
import shutil

from archive import (
    create_deterministic_tar,
    sha256_file,
    ARCHIVE_MTIME,
    DEFAULT_DIR_MODE,
    VAR_DIR_MODE,
    PRIVATE_DIR_MODE,
    FILE_MODE,
    XZ_COMPRESSION_LEVEL,
    verify_no_extra_rootfs,
)

SCRIPT_DIR = pathlib.Path(__file__).resolve().parent
LOCK_FILE = SCRIPT_DIR / "guest-layout.lock.json"
DEFAULT_OUTPUT_DIR = pathlib.Path(__file__).resolve().parent.parent.parent / "out" / "guest-layout" / "ubuntu-arm64"
ARCHIVE_NAME = "amitia-guest-layout-v1-ubuntu-arm64.tar.xz"
MANIFEST_DIR_NAME = "manifest"
MANIFEST_GUEST_LAYOUT = "guest-layout.json"
MANIFEST_MOUNT_CONTRACT = "mount-contract.json"
FILE_MANIFEST = "file-manifest.json"
SHA256SUMS_FILE = "SHA256SUMS"

AMITIA_UID = 1000
AMITIA_GID = 1000
MAX_PATH_LEN = 4096

ROOT_CHILDREN = ("opt", "etc", "var", "run")


def load_json(path):
    with open(path, "r", encoding="utf-8") as f:
        return json.load(f)


def write_json(path, data):
    with open(path, "w", encoding="utf-8", newline="") as f:
        json.dump(data, f, indent=2, ensure_ascii=False, sort_keys=False)
        f.write("\n")


def mode_from_string(mode_str):
    return int(mode_str, 8)


def build_guest_layout_json(lock):
    env = lock["environment"]
    comp = lock["components"]
    paths = lock["paths"]
    mapping = lock["resourceUriMapping"]
    return {
        "schemaVersion": lock["schemaVersion"],
        "componentId": lock["componentId"],
        "version": lock["version"],
        "platform": lock["platform"],
        "architecture": lock["architecture"],
        "runtimeKind": lock["runtimeKind"],
        "paths": {
            "runtimeRoot": paths["runtimeRoot"],
            "backendBinary": comp["backend"]["binary"],
            "nodeBinary": comp["node"]["binary"],
            "npmCli": comp["node"]["npmCli"],
            "npxCli": comp["node"]["npxCli"],
            "qdrantBinary": comp["qdrant"]["binary"],
            "pluginHostEntry": comp["pluginHost"]["entry"],
            "taskHostEntry": comp["taskHost"]["entry"],
            "configRoot": paths["configRoot"],
            "dataRoot": paths["dataRoot"],
            "cacheRoot": paths["cacheRoot"],
            "logRoot": paths["logRoot"],
            "runRoot": paths["runRoot"],
            "tempRoot": paths["tempRoot"],
            "workspaceRoot": paths["workspaceRoot"],
            "qdrantConfig": comp["qdrant"]["config"],
            "qdrantStorage": comp["qdrant"]["storage"],
            "qdrantSnapshots": comp["qdrant"]["snapshots"],
        },
        "environment": {k: env[k] for k in [
            "AMITIA_RUNTIME_ROOT", "AMITIA_CONFIG_ROOT", "AMITIA_DATA_ROOT",
            "AMITIA_CACHE_ROOT", "AMITIA_LOG_ROOT", "AMITIA_RUN_ROOT",
            "AMITIA_TEMP_ROOT", "AMITIA_WORKSPACE_ROOT", "AMITIA_HOME",
            "HOME", "LANG", "LC_ALL", "TZ",
        ] if k in env},
        "resourceUriMapping": mapping,
        "nodePaths": {
            "AMITIA_NODE_HOME": env.get("AMITIA_NODE_HOME", "/var/lib/amitia/node/home"),
            "AMITIA_NODE_PREFIX": env.get("AMITIA_NODE_PREFIX", "/var/lib/amitia/node/prefix"),
            "AMITIA_NPM_CACHE": env.get("AMITIA_NPM_CACHE", "/var/cache/amitia/node/npm"),
            "AMITIA_NODE_TMP": env.get("AMITIA_NODE_TMP", "/run/amitia/tmp/node"),
        },
    }


def build_mount_contract_json(lock):
    return {
        "schemaVersion": lock["schemaVersion"],
        "componentId": "runtime.guest-mount-contract",
        "version": lock["version"],
        "mounts": lock["mountContract"],
        "order": [m["id"] for m in lock["mountContract"]],
    }


def build_file_manifest(overlay_root, data):
    manifest = []
    overlay_root = pathlib.Path(overlay_root)
    dir_lookup = {}
    for d in data["directories"]:
        dir_lookup[d["path"]] = d

    seen = set()
    for root, dirs, files in os.walk(str(overlay_root), followlinks=False):
        rel = pathlib.Path(root).relative_to(overlay_root)
        rel_str = rel.as_posix() if str(rel) != "." else ""
        if rel_str == "":
            continue
        if rel_str in seen:
            continue
        seen.add(rel_str)
        guest_abs = "/" + rel_str
        doc = dir_lookup.get(guest_abs, {})
        mode_str = doc.get("mode") if doc else None
        if mode_str is None:
            parts = rel_str.split("/")
            first = parts[0] if parts else ""
            mode_str = "0755" if first == "opt" else "0750"
        manifest.append({
            "path": guest_abs,
            "type": "directory",
            "mode": mode_str,
            "uid": doc.get("ownerUid", 0),
            "gid": doc.get("ownerGid", 0),
            "persistence": doc.get("persistence", "immutable"),
            "purpose": doc.get("purpose", "immutable"),
        })
    manifest.sort(key=lambda e: e["path"])
    return manifest


def build_overlay(output_dir, keep_work_dir=False):
    lock = load_json(LOCK_FILE)
    output_dir = pathlib.Path(output_dir)

    if output_dir.exists():
        shutil.rmtree(output_dir)
    output_dir.mkdir(parents=True, exist_ok=True)

    work_dir = output_dir / ".work"
    overlay_root = work_dir / "overlay"
    work_dir.mkdir(parents=True, exist_ok=True)
    overlay_root.mkdir()

    dir_lookup = {}
    for d in lock["directories"]:
        dir_lookup[d["path"]] = d

    for d in lock["directories"]:
        guest_path = d["path"]
        rel = guest_path.lstrip("/")
        target = overlay_root / rel
        target.mkdir(parents=True, exist_ok=True)

    manifest_dir = overlay_root / "opt" / "amitia" / MANIFEST_DIR_NAME
    guest_layout_json = build_guest_layout_json(lock)
    mount_contract_json = build_mount_contract_json(lock)
    write_json(manifest_dir / MANIFEST_GUEST_LAYOUT, guest_layout_json)
    write_json(manifest_dir / MANIFEST_MOUNT_CONTRACT, mount_contract_json)

    file_manifest = build_file_manifest(overlay_root, lock)
    write_json(work_dir / FILE_MANIFEST, file_manifest)

    archive_path = work_dir / ARCHIVE_NAME
    create_deterministic_tar(overlay_root, archive_path)

    sha_archive = sha256_file(archive_path)
    gl_json_bytes = json.dumps(guest_layout_json, indent=2, ensure_ascii=False, sort_keys=False).encode("utf-8") + b"\n"
    mc_json_bytes = json.dumps(mount_contract_json, indent=2, ensure_ascii=False, sort_keys=False).encode("utf-8") + b"\n"
    fm_json_bytes = json.dumps(file_manifest, indent=2, ensure_ascii=False, sort_keys=False).encode("utf-8") + b"\n"
    sha_gl = hashlib.sha256(gl_json_bytes).hexdigest()
    sha_mc = hashlib.sha256(mc_json_bytes).hexdigest()
    sha_fm = hashlib.sha256(fm_json_bytes).hexdigest()

    sha256_lines = []
    for fname, sha_val in sorted([
        (ARCHIVE_NAME, sha_archive),
        (MANIFEST_GUEST_LAYOUT, sha_gl),
        (MANIFEST_MOUNT_CONTRACT, sha_mc),
        (FILE_MANIFEST, sha_fm),
    ]):
        sha256_lines.append(f"{sha_val}  {fname}")
    sha256_content = "\n".join(sha256_lines) + "\n"

    shutil.copytree(str(overlay_root), str(output_dir / "overlay"))
    shutil.copy2(str(manifest_dir / MANIFEST_GUEST_LAYOUT), str(output_dir / MANIFEST_GUEST_LAYOUT))
    shutil.copy2(str(manifest_dir / MANIFEST_MOUNT_CONTRACT), str(output_dir / MANIFEST_MOUNT_CONTRACT))
    shutil.copy2(str(work_dir / FILE_MANIFEST), str(output_dir / FILE_MANIFEST))
    shutil.copy2(str(archive_path), str(output_dir / ARCHIVE_NAME))
    with open(output_dir / SHA256SUMS_FILE, "w", encoding="utf-8", newline="") as f:
        f.write(sha256_content)

    if not keep_work_dir:
        shutil.rmtree(work_dir)
    else:
        print(work_dir)

    return {
        "archive_sha": sha_archive,
        "guest_layout_json": guest_layout_json,
        "mount_contract_json": mount_contract_json,
        "file_manifest": file_manifest,
    }


def main():
    parser = argparse.ArgumentParser(description="Guest Layout builder")
    parser.add_argument("--clean", action="store_true", help="clean output dir before build")
    parser.add_argument("--output-dir", type=str, default=str(DEFAULT_OUTPUT_DIR))
    parser.add_argument("--verify-rootfs", type=str, default=None)
    parser.add_argument("--keep-work-dir", action="store_true")
    args = parser.parse_args()
    output_dir = pathlib.Path(args.output_dir)
    if args.clean and output_dir.exists():
        shutil.rmtree(output_dir)
    if output_dir.exists() and args.clean is False:
        pass
    result = build_overlay(output_dir, keep_work_dir=args.keep_work_dir)
    print(f"[build] archive sha256: {result['archive_sha']}")
    print(f"[build] output: {output_dir}")


if __name__ == "__main__":
    main()
