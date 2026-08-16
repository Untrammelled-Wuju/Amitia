import os
import shutil
from typing import Optional

from .errors import VersionPolicyError


def check_version_policy(
    output_dir: str,
    component_id: str,
    version: str,
    allow_overwrite_same_version: bool = False,
) -> None:
    version_dir = os.path.join(output_dir, component_id, version)
    if not os.path.exists(version_dir):
        return
    if allow_overwrite_same_version:
        return
    raise VersionPolicyError(
        f"Version directory already exists for {component_id}@{version}. "
        f"Same-version overwrite is disabled. Dir: {version_dir}"
    )


def same_version_gate(
    output_dir: str,
    component_id: str,
    version: str,
    expected_tree_sha: Optional[str] = None,
    actual_tree_sha: Optional[str] = None,
) -> None:
    version_dir = os.path.join(output_dir, component_id, version)
    if not os.path.exists(version_dir):
        return
    if expected_tree_sha and actual_tree_sha:
        if expected_tree_sha != actual_tree_sha:
            raise VersionPolicyError(
                f"Same version {component_id}@{version} but different tree SHA. "
                f"Expected: {expected_tree_sha}, Got: {actual_tree_sha}"
            )
