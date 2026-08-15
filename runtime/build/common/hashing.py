import hashlib
import os
from pathlib import Path
from typing import Dict


def sha256_file(path: str) -> str:
    digest = hashlib.sha256()
    with open(path, "rb") as f:
        for chunk in iter(lambda: f.read(65536), b""):
            digest.update(chunk)
    return digest.hexdigest()


def sha256_bytes(data: bytes) -> str:
    return hashlib.sha256(data).hexdigest()


def sha256_tree_manifest(root_dir: str) -> str:
    """Generate a deterministic SHA-256 over all files in a directory tree."""
    root_path = Path(root_dir)
    if not root_path.exists():
        raise FileNotFoundError(f"Directory not found: {root_dir}")

    files = sorted(p for p in root_path.rglob("*") if p.is_file())
    digest = hashlib.sha256()
    for file_path in files:
        rel = file_path.relative_to(root_path).as_posix()
        digest.update(rel.encode("utf-8"))
        digest.update(b"\0")
        with open(file_path, "rb") as f:
            for chunk in iter(lambda: f.read(65536), b""):
                digest.update(chunk)
    return digest.hexdigest()


def compute_tree_file_manifest(root_dir: str) -> Dict[str, str]:
    """Compute individual SHA-256 for each file in the tree."""
    root_path = Path(root_dir)
    if not root_path.exists():
        raise FileNotFoundError(f"Directory not found: {root_dir}")

    result = {}
    for file_path in sorted(root_path.rglob("*")):
        if file_path.is_file():
            rel = file_path.relative_to(root_path).as_posix()
            result[rel] = sha256_file(str(file_path))
    return result
