import argparse
import hashlib
import json
import os
import pathlib
import platform
import stat
import subprocess
import sys

import inspect_elf

DEFAULT_OUTPUT_DIR = pathlib.Path(__file__).resolve().parent.parent.parent / "out" / "backend" / "linux-arm64"
LOCK_FILE_NAME = "backend-build.lock.json"
LOCK_FILE_PATH = pathlib.Path(__file__).resolve().parent / LOCK_FILE_NAME

BINARY_PATH = "backend/amitia-server"
REQUIRED_FILES = [
    "backend/amitia-server",
    "manifest/backend-artifact.json",
    "manifest/build-inputs.json",
    "manifest/dependency-manifest.json",
    "manifest/go-version-metadata.txt",
]


class VerifyError(Exception):
    pass


def sha256_file(path):
    h = hashlib.sha256()
    with open(path, "rb") as f:
        while True:
            chunk = f.read(1048576)
            if not chunk:
                break
            h.update(chunk)
    return h.hexdigest()


def load_lock():
    with open(LOCK_FILE_PATH, "r", encoding="utf-8") as f:
        return json.load(f)


def verify_directory_structure(output_dir):
    issues = []
    for name in REQUIRED_FILES:
        fp = output_dir / name
        if not fp.exists():
            issues.append(f"缺少: {name}")
    return issues


def verify_permissions(output_dir):
    issues = []
    bin_path = output_dir / BINARY_PATH
    if not bin_path.exists():
        issues.append(f"缺少二进制: {BINARY_PATH}")
        return issues
    mode = stat.S_IMODE(bin_path.stat().st_mode)
    if mode != 0o755:
        issues.append(f"二进制权限异常: {oct(mode)} != 0o755")
    for manifest_file in ["manifest/backend-artifact.json", "manifest/build-inputs.json",
                          "manifest/dependency-manifest.json", "manifest/go-version-metadata.txt"]:
        fp = output_dir / manifest_file
        if fp.exists():
            mode = stat.S_IMODE(fp.stat().st_mode)
            if mode != 0o644:
                issues.append(f"元数据权限异常: {manifest_file} {oct(mode)}")
    return issues


def verify_lock_consistency(output_dir, lock):
    issues = []
    if lock["platform"] != "linux":
        issues.append(f"Lock 平台错误: {lock['platform']}")
    if lock["architecture"] != "arm64":
        issues.append(f"Lock 架构错误: {lock['architecture']}")
    if lock["goarm64"] != "v8.0":
        issues.append(f"Lock GOARM64 错误: {lock['goarm64']}")
    if lock["cgoEnabled"] is not False:
        issues.append(f"Lock cgoEnabled 错误: {lock['cgoEnabled']}")
    return issues


def verify_elf(output_dir):
    issues = []
    errors = []
    bin_path = output_dir / BINARY_PATH
    if not bin_path.exists():
        errors.append(f"缺少二进制: {BINARY_PATH}")
        return issues, errors
    try:
        elf = inspect_elf.inspect(str(bin_path))
        if elf["elfClass"] != 64:
            errors.append(f"ELF Class 错误: {elf['elfClass']}")
        if elf["endianness"] != "little":
            errors.append(f"ELF 端序错误: {elf['endianness']}")
        if elf["machine"] != "aarch64":
            errors.append(f"ELF Machine 错误: {elf['machine']}")
        if elf["type"] != "executable":
            errors.append(f"ELF Type 错误: {elf['type']}")
        if elf["hasInterpreter"]:
            errors.append(f"存在动态解释器: {elf['interpreter']}")
        if elf["neededLibraries"]:
            errors.append(f"存在动态依赖: {elf['neededLibraries']}")
        if elf["rpath"]:
            errors.append(f"存在 RPATH: {elf['rpath']}")
        if elf["runpath"]:
            errors.append(f"存在 RUNPATH: {elf['runpath']}")
        if not elf["static"]:
            errors.append("非静态可执行文件")
    except Exception as e:
        errors.append(f"ELF 检查失败: {e}")
    return issues, errors


def verify_sha256sums(output_dir):
    issues = []
    sums_path = output_dir / "SHA256SUMS"
    if not sums_path.exists():
        return ["缺少 SHA256SUMS"]
    with open(str(sums_path), "r", encoding="utf-8") as f:
        content = f.read().strip()
    if not content:
        return ["SHA256SUMS 为空"]
    lines = content.split("\n")
    for line in lines:
        parts = line.split("  ", 1)
        if len(parts) != 2:
            issues.append(f"SHA256SUMS 行格式错误: {line}")
            continue
        digest, name = parts
        fp = output_dir / name
        if not fp.exists():
            issues.append(f"SHA256SUMS 引用缺失: {name}")
            continue
        actual = sha256_file(str(fp))
        if actual != digest:
            issues.append(f"SHA 校验失败: {name}")
    return issues


