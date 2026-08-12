import hashlib
import json
import os
import pathlib
import shutil
import stat

SCRIPT_DIR = pathlib.Path(__file__).resolve().parent
LOCK_FILE = SCRIPT_DIR / "rootfs.lock.json"
OVERLAYS_DIR = SCRIPT_DIR / "overlays"

BLOCKED_PACKAGES = {
    "systemd", "systemd-sysv", "snapd", "dbus", "udev", "cron",
    "rsyslog", "openssh-server", "sudo", "docker", "podman", "docker.io",
    "containerd", "runc", "network-manager", "cloud-init", "lxc",
}
BLOCKED_OVERLAY_TARGETS = {
    "opt/amitia", "var/lib/amitia", "var/log/amitia", "var/cache/amitia",
    "run/amitia", "workspace", "data", "runtime", "extensions", "plugins", "skills",
}


def load_lock():
    with open(LOCK_FILE, "r", encoding="utf-8") as f:
        data = json.load(f)
    required = [
        "schemaVersion", "componentId", "distribution", "flavor", "release",
        "codename", "architecture", "guestPlatform", "runtimeKind",
        "baseArchiveName", "baseArchiveSha256", "aptSnapshot",
        "aptComponents", "aptSuites", "defaultLocale", "defaultTimezone",
    ]
    for key in required:
        if key not in data:
            raise ValueError(f"锁文件缺少必填字段: {key}")
    sha = data["baseArchiveSha256"]
    if not isinstance(sha, str) or len(sha) != 64:
        raise ValueError("baseArchiveSha256 格式无效")
    try:
        int(sha, 16)
    except ValueError:
        raise ValueError("baseArchiveSha256 不是有效十六进制")
    if data.get("distribution") != "ubuntu":
        raise ValueError("distribution 必须为 ubuntu")
    if data.get("flavor") != "ubuntu-base":
        raise ValueError("flavor 必须为 ubuntu-base")
    if data.get("release") != "24.04.4":
        raise ValueError("release 必须为 24.04.4")
    if data.get("codename") != "noble":
        raise ValueError("codename 必须为 noble")
    if data.get("architecture") != "arm64":
        raise ValueError("architecture 必须为 arm64")
    if data.get("guestPlatform") != "linux":
        raise ValueError("guestPlatform 必须为 linux")
    if data.get("runtimeKind") != "proot":
        raise ValueError("runtimeKind 必须为 proot")
    if data.get("aptSnapshot") != "20260212T150000Z":
        raise ValueError("aptSnapshot 必须为 20260212T150000Z")
    return data


def sha256_file(path):
    h = hashlib.sha256()
    with open(path, "rb") as f:
        while True:
            chunk = f.read(1048576)
            if not chunk:
                break
            h.update(chunk)
    return h.hexdigest()


def safe_extract_archive(archive_path, extract_dir, expected_files=None):
    extract_dir = pathlib.Path(extract_dir)
    extract_dir.mkdir(parents=True, exist_ok=True)
    import tarfile
    with tarfile.open(archive_path, "r:gz") as tf:
        members = tf.getmembers()
        for m in members:
            if not m.name:
                raise RuntimeError("归档成员路径为空")
            if m.name.startswith("/"):
                raise RuntimeError(f"绝对路径成员: {m.name}")
            parts = pathlib.PurePosixPath(m.name).parts
            if ".." in parts:
                raise RuntimeError(f"路径穿越成员: {m.name}")
            rel = os.path.normpath(m.name)
            if rel.startswith(".."):
                raise RuntimeError(f"规范化后路径越界: {m.name}")
            if m.issym():
                link_target = m.linkname
                if link_target.startswith("/"):
                    normalized = "/" + link_target.lstrip("/")
                    if not normalized.startswith("/"):
                        raise RuntimeError(f"无效符号链接目标: {m.name} -> {link_target}")
                target_abs = os.path.normpath(os.path.join(os.path.dirname(m.name), link_target))
                if target_abs.startswith(".."):
                    raise RuntimeError(f"符号链接越界: {m.name} -> {link_target}")
            elif m.islnk():
                link_target = m.linkname
                if link_target.startswith("/"):
                    target_abs = os.path.normpath(link_target)
                else:
                    target_abs = os.path.normpath(os.path.join(os.path.dirname(m.name), link_target))
                if target_abs.startswith(".."):
                    raise RuntimeError(f"硬链接越界: {m.name} -> {link_target}")
            if m.ischr() or m.isblk() or m.isfifo() or m.isdev():
                if m.isdir():
                    target_path = extract_dir / m.name
                    target_path.mkdir(parents=True, exist_ok=True)
                    continue
                raise RuntimeError(f"设备/FIFO/Socket 成员: {m.name}")
        for m in members:
            if m.ischr() or m.isblk() or m.isfifo() or m.isdev():
                continue
            try:
                tf.extract(m, str(extract_dir), set_attrs=False)
            except (tarfile.TarError, OSError) as e:
                error_msg = str(e)
                if "absolute path" in error_msg and m.issym():
                    link_path = extract_dir / m.name
                    link_path.parent.mkdir(parents=True, exist_ok=True)
                    if link_path.exists() or link_path.is_symlink():
                        link_path.unlink()
                    os.symlink(m.linkname, link_path)
                else:
                    raise
            # Restore mode permissions (workaround for set_attrs=False)
            if m.isdir() or m.isfile() or m.islnk():
                extracted_path = extract_dir / m.name
                if extracted_path.exists():
                    os.chmod(extracted_path, m.mode)


