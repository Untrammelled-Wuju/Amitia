import os
import signal
import subprocess
import time
from dataclasses import dataclass, field
from pathlib import Path
from typing import Dict, List, Optional


@dataclass
class ProcessStartConfig:
    args: List[str]
    cwd: str = "."
    env: Optional[Dict[str, str]] = None
    stdoutPath: Optional[str] = None
    stderrPath: Optional[str] = None
    stdioSuppressed: bool = False


@dataclass
class ProcessInfo:
    pid: int
    args: List[str]
    cwd: str
    startTime: float
    stdoutPath: Optional[str] = None
    stderrPath: Optional[str] = None
    process: Optional[subprocess.Popen] = None
    metadata: Dict[str, str] = field(default_factory=dict)

    def to_safe_dict(self) -> dict:
        return {
            "pid": self.pid,
            "args": self.args,
            "cwd": self.cwd,
            "startTime": self.startTime,
            "stdoutPath": self.stdoutPath,
            "stderrPath": self.stderrPath,
            "metadata": self.metadata,
        }


@dataclass
class ProcessResult:
    pid: int
    returncode: int
    stdout: str = ""
    stderr: str = ""
    durationMs: int = 0


def start_process(config: ProcessStartConfig) -> ProcessInfo:
    stdout_handle = subprocess.DEVNULL
    stderr_handle = subprocess.DEVNULL
    stdout_path = None
    stderr_path = None

    if config.stdoutPath:
        os.makedirs(os.path.dirname(config.stdoutPath) or ".", exist_ok=True)
        stdout_handle = open(config.stdoutPath, "w", encoding="utf-8", errors="replace")
        stdout_path = config.stdoutPath

    if config.stderrPath:
        os.makedirs(os.path.dirname(config.stderrPath) or ".", exist_ok=True)
        stderr_handle = open(config.stderrPath, "w", encoding="utf-8", errors="replace")
        stderr_path = config.stderrPath

    combined_env = dict(os.environ)
    if config.env:
        combined_env.update(config.env)

    popen_kwargs = {
        "cwd": config.cwd,
        "env": combined_env,
        "start_new_session": True,
    }
    if config.stdioSuppressed:
        popen_kwargs["stdout"] = subprocess.DEVNULL
        popen_kwargs["stderr"] = subprocess.DEVNULL
    else:
        popen_kwargs["stdout"] = stdout_handle
        popen_kwargs["stderr"] = stderr_handle

    process = subprocess.Popen(config.args, **popen_kwargs)

    if stdout_handle != subprocess.DEVNULL:
        stdout_handle.close()
    if stderr_handle != subprocess.DEVNULL and stderr_handle != stdout_handle:
        stderr_handle.close()

    return ProcessInfo(
        pid=process.pid,
        args=list(config.args),
        cwd=config.cwd,
        startTime=time.time(),
        stdoutPath=stdout_path,
        stderrPath=stderr_path,
        process=process,
    )


def run_process(args: List[str], timeout: float = 60.0, cwd: str = ".", env: Optional[Dict[str, str]] = None) -> ProcessResult:
    combined_env = dict(os.environ)
    if env:
        combined_env.update(env)

    start = time.time()
    try:
        result = subprocess.run(
            args,
            capture_output=True,
            text=True,
            timeout=timeout,
            cwd=cwd,
            env=combined_env,
            start_new_session=True,
            check=False,
        )
        duration = int((time.time() - start) * 1000)
        return ProcessResult(
            pid=result.pid or 0,
            returncode=result.returncode,
            stdout=result.stdout or "",
            stderr=result.stderr or "",
            durationMs=duration,
        )
    except subprocess.TimeoutExpired as e:
        duration = int((time.time() - start) * 1000)
        stdout_text = (e.stdout or b"").decode("utf-8", errors="replace") if e.stdout else ""
        stderr_text = (e.stderr or b"").decode("utf-8", errors="replace") if e.stderr else ""
        return ProcessResult(
            pid=e.pid or 0,
            returncode=-1,
            stdout=stdout_text,
            stderr=stderr_text,
            durationMs=duration,
        )


def wait_for_exit(info: ProcessInfo, timeout: float = 30.0) -> int:
    if info.process is None:
        return -1
    try:
        info.process.wait(timeout=timeout)
    except subprocess.TimeoutExpired:
        return -1
    return info.process.returncode if info.process.returncode is not None else -1


def terminate_process(info: ProcessInfo, timeout: float = 10.0) -> int:
    if info.process is None:
        return 0
    try:
        os.killpg(os.getpgid(info.pid), signal.SIGTERM)
    except (ProcessLookupError, OSError):
        pass
    try:
        info.process.wait(timeout=timeout)
        return info.process.returncode or 0
    except subprocess.TimeoutExpired:
        try:
            os.killpg(os.getpgid(info.pid), signal.SIGKILL)
        except (ProcessLookupError, OSError):
            pass
        try:
            info.process.wait(timeout=5.0)
            return info.process.returncode or -9
        except subprocess.TimeoutExpired:
            return -9


def terminate_pid(pid: int, timeout: float = 10.0) -> bool:
    try:
        os.kill(pid, signal.SIGTERM)
    except ProcessLookupError:
        return True
    except OSError:
        return False
    deadline = time.time() + timeout
    while time.time() < deadline:
        try:
            os.kill(pid, 0)
            time.sleep(0.1)
        except ProcessLookupError:
            return True
        except OSError:
            return True
    try:
        os.kill(pid, signal.SIGKILL)
    except (ProcessLookupError, OSError):
        pass
    return True


def read_stderr_tail(info: ProcessInfo, limit_bytes: int = 65536) -> str:
    if not info.stderrPath or not os.path.exists(info.stderrPath):
        return ""
    size = os.path.getsize(info.stderrPath)
    start = max(0, size - limit_bytes)
    with open(info.stderrPath, "r", encoding="utf-8", errors="replace") as f:
        if start > 0:
            f.seek(start)
        return f.read()


def read_stdout_tail(info: ProcessInfo, limit_bytes: int = 65536) -> str:
    if not info.stdoutPath or not os.path.exists(info.stdoutPath):
        return ""
    size = os.path.getsize(info.stdoutPath)
    start = max(0, size - limit_bytes)
    with open(info.stdoutPath, "r", encoding="utf-8", errors="replace") as f:
        if start > 0:
            f.seek(start)
        return f.read()


def process_alive(pid: int) -> bool:
    try:
        os.kill(pid, 0)
        return True
    except ProcessLookupError:
        return False
    except OSError:
        return True
