import argparse
import hashlib
import io
import json
import shutil
import subprocess
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


def write_json(path, value):
    path.parent.mkdir(parents=True, exist_ok=True)
    path.write_text(json.dumps(value, ensure_ascii=False, indent=2) + "\n", encoding="utf-8")


def copy(source, destination):
    destination.parent.mkdir(parents=True, exist_ok=True)
    shutil.copy2(source, destination)


def extract_binary(archive, member_name, destination):
    with tarfile.open(archive, "r:gz") as source:
        member = source.getmember(member_name)
        stream = source.extractfile(member)
        if stream is None:
            raise ValueError(f"missing binary: {member_name}")
        destination.parent.mkdir(parents=True, exist_ok=True)
        with destination.open("wb") as target:
            shutil.copyfileobj(stream, target)


def extract_node(archive, destination):
    prefix = "node-v26.12.1-linux-arm64/"
    with tarfile.open(archive, "r:xz") as source:
        for member in source:
            name = member.name
            if not name.startswith(prefix) or not member.isfile():
                continue
            relative = name[len(prefix):]
            if relative != "bin/node" and not relative.startswith("lib/node_modules/npm/"):
                continue
            stream = source.extractfile(member)
            if stream is None:
                continue
            target = destination / relative
            target.parent.mkdir(parents=True, exist_ok=True)
            with target.open("wb") as output:
                shutil.copyfileobj(stream, output)
    for name, cli in (("npm", "npm-cli.js"), ("npx", "npx-cli.js")):
        wrapper = destination / "bin" / name
        wrapper.write_text(f'#!/bin/sh\nexec "$(dirname "$0")/node" "$(dirname "$0")/../lib/node_modules/npm/bin/{cli}" "$@"\n', encoding="utf-8")


def read_ar_member(archive, member_name, destination):
    with archive.open("rb") as source:
        if source.read(8) != b"!<arch>\n":
            raise ValueError(f"invalid Debian archive: {archive}")
        while True:
            header = source.read(60)
            if not header:
                break
            if len(header) != 60 or header[58:60] != b"`\n":
                raise ValueError(f"invalid ar member header: {archive}")
            name = header[0:16].decode("ascii").strip().rstrip("/")
            size = int(header[48:58].decode("ascii").strip())
            offset = source.tell()
            if name == member_name:
                destination.parent.mkdir(parents=True, exist_ok=True)
                with destination.open("wb") as output:
                    remaining = size
                    while remaining:
                        chunk = source.read(min(remaining, 1024 * 1024))
                        if not chunk:
                            raise ValueError(f"truncated ar member: {member_name}")
                        output.write(chunk)
                        remaining -= len(chunk)
                return
            source.seek(offset + size + (size % 2))
    raise ValueError(f"missing ar member: {member_name}")


def verify_elf(path):
    with path.open("rb") as stream:
        if stream.read(4) != b"\x7fELF":
            raise ValueError(f"invalid ELF binary: {path}")


def compress_rootfs(source, destination, library_archive=None, node_executable=None):
    required_dirs = (
        "opt/amitia",
        "etc/amitia",
        "etc/ssl/certs",
        "var/lib/amitia",
        "var/cache/amitia",
        "var/log/amitia",
        "run/amitia",
        "home/amitia",
    )
    capture = tempfile.TemporaryDirectory(prefix="amitia-rootfs-library-")
    library_tar = None
    try:
        if library_archive is not None:
            if node_executable is None:
                raise ValueError("node executable is required for the library archive")
            compressed = Path(capture.name) / "data.tar.zst"
            decompressed = Path(capture.name) / "data.tar"
            read_ar_member(library_archive, "data.tar.zst", compressed)
            subprocess.run([
                str(node_executable),
                "-e",
                "const fs=require('fs'),z=require('zlib');fs.writeFileSync(process.argv[2],z.zstdDecompressSync(fs.readFileSync(process.argv[1])))",
                str(compressed),
                str(decompressed),
            ], check=True, capture_output=True)
            library_tar = tarfile.open(decompressed, "r:")

        with tarfile.open(source, "r:gz") as archive, tarfile.open(destination, "w:xz", preset=3) as output:
            existing = set()
            for member in archive:
                existing.add(member.name.rstrip("/"))
                stream = archive.extractfile(member) if member.isfile() else None
                output.addfile(member, stream)
            for name in required_dirs:
                if name not in existing:
                    member = tarfile.TarInfo(name)
                    member.type = tarfile.DIRTYPE
                    member.mode = 0o755
                    output.addfile(member)
            if "etc/ssl/certs/ca-certificates.crt" not in existing:
                member = tarfile.TarInfo("etc/ssl/certs/ca-certificates.crt")
                member.mode = 0o644
                member.size = 0
                output.addfile(member, io.BytesIO())
            if library_tar is not None:
                required_library_members = {
                    "usr/lib/aarch64-linux-gnu/libatomic.so.1",
                    "usr/lib/aarch64-linux-gnu/libatomic.so.1.2.0",
                }
                added = set()
                for member in library_tar:
                    name = member.name[2:] if member.name.startswith("./") else member.name
                    if name not in required_library_members:
                        continue
                    if name in existing:
                        continue
                    info = tarfile.TarInfo(name)
                    info.mode = member.mode
                    info.type = member.type
                    info.linkname = member.linkname
                    if member.isfile():
                        data = library_tar.extractfile(member).read()
                        info.size = len(data)
                        output.addfile(info, io.BytesIO(data))
                    else:
                        output.addfile(info)
                    added.add(name)
                missing = required_library_members - existing - added
                if missing:
                    raise ValueError("missing rootfs libraries: " + ", ".join(sorted(missing)))
    finally:
        if library_tar is not None:
            library_tar.close()
        capture.cleanup()


