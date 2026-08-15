import argparse
import hashlib
import json
import os
import pathlib
import re
import shutil
import subprocess
import sys
import tempfile

SCRIPT_DIR = pathlib.Path(__file__).resolve().parent
REPO_ROOT = SCRIPT_DIR.parent.parent.parent.parent
LOCK_FILE = SCRIPT_DIR / "runtime-package.lock.json"
DEFAULT_OUTPUT_DIR = REPO_ROOT / "runtime" / "out" / "runtime-package" / "android-arm64"
DEFAULT_CACHE_DIR = SCRIPT_DIR / ".cache"

SEMVER_RE = re.compile(r"^[0-9]+\.[0-9]+\.[0-9]+(-[0-9A-Za-z.]+)?(\+[0-9A-Za-z.]+)?$")
COMMIT_RE = re.compile(r"^[0-9a-f]{40}$")

AMITIA_COMMIT = "ed7e2f0140b78bd3a5088227882a2e78dd4c98e9"


def load_lock():
    with open(LOCK_FILE, "r", encoding="utf-8") as f:
        return json.load(f)


def validate_version(version):
    if not version:
        raise ValueError("runtime-version为空")
    if len(version) > 128 or " " in version or "/" in version or "\\" in version or "\n" in version:
        raise ValueError(f"runtime-version非法: {version}")
    if version in ("latest", "stable", "dev", "unknown", "current"):
        raise ValueError(f"runtime-version为保留字: {version}")
    if not SEMVER_RE.match(version):
        raise ValueError(f"runtime-version格式非法: {version}")
    return True


def validate_commit(commit):
    if not commit or len(commit) < 40 or not COMMIT_RE.match(commit.lower()):
        raise ValueError(f"commit非法: {commit}")
    if commit == "0" * len(commit):
        raise ValueError("commit不能为全零")
    return True


def resolve_path(repo_relative):
    return (REPO_ROOT / repo_relative).resolve()


def verify_component_sha(comp, repo_root):
    artifact = comp.get("artifact", "")
    sha = comp.get("sha256", "")
    if not artifact:
        raise ValueError(f"组件缺少artifact字段: {comp.get('componentId')}")
    if sha and "PENDING" in sha:
        raise ValueError(f"组件SHA未锁定: {comp.get('componentId')} sha={sha}")
    art_path = (repo_root / artifact).resolve() if not pathlib.Path(artifact).is_absolute() else pathlib.Path(artifact)
    if not art_path.exists():
        raise RuntimeError(f"组件Artifact不存在: {artifact}")
    if sha:
        actual = hashlib.sha256(art_path.read_bytes()).hexdigest()
        if actual != sha:
            raise RuntimeError(f"组件SHA不匹配: {comp.get('componentId')} 实际={actual} 期望={sha}")
    return str(art_path)


def build_rootfs_payload(lock, work_dir, offline=False):
    comp = lock["components"]["rootfs"]
    art_path = verify_component_sha(comp, REPO_ROOT)
    guest_comp = lock["components"]["guestLayout"]
    guest_art = verify_component_sha(guest_comp, REPO_ROOT)
    from rootfs_composer import compose_rootfs
    out_name = f"amitia-ubuntu-rootfs-seed-24.04.4-arm64.tar.xz"
    out_path = work_dir / "payload" / "rootfs" / out_name
    result_path, result_sha = compose_rootfs(art_path, guest_art, out_path)
    return result_path, result_sha


def build_runtime_payload(lock, work_dir, plugin_host_files, task_host_files, manifest_files, offline=False):
    backend_art = verify_component_sha(lock["components"]["backend"], REPO_ROOT)
    node_art = verify_component_sha(lock["components"]["node"], REPO_ROOT)
    node_scripts_art = verify_component_sha(lock["components"]["nodeScripts"], REPO_ROOT)
    qdrant_art = verify_component_sha(lock["components"]["qdrant"], REPO_ROOT)
    from runtime_composer import compose_runtime_root
    version = lock.get("_runtime_version", "1.0.0")
    out_name = f"amitia-runtime-root-{version}-linux-arm64.tar.xz"
    out_path = work_dir / "payload" / "runtime" / out_name
    result_path, result_sha, members = compose_runtime_root(
        backend_art, node_art, node_scripts_art, qdrant_art,
        plugin_host_files, task_host_files, manifest_files,
        out_path
    )
    return result_path, result_sha


