import argparse
import json
import os
import pathlib
import platform
import shutil
import stat
import subprocess
import sys

SCRIPT_DIR = pathlib.Path(__file__).resolve().parent
LOCK_FILE = SCRIPT_DIR / "rootfs.lock.json"
DEFAULT_OUTPUT_DIR = SCRIPT_DIR.parent.parent / "out" / "rootfs" / "ubuntu-noble-arm64"
DEFAULT_ROOT_FS_DIR = DEFAULT_OUTPUT_DIR / "rootfs"
RUNTIME_REPORTS_DIR = DEFAULT_OUTPUT_DIR / "test-reports"
DEFAULT_RUNTIME_ENV = {
    "HOME": "/home/amitia",
    "USER": "amitia",
    "LOGNAME": "amitia",
    "LANG": "C.UTF-8",
    "LC_ALL": "C.UTF-8",
    "TZ": "Etc/UTC",
    "PATH": "/usr/local/sbin:/usr/local/bin:/usr/sbin:/usr/bin:/sbin:/bin",
}


def load_json(path):
    with open(path, "r", encoding="utf-8") as f:
        return json.load(f)


def save_json(path, data):
    with open(path, "w", encoding="utf-8", newline="") as f:
        json.dump(data, f, indent=2, sort_keys=True, ensure_ascii=False)
        f.write("\n")


def sha256_file(path):
    import hashlib
    h = hashlib.sha256()
    with open(path, "rb") as f:
        while True:
            chunk = f.read(1048576)
            if not chunk:
                break
            h.update(chunk)
    return h.hexdigest()


def proot_run(rootfs_path, command, binds=None, working_dir="/", timeout=None):
    proot_path = shutil.which("proot")
    if not proot_path:
        return None
    import platform as plat
    host_machine = plat.machine().lower()
    qemu_needed = host_machine not in ("aarch64", "arm64")
    cmd = [proot_path]
    if qemu_needed:
        qemu_path = shutil.which("qemu-aarch64-static")
        if not qemu_path:
            return None
        cmd.extend(["-q", qemu_path])
    cmd.extend(["-S", str(rootfs_path)])
    cmd.extend(["-b", "/proc:proc"])
    cmd.extend(["-b", "/dev:dev"])
    cmd.extend(["-b", "/sys:sys"])
    if binds:
        for host_path, guest_path in binds:
            cmd.extend(["-b", f"{pathlib.Path(host_path).resolve()}:{guest_path}"])
    cmd.extend(["-w", working_dir])
    env_vars = ["env", "-i"]
    for k, v in DEFAULT_RUNTIME_ENV.items():
        env_vars.append(f"{k}={v}")
    full_cmd = env_vars + cmd + list(command)
    try:
        result = subprocess.run(
            full_cmd, capture_output=True, timeout=timeout
        )
        return result
    except subprocess.TimeoutExpired:
        return None


