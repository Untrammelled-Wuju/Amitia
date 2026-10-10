import hashlib
import io
import json
import pathlib
import posixpath
import re
import struct
import sys
import tarfile
import zipfile


CORE_PATH = "/opt/amitia/core/AmitiaCore"
NODE_PATH = "/opt/amitia/node/bin/node"
QDRANT_PATH = "/opt/amitia/qdrant/qdrant"


def digest(data):
    return hashlib.sha256(data).hexdigest()


def verify_input(path, expected):
    if not re.fullmatch(r"[a-fA-F0-9]{64}", expected):
        raise ValueError("explicit SHA256 required")
    data = pathlib.Path(path).read_bytes()
    if digest(data) != expected.lower():
        raise ValueError("runtime input SHA256 mismatch")


def verify_elf(data, musl=False):
    if len(data) < 64 or data[:6] != b"\x7fELF\x02\x01" or struct.unpack_from("<H", data, 18)[0] != 183:
        raise ValueError("required Linux ELF64 little endian ARM64 binary")
    if struct.unpack_from("<H", data, 16)[0] not in (2, 3) or struct.unpack_from("<I", data, 20)[0] != 1 or struct.unpack_from("<H", data, 52)[0] != 64:
        raise ValueError("invalid ELF executable header")
    offset = struct.unpack_from("<Q", data, 32)[0]
    entry_size, count = struct.unpack_from("<HH", data, 54)
    if offset < 64 or entry_size < 56 or not 1 <= count <= 256 or offset + entry_size * count > len(data):
        raise ValueError("invalid ELF program headers")
    executable_load = False
    for index in range(count):
        header = offset + index * entry_size
        kind, flags = struct.unpack_from("<II", data, header)
        start = struct.unpack_from("<Q", data, header + 8)[0]
        size = struct.unpack_from("<Q", data, header + 32)[0]
        memory = struct.unpack_from("<Q", data, header + 40)[0]
        if start + size > len(data) or kind == 1 and memory < size:
            raise ValueError("ELF segment outside binary")
        executable_load |= kind == 1 and bool(flags & 1) and size > 0
    if not executable_load:
        raise ValueError("ELF executable load segment missing")
    if musl:
        offset = struct.unpack_from("<Q", data, 32)[0]
        entry_size, count = struct.unpack_from("<HH", data, 54)
        if entry_size < 56 or count > 256 or offset + entry_size * count > len(data):
            raise ValueError("invalid ELF program headers")
        interpreters = []
        for index in range(count):
            header = offset + index * entry_size
            if struct.unpack_from("<I", data, header)[0] == 3:
                start = struct.unpack_from("<Q", data, header + 8)[0]
                size = struct.unpack_from("<Q", data, header + 32)[0]
                if size > 256 or start + size > len(data):
                    raise ValueError("invalid ELF interpreter")
                interpreters.append(data[start:start + size].rstrip(b"\0"))
        if interpreters != [b"/lib/ld-musl-aarch64.so.1"]:
            raise ValueError("Node must use Alpine ARM64 musl interpreter")


def elf_dependencies(data):
    verify_elf(data)
    offset = struct.unpack_from("<Q", data, 32)[0]
    entry_size, count = struct.unpack_from("<HH", data, 54)
    if entry_size < 56 or count > 256 or offset + entry_size * count > len(data):
        raise ValueError("invalid ELF program headers")
    loads, dynamic = [], None
    for index in range(count):
        header = offset + index * entry_size
        kind = struct.unpack_from("<I", data, header)[0]
        start, address = struct.unpack_from("<QQ", data, header + 8)
        size = struct.unpack_from("<Q", data, header + 32)[0]
        if start + size > len(data):
            raise ValueError("ELF segment outside binary")
        if kind == 1:
            loads.append((address, address + size, start))
        elif kind == 2:
            dynamic = (start, size)
        elif kind == 3:
            interpreter = data[start:start + size].rstrip(b"\0")
            if interpreter != b"/lib/ld-musl-aarch64.so.1":
                raise ValueError("ARM64 runtime requires static or musl ELF")
    if dynamic is None:
        return []
    strings, needed = None, []
    for cursor in range(dynamic[0], dynamic[0] + dynamic[1] - 15, 16):
        kind, value = struct.unpack_from("<qQ", data, cursor)
        if kind == 0:
            break
        if kind == 5:
            strings = value
        elif kind == 1:
            needed.append(value)
    if not needed:
        return []
    table = next((start + strings - begin for begin, end, start in loads if strings is not None and begin <= strings < end), None)
    if table is None:
        raise ValueError("ELF dependency string table missing")
    result = []
    for index in needed:
        start = table + index
        end = data.find(b"\0", start, start + 256)
        if start >= len(data) or end < 0:
            raise ValueError("invalid ELF dependency")
        name = data[start:end].decode("ascii")
        if "/" in name or not name:
            raise ValueError("invalid ELF dependency name")
        result.append(name)
    return result


