import argparse
import hashlib
import json
import lzma
import os
import pathlib
import shutil
import stat
import subprocess
import sys
import tarfile
import tempfile
import time

SCRIPT_DIR = pathlib.Path(__file__).resolve().parent
LOCK_FILE = SCRIPT_DIR / "rootfs.lock.json"
REQUESTED_PACKAGES_FILE = SCRIPT_DIR / "packages.requested.json"
PACKAGES_LOCK_FILE = SCRIPT_DIR / "packages.lock.json"
OVERLAYS_DIR = SCRIPT_DIR / "overlays"
DEFAULT_CACHE_DIR = SCRIPT_DIR / ".cache"
DEFAULT_WORK_DIR = SCRIPT_DIR / ".work"
DEFAULT_OUTPUT_DIR = SCRIPT_DIR.parent.parent / "out" / "rootfs" / "ubuntu-noble-arm64"

from archive import (
    create_deterministic_tar,
    write_file_manifest,
    write_sha256sums,
    sha256_file,
    verify_archive_determinism,
    verify_archive_security,
    verify_no_extra_rootfs,
)
from filesystem import (
    load_lock,
    sha256_file as sha256_file_util,
    safe_extract_archive,
    apply_overlay,
    fix_permissions,
    create_mount_points,
    setup_users,
    cleanup_cache,
    scan_forbidden_files,
    build_file_manifest,
    check_base_structure,
    check_merged_usr,
)

DEFAULT_BUILD_ENV = {
    "DEBIAN_FRONTEND": "noninteractive",
    "LANG": "C.UTF-8",
    "LC_ALL": "C.UTF-8",
    "TZ": "Etc/UTC",
    "HOME": "/root",
    "PATH": "/usr/local/sbin:/usr/local/bin:/usr/sbin:/usr/bin:/sbin:/bin",
    "APT_LISTCHANGES_FRONTEND": "none",
    "UCF_FORCE_CONFFOLD": "1",
}


def load_json(path):
    with open(path, "r", encoding="utf-8") as f:
        return json.load(f)


def save_json(path, data):
    with open(path, "w", encoding="utf-8", newline="") as f:
        json.dump(data, f, indent=2, sort_keys=True, ensure_ascii=False)
        f.write("\n")


def find_proot():
    proot_path = shutil.which("proot")
    if proot_path:
        return proot_path
    raise RuntimeError("未找到 proot 可执行文件")


def find_qemu_needed():
    import platform
    host_machine = platform.machine().lower()
    return host_machine not in ("aarch64", "arm64")


def find_qemu_aarch64_static():
    qemu_path = shutil.which("qemu-aarch64-static")
    if qemu_path:
        return qemu_path
    raise RuntimeError("未找到 qemu-aarch64-static")


def proot_run(rootfs_path, command, env=None, binds=None, working_dir="/", timeout=None):
    proot_path = find_proot()
    qemu_needed = find_qemu_needed()
    cmd = [proot_path]
    if qemu_needed:
        qemu_path = find_qemu_aarch64_static()
        cmd.extend(["-q", qemu_path])
    cmd.extend(["-S", str(rootfs_path)])
    cmd.extend(["-b", "/proc:proc"])
    cmd.extend(["-b", "/dev:dev"])
    cmd.extend(["-b", "/sys:sys"])
    if binds:
        for host_path, guest_path in binds:
            cmd.extend(["-b", f"{pathlib.Path(host_path).resolve()}:{guest_path}"])
    cmd.extend(["-w", working_dir])
    clean_env = dict(DEFAULT_BUILD_ENV)
    if env:
        blocked_keys = {"HOME", "USER", "PATH", "http_proxy", "https_proxy", "GOPATH", "GOROOT", "NODE_PATH", "ANDROID_HOME"}
        for k, v in env.items():
            if k not in blocked_keys:
                clean_env[k] = v
    for k, v in clean_env.items():
        cmd.extend(["-e", f"{k}={v}"])
    if not isinstance(command, (list, tuple)):
        raise RuntimeError("command 必须为数组参数")
    cmd.extend([str(c) for c in command])
    try:
        result = subprocess.run(cmd, capture_output=True, timeout=timeout)
        return subprocess.CompletedProcess(
            args=command,
            returncode=result.returncode,
            stdout=result.stdout,
            stderr=result.stderr,
        )
    except subprocess.TimeoutExpired as e:
        raise RuntimeError(f"PRoot 命令超时: {command}") from e


