#!/usr/bin/env python3
"""Strict validator for the Android embedded Amitia runtime package.

This validator is intentionally kept inside mobile_app so ordinary Flutter/Gradle
builds can reject stale embedded runtime assets before an APK is produced.
"""

import argparse
import hashlib
import json
import sys
import tarfile
import zipfile


REQUIRED_RUNTIME_FILES = {
    "backend/amitia-server",
    "node/bin/node",
    "node/lib/node_modules/npm/bin/npm-cli.js",
    "node/lib/node_modules/npm/bin/npx-cli.js",
    "qdrant/bin/qdrant",
    "plugin-host/dist/index.js",
    "task-host/dist/index.js",
    "scripts/node/amitia-node-prepare.sh",
    "scripts/node/amitia-node-probe.sh",
}

ELF_BINARIES = {
    "backend/amitia-server",
    "node/bin/node",
    "qdrant/bin/qdrant",
}

EXECUTABLE_FILES = ELF_BINARIES | {
    "scripts/node/amitia-node-prepare.sh",
    "scripts/node/amitia-node-probe.sh",
}

REQUIRED_COMPONENTS = {
    "runtime.backend",
    "runtime.node",
    "runtime.plugin-host",
    "runtime.qdrant",
    "runtime.task-host",
}

REQUIRED_METADATA = {
    "metadata/package-index.json",
    "metadata/component-index.json",
    "metadata/component-lock.json",
    "metadata/file-manifest.json",
    "metadata/guest-layout.json",
    "metadata/mount-contract.json",
    "metadata/SHA256SUMS",
}

FORBIDDEN_RUNTIME_DIRS = {"config", "data", "cache", "logs", "run", "workspaces"}

REQUIRED_ROOTFS_EXECUTABLES = {"usr/bin/true"}


def normalize_member_name(name: str) -> str:
    return name[2:] if name.startswith("./") else name


def sha256_file(path: str) -> str:
    digest = hashlib.sha256()
    with open(path, "rb") as stream:
        for chunk in iter(lambda: stream.read(1024 * 1024), b""):
            digest.update(chunk)
    return digest.hexdigest()


def sha256_zip_entry(archive: zipfile.ZipFile, path: str) -> str:
    digest = hashlib.sha256()
    with archive.open(path) as stream:
        for chunk in iter(lambda: stream.read(1024 * 1024), b""):
            digest.update(chunk)
    return digest.hexdigest()


def parse_sha256sums(text: str) -> dict[str, str]:
    result: dict[str, str] = {}
    for line in text.splitlines():
        parts = line.strip().split(None, 1)
        if len(parts) == 2:
            result[parts[1].strip()] = parts[0].strip().lower()
    return result


