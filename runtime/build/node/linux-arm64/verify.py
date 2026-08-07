import argparse
import hashlib
import json
import os
import pathlib
import subprocess
import sys

LOCK_FILE_NAME = "node.lock.json"
SCRIPT_DIR = pathlib.Path(__file__).resolve().parent
DEFAULT_OUTPUT_DIR = SCRIPT_DIR.parent.parent.parent / "out" / "node" / "linux-arm64"
ELF_MAGIC = b"\x7fELF"
ELFCLASS64 = 2
ELFDATA2LSB = 1
EM_AARCH64 = 183
ET_EXEC = 2
ET_DYN = 3


def load_json(path):
    with open(path, "r", encoding="utf-8") as f:
        return json.load(f)


def check_lock(output_dir):
    lock_path = SCRIPT_DIR / LOCK_FILE_NAME
    if not lock_path.exists():
        return ["锁文件不存在: " + str(lock_path)]
    lock = load_json(lock_path)
    issues = []
    required = [
        "schemaVersion", "componentId", "name", "version", "ltsCodename",
        "npmVersion", "napiVersion", "platform", "architecture",
        "archiveName", "archiveRoot", "archiveSha256",
    ]
    for key in required:
        if key not in lock:
            issues.append(f"锁文件缺少字段: {key}")
    if lock.get("version") != "24.19.0":
        issues.append(f"Node 版本不匹配: {lock.get('version')}")
    if lock.get("npmVersion") != "11.17.0":
        issues.append(f"npm 版本不匹配: {lock.get('npmVersion')}")
    if lock.get("napiVersion") != 137:
        issues.append(f"N-API 版本不匹配: {lock.get('napiVersion')}")
    if lock.get("platform") != "linux":
        issues.append(f"平台不匹配: {lock.get('platform')}")
    if lock.get("architecture") != "arm64":
        issues.append(f"架构不匹配: {lock.get('architecture')}")
    if lock.get("archiveSha256") != "01443c1e1a29e531ccad5a46fefa6df490d2189c49f7955904aecdbb0fe86fdc":
        issues.append("SHA-256 不匹配")
    return issues


def check_output_dir(output_dir):
    issues = []
    if not output_dir.exists():
        return ["输出目录不存在: " + str(output_dir)]
    required_files = [
        "node/bin/node",
        "node/bin/npm",
        "node/bin/npx",
        "node/lib/node_modules/npm/bin/npm-cli.js",
        "node/lib/node_modules/npm/bin/npx-cli.js",
        "node/include/node",
        "node/LICENSE",
        "node-runtime.json",
        "file-manifest.json",
        "SHA256SUMS",
    ]
    for rel in required_files:
        fp = output_dir / rel
        if not fp.exists():
            issues.append(f"缺失文件: {rel}")
    return issues


def check_permissions(output_dir):
    issues = []
    node_bin = output_dir / "node" / "bin" / "node"
    if node_bin.exists() and not node_bin.is_symlink():
        mode = node_bin.stat().st_mode & 0o777
        if mode != 0o755:
            issues.append(f"node 权限不正确: {oct(mode)}")
    return issues


def check_symlinks(output_dir):
    issues = []
    for rel in ["node/bin/npm", "node/bin/npx"]:
        fp = output_dir / rel
        if fp.is_symlink():
            target = os.readlink(fp)
            if target.startswith("/"):
                issues.append(f"绝对符号链接: {rel} -> {target}")
    return issues


def check_no_escape(output_dir):
    issues = []
    for dirpath, dirnames, filenames in os.walk(output_dir):
        for name in filenames + dirnames:
            fp = pathlib.Path(dirpath) / name
            try:
                fp.relative_to(output_dir)
            except ValueError:
                issues.append(f"路径越界: {fp}")
    return issues


