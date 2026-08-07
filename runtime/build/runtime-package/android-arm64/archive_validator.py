import os
import pathlib

MAX_PATH_LENGTH = 4096
FORBIDDEN_TYPES = {"block", "char", "fifo", "socket"}


def normalize_path(name):
    if not name:
        raise ValueError("成员路径为空")
    if "\x00" in name:
        raise ValueError(f"路径包含NUL: {repr(name)}")
    if not all(c == "/" or c == "\\" or c > " " for c in name):
        raise ValueError(f"路径包含非法字符: {repr(name)}")
    if "\\" in name:
        raise ValueError(f"路径包含反斜杠: {repr(name)}")
    if name.startswith("/"):
        raise ValueError(f"路径为绝对路径: {repr(name)}")
    if len(name) > MAX_PATH_LENGTH:
        raise ValueError(f"路径过长: {len(name)}")
    if name.startswith("./"):
        raise ValueError(f"路径以 ./ 开头: {repr(name)}")
    if ":" in name.split("/")[0]:
        raise ValueError(f"路径含Windows盘符: {repr(name)}")
    parts = name.split("/")
    depth = 0
    for p in parts:
        if p == "..":
            depth -= 1
        elif p == "." or p == "":
            pass
        else:
            depth += 1
        if depth < 0:
            raise ValueError(f"路径穿越: {repr(name)}")
    if depth < 0:
        raise ValueError(f"规范化后路径穿越: {repr(name)}")
    return name


def validate_member(name, member_type, linkname=None, allow_device=False):
    normalize_path(name)
    if member_type in FORBIDDEN_TYPES and not allow_device:
        raise ValueError(f"禁止的成员类型 {member_type}: {repr(name)}")
    if member_type == "symlink":
        if not linkname:
            raise ValueError(f"Symlink 缺少目标: {repr(name)}")
        if linkname.startswith("/"):
            raise ValueError(f"不允许绝对Symlink目标: {repr(name)} -> {repr(linkname)}")
        parts = linkname.split("/")
        depth = 0
        for p in parts:
            if p == "..":
                depth -= 1
            elif p == "." or p == "":
                pass
            else:
                depth += 1
            if depth < -1:
                raise ValueError(f"Symlink目标逃逸: {repr(name)} -> {repr(linkname)}")
    return True


def validate_runtime_member(name, member_type, linkname=None):
    normalize_path(name)
    if member_type in FORBIDDEN_TYPES:
        raise ValueError(f"Runtime禁止成员类型 {member_type}: {repr(name)}")
    if member_type == "symlink":
        if linkname and linkname.startswith("/"):
            raise ValueError(f"Runtime禁止绝对Symlink: {repr(name)} -> {repr(linkname)}")
    return True


def check_no_duplicates(names):
    seen = set()
    for n in names:
        if n in seen:
            raise ValueError(f"重复成员路径: {repr(n)}")
        seen.add(n)
    return True
