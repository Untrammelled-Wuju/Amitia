import os
import stat
from pathlib import Path
from typing import List

from .hashing import sha256_file


def _format_mode(mode: int) -> str:
    return oct(mode)[-4:]


def generate_tree_manifest(root_dir: str) -> List[str]:
    root_path = Path(root_dir)
    if not root_path.exists():
        raise FileNotFoundError(f"Directory not found: {root_dir}")

    entries = []
    for dirpath, dirnames, filenames in os.walk(root_dir):
        dirnames.sort()
        filenames.sort()
        for name in dirnames:
            full = os.path.join(dirpath, name)
            rel = os.path.relpath(full, root_dir).replace(os.sep, "/")
            st = os.lstat(full)
            mode = _format_mode(stat.S_IMODE(st.st_mode))
            entries.append(("D", rel, mode, None, None))
        for name in filenames:
            full = os.path.join(dirpath, name)
            rel = os.path.relpath(full, root_dir).replace(os.sep, "/")
            st = os.lstat(full)
            mode = _format_mode(stat.S_IMODE(st.st_mode))
            if stat.S_ISLNK(st.st_mode):
                target = os.readlink(full)
                entries.append(("L", rel, mode, target, None))
            else:
                digest = sha256_file(full)
                entries.append(("F", rel, mode, digest, None))

    lines = []
    for entry in entries:
        kind, rel, mode = entry[0], entry[1], entry[2]
        if kind == "F":
            digest = entry[3]
            lines.append(f"F {mode} {digest} {rel}")
        elif kind == "L":
            target = entry[3]
            lines.append(f"L {mode} {target} {rel}")
        elif kind == "D":
            lines.append(f"D {mode} - {rel}")

    return lines
