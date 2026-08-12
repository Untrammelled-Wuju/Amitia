import hashlib
import json
import lzma
import os
import pathlib
import stat
import tarfile

SCRIPT_DIR = pathlib.Path(__file__).resolve().parent
FIXED_MTIME = 0
FIXED_UID = 0
FIXED_GID = 0
FIXED_UNAME = "root"
FIXED_GNAME = "root"
XZ_COMPRESSION_LEVEL = 5


def sha256_file(path):
    h = hashlib.sha256()
    with open(path, "rb") as f:
        while True:
            chunk = f.read(1048576)
            if not chunk:
                break
            h.update(chunk)
    return h.hexdigest()


def build_file_manifest(root):
    root = pathlib.Path(root)
    entries = []
    all_paths = []
    for dirpath, dirnames, filenames in os.walk(root, followlinks=False):
        dirnames.sort()
        dirp = pathlib.Path(dirpath)
        rel_dir = dirp.relative_to(root)
        for dn in sorted(dirnames):
            dp = dirp / dn
            rel = (rel_dir / dn).as_posix() if str(rel_dir) != "." else dn
            all_paths.append((rel, dp))
        for fn in sorted(filenames):
            fp = dirp / fn
            rel = (rel_dir / fn).as_posix() if str(rel_dir) != "." else fn
            all_paths.append((rel, fp))
    for rel, fp in all_paths:
        try:
            st = fp.lstat()
        except OSError:
            continue
        if fp.is_symlink():
            target = os.readlink(fp)
            if fp.is_dir():
                entries.append({
                    "path": rel,
                    "type": "dirsymlink",
                    "target": target,
                    "mode": oct(stat.S_IMODE(st.st_mode)),
                    "uid": st.st_uid,
                    "gid": st.st_gid,
                })
            else:
                entries.append({
                    "path": rel,
                    "type": "symlink",
                    "target": target,
                    "mode": oct(stat.S_IMODE(st.st_mode)),
                    "uid": st.st_uid,
                    "gid": st.st_gid,
                })
        elif fp.is_dir():
            entries.append({
                "path": rel,
                "type": "directory",
                "mode": oct(stat.S_IMODE(st.st_mode)),
                "uid": st.st_uid,
                "gid": st.st_gid,
            })
        elif fp.is_file():
            entries.append({
                "path": rel,
                "type": "file",
                "size": st.st_size,
                "sha256": sha256_file(fp),
                "mode": oct(stat.S_IMODE(st.st_mode)),
                "uid": st.st_uid,
                "gid": st.st_gid,
            })
    entries.sort(key=lambda e: e["path"])
    return entries


def write_file_manifest(output_dir, root):
    manifest = build_file_manifest(root)
    out = output_dir / "file-manifest.json"
    content = json.dumps(manifest, indent=2, sort_keys=True, ensure_ascii=False) + "\n"
    with open(out, "w", encoding="utf-8", newline="") as f:
        f.write(content)
    return out


def write_sha256sums(output_dir, names):
    lines = []
    for name in sorted(names):
        fp = output_dir / name
        if not fp.exists():
            continue
        digest = sha256_file(fp)
        lines.append(f"{digest}  {name}")
    out = output_dir / "SHA256SUMS"
    with open(out, "w", encoding="utf-8", newline="") as f:
        f.write("\n".join(lines) + "\n")
    return out


