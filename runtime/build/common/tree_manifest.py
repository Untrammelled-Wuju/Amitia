import hashlib
import json
import os
from typing import Dict, List, Optional


def compute_tree_manifest(root_dir: str) -> Dict[str, str]:
    manifest = {}
    for dirpath, dirnames, filenames in os.walk(root_dir):
        dirnames.sort()
        for filename in sorted(filenames):
            filepath = os.path.join(dirpath, filename)
            relpath = os.path.relpath(filepath, root_dir).replace("\\", "/")
            manifest[relpath] = _sha256_file(filepath)
    return manifest


def write_tree_manifest(root_dir: str, output_path: str) -> None:
    manifest = compute_tree_manifest(root_dir)
    with open(output_path, "w", encoding="utf-8") as f:
        json.dump(manifest, f, indent=2, sort_keys=True)
        f.write("\n")


def read_tree_manifest(path: str) -> Dict[str, str]:
    with open(path, "r", encoding="utf-8") as f:
        return json.load(f)


def validate_tree_against_manifest(root_dir: str, expected: Dict[str, str]) -> List[str]:
    errors = []
    actual = compute_tree_manifest(root_dir)
    for relpath, expected_sha in expected.items():
        if relpath not in actual:
            errors.append(f"Missing file: {relpath}")
        elif actual[relpath] != expected_sha:
            errors.append(f"SHA mismatch: {relpath}")
    for relpath in actual:
        if relpath not in expected:
            errors.append(f"Unexpected file: {relpath}")
    return errors


def _sha256_file(path: str) -> str:
    digest = hashlib.sha256()
    with open(path, "rb") as f:
        for chunk in iter(lambda: f.read(65536), b""):
            digest.update(chunk)
    return digest.hexdigest()