def source_manifest(packages_path, backend_path, output_path):
    decoder = json.JSONDecoder()
    remaining = pathlib.Path(packages_path).read_text(encoding="utf-8")
    entries = {}
    while remaining.strip():
        package, length = decoder.raw_decode(remaining.lstrip())
        remaining = remaining.lstrip()[length:]
        if package.get("Error") or package.get("DepsErrors"):
            raise ValueError("Go source dependency discovery failed")
        directory = pathlib.Path(package["Dir"])
        for category in ("GoFiles", "CgoFiles", "CFiles", "HFiles", "SFiles", "SysoFiles", "EmbedFiles"):
            for name in package.get(category, []):
                path = directory / name
                data = path.read_bytes()
                key = package["ImportPath"] + "/" + name
                entries[key] = {"path": key, "sha256": digest(data), "size": len(data)}
    for name in ("go.mod", "go.sum"):
        data = (pathlib.Path(backend_path) / name).read_bytes()
        entries[name] = {"path": name, "sha256": digest(data), "size": len(data)}
    encoded = json.dumps({"schemaVersion": 1, "goos": "linux", "goarch": "arm64", "inputs": sorted(entries.values(), key=lambda item: item["path"])}, sort_keys=True, separators=(",", ":")).encode()
    pathlib.Path(output_path).write_bytes(encoded + b"\n")


def safe_path(name):
    path = posixpath.normpath(name)
    if name.startswith("/") or path == ".." or path.startswith("../") or "\x00" in name:
        raise ValueError("unsafe archive path")
    return "" if path == "." else path


def add_bytes(archive, name, data, mode=0o644, uid=0):
    info = tarfile.TarInfo(name)
    info.size, info.mode, info.uid, info.gid = len(data), mode, uid, uid
    archive.addfile(info, io.BytesIO(data))