def create_deterministic_tar(root, output_path):
    root = pathlib.Path(root)
    output_path = pathlib.Path(output_path)
    members = []
    for dirpath, dirnames, filenames in os.walk(root, followlinks=False):
        dirnames.sort()
        dirp = pathlib.Path(dirpath)
        rel_dir = dirp.relative_to(root)
        if str(rel_dir) == ".":
            continue
        for dn in sorted(dirnames):
            dp = dirp / dn
            rel = (rel_dir / dn)
            members.append((rel, dp))
        for fn in sorted(filenames):
            fp = dirp / fn
            rel = (rel_dir / fn)
            members.append((rel, fp))
    for dn in sorted([d for d in os.listdir(root) if (root / d).is_dir()]):
        members.append((pathlib.Path(dn), root / dn))
    members.sort(key=lambda x: x[0].as_posix())
    tmp_out = output_path.with_suffix(".tmp")
    seen_paths = set()
    for _, fp in members:
        normalized = str(_)
        if normalized in seen_paths:
            raise RuntimeError(f"重复归档成员: {normalized}")
        seen_paths.add(normalized)
    try:
        with lzma.open(tmp_out, "wb", preset=XZ_COMPRESSION_LEVEL) as xz:
            with tarfile.open(fileobj=xz, mode="w") as tf:
                for rel, fp in members:
                    info = tf.gettarinfo(str(fp), arcname=str(rel))
                    info.uid = FIXED_UID
                    info.gid = FIXED_GID
                    info.uname = FIXED_UNAME
                    info.gname = FIXED_GNAME
                    info.mtime = FIXED_MTIME
                    if fp.is_symlink():
                        info.type = tarfile.SYMTYPE
                        info.linkname = os.readlink(fp)
                        tf.addfile(info)
                    elif fp.is_dir():
                        info.type = tarfile.DIRTYPE
                        info.mode = stat.S_IMODE(fp.lstat().st_mode)
                        tf.addfile(info)
                    else:
                        info.mode = stat.S_IMODE(fp.lstat().st_mode)
                        with open(fp, "rb") as fobj:
                            tf.addfile(info, fobj)
        os.replace(tmp_out, output_path)
    except Exception:
        if tmp_out.exists():
            tmp_out.unlink()
        raise


def verify_archive_determinism(archive_list):
    seen_shas = {}
    for archive_path in archive_list:
        archive_path = pathlib.Path(archive_path)
        if not archive_path.exists():
            continue
        digest = sha256_file(archive_path)
        if archive_path.name in seen_shas:
            if seen_shas[archive_path.name] != digest:
                return False
        seen_shas[archive_path.name] = digest
    return True


def verify_archive_security(archive_path):
    archive_path = pathlib.Path(archive_path)
    issues = []
    with lzma.open(archive_path, "rb") as xz:
        with tarfile.open(fileobj=xz, mode="r") as tf:
            for m in tf.getmembers():
                if not m.name:
                    issues.append("归档包含空路径成员")
                    continue
                if m.name.startswith("/"):
                    issues.append(f"绝对路径成员: {m.name}")
                parts = pathlib.PurePosixPath(m.name).parts
                if ".." in parts:
                    issues.append(f"路径穿越成员: {m.name}")
                if m.ischr() or m.isblk() or m.isfifo():
                    issues.append(f"设备/FIFO 成员: {m.name}")
                if m.pax_headers:
                    issues.append(f"成员包含 PAX 头: {m.name}")
    return issues


def verify_no_extra_rootfs(archive_path):
    archive_path = pathlib.Path(archive_path)
    with lzma.open(archive_path, "rb") as xz:
        with tarfile.open(fileobj=xz, mode="r") as tf:
            for m in tf.getmembers():
                if m.name == "rootfs":
                    if m.isdir():
                        return True
    return False


def safe_replace_output(tmp_output, final_output):
    final_output = pathlib.Path(final_output)
    tmp_output = pathlib.Path(tmp_output)
    if final_output.exists():
        backup = final_output.with_name(final_output.name + ".old")
        if backup.exists():
            if backup.is_dir():
                import shutil
                shutil.rmtree(backup, ignore_errors=True)
            else:
                backup.unlink()
        final_output.rename(backup)
        if backup.is_dir():
            import shutil
            shutil.rmtree(backup, ignore_errors=True)
        else:
            if backup.exists():
                backup.unlink()
    final_output.parent.mkdir(parents=True, exist_ok=True)
    tmp_output.rename(final_output)
