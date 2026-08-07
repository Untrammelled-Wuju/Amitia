import io
import os
import pathlib
import struct
import tarfile
import zipfile


class ArchiveMember:
    __slots__ = ("name", "size", "mode", "uid", "gid", "mtime", "data", "linkname", "is_file", "is_dir", "is_symlink", "is_hardlink")

    def __init__(self, name, size=0, mode=0o644, uid=0, gid=0, mtime=0, data=b"", linkname="", is_file=False, is_dir=False, is_symlink=False, is_hardlink=False):
        self.name = name
        self.size = size
        self.mode = mode
        self.uid = uid
        self.gid = gid
        self.mtime = mtime
        self.data = data
        self.linkname = linkname
        self.is_file = is_file
        self.is_dir = is_dir
        self.is_symlink = is_symlink
        self.is_hardlink = is_hardlink

    @property
    def type_label(self):
        if self.is_dir:
            return "d"
        if self.is_symlink:
            return "l"
        if self.is_hardlink:
            return "h"
        if self.is_file:
            return "f"
        return "?"


MAX_TAR_MEMBERS = 500000
MAX_TAR_SINGLE_FILE = 1024 * 1024 * 1024


class TarReader:
    def __init__(self, path, max_members=MAX_TAR_MEMBERS, max_single_file=MAX_TAR_SINGLE_FILE):
        self.path = pathlib.Path(path)
        self.max_members = max_members
        self.max_single_file = max_single_file

    def read_members(self):
        members = []
        with tarfile.open(str(self.path), "r:*") as tf:
            for i, m in enumerate(tf.getmembers()):
                if i >= self.max_members:
                    raise RuntimeError(f"Tar 成员数量超限: >= {self.max_members}")
                if m.size < 0:
                    raise RuntimeError(f"成员大小为负: {m.name}")
                if m.size > self.max_single_file and m.isfile():
                    raise RuntimeError(f"成员文件过大: {m.name} ({m.size} bytes)")
                data = None
                if m.isfile():
                    f = tf.extractfile(m)
                    if f is None:
                        raise RuntimeError(f"无法读取成员: {m.name}")
                    data = f.read()
                    if len(data) != m.size:
                        raise RuntimeError(f"成员读取大小不一致: {m.name}")
                am = ArchiveMember(
                    name=m.name,
                    size=m.size if m.isfile() else 0,
                    mode=m.mode,
                    uid=m.uid,
                    gid=m.gid,
                    mtime=m.mtime,
                    data=data if data is not None else b"",
                    linkname=m.linkname if m.issym() or m.islnk() else "",
                    is_file=m.isfile(),
                    is_dir=m.isdir(),
                    is_symlink=m.issym(),
                    is_hardlink=m.islnk(),
                )
                members.append(am)
        return members

    def get_path_set(self):
        members = self.read_members()
        return {m.name for m in members}


class ZipReader:
    def __init__(self, path):
        self.path = pathlib.Path(path)

    def namelist(self):
        with zipfile.ZipFile(str(self.path), "r") as zf:
            return zf.namelist()

    def infolist(self):
        with zipfile.ZipFile(str(self.path), "r") as zf:
            return zf.infolist()

    def read_entry(self, name):
        with zipfile.ZipFile(str(self.path), "r") as zf:
            return zf.read(name)
