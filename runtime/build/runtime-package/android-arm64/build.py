import argparse
import hashlib
import json
import os
import shutil
import sys
import tarfile
import tempfile
import zipfile
from pathlib import Path

sys.path.insert(0, str(Path(__file__).resolve().parents[3]))
from common import (
    ArtifactRecord,
    BuildRecordBuilder,
    sha256_file,
)


REQUIRED_PROGRAM_PATHS = [
    "backend/amitia-server",
    "node/bin/node",
    "node/lib/node_modules/npm/bin/npm-cli.js",
    "node/lib/node_modules/npm/bin/npx-cli.js",
    "qdrant/bin/qdrant",
    "plugin-host/dist/index.js",
    "task-host/dist/index.js",
    "scripts/node/amitia-node-prepare.sh",
    "scripts/node/amitia-node-probe.sh",
    "manifest/guest-layout.json",
    "manifest/mount-contract.json",
]


def sha256_bytes(data: bytes) -> str:
    return hashlib.sha256(data).hexdigest()


def resolve_frozen_artifact(output_base: str, component_id: str) -> tuple:
    """Find a frozen artifact by reading its build-record.json."""
    build_record_path = None
    base_dir = os.path.join(output_base, component_id, "linux-arm64")

    if not os.path.exists(base_dir):
        raise FileNotFoundError(f"Component directory not found: {base_dir}")

    for entry in sorted(os.listdir(base_dir)):
        candidate = os.path.join(base_dir, entry, "build-record.json")
        if os.path.isfile(candidate):
            build_record_path = candidate
            break

    if not build_record_path:
        raise FileNotFoundError(f"No build-record.json found for component: {component_id}")

    with open(build_record_path, "r", encoding="utf-8") as f:
        record = json.load(f)

    artifact_rel = record.get("artifactRelativePath", "")
    artifact_dir = os.path.dirname(build_record_path)
    return artifact_dir, record


def build_program_tree(
    output_base: str,
    components: list,
    program_tree_dir: str,
) -> dict:
    """Assemble the final /opt/amitia program tree from frozen artifacts."""
    results = {}
    for comp_id in components:
        artifact_dir, record = resolve_frozen_artifact(output_base, comp_id)
        dest_dir = os.path.join(program_tree_dir, comp_id)
        shutil.copytree(artifact_dir, dest_dir, dirs_exist_ok=True)
        results[comp_id] = record

    manifest_src = os.path.join(os.path.dirname(output_base), "..", "contracts", "guest-layout.json")
    contract_src = os.path.join(os.path.dirname(output_base), "..", "contracts", "mount-contract.json")
    manifest_dest = os.path.join(program_tree_dir, "manifest")
    os.makedirs(manifest_dest, exist_ok=True)
    if os.path.exists(manifest_src):
        shutil.copy2(manifest_src, os.path.join(manifest_dest, "guest-layout.json"))
    if os.path.exists(contract_src):
        shutil.copy2(contract_src, os.path.join(manifest_dest, "mount-contract.json"))

    return results