def verify_source_path_leak(output_dir):
    bin_path = output_dir / BINARY_PATH
    if not bin_path.exists():
        return []
    repo_root = str(pathlib.Path(__file__).resolve().parent.parent.parent.parent.parent)
    root_bytes = repo_root.encode("utf-8")
    with open(str(bin_path), "rb") as f:
        content = f.read()
    if root_bytes in content:
        return [f"二进制包含源码绝对路径"]
    return []


def verify_static(output_dir, expected_version=None, expected_commit=None):
    issues = []
    errors = []
    artifact_path = output_dir / "manifest" / "backend-artifact.json"
    if not artifact_path.exists():
        errors.append("缺少 backend-artifact.json")
        return issues, errors
    try:
        with open(artifact_path, "r", encoding="utf-8") as f:
            artifact = json.load(f)
    except json.JSONDecodeError as e:
        errors.append(f"backend-artifact.json 解析失败: {e}")
        return issues, errors
    expected_keys = ["schemaVersion", "name", "version", "commit", "platform",
                     "architecture", "cgoEnabled", "sha256", "size", "elf"]
    for k in expected_keys:
        if k not in artifact:
            errors.append(f"backend-artifact.json 缺少: {k}")
    bin_path = output_dir / BINARY_PATH
    if bin_path.exists():
        actual_sha = sha256_file(str(bin_path))
        if artifact.get("sha256") != actual_sha:
            errors.append(f"SHA 不一致: {artifact.get('sha256')} vs {actual_sha}")
        actual_size = bin_path.stat().st_size
        if artifact.get("size") != actual_size:
            errors.append(f"Size 不一致: {artifact.get('size')} vs {actual_size}")
    if artifact.get("version") == "dev" or artifact.get("commit") == "unknown":
        issues.append("标记为开发构建")
    if expected_version and artifact.get("version") != expected_version:
        errors.append(f"版本不匹配: {artifact.get('version')} vs {expected_version}")
    if expected_commit and artifact.get("commit") != expected_commit:
        errors.append(f"Commit 不匹配: {artifact.get('commit')} vs {expected_commit}")
    if artifact.get("platform") != "linux":
        errors.append(f"platform 错误: {artifact.get('platform')}")
    if artifact.get("architecture") != "arm64":
        errors.append(f"architecture 错误: {artifact.get('architecture')}")
    if artifact.get("cgoEnabled") is not False:
        errors.append(f"cgoEnabled 错误: {artifact.get('cgoEnabled')}")
    return issues, errors


def verify_archive(output_dir, version):
    issues = []
    for f in output_dir.iterdir():
        if f.name.endswith(".tar.xz"):
            result = subprocess.run(
                [sys.executable, str(pathlib.Path(__file__).resolve().parent / "archive.py"),
                 "verify", str(f)],
                capture_output=True, text=True, check=False
            )
            if result.returncode != 0:
                issues.append(f"归档验证失败: {f.name}")
            break
    return issues


def run_static_checks(output_dir, expected_version=None, expected_commit=None):
    all_issues = []
    all_errors = []

    lock = load_lock()
    issues = verify_directory_structure(output_dir)
    all_issues.extend(issues)
    issues = verify_permissions(output_dir)
    all_issues.extend(issues)
    issues = verify_lock_consistency(output_dir, lock)
    all_issues.extend(issues)
    issues, errors = verify_elf(output_dir)
    all_issues.extend(issues)
    all_errors.extend(errors)
    issues = verify_sha256sums(output_dir)
    all_issues.extend(issues)
    issues = verify_source_path_leak(output_dir)
    all_issues.extend(issues)
    issues, errors = verify_static(output_dir, expected_version, expected_commit)
    all_issues.extend(issues)
    all_errors.extend(errors)

    if expected_version:
        issues = verify_archive(output_dir, expected_version)
        all_issues.extend(issues)

    return all_issues, all_errors


def run_static(output_dir, expected_version=None, expected_commit=None, report_path=None):
    issues, errors = run_static_checks(output_dir, expected_version, expected_commit)
    passed = not errors and not issues
    report = {
        "mode": "static",
        "passed": passed,
        "checks": {
            "total": 7,
            "errors": len(errors),
            "warnings": len(issues),
        },
        "failures": errors + issues,
        "skipped": [],
    }
    if report_path:
        pathlib.Path(report_path).parent.mkdir(parents=True, exist_ok=True)
        with open(str(report_path), "w", encoding="utf-8", newline="") as f:
            json.dump(report, f, indent=2, ensure_ascii=False)
            f.write("\n")
    for e in errors:
        print(f"[错误] {e}", file=sys.stderr)
    for w in issues:
        print(f"[警告] {w}")
    if passed:
        print("[静态验证] 通过")
    else:
        print(f"[静态验证] 失败 ({len(errors)} 错误, {len(issues)} 警告)")
    return 0 if passed else 1