def apply_overlay(overlays_dir, target_root):
    overlays_dir = pathlib.Path(overlays_dir)
    target_root = pathlib.Path(target_root)
    if not overlays_dir.exists():
        return
    blocked_targets = set(BLOCKED_OVERLAY_TARGETS)
    for rel_path in sorted(overlays_dir.rglob("*")):
        rel = rel_path.relative_to(overlays_dir)
        rel_str = rel.as_posix()
        if rel_str in blocked_targets or any(rel_str.startswith(b + "/") for b in blocked_targets):
            raise RuntimeError(f"Overlay 包含禁止目标: {rel_str}")
        if "node" in rel_str.lower() and ("bin/node" in rel_str or "node.exe" in rel_str):
            raise RuntimeError(f"Overlay 不允许包含 Node: {rel_str}")
        if "qdrant" in rel_str.lower() and "bin/qdrant" in rel_str:
            raise RuntimeError(f"Overlay 不允许包含 Qdrant: {rel_str}")
        target_path = target_root / rel
        if rel_path.is_dir():
            target_path.mkdir(parents=True, exist_ok=True)
        elif rel_path.is_symlink():
            target_path.parent.mkdir(parents=True, exist_ok=True)
            if target_path.exists() or target_path.is_symlink():
                target_path.unlink()
            link_target = os.readlink(rel_path)
            os.symlink(link_target, target_path)
        elif rel_path.is_file():
            target_path.parent.mkdir(parents=True, exist_ok=True)
            shutil.copy2(str(rel_path), str(target_path))


def fix_permissions(root):
    root = pathlib.Path(root)
    for dirpath, dirnames, filenames in os.walk(root, followlinks=False):
        dirp = pathlib.Path(dirpath)
        try:
            st = dirp.lstat()
            if not stat.S_ISLNK(st.st_mode):
                os.chmod(dirp, 0o755)
        except OSError:
            pass
        for fn in filenames:
            fp = dirp / fn
            try:
                st = fp.lstat()
                if stat.S_ISLNK(st.st_mode):
                    continue
                current_mode = stat.S_IMODE(st.st_mode)
                if current_mode & stat.S_ISUID:
                    current_mode &= ~stat.S_ISUID
                if current_mode & stat.S_ISGID:
                    current_mode &= ~stat.S_ISGID
                if fp.name in ("bash", "dash", "sh") and fp.parent.name in ("bin", "sbin", "usr/bin", "usr/sbin"):
                    current_mode |= 0o755
                else:
                    if current_mode & 0o100:
                        pass
                    else:
                        current_mode = current_mode & 0o666 | 0o444
                os.chmod(fp, current_mode)
            except OSError:
                pass


def create_mount_points(root):
    root = pathlib.Path(root)
    mount_points = [
        ("dev", 0o755),
        ("dev/pts", 0o755),
        ("proc", 0o555),
        ("sys", 0o555),
        ("run", 0o755),
        ("tmp", 0o1777),
        ("var/tmp", 0o1777),
    ]
    for rel, mode in mount_points:
        p = root / rel
        p.mkdir(parents=True, exist_ok=True)
        try:
            os.chmod(p, mode)
        except OSError:
            pass
    var_run = root / "var" / "run"
    if var_run.exists() and not var_run.is_symlink():
        shutil.rmtree(var_run, ignore_errors=True)
    if var_run.is_symlink() or not var_run.exists():
        if var_run.is_symlink():
            var_run.unlink()
        try:
            os.symlink("../run", var_run)
        except (OSError, NotImplementedError):
            var_run.mkdir(parents=True, exist_ok=True)
            try:
                os.chmod(var_run, 0o755)
            except OSError:
                pass


