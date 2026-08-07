#!/usr/bin/env python3
import argparse
import hashlib
import json
import os
import pathlib
import platform
import re
import subprocess
import sys

SCRIPT_DIR = pathlib.Path(__file__).resolve().parent
SOURCE_DIR = SCRIPT_DIR / "source"
LOCK_FILE = SCRIPT_DIR / "scripts.lock.json"
SCRIPTS_ROOT_NAME = "scripts/node"

RUNTIME_ROOT = SCRIPT_DIR.parent.parent.parent
DEFAULT_OUTPUT_DIR = RUNTIME_ROOT / "out" / "node-runtime-scripts" / "linux-arm64"

IS_WINDOWS = platform.system() == "Windows"

FORBIDDEN_SHELL = ["eval", "sh -c", "bash -c", "nohup", "setsid", "daemon", "killall", "pkill", "pidof"]
FORBIDDEN_NODE_PATHS = [
    "node.exe", "npm.cmd", "npx.cmd", "cmd /c",
    "/usr/bin/node", "/usr/local/bin/node",
]
FORBIDDEN_NPM_REFERENCES = [
    "which node", "command -v node",
]
FORBIDDEN_ANDROID_PATHS = [
    "/data/data", "/data/user", "com.termux", "TERMUX_PREFIX",
    "/sdcard", "/storage/emulated",
]

REQUIRED_SCRIPTS = [
    "amitia-node-prepare.sh",
    "amitia-node-probe.sh",
    "amitia-node-exec.sh",
    "amitia-npm-exec.sh",
    "amitia-npx-exec.sh",
    "amitia-plugin-host.sh",
    "amitia-task-host.sh",
    "probe-node-runtime.mjs",
]


def load_json(path):
    with open(path, "r", encoding="utf-8") as f:
        return json.load(f)


def sha256_file(path):
    h = hashlib.sha256()
    with open(path, "rb") as f:
        while True:
            chunk = f.read(1048576)
            if not chunk:
                break
            h.update(chunk)
    return h.hexdigest()


def verify_lock():
    if not LOCK_FILE.exists():
        return ["锁文件不存在"]
    lock = load_json(LOCK_FILE)
    issues = []
    if lock.get("nodeVersion") != "24.19.0":
        issues.append(f"lock.nodeVersion 错误: {lock.get('nodeVersion')}")
    if lock.get("npmVersion") != "11.17.0":
        issues.append(f"lock.npmVersion 错误: {lock.get('npmVersion')}")
    if lock.get("platform") != "linux":
        issues.append(f"lock.platform 错误: {lock.get('platform')}")
    if lock.get("architecture") != "arm64":
        issues.append(f"lock.architecture 错误: {lock.get('architecture')}")
    return issues


def verify_sources():
    issues = []
    all_files = []
    for root, dirs, files in os.walk(SOURCE_DIR):
        for fn in files:
            fp = pathlib.Path(root) / fn
            all_files.append(fp)
            data = fp.read_bytes()
            rel = fp.relative_to(SOURCE_DIR)
            if b"\xEF\xBB\xBF" in data[:4]:
                issues.append(f"{rel} 含 BOM")
            if b"\r\n" in data:
                issues.append(f"{rel} 含 CRLF")
            if not data.endswith(b"\n"):
                issues.append(f"{rel} 末尾无换行")
            if fn.endswith(".sh"):
                is_lib = rel.parts[0] == "lib" if rel.parts else False
                if not is_lib and not data.startswith(b"#!/bin/sh\n"):
                    issues.append(f"{rel} Shebang 错误")
                text = data.decode("utf-8", errors="replace")
                if "env sh" in text:
                    issues.append(f"{rel} 建议使用 /bin/sh 而非 env sh")
                check_forbidden = False if is_lib else True
                if check_forbidden:
                    for pat in FORBIDDEN_SHELL:
                        if pat in text:
                            issues.append(f"{rel} 含禁止关键字: {pat}")
                for pat in FORBIDDEN_NODE_PATHS:
                    if pat in text:
                        issues.append(f"{rel} 含系统路径: {pat}")
                if "system node" in text.lower():
                    issues.append(f"{rel} 引用系统 Node")
                if "NODE_PATH=" in text:
                    if "unset NODE_PATH" not in text and "# NODE_PATH" not in text:
                        issues.append(f"{rel} 设置了 NODE_PATH")
                if "NODE_OPTIONS=" in text:
                    if "unset NODE_OPTIONS" not in text and "# NODE_OPTIONS" not in text:
                        issues.append(f"{rel} 设置了 NODE_OPTIONS")
                for pat in FORBIDDEN_ANDROID_PATHS:
                    if pat in text:
                        issues.append(f"{rel} 含 Android 路径: {pat}")
            if fn == "probe-node-runtime.mjs":
                text = data.decode("utf-8", errors="replace")
                if "setTimeout" in text or "setInterval" in text:
                    issues.append(f"{rel} 含定时器")
                if "fetch(" in text or "http" in text:
                    if "node:http" not in text:
                        issues.append(f"{rel} 可能联网")
    source_basenames = [(f.name, f) for f in all_files if f.suffix in (".sh", ".mjs", ".js")]
    for required in REQUIRED_SCRIPTS:
        found = any(required == name for name, _ in source_basenames)
        if not found:
            issues.append(f"缺失脚本: {required}")
    return issues