def prepare(alpine_path, node_path, core_path, sources_path, version, output_path, manifest_path, qdrant_path):
    core = pathlib.Path(core_path).read_bytes()
    verify_elf(core)
    node_files = []
    node_binary = None
    with tarfile.open(node_path) as archive:
        members = archive.getmembers()
        roots = {safe_path(item.name).split("/")[0] for item in members if safe_path(item.name)}
        if len(roots) != 1:
            raise ValueError("Node archive requires one root directory")
        root = next(iter(roots))
        for item in members:
            name = safe_path(item.name)
            if name == root:
                continue
            relative = name[len(root) + 1:]
            item.name = "opt/amitia/node/" + relative
            if item.issym():
                safe_path(posixpath.join(posixpath.dirname(relative), item.linkname))
                if item.linkname.startswith("/"):
                    raise ValueError("absolute Node symlink")
            if item.islnk():
                link = safe_path(item.linkname)
                if not link.startswith(root + "/"):
                    raise ValueError("Node hardlink outside archive")
                item.linkname = "opt/amitia/node/" + link[len(root) + 1:]
            if not (item.isfile() or item.isdir() or item.issym() or item.islnk()):
                raise ValueError("unsupported Node archive entry")
            data = archive.extractfile(item).read() if item.isfile() else None
            if relative.startswith("lib/") and ".so" in posixpath.basename(relative) and item.isfile():
                verify_elf(data)
                item.name = "usr/lib/" + posixpath.basename(relative)
            item.uid = item.gid = 0
            if relative == "bin/node" and item.isfile():
                node_binary = data
                item.mode = 0o755
            node_files.append((item, data))
    if node_binary is None:
        raise ValueError("Node bin/node missing")
    verify_elf(node_binary, musl=True)
    qdrant_files, qdrant_binary = [], None
    with tarfile.open(qdrant_path) as archive:
        for item in archive:
            name = safe_path(item.name)
            if not (item.isfile() or item.isdir()):
                raise ValueError("Qdrant archive permits files/directories only")
            data = archive.extractfile(item).read() if item.isfile() else None
            if name.endswith("/qdrant") or name == "qdrant":
                if qdrant_binary is not None:
                    raise ValueError("multiple Qdrant binaries")
                qdrant_binary = data
                item.name, item.mode = QDRANT_PATH[1:], 0o755
            elif item.isfile() and ".so" in posixpath.basename(name):
                item.name = "usr/lib/" + posixpath.basename(name)
                verify_elf(data)
            elif item.isfile():
                raise ValueError("unexpected Qdrant archive file")
            else:
                continue
            item.uid = item.gid = 0
            qdrant_files.append((item, data))
    if qdrant_binary is None:
        raise ValueError("Qdrant binary missing")
    verify_elf(qdrant_binary)
    replacements = {"etc/apk/repositories", "etc/passwd", "etc/group"}
    fixed = {"var/lib/amitia": (0, 0o700), "home/amitia": (1000, 0o750), "home/amitia/workspace": (1000, 0o750)}
    for name in ("config", "data", "cache", "log", "run", "temp", "workspace", "home"):
        fixed["var/lib/amitia/" + name] = (0, 0o700)
    for name in ("var", "var/lib", "home", "dev", "dev/pts", "proc", "sys", "run", "root", "opt", "opt/amitia", "opt/amitia/core", "opt/amitia/qdrant"):
        fixed.setdefault(name, (0, 0o755))
    fixed["tmp"] = (0, 0o1777)
    text = {}
    with tarfile.open(alpine_path) as archive:
        libraries = {}
        stored_libraries = {}
        for item in archive:
            name = safe_path(item.name)
            if name.startswith(("lib/", "usr/lib/")) and ".so" in posixpath.basename(name):
                if item.isfile():
                    libraries[name] = archive.extractfile(item).read()
                    stored_libraries[name] = libraries[name]
                elif item.issym() or item.islnk():
                    target = item.linkname.lstrip("/") if item.linkname.startswith("/") or item.islnk() else posixpath.join(posixpath.dirname(name), item.linkname)
                    libraries[name] = safe_path(target)
                    stored_libraries[name] = item.linkname.encode("utf-8") if item.issym() else None
    for item, data in node_files + qdrant_files:
        if item.name.startswith(("lib/", "usr/lib/")) and ".so" in posixpath.basename(item.name):
            if item.name in libraries and libraries[item.name] != data:
                raise ValueError("runtime library would overwrite Alpine library")
            libraries[item.name] = data
            stored_libraries[item.name] = data
    def library_bytes(path, visited=None):
        visited = set() if visited is None else visited
        if path in visited or len(visited) > 40 or path not in libraries:
            raise ValueError("runtime library missing or invalid link: " + path)
        visited.add(path)
        value = libraries[path]
        return library_bytes(value, visited) if isinstance(value, str) else value
    verify_elf(library_bytes("lib/ld-musl-aarch64.so.1"))
    checked = set()
    def check_dependencies(binary):
        for name in elf_dependencies(binary):
            if name in checked:
                continue
            path = next((prefix + name for prefix in ("lib/", "usr/lib/") if prefix + name in libraries), None)
            if path is None:
                raise ValueError("runtime ELF dependency missing: " + name)
            checked.add(name)
            dependency = library_bytes(path)
            verify_elf(dependency)
            check_dependencies(dependency)
    for _, binary in [(None, node_binary), (None, qdrant_binary)] + [(item, data) for item, data in node_files + qdrant_files if item.isfile() and data and data[:4] == b"\x7fELF"]:
        check_dependencies(binary)
    with tarfile.open(alpine_path) as original, tarfile.open(output_path, "w:gz") as archive:
        seen = set()
        for item in original:
            name = safe_path(item.name)
            if name in seen:
                raise ValueError("duplicate Alpine archive path")
            seen.add(name)
            if name in replacements:
                text[name] = original.extractfile(item).read().decode("utf-8")
                continue
            if name in fixed:
                continue
            if name.startswith("opt/amitia/"):
                raise ValueError("Alpine archive contains reserved runtime path")
            archive.addfile(item, original.extractfile(item) if item.isfile() else None)
        for name, (uid, mode) in fixed.items():
            info = tarfile.TarInfo(name)
            info.type, info.uid, info.gid, info.mode = tarfile.DIRTYPE, uid, uid, mode
            archive.addfile(info)
        passwd = text.get("etc/passwd", "root:x:0:0:root:/root:/bin/sh\n")
        group = text.get("etc/group", "root:x:0:\n")
        if any(line.split(":")[0] == "amitia" or len(line.split(":")) > 2 and line.split(":")[2] == "1000" for line in passwd.splitlines()):
            raise ValueError("Alpine user 1000 already allocated")
        add_bytes(archive, "etc/passwd", (passwd.rstrip("\n") + "\namitia:x:1000:1000:Amitia:/home/amitia:/bin/sh\n").encode())
        add_bytes(archive, "etc/group", (group.rstrip("\n") + "\namitia:x:1000:\n").encode())
        release = ".".join(version.split(".")[:2])
        add_bytes(archive, "etc/apk/repositories", (f"https://dl-cdn.alpinelinux.org/alpine/v{release}/main\nhttps://dl-cdn.alpinelinux.org/alpine/v{release}/community\n").encode())
        add_bytes(archive, CORE_PATH[1:], core, 0o755)
        for item, data in node_files:
            archive.addfile(item, io.BytesIO(data) if data is not None else None)
        for item, data in qdrant_files:
            archive.addfile(item, io.BytesIO(data))
    source_data = pathlib.Path(sources_path).read_bytes()
    manifest = {"schemaVersion": 1, "format": "ish_fakefs", "formatVersion": "1", "distribution": "alpine", "version": version, "architecture": "aarch64", "guestArch": "arm64", "sourceType": "bundled", "core": {"path": CORE_PATH, "sha256": digest(core), "size": len(core)}, "node": {"path": NODE_PATH, "sha256": digest(node_binary), "size": len(node_binary)}, "sourceManifest": {"file": "core-source-inputs.json", "sha256": digest(source_data)}}
    manifest["qdrant"] = {"path": QDRANT_PATH, "sha256": digest(qdrant_binary), "size": len(qdrant_binary)}
    manifest["runtimeInputs"] = {"alpineSha256": digest(pathlib.Path(alpine_path).read_bytes()), "nodeArchiveSha256": digest(pathlib.Path(node_path).read_bytes()), "qdrantArchiveSha256": digest(pathlib.Path(qdrant_path).read_bytes())}
    manifest["sourceManifest"]["size"] = len(source_data)
    manifest["runtimeLibraries"] = [{"path": "/" + path, "sha256": digest(library_bytes(path)), "size": len(library_bytes(path)), "hostStoredPath": "/" + path, "hostStoredSha256": digest(stored_libraries[path] if stored_libraries[path] is not None else library_bytes(path)), "hostStoredSize": len(stored_libraries[path] if stored_libraries[path] is not None else library_bytes(path))} for path in sorted(libraries)]
    pathlib.Path(manifest_path).write_text(json.dumps(manifest, indent=2) + "\n", encoding="utf-8")