def generate_manifest_file(payloads, rootfs_sha, runtime_sha, version, commit, sha256sums_ref=None):
    metadata = [
        {
            "role": "guest-layout",
            "path": "metadata/guest-layout.json",
            "sha256": payloads["guestLayoutSha"],
            "size": payloads["guestLayoutSize"],
        },
        {
            "role": "mount-contract",
            "path": "metadata/mount-contract.json",
            "sha256": payloads["mountContractSha"],
            "size": payloads["mountContractSize"],
        },
        {
            "role": "sha256sums",
            "path": "metadata/SHA256SUMS",
            "sha256": sha256sums_ref["sha256"] if sha256sums_ref else "placeholder",
            "size": sha256sums_ref["size"] if sha256sums_ref else 0,
        },
    ]
    return {
        "schemaVersion": 1,
        "packageFormatVersion": 1,
        "packageId": "amitia.runtime.android",
        "runtimeVersion": version,
        "sourceCommit": commit,
        "target": {
            "hostPlatform": "android",
            "hostAbi": "arm64-v8a",
            "runtimeKind": "proot",
            "guestPlatform": "linux",
            "guestArchitecture": "arm64",
            "distribution": "ubuntu",
            "distributionRelease": "24.04.4",
        },
        "payloads": [
            {
                "role": "rootfs",
                "path": payloads["rootfs"]["path"],
                "sha256": payloads["rootfs"]["sha256"],
                "size": payloads["rootfs"]["size"],
            },
            {
                "role": "runtime",
                "path": payloads["runtime"]["path"],
                "sha256": payloads["runtime"]["sha256"],
                "size": payloads["runtime"]["size"],
            },
        ],
        "metadata": metadata,
    }


def generate_file_manifest(payload_rootfs, payload_runtime):
    files = []
    for p in [payload_rootfs, payload_runtime]:
        art = pathlib.Path(p)
        files.append({
            "path": f"payload/{'rootfs' if 'rootfs' in p else 'runtime'}/{art.name}",
            "type": "archive",
            "size": art.stat().st_size,
            "sha256": hashlib.sha256(art.read_bytes()).hexdigest(),
        })
    return files


def compute_sha256sums(work_dir):
    lines = []
    for root, dirs, files in os.walk(str(work_dir)):
        root_path = pathlib.Path(root)
        for fname in sorted(files):
            fp = root_path / fname
            rel = fp.relative_to(work_dir).as_posix()
            if rel == "SHA256SUMS":
                continue
            sha = hashlib.sha256(fp.read_bytes()).hexdigest()
            lines.append((rel, sha))
    lines.sort(key=lambda x: x[0])
    content = "".join(f"{sha}  {name}\n" for name, sha in lines)
    return content


def atomic_publish(work_dir, output_dir):
    output_dir = pathlib.Path(output_dir)
    # Collect existing ZIP and SHA files to preserve them
    preserved = []
    if output_dir.exists():
        for child in output_dir.iterdir():
            if child.suffix in ('.zip', '.sha256'):
                preserved.append((child.name, child.read_bytes()))
    if output_dir.exists():
        for child in output_dir.iterdir():
            if child.is_dir():
                shutil.rmtree(str(child), ignore_errors=True)
            else:
                child.unlink()
    else:
        output_dir.mkdir(parents=True)
    for child in work_dir.iterdir():
        dest = output_dir / child.name
        if child.is_dir():
            shutil.copytree(str(child), str(dest), dirs_exist_ok=False)
        else:
            shutil.copy2(str(child), str(dest))
    # Restore preserved ZIP and SHA files
    for name, data in preserved:
        (output_dir / name).write_bytes(data)


def update_runtime_version_in_lock(lock, version):
    lock["_runtime_version"] = version
    return lock


def module_relative_path(abs_path):
    p = pathlib.Path(abs_path)
    return p.relative_to(REPO_ROOT).as_posix()


def read_runtime_version_from_args(args):
    return args.runtime_version


