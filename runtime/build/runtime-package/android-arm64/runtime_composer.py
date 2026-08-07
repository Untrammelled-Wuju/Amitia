import hashlib
import lzma
import os
import pathlib
import shutil
import tarfile
import tempfile


ALLOWED_RUNTIME_ROOTS = {"backend", "node", "qdrant", "plugin-host", "task-host", "scripts", "manifest", "licenses"}
FORBIDDEN_RUNTIME_TYPE = {"block", "char", "fifo", "socket"}
RSIX_COMPRESSION_LEVEL = 5
ENTRY_FILES_REQUIRED = [
    "backend/amitia-server",
    "node/bin/node",
    "node/lib/node_modules/npm/bin/npm-cli.js",
    "node/lib/node_modules/npm/bin/npx-cli.js",
    "qdrant/bin/qdrant",
    "plugin-host/dist/index.js",
    "task-host/dist/index.js",
    "scripts/node/amitia-node-prepare.sh",
    "scripts/node/amitia-node-probe.sh",
    "scripts/node/amitia-plugin-host.sh",
    "scripts/node/amitia-task-host.sh",
    "manifest/guest-layout.json",
    "manifest/mount-contract.json",
]


def _open_deterministic_xz(filename, mode="wb"):
    if "w" in mode:
        filters = [{"id": lzma.FILTER_LZMA2, "preset": RSIX_COMPRESSION_LEVEL}]
        return lzma.open(filename, mode=mode, format=lzma.FORMAT_XZ, check=lzma.CHECK_SHA256, filters=filters)
    return lzma.open(filename, mode=mode)


def extract_archive_to_dir(archive_path, target_dir):
    with tarfile.open(str(archive_path), "r:*") as tf:
        members = tf.getmembers()
        for m in members:
            if m.type in FORBIDDEN_RUNTIME_TYPE:
                raise RuntimeError(f"禁止的成员类型 {m.type}: {m.name}")
            if m.issym() and m.linkname.startswith("/"):
                raise RuntimeError(f"Runtime禁止绝对Symlink: {m.name} -> {m.linkname}")
        for m in members:
            dest = os.path.join(target_dir, m.name)
            abs_target = os.path.abspath(dest)
            abs_dir = os.path.abspath(target_dir)
            if not abs_target.startswith(abs_dir):
                raise RuntimeError(f"路径穿越: {m.name}")
            if m.isdir():
                os.makedirs(dest, exist_ok=True)
            elif m.isfile():
                os.makedirs(os.path.dirname(dest), exist_ok=True)
                fp = tf.extractfile(m)
                with open(dest, "wb") as outf:
                    outf.write(fp.read())
                os.chmod(dest, m.mode if m.mode else 0o644)
            elif m.issym():
                os.makedirs(os.path.dirname(dest), exist_ok=True)
                if os.path.lexists(dest):
                    os.remove(dest)
                os.symlink(m.linkname, dest)
            elif m.islnk():
                os.makedirs(os.path.dirname(dest), exist_ok=True)
                link_dest = os.path.join(target_dir, m.linkname)
                if os.path.lexists(dest):
                    os.remove(dest)
                os.link(link_dest, dest)


def compose_runtime_root(backend_artifact, node_artifact, node_scripts_artifact,
                        qdrant_artifact, plugin_host_files, task_host_files,
                        manifest_files, output_path):
    work_dir = pathlib.Path(tempfile.mkdtemp(prefix="runtime_composer_"))
    try:
        parts_dir = work_dir / "parts"
        parts_dir.mkdir()
        extract_archive_to_dir(backend_artifact, str(parts_dir / "backend"))
        extract_archive_to_dir(node_artifact, str(parts_dir / "node"))
        extract_archive_to_dir(node_scripts_artifact, str(parts_dir / "scripts"))
        extract_archive_to_dir(qdrant_artifact, str(parts_dir / "qdrant"))
        plugin_dir = parts_dir / "plugin-host"
        plugin_dir.mkdir(parents=True, exist_ok=True)
        for rel, src in plugin_host_files:
            dest = plugin_dir / rel
            dest.parent.mkdir(parents=True, exist_ok=True)
            shutil.copy2(str(src), str(dest))
        task_dir = parts_dir / "task-host"
        task_dir.mkdir(parents=True, exist_ok=True)
        for rel, src in task_host_files:
            dest = task_dir / rel
            dest.parent.mkdir(parents=True, exist_ok=True)
            shutil.copy2(str(src), str(dest))
        manifest_dir = parts_dir / "manifest"
        manifest_dir.mkdir(parents=True, exist_ok=True)
        if manifest_files:
            for rel, src in manifest_files:
                dest = manifest_dir / rel
                dest.parent.mkdir(parents=True, exist_ok=True)
                shutil.copy2(str(src), str(dest))
        all_members = {}
        for root_dir in parts_dir.iterdir():
            if root_dir.is_dir():
                if root_dir.name not in ALLOWED_RUNTIME_ROOTS:
                    raise RuntimeError(f"非法Runtime根目录: {root_dir.name}")
                record_members(all_members, root_dir, parts_dir)
        for rel_path in ENTRY_FILES_REQUIRED:
            if rel_path not in all_members:
                raise RuntimeError(f"Runtime Root缺少入口: {rel_path}")
        runtime_roots = set(all_members[p].owner for p in all_members)
        runtime_roots.discard(None)
        symlink_check(parts_dir, all_members)
        check_no_secrets(parts_dir)
        check_forbidden_runtime_paths(all_members)
        out_path = pathlib.Path(output_path)
        out_path.parent.mkdir(parents=True, exist_ok=True)
        output_tmp = work_dir / out_path.name
        with _open_deterministic_xz(str(output_tmp), "wb") as xz_f:
            with tarfile.open(fileobj=xz_f, mode="w:", format=tarfile.GNU_FORMAT) as tar_out:
                for rel_path in sorted(all_members.keys()):
                    info = all_members[rel_path]
                    src_abs = info.abs_path
                    tar_info = tarfile.TarInfo(name=rel_path)
                    st = src_abs.stat()
                    tar_info.size = st.st_size
                    tar_info.mtime = 0
                    tar_info.uid = 0
                    tar_info.gid = 0
                    tar_info.uname = ""
                    tar_info.gname = ""
                    tar_info.mode = st.st_mode & 0o7777
                    tar_info.pax_headers = {}
                    if src_abs.is_dir():
                        tar_info.type = tarfile.DIRTYPE
                        tar_out.addfile(tar_info)
                    elif src_abs.is_symlink():
                        tar_info.type = tarfile.SYMTYPE
                        tar_info.linkname = os.readlink(str(src_abs))
                        tar_out.addfile(tar_info)
                    else:
                        tar_info.type = tarfile.REGTYPE
                        with open(str(src_abs), "rb") as fp:
                            tar_out.addfile(tar_info, fp)
        sha = hashlib.sha256(output_tmp.read_bytes()).hexdigest()
        shutil.copy2(str(output_tmp), str(out_path))
        return str(out_path), sha, all_members
    finally:
        shutil.rmtree(str(work_dir), ignore_errors=True)