def build(
    output_base: str,
    version: str,
    package_id: str,
    components: list,
) -> dict:
    staging_dir = os.path.join(output_base, ".staging", version)
    if os.path.exists(staging_dir):
        shutil.rmtree(staging_dir, ignore_errors=True)
    os.makedirs(staging_dir, exist_ok=True)

    program_dir = os.path.join(staging_dir, "program")
    os.makedirs(program_dir, exist_ok=True)

    comp_records = build_program_tree(output_base, components, program_dir)

    for required in REQUIRED_PROGRAM_PATHS:
        full_path = os.path.join(program_dir, required)
        if not os.path.exists(full_path):
            raise FileNotFoundError(f"Required program file missing: {required}")

    runtime_tar = os.path.join(staging_dir, "runtime.tar.xz")
    with tarfile.open(runtime_tar, "w:xz") as tf:
        tf.add(program_dir, arcname=".")

    rootfs_dir = os.path.join(output_base, "rootfs", "linux-arm64")
    rootfs_archive = None
    if os.path.exists(rootfs_dir):
        for entry in sorted(os.listdir(rootfs_dir)):
            candidate = os.path.join(rootfs_dir, entry, "rootfs.tar.xz")
            if os.path.isfile(candidate):
                rootfs_archive = candidate
                break

    if not rootfs_archive:
        raise FileNotFoundError("No rootfs.tar.xz frozen artifact found")

    guest_layout_src = os.path.join(os.path.dirname(output_base), "..", "contracts", "guest-layout.json")
    mount_contract_src = os.path.join(os.path.dirname(output_base), "..", "contracts", "mount-contract.json")

    guest_layout_sha = sha256_file(guest_layout_src) if os.path.exists(guest_layout_src) else ""
    mount_contract_sha = sha256_file(mount_contract_src) if os.path.exists(mount_contract_src) else ""
    runtime_sha = sha256_file(runtime_tar)
    rootfs_sha = sha256_file(rootfs_archive)

    comp_lock_components = []
    for comp_id, rec in comp_records.items():
        comp_lock_components.append({
            "id": comp_id,
            "version": rec.get("version", ""),
            "architecture": rec.get("architecture", "arm64"),
            "path": rec.get("artifactRelativePath", ""),
            "sha256": rec.get("artifactSha256", ""),
        })

    component_lock = {
        "runtimeVersion": version,
        "packageId": package_id,
        "components": comp_lock_components,
    }

    index = {
        "schemaVersion": 1,
        "runtimeVersion": version,
        "packageId": package_id,
        "target": {
            "hostPlatform": "android",
            "hostAbi": "arm64-v8a",
            "runtimeKind": "proot",
            "guestPlatform": "linux",
            "guestArchitecture": "arm64",
        },
        "payloads": [
            {
                "role": "rootfs",
                "path": "payload/rootfs/rootfs.tar.xz",
                "sha256": rootfs_sha,
                "size": os.path.getsize(rootfs_archive),
            },
            {
                "role": "runtime",
                "path": "payload/runtime/runtime.tar.xz",
                "sha256": runtime_sha,
                "size": os.path.getsize(runtime_tar),
            },
        ],
        "metadata": [
            {
                "role": "guest-layout",
                "path": "metadata/guest-layout.json",
                "sha256": guest_layout_sha,
                "size": os.path.getsize(guest_layout_src) if os.path.exists(guest_layout_src) else 0,
            },
            {
                "role": "mount-contract",
                "path": "metadata/mount-contract.json",
                "sha256": mount_contract_sha,
                "size": os.path.getsize(mount_contract_src) if os.path.exists(mount_contract_src) else 0,
            },
            {
                "role": "sha256sums",
                "path": "metadata/SHA256SUMS",
                "sha256": "",
                "size": 0,
            },
        ],
    }

    metadata_dir = os.path.join(staging_dir, "metadata")
    os.makedirs(metadata_dir, exist_ok=True)
    payload_dir = os.path.join(staging_dir, "payload", "runtime")
    payload_rootfs_dir = os.path.join(staging_dir, "payload", "rootfs")
    os.makedirs(payload_dir, exist_ok=True)
    os.makedirs(payload_rootfs_dir, exist_ok=True)

    shutil.copy2(runtime_tar, os.path.join(payload_dir, "runtime.tar.xz"))
    shutil.copy2(rootfs_archive, os.path.join(payload_rootfs_dir, "rootfs.tar.xz"))

    with open(os.path.join(metadata_dir, "package-index.json"), "w", encoding="utf-8") as f:
        json.dump(index, f, indent=2, ensure_ascii=False)

    with open(os.path.join(metadata_dir, "component-lock.json"), "w", encoding="utf-8") as f:
        json.dump(component_lock, f, indent=2, ensure_ascii=False)

    if os.path.exists(guest_layout_src):
        shutil.copy2(guest_layout_src, os.path.join(metadata_dir, "guest-layout.json"))
    if os.path.exists(mount_contract_src):
        shutil.copy2(mount_contract_src, os.path.join(metadata_dir, "mount-contract.json"))

    sums = {}
    for root, dirs, files in os.walk(staging_dir):
        for fname in files:
            full = os.path.join(root, fname)
            rel = os.path.relpath(full, staging_dir).replace("\\", "/")
            if rel == "metadata/SHA256SUMS":
                continue
            sums[rel] = sha256_file(full)

    sums_content = "".join(f"{v}  {k}\n" for k, v in sorted(sums.items()))
    sums_path = os.path.join(metadata_dir, "SHA256SUMS")
    with open(sums_path, "w", encoding="utf-8") as f:
        f.write(sums_content)

    sums_sha = sha256_bytes(sums_content.encode("utf-8"))
    index["metadata"][2]["sha256"] = sums_sha
    index["metadata"][2]["size"] = len(sums_content.encode("utf-8"))
    with open(os.path.join(metadata_dir, "package-index.json"), "w", encoding="utf-8") as f:
        json.dump(index, f, indent=2, ensure_ascii=False)

    package_dir = os.path.join(output_base, "runtime-package", "android-arm64", version)
    os.makedirs(package_dir, exist_ok=True)
    package_file = os.path.join(package_dir, f"amitia-runtime-{version}-linux-arm64.zip")

    with zipfile.ZipFile(package_file, "w", zipfile.ZIP_DEFLATED) as zf:
        for root, dirs, files in os.walk(staging_dir):
            for fname in files:
                full = os.path.join(root, fname)
                rel = os.path.relpath(full, staging_dir).replace("\\", "/")
                zf.write(full, rel)

    package_sha = sha256_file(package_file)

    build_record = {
        "schemaVersion": 1,
        "runtimeVersion": version,
        "packageId": package_id,
        "packageFormatVersion": 1,
        "packageFileName": f"amitia-runtime-{version}-linux-arm64.zip",
        "packageSha256": package_sha,
        "packageSize": os.path.getsize(package_file),
        "runtimePayloadSha256": runtime_sha,
        "rootfsSha256": rootfs_sha,
        "guestLayoutSha256": guest_layout_sha,
        "mountContractSha256": mount_contract_sha,
        "componentLockSha256": sha256_bytes(json.dumps(component_lock, sort_keys=True).encode("utf-8")),
        "componentBuildRecordShas": {cid: r.get("artifactSha256", "") for cid, r in comp_records.items()},
        "buildMode": "release",
    }

    record_path = os.path.join(package_dir, "runtime-package-build-record.json")
    with open(record_path, "w", encoding="utf-8") as f:
        json.dump(build_record, f, indent=2, ensure_ascii=False)

    sha256_files_path = os.path.join(package_dir, "runtime-package-files.sha256")
    with open(sha256_files_path, "w", encoding="utf-8") as f:
        for k, v in sorted(sums.items()):
            f.write(f"{v}  {k}\n")

    shutil.rmtree(os.path.join(output_base, ".staging"), ignore_errors=True)

    return build_record


def main():
    parser = argparse.ArgumentParser(description="Runtime Package android-arm64 Builder")
    parser.add_argument("--output-base", required=True)
    parser.add_argument("--version", required=True)
    parser.add_argument("--package-id", required=True)
    parser.add_argument("--components", nargs="+", required=True)
    args = parser.parse_args()

    record = build(args.output_base, args.version, args.package_id, args.components)
    print(json.dumps(record, indent=2))


if __name__ == "__main__":
    main()
