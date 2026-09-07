import os
import shutil
from typing import List, Optional

try:
    from . import process_runner
except ImportError:
    import process_runner


def kill_amitia_processes(safe_pids: Optional[List[int]] = None) -> bool:
    safe_pids_set = set(safe_pids or [])
    try:
        import glob
        for proc_dir in glob.glob("/proc/[0-9]*"):
            try:
                pid = int(os.path.basename(proc_dir))
                if pid in safe_pids_set:
                    continue
                comm_path = os.path.join(proc_dir, "comm")
                with open(comm_path, "r", encoding="utf-8", errors="replace") as f:
                    name = f.read().strip()
                if name.lower() in ("amitia-server", "qdrant"):
                    process_runner.terminate_pid(pid, timeout=15)
                elif name.lower() == "node":
                    exe_path = os.readlink(os.path.join(proc_dir, "exe")) if os.path.exists(os.path.join(proc_dir, "exe")) else ""
                    if "/opt/amitia" in exe_path:
                        process_runner.terminate_pid(pid, timeout=15)
            except (OSError, PermissionError, ValueError):
                continue
        return True
    except Exception:
        return False


def release_work_dir(work_dir: str) -> bool:
    if not os.path.exists(work_dir):
        return True
    for entry in os.listdir(work_dir):
        path = os.path.join(work_dir, entry)
        try:
            if os.path.isdir(path) and not os.path.islink(path):
                shutil.rmtree(path, ignore_errors=True)
            else:
                os.unlink(path)
        except OSError:
            continue
    return True


def safe_remove_directory(path: str) -> bool:
    if not os.path.exists(path):
        return True
    try:
        shutil.rmtree(path, ignore_errors=True)
        return not os.path.exists(path)
    except Exception:
        return False


def safe_remove_file(path: str) -> bool:
    try:
        if os.path.exists(path) or os.path.islink(path):
            os.unlink(path)
        return True
    except OSError:
        return False


def cleanup_work_dir(work_dir: str, keep_report_path: Optional[str] = None) -> bool:
    if not os.path.exists(work_dir):
        return True
    for entry in os.listdir(work_dir):
        if keep_report_path and os.path.join(work_dir, entry) == os.path.dirname(keep_report_path):
            continue
        path = os.path.join(work_dir, entry)
        try:
            if os.path.isdir(path) and not os.path.islink(path):
                shutil.rmtree(path, ignore_errors=True)
            else:
                os.unlink(path)
        except OSError:
            continue
    return True