class MemberOwner:
    __slots__ = ("rel", "owner", "abs_path")
    def __init__(self, rel, owner, abs_path):
        self.rel = rel
        self.owner = owner
        self.abs_path = abs_path


def record_members(store, root_dir, base_dir):
    for root, dirs, files in os.walk(str(root_dir)):
        root_path = pathlib.Path(root)
        rel_root = root_path.relative_to(base_dir).as_posix()
        if rel_root and rel_root != ".":
            if rel_root not in store:
                store[rel_root] = MemberOwner(rel_root, root_dir.name, root_path)
        for fname in files:
            full = root_path / fname
            rel = full.relative_to(base_dir).as_posix()
            if rel not in store:
                store[rel] = MemberOwner(rel, root_dir.name, full)
        for d in dirs:
            sub = root_path / d
            rel = sub.relative_to(base_dir).as_posix()
            if rel not in store:
                store[rel] = MemberOwner(rel, root_dir.name, sub)
    for d in sorted(pathlib.Path(root_dir).rglob("*")):
        if d.is_dir():
            rel = d.relative_to(base_dir).as_posix()
            if rel not in store:
                store[rel] = MemberOwner(rel, root_dir.name, d)


def symlink_check(parts_dir, members):
    for rel, info in members.items():
        p = info.abs_path
        if p.is_symlink():
            target = os.readlink(str(p))
            resolved = (p.parent / target).resolve()
            parts_resolved = parts_dir.resolve()
            if not str(resolved).startswith(str(parts_resolved)):
                raise RuntimeError(f"Symlink逃逸Runtime Root: {rel} -> {target}")
    return True


def check_no_secrets(parts_dir):
    import re
    patterns = [
        (re.compile(r"\bsk-[A-Za-z0-9]{20,}\b"), "sk-"),
        (re.compile(r'"password"\s*:\s*"[^"]{8,}"', re.IGNORECASE), "password"),
        (re.compile(r'"secret"\s*:\s*"[^"]{16,}"', re.IGNORECASE), "secret"),
    ]
    for root, dirs, files in os.walk(str(parts_dir)):
        for fname in files:
            full = pathlib.Path(root) / fname
            if full.is_symlink() or not full.is_file():
                continue
            if full.suffix in (".js", ".mjs", ".json", ".sh", ".txt", ".md"):
                text = full.read_text(encoding="utf-8", errors="ignore")
                for pat, label in patterns:
                    if pat.search(text):
                        rel = full.relative_to(parts_dir).as_posix()
                        raise RuntimeError(f"Runtime安全告警 [{label}]: {rel}")
    return True


def check_forbidden_runtime_paths(members):
    forbidden_roots = {"bin", "etc", "usr", "lib", "lib64", "var", "proc", "dev", "sys", "home", "root", "tmp"}
    for rel in members:
        parts = rel.split("/")
        if parts and parts[0] in forbidden_roots:
            if parts[0] == "backend" or parts[0] == "node" or parts[0] == "qdrant" or \
               parts[0] == "plugin-host" or parts[0] == "task-host":
                continue
            raise RuntimeError(f"Runtime禁止系统根目录: {rel}")
    forbidden_files = {"config", "data", "cache", "logs", "run", "workspace", "workspaces", "home", "tmp"}
    for rel in members:
        first = rel.split("/")[0]
        if first in forbidden_files:
            raise RuntimeError(f"Runtime禁止目录: {rel}")
    return True