def create_runtime_tar(root, destination):
    executable = {
        "backend/amitia-server",
        "node/bin/node",
        "node/bin/npm",
        "node/bin/npx",
        "qdrant/bin/qdrant",
        "surrealdb/surreal",
        "scripts/node/amitia-node-prepare.sh",
        "scripts/node/amitia-node-probe.sh",
    }
    with tarfile.open(destination, "w:xz", preset=3) as archive:
        for path in sorted(root.rglob("*")):
            if not path.is_file():
                continue
            name = path.relative_to(root).as_posix()
            info = archive.gettarinfo(str(path), arcname=name)
            info.mode = 0o755 if name in executable else 0o644
            with path.open("rb") as stream:
                archive.addfile(info, stream)


def make_component(name, root, entry, version, path):
    return {
        "id": f"runtime.{name}",
        "root": root,
        "entry": entry,
        "version": version,
        "architecture": "arm64",
        "sha256": digest(path),
        "source": "package",
    }


def main():
    parser = argparse.ArgumentParser()
    parser.add_argument("--project-root", required=True)
    parser.add_argument("--backend", required=True)
    parser.add_argument("--node-archive", required=True)
    parser.add_argument("--rootfs-archive", required=True)
    parser.add_argument("--libatomic-deb", required=True)
    parser.add_argument("--node-executable", required=True)
    parser.add_argument("--qdrant-archive", required=True)
    parser.add_argument("--surrealdb", required=True)
    parser.add_argument("--source-commit", required=True)
    parser.add_argument("--output", required=True)
    args = parser.parse_args()
    project = Path(args.project_root).resolve()
    output = Path(args.output).resolve()
    if not output.is_relative_to(project):
        raise ValueError("output must be inside the project")
    commit = args.source_commit.strip().lower()
    if len(commit) != 40 or any(char not in "0123456789abcdef" for char in commit):
        raise ValueError("source commit must be a 40-character hex revision")
    node_archive = Path(args.node_archive).resolve()
    rootfs_archive = Path(args.rootfs_archive).resolve()
    libatomic_deb = Path(args.libatomic_deb).resolve()
    node_executable = Path(args.node_executable).resolve()
    node_lock = json.loads((project / "runtime/artifacts/node/linux-arm64/node-runtime-lock.json").read_text(encoding="utf-8"))
    rootfs_lock = json.loads((project / "runtime/artifacts/ubuntu-rootfs/linux-arm64/ubuntu-rootfs-lock.json").read_text(encoding="utf-8"))
    libatomic_lock = json.loads((project / "runtime/artifacts/ubuntu-rootfs/linux-arm64/libatomic1-lock.json").read_text(encoding="utf-8"))
    if digest(node_archive) != node_lock["sha256"] or digest(rootfs_archive) != rootfs_lock["sha256"] or digest(libatomic_deb) != libatomic_lock["sha256"]:
        raise ValueError("locked Node, rootfs or libatomic archive hash mismatch")
    if not node_executable.is_file():
        raise ValueError("Node executable is unavailable")
    output.parent.mkdir(parents=True, exist_ok=True)
    with tempfile.TemporaryDirectory(prefix="amitia-runtime-fresh-", dir=output.parent) as temporary:
        stage = Path(temporary)
        program = stage / "program"
        copy(Path(args.backend), program / "backend/amitia-server")
        extract_node(node_archive, program / "node")
        extract_binary(Path(args.qdrant_archive), "qdrant", program / "qdrant/bin/qdrant")
        copy(Path(args.surrealdb), program / "surrealdb/surreal")
        for host in ("plugin-host", "task-host"):
            shutil.copytree(project / "runtime" / host / "dist", program / host / "dist")
            copy(project / "runtime" / host / "package.json", program / host / "package.json")
        for script in ("amitia-node-prepare.sh", "amitia-node-probe.sh"):
            copy(project / "runtime/scripts/node" / script, program / "scripts/node" / script)
        for contract in ("guest-layout.json", "mount-contract.json"):
            copy(project / "runtime/contracts" / contract, program / "manifest" / contract)
        for path in (program / "backend/amitia-server", program / "node/bin/node", program / "qdrant/bin/qdrant", program / "surrealdb/surreal"):
            verify_elf(path)

        payload = stage / "payload"
        payload.mkdir()
        rootfs_tar = payload / "rootfs.tar.xz"
        runtime_tar = payload / "runtime.tar.xz"
        compress_rootfs(rootfs_archive, rootfs_tar, libatomic_deb, node_executable)
        create_runtime_tar(program, runtime_tar)

        metadata = stage / "metadata"
        copy(project / "runtime/contracts/guest-layout.json", metadata / "guest-layout.json")
        copy(project / "runtime/contracts/mount-contract.json", metadata / "mount-contract.json")
        copy(project / "THIRD_PARTY_NOTICES.md", stage / "licenses/THIRD_PARTY_NOTICES.md")
        components = [
            make_component("backend", "backend", "backend/amitia-server", "dev", program / "backend/amitia-server"),
            make_component("node", "node", "node/bin/node", node_lock["version"], program / "node/bin/node"),
            make_component("qdrant", "qdrant", "qdrant/bin/qdrant", "1.19.0", program / "qdrant/bin/qdrant"),
            make_component("plugin-host", "plugin-host", "plugin-host/dist/index.js", "1.0.0", program / "plugin-host/dist/index.js"),
            make_component("task-host", "task-host", "task-host/dist/index.js", "1.0.0", program / "task-host/dist/index.js"),
            make_component("surrealdb", "surrealdb", "surrealdb/surreal", "2.3.8", program / "surrealdb/surreal"),
        ]
        write_json(metadata / "component-index.json", {"components": components})
        package_id = "amitia.runtime.android"
        write_json(metadata / "component-lock.json", {
            "runtimeVersion": "1.0.0",
            "packageId": package_id,
            "components": {component["root"]: {"componentId": component["id"], "version": component["version"], "sha256": component["sha256"]} for component in components},
        })
        files = [rootfs_tar, runtime_tar, metadata / "component-index.json", metadata / "component-lock.json", metadata / "guest-layout.json", metadata / "mount-contract.json", stage / "licenses/THIRD_PARTY_NOTICES.md"]
        write_json(metadata / "file-manifest.json", [
            {"path": path.relative_to(stage).as_posix(), "size": path.stat().st_size, "sha256": digest(path)} for path in files
        ])
        files.append(metadata / "file-manifest.json")
        (metadata / "SHA256SUMS").write_text(
            "".join(f"{digest(path)}  {path.relative_to(stage).as_posix()}\n" for path in files), encoding="utf-8"
        )
        payloads = [{"id": name, "role": name, "path": f"payload/{name}.tar.xz", "sha256": digest(path), "size": path.stat().st_size} for name, path in (("rootfs", rootfs_tar), ("runtime", runtime_tar))]
        meta_refs = [{"role": role, "path": f"metadata/{name}", "sha256": digest(metadata / name), "size": (metadata / name).stat().st_size} for role, name in (("guest-layout", "guest-layout.json"), ("mount-contract", "mount-contract.json"), ("sha256sums", "SHA256SUMS"))]
        write_json(metadata / "package-index.json", {
            "schemaVersion": 1,
            "packageFormatVersion": 1,
            "runtimeVersion": "1.0.0",
            "packageId": package_id,
            "sourceRevision": commit,
            "sourceCommit": commit,
            "target": {"hostPlatform": "android", "hostAbi": "arm64-v8a", "runtimeKind": "embedded-proot", "guestPlatform": "linux", "guestArchitecture": "arm64"},
            "payloads": payloads,
            "metadata": meta_refs,
            "components": components,
        })
        with zipfile.ZipFile(output, "w", compression=zipfile.ZIP_STORED, allowZip64=True) as archive:
            for path in sorted(stage.rglob("*")):
                if path.is_file() and not path.is_relative_to(program):
                    archive.write(path, path.relative_to(stage).as_posix())
    print(json.dumps({"output": str(output), "sha256": digest(output), "backendSha256": digest(Path(args.backend))}))


if __name__ == "__main__":
    main()