def proot_run_checked(rootfs_path, command, env=None, binds=None, working_dir="/", timeout=None):
    result = proot_run(rootfs_path, command, env=env, binds=binds, working_dir=working_dir, timeout=timeout)
    if result.returncode != 0:
        stderr_text = result.stderr.decode("utf-8", errors="replace") if result.stderr else ""
        raise RuntimeError(
            f"PRoot 命令失败 (exit={result.returncode}): {' '.join(command)}\n{stderr_text}"
        )
    return result


def validate_base_archive(base_archive_path, lock):
    if not base_archive_path.exists():
        raise RuntimeError(f"Base 归档不存在: {base_archive_path}")
    actual = sha256_file(base_archive_path)
    expected = lock["baseArchiveSha256"]
    if actual != expected:
        raise RuntimeError(f"Base 归档 SHA 不匹配: {actual} != {expected}")


def verify_packages_lock(packages_lock, lock, cache_dir):
    snapshot = packages_lock.get("aptSnapshot")
    if snapshot != lock["aptSnapshot"]:
        raise RuntimeError(f"Snapshot 不匹配: {snapshot} != {lock['aptSnapshot']}")
    if packages_lock.get("codename") != lock["codename"]:
        raise RuntimeError(f"Codename 不匹配: {packages_lock.get('codename')}")
    if packages_lock.get("architecture") != lock["architecture"]:
        raise RuntimeError(f"Architecture 不匹配: {packages_lock.get('architecture')}")
    resolved = packages_lock.get("resolvedPackages", [])
    if not resolved:
        raise RuntimeError("resolvedPackages 为空")
    deb_dir = cache_dir / "apt" / "archives"
    if not deb_dir.exists():
        raise RuntimeError(f"APT 归档目录不存在: {deb_dir}")
    deb_files = {p.name: p for p in deb_dir.glob("*.deb")}
    issues = []
    for pkg in resolved:
        filename = pkg["filename"]
        expected_sha256 = pkg["sha256"]
        expected_size = pkg.get("size", 0)
        if filename not in deb_files:
            issues.append(f"缺少 .deb: {filename}")
            continue
        if deb_files[filename].stat().st_size != expected_size and expected_size > 0:
            issues.append(f"大小不匹配: {filename}")
        with open(deb_files[filename], "rb") as f:
            actual_sha256 = hashlib.sha256(f.read()).hexdigest()
        if actual_sha256 != expected_sha256:
            issues.append(f"SHA 不匹配: {filename}")
    for pkg in resolved:
        if pkg["architecture"] != "arm64":
            issues.append(f"非 arm64 架构: {pkg['name']} {pkg['architecture']}")
    if issues:
        for issue in issues:
            print(f"[问题] {issue}")
        raise RuntimeError(f"Package 锁验证失败: {len(issues)} 个问题")


