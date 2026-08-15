import argparse
import hashlib
import json
import os
import pathlib
import re
import sys
import zipfile


SCRIPT_DIR = pathlib.Path(__file__).resolve().parent
REPO_ROOT = SCRIPT_DIR.parent.parent.parent.parent


def verify_inputs(lock_data, runtime_version, commit):
    issues = []
    if not re.match(r"^[0-9]+\.[0-9]+\.[0-9]+", runtime_version or ""):
        issues.append("runtime-version格式非法")
    if not re.match(r"^[0-9a-f]{40}$", (commit or "").lower()):
        issues.append("commit格式非法")
    for key in ["rootfs", "backend", "node", "nodeScripts", "qdrant", "guestLayout"]:
        comp = lock_data["components"].get(key, {})
        artifact_field = comp.get("artifact", "")
        sha_field = comp.get("sha256", "")
        if artifact_field and sha_field and "PENDING" not in sha_field:
            art_path = (REPO_ROOT / artifact_field).resolve() if not pathlib.Path(artifact_field).is_absolute() else pathlib.Path(artifact_field)
            if not art_path.exists():
                issues.append(f"{key} Artifact不存在: {artifact_field}")
            else:
                actual = hashlib.sha256(art_path.read_bytes()).hexdigest()
                if actual != sha_field:
                    issues.append(f"{key} SHA不匹配: {actual[:16]}... != {sha_field[:16]}...")
    for key in ["pluginHost", "taskHost"]:
        comp = lock_data["components"].get(key, {})
        source_dir = REPO_ROOT / comp.get("source", "") if comp.get("source") else None
        if source_dir is None or not source_dir.exists():
            issues.append(f"{key} source不存在: {comp.get('source', '')}")
        elif not (source_dir / "dist" / "index.js").exists():
            issues.append(f"{key}入口不存在: {source_dir / 'dist' / 'index.js'}")
        commit_val = comp.get("commit", "")
        if commit and commit_val and commit_val != commit:
            issues.append(f"{key} commit不一致: {commit_val[:8]} != {commit[:8]}")
    return issues


def verify_rootfs_payload(artifact_path):
    import tarfile
    issues = []
    try:
        with tarfile.open(str(artifact_path), "r:*") as tf:
            members = tf.getmembers()
    except Exception as e:
        return [f"RootFS读取失败: {e}"]
    forbidden_programs = [
        "opt/amitia/backend/amitia-server",
        "opt/amitia/node/bin/node",
        "opt/amitia/qdrant/bin/qdrant",
        "opt/amitia/plugin-host/dist/index.js",
        "opt/amitia/task-host/dist/index.js",
    ]
    names = {m.name for m in members}
    for pf in forbidden_programs:
        if pf in names:
            issues.append(f"RootFS禁止包含程序: {pf}")
    has_guest_layout = False
    has_mount_contract = False
    for m in members:
        if m.name == "opt/amitia/manifest/guest-layout.json":
            has_guest_layout = True
        if m.name == "opt/amitia/manifest/mount-contract.json":
            has_mount_contract = True
    if not has_guest_layout:
        issues.append("RootFS缺少guest-layout.json")
    if not has_mount_contract:
        issues.append("RootFS缺少mount-contract.json")
    variable_dirs = ["etc/amitia", "var/lib/amitia", "var/cache/amitia",
                     "var/log/amitia", "run/amitia"]
    for m in members:
        for vdir in variable_dirs:
            if m.name.startswith(vdir + "/") and m.name != vdir and m.isfile():
                issues.append(f"RootFS可变目录包含文件: {m.name}")
    if not issues:
        print(f"RootFS验证通过: {len(members)}成员")
    return issues


def verify_runtime_payload(artifact_path, expected_entries=None):
    import tarfile
    issues = []
    if expected_entries is None:
        expected_entries = [
            "backend/amitia-server",
            "node/bin/node",
            "node/lib/node_modules/npm/bin/npm-cli.js",
            "node/lib/node_modules/npm/bin/npx-cli.js",
            "qdrant/bin/qdrant",
            "plugin-host/dist/index.js",
            "task-host/dist/index.js",
        ]
    try:
        with tarfile.open(str(artifact_path), "r:*") as tf:
            members = tf.getmembers()
    except Exception as e:
        return [f"Runtime读取失败: {e}"]
    names = {m.name for m in members}
    for entry in expected_entries:
        if entry not in names:
            issues.append(f"Runtime缺少入口: {entry}")
    top_level_roots = {m.name.split("/")[0] for m in members if "/" in m.name}
    required_roots = {"backend", "node", "qdrant", "plugin-host", "task-host", "scripts", "manifest", "licenses"}
    forbidden_roots = {"bin", "etc", "usr", "lib", "lib64", "var", "proc", "dev", "sys", "home", "root", "tmp"}
    for root in top_level_roots:
        if root in forbidden_roots:
            issues.append(f"Runtime禁止系统根: {root}")
    for root in required_roots:
        if root not in top_level_roots:
            issues.append(f"Runtime缺少根: {root}")
    if not issues:
        print(f"Runtime验证通过: {len(members)}成员")
    return issues