def verify_output(output_dir):
    issues = []
    out = pathlib.Path(output_dir)
    if not out.exists():
        return ["输出目录不存在: " + str(out)]
    scripts_dir = out / SCRIPTS_ROOT_NAME
    if not scripts_dir.exists():
        issues.append("scripts/node 目录不存在")
    for required in REQUIRED_SCRIPTS:
        fp = scripts_dir / required if scripts_dir.exists() else out / required
        if not fp.exists():
            issues.append(f"缺失: {required}")
    for name in ("node-runtime-scripts.json", "file-manifest.json", "SHA256SUMS"):
        if not (out / name).exists():
            issues.append(f"{name} 缺失")
    if IS_WINDOWS:
        return issues
    for fp in out.rglob("*.sh"):
        data = fp.read_bytes()
        rel_str = str(fp.relative_to(scripts_dir)) if scripts_dir.exists() else fp.name
        parts = pathlib.PurePosixPath(rel_str).parts
        is_lib = parts[0] == "lib" if parts else False
        if not is_lib and not data.startswith(b"#!/bin/sh\n"):
            issues.append(f"{fp.name} Shebang 错误")
        if b"\r\n" in data:
            issues.append(f"{fp.name} 含 CRLF")
        mode = fp.stat().st_mode & 0o777
        if mode != 0o755:
            issues.append(f"{fp.name} 权限 {oct(mode)} != 0755")
    for fp in out.rglob("*.mjs"):
        mode = fp.stat().st_mode & 0o777
        if mode != 0o644:
            issues.append(f"{fp.name} 权限 {oct(mode)} != 0644")
    return issues


def run_static_checks(output_dir):
    checks = [
        ("锁文件", verify_lock),
        ("源文件静态分析", verify_sources),
        ("产物目录检查", lambda: verify_output(output_dir)),
    ]
    issues_all = []
    for name, fn in checks:
        try:
            res = fn()
        except Exception as e:
            res = [f"检查异常: {e}"]
        if res:
            issues_all.extend(res)
            print(f"[FAIL] {name}:")
            for i in res:
                print(f"  - {i}")
        else:
            print(f"[PASS] {name}")
    return issues_all


def can_run_sh():
    if sys.platform == "win32":
        try:
            r = subprocess.run(
                ["wsl", "--", "uname", "-s"],
                capture_output=True, text=True, timeout=10,
            )
            if r.returncode == 0:
                return True
        except Exception:
            pass
        return False
    return True


