import hashlib
import lzma
import os
import pathlib
import stat
import tarfile

ARCHIVE_MTIME = 0
ROOT_UNAME = "root"
ROOT_GNAME = "root"
DEFAULT_DIR_MODE = 0o755
VAR_DIR_MODE = 0o750
PRIVATE_DIR_MODE = 0o700
FILE_MODE = 0o644
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


def create_deterministic_tar(src_root, archive_path, uid=0, gid=0, mode_overrides=None):
    src_root = pathlib.Path(src_root)
    archive_path = pathlib.Path(archive_path)
    mode_overrides = mode_overrides or {}

    entries = []
    root_entries = sorted(src_root.iterdir(), key=lambda p: p.name)
    for p in root_entries:
        entries.append((p.name, p, p.is_dir() or (p.is_symlink() and p.exists() is False)))

    for dirpath, dirnames, filenames in os.walk(str(src_root), followlinks=False):
        dirnames.sort()
        dp = pathlib.Path(dirpath)
        rel = dp.relative_to(src_root)
        if str(rel) == ".":
            continue
        for dn in sorted(dirnames):
            p = dp / dn
            rel_name = f"{rel.as_posix()}/{dn}"
            entries.append((rel_name, p, True))
        for fn in sorted(filenames):
            p = dp / fn
            rel_name = f"{rel.as_posix()}/{fn}"
            entries.append((rel_name, p, False))

    entries.sort(key=lambda e: e[0])

    with lzma.open(str(archive_path), "wb", preset=XZ_COMPRESSION_LEVEL) as xz:
        with tarfile.open(fileobj=xz, mode="w:") as tf:
            for rel_name, path, is_dir in entries:
                try:
                    st = path.lstat()
                except OSError:
                    continue
                info = tarfile.TarInfo(name=rel_name)
                info.uid = uid
                info.gid = gid
                info.uname = ROOT_UNAME
                info.gname = ROOT_GNAME
                info.mtime = ARCHIVE_MTIME

                if is_dir:
                    if path.is_symlink():
                        info.type = tarfile.SYMTYPE
                        info.linkname = os.readlink(str(path))
                        info.mode = DEFAULT_DIR_MODE
                        info.size = 0
                    else:
                        info.type = tarfile.DIRTYPE
                        info.mode = mode_overrides.get(rel_name, DEFAULT_DIR_MODE)
                        info.size = 0
                elif path.is_symlink():
                    info.type = tarfile.SYMTYPE
                    info.linkname = os.readlink(str(path))
                    info.mode = FILE_MODE
                    info.size = 0
                elif stat.S_ISREG(st.st_mode):
                    info.type = tarfile.REGTYPE
                    info.size = st.st_size
                    if rel_name.endswith(".json"):
                        info.mode = FILE_MODE
                    elif rel_name in mode_overrides:
                        info.mode = mode_overrides[rel_name]
                    else:
                        info.mode = stat.S_IMODE(st.st_mode)
                else:
                    continue

                if info.type == tarfile.REGTYPE:
                    with open(str(path), "rb") as fp:
                        tf.addfile(info, fp)
                else:
                    tf.addfile(info)

    return archive_path


def create_directory_tar(src_root, archive_path):
    src_root = pathlib.Path(src_root)
    archive_path = pathlib.Path(archive_path)
    with lzma.open(str(archive_path), "wb", preset=XZ_COMPRESSION_LEVEL) as xz:
        with tarfile.open(fileobj=xz, mode="w:") as tf:
            for dirpath, dirnames, filenames in os.walk(str(src_root), followlinks=False):
                dirnames.sort()
                dp = pathlib.Path(dirpath)
                rel = dp.relative_to(src_root)
                for dn in sorted(dirnames):
                    p = dp / dn
                    rel_name = (rel / dn).as_posix() if str(rel) != "." else dn
                    try:
                        st = p.lstat()
                    except OSError:
                        continue
                    info = tarfile.TarInfo(name=rel_name)
                    info.type = tarfile.DIRTYPE
                    info.uid = 0
                    info.gid = 0
                    info.uname = ROOT_UNAME
                    info.gname = ROOT_GNAME
                    info.mtime = ARCHIVE_MTIME
                    info.mode = stat.S_IMODE(st.st_mode) if stat.S_ISDIR(st.st_mode) else DEFAULT_DIR_MODE
                    info.size = 0
                    tf.addfile(info)
                for fn in sorted(filenames):
                    p = dp / fn
                    rel_name = (rel / fn).as_posix() if str(rel) != "." else fn
                    try:
                        st = p.lstat()
                    except OSError:
                        continue
                    if not stat.S_ISREG(st.st_mode):
                        continue
                    info = tarfile.TarInfo(name=rel_name)
                    info.type = tarfile.REGTYPE
                    info.uid = 0
                    info.gid = 0
                    info.uname = ROOT_UNAME
                    info.gname = ROOT_GNAME
                    info.mtime = ARCHIVE_MTIME
                    info.size = st.st_size
                    info.mode = FILE_MODE if rel_name.endswith(".json") else stat.S_IMODE(st.st_mode)
                    with open(str(p), "rb") as fp:
                        tf.addfile(info, fp)
    return archive_path


def verify_no_extra_rootfs(archive_path):
    archive_path = pathlib.Path(archive_path)
    with lzma.open(archive_path, "rb") as xz:
        with tarfile.open(fileobj=xz, mode="r:") as tf:
            for m in tf.getmembers():
                if m.name.startswith(("overlay/", "guest-layout/", "rootfs/")):
                    return False
                if m.name.startswith("/"):
                    return False
    return True


def iterate_archive_members(archive_path):
    archive_path = pathlib.Path(archive_path)
    with lzma.open(archive_path, "rb") as xz:
        with tarfile.open(fileobj=xz, mode="r:") as tf:
            for m in tf.getmembers():
                yield m