def verify_package(package_path, runtime_version, commit):
    issues = []
    try:
        with zipfile.ZipFile(str(package_path), "r") as zf:
            names = zf.namelist()
            infos = zf.infolist()
    except Exception as e:
        issues.append(f"ZIP读取失败: {e}")
        return issues
    if len(names) > 1000:
        issues.append(f"ZIP Entry过多: {len(names)}")
    for n in names:
        if n.startswith("/") or ".." in n.split("/"):
            issues.append(f"ZIP非法路径: {n}")
    if len(names) != len(set(names)):
        issues.append("ZIP存在重复Entry")
    for info in infos:
        expected_time = (1980, 1, 1, 0, 0, 0)
        if info.date_time != expected_time:
            issues.append(f"ZIP时间非固定: {info.filename} -> {info.date_time}")
        if info.compress_type != zipfile.ZIP_STORED:
            issues.append(f"ZIP非STORED压缩: {info.filename}")
        if info.create_system != 3:
            issues.append(f"ZIP create_system错误: {info.filename}")
    required_entries = {
        "metadata/package-index.json",
        "metadata/component-lock.json",
        "metadata/component-index.json",
        "metadata/SHA256SUMS",
        "payload/rootfs/",
        "payload/runtime/",
        "licenses/THIRD_PARTY_NOTICES.md",
    }
    for r in required_entries:
        if not any(n == r or n.startswith(r) for n in names):
            issues.append(f"ZIP缺少必要Entry: {r}")
    if "metadata/package-index.json" in names:
        idx_data = zf.read("metadata/package-index.json")
        try:
            idx = json.loads(idx_data)
        except json.JSONDecodeError:
            issues.append("package-index.json解析失败")
        else:
            if idx.get("runtimeVersion") != runtime_version:
                issues.append(f"Package版本不匹配: {idx.get('runtimeVersion')} != {runtime_version}")
            if idx.get("sourceCommit") != commit:
                issues.append(f"Package Commit不匹配: {idx.get('sourceCommit')} != {commit}")
            target = idx.get("target", {})
            if target.get("hostPlatform") != "android":
                issues.append("Package hostPlatform错误")
            if target.get("hostAbi") != "arm64-v8a":
                issues.append("Package hostAbi错误")
            payloads = idx.get("payloads", [])
            payload_roles = {p.get("role") for p in payloads if isinstance(p, dict)}
            if "rootfs" not in payload_roles or "runtime" not in payload_roles:
                issues.append("Package缺少Payload声明")
    if not issues:
        print(f"包体验证通过: {len(names)} entry")
    return issues


def main():
    parser = argparse.ArgumentParser()
    parser.add_argument("--mode", required=True)
    parser.add_argument("--artifact", default=None, type=pathlib.Path)
    parser.add_argument("--runtime-version", default=None)
    parser.add_argument("--commit", default=None)
    parser.add_argument("--report", default=None)
    args = parser.parse_args()
    if args.mode == "inputs":
        lock_path = SCRIPT_DIR / "runtime-package.lock.json"
        if not lock_path.exists():
            print("Lock文件不存在")
            sys.exit(1)
        lock = json.loads(lock_path.read_text(encoding="utf-8"))
        issues = verify_inputs(lock, args.runtime_version or "0.0.0", args.commit or "0" * 40)
        for i in issues:
            print(f"[失败] {i}")
        if issues:
            sys.exit(1)
        print("输入验证通过")
    elif args.mode == "rootfs":
        if not args.artifact or not args.artifact.exists():
            print("RootFS Artifact不存在")
            sys.exit(1)
        issues = verify_rootfs_payload(args.artifact)
        for i in issues:
            print(f"[失败] {i}")
        if issues:
            sys.exit(1)
    elif args.mode == "runtime":
        if not args.artifact or not args.artifact.exists():
            print("Runtime Artifact不存在")
            sys.exit(1)
        issues = verify_runtime_payload(args.artifact)
        for i in issues:
            print(f"[失败] {i}")
        if issues:
            sys.exit(1)
    elif args.mode == "package":
        if not args.artifact or not args.artifact.exists():
            print("Package Artifact不存在")
            sys.exit(1)
        issues = verify_package(args.artifact, args.runtime_version, args.commit)
        for i in issues:
            print(f"[失败] {i}")
        if issues:
            sys.exit(1)
    else:
        print(f"未知模式: {args.mode}")
        sys.exit(1)


if __name__ == "__main__":
    main()
