import os
import pathlib
import platform
import shutil
import subprocess
import sys

SCRIPT_DIR = pathlib.Path(__file__).resolve().parent
FIXED_ENV = {
    "DEBIAN_FRONTEND": "noninteractive",
    "LANG": "C.UTF-8",
    "LC_ALL": "C.UTF-8",
    "TZ": "Etc/UTC",
    "HOME": "/root",
    "PATH": "/usr/local/sbin:/usr/local/bin:/usr/sbin:/usr/bin:/sbin:/bin",
    "APT_LISTCHANGES_FRONTEND": "none",
    "UCF_FORCE_CONFFOLD": "1",
}
BLOCKED_ENV_KEYS = {
    "HOME", "USER", "PATH", "http_proxy", "https_proxy", "ftp_proxy", "all_proxy",
    "HTTP_PROXY", "HTTPS_PROXY", "FTP_PROXY", "ALL_PROXY", "GOPATH", "GOROOT",
    "GOMODCACHE", "NODE_PATH", "NODE_OPTIONS", "ANDROID_HOME", "ANDROID_SDK_ROOT",
    "JAVA_HOME", "TMPDIR",
}


def detect_qemu_needed(target_arch):
    host_machine = platform.machine().lower()
    if target_arch == "arm64":
        return host_machine not in ("aarch64", "arm64")
    if target_arch == "amd64":
        return host_machine not in ("x86_64", "amd64")
    return True


def find_proot():
    proot_path = shutil.which("proot")
    if proot_path:
        return proot_path
    raise RuntimeError("未找到 proot 可执行文件，请确保已安装 proot 并加入 PATH")


def find_qemu_aarch64_static():
    qemu_path = shutil.which("qemu-aarch64-static")
    if qemu_path:
        return qemu_path
    common_paths = [
        "/usr/bin/qemu-aarch64-static",
        "/usr/local/bin/qemu-aarch64-static",
    ]
    for p in common_paths:
        if pathlib.Path(p).exists():
            return p
    raise RuntimeError("未找到 qemu-aarch64-static，x86_64 构建环境需要该组件")


def architecture():
    return platform.machine().lower()


def supports_arm64_execution():
    return platform.machine().lower() in ("aarch64", "arm64")


def _validate_bindings(binds):
    validated = []
    for host_path, guest_path in binds:
        host_abs = pathlib.Path(host_path).resolve()
        if not host_abs.exists():
            raise RuntimeError(f"绑定源不存在: {host_abs}")
        guest_str = str(guest_path)
        if not guest_str.startswith("/"):
            raise RuntimeError(f"Guest 绑定目标必须为绝对路径: {guest_path}")
        validated.append((host_abs, guest_str))
    return validated


def _validate_env(env):
    if env is None:
        return {}
    clean = {}
    for k, v in env.items():
        if k in BLOCKED_ENV_KEYS:
            continue
        clean[k] = v
    return clean


def build_command(rootfs_path, qemu_needed, qemu_path=None, proot_path=None):
    if proot_path is None:
        proot_path = find_proot()
    elif proot_path == "":
        proot_path = "/usr/bin/proot"
    cmd = [proot_path]
    if qemu_needed:
        if qemu_path is None:
            qemu_path = find_qemu_aarch64_static()
        elif qemu_path == "":
            qemu_path = "/usr/bin/qemu-aarch64-static"
        cmd.extend(["-q", qemu_path])
    cmd.extend(["-S", str(rootfs_path)])
    cmd.extend(["-b", "/proc:proc"])
    cmd.extend(["-b", "/dev:dev"])
    cmd.extend(["-b", "/sys:sys"])
    return cmd


def run(command, env=None, binds=None, working_dir="/", timeout=None,
        rootfs_path=None, capture_output=False):
    if rootfs_path is None:
        raise RuntimeError("必须指定 rootfs_path")
    target_arch = "arm64"
    qemu_needed = detect_qemu_needed(target_arch)
    qemu_path = None
    if qemu_needed:
        qemu_path = find_qemu_aarch64_static()
    base_cmd = build_command(rootfs_path, qemu_needed, qemu_path)
    if binds:
        validated = _validate_bindings(binds)
        for host_abs, guest_str in validated:
            base_cmd.extend(["-b", f"{host_abs}:{guest_str}"])
    base_cmd.extend(["-w", working_dir])
    clean_env = _validate_env(env)
    full_env = dict(FIXED_ENV)
    full_env.update(clean_env)
    for k, v in full_env.items():
        base_cmd.extend(["-e", f"{k}={v}"])
    if not isinstance(command, (list, tuple)):
        raise RuntimeError("command 必须为数组参数")
    cmd_array = [str(c) for c in command]
    try:
        proc = subprocess.run(
            base_cmd + cmd_array,
            capture_output=capture_output,
            timeout=timeout,
        )
        return subprocess.CompletedProcess(
            args=cmd_array,
            returncode=proc.returncode,
            stdout=proc.stdout,
            stderr=proc.stderr,
        )
    except subprocess.TimeoutExpired as e:
        raise RuntimeError(f"PRoot 命令超时: {cmd_array}") from e


def run_checked(command, env=None, binds=None, working_dir="/", timeout=None,
                rootfs_path=None):
    result = run(
        command, env=env, binds=binds, working_dir=working_dir,
        timeout=timeout, rootfs_path=rootfs_path, capture_output=True,
    )
    if result.returncode != 0:
        stderr_text = result.stderr.decode("utf-8", errors="replace") if result.stderr else ""
        raise RuntimeError(
            f"PRoot 命令失败 (exit={result.returncode}): {' '.join(command)}\n{stderr_text}"
        )
    return result


def clear_env_prefix(prefix):
    to_remove = [k for k in os.environ if k.startswith(prefix)]
    for k in to_remove:
        del os.environ[k]
