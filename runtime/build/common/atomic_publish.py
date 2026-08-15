import os
import shutil
import tempfile
from dataclasses import dataclass
from pathlib import Path
from typing import List, Optional


@dataclass
class AtomicPublishResult:
    success: bool = False
    published_dir: str = ""
    errors: List[str] = None

    def __post_init__(self):
        if self.errors is None:
            self.errors = []


def atomic_publish_directory(
    source_dir: str,
    target_base_dir: str,
    version_dir_name: str,
) -> AtomicPublishResult:
    result = AtomicPublishResult()
    source_path = Path(source_dir)
    target_base = Path(target_base_dir)
    final_dir = target_base / version_dir_name

    if not source_path.exists():
        result.errors.append(f"Source directory does not exist: {source_dir}")
        return result

    if not any(source_path.iterdir()):
        result.errors.append(f"Source directory is empty: {source_dir}")
        return result

    if final_dir.exists():
        result.errors.append(f"Target version directory already exists: {final_dir}")
        return result

    target_base.mkdir(parents=True, exist_ok=True)
    tmp_dir = None
    try:
        tmp_dir = tempfile.mkdtemp(prefix=".publish-", dir=str(target_base))
        tmp_path = Path(tmp_dir) / version_dir_name
        shutil.copytree(str(source_path), str(tmp_path))
        tmp_path.rename(str(final_dir))
        result.success = True
        result.published_dir = str(final_dir)
    except Exception as e:
        result.errors.append(f"Atomic publish failed: {e}")
        if final_dir.exists():
            try:
                shutil.rmtree(str(final_dir), ignore_errors=True)
            except Exception:
                pass
    finally:
        if tmp_dir and os.path.exists(tmp_dir):
            try:
                shutil.rmtree(tmp_dir, ignore_errors=True)
            except Exception:
                pass

    return result