def check_runtime_json(output_dir):
    issues = []
    rp = output_dir / "node-runtime.json"
    if not rp.exists():
        return issues
    data = load_json(rp)
    if data.get("version") != "24.19.0":
        issues.append(f"runtime.json 版本错误: {data.get('version')}")
    if data.get("npmVersion") != "11.17.0":
        issues.append(f"runtime.json npm 版本错误: {data.get('npmVersion')}")
    if data.get("platform") != "linux":
        issues.append(f"runtime.json 平台错误: {data.get('platform')}")
    if data.get("architecture") != "arm64":
        issues.append(f"runtime.json 架构错误: {data.get('architecture')}")
    entrypoints = data.get("entrypoints", {})
    for key in ["node", "npmCli", "npxCli"]:
        if key not in entrypoints:
            issues.append(f"runtime.json 缺少 entrypoint: {key}")
    return issues


def check_manifest(output_dir):
    issues = []
    mp = output_dir / "file-manifest.json"
    if not mp.exists():
        return ["file-manifest.json 不存在"]
    data = load_json(mp)
    if not isinstance(data, list):
        return ["file-manifest.json 不是数组"]
    paths = [e.get("path", "") for e in data]
    sorted_paths = sorted(paths)
    if paths != sorted_paths:
        issues.append("file-manifest.json 未按路径排序")
    for entry in data:
        p = entry.get("path", "")
        if p.startswith("/") or ":" in p:
            issues.append(f"file-manifest.json 含非法路径: {p}")
    return issues


def check_sha256sums(output_dir):
    issues = []
    sp = output_dir / "SHA256SUMS"
    if not sp.exists():
        return issues
    with open(sp, "r", encoding="utf-8") as f:
        lines = f.read().strip().split("\n")
    for line in lines:
        parts = line.split("  ", 1)
        if len(parts) != 2:
            issues.append(f"SHA256SUMS 格式错误: {line}")
            continue
        digest, name = parts
        fp = output_dir / name
        if not fp.exists():
            issues.append(f"SHA256SUMS 引用缺失文件: {name}")
            continue
        actual = hashlib.sha256(fp.read_bytes()).hexdigest()
        if actual != digest:
            issues.append(f"SHA256SUMS 校验失败: {name}")
    return issues


def check_archive_integrity(output_dir):
    issues = []
    for name in sorted(output_dir.iterdir()):
        if name.name.endswith(".tar.xz"):
            try:
                import tarfile
                with tarfile.open(name, "r:xz") as tf:
                    members = tf.getmembers()
                    names = [m.name for m in members]
                    if names != sorted(names):
                        issues.append(f"归档成员未排序: {name.name}")
                    for m in members:
                        if m.uid != 0 or m.gid != 0:
                            issues.append(f"归档 uid/gid 非零: {m.name}")
                        if m.mtime != 0:
                            issues.append(f"归档 mtime 非零: {m.name}")
            except Exception as e:
                issues.append(f"归档无法解析: {name.name} ({e})")
    return issues


def check_elf_binary(output_dir):
    issues = []
    node_bin = output_dir / "node" / "bin" / "node"
    if not node_bin.exists():
        return ["node 二进制不存在"]
    if node_bin.stat().st_size == 0:
        return ["node 二进制为空文件"]
    with open(node_bin, "rb") as f:
        header = f.read(64)
    if len(header) < 20:
        return ["node 二进制过小"]
    if header[:4] != ELF_MAGIC:
        return ["node 不是 ELF 文件"]
    if header[4] != ELFCLASS64:
        issues.append("node 非 64 位")
    if header[5] != ELFDATA2LSB:
        issues.append("node 非 Little Endian")
    machine = int.from_bytes(header[18:20], "little")
    if machine != EM_AARCH64:
        issues.append(f"node Machine 非 AArch64: {machine}")
    e_type = int.from_bytes(header[16:18], "little")
    if e_type not in (ET_EXEC, ET_DYN):
        issues.append(f"node 类型非可执行/共享对象: {e_type}")
    if header[:2] == b"MZ":
        issues.append("node 是 Windows PE")
    if header[:4] in (b"\xfe\xed\xfa\xce", b"\xfe\xed\xfa\xcf", b"\xce\xfa\xed\xfe", b"\xcf\xfa\xed\xfe"):
        issues.append("node 是 Mach-O")
    return issues


