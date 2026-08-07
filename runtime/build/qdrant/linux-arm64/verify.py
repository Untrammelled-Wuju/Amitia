import argparse
import hashlib
import json
import os
import pathlib
import platform
import re
import shutil
import stat
import subprocess
import sys
import tarfile

import elf_inspector

SCRIPT_DIR = pathlib.Path(__file__).resolve().parent
DEFAULT_OUTPUT_DIR = SCRIPT_DIR.parent.parent / "out" / "qdrant" / "linux-arm64"
LOCK_FILE_PATH = SCRIPT_DIR / "qdrant.lock.json"

REQUIRED_FILES = ["qdrant-runtime.json", "file-manifest.json", "SHA256SUMS"]
REQUIRED_BIN_PATH = "qdrant/bin/qdrant"
REQUIRED_LICENSE_PATH = "qdrant/LICENSE"


def sha256_file(path):
    h = hashlib.sha256()
    with open(path, "rb") as f:
        while True:
            chunk = f.read(1048576)
            if not chunk:
                break
            h.update(chunk)
    return h.hexdigest()


def load_json(path):
    with open(path, "r", encoding="utf-8") as f:
        return json.load(f)


def validate_lock(lock):
    required = [
        "schemaVersion", "componentId", "name", "version",
        "releaseTag", "releaseCommit", "releasePublishedAt",
        "platform", "architecture", "rustTarget", "libc",
        "assetName", "assetSize", "assetContentType",
        "assetSha256", "licenseFile", "licenseSha256",
    ]
    for k in required:
        if k not in lock:
            return f"锁文件缺少: {k}"
    if lock["platform"] != "linux":
        return f"platform 非 linux: {lock['platform']}"
    if lock["architecture"] != "arm64":
        return f"architecture 非 arm64: {lock['architecture']}"
    if lock["rustTarget"] != "aarch64-unknown-linux-musl":
        return f"rustTarget 非 musl: {lock['rustTarget']}"
    if lock["libc"] != "musl":
        return f"libc 非 musl: {lock['libc']}"
    if not re.fullmatch(r"[0-9a-f]{64}", lock["assetSha256"]):
        return "assetSha256 格式错误"
    if not re.fullmatch(r"[0-9a-f]{64}", lock["licenseSha256"]):
        return "licenseSha256 格式错误"
    if not re.fullmatch(r"[0-9a-f]{40}", lock["releaseCommit"]):
        return "releaseCommit 非完整 SHA"
    if not isinstance(lock["assetSize"], int) or lock["assetSize"] <= 0:
        return "assetSize 非正整数"
    return None


def check_permissions(root):
    issues = []
    rp = pathlib.Path(root)
    for dirpath, dirnames, filenames in os.walk(rp, followlinks=False):
        dirp = pathlib.Path(dirpath)
        dir_mode = stat.S_IMODE(dirp.stat().st_mode)
        if dir_mode != 0o755:
            issues.append(f"目录权限异常: {dirp.relative_to(rp)} {oct(dir_mode)}")
        for fn in filenames:
            fp = dirp / fn
            if fp.is_symlink():
                continue
            if fn == "qdrant" and fp.parent.name == "bin":
                expected = 0o755
            elif fn == "LICENSE":
                expected = 0o644
            else:
                expected = 0o644
            mode = stat.S_IMODE(fp.stat().st_mode)
            if mode != expected:
                rel = fp.relative_to(rp)
                issues.append(f"文件权限异常: {rel} {oct(mode)} != {oct(expected)}")
    return issues


def validate_runtime_json(path):
    issues = []
    data = load_json(path)
    for k in ("schemaVersion", "componentId", "name", "version", "platform",
              "architecture", "rustTarget", "libc", "distributionRoot", "entrypoints"):
        if k not in data:
            issues.append(f"qdrant-runtime.json 缺少: {k}")
    if data.get("entrypoints", {}).get("server") != "qdrant/bin/qdrant":
        issues.append("entrypoints.server 路径不正确")
    if data.get("compatibility", {}).get("pageSize4KTested") not in (True, False):
        issues.append("pageSize4KTested 必须为布尔")
    if data.get("compatibility", {}).get("pageSize16KTested") not in (True, False):
        issues.append("pageSize16KTested 必须为布尔")
    if data.get("distributionRoot") != "qdrant":
        issues.append("distributionRoot 必须为 qdrant")
    return issues


