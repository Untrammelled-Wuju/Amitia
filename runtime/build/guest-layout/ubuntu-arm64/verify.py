import argparse
import hashlib
import json
import lzma
import os
import pathlib
import re
import tarfile

SCRIPT_DIR = pathlib.Path(__file__).resolve().parent
LOCK_FILE = SCRIPT_DIR / "guest-layout.lock.json"
DEFAULT_OUTPUT_DIR = pathlib.Path(__file__).resolve().parent.parent.parent / "out" / "guest-layout" / "ubuntu-arm64"
ARCHIVE_NAME = "amitia-guest-layout-v1-ubuntu-arm64.tar.xz"
ALLOWED_PERSISTENCE = {"immutable", "persistent-critical", "persistent-diagnostic", "rebuildable", "ephemeral"}
ALLOWED_PURPOSE = {
    "runtime-root", "backend", "node-runtime", "qdrant-runtime", "plugin-host", "task-host",
    "runtime-scripts", "runtime-manifest", "licenses",
    "config-root", "app-config", "runtime-config", "provider-config",
    "data-root", "app-data", "databases", "provider-data",
    "qdrant-storage", "qdrant-snapshots", "qdrant-migration",
    "node-data", "extensions", "tasks", "workspaces", "state",
    "cache-root", "npm-cache", "extension-cache", "runtime-cache",
    "log-root", "backend-log", "provider-log", "extension-log",
    "run-root", "temp", "locks", "sockets",
}
ALLOWED_SOURCE_CLASS = {"runtime-package", "config", "data", "cache", "logs", "run"}
ALLOWED_MODES = {"0755", "0750", "0700", "0644"}
FORBIDDEN_PATHS = {
    "/", "/proc", "/dev", "/sys", "/system", "/vendor",
    "/data", "/sdcard", "/storage", "/storage/emulated",
    "/data/user", "/data/data",
}
FORBIDDEN_ROOT_ALIASES = {"/runtime", "/data", "/workspace", "/amitia"}
AMITIA_UID = 1000
AMITIA_GID = 1000


def load_json(path):
    with open(path, "r", encoding="utf-8") as f:
        return json.load(f)


def validate_linux_path(path):
    if not isinstance(path, str):
        return "path must be string"
    if not path.startswith("/"):
        return "path must start with /"
    if "\\" in path:
        return "path must not contain backslash"
    if "\x00" in path:
        return "path must not contain NUL"
    if path != "/":
        if path.endswith("/"):
            return "non-root path must not end with /"
    segments = [s for s in path.split("/") if s != ""]
    if "." in segments:
        return "path must not contain . segment"
    if ".." in segments:
        return "path must not contain .. segment"
    if "//" in path:
        return "path must not contain //"
    if path in FORBIDDEN_PATHS:
        return f"path is forbidden: {path}"
    for alias in FORBIDDEN_ROOT_ALIASES:
        if path == alias or (path.startswith(alias + "/")):
            return f"path uses forbidden alias: {alias}"
    return None


def paths_overlap(p1, p2):
    if p1 == p2:
        return True
    if p1.startswith(p2 + "/"):
        return True
    if p2.startswith(p1 + "/"):
        return True
    return False


def validate_lock_structure(data):
    errors = []
    required_top = ["schemaVersion", "componentId", "version", "distribution", "release",
                    "codename", "platform", "architecture", "runtimeKind", "user", "paths",
                    "components", "mountContract", "directories"]
    for key in required_top:
        if key not in data:
            errors.append(f"missing top-level key: {key}")
    if data.get("distribution") != "ubuntu":
        errors.append("distribution must be ubuntu")
    if data.get("release") != "24.04.4":
        errors.append("release must be 24.04.4")
    if data.get("codename") != "noble":
        errors.append("codename must be noble")
    if data.get("platform") != "linux":
        errors.append("platform must be linux")
    if data.get("architecture") != "arm64":
        errors.append("architecture must be arm64")
    if data.get("runtimeKind") != "proot":
        errors.append("runtimeKind must be proot")
    user = data.get("user", {})
    if user.get("uid") != AMITIA_UID:
        errors.append(f"user.uid must be {AMITIA_UID}")
    if user.get("gid") != AMITIA_GID:
        errors.append(f"user.gid must be {AMITIA_GID}")
    return errors


def validate_paths_section(paths):
    errors = []
    required_roots = ["runtimeRoot", "configRoot", "dataRoot", "cacheRoot",
                     "logRoot", "runRoot", "tempRoot", "workspaceRoot"]
    for key in required_roots:
        if key not in paths:
            errors.append(f"paths.{key} is required")
            continue
        err = validate_linux_path(paths[key])
        if err:
            errors.append(f"paths.{key}: {err}")
    rt = paths.get("runtimeRoot")
    if rt != "/opt/amitia":
        errors.append("runtimeRoot must be /opt/amitia")
    return errors