def run_static_checks(output_dir):
    all_issues = []
    checks = [
        ("锁文件", check_lock),
        ("输出目录", check_output_dir),
        ("文件权限", check_permissions),
        ("符号链接", check_symlinks),
        ("路径越界", check_no_escape),
        ("runtime.json", check_runtime_json),
        ("file-manifest.json", check_manifest),
        ("SHA256SUMS", check_sha256sums),
        ("归档完整性", check_archive_integrity),
        ("ELF 校验", check_elf_binary),
    ]
    for name, fn in checks:
        try:
            issues = fn(output_dir)
        except Exception as e:
            issues = [f"{name} 检查异常: {e}"]
        if issues:
            all_issues.extend(issues)
            print(f"[FAIL] {name}:")
            for i in issues:
                print(f"  - {i}")
        else:
            print(f"[PASS] {name}")
    return all_issues


def run_runtime_checks(output_dir):
    node_bin = output_dir / "node" / "bin" / "node"
    npm_cli = output_dir / "node" / "lib" / "node_modules" / "npm" / "bin" / "npm-cli.js"
    npx_cli = output_dir / "node" / "lib" / "node_modules" / "npm" / "bin" / "npx-cli.js"
    test_script = SCRIPT_DIR / "test-runtime.mjs"
    failures = []
    commands = [
        ("node --version", [str(node_bin), "--version"], "v24.19.0"),
        ("npm --version", [str(node_bin), str(npm_cli), "--version"], "11.17.0"),
        ("npx --version", [str(node_bin), str(npx_cli), "--version"], "11.17.0"),
    ]
    for label, cmd, expected in commands:
        try:
            result = subprocess.run(cmd, capture_output=True, text=True, timeout=60)
            out = result.stdout.strip()
            if expected not in out:
                failures.append(f"{label}: 期望 {expected!r}, 实际 {out!r}")
            else:
                print(f"[PASS] {label}: {out}")
        except Exception as e:
            failures.append(f"{label}: 执行失败: {e}")
    if test_script.exists():
        try:
            result = subprocess.run(
                [str(node_bin), str(test_script)],
                capture_output=True, text=True, timeout=60,
                cwd=str(output_dir),
            )
            if result.returncode != 0:
                failures.append(f"test-runtime.mjs: 退出码 {result.returncode}\n{result.stderr}")
            else:
                print(f"[PASS] test-runtime.mjs: {result.stdout.strip()}")
        except Exception as e:
            failures.append(f"test-runtime.mjs: 执行失败: {e}")
    return failures


def parse_args():
    parser = argparse.ArgumentParser(description="验证 Linux ARM64 Node Runtime 产物")
    parser.add_argument("--mode", choices=["static", "runtime"], default="static")
    parser.add_argument("--artifact", help="验证单个归档文件")
    parser.add_argument("--distribution", help="指定分发目录路径")
    return parser.parse_args()


def main():
    args = parse_args()
    output_dir = pathlib.Path(args.distribution).resolve() if args.distribution else DEFAULT_OUTPUT_DIR
    if args.artifact:
        print(f"[单件] 归档: {args.artifact}")
        return
    if args.mode == "static":
        print(f"[静态验证] 目录: {output_dir}")
        issues = run_static_checks(output_dir)
        if issues:
            print(f"\n[结果] {len(issues)} 项失败")
            sys.exit(1)
        print("\n[结果] 全部通过")
    elif args.mode == "runtime":
        print(f"[Runtime 验证] 目录: {output_dir}")
        failures = run_runtime_checks(output_dir)
        if failures:
            print(f"\n[结果] {len(failures)} 项失败")
            for f in failures:
                print(f"  - {f}")
            sys.exit(1)
        print("\n[结果] 全部通过")


if __name__ == "__main__":
    main()