def run_static_checks(rootfs_path):
    issues = []
    checks_passed = 0
    required_dirs = [
        "bin", "boot", "dev", "etc", "home", "lib", "media", "mnt",
        "opt", "proc", "root", "run", "sbin", "srv", "sys", "tmp", "usr", "var",
    ]
    for d in required_dirs:
        if not (rootfs_path / d).exists():
            issues.append(f"缺少目录: {d}")
        else:
            checks_passed += 1
    if not (rootfs_path / "usr" / "bin" / "env").exists():
        issues.append("缺少 /usr/bin/env")
    else:
        checks_passed += 1
    ld_paths = list((rootfs_path / "lib").glob("ld-linux*.so*"))
    if not ld_paths:
        issues.append("缺少 ARM64 动态加载器")
    else:
        checks_passed += 1
    libstdcpp = list((rootfs_path / "usr" / "lib").glob("libstdc++.so*"))
    if not libstdcpp:
        issues.append("缺少 libstdc++")
    else:
        checks_passed += 1
    libgcc = list((rootfs_path / "usr" / "lib").glob("libgcc_s.so*"))
    if not libgcc:
        issues.append("缺少 libgcc")
    else:
        checks_passed += 1
    libcacerts = rootfs_path / "etc" / "ssl" / "certs" / "ca-certificates.crt"
    if not libcacerts.exists():
        issues.append("缺少 CA 证书")
    else:
        checks_passed += 1
    merged_usr_ok = True
    for link_name, target in [("bin", "usr/bin"), ("sbin", "usr/sbin"), ("lib", "usr/lib")]:
        link_path = rootfs_path / link_name
        if link_path.is_symlink():
            actual_target = os.readlink(link_path)
            if actual_target != target:
                merged_usr_ok = False
                issues.append(f"{link_name} 符号链接目标错误: {actual_target} != {target}")
    if merged_usr_ok:
        checks_passed += 1
    passwd_file = rootfs_path / "etc" / "passwd"
    passwd_ok = False
    root_ok = False
    amitia_ok = False
    if passwd_file.exists():
        content = passwd_file.read_text(encoding="utf-8")
        for line in content.splitlines():
            line = line.strip()
            if not line or line.startswith("#"):
                continue
            parts = line.split(":")
            if len(parts) >= 7:
                if parts[0] == "root" and parts[2] == "0" and parts[3] == "0":
                    root_ok = True
                if parts[0] == "amitia" and parts[2] == "1000" and parts[3] == "1000":
                    amitia_ok = True
    if root_ok and amitia_ok:
        passwd_ok = True
        checks_passed += 1
    else:
        issues.append("passwd 用户配置不正确")
    shadow_file = rootfs_path / "etc" / "shadow"
    if shadow_file.exists():
        content = shadow_file.read_text(encoding="utf-8")
        for line in content.splitlines():
            line = line.strip()
            if not line or line.startswith("#"):
                continue
            parts = line.split(":")
            if len(parts) >= 2:
                if parts[0] in ("root", "amitia") and parts[1] in ("!", "*", "!*"):
                    checks_passed += 1
                elif parts[0] in ("root", "amitia"):
                    issues.append(f"用户 {parts[0]} 密码未锁定")
    tmp = rootfs_path / "tmp"
    if tmp.exists():
        st = tmp.stat()
        if stat.S_IMODE(st.st_mode) == 0o1777:
            checks_passed += 1
        else:
            issues.append(f"/tmp 权限不正确: {oct(stat.S_IMODE(st.st_mode))}")
    for check_dir in ["dev", "proc", "sys"]:
        for item in (rootfs_path / check_dir).iterdir() if (rootfs_path / check_dir).exists() else []:
            try:
                st = item.stat()
                if stat.S_ISFIFO(st.st_mode) or stat.S_ISBLK(st.st_mode) or stat.S_ISCHR(st.st_mode):
                    if item.name in ("null", "zero", "random", "urandom", "tty", "ptmx", "pts"):
                        pass
                    else:
                        issues.append(f"{check_dir} 中存在意外特殊节点: {item.name}")
            except OSError:
                pass
    if (rootfs_path / "opt" / "amitia").exists():
        issues.append("检测到 /opt/amitia")
    if (rootfs_path / "var" / "lib" / "amitia").exists():
        issues.append("检测到 /var/lib/amitia")
    if (rootfs_path / "etc" / "machine-id").exists():
        mid_content = (rootfs_path / "etc" / "machine-id").read_text().strip()
        if mid_content:
            issues.append("machine-id 非空")
        else:
            checks_passed += 1
    policy_rc = rootfs_path / "usr" / "sbin" / "policy-rc.d"
    if policy_rc.exists():
        if policy_rc.stat().st_mode & 0o755 == 0o755:
            checks_passed += 1
        else:
            issues.append("policy-rc.d 权限不正确")
    forbidden_files = []
    for fp in rootfs_path.rglob("*"):
        if fp.is_file() and not fp.is_symlink():
            try:
                st = fp.stat()
                if st.st_mode & stat.S_ISUID:
                    forbidden_files.append(f"SetUID: {fp.relative_to(rootfs_path)}")
                    if len(forbidden_files) > 10:
                        break
                if st.st_mode & stat.S_ISGID:
                    forbidden_files.append(f"SetGID: {fp.relative_to(rootfs_path)}")
                    if len(forbidden_files) > 10:
                        break
            except OSError:
                pass
    if forbidden_files:
        issues.extend(forbidden_files[:10])
    else:
        checks_passed += 1
    return issues, checks_passed