def validate_directories(dirs):
    errors = []
    seen_paths = {}
    root_roots = {"/opt/amitia", "/etc/amitia", "/var/lib/amitia",
                  "/var/cache/amitia", "/var/log/amitia", "/run/amitia"}
    root_set_seen = set()
    for i, d in enumerate(dirs):
        prefix = f"directories[{i}]"
        for key in ["path", "ownerUid", "ownerGid", "mode", "persistence", "purpose"]:
            if key not in d:
                errors.append(f"{prefix}: missing {key}")
        p = d.get("path", "")
        err = validate_linux_path(p)
        if err and p:
            errors.append(f"{prefix}.path: {err}")
        if p in seen_paths:
            errors.append(f"{prefix}.path duplicate: {p}")
        seen_paths[p] = True
        if d.get("persistence") not in ALLOWED_PERSISTENCE:
            errors.append(f"{prefix}.persistence invalid: {d.get('persistence')}")
        if d.get("purpose") not in ALLOWED_PURPOSE:
            errors.append(f"{prefix}.purpose invalid: {d.get('purpose')}")
        mode = d.get("mode")
        if mode not in ALLOWED_MODES:
            errors.append(f"{prefix}.mode invalid: {mode}")
        uid = d.get("ownerUid")
        gid = d.get("ownerGid")
        if uid not in (0, AMITIA_UID):
            errors.append(f"{prefix}.ownerUid invalid: {uid}")
        if gid not in (0, AMITIA_GID):
            errors.append(f"{prefix}.ownerGid invalid: {gid}")
        if p in root_roots:
            root_set_seen.add(p)
    missing_roots = root_roots - root_set_seen
    for r in missing_roots:
        errors.append(f"missing root directory: {r}")
    roots_list = list(root_roots)
    for i in range(len(roots_list)):
        for j in range(i + 1, len(roots_list)):
            if paths_overlap(roots_list[i], roots_list[j]):
                errors.append(f"root overlap: {roots_list[i]} vs {roots_list[j]}")
    ws_root = "/var/lib/amitia/workspaces"
    tmp_root = "/run/amitia/tmp"
    seen_set = set(seen_paths.keys())
    if not any(p.startswith("/var/lib/amitia/") for p in seen_set if p == "/var/lib/amitia/workspaces"):
        errors.append("workspaces must be under data root")
    if not any(p.startswith("/run/amitia/") for p in seen_set if p == "/run/amitia/tmp"):
        errors.append("temp must be under run root")
    return errors


def validate_mount_contract(mounts):
    errors = []
    if len(mounts) < 6:
        errors.append(f"expected 6 mounts, got {len(mounts)}")
    required_ids = ["runtime", "config", "data", "cache", "logs", "run"]
    seen_ids = set()
    seen_targets = set()
    for i, m in enumerate(mounts):
        prefix = f"mounts[{i}]"
        for key in ["id", "guestTarget", "persistence", "required", "sourceClass"]:
            if key not in m:
                errors.append(f"{prefix}: missing {key}")
        mid = m.get("id", "")
        if mid in seen_ids:
            errors.append(f"{prefix}: duplicate id: {mid}")
        seen_ids.add(mid)
        if mid not in required_ids:
            errors.append(f"{prefix}: unexpected id: {mid}")
        gt = m.get("guestTarget", "")
        if gt:
            if gt == "/":
                errors.append(f"{prefix}: guestTarget must not be /")
            if gt.startswith(("/proc", "/dev", "/sys", "/data", "/sdcard", "/storage")):
                errors.append(f"{prefix}: forbidden target: {gt}")
        if gt in seen_targets:
            errors.append(f"{prefix}: duplicate target: {gt}")
        seen_targets.add(gt)
        if m.get("persistence") not in ALLOWED_PERSISTENCE:
            errors.append(f"{prefix}: invalid persistence: {m.get('persistence')}")
        if m.get("sourceClass") not in ALLOWED_SOURCE_CLASS:
            errors.append(f"{prefix}: invalid sourceClass: {m.get('sourceClass')}")
        if not isinstance(m.get("required"), bool):
            errors.append(f"{prefix}: required must be bool")
        if "hostSource" in m or "host_source" in m:
            errors.append(f"{prefix}: must not contain host source")
    if "runtime" in seen_ids:
        if mounts[0].get("id") != "runtime":
            errors.append("runtime must be first mount")
    for expected_id in required_ids:
        if expected_id not in seen_ids:
            errors.append(f"missing mount id: {expected_id}")
    return errors


