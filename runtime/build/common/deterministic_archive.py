import hashlib
import os
import stat
import tarfile
import time
from typing import Optional


def create_deterministic_tar(
    source_dir: str,
    output_path: str,
    mode: str = "w:xz",
    preset: int = 5,
) -> None:
    with tarfile.open(output_path, mode, preset=preset) as tf:
        for dirpath, dirnames, filenames in os.walk(source_dir):
            dirnames.sort()
            for filename in sorted(filenames):
                filepath = os.path.join(dirpath, filename)
                relpath = os.path.relpath(filepath, source_dir).replace("\\", "/")
                tarinfo = tf.gettarinfo(filepath, arcname=relpath)
                tarinfo.uid = 0
                tarinfo.gid = 0
                tarinfo.uname = "root"
                tarinfo.gname = "root"
                tarinfo.mtime = 0
                if tarinfo.isfile():
                    with open(filepath, "rb") as f:
                        tf.addfile(tarinfo, f)
                else:
                    tf.addfile(tarinfo)


def verify_tar_reopen(archive_path: str) -> bool:
    try:
        with tarfile.open(archive_path, "r") as tf:
            for member in tf.getmembers():
                if member.isfile():
                    tf.extractfile(member)
        return True
    except Exception:
        return False
