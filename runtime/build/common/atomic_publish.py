import os
import shutil
import tempfile
from typing import Optional


def atomic_publish_dir(staging_dir: str, target_dir: str) -> None:
    parent = os.path.dirname(os.path.abspath(target_dir))
    os.makedirs(parent, exist_ok=True)
    with tempfile.TemporaryDirectory(dir=parent) as tmp:
        tmp_name = os.path.basename(tmp)
        final_name = os.path.basename(target_dir)
        tmp_target = os.path.join(parent, f".{final_name}.tmp-{tmp_name}")
        if os.path.exists(tmp_target):
            shutil.rmtree(tmp_target)
        shutil.copytree(staging_dir, tmp_target)
        final_target = os.path.join(parent, final_name)
        if os.path.exists(final_target):
            old_target = os.path.join(parent, f".{final_name}.old-{tmp_name}")
            os.rename(final_target, old_target)
            os.rename(tmp_target, final_target)
            shutil.rmtree(old_target)
        else:
            os.rename(tmp_target, final_target)


def atomic_publish_file(source_path: str, target_path: str) -> None:
    parent = os.path.dirname(os.path.abspath(target_path))
    os.makedirs(parent, exist_ok=True)
    with tempfile.TemporaryDirectory(dir=parent) as tmp:
        tmp_path = os.path.join(tmp, os.path.basename(target_path))
        shutil.copy2(source_path, tmp_path)
        if os.path.exists(target_path):
            os.replace(tmp_path, target_path)
        else:
            os.rename(tmp_path, target_path)
