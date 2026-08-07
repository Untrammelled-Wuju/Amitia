import argparse
import hashlib
import json
import os
import pathlib
import sys

SCRIPT_DIR = pathlib.Path(__file__).resolve().parent
LOCK_FILE = SCRIPT_DIR / "runtime-package.lock.json"
REPO_ROOT = SCRIPT_DIR.parent.parent.parent.parent


def load_lock():
    if not LOCK_FILE.is_file():
        raise RuntimeError(f"Lock文件不存在: {LOCK_FILE}")
    with open(LOCK_FILE, "r", encoding="utf-8") as f:
        return json.load(f)


def save_lock(data):
    with open(LOCK_FILE, "w", encoding="utf-8", newline="") as f:
        json.dump(data, f, indent=2, sort_keys=False, ensure_ascii=False)
        f.write("\n")


def compute_file_sha(path):
    h = hashlib.sha256()
    with open(path, "rb") as f:
        while True:
            chunk = f.read(1048576)
            if not chunk:
                break
            h.update(chunk)
    return h.hexdigest()


def compute_tree_sha(host_dir, entry_rel):
    base = pathlib.Path(host_dir).resolve()
    dist_dir = base / "dist"
    if not dist_dir.is_dir():
        raise RuntimeError(f"dist不存在: {dist_dir}")
    tree_entries = []
    for root, dirs, files in os.walk(str(dist_dir)):
        dirs[:] = [d for d in dirs if d not in ("src", "node_modules", "coverage", "tests")]
        for fname in files:
            fp = pathlib.Path(root) / fname
            rel = fp.relative_to(base).as_posix()
            st = fp.stat()
            sha = compute_file_sha(str(fp))
            tree_entries.append((rel, st.st_mode, st.st_size, sha))
    package_json = base / "package.json"
    if package_json.is_file():
        st = package_json.stat()
        sha = compute_file_sha(str(package_json))
        tree_entries.append(("package.json", st.st_mode, st.st_size, sha))
    tree_entries.sort(key=lambda x: x[0])
    h = hashlib.sha256()
    for rel, mode, size, sha in tree_entries:
        h.update(rel.encode("utf-8"))
        h.update(b"\x00")
        h.update(mode.to_bytes(4, "little"))
        h.update(size.to_bytes(8, "little"))
        h.update(sha.encode("utf-8"))
        h.update(b"\x00")
    return h.hexdigest()


def update_lock(runtime_version, commit):
    data = load_lock()
    components = data["components"]
    if "rootfs" in components:
        rootfs_artifact = REPO_ROOT / components["rootfs"]["artifact"]
        if rootfs_artifact.exists():
            components["rootfs"]["sha256"] = compute_file_sha(str(rootfs_artifact))
            print(f"[rootfs] SHA已更新: {components['rootfs']['sha256']}")
        else:
            print(f"[rootfs] Artifact不存在，跳过: {rootfs_artifact}")
    if "node" in components:
        node_artifact = REPO_ROOT / components["node"]["artifact"]
        if node_artifact.exists():
            components["node"]["sha256"] = compute_file_sha(str(node_artifact))
            print(f"[node] SHA已更新: {components['node']['sha256']}")
        else:
            print(f"[node] Artifact不存在，跳过: {node_artifact}")
    if "backend" in components:
        backend_artifact = REPO_ROOT / components["backend"]["artifact"]
        if backend_artifact.exists():
            components["backend"]["sha256"] = compute_file_sha(str(backend_artifact))
    if "guestLayout" in components:
        gl_artifact = REPO_ROOT / components["guestLayout"]["artifact"]
        if gl_artifact.exists():
            components["guestLayout"]["sha256"] = compute_file_sha(str(gl_artifact))
    if "nodeScripts" in components:
        ns_artifact = REPO_ROOT / components["nodeScripts"]["artifact"]
        if ns_artifact.exists():
            components["nodeScripts"]["sha256"] = compute_file_sha(str(ns_artifact))
    if "qdrant" in components:
        q_artifact = REPO_ROOT / components["qdrant"]["artifact"]
        if q_artifact.exists():
            components["qdrant"]["sha256"] = compute_file_sha(str(q_artifact))
            print(f"[qdrant] SHA已更新: {components['qdrant']['sha256']}")
        else:
            print(f"[qdrant] Artifact不存在，跳过: {q_artifact}")
    if "pluginHost" in components:
        plugin_path = REPO_ROOT / components["pluginHost"]["source"]
        if (plugin_path / "dist" / "index.js").exists():
            components["pluginHost"]["treeSha256"] = compute_tree_sha(str(plugin_path), "dist/index.js")
            components["pluginHost"]["commit"] = commit
            print(f"[pluginHost] Tree SHA已更新: {components['pluginHost']['treeSha256']}")
    if "taskHost" in components:
        task_path = REPO_ROOT / components["taskHost"]["source"]
        if (task_path / "dist" / "index.js").exists():
            components["taskHost"]["treeSha256"] = compute_tree_sha(str(task_path), "dist/index.js")
            components["taskHost"]["commit"] = commit
            print(f"[taskHost] Tree SHA已更新: {components['taskHost']['treeSha256']}")
    save_lock(data)
    print("Lock文件已更新")


def main():
    parser = argparse.ArgumentParser(description="更新 runtime-package.lock.json")
    parser.add_argument("--runtime-version", required=True, help="运行时包版本")
    parser.add_argument("--commit", required=True, help="完整Git Commit SHA")
    args = parser.parse_args()
    if len(args.commit) < 40 or not all(c in "0123456789abcdefABCDEF" for c in args.commit):
        print("非法的完整Commit SHA")
        sys.exit(1)
    update_lock(args.runtime_version, args.commit)


if __name__ == "__main__":
    main()