def run_arm64_runtime(output_dir, expected_version=None, expected_commit=None, report_path=None):
    errors = []
    warnings = []
    current_system = platform.system().lower()
    current_machine = platform.machine().lower()
    if current_system != "linux":
        errors.append(f"当前系统非 Linux: {platform.system()}")
    if current_machine not in ("aarch64", "arm64"):
        errors.append(f"当前架构非 arm64: {platform.machine()}")
    if errors:
        report = {"mode": "arm64-runtime", "passed": False, "errors": errors, "warnings": warnings, "skipped": []}
        if report_path:
            pathlib.Path(report_path).parent.mkdir(parents=True, exist_ok=True)
            with open(str(report_path), "w", encoding="utf-8", newline="") as f:
                json.dump(report, f, indent=2, ensure_ascii=False)
                f.write("\n")
        for e in errors:
            print(f"[错误] {e}", file=sys.stderr)
        print("[ARM64 Runtime 验证] 环境不兼容，跳过")
        return 1
    bin_path = output_dir / BINARY_PATH
    if not bin_path.exists():
        errors.append(f"缺少: {BINARY_PATH}")
    if errors:
        report = {"mode": "arm64-runtime", "passed": False, "errors": errors, "warnings": warnings, "skipped": []}
        if report_path:
            pathlib.Path(report_path).parent.mkdir(parents=True, exist_ok=True)
            with open(str(report_path), "w", encoding="utf-8", newline="") as f:
                json.dump(report, f, indent=2, ensure_ascii=False)
                f.write("\n")
        return 1
    print("环境兼容，请在真实 Linux ARM64 环境继续运行版本检查")
    return 0


def run_ubuntu_rootfs(output_dir, rootfs_path, expected_version=None, expected_commit=None, report_path=None):
    errors = []
    warnings = []
    if platform.system().lower() != "linux":
        errors.append(f"当前系统非 Linux: {platform.system()}")
    if errors:
        report = {"mode": "ubuntu-rootfs", "passed": False, "errors": errors, "warnings": warnings, "skipped": []}
        if report_path:
            pathlib.Path(report_path).parent.mkdir(parents=True, exist_ok=True)
            with open(str(report_path), "w", encoding="utf-8", newline="") as f:
                json.dump(report, f, indent=2, ensure_ascii=False)
                f.write("\n")
        for e in errors:
            print(f"[错误] {e}", file=sys.stderr)
        print("[Ubuntu RootFS 验证] 环境不兼容，跳过")
        return 1
    bin_path = output_dir / BINARY_PATH
    target = pathlib.Path(rootfs_path) / "opt" / "amitia" / "backend" / "amitia-server"
    if not bin_path.exists():
        errors.append(f"缺少: {BINARY_PATH}")
    if not pathlib.Path(rootfs_path).exists():
        errors.append(f"RootFS 不存在: {rootfs_path}")
    if errors:
        report = {"mode": "ubuntu-rootfs", "passed": False, "errors": errors, "warnings": warnings, "skipped": []}
        if report_path:
            pathlib.Path(report_path).parent.mkdir(parents=True, exist_ok=True)
            with open(str(report_path), "w", encoding="utf-8", newline="") as f:
                json.dump(report, f, indent=2, ensure_ascii=False)
                f.write("\n")
        return 1
    target.parent.mkdir(parents=True, exist_ok=True)
    import shutil
    shutil.copy2(str(bin_path), str(target))
    print(f"二进制已复制到: {target}")
    print("请在 PRoot 环境中执行版本检查")
    return 0


def parse_args():
    parser = argparse.ArgumentParser(description="验证 Linux ARM64 Go 后端产物")
    parser.add_argument("--mode", choices=["static", "arm64-runtime", "ubuntu-rootfs"], required=True)
    parser.add_argument("--artifact", help="产物目录")
    parser.add_argument("--rootfs", help="Ubuntu RootFS 路径")
    parser.add_argument("--expected-version", help="期望版本号")
    parser.add_argument("--expected-commit", help="期望 Commit")
    parser.add_argument("--report", help="报告输出路径")
    return parser.parse_args()


def main():
    args = parse_args()
    output_dir = pathlib.Path(args.artifact).resolve() if args.artifact else DEFAULT_OUTPUT_DIR
    if args.mode == "static":
        sys.exit(run_static(output_dir, args.expected_version, args.expected_commit, args.report))
    elif args.mode == "arm64-runtime":
        sys.exit(run_arm64_runtime(output_dir, args.expected_version, args.expected_commit, args.report))
    elif args.mode == "ubuntu-rootfs":
        rootfs = pathlib.Path(args.rootfs).resolve() if args.rootfs else None
        sys.exit(run_ubuntu_rootfs(output_dir, str(rootfs) if rootfs else "", args.expected_version, args.expected_commit, args.report))


if __name__ == "__main__":
    main()