def setup_users(root, proot_runner):
    root = pathlib.Path(root)
    passwd_file = root / "etc" / "passwd"
    shadow_file = root / "etc" / "shadow"
    group_file = root / "etc" / "group"
    passwd_lines = []
    root_home = "/root"
    if passwd_file.exists():
        with open(passwd_file, "r", encoding="utf-8") as f:
            for line in f:
                line = line.strip()
                if not line or line.startswith("#"):
                    continue
                parts = line.split(":")
                if len(parts) >= 7:
                    if parts[0] != "root":
                        passwd_lines.append(line)
    passwd_lines.append(f"root:x:0:0:root:{root_home}:/bin/bash")
    passwd_lines.append("amitia:x:1000:1000:amitia:/home/amitia:/bin/bash")
    passwd_file.parent.mkdir(parents=True, exist_ok=True)
    with open(passwd_file, "w", encoding="utf-8", newline="") as f:
        f.write("\n".join(sorted(set(passwd_lines))) + "\n")
    shadow_lines = []
    if shadow_file.exists():
        with open(shadow_file, "r", encoding="utf-8") as f:
            for line in f:
                line = line.strip()
                if not line or line.startswith("#"):
                    continue
                parts = line.split(":", 2)
                if len(parts) >= 2 and parts[0] != "root" and parts[0] != "amitia":
                    shadow_lines.append(line)
    shadow_lines.append("root:!:1::::::")
    shadow_lines.append("amitia:!:1::::::")
    with open(shadow_file, "w", encoding="utf-8", newline="") as f:
        f.write("\n".join(sorted(set(shadow_lines))) + "\n")
    group_lines = []
    if group_file.exists():
        with open(group_file, "r", encoding="utf-8") as f:
            for line in f:
                line = line.strip()
                if not line or line.startswith("#"):
                    continue
                parts = line.split(":")
                if len(parts) >= 3:
                    if parts[0] != "root" and parts[0] != "amitia":
                        group_lines.append(line)
    group_lines.append("root:x:0:")
    group_lines.append("amitia:x:1000:")
    with open(group_file, "w", encoding="utf-8", newline="") as f:
        f.write("\n".join(sorted(set(group_lines))) + "\n")
    home_dir = root / "home" / "amitia"
    home_dir.mkdir(parents=True, exist_ok=True)
    try:
        os.chown(home_dir, 1000, 1000)
    except (OSError, AttributeError):
        pass
    os.chmod(home_dir, 0o755)

    localtime_src = root / "usr" / "share" / "zoneinfo" / "Etc" / "UTC"
    localtime_dst = root / "etc" / "localtime"
    if localtime_src.exists():
        if localtime_dst.exists() or localtime_dst.is_symlink():
            localtime_dst.unlink()
        shutil.copy2(str(localtime_src), str(localtime_dst))
    timezone_file = root / "etc" / "timezone"
    timezone_file.parent.mkdir(parents=True, exist_ok=True)
    with open(timezone_file, "w", encoding="utf-8") as f:
        f.write("Etc/UTC\n")


def cleanup_cache(root):
    root = pathlib.Path(root)
    cache_paths = [
        "var/cache/apt/archives",
        "var/lib/apt/lists",
        "var/log/apt",
        "var/log/dpkg.log",
        "var/log/alternatives.log",
        "var/cache/debconf",
    ]
    for rel in cache_paths:
        p = root / rel
        if p.exists():
            if p.is_dir():
                shutil.rmtree(p, ignore_errors=True)
                p.mkdir(parents=True, exist_ok=True)
            else:
                p.unlink()
    machine_id_paths = [
        "etc/machine-id",
        "var/lib/dbus/machine-id",
        "var/lib/systemd/random-seed",
    ]
    for rel in machine_id_paths:
        p = root / rel
        if p.exists() or p.is_symlink():
            p.unlink()
        if "machine-id" in rel:
            p.parent.mkdir(parents=True, exist_ok=True)
            with open(p, "w", encoding="utf-8") as f:
                f.write("")
    history_paths = [
        "root/.bash_history",
        "home/amitia/.bash_history",
    ]
    for rel in history_paths:
        p = root / rel
        if p.exists():
            p.unlink()
    for pycache_dir in root.rglob("__pycache__"):
        if pycache_dir.is_dir():
            shutil.rmtree(pycache_dir, ignore_errors=True)
    for ds_store in root.rglob(".DS_Store"):
        if ds_store.is_file():
            ds_store.unlink()
    for zone_identifier in root.rglob("*Zone.Identifier"):
        if zone_identifier.is_file():
            zone_identifier.unlink()
    tmp_dirs = ["tmp", "var/tmp"]
    for rel in tmp_dirs:
        tmp_dir = root / rel
        if tmp_dir.exists():
            for item in tmp_dir.iterdir():
                if item.is_dir():
                    shutil.rmtree(item, ignore_errors=True)
                else:
                    item.unlink()
    dirs_to_remove = [
        "usr/share/doc",
        "usr/share/man",
        "usr/share/info",
        "usr/share/locale",
    ]
    for rel in dirs_to_remove:
        p = root / rel
        if p.exists():
            shutil.rmtree(p, ignore_errors=True)