def install_packages_offline(rootfs_path, deb_dir, proot_timeout=600):
    apt_conf_dir = rootfs_path / "etc" / "apt"
    apt_conf_dir.mkdir(parents=True, exist_ok=True)
    archives_dir = rootfs_path / "var" / "cache" / "apt" / "archives"
    archives_dir.mkdir(parents=True, exist_ok=True)
    for deb_path in deb_dir.glob("*.deb"):
        dest = archives_dir / deb_path.name
        shutil.copy2(str(deb_path), str(dest))
    policy_path = rootfs_path / "usr" / "sbin" / "policy-rc.d"
    if not policy_path.exists():
        policy_path.parent.mkdir(parents=True, exist_ok=True)
        with open(policy_path, "w", encoding="utf-8") as f:
            f.write("#!/bin/sh\nexit 101\n")
        os.chmod(policy_path, 0o755)
    apt_conf_content = """APT::Install-Recommends "false";
APT::Install-Suggests "false";
Acquire::Languages "none";
Acquire::Retries "3";
Acquire::http::Timeout "120";
"""
    with open(apt_conf_dir / "99amitia-offline", "w", encoding="utf-8") as f:
        f.write(apt_conf_content)
    proot_run_checked(
        rootfs_path,
        ["apt-get", "update"],
        timeout=proot_timeout,
    )
    install_cmd = ["apt-get", "install", "--no-download", "--fix-broken", "-y", "--allow-downgrades"]
    deb_names = sorted([p.name for p in archives_dir.glob("*.deb")])
    deb_names_no_version = []
    for deb_name in deb_names:
        parts = deb_name.split("_")
        if len(parts) >= 1:
            deb_names_no_version.append(parts[0])
    install_cmd.extend(deb_names_no_version)
    proot_run_checked(
        rootfs_path,
        install_cmd,
        timeout=proot_timeout,
    )
    proot_run_checked(
        rootfs_path,
        ["dpkg", "--configure", "-a"],
        timeout=proot_timeout,
    )
    result = proot_run_checked(
        rootfs_path,
        ["dpkg-query", "-W", "-f", "${Package}:${Architecture}:${Version}:${Status}\n"],
    )
    dpkg_status = result.stdout.decode("utf-8", errors="replace")
    for line in dpkg_status.splitlines():
        if not line.strip():
            continue
        parts = line.split(":")
        if len(parts) == 4:
            pkg_name, arch, version, status = parts
            if "installed" not in status:
                raise RuntimeError(f"Package 未正确安装: {pkg_name} {status}")


def create_runtime_json(output_dir, lock):
    runtime_json = {
        "schemaVersion": 1,
        "componentId": lock["componentId"],
        "distribution": lock["distribution"],
        "flavor": lock["flavor"],
        "release": lock["release"],
        "codename": lock["codename"],
        "architecture": lock["architecture"],
        "guestPlatform": lock["guestPlatform"],
        "runtimeKind": lock["runtimeKind"],
        "baseArchive": {
            "name": lock["baseArchiveName"],
            "sha256": lock["baseArchiveSha256"],
        },
        "aptSnapshot": lock["aptSnapshot"],
        "defaultLocale": lock["defaultLocale"],
        "defaultTimezone": lock["defaultTimezone"],
        "defaultUser": {
            "name": "amitia",
            "uid": 1000,
            "gid": 1000,
            "home": "/home/amitia",
        },
        "entrypoint": "/bin/bash",
    }
    out = output_dir / "rootfs-runtime.json"
    save_json(out, runtime_json)
    return out


def create_package_manifest(output_dir, lock):
    output_dir = pathlib.Path(output_dir)
    manifest = []
    out = output_dir / "package-manifest.json"
    save_json(out, manifest)
    return out


def run_static_checks(rootfs_path):
    issues = []
    required_dirs = [
        "bin", "boot", "dev", "etc", "home", "lib", "media", "mnt",
        "opt", "proc", "root", "run", "sbin", "srv", "sys", "tmp", "usr", "var",
    ]
    for d in required_dirs:
        if not (rootfs_path / d).exists():
            issues.append(f"缺少目录: {d}")
    if not (rootfs_path / "bin" / "bash").exists():
        issues.append("缺少 /bin/bash")
    if not (rootfs_path / "usr" / "bin" / "env").exists():
        issues.append("缺少 /usr/bin/env")
    ld_paths = [
        "lib/ld-linux-aarch64.so.1",
        "lib/ld-linux.so.3",
        "usr/lib/ld-linux-aarch64.so.1",
    ]
    ld_found = any((rootfs_path / p).exists() for p in ld_paths)
    if not ld_found:
        lib_dir = rootfs_path / "lib"
        ld_any = any(f.name.startswith("ld-linux") for f in lib_dir.iterdir() if f.is_file()) if lib_dir.exists() else False
        if not ld_any:
            issues.append("缺少 ARM64 动态加载器")
    libstdcpp = list((rootfs_path / "usr" / "lib").glob("libstdc++.so*"))
    if not libstdcpp:
        issues.append("缺少 libstdc++")
    libgcc = list((rootfs_path / "usr" / "lib").glob("libgcc_s.so*"))
    if not libgcc:
        issues.append("缺少 libgcc")
    libcacerts = rootfs_path / "etc" / "ssl" / "certs" / "ca-certificates.crt"
    if not libcacerts.exists():
        issues.append("缺少 CA 证书")
    if not (rootfs_path / "tmp").exists() or not os.access(rootfs_path / "tmp", os.W_OK):
        issues.append("/tmp 目录不存在或不可写")
    if not (rootfs_path / "var" / "tmp").exists():
        issues.append("/var/tmp 目录不存在")
    if (rootfs_path / "usr" / "bin" / "node").exists():
        issues.append("检测到 Node 二进制文件")
    if (rootfs_path / "opt" / "amitia").exists():
        issues.append("检测到 /opt/amitia")
    if (rootfs_path / "var" / "lib" / "amitia").exists():
        issues.append("检测到 /var/lib/amitia")
    if (rootfs_path / "var" / "log" / "amitia").exists():
        issues.append("检测到 /var/log/amitia")
    return issues