def validate_file_manifest(path):
    issues = []
    data = load_json(path)
    if not isinstance(data, list):
        return ["file-manifest.json 必须为数组"]
    if len(data) == 0:
        return ["file-manifest.json 不能为空"]
    paths = [e["path"] for e in data]
    if paths != sorted(paths):
        issues.append("file-manifest.json 路径未排序")
    for e in data:
        if e.get("type") == "file":
            if "path" not in e or "size" not in e or "sha256" not in e:
                issues.append(f"文件条目缺少字段: {e.get('path')}")
            elif e["size"] > 0 and not re.fullmatch(r"[0-9a-f]{64}", e["sha256"]):
                issues.append(f"文件条目 SHA 格式错误: {e['path']}")
    return issues


def validate_sha256sums(output_dir):
    issues = []
    sums_path = output_dir / "SHA256SUMS"
    if not sums_path.exists():
        return ["缺少 SHA256SUMS"]
    with open(str(sums_path), "r", encoding="utf-8") as f:
        lines = f.read().strip().split("\n")
    if not lines:
        return ["SHA256SUMS 为空"]
    digests = {}
    for line in lines:
        m = re.fullmatch(r"([0-9a-f]{64})  (.+)", line)
        if not m:
            issues.append(f"SHA256SUMS 行格式错误: {line}")
            continue
        digest, name = m.group(1), m.group(2)
        digests[name] = digest
        fp = output_dir / name
        if not fp.exists():
            issues.append(f"SHA256SUMS 引用缺失: {name}")
        elif sha256_file(str(fp)) != digest:
            issues.append(f"SHA256SUMS 校验失败: {name}")
    return issues


def validate_archive(output_dir, version):
    issues = []
    expected_name = f"amitia-qdrant-v{version}-linux-arm64-musl.tar.xz"
    archive_path = output_dir / expected_name
    if not archive_path.exists():
        return [f"缺少归档: {expected_name}"]
    try:
        with tarfile.open(str(archive_path), "r:xz") as tf:
            members = tf.getmembers()
            if not members:
                return ["归档为空"]
            names = sorted(m.name for m in members)
            names_unsorted = [m.name for m in members]
            if names != names_unsorted:
                issues.append("归档成员未排序")
            for m in members:
                if m.name.startswith("/"):
                    issues.append(f"归档成员绝对路径: {m.name}")
                if m.uid != 0 or m.gid != 0:
                    issues.append(f"归档成员 uid/gid 非 0: {m.name}")
                if m.mtime != 0:
                    issues.append(f"归档成员 mtime 非 0: {m.name}")
                full = os.path.normpath(m.name)
                if ".." in pathlib.PurePosixPath(full).parts:
                    issues.append(f"归档路径穿越: {m.name}")
    except Exception as e:
        issues.append(f"归档解析失败: {e}")
    return issues


