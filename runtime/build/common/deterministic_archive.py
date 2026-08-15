import os
import stat
import tarfile
import time
import zipfile
from pathlib import Path
from typing import List, Tuple

FIXED_MTIME = 1700000000
FIXED_UID = 0
FIXED_GID = 0
FIXED_UNAME = "root"
FIXED_GNAME = "root"
TAR_FORMAT = tarfile.GNU_FORMAT
XZ_PRESET = 6


def _collect_sorted_entries(source_dir: str) -> List[Tuple[str, str]]:
    source_path = Path(source_dir)
    entries = []
    for root, dirs, files in os.walk(source_dir):
        dirs.sort()
        rel_root = os.path.relpath(root, source_dir)
        if rel_root != ".":
            entries.append((root, rel_root))
        for fname in sorted(files):
            full = os.path.join(root, fname)
            rel = os.path.relpath(full, source_dir)
            entries.append((full, rel))
    return entries


def _normalize_path(rel_path: str) -> str:
    return rel_path.replace(os.sep, "/")


def create_deterministic_tar_xz(source_dir: str, output_path: str) -> None:
    entries = _collect_sorted_entries(source_dir)
    Path(output_path).parent.mkdir(parents=True, exist_ok=True)

    with tarfile.open(output_path, "w:xz", format=TAR_FORMAT, preset=XZ_PRESET) as tf:
        for full_path, rel_path in sorted(entries, key=lambda x: _normalize_path(x[1])):
            info = tf.gettarinfo(name=full_path, arcname=_normalize_path(rel_path))
            info.mtime = FIXED_MTIME
            info.uid = FIXED_UID
            info.gid = FIXED_GID
            info.uname = FIXED_UNAME
            info.gname = FIXED_GNAME

            if info.isdir():
                tf.addfile(info)
            elif info.isfile():
                with open(full_path, "rb") as f:
                    tf.addfile(info, f)
            elif info.issym() or info.islnk():
                tf.addfile(info)
            else:
                tf.addfile(info)


def create_deterministic_zip(source_dir: str, output_path: str) -> None:
    entries = _collect_sorted_entries(source_dir)
    Path(output_path).parent.mkdir(parents=True, exist_ok=True)

    fixed_time = time.localtime(FIXED_MTIME)[:6]

    with zipfile.ZipFile(output_path, "w", compression=zipfile.ZIP_DEFLATED, compresslevel=9) as zf:
        for full_path, rel_path in sorted(entries, key=lambda x: _normalize_path(x[1])):
            norm_path = _normalize_path(rel_path)
            zinfo = zipfile.ZipInfo(filename=norm_path, date_time=fixed_time)
            zinfo.compress_type = zipfile.ZIP_DEFLATED
            zinfo.create_system = 3

            st = os.lstat(full_path)
            zinfo.external_attr = (st.st_mode & 0xFFFF) << 16

            if os.path.isdir(full_path):
                if not zinfo.filename.endswith("/"):
                    zinfo.filename += "/"
                zf.writestr(zinfo, "")
            elif os.path.isfile(full_path):
                with open(full_path, "rb") as f:
                    zf.writestr(zinfo, f.read())
            else:
                data = os.readlink(full_path).encode("utf-8")
                zf.writestr(zinfo, data)