def validate_overlay_archive(archive_path):
    errors = []
    if not os.path.exists(archive_path):
        return [f"archive not found: {archive_path}"]
    with lzma.open(archive_path, "rb") as xz:
        with tarfile.open(fileobj=xz, mode="r:") as tf:
            members = tf.getmembers()
            for m in members:
                if m.name.startswith("/"):
                    errors.append(f"absolute path in archive: {m.name}")
                if m.name.startswith("overlay/") or m.name.startswith("guest-layout/") or m.name.startswith("rootfs/"):
                    errors.append(f"forbidden top-level dir: {m.name}")
                if ".." in m.name.split("/"):
                    errors.append(f"path traversal in archive: {m.name}")
                if m.issym() or m.islnk():
                    linktarget = m.linkname or ""
                    if linktarget.startswith("/"):
                        errors.append(f"absolute link target: {m.name} -> {linktarget}")
                    if ".." in linktarget.split("/"):
                        errors.append(f"link traversal: {m.name} -> {linktarget}")
            names = sorted(m.name for m in members)
            actual_names = [m.name for m in members]
            if actual_names != names:
                errors.append("archive members not sorted by path")
            first_segments = set()
            for n in actual_names:
                parts = n.split("/")
                if parts:
                    first_segments.add(parts[0])
            expected_firsts = {"opt", "etc", "var", "run"}
            for s in first_segments:
                if s not in expected_firsts:
                    errors.append(f"unexpected top-level segment: {s}")
    return errors


def sha256_file(path):
    h = hashlib.sha256()
    with open(path, "rb") as f:
        while True:
            c = f.read(1048576)
            if not c:
                break
            h.update(c)
    return h.hexdigest()


def verify_static(output_dir):
    output_dir = pathlib.Path(output_dir)
    errors = []
    lock_path = SCRIPT_DIR / "guest-layout.lock.json"
    if not lock_path.exists():
        errors.append("guest-layout.lock.json not found")
        return errors
    data = load_json(lock_path)
    errors.extend(validate_lock_structure(data))
    errors.extend(validate_paths_section(data.get("paths", {})))
    errors.extend(validate_directories(data.get("directories", [])))
    errors.extend(validate_mount_contract(data.get("mountContract", [])))
    overlay_opt = output_dir / "overlay" / "opt" / "amitia"
    if not overlay_opt.exists():
        errors.append("overlay/opt/amitia not generated")
    for name in ("guest-layout.json", "mount-contract.json", "file-manifest.json", "SHA256SUMS"):
        if not (output_dir / name).exists():
            errors.append(f"missing output: {name}")
    archive_path = output_dir / ARCHIVE_NAME
    if archive_path.exists():
        errors.extend(validate_overlay_archive(archive_path))
        manifest_path = output_dir / "file-manifest.json"
        if manifest_path.exists():
            manifest = load_json(manifest_path)
            if not isinstance(manifest, list):
                errors.append("file-manifest.json must be array")
            else:
                for entry in manifest:
                    if entry.get("type") != "directory":
                        errors.append(f"unexpected manifest entry type: {entry.get('type')}")
    return errors


def verify_integration(rootfs_path, output_dir):
    rootfs_path = pathlib.Path(rootfs_path) if rootfs_path else None
    output_dir = pathlib.Path(output_dir)
    errors = []
    if not rootfs_path or not rootfs_path.exists():
        errors.append("rootfs path not found for integration test")
        return errors
    lock_path = SCRIPT_DIR / "guest-layout.lock.json"
    if not lock_path.exists():
        errors.append("guest-layout.lock.json not found")
        return errors
    data = load_json(lock_path)
    system_dirs = ["bin", "sbin", "usr", "lib", "lib64", "proc", "dev", "sys", "etc", "var", "run", "tmp", "home", "root", "boot", "mnt", "opt", "media", "srv", "snap"]
    for sd in system_dirs:
        if sd == "opt":
            continue
        p = rootfs_path / sd
        if not p.exists():
            errors.append(f"system directory missing: /{sd}")
    home_amitia = rootfs_path / "home" / "amitia"
    if not home_amitia.exists():
        errors.append("/home/amitia missing in base rootfs")
    var_run = rootfs_path / "var" / "run"
    run_dir = rootfs_path / "run"
    if not (var_run.is_symlink() or var_run.exists() or run_dir.exists()):
        errors.append("var/run link missing")
    overlay_dir = output_dir / "overlay"
    if not overlay_dir.exists():
        errors.append("overlay directory missing")
        return errors
    for root, dirs, files in os.walk(str(overlay_dir)):
        for f in files:
            if f in (".keep", ".gitkeep", "placeholder", "README"):
                errors.append(f"placeholder file found: {os.path.join(root, f)}")
    return errors


def main():
    parser = argparse.ArgumentParser(description="Guest Layout verifier")
    parser.add_argument("--mode", choices=["static", "integration"], required=True)
    parser.add_argument("--overlay", type=str, default=None)
    parser.add_argument("--artifact", type=str, default=None)
    parser.add_argument("--rootfs", type=str, default=None)
    parser.add_argument("--report", type=str, default=None)
    parser.add_argument("--output-dir", type=str, default=str(DEFAULT_OUTPUT_DIR))
    args = parser.parse_args()
    if args.mode == "static":
        errors = verify_static(args.output_dir)
    elif args.mode == "integration":
        errors = verify_integration(args.rootfs, args.output_dir)
    else:
        errors = ["unknown mode"]
    if errors:
        for e in errors:
            print(f"[ERROR] {e}")
        raise SystemExit(1)
    print(f"OK: {args.mode} verification passed")


if __name__ == "__main__":
    main()