def validate(package_path: str) -> dict[str, object]:
    package_hash = sha256_file(package_path)
    with zipfile.ZipFile(package_path) as archive:
        entries = archive.infolist()
        names = {entry.filename for entry in entries}
        if len(names) != len(entries):
            seen: set[str] = set()
            duplicates: list[str] = []
            for entry in entries:
                if entry.filename in seen:
                    duplicates.append(entry.filename)
                seen.add(entry.filename)
            raise RuntimeError("duplicate archive entries: " + ", ".join(sorted(set(duplicates))))

        missing_metadata = sorted(REQUIRED_METADATA - names)
        if missing_metadata:
            raise RuntimeError("missing metadata: " + ", ".join(missing_metadata))

        package_index = json.loads(archive.read("metadata/package-index.json"))

        file_manifest = json.loads(archive.read("metadata/file-manifest.json"))
        if not isinstance(file_manifest, list):
            raise RuntimeError("file-manifest must be an array")
        for item in file_manifest:
            if not isinstance(item, dict):
                raise RuntimeError("file-manifest contains a non-object entry")
            path = item.get("path")
            if not isinstance(path, str) or path not in names:
                raise RuntimeError(f"file-manifest entry missing from package: {path}")
            actual_size = archive.getinfo(path).file_size
            expected_size = item.get("size")
            if isinstance(expected_size, int) and actual_size != expected_size:
                raise RuntimeError(
                    f"file-manifest size mismatch: {path} expected={expected_size} actual={actual_size}"
                )
            expected_sha = str(item.get("sha256") or "").lower()
            if expected_sha and sha256_zip_entry(archive, path) != expected_sha:
                raise RuntimeError(f"file-manifest hash mismatch: {path}")

        for item in package_index.get("metadata", []):
            if not isinstance(item, dict):
                continue
            path = item.get("path")
            if not isinstance(path, str) or path not in names:
                raise RuntimeError(f"package-index metadata entry missing: {path}")
            expected_size = item.get("size")
            actual_size = archive.getinfo(path).file_size
            if isinstance(expected_size, int) and expected_size != actual_size:
                raise RuntimeError(
                    f"package-index metadata size mismatch: {path} expected={expected_size} actual={actual_size}"
                )
            expected_sha = str(item.get("sha256") or "").lower()
            if expected_sha and sha256_zip_entry(archive, path) != expected_sha:
                raise RuntimeError(f"package-index metadata hash mismatch: {path}")

        payload_items = package_index.get("payloads", [])
        if not isinstance(payload_items, list):
            raise RuntimeError("package-index payloads must use the Android array schema")
        payloads = {item.get("id"): item for item in payload_items if isinstance(item, dict)}

        sums = parse_sha256sums(archive.read("metadata/SHA256SUMS").decode("utf-8"))
        for payload_id in ("rootfs", "runtime"):
            payload = payloads.get(payload_id)
            if not payload:
                raise RuntimeError(f"missing payload registration: {payload_id}")
            path = payload.get("path")
            if not isinstance(path, str) or path not in names:
                raise RuntimeError(f"payload file missing: {payload_id}")
            actual = sha256_zip_entry(archive, path)
            expected = str(payload.get("sha256") or "").lower()
            sum_expected = sums.get(path) or sums.get("/".join(path.split("/")[:-1]))
            if expected and actual != expected:
                raise RuntimeError(f"payload hash mismatch in package-index: {payload_id}")
            if sum_expected and actual != sum_expected:
                raise RuntimeError(f"payload hash mismatch in SHA256SUMS: {payload_id}")

        component_index = json.loads(archive.read("metadata/component-index.json"))
        components = {
            item.get("id")
            for item in component_index.get("components", [])
            if isinstance(item, dict)
        }
        missing_components = sorted(REQUIRED_COMPONENTS - components)
        if missing_components:
            raise RuntimeError("missing runtime components: " + ", ".join(missing_components))

        rootfs_path = payloads["rootfs"]["path"]
        rootfs_seen: set[str] = set()
        rootfs_errors: list[str] = []
        with tarfile.open(fileobj=archive.open(rootfs_path), mode="r|xz") as rootfs:
            for member in rootfs:
                name = normalize_member_name(member.name)
                if name in REQUIRED_ROOTFS_EXECUTABLES:
                    rootfs_seen.add(name)
                    if not member.isfile():
                        rootfs_errors.append("rootfs executable is not a regular file: " + name)
                    elif not member.mode & 0o111:
                        rootfs_errors.append("rootfs executable is not executable: " + name)
        for required in sorted(REQUIRED_ROOTFS_EXECUTABLES - rootfs_seen):
            rootfs_errors.append("rootfs executable is missing: " + required)
        if rootfs_errors:
            raise RuntimeError("; ".join(rootfs_errors))

        runtime_path = payloads["runtime"]["path"]
        runtime_names: set[str] = set()
        errors: list[str] = []
        with tarfile.open(fileobj=archive.open(runtime_path), mode="r|xz") as runtime:
            for member in runtime:
                name = normalize_member_name(member.name)
                runtime_names.add(name)
                if member.isfile() and name in EXECUTABLE_FILES:
                    if name in ELF_BINARIES:
                        source = runtime.extractfile(member)
                        header = source.read(4) if source else b""
                        if header != b"\x7fELF":
                            errors.append("invalid ELF binary: " + name)
                    if not member.mode & 0o111:
                        errors.append("required file is not executable: " + name)
                elif member.isdir() and "/" not in name.rstrip("/") and name in FORBIDDEN_RUNTIME_DIRS:
                    errors.append("runtime root contains mutable directory: " + name)

        missing_runtime_files = sorted(REQUIRED_RUNTIME_FILES - runtime_names)
        if missing_runtime_files:
            errors.append("missing runtime files: " + ", ".join(missing_runtime_files))
        if errors:
            raise RuntimeError("; ".join(errors))

        return {
            "packageSha256": package_hash,
            "runtimeVersion": package_index.get("runtimeVersion"),
            "requiredFiles": sorted(REQUIRED_RUNTIME_FILES),
            "components": sorted(str(item) for item in components if item),
        }


def main() -> None:
    parser = argparse.ArgumentParser()
    parser.add_argument("--package", required=True)
    args = parser.parse_args()
    result = validate(args.package)
    print(json.dumps(result, indent=2, ensure_ascii=False))


if __name__ == "__main__":
    try:
        main()
    except Exception as error:
        print(f"Android embedded runtime validation failed: {error}", file=sys.stderr)
        sys.exit(1)