def main():
    parser = argparse.ArgumentParser(description="Amitia Android PRoot Runtime Package Builder")
    parser.add_argument("--runtime-version", required=True)
    parser.add_argument("--commit", required=True)
    parser.add_argument("--clean", action="store_true", help="清理后构建")
    parser.add_argument("--offline", action="store_true", help="完全离线模式")
    parser.add_argument("--output-dir", default=str(DEFAULT_OUTPUT_DIR), type=pathlib.Path)
    parser.add_argument("--cache-dir", default=str(DEFAULT_CACHE_DIR), type=pathlib.Path)
    parser.add_argument("--rootfs-artifact", default=None)
    parser.add_argument("--guest-layout-artifact", default=None)
    parser.add_argument("--backend-artifact", default=None)
    parser.add_argument("--node-artifact", default=None)
    parser.add_argument("--node-scripts-artifact", default=None)
    parser.add_argument("--qdrant-artifact", default=None)
    parser.add_argument("--plugin-host-dir", default=str(REPO_ROOT / "runtime" / "plugin-host"))
    parser.add_argument("--task-host-dir", default=str(REPO_ROOT / "runtime" / "task-host"))
    parser.add_argument("--keep-work-dir", action="store_true")
    args = parser.parse_args()
    validate_version(args.runtime_version)
    validate_commit(args.commit)
    lock = load_lock()
    lock["_runtime_version"] = args.runtime_version
    components = lock["components"]
    if components["backend"].get("commit") and components["backend"]["commit"] != args.commit:
        pass
    for cid in ["pluginHost", "taskHost"]:
        if components[cid]["commit"] != args.commit:
            print(f"[警告] {cid} commit不一致: {components[cid]['commit']} != {args.commit}")
    for key in ["rootfs", "backend", "node", "nodeScripts", "qdrant", "guestLayout"]:
        comp = components[key]
        if "PENDING" in comp.get("sha256", ""):
            raise RuntimeError(f"组件SHA尚未锁定，请先运行update_lock.py: {key}")
        art = comp.get("artifact", "")
        art_path = (REPO_ROOT / art).resolve() if art and not pathlib.Path(art).is_absolute() else pathlib.Path(art) if art else None
        if not art_path or not art_path.exists():
            raise RuntimeError(f"组件Artifact不存在: {key} -> {art}")
        verify_component_sha(comp, REPO_ROOT)
    import script_host_packager
    plugin_host_result = script_host_packager.package_host(args.plugin_host_dir)
    task_host_result = script_host_packager.package_host(args.task_host_dir)
    validate_commit(args.commit)
    lock["_runtime_version"] = args.runtime_version
    work_dir = pathlib.Path(tempfile.mkdtemp(prefix="runtime_package_"))
    try:
        (work_dir / "payload" / "rootfs").mkdir(parents=True)
        (work_dir / "payload" / "runtime").mkdir(parents=True)
        (work_dir / "metadata").mkdir(parents=True)
        (work_dir / "licenses").mkdir(parents=True)
        rootfs_path, rootfs_sha = build_rootfs_payload(lock, work_dir, offline=args.offline)
        guest_layout_src = REPO_ROOT / "runtime" / "build" / "out" / "guest-layout" / "ubuntu-arm64" / "guest-layout.json"
        mount_contract_src = REPO_ROOT / "runtime" / "build" / "out" / "guest-layout" / "ubuntu-arm64" / "mount-contract.json"
        runtime_path, runtime_sha = build_runtime_payload(
            lock, work_dir, plugin_host_result["files"], task_host_result["files"],
            [("guest-layout.json", str(guest_layout_src)),
             ("mount-contract.json", str(mount_contract_src))],
            offline=args.offline
        )
        rootfs_name = pathlib.Path(rootfs_path).name
        runtime_name = pathlib.Path(runtime_path).name
        guest_payload_path = work_dir / "metadata" / "guest-layout.json"
        mount_payload_path = work_dir / "metadata" / "mount-contract.json"
        shutil.copy2(str(guest_layout_src), str(guest_payload_path))
        shutil.copy2(str(mount_contract_src), str(mount_payload_path))
        guest_layout_sha = hashlib.sha256(guest_payload_path.read_bytes()).hexdigest()
        mount_contract_sha = hashlib.sha256(mount_payload_path.read_bytes()).hexdigest()
        payload_index = {
            "rootfs": {
                "path": f"payload/rootfs/{rootfs_name}",
                "sha256": rootfs_sha,
                "size": pathlib.Path(rootfs_path).stat().st_size,
            },
            "runtime": {
                "path": f"payload/runtime/{runtime_name}",
                "sha256": runtime_sha,
                "size": pathlib.Path(runtime_path).stat().st_size,
            },
        }
        payload_index["guestLayoutSha"] = guest_layout_sha
        payload_index["guestLayoutSize"] = guest_payload_path.stat().st_size
        payload_index["mountContractSha"] = mount_contract_sha
        payload_index["mountContractSize"] = mount_payload_path.stat().st_size
        comp_lock_out = {
            "schemaVersion": 1,
            "runtimeVersion": args.runtime_version,
            "components": {}
        }
        for key in ["rootfs", "backend", "node", "nodeScripts", "qdrant", "guestLayout"]:
            c = components[key]
            comp_lock_out["components"][key] = {
                "componentId": c["componentId"],
                "version": c["version"],
                "sha256": c["sha256"],
                "platform": c["platform"],
                "architecture": c["architecture"],
            }
            if "artifact" in c:
                comp_lock_out["components"][key]["artifact"] = c["artifact"]
        comp_lock_out["components"]["pluginHost"] = {
            "componentId": components["pluginHost"]["componentId"],
            "source": components["pluginHost"]["source"],
            "entry": components["pluginHost"]["entry"],
            "commit": components["pluginHost"]["commit"],
            "treeSha256": plugin_host_result["tree_sha256"],
        }
        comp_lock_out["components"]["taskHost"] = {
            "componentId": components["taskHost"]["componentId"],
            "source": components["taskHost"]["source"],
            "entry": components["taskHost"]["entry"],
            "commit": components["taskHost"]["commit"],
            "treeSha256": task_host_result["tree_sha256"],
        }
        cl_path = work_dir / "metadata" / "component-lock.json"
        cl_path.write_text(json.dumps(comp_lock_out, indent=2, sort_keys=False, ensure_ascii=False) + "\n", encoding="utf-8")
        component_index = {
            "schemaVersion": 1,
            "components": sorted([
                {
                    "id": "runtime.backend",
                    "root": "backend",
                    "entry": "backend/amitia-server",
                    "version": components["backend"]["version"],
                    "sha256": components["backend"]["sha256"],
                },
                {
                    "id": "runtime.node",
                    "root": "node",
                    "entry": "node/bin/node",
                    "version": components["node"]["version"],
                    "sha256": components["node"]["sha256"],
                },
                {
                    "id": "runtime.qdrant",
                    "root": "qdrant",
                    "entry": "qdrant/bin/qdrant",
                    "version": components["qdrant"]["version"],
                    "sha256": components["qdrant"]["sha256"],
                },
                {
                    "id": "runtime.plugin-host",
                    "root": "plugin-host",
                    "entry": "plugin-host/dist/index.js",
                    "version": "1.0.0",
                    "sha256": plugin_host_result["tree_sha256"],
                },
                {
                    "id": "runtime.task-host",
                    "root": "task-host",
                    "entry": "task-host/dist/index.js",
                    "version": "1.0.0",
                    "sha256": task_host_result["tree_sha256"],
                },
            ], key=lambda x: x["id"]),
        }
        ci_path = work_dir / "metadata" / "component-index.json"
        ci_path.write_text(json.dumps(component_index, indent=2, sort_keys=False, ensure_ascii=False) + "\n", encoding="utf-8")
        file_manifest = generate_file_manifest(rootfs_path, runtime_path)
        for fm in file_manifest:
            full_path = work_dir / fm["path"]
            if full_path.exists():
                fm["size"] = full_path.stat().st_size
                fm["sha256"] = hashlib.sha256(full_path.read_bytes()).hexdigest()
        fm_path = work_dir / "metadata" / "file-manifest.json"
        fm_path.write_text(json.dumps(file_manifest, indent=2, sort_keys=False, ensure_ascii=False) + "\n", encoding="utf-8")
        third_party_src = REPO_ROOT / "THIRD_PARTY_NOTICES.md"
        if third_party_src.exists():
            shutil.copy2(str(third_party_src), str(work_dir / "licenses" / "THIRD_PARTY_NOTICES.md"))
        compute_sha256sums(work_dir)
        for cid, files_info in [("pluginHost", plugin_host_result), ("taskHost", task_host_result)]:
            meta = {
                "schemaVersion": 1,
                "componentId": f"runtime.{cid.lower()}",
                "entry": f"{cid.lower()}/dist/index.js",
                "sourceCommit": args.commit,
                "treeSha256": files_info["tree_sha256"],
            }
            meta_path = work_dir / "metadata" / f"{cid.lower()}.json"
            meta_path.write_text(json.dumps(meta, indent=2, sort_keys=False, ensure_ascii=False) + "\n", encoding="utf-8")
        import package_writer
        zip_payload = []
        for root, dirs, files in os.walk(str(work_dir)):
            for fname in files:
                full = pathlib.Path(root) / fname
                rel = full.relative_to(work_dir).as_posix()
                zip_payload.append({"name": rel, "data": full.read_bytes()})
        zip_name = f"amitia-runtime-{args.runtime_version}-android-arm64.zip"
        zip_path = args.output_dir / zip_name
        args.output_dir.mkdir(parents=True, exist_ok=True)
        zip_result, zip_sha = package_writer.write_zip(zip_payload, zip_path)
        sha_filename = f"{zip_name}.sha256"
        sha_file_path = args.output_dir / sha_filename
        package_writer.write_zip_sha_file = package_writer.write_sha_file
        package_writer.write_zip_sha_file(zip_sha, zip_name, sha_file_path)
        atomic_publish(work_dir, args.output_dir)
        print(f"构建成功:")
        print(f"  RootFS: {rootfs_path} ({rootfs_sha})")
        print(f"  Runtime: {runtime_path} ({runtime_sha})")
        print(f"  ZIP: {zip_path} ({zip_sha})")
        print(f"  输出目录: {args.output_dir}")
    finally:
        if not args.keep_work_dir:
            shutil.rmtree(str(work_dir), ignore_errors=True)


if __name__ == "__main__":
    main()
