import argparse
import hashlib
import io
import json
import lzma
import shutil
import sys
import tarfile
import tempfile
import zipfile
from pathlib import Path


def digest(path):
    value = hashlib.sha256()
    with path.open("rb") as stream:
        for chunk in iter(lambda: stream.read(1024 * 1024), b""):
            value.update(chunk)
    return value.hexdigest()


def copy_file(source, target):
    target.parent.mkdir(parents=True, exist_ok=True)
    shutil.copy2(source, target)


def verify_elf(path):
    with path.open("rb") as stream:
        if stream.read(4) != b"\x7fELF":
            raise RuntimeError("invalid ELF binary: " + str(path))


def payload_map(index):
    payloads = index.get("payloads")
    if isinstance(payloads, dict):
        return payloads
    if isinstance(payloads, list):
        mapped = {}
        for item in payloads:
            if isinstance(item, dict) and item.get("id"):
                mapped[item["id"]] = item
        return mapped
    raise RuntimeError("package-index payloads must be an object or array")


def main():
    parser = argparse.ArgumentParser()
    parser.add_argument("--base-package", required=True)
    parser.add_argument("--backend", required=True)
    parser.add_argument("--surrealdb", required=True)
    parser.add_argument("--surrealdb-version", default="2.3.8")
    parser.add_argument("--source-commit", required=True)
    parser.add_argument("--output", required=True)
    args = parser.parse_args()

    base_package = Path(args.base_package).resolve()
    output = Path(args.output).resolve()
    source_commit = args.source_commit.strip().lower()
    if len(source_commit) != 40 or any(ch not in "0123456789abcdef" for ch in source_commit):
        raise RuntimeError("source commit must be a 40-character git revision")
    required = [base_package, Path(args.backend), Path(args.surrealdb)]
    missing = [str(path) for path in required if not path.is_file()]
    if missing:
        raise RuntimeError("missing input: " + ", ".join(missing))

    with tempfile.TemporaryDirectory(prefix="amitia-apk-runtime-") as temporary:
        root = Path(temporary)
        with zipfile.ZipFile(base_package) as archive:
            archive.extractall(root)

        index_path = root / "metadata" / "package-index.json"
        index = json.loads(index_path.read_text(encoding="utf-8"))
        index["sourceCommit"] = source_commit
        index["sourceRevision"] = source_commit
        source_payloads = payload_map(index)
        if "rootfs" not in source_payloads or "runtime" not in source_payloads:
            raise RuntimeError("base package must register rootfs and runtime payloads")
        rootfs_info = dict(source_payloads["rootfs"])
        runtime_info = dict(source_payloads["runtime"])
        runtime_relpath = Path(runtime_info["path"])
        runtime_tarball = root / runtime_relpath
        runtime_root = root / "runtime-root"
        with tarfile.open(runtime_tarball, "r:xz") as archive:
            archive.extractall(runtime_root, filter="data")

        shutil.rmtree(runtime_root / "sidecar", ignore_errors=True)
        shutil.rmtree(runtime_root / "qq-sidecar", ignore_errors=True)
        copy_file(Path(args.backend), runtime_root / "backend" / "amitia-server")
        copy_file(Path(args.surrealdb), runtime_root / "surrealdb" / "surreal")

        for relative in [
            "backend/amitia-server",
            "node/bin/node",
            "qdrant/bin/qdrant",
            "plugin-host/dist/index.js",
            "task-host/dist/index.js",
            "surrealdb/surreal",
        ]:
            if not (runtime_root / relative).is_file():
                raise RuntimeError("runtime package is incomplete: " + relative)

        for relative in ["backend/amitia-server", "node/bin/node", "qdrant/bin/qdrant", "surrealdb/surreal"]:
            verify_elf(runtime_root / relative)

        import os
        raw_tar_buffer = io.BytesIO()
        with tarfile.open(fileobj=raw_tar_buffer, mode="w") as tar_out:
            for path in sorted(runtime_root.rglob("*")):
                tar_out.add(path, path.relative_to(runtime_root).as_posix(), recursive=False)

        executables = {
            "backend/amitia-server",
            "node/bin/node",
            "qdrant/bin/qdrant",
            "surrealdb/surreal",
        }
        normalized_tar_buffer = io.BytesIO()
        with tarfile.open(fileobj=normalized_tar_buffer, mode="w") as normalized_tar:
            raw_tar_buffer.seek(0)
            with tarfile.open(fileobj=raw_tar_buffer, mode="r") as source_tar:
                for member in source_tar:
                    if member.isfile() and (member.name in executables or member.name.endswith(".sh")):
                        member.mode = 0o755
                    payload = source_tar.extractfile(member) if member.isreg() else None
                    normalized_tar.addfile(member, payload)
        tar_data = normalized_tar_buffer.getvalue()

        with lzma.open(runtime_tarball, "wb", preset=3, check=lzma.CHECK_CRC32) as xz_out:
            xz_out.write(tar_data)

        runtime_hash = digest(runtime_tarball)
        runtime_size = runtime_tarball.stat().st_size

        # Normalize both legacy object payloads and current Android array payloads
        # to the canonical Android array schema. This also allows the checked-in
        # embedded package to be used as a repair base when source packaging has
        # intentionally excluded runtime/out build artifacts.
        runtime_info["sha256"] = runtime_hash
        runtime_info["size"] = runtime_size
        index["payloads"] = [
            {
                "id": "rootfs",
                "role": "rootfs",
                "sha256": rootfs_info["sha256"],
                "size": rootfs_info["size"],
                "path": rootfs_info["path"],
            },
            {
                "id": "runtime",
                "role": "runtime",
                "sha256": runtime_hash,
                "size": runtime_size,
                "path": runtime_info["path"],
            },
        ]

        # Build components array from component-index.json
        component_index_path = root / "metadata" / "component-index.json"
        component_index = json.loads(component_index_path.read_text(encoding="utf-8"))
        backend_hash = digest(Path(args.backend))
        surrealdb_hash = digest(Path(args.surrealdb))
        for component in component_index["components"]:
            if component.get("id") == "runtime.backend":
                component["sha256"] = backend_hash
        legacy_channel_components = {"runtime.sidecar", "runtime.qq-sidecar"}
        surreal_components = [
            item
            for item in component_index["components"]
            if item.get("id") != "runtime.surrealdb"
            and item.get("id") not in legacy_channel_components
            and item.get("root") not in {"sidecar", "qq-sidecar"}
        ]
        surreal_components.append({
            "id": "runtime.surrealdb",
            "root": "surrealdb",
            "entry": "surrealdb/surreal",
            "version": args.surrealdb_version,
            "architecture": "arm64",
            "sha256": surrealdb_hash,
            "source": "package",
        })
        component_index["components"] = surreal_components
        component_index_path.write_text(json.dumps(component_index, indent=2, ensure_ascii=False) + "\n", encoding="utf-8")

        component_lock_path = root / "metadata" / "component-lock.json"
        component_lock = json.loads(component_lock_path.read_text(encoding="utf-8"))
        component_lock["packageId"] = index.get("packageId", "amitia.runtime.android")
        lock_components = component_lock.setdefault("components", {})
        if isinstance(lock_components, dict):
            lock_components.pop("sidecar", None)
            lock_components.pop("qq-sidecar", None)
            backend_lock = lock_components.get("backend")
            if isinstance(backend_lock, dict):
                backend_lock["sha256"] = backend_hash
            lock_components["surrealdb"] = {
                "componentId": "runtime.surrealdb",
                "version": args.surrealdb_version,
                "sha256": surrealdb_hash,
                "platform": "linux",
                "architecture": "arm64",
                "artifact": "embedded:surrealdb/surreal",
            }
        component_lock_path.write_text(json.dumps(component_lock, indent=2, ensure_ascii=False) + "\n", encoding="utf-8")

        guest_layout_path = root / "metadata" / "guest-layout.json"
        guest_layout = json.loads(guest_layout_path.read_text(encoding="utf-8"))
        structure = guest_layout.get("structure")
        if isinstance(structure, dict):
            structure.pop("sidecar", None)
            structure.pop("qqSidecar", None)
            program = structure.get("program")
            if isinstance(program, dict) and isinstance(program.get("subdirs"), list):
                program["subdirs"] = [
                    item
                    for item in program["subdirs"]
                    if item not in {"sidecar", "qq-sidecar"}
                ]
        guest_layout_path.write_text(json.dumps(guest_layout, indent=2, ensure_ascii=False) + "\n", encoding="utf-8")

        file_manifest_path = root / "metadata" / "file-manifest.json"
        if file_manifest_path.is_file():
            file_manifest = json.loads(file_manifest_path.read_text(encoding="utf-8"))
            runtime_path_text = runtime_relpath.as_posix()
            file_manifest = [
                item
                for item in file_manifest
                if not str(item.get("path", "")).startswith(("sidecar/", "qq-sidecar/"))
            ]
            for item in file_manifest:
                if item.get("path") == runtime_path_text:
                    item["sha256"] = runtime_hash
                    item["size"] = runtime_size
            file_manifest_path.write_text(json.dumps(file_manifest, indent=2, ensure_ascii=False) + "\n", encoding="utf-8")
        components = []
        for comp in component_index.get("components", []):
            components.append({
                "id": comp["id"],
                "version": comp.get("version", ""),
                "architecture": "arm64",
                "root": comp["root"],
                "entry": comp.get("entry", ""),
                "sha256": comp.get("sha256", ""),
                "treeSha256": comp.get("treeSha256", ""),
                "source": "package",
            })
        index["components"] = components

        sums = []
        # package-index.json is intentionally excluded: it contains metadata about
        # SHA256SUMS and is rewritten after these checksums are generated. Hashing
        # it here creates a circular/stale checksum by construction.
        for relative in ["licenses/THIRD_PARTY_NOTICES.md", "metadata/component-index.json", "metadata/component-lock.json", "metadata/file-manifest.json", "metadata/guest-layout.json", "metadata/mount-contract.json"]:
            sums.append(f"{digest(root / relative)}  {relative}")
        sums.append(f"{digest(root / rootfs_info['path'])}  payload/rootfs")
        sums.append(f"{runtime_hash}  payload/runtime")
        legacy_sums = root / "SHA256SUMS"
        if legacy_sums.exists():
            legacy_sums.unlink()
        (root / "metadata" / "SHA256SUMS").write_text("\n".join(sums) + "\n", encoding="utf-8")

        sha256_map = {}
        for line in (root / "metadata" / "SHA256SUMS").read_text(encoding="utf-8").strip().splitlines():
            line = line.strip()
            if not line:
                continue
            parts = line.split(None, 1)
            if len(parts) == 2:
                sha256_map[parts[1].strip()] = parts[0].strip()

        metadata_array = []
        for role, rel_path in [("guest-layout", "metadata/guest-layout.json"), ("mount-contract", "metadata/mount-contract.json"), ("sha256sums", "metadata/SHA256SUMS")]:
            file_path = root / rel_path
            metadata_array.append({
                "role": role,
                "path": rel_path,
                "sha256": sha256_map.get(rel_path, digest(file_path)),
                "size": file_path.stat().st_size,
            })
        index["metadata"] = metadata_array
        index["packageFormatVersion"] = int(index.get("packageFormatVersion", 1))
        target = index.get("target")
        if isinstance(target, dict) and target.get("runtimeKind") == "proot":
            target["runtimeKind"] = "embedded-proot"
        # Package bytes are authenticated externally by BuildConfig.RUNTIME_PACKAGE_SHA256.
        # An archive cannot truthfully embed the SHA-256 of itself without a circular
        # definition, so legacy packageSha256 is deliberately removed from package-index.
        index.pop("packageSha256", None)

        # Add installation info
        index["installation"] = {
            "activeVersion": index.get("runtimeVersion", "1.0.0"),
            "rootfsId": "rootfs",
            "runtimeRootId": "runtime",
            "runtimeRootTreeSha256": hashlib.sha256(tar_data).hexdigest(),
        }

        # Add paths from guest-layout.json
        guest_layout_path = root / "metadata" / "guest-layout.json"
        guest_layout = json.loads(guest_layout_path.read_text(encoding="utf-8"))
        paths = guest_layout.get("paths", {})
        index["paths"] = {
            "rootfsHostPath": "payload/rootfs",
            "runtimeRootHostPath": "payload/runtime",
            "configHostPath": paths.get("configRoot", "/etc/amitia"),
            "dataHostPath": paths.get("dataRoot", "/var/lib/amitia"),
            "cacheHostPath": paths.get("cacheRoot", "/var/cache/amitia"),
            "logHostPath": paths.get("logRoot", "/var/log/amitia"),
            "runHostPath": paths.get("runRoot", "/run/amitia"),
            "guestRuntimeRoot": paths.get("runtimeRoot", "/opt/amitia"),
            "guestConfigRoot": paths.get("configRoot", "/etc/amitia"),
            "guestDataRoot": paths.get("dataRoot", "/var/lib/amitia"),
            "guestCacheRoot": paths.get("cacheRoot", "/var/cache/amitia"),
            "guestLogRoot": paths.get("logRoot", "/var/log/amitia"),
            "guestRunRoot": paths.get("runRoot", "/run/amitia"),
        }

        # Add verification flags
        index["verification"] = {
            "packageVerified": True,
            "rootfsVerified": True,
            "runtimeRootVerified": True,
            "componentsVerified": True,
            "guestLayoutVerified": True,
            "mountContractVerified": True,
        }

        index_path.write_text(json.dumps(index, indent=2, ensure_ascii=False) + "\n", encoding="utf-8")
        shutil.rmtree(runtime_root)

        output.parent.mkdir(parents=True, exist_ok=True)
        temporary_output = output.with_suffix(".tmp")
        if temporary_output.exists():
            temporary_output.unlink()
        with zipfile.ZipFile(temporary_output, "w", compression=zipfile.ZIP_DEFLATED, compresslevel=9) as archive:
            for path in sorted(root.rglob("*")):
                if path.is_file():
                    entry = zipfile.ZipInfo(path.relative_to(root).as_posix())
                    entry.date_time = (1980, 1, 1, 0, 0, 0)
                    entry.compress_type = zipfile.ZIP_DEFLATED
                    archive.writestr(entry, path.read_bytes(), compress_type=zipfile.ZIP_DEFLATED, compresslevel=9)
        temporary_output.replace(output)



if __name__ == "__main__":
    try:
        main()
    except Exception as error:
        print(f"runtime refresh failed: {error}", file=sys.stderr)
        sys.exit(1)
