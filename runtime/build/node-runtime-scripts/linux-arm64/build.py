import argparse
import hashlib
import json
import lzma
import os
import pathlib
import shutil
import stat
import sys
import tarfile
import tempfile

LOCK_FILE_NAME = "scripts.lock.json"
SCRIPT_DIR = pathlib.Path(__file__).resolve().parent
SOURCE_DIR = SCRIPT_DIR / "source"
RUNTIME_ROOT = SCRIPT_DIR.parent.parent.parent
DEFAULT_OUTPUT_DIR = RUNTIME_ROOT / "out" / "node-runtime-scripts" / "linux-arm64"
DEFAULT_NODE_DIST = RUNTIME_ROOT / "out" / "node" / "linux-arm64" / "node"
FIXED_MTIME = 0
FIXED_UID = 0
FIXED_GID = 0
FIXED_UNAME = "root"
FIXED_GNAME = "root"
DIR_PERM = 0o755
FILE_PERM = 0o644
SH_PERM = 0o755
XZ_COMPRESSION_LEVEL = 5

SCRIPTS_ROOT_NAME = "scripts/node"
REQUIRED_SCRIPTS = [
    "lib/amitia-node-common.sh",
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


def load_lock(path=None):
    lock_path = pathlib.Path(path) if path else SCRIPT_DIR / LOCK_FILE_NAME
    lock = load_json(lock_path)
    required = [
        "schemaVersion", "componentId", "version", "platform", "architecture",
        "nodeVersion", "npmVersion", "layout",
    ]
    for key in required:
        if key not in lock:
            raise ValueError(f"锁文件缺少必填字段: {key}")
    if lock.get("platform") != "linux":
        raise ValueError("platform 必须为 linux")
    if lock.get("architecture") != "arm64":
        raise ValueError("architecture 必须为 arm64")
    return lock


def verify_node_distribution(node_dist):
    dist = pathlib.Path(node_dist)
    issues = []
    node_bin = dist / "bin" / "node"
    npm_cli = dist / "lib" / "node_modules" / "npm" / "bin" / "npm-cli.js"
    npx_cli = dist / "lib" / "node_modules" / "npm" / "bin" / "npx-cli.js"
    runtime_json = dist.parent / "node-runtime.json"
    if not node_bin.exists():
        issues.append("node/bin/node 不存在")
    if not npm_cli.exists():
        issues.append("npm-cli.js 不存在")
    if not npx_cli.exists():
        issues.append("npx-cli.js 不存在")
    if runtime_json.exists():
        try:
            rj = load_json(runtime_json)
            if rj.get("version") != "24.19.0":
                issues.append(f"node-runtime.json version 错误: {rj.get('version')}")
            if rj.get("npmVersion") != "11.17.0":
                issues.append(f"node-runtime.json npmVersion 错误: {rj.get('npmVersion')}")
        except Exception as e:
            issues.append(f"node-runtime.json 解析失败: {e}")
    return issues


def verify_source_scripts():
    issues = []
    for rel in REQUIRED_SCRIPTS:
        fp = SOURCE_DIR / rel
        if not fp.exists():
            issues.append(f"缺失脚本: {rel}")
            continue
        data = fp.read_bytes()
        if fp.suffix == ".sh" and not rel.startswith("lib/"):
            if not data.startswith(b"#!/bin/sh\n"):
                issues.append(f"{rel}: Shebang 错误")
        if b"\r\n" in data:
            issues.append(f"{rel}: 含 CRLF")
        if not data.endswith(b"\n"):
            issues.append(f"{rel}: 末尾无换行")
        if b"\xEF\xBB\xBF" in data[:4]:
            issues.append(f"{rel}: 含 BOM")
    return issues


def build_file_manifest(root):
    entries = []
    for dirpath, dirnames, filenames in os.walk(root, followlinks=False):
        dirnames.sort()
        for fn in sorted(filenames):
            fp = pathlib.Path(dirpath) / fn
            rel = fp.relative_to(root).as_posix()
            st = fp.stat()
            if fp.is_symlink():
                entries.append({
                    "path": rel,
                    "type": "symlink",
                    "size": 0,
                    "sha256": "",
                    "mode": "",
                })
            else:
                entries.append({
                    "path": rel,
                    "type": "file",
                    "size": st.st_size,
                    "sha256": sha256_file(fp),
                    "mode": oct(stat.S_IMODE(st.st_mode)),
                })
    entries.sort(key=lambda e: e["path"])
    return entries


def write_runtime_json(output_dir, lock):
    runtime_json = {
        "schemaVersion": 1,
        "componentId": lock["componentId"],
        "version": lock["version"],
        "platform": lock["platform"],
        "architecture": lock["architecture"],
        "requires": {
            "nodeVersion": lock["nodeVersion"],
            "npmVersion": lock["npmVersion"],
        },
        "entrypoints": {
            "prepare": f"{SCRIPTS_ROOT_NAME}/amitia-node-prepare.sh",
            "probe": f"{SCRIPTS_ROOT_NAME}/amitia-node-probe.sh",
            "node": f"{SCRIPTS_ROOT_NAME}/amitia-node-exec.sh",
            "npm": f"{SCRIPTS_ROOT_NAME}/amitia-npm-exec.sh",
            "npx": f"{SCRIPTS_ROOT_NAME}/amitia-npx-exec.sh",
            "pluginHost": f"{SCRIPTS_ROOT_NAME}/amitia-plugin-host.sh",
            "taskHost": f"{SCRIPTS_ROOT_NAME}/amitia-task-host.sh",
        },
        "runtimeLayout": lock["layout"],
    }
    out = output_dir / "node-runtime-scripts.json"
    with open(out, "w", encoding="utf-8", newline="") as f:
        json.dump(runtime_json, f, indent=2, sort_keys=True)
        f.write("\n")
    return out


def write_file_manifest(output_dir, root):
    manifest = build_file_manifest(root)
    out = output_dir / "file-manifest.json"
    content = json.dumps(manifest, indent=2, sort_keys=True) + "\n"
    with open(out, "w", encoding="utf-8", newline="") as f:
        f.write(content)
    return out


def write_sha256sums(output_dir, names):
    lines = []
    for name in sorted(names):
        fp = output_dir / name
        digest = sha256_file(fp)
        lines.append(f"{digest}  {name}")
    out = output_dir / "SHA256SUMS"
    with open(out, "w", encoding="utf-8", newline="") as f:
        f.write("\n".join(lines) + "\n")
    return out


def fix_permissions(scripts_root):
    for dirpath, dirnames, filenames in os.walk(scripts_root):
        dirp = pathlib.Path(dirpath)
        os.chmod(dirp, DIR_PERM)
        for fn in filenames:
            fp = dirp / fn
            if fp.is_symlink():
                continue
            if fn.endswith(".sh"):
                os.chmod(fp, SH_PERM)
            else:
                os.chmod(fp, FILE_PERM)


def create_deterministic_tar(source_dir, output_path):
    members = []
    for dirpath, dirnames, filenames in os.walk(source_dir, followlinks=False):
        dirnames.sort()
        for dn in dirnames:
            dp = pathlib.Path(dirpath) / dn
            arcname = dp.relative_to(source_dir)
            members.append((arcname, dp))
        for fn in sorted(filenames):
            fp = pathlib.Path(dirpath) / fn
            arcname = fp.relative_to(source_dir)
            members.append((arcname, fp))
    members.sort(key=lambda x: x[0].as_posix())
    tmp_out = output_path.with_suffix(".tmp")
    with lzma.open(tmp_out, "wb", preset=XZ_COMPRESSION_LEVEL) as xz:
        with tarfile.open(fileobj=xz, mode="w") as tf:
            for arcname, fp in members:
                info = tf.gettarinfo(str(fp), arcname=str(arcname))
                info.uid = FIXED_UID
                info.gid = FIXED_GID
                info.uname = FIXED_UNAME
                info.gname = FIXED_GNAME
                info.mtime = FIXED_MTIME
                if fp.is_symlink():
                    info.type = tarfile.SYMTYPE
                    info.linkname = os.readlink(fp)
                    tf.addfile(info)
                elif fp.is_dir():
                    info.type = tarfile.DIRTYPE
                    info.mode = DIR_PERM
                    tf.addfile(info)
                else:
                    if fp.name.endswith(".sh"):
                        info.mode = SH_PERM
                    else:
                        info.mode = FILE_PERM
                    with open(fp, "rb") as fobj:
                        tf.addfile(info, fobj)
    os.replace(tmp_out, output_path)


def safe_replace_output(tmp_output, final_output):
    if final_output.exists():
        backup = final_output.with_name(final_output.name + ".old")
        if backup.exists():
            shutil.rmtree(backup)
        final_output.rename(backup)
        shutil.rmtree(backup, ignore_errors=True)
    final_output.parent.mkdir(parents=True, exist_ok=True)
    tmp_output.rename(final_output)


def run_build(args):
    lock = load_lock()
    print(f"[信息] 脚本版本: {lock['version']}")
    print(f"[信息] Node {lock['nodeVersion']} / npm {lock['npmVersion']}")
    print(f"[信息] 平台 {lock['platform']}/{lock['architecture']}")

    node_dist = pathlib.Path(args.node_distribution).resolve() if args.node_distribution else DEFAULT_NODE_DIST
    output_dir = pathlib.Path(args.output_dir).resolve() if args.output_dir else DEFAULT_OUTPUT_DIR

    if args.clean and output_dir.exists():
        print(f"[清理] 删除旧输出: {output_dir}")
        shutil.rmtree(output_dir)

    work_dir = SCRIPT_DIR / ".work"
    if work_dir.exists():
        shutil.rmtree(work_dir)
    work_dir.mkdir(parents=True, exist_ok=True)

    node_issues = verify_node_distribution(node_dist)
    if node_issues:
        print(f"[警告] Node 产物问题: {node_issues}")

    source_issues = verify_source_scripts()
    if source_issues:
        for i in source_issues:
            print(f"[错误] {i}")
        raise RuntimeError(f"源文件验证失败: {len(source_issues)} 项")

    final_output = output_dir
    tmp_output = output_dir.with_name(output_dir.name + ".partial")
    if tmp_output.exists():
        shutil.rmtree(tmp_output)
    tmp_output.mkdir(parents=True, exist_ok=True)

    try:
        scripts_target = tmp_output / SCRIPTS_ROOT_NAME
        shutil.copytree(SOURCE_DIR, scripts_target, symlinks=True)
        fix_permissions(scripts_target)
        print(f"[组装] 脚本复制到: {scripts_target}")

        write_runtime_json(tmp_output, lock)
        manifest_path = write_file_manifest(tmp_output, scripts_target)
        print("[元数据] 已生成 node-runtime-scripts.json / file-manifest.json")

        archive_out = tmp_output / f"amitia-node-runtime-scripts-v{lock['version']}-linux-{lock['architecture']}.tar.xz"
        create_deterministic_tar(tmp_output, archive_out)
        final_sha = sha256_file(archive_out)
        print(f"[SHA] {final_sha}")

        write_sha256sums(
            tmp_output,
            [archive_out.name, "node-runtime-scripts.json", manifest_path.name],
        )
        print("[元数据] 已生成 SHA256SUMS")

        safe_replace_output(tmp_output, final_output)
        print(f"[发布] 输出目录: {final_output}")
        print("[完成] Linux ARM64 Node Runtime Scripts 构建成功")
        print(f"[产物] {final_output / archive_out.name}")
    except Exception:
        if tmp_output.exists():
            shutil.rmtree(tmp_output, ignore_errors=True)
        raise
    finally:
        if work_dir.exists():
            shutil.rmtree(work_dir, ignore_errors=True)


def parse_args():
    parser = argparse.ArgumentParser(description="构建 Linux ARM64 Node Runtime Scripts")
    parser.add_argument("--clean", action="store_true")
    parser.add_argument("--output-dir", help="输出目录")
    parser.add_argument("--node-distribution", help="第 15 步 Node 产物路径")
    parser.add_argument("--plugin-host", help="Plugin Host 路径（可选）")
    parser.add_argument("--task-host", help="Task Host 路径（可选）")
    parser.add_argument("--skip-integration-test", action="store_true")
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