def scan_forbidden_files(root):
    root = pathlib.Path(root)
    issues = []
    if (root / "sbin" / "init").exists():
        init_target = os.readlink(root / "sbin" / "init")
        if "systemd" in init_target:
            issues.append("检测到 systemd init")
    bin_paths = list((root / "usr" / "bin").rglob("*")) + list((root / "usr" / "sbin").rglob("*"))
    for fp in bin_paths:
        if fp.is_file() and not fp.is_symlink():
            try:
                st = fp.stat()
                if st.st_mode & stat.S_ISUID:
                    issues.append(f"SetUID 文件: {fp.relative_to(root)}")
                if st.st_mode & stat.S_ISGID:
                    issues.append(f"SetGID 文件: {fp.relative_to(root)}")
                if fp.name in ("node",) and "node" in str(fp).lower():
                    issues.append(f"检测到 Node: {fp.relative_to(root)}")
                if fp.name in ("qdrant",) and "qdrant" in str(fp).lower():
                    issues.append(f"检测到 Qdrant: {fp.relative_to(root)}")
            except OSError:
                pass
    for special in ["dev", "proc", "sys"]:
        special_dir = root / special
        if special_dir.exists():
            for item in special_dir.iterdir():
                if item.is_dir() and item.name not in ("pts",):
                    continue
                try:
                    st = item.stat()
                    if stat.S_ISFIFO(st.st_mode) or stat.S_ISBLK(st.st_mode) or stat.S_ISCHR(st.st_mode):
                        if item.name not in ("null", "zero", "random", "urandom", "tty", "ptmx"):
                            if special == "dev":
                                pass
                        issues.append(f"{special} 中存在特殊节点: {item.name}")
                except OSError:
                    pass
    return issues


def build_file_manifest(root):
    root = pathlib.Path(root)
    entries = []
    all_paths = []
    for dirpath, dirnames, filenames in os.walk(root, followlinks=False):
        dirnames.sort()
        dirp = pathlib.Path(dirpath)
        rel_dir = dirp.relative_to(root)
        for dn in sorted(dirnames):
            dp = dirp / dn
            rel = (rel_dir / dn).as_posix() if str(rel_dir) != "." else dn
            all_paths.append((rel, dp))
        for fn in sorted(filenames):
            fp = dirp / fn
            rel = (rel_dir / fn).as_posix() if str(rel_dir) != "." else fn
            all_paths.append((rel, fp))
    for rel, fp in all_paths:
        try:
            st = fp.lstat()
        except OSError:
            continue
        if fp.is_symlink():
            target = os.readlink(fp)
            if fp.is_dir():
                entries.append({
                    "path": rel,
                    "type": "dirsymlink",
                    "target": target,
                    "mode": oct(stat.S_IMODE(st.st_mode)),
                    "uid": st.st_uid,
                    "gid": st.st_gid,
                })
            else:
                entries.append({
                    "path": rel,
                    "type": "symlink",
                    "target": target,
                    "mode": oct(stat.S_IMODE(st.st_mode)),
                    "uid": st.st_uid,
                    "gid": st.st_gid,
                })
        elif fp.is_dir():
            entries.append({
                "path": rel,
                "type": "directory",
                "mode": oct(stat.S_IMODE(st.st_mode)),
                "uid": st.st_uid,
                "gid": st.st_gid,
            })
        elif fp.is_file():
            entries.append({
                "path": rel,
                "type": "file",
                "size": st.st_size,
                "sha256": sha256_file(fp),
                "mode": oct(stat.S_IMODE(st.st_mode)),
                "uid": st.st_uid,
                "gid": st.st_gid,
            })
    entries.sort(key=lambda e: e["path"])
    return entries


def check_base_structure(root):
    root = pathlib.Path(root)
    required_dirs = [
        "bin", "boot", "dev", "etc", "home", "lib", "media", "mnt",
        "opt", "proc", "root", "run", "sbin", "srv", "sys", "tmp", "usr", "var",
    ]
    missing = []
    for d in required_dirs:
        p = root / d
        if not p.exists():
            missing.append(d)
    return missing


def check_merged_usr(root):
    root = pathlib.Path(root)
    checks = [
        ("usr/bin", "bin"),
        ("usr/sbin", "sbin"),
        ("usr/lib", "lib"),
    ]
    issues = []
    for required, link_name in checks:
        if not (root / required).exists():
            issues.append(f"缺少 {required}")
        link_path = root / link_name
        if link_path.is_symlink():
            target = os.readlink(link_path)
            if target != f"usr/{link_name}":
                issues.append(f"{link_name} 符号链接目标错误: {target} != usr/{link_name}")
        elif not link_path.exists():
            issues.append(f"缺少 {link_name} 符号链接")
    return issues