def run_build(args):
    lock = load_json(LOCK_FILE)
    requested = load_json(REQUESTED_PACKAGES_FILE)
    packages_lock = load_json(PACKAGES_LOCK_FILE)
    print(f"[信息] Ubuntu {lock['release']} ({lock['codename']})")
    print(f"[信息] 架构 {lock['architecture']}")
    print(f"[信息] APT Snapshot {lock['aptSnapshot']}")
    cache_dir = pathlib.Path(args.cache_dir).resolve() if args.cache_dir else DEFAULT_CACHE_DIR
    work_dir = pathlib.Path(args.work_dir).resolve() if args.work_dir else DEFAULT_WORK_DIR
    output_dir = pathlib.Path(args.output_dir).resolve() if args.output_dir else DEFAULT_OUTPUT_DIR
    cache_dir.mkdir(parents=True, exist_ok=True)
    work_dir.mkdir(parents=True, exist_ok=True)
    base_archive_path = cache_dir / "base" / lock["baseArchiveName"]
    deb_archive_dir = cache_dir / "apt" / "archives"
    print("[校验] 验证 Base 归档")
    validate_base_archive(base_archive_path, lock)
    print("  Base 归档 SHA 校验通过")
    print("[校验] 验证 Package 锁")
    verify_packages_lock(packages_lock, lock, cache_dir)
    print("  Package 锁校验通过")
    if args.clean and output_dir.exists():
        print(f"[清理] 删除旧输出: {output_dir}")
        shutil.rmtree(output_dir)
    tmp_output = output_dir.with_name(output_dir.name + ".partial")
    if tmp_output.exists():
        shutil.rmtree(tmp_output)
    tmp_output.mkdir(parents=True, exist_ok=True)
    work_root = work_dir / "build"
    if work_root.exists():
        shutil.rmtree(work_root)
    work_root.mkdir(parents=True, exist_ok=True)
    try:
        print("[构建] 安全解压 Ubuntu Base")
        rootfs_work = work_root / "rootfs"
        if rootfs_work.exists():
            shutil.rmtree(rootfs_work)
        safe_extract_archive(base_archive_path, rootfs_work)
        extracted_root = rootfs_work
        subdirs = [d for d in rootfs_work.iterdir() if d.is_dir()]
        if len(subdirs) == 1:
            for item in subdirs[0].iterdir():
                dest = rootfs_work / item.name
                if dest.exists():
                    if dest.is_dir():
                        shutil.rmtree(dest)
                    else:
                        dest.unlink()
                shutil.move(str(item), str(dest))
            if not any(subdirs[0].iterdir()):
                subdirs[0].rmdir()
        missing_dirs = check_base_structure(extracted_root)
        if missing_dirs:
            raise RuntimeError(f"解压后缺少关键目录: {missing_dirs}")
        print("  Ubuntu Base 解压完成")
        print("[构建] 创建标准挂载点")
        create_mount_points(extracted_root)
        print("[构建] 安装锁定 Package")
        install_packages_offline(extracted_root, deb_archive_dir)
        print("  Package 安装完成")
        print("[构建] 应用 Overlay")
        apply_overlay(OVERLAYS_DIR, extracted_root)
        print("  Overlay 应用完成")
        print("[构建] 配置用户")
        setup_users(extracted_root, proot_run)
        print("  用户配置完成")
        print("[构建] 清理缓存和日志")
        cleanup_cache(extracted_root)
        print("  清理完成")
        print("[构建] 修复权限")
        fix_permissions(extracted_root)
        print("  权限修复完成")
        print("[构建] 静态检查")
        issues = run_static_checks(extracted_root)
        if issues:
            for issue in issues:
                print(f"  [问题] {issue}")
            raise RuntimeError(f"静态检查失败: {len(issues)} 个问题")
        print("  静态检查通过")
        print("[构建] 检查系统配置")
        usr_issues = check_merged_usr(extracted_root)
        if usr_issues:
            for issue in usr_issues:
                print(f"  [问题] {issue}")
        print("[打包] 生成归档")
        final_archive_name = f"amitia-ubuntu-base-{lock['release']}-{lock['architecture']}.tar.xz"
        runtime_json_path = create_runtime_json(tmp_output, lock)
        create_package_manifest(tmp_output, lock)
        manifest_path = write_file_manifest(tmp_output, extracted_root)
        sha256sums_path = write_sha256sums(
            tmp_output.parent,
            [final_archive_name, runtime_json_path.name, manifest_path.name, "package-manifest.json"],
        )
        final_archive_path = tmp_output.parent / final_archive_name
        create_deterministic_tar(extracted_root, final_archive_path)
        print(f"  归档已生成: {final_archive_path}")
        print("[验证] 归档安全性检查")
        archive_issues = verify_archive_security(final_archive_path)
        if archive_issues:
            for issue in archive_issues:
                print(f"  [问题] {issue}")
            raise RuntimeError(f"归档安全检查失败: {len(archive_issues)} 个问题")
        if verify_no_extra_rootfs(final_archive_path):
            raise RuntimeError("归档包含多余的 rootfs/ 顶层目录")
        print("  归档安全检查通过")
        print("[发布] 原子替换输出")
        if output_dir.exists():
            backup = output_dir.with_name(output_dir.name + ".old")
            if backup.exists():
                shutil.rmtree(backup, ignore_errors=True)
            output_dir.rename(backup)
            shutil.rmtree(backup, ignore_errors=True)
        tmp_output.rename(output_dir)
        final_sha = sha256_file(final_archive_path)
        print(f"[SHA] {final_sha}")
        print("[完成] Ubuntu RootFS 构建成功")
        print(f"[产物] {output_dir / final_archive_name}")
        print(f"[元数据] {output_dir / 'rootfs-runtime.json'}")
        print(f"[清单] {output_dir / 'file-manifest.json'}")
        print(f"[校验] {output_dir / 'SHA256SUMS'}")
        if not args.skip_runtime_test:
            print("[信息] 请运行 verify.py --mode runtime 执行 PRoot Runtime 验证")
    except Exception:
        if tmp_output.exists():
            shutil.rmtree(tmp_output, ignore_errors=True)
        raise
    finally:
        if work_root.exists():
            shutil.rmtree(work_root, ignore_errors=True)


def parse_args():
    parser = argparse.ArgumentParser(description="构建 Ubuntu ARM64 PRoot 基础 RootFS")
    parser.add_argument("--clean", action="store_true", help="清理后重新构建")
    parser.add_argument("--offline", action="store_true", help="离线模式")
    parser.add_argument("--cache-dir", help="自定义缓存目录")
    parser.add_argument("--work-dir", help="自定义临时工作目录")
    parser.add_argument("--output-dir", help="自定义输出目录")
    parser.add_argument("--source-archive", help="指定 Base 归档路径")
    parser.add_argument("--skip-runtime-test", action="store_true", help="跳过运行时测试")
    parser.add_argument("--keep-work-dir", action="store_true", help="保留临时工作目录")
    return parser.parse_args()


def main():
    args = parse_args()
    try:
        run_build(args)
    except Exception as e:
        print(f"[错误] {e}", file=sys.stderr)
        sys.exit(1)


if __name__ == "__main__":
    main()