def run_mode_static(args):
    lock = load_json(LOCK_FILE)
    root_fs_dir = pathlib.Path(args.rootfs_dir).resolve() if args.root_fs_dir else DEFAULT_ROOT_FS_DIR
    print("[静态验证] 开始")
    print(f"  RootFS 目录: {root_fs_dir}")
    print(f"  固定版本: {lock['release']} ({lock['codename']})")
    print(f"  架构: {lock['architecture']}")
    if not root_fs_dir.exists():
        raise RuntimeError(f"RootFS 目录不存在: {root_fs_dir}")
    print("\n[检查 1/5] 基础结构检查")
    missing_dirs = []
    for d in ["bin", "etc", "lib", "usr", "var"]:
        if not (root_fs_dir / d).exists():
            missing_dirs.append(d)
    if missing_dirs:
        raise RuntimeError(f"缺少关键目录: {missing_dirs}")
    print("  基础结构检查通过")
    print("\n[检查 2/5] 关键文件检查")
    if not (root_fs_dir / "bin" / "bash").exists():
        raise RuntimeError("缺少 /bin/bash")
    print("  关键文件检查通过")
    print("\n[检查 3/5] 用户和权限检查")
    passwd_file = root_fs_dir / "etc" / "passwd"
    if not passwd_file.exists():
        raise RuntimeError("缺少 /etc/passwd")
    print("  用户和权限检查通过")
    print("\n[检查 4/5] 安全清理检查")
    if (root_fs_dir / "var" / "cache" / "apt" / "archives").exists():
        deb_files = list((root_fs_dir / "var" / "cache" / "apt" / "archives").glob("*.deb"))
        if deb_files:
            raise RuntimeError("APT 缓存未清理")
    print("  安全清理检查通过")
    print("\n[检查 5/5] 运行库检查")
    if not list((root_fs_dir / "usr" / "lib").glob("libstdc++.so*")):
        raise RuntimeError("缺少 libstdc++")
    print("  运行库检查通过")
    print("\n[详细检查] 执行完整静态检查")
    issues, checks_passed = run_static_checks(root_fs_dir)
    print(f"  通过: {checks_passed}")
    if issues:
        print(f"  发现问题: {len(issues)}")
        for issue in issues[:20]:
            print(f"    - {issue}")
    report = {
        "mode": "static",
        "result": "pass" if not issues else "fail",
        "architecture": lock["architecture"],
        "release": lock["release"],
        "checksPassed": checks_passed,
        "issuesCount": len(issues),
        "issues": issues[:50],
    }
    RUNTIME_REPORTS_DIR.mkdir(parents=True, exist_ok=True)
    report_path = RUNTIME_REPORTS_DIR / "static-report.json"
    save_json(report_path, report)
    print(f"\n[报告] 已生成: {report_path}")
    if issues:
        raise RuntimeError(f"静态验证失败: {len(issues)} 个问题")
    print("\n[静态验证] 通过")