def package(fakefs_path, manifest_path, sources_path, output_path, release_path):
    root = pathlib.Path(fakefs_path)
    if not (root / "data").is_dir() or not (root / "meta.db").is_file():
        raise ValueError("fakefs output incomplete")
    files = sorted(path for path in root.rglob("*") if path.is_file() or path.is_dir())
    temporary = pathlib.Path(output_path + ".pending")
    with zipfile.ZipFile(temporary, "w", zipfile.ZIP_DEFLATED, compresslevel=9) as archive:
        for path in files:
            if path.is_symlink():
                raise ValueError("fakefs output unexpectedly contains host symlink")
            archive.write(path, path.relative_to(root).as_posix())
        archive.write(manifest_path, "rootfs.manifest.json")
        archive.write(sources_path, "core-source-inputs.json")
    data = temporary.read_bytes()
    manifest = json.loads(pathlib.Path(manifest_path).read_text(encoding="utf-8"))
    manifest.update({"asset": pathlib.Path(output_path).name, "sha256": digest(data), "packageSha256": digest(data), "size": len(data)})
    temporary.replace(output_path)
    pathlib.Path(release_path).write_text(json.dumps(manifest, indent=2) + "\n", encoding="utf-8")


def main():
    action, *args = sys.argv[1:]
    if action == "verify-elf":
        verify_elf(pathlib.Path(args[0]).read_bytes())
    elif action == "sources":
        source_manifest(*args)
    elif action == "verify-input":
        verify_input(*args)
    elif action == "prepare":
        prepare(*args)
    elif action == "package":
        package(*args)
    else:
        raise ValueError("unknown action")


if __name__ == "__main__":
    main()