def run_static(output_dir, report_path=None):
    issues = []
    errors = []

    lock = load_json(str(LOCK_FILE_PATH))
    err = validate_lock(lock)
    if err:
        errors.append(f"锁文件: {err}")

    out = pathlib.Path(output_dir)
    for name in REQUIRED_FILES:
        if not (out / name).exists():
            issues.append(f"缺少: {name}")

    bin_path = out / REQUIRED_BIN_PATH
    if not bin_path.exists():
        errors.append(f"缺少 qdrant 二进制: {REQUIRED_BIN_PATH}")
    elif bin_path.stat().st_size == 0:
        errors.append("qdrant 二进制为空")
    else:
        try:
            elf = elf_inspector.inspect(str(bin_path))
            if elf["machine"] != "aarch64":
                errors.append(f"ELF Machine 错误: {elf['machine']}")
            if elf["elfClass"] != 64:
                errors.append(f"ELF Class 错误: {elf['elfClass']}")
            if elf["endianness"] != "little":
                errors.append(f"ELF 端序错误: {elf['endianness']}")
            if not elf["loadSegmentAlignments"]:
                errors.append("缺少 Load Segment")
            print(f"[ELF] machine={elf['machine']} class={elf['elfClass']} "
                  f"type={elf['type']} hasInterp={elf['hasInterpreter']} "
                  f"hasDyn={elf['hasDynamicSegment']} "
                  f"alignments={elf['loadSegmentAlignments']}")
        except Exception as e:
            errors.append(f"ELF 检查失败: {e}")

    lic_path = out / REQUIRED_LICENSE_PATH
    if not lic_path.exists():
        issues.append("缺少 LICENSE")
    elif sha256_file(str(lic_path)) != lock.get("licenseSha256"):
        issues.append("LICENSE SHA 与锁文件不符")

    issues.extend(validate_runtime_json(str(out / "qdrant-runtime.json")))
    issues.extend(validate_file_manifest(str(out / "file-manifest.json")))
    issues.extend(validate_sha256sums(out))
    issues.extend(check_permissions(str(out / "qdrant")))
    issues.extend(validate_archive(str(out), lock.get("version", "")))

    report = {
        "mode": "static",
        "status": "fail" if (issues or errors) else "pass",
        "errors": errors,
        "warnings": issues,
    }

    if report_path:
        pathlib.Path(report_path).parent.mkdir(parents=True, exist_ok=True)
        with open(str(report_path), "w", encoding="utf-8", newline="") as f:
            json.dump(report, f, indent=2, ensure_ascii=False)
            f.write("\n")

    if errors:
        for e in errors:
            print(f"[错误] {e}", file=sys.stderr)
    if issues:
        for w in issues:
            print(f"[警告] {w}")

    if not errors and not issues:
        print("[静态验证] 通过")
    else:
        print(f"[静态验证] 失败 ({len(errors)} 错误, {len(issues)} 警告)")

    return 0 if not errors else 1


def run_runtime(distribution, report_path=None):
    errors = []
    warnings = []

    if platform.system().lower() != "linux":
        errors.append(f"当前系统非 Linux: {platform.system()}")
    machine = platform.machine().lower()
    if machine not in ("aarch64", "arm64"):
        errors.append(f"当前架构非 arm64: {platform.machine()}")

    if errors:
        report = {"mode": "runtime", "status": "fail", "errors": errors, "warnings": warnings}
        if report_path:
            pathlib.Path(report_path).parent.mkdir(parents=True, exist_ok=True)
            with open(str(report_path), "w", encoding="utf-8", newline="") as f:
                json.dump(report, f, indent=2, ensure_ascii=False)
                f.write("\n")
        for e in errors:
            print(f"[错误] {e}", file=sys.stderr)
        print("[Runtime 验证] 环境不兼容，跳过")
        return 1

    dist = pathlib.Path(distribution)
    bin_path = dist / "bin" / "qdrant"
    if not bin_path.exists():
        errors.append(f"缺少二进制: {bin_path}")

    page_size = os.sysconf("SC_PAGE_SIZE")
    print(f"[环境] arch={machine} page_size={page_size}")

    report = {
        "mode": "runtime",
        "status": "skip" if errors else "pending",
        "page_size": page_size,
        "errors": errors,
        "warnings": warnings,
    }

    if report_path:
        pathlib.Path(report_path).parent.mkdir(parents=True, exist_ok=True)
        with open(str(report_path), "w", encoding="utf-8", newline="") as f:
            json.dump(report, f, indent=2, ensure_ascii=False)
            f.write("\n")

    if errors:
        return 1

    print("[Runtime 验证] 环境兼容，请在真实环境继续运行 smoke_test.py")
    return 0


def parse_args():
    parser = argparse.ArgumentParser(description="验证 Qdrant Linux ARM64 Runtime 产物")
    parser.add_argument("--mode", choices=["static", "runtime"], required=True)
    parser.add_argument("--output-dir", help="构建输出目录")
    parser.add_argument("--distribution", help="分发目录路径（runtime 模式）")
    parser.add_argument("--report", help="报告输出路径")
    return parser.parse_args()


def main():
    args = parse_args()
    if args.mode == "static":
        output_dir = pathlib.Path(args.output_dir) if args.output_dir else DEFAULT_OUTPUT_DIR
        sys.exit(run_static(output_dir, args.report))
    elif args.mode == "runtime":
        distribution = pathlib.Path(args.distribution) if args.distribution else DEFAULT_OUTPUT_DIR / "qdrant"
        sys.exit(run_runtime(distribution, args.report))


if __name__ == "__main__":
    main()