def run_mode_runtime(args):
    lock = load_json(LOCK_FILE)
    root_fs_dir = pathlib.Path(args.root_fs_dir).resolve() if args.root_fs_dir else DEFAULT_ROOT_FS_DIR
    print("[Runtime 验证] 开始")
    print(f"  RootFS 目录: {root_fs_dir}")
    print(f"  固定版本: {lock['release']} ({lock['codename']})")
    print(f"  架构: {lock['architecture']}")
    if not root_fs_dir.exists():
        raise RuntimeError(f"RootFS 目录不存在: {root_fs_dir}")
    if not shutil.which("proot"):
        raise RuntimeError("未找到 proot")
    qemu_needed = platform.machine().lower() not in ("aarch64", "arm64")
    if qemu_needed and not shutil.which("qemu-aarch64-static"):
        raise RuntimeError("x86_64 环境需要 qemu-aarch64-static")
    print("\n[检查] 测试基础命令")
    test_results = []
    basic_commands = [
        (["/bin/bash", "--version"], "bash 版本"),
        (["/usr/bin/env"], "env 可用"),
        (["/usr/bin/id"], "id 命令"),
        (["/usr/bin/uname", "-m"], "架构"),
        (["/usr/bin/getconf", "PAGESIZE"], "页面大小"),
        (["/usr/bin/ps"], "ps 命令"),
        (["/usr/bin/find", "--version"], "find 命令"),
        (["/usr/bin/grep", "--version"], "grep 命令"),
        (["/usr/bin/tar", "--version"], "tar 命令"),
        (["/usr/bin/xz", "--version"], "xz 命令"),
    ]
    for cmd, desc in basic_commands:
        result = proot_run(root_fs_dir, cmd, timeout=60)
        if result and result.returncode == 0:
            output = result.stdout.decode("utf-8", errors="replace").strip()[:100]
            test_results.append({"command": " ".join(cmd), "description": desc, "status": "pass", "output": output})
            print(f"  [通过] {desc}")
        else:
            exit_code = result.returncode if result else "N/A"
            test_results.append({"command": " ".join(cmd), "description": desc, "status": "fail", "exitCode": exit_code})
            print(f"  [失败] {desc} (exit={exit_code})")
    print("\n[检查] C.UTF-8 环境测试")
    utf8_script = """
echo '中文测试 UTF-8 文件名'
mkdir -p /tmp/amitia-utf8-test
echo 'Hello 中文' > /tmp/amitia-utf8-test/测试文件.txt
cat /tmp/amitia-utf8-test/测试文件.txt
rm -rf /tmp/amitia-utf8-test
echo 'UTF-8 test complete'
"""
    result = proot_run(root_fs_dir, ["/bin/bash", "-c", utf8_script], timeout=60)
    utf8_pass = False
    if result and result.returncode == 0:
        output = result.stdout.decode("utf-8", errors="replace")
        if "中文测试" in output and "Hello 中文" in output:
            utf8_pass = True
            print("  [通过] UTF-8 中文读写")
        else:
            print("  [失败] UTF-8 输出不正确")
    else:
        print("  [失败] UTF-8 测试命令失败")
    print("\n[检查] /tmp 可写测试")
    tmp_script = """
mkdir -p /tmp/amitia-write-test
echo 'write test' > /tmp/amitia-write-test/test.txt
cat /tmp/amitia-write-test/test.txt
rm -rf /tmp/amitia-write-test
echo '/tmp write test complete'
"""
    result = proot_run(root_fs_dir, ["/bin/bash", "-c", tmp_script], timeout=60)
    tmp_pass = False
    if result and result.returncode == 0:
        tmp_pass = True
        print("  [通过] /tmp 可写")
    else:
        print("  [失败] /tmp 写入失败")
    print("\n[检查] /home/amitia 可写测试")
    home_script = """
su - amitia -c 'mkdir -p /home/amitia/test && echo test > /home/amitia/test/file.txt && cat /home/amitia/test/file.txt && rm -rf /home/amitia/test'
"""
    result = proot_run(root_fs_dir, ["/bin/bash", "-c", home_script], timeout=60)
    home_pass = result and result.returncode == 0
    if home_pass:
        print("  [通过] /home/amitia 可写")
    else:
        print("  [失败] /home/amitia 写入失败")
    print("\n[检查] /proc 绑定验证")
    proc_script = """
cat /proc/version
cat /proc/cpuinfo
echo '/proc test complete'
"""
    result = proot_run(root_fs_dir, ["/bin/bash", "-c", proc_script], timeout=60)
    proc_pass = False
    if result and result.returncode == 0:
        output = result.stdout.decode("utf-8", errors="replace")
        if "processor" in output or "cpu" in output.lower():
            proc_pass = True
            print("  [通过] /proc 可读取")
        else:
            print("  [警告] /proc 输出不完整")
    else:
        print("  [失败] /proc 读取失败")
    print("\n[检查] 设备测试")
    dev_script = """
test -c /dev/null && echo '/dev/null ok'
test -c /dev/urandom && head -c 8 /dev/urandom > /dev/null && echo '/dev/urandom ok'
echo 'dev test complete'
"""
    result = proot_run(root_fs_dir, ["/bin/bash", "-c", dev_script], timeout=60)
    dev_pass = result and result.returncode == 0
    if dev_pass:
        print("  [通过] 设备节点可用")
    else:
        print("  [失败] 设备节点不可用")
    print("\n[检查] 时区验证")
    tz_script = """
echo "TZ=$TZ"
date +%Z
echo 'tz test complete'
"""
    result = proot_run(root_fs_dir, ["/bin/bash", "-c", tz_script], timeout=60)
    tz_pass = False
    if result and result.returncode == 0:
        output = result.stdout.decode("utf-8", errors="replace")
        if "UTC" in output:
            tz_pass = True
            print("  [通过] 时区为 UTC")
        else:
            print("  [失败] 时区不是 UTC")
    else:
        print("  [失败] 时区测试失败")
    print("\n[检查] 回环网络测试")
    net_script = """
if [ -d /sys/class/net/lo ]; then
    echo 'loopback interface exists'
fi
cat /etc/hosts | head -5
echo 'network test complete'
"""
    result = proot_run(root_fs_dir, ["/bin/bash", "-c", net_script], timeout=60)
    net_pass = result and result.returncode == 0
    if net_pass:
        print("  [通过] 网络配置可用")
    else:
        print("  [失败] 网络配置不可用")
    node_compat_status = "not_tested"
    node_compat_output = ""
    if args.node_distribution:
        node_dist = pathlib.Path(args.node_distribution)
        if node_dist.exists():
            print("\n[检查] Node 兼容预验证")
            node_script = f"""
ls -la /tmp/amitia-node-test/
/tmp/amitia-node-test/bin/node --version 2>&1
echo 'node compat test complete'
"""
            node_tmp = root_fs_dir / "tmp" / "amitia-node-test"
            if node_tmp.exists():
                shutil.rmtree(node_tmp, ignore_errors=True)
            shutil.copytree(str(node_dist), str(node_tmp))
            result = proot_run(root_fs_dir, ["/bin/bash", "-c", node_script], timeout=60)
            if result and result.returncode == 0:
                node_output = result.stdout.decode("utf-8", errors="replace")
                node_compat_output = node_output
                if "v" in node_output:
                    node_compat_status = "pass"
                    print("  [通过] Node 可启动")
                else:
                    node_compat_status = "fail"
                    print("  [失败] Node 启动异常")
            else:
                node_compat_status = "fail"
                print("  [失败] Node 启动失败")
            shutil.rmtree(node_tmp, ignore_errors=True)
        else:
            print(f"\n[信息] Node 分发目录不存在，跳过: {args.node_distribution}")
            node_compat_status = "directory_missing"
    qdrant_compat_status = "not_tested"
    qdrant_compat_output = ""
    if args.qdrant_distribution:
        qdrant_dist = pathlib.Path(args.qdrant_distribution)
        if qdrant_dist.exists():
            print("\n[检查] Qdrant 兼容预验证")
            qdrant_script = f"""
ls -la /tmp/amitia-qdrant-test/
/tmp/amitia-qdrant-test/bin/qdrant --version 2>&1
echo 'qdrant compat test complete'
"""
            qdrant_tmp = root_fs_dir / "tmp" / "amitia-qdrant-test"
            if qdrant_tmp.exists():
                shutil.rmtree(qdrant_tmp, ignore_errors=True)
            shutil.copytree(str(qdrant_dist), str(qdrant_tmp))
            result = proot_run(root_fs_dir, ["/bin/bash", "-c", qdrant_script], timeout=60)
            if result and result.returncode == 0:
                qdrant_output = result.stdout.decode("utf-8", errors="replace")
                qdrant_compat_output = qdrant_output
                if "qdrant" in qdrant_output.lower():
                    qdrant_compat_status = "pass"
                    print("  [通过] Qdrant 可启动版本命令")
                else:
                    qdrant_compat_status = "fail"
                    print("  [失败] Qdrant 启动异常")
            else:
                qdrant_compat_status = "fail"
                print("  [失败] Qdrant 启动失败")
            shutil.rmtree(qdrant_tmp, ignore_errors=True)
        else:
            print(f"\n[信息] Qdrant 分发目录不存在，跳过: {args.qdrant_distribution}")
            qdrant_compat_status = "directory_missing"
    print("\n[总结] Runtime 验证结果")
    all_tests_pass = all(t["status"] == "pass" for t in test_results) and utf8_pass and tmp_pass
    architecture_result = platform.machine().lower()
    if any("arm64" in t.get("output", "") or "aarch64" in t.get("output", "") for t in test_results):
        architecture_result = "aarch64"
    print(f"  主机架构: {platform.machine()}")
    print(f"  PRoot 版本: {shutil.which('proot')}")
    if qemu_needed:
        print(f"  QEMU 版本: {shutil.which('qemu-aarch64-static')}")
    print(f"  测试命令: {len(test_results)} / {sum(1 for t in test_results if t['status'] == 'pass')} 通过")
    report = {
        "mode": "runtime",
        "result": "pass" if all_tests_pass else "fail",
        "hostArchitecture": platform.machine(),
        "targetArchitecture": architecture_result,
        "guestPlatform": lock["guestPlatform"],
        "qemuNeeded": qemu_needed,
        "testResults": test_results,
        "utf8Test": utf8_pass,
        "tmpWritable": tmp_pass,
        "homeWritable": home_pass,
        "procReadable": proc_pass,
        "devicesAvailable": dev_pass,
        "timezoneUTC": tz_pass,
        "networkConfigAvailable": net_pass,
        "nodeCompatibility": {
            "status": node_compat_status,
            "output": node_compat_output[:500],
        },
        "qdrantCompatibility": {
            "status": qdrant_compat_status,
            "output": qdrant_compat_output[:500],
        },
    }
    RUNTIME_REPORTS_DIR.mkdir(parents=True, exist_ok=True)
    report_path = RUNTIME_REPORTS_DIR / "proot-report.json"
    save_json(report_path, report)
    print(f"\n[报告] 已生成: {report_path}")
    if not all_tests_pass:
        raise RuntimeError("Runtime 验证失败")
    print("\n[Runtime 验证] 通过")


def parse_args():
    parser = argparse.ArgumentParser(description="验证 Ubuntu ARM64 RootFS")
    parser.add_argument("--mode", choices=["static", "runtime"], required=True, help="验证模式")
    parser.add_argument("--rootfs-dir", help="自定义 RootFS 目录路径")
    parser.add_argument("--node-distribution", help="Node 分发目录路径（用于兼容验证）")
    parser.add_argument("--qdrant-distribution", help="Qdrant 分发目录路径（用于兼容验证）")
    parser.add_argument("--enable-network-test", action="store_true", help="启用网络测试（默认关闭）")
    return parser.parse_args()


def main():
    args = parse_args()
    try:
        if args.mode == "static":
            run_mode_static(args)
        elif args.mode == "runtime":
            run_mode_runtime(args)
    except Exception as e:
        print(f"[错误] {e}", file=sys.stderr)
        sys.exit(1)


if __name__ == "__main__":
    main()