def run_runtime_checks(output_dir):
    import tempfile
    out = pathlib.Path(output_dir)
    scripts_dir = out / SCRIPTS_ROOT_NAME
    failures = []

    if not can_run_sh():
        print("[SKIP] 当前环境无 sh，跳过 Runtime 测试")
        return failures

    if not scripts_dir.exists():
        return ["脚本目录不存在"]

    runtime_root = pathlib.Path(tempfile.mkdtemp(prefix="amitia-script-verify-"))
    node_src = pathlib.Path(os.environ.get("AMITIA_NODE_DIST", ""))
    if not node_src.exists():
        node_src = RUNTIME_ROOT / "out" / "node" / "linux-arm64" / "node"
    if node_src.exists():
        (runtime_root / "node").mkdir(parents=True, exist_ok=True)
        if (node_src / "bin").exists():
            run(["cp", "-a", str(node_src), str(runtime_root / "node")])
        elif node_src.is_dir() and (node_src / "bin" / "node").exists():
            run(["cp", "-a", str(node_src), str(runtime_root / "node")])
    scripts_target = runtime_root / "scripts"
    if scripts_target.exists():
        import shutil
        shutil.rmtree(scripts_target)
    run(["cp", "-a", str(scripts_dir), str(scripts_target / "node")])

    prepare_sh = scripts_target / "node" / "amitia-node-prepare.sh"
    if prepare_sh.exists():
        cache_root = pathlib.Path(tempfile.mkdtemp(prefix="amitia-prepare-cache-"))
        tmp_root = pathlib.Path(tempfile.mkdtemp(prefix="amitia-prepare-tmp-"))
        env_prepare = {
            "AMITIA_DATA_ROOT": str(cache_root),
            "AMITIA_CACHE_ROOT": str(cache_root),
            "AMITIA_TEMP_ROOT": str(tmp_root),
            "PATH": "/usr/local/sbin:/usr/local/bin:/usr/sbin:/usr/bin:/sbin:/bin",
        }
        r = run(["/bin/sh", str(prepare_sh)], env=env_prepare)
        if r.returncode != 0:
            failures.append(f"prepare 退出码 {r.returncode}: {r.stderr}")
        else:
            print("[PASS] prepare 脚本执行成功")
            for key in ["AMITIA_NODE_HOME", "AMITIA_NODE_PREFIX", "AMITIA_NPM_CACHE", "AMITIA_NODE_TMP"]:
                if f"{key}=" not in r.stdout:
                    failures.append(f"prepare 未输出 {key}")
                else:
                    print(f"[PASS] prepare 输出 {key}")
    else:
        failures.append("prepare 脚本不存在")

    return failures


def run(cmd, env=None):
    _env = os.environ.copy()
    if env:
        _env.update(env)
    try:
        result = subprocess.run(
            cmd,
            capture_output=True,
            text=True,
            timeout=120,
            env=_env,
        )
        return result
    except Exception as e:
        class R:
            returncode = 1
            stdout = ""
            stderr = str(e)
        return R()


def parse_args():
    parser = argparse.ArgumentParser(description="验证 Node Runtime Scripts")
    parser.add_argument("--mode", choices=["static", "runtime"], default="static")
    parser.add_argument("--scripts", help="指定脚本目录")
    parser.add_argument("--runtime-root", help="指定 Runtime Root")
    parser.add_argument("--report", help="输出报告路径")
    return parser.parse_args()


def main():
    args = parse_args()
    output_dir = pathlib.Path(args.scripts).resolve() if args.scripts else DEFAULT_OUTPUT_DIR

    if args.mode == "static":
        print(f"[静态验证] 源: {SOURCE_DIR}")
        print(f"[静态验证] 输出: {output_dir}")
        issues = run_static_checks(output_dir)
        if issues:
            print(f"\n[结果] {len(issues)} 项失败")
            sys.exit(1)
        print("\n[结果] 全部通过")

    elif args.mode == "runtime":
        print(f"[Runtime 验证] Runtime Root: {args.runtime_root or '内部临时构建'}")
        failures = run_runtime_checks(output_dir)
        if failures:
            print(f"\n[结果] {len(failures)} 项失败")
            for f in failures:
                print(f"  - {f}")
            sys.exit(1)
        print("\n[结果] 全部通过")


if __name__ == "__main__":
    main()
