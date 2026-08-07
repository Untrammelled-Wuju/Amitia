import hashlib
import lzma
import pathlib
import shutil
import tarfile
import tempfile


ALLOWED_ROOTFS_TYPES = {"f", "d", "l", "h"}
MAX_ROOTFS_MEMBERS = 500000
RSIX_COMPRESSION_LEVEL = 5


def _open_deterministic_xz(filename, mode="wb"):
    if "w" in mode:
        filters = [{"id": lzma.FILTER_LZMA2, "preset": RSIX_COMPRESSION_LEVEL}]
        return lzma.open(filename, mode=mode, format=lzma.FORMAT_XZ, check=lzma.CHECK_SHA256, filters=filters)
    return lzma.open(filename, mode=mode)


def read_archive_members(path):
    members = {}
    with tarfile.open(str(path), "r:*") as tf:
        for m in tf.getmembers():
            if len(members) >= MAX_ROOTFS_MEMBERS:
                raise RuntimeError(f"rootfs 成员数量超限: >= {MAX_ROOTFS_MEMBERS}")
            if m.size < 0:
                raise RuntimeError(f"rootfs成员负大小: {m.name}")
            members[m.name] = m
    return members


def validate_rootfs_seed(members):
    forbidden_programs = [
        "opt/amitia/backend/amitia-server",
        "opt/amitia/node/bin/node",
        "opt/amitia/qdrant/bin/qdrant",
        "opt/amitia/plugin-host/dist/index.js",
        "opt/amitia/task-host/dist/index.js",
    ]
    for pf in forbidden_programs:
        if pf in members:
            raise RuntimeError(f"RootFS禁止包含程序文件: {pf}")
    for name in members:
        if name.startswith("opt/amitia/") and name not in [
            "opt/amitia",
            "opt/amitia/backend",
            "opt/amitia/licenses",
            "opt/amitia/manifest",
            "opt/amitia/manifest/guest-layout.json",
            "opt/amitia/manifest/mount-contract.json",
            "opt/amitia/node",
            "opt/amitia/plugin-host",
            "opt/amitia/qdrant",
            "opt/amitia/scripts",
            "opt/amitia/scripts/node",
            "opt/amitia/scripts/runtime",
            "opt/amitia/task-host",
        ]:
            raise RuntimeError(f"RootFS opt/amitee非法文件: {name}")
    variable_dirs = ["etc/amitia", "var/lib/amitia", "var/cache/amitia",
                     "var/log/amitia", "run/amitia"]
    for name, info in members.items():
        for vdir in variable_dirs:
            if name.startswith(vdir + "/") and name != vdir and info.isfile():
                raise RuntimeError(f"RootFS可变目录禁止文件: {name}")
    return True


def compose_rootfs(rootfs_artifact_path, guest_layout_artifact_path, output_path):
    rootfs_path = pathlib.Path(rootfs_artifact_path)
    overlay_path = pathlib.Path(guest_layout_artifact_path)
    out_path = pathlib.Path(output_path)
    out_path.parent.mkdir(parents=True, exist_ok=True)
    if not rootfs_path.is_file():
        raise RuntimeError(f"RootFS Artifact不存在: {rootfs_path}")
    if not overlay_path.is_file():
        raise RuntimeError(f"Guest Layout Artifact不存在: {overlay_path}")
    rootfs_members = read_archive_members(rootfs_path)
    overlay_members = read_archive_members(overlay_path)
    merged = dict(rootfs_members)
    for name, om in overlay_members.items():
        if name in merged and merged[name].type != tarfile.DIRTYPE:
            raise RuntimeError(f"Overlay禁止覆盖非目录文件: {name}")
        merged[name] = om
    validate_rootfs_seed(merged)
    sorted_names = sorted(merged.keys())
    work_dir = pathlib.Path(tempfile.mkdtemp(prefix="rootfs_composer_"))
    try:
        output_tmp = work_dir / out_path.name
        fzf = _open_deterministic_xz(str(output_tmp), "wb")
        tar_out = tarfile.open(fileobj=fzf, mode="w:", format=tarfile.GNU_FORMAT)
        membership_root = tarfile.open(str(rootfs_path), "r:*")
        membership_overlay = tarfile.open(str(overlay_path), "r:*")
        try:
            for name in sorted_names:
                info = merged[name]
                info.mtime = 0
                info.uid = 0
                info.gid = 0
                info.uname = ""
                info.gname = ""
                info.pax_headers = {}
                if info.isdir():
                    tar_out.addfile(info)
                elif info.isfile():
                    src = membership_overlay if (name in overlay_members and name not in rootfs_members) else membership_root
                    src_info = src.getmember(name)
                    fp = src.extractfile(src_info)
                    tar_out.addfile(info, fp)
                else:
                    tar_out.addfile(info)
        finally:
            tar_out.close()
            fzf.close()
            membership_root.close()
            membership_overlay.close()
        sha = hashlib.sha256(output_tmp.read_bytes()).hexdigest()
        shutil.copy2(str(output_tmp), str(out_path))
    finally:
        shutil.rmtree(str(work_dir), ignore_errors=True)
    return str(out_path), sha
