import argparse
import hashlib
import json
import os
import re
import stat
import sys
from pathlib import Path
from typing import Dict, List, Optional, Tuple

try:
    from .errors import ValidationErrorCode, ValidationException
except ImportError:
    from errors import ValidationErrorCode, ValidationException

ROOTFS_REQUIRED_DIRS = [
    "bin", "boot", "dev", "etc", "home", "lib", "media", "mnt",
    "opt", "proc", "root", "run", "sbin", "srv", "sys", "tmp", "usr", "var",
]

ROOTFS_GUEST_LAYOUT_DIRS = [
    "/opt/amitia",
    "/etc/amitia",
    "/var/lib/amitia",
    "/var/cache/amitia",
    "/var/log/amitia",
    "/run/amitia",
    "/home/amitia",
    "/tmp",
]

ROOTFS_REQUIRED_PATHS = [
    "/bin/sh",
    "/usr/bin/env",
    "/etc/ssl/certs",
]

ROOTFS_FORBIDDEN_EXECUTABLES = [
    "/usr/sbin/sshd",
    "/usr/bin/sudo",
]

ROOTFS_LOADER_CANDIDATES = [
    "/lib/ld-linux-aarch64.so.1",
    "/lib/ld-linux-aarch64.so",
]

ROOTFS_REQUIRED_LIBRARIES = [
    "libstdc++.so",
    "libgcc_s.so",
    "libpthread.so",
    "libdl.so",
    "librt.so",
    "libm.so",
]

ROOTFS_COMPILER_PATTERNS = [
    "gcc", "g++", "clang", "make", "cmake", "rustc", "cargo", "go ", "javac",
    "build-essential",
]

ROOTFS_SECRET_PATTERNS = [
    re.compile(r'(?i)(token|secret|password|private[_-]?key)\s*[:=]\s*\S+'),
    re.compile(r'(?i)AKIA[0-9A-Z]{16}'),
    re.compile(r'(?i)npm_[a-zA-Z0-9]{36}'),
    re.compile(r'(?i)netrc'),
]


def sha256_file(path: str) -> str:
    digest = hashlib.sha256()
    with open(path, "rb") as f:
        for chunk in iter(lambda: f.read(65536), b""):
            digest.update(chunk)
    return digest.hexdigest()


def compute_tree_sha(rootfs_path: str) -> str:
    root = Path(rootfs_path)
    hasher = hashlib.sha256()
    for dirpath, dirnames, filenames in os.walk(root, followlinks=False):
        dirnames.sort()
        for fn in sorted(filenames):
            full = Path(dirpath) / fn
            rel = str(full.relative_to(root))
            hasher.update(rel.encode("utf-8"))
            hasher.update(b"\n")
            st = full.lstat()
            if stat.S_ISLNK(st.st_mode):
                target = os.readlink(full)
                hasher.update(f"symlink:{target}".encode("utf-8"))
            elif stat.S_ISREG(st.st_mode):
                hasher.update(f"reg:{st.st_size}:".encode("utf-8"))
                hasher.update(sha256_file(str(full)).encode("utf-8"))
            hasher.update(b"\n")
    return hasher.hexdigest()


def compute_files_sha256(rootfs_path: str) -> Tuple[str, Dict[str, str]]:
    root = Path(rootfs_path)
    file_hashes: Dict[str, str] = {}
    for dirpath, dirnames, filenames in os.walk(root, followlinks=False):
        dirnames.sort()
        for fn in sorted(filenames):
            full = Path(dirpath) / fn
            if full.is_symlink() or not full.is_file():
                continue
            rel = str(full.relative_to(root))
            file_hashes[rel] = sha256_file(str(full))
    hasher = hashlib.sha256()
    for rel in sorted(file_hashes.keys()):
        hasher.update(f"{file_hashes[rel]}  {rel}\n".encode("utf-8"))
    return hasher.hexdigest(), file_hashes


def compute_fld_manifest(rootfs_path: str) -> Tuple[str, bytes]:
    root = Path(rootfs_path)
    lines: List[str] = []
    for dirpath, dirnames, filenames in os.walk(root, followlinks=False):
        dirnames.sort()
        entries: List[Tuple[str, str, str]] = []
        dp = Path(dirpath)
        dir_rel = str(dp.relative_to(root))
        if dir_rel != ".":
            entries.append(("D", dir_rel, "-"))
        for dn in sorted(direntries := dirnames):
            entries.append(("D", str((dp / dn).relative_to(root)), "-"))
        for fn in sorted(filenames):
            full = dp / fn
            rel = str(full.relative_to(root))
            st = full.lstat()
            if stat.S_ISLNK(st.st_mode):
                target = os.readlink(full)
                perm = oct(stat.S_IMODE(st.st_mode))[2:]
                entries.append(("L", rel, target))
            elif stat.S_ISREG(st.st_mode):
                perm = oct(stat.S_IMODE(st.st_mode))[2:]
                sha = sha256_file(str(full))
                entries.append(("F", rel, f"{perm} {sha}"))
            elif stat.S_ISDIR(st.st_mode):
                perm = oct(stat.S_IMODE(st.st_mode))[2:]
                entries.append(("D", rel, perm))
        entries.sort(key=lambda x: x[1])
        for mode, rel, payload in entries:
            if mode == "D":
                lines.append(f"D {payload} {rel}" if payload != "-" else f"D - {rel}")
            elif mode == "L":
                lines.append(f"L {payload} {rel}")
            elif mode == "F":
                lines.append(f"F {payload} {rel}")
    content = "\n".join(lines) + "\n"
    return hashlib.sha256(content.encode("utf-8")).hexdigest(), content.encode("utf-8")


def validate_lock_format(lock_data: dict) -> List[str]:
    errors = []
    required_fields = [
        "schemaVersion", "component", "distribution", "release",
        "architecture", "archiveFileName", "sourceUrl", "sha256",
    ]
    for field in required_fields:
        if field not in lock_data:
            errors.append(f"Missing lock field: {field}")
    if lock_data.get("component") != "ubuntu-rootfs":
        errors.append(f"Invalid component: {lock_data.get('component')}")
    if lock_data.get("distribution") != "ubuntu":
        errors.append(f"Invalid distribution: {lock_data.get('distribution')}")
    if lock_data.get("architecture") != "arm64":
        errors.append(f"Invalid architecture: {lock_data.get('architecture')}")
    sha = lock_data.get("sha256", "")
    if not isinstance(sha, str) or len(sha) != 64:
        errors.append(f"Invalid sha256 format: {sha}")
    else:
        try:
            int(sha, 16)
        except ValueError:
            errors.append(f"sha256 is not valid hex: {sha}")
    source_url = lock_data.get("sourceUrl", "")
    if "latest" in source_url.lower():
        errors.append("sourceUrl must not contain 'latest'")
    if not source_url.startswith("https://cdimage.ubuntu.com"):
        errors.append(f"sourceUrl must be from Ubuntu official source: {source_url}")
    return errors


def validate_required_paths(root: Path) -> List[str]:
    errors = []
    for req_path in ROOTFS_REQUIRED_PATHS:
        if not (root / req_path.lstrip("/")).exists():
            errors.append(f"Missing required path: {req_path}")
    return errors


def validate_required_dirs(root: Path) -> List[str]:
    errors = []
    for req_dir in ROOTFS_REQUIRED_DIRS:
        if not (root / req_dir).exists():
            errors.append(f"Missing required directory: {req_dir}")
    return errors


def validate_guest_layout_dirs(root: Path, policy: Optional[dict] = None) -> List[str]:
    errors = []
    required_guest_dirs = ROOTFS_GUEST_LAYOUT_DIRS
    if policy and "requiredGuestDirs" in policy:
        required_guest_dirs = policy["requiredGuestDirs"]
    for guest_dir in required_guest_dirs:
        full_path = root / guest_dir.lstrip("/")
        if not full_path.exists():
            errors.append(f"Missing guest layout directory: {guest_dir}")
    return errors


def validate_loader_exists(root: Path, lock_data: Optional[dict] = None) -> List[str]:
    errors = []
    loader_candidates = ROOTFS_LOADER_CANDIDATES
    if lock_data and lock_data.get("guestLoader"):
        loader_candidates = [lock_data["guestLoader"]] + loader_candidates
    loader_found = any((root / cand.lstrip("/")).exists() for cand in loader_candidates)
    if not loader_found:
        errors.append(f"Missing ARM64 glibc loader (expected one of: {loader_candidates})")
    return errors


def validate_required_libraries(root: Path, policy: Optional[dict] = None) -> List[str]:
    errors = []
    required_libs = ROOTFS_REQUIRED_LIBRARIES
    if policy and "requiredLibraries" in policy:
        required_libs = policy["requiredLibraries"]
    lib_dirs = [root / "lib", root / "usr" / "lib"]
    for lib_name in required_libs:
        lib_found = any(
            any(match.name.startswith(lib_name.rstrip("*")) for match in lib_dir.glob(f"{lib_name}*"))
            for lib_dir in lib_dirs
            if lib_dir.exists()
        )
        if not lib_found:
            errors.append(f"Missing required library: {lib_name}")
    return errors


def validate_ca_certificates(root: Path, policy: Optional[dict] = None) -> List[str]:
    errors = []
    ca_bundle_candidates = [
        root / "etc" / "ssl" / "certs" / "ca-certificates.crt",
        root / "usr" / "lib" / "ssl" / "certs" / "ca-certificates.crt",
    ]
    if policy and policy.get("caCertificatesPath"):
        custom_path = root / policy["caCertificatesPath"].lstrip("/")
        ca_bundle_candidates.insert(0, custom_path)
    ca_found = any(p.exists() for p in ca_bundle_candidates)
    if not ca_found:
        errors.append("Missing CA certificates bundle")
    return errors


def validate_forbidden_executables(root: Path, policy: Optional[dict] = None) -> List[str]:
    errors = []
    forbidden = ROOTFS_FORBIDDEN_EXECUTABLES
    if policy and "forbiddenExecutables" in policy:
        forbidden = policy["forbiddenExecutables"]
    for exec_path in forbidden:
        if (root / exec_path.lstrip("/")).exists():
            errors.append(f"Forbidden executable present: {exec_path}")
    return errors


def validate_no_node_bundled(root: Path) -> List[str]:
    errors = []
    node_paths = [
        root / "usr" / "bin" / "node",
        root / "usr" / "local" / "bin" / "node",
        root / "opt" / "node" / "bin" / "node",
    ]
    for node_path in node_paths:
        if node_path.exists():
            errors.append(f"Node bundled in rootfs: {node_path.relative_to(root)}")
    return errors


def validate_no_qdrant_bundled(root: Path) -> List[str]:
    errors = []
    qdrant_paths = [
        root / "opt" / "amitia" / "qdrant",
        root / "usr" / "bin" / "qdrant",
        root / "usr" / "local" / "bin" / "qdrant",
    ]
    for qdrant_path in qdrant_paths:
        if qdrant_path.exists():
            errors.append(f"Qdrant bundled in rootfs: {qdrant_path.relative_to(root)}")
    return errors


def validate_no_backend_bundled(root: Path) -> List[str]:
    errors = []
    backend_names = ["amitia-server", "amitia-backend", "server.exe", "backend.exe"]
    search_dirs = [root / "opt", root / "usr" / "bin", root / "usr" / "local" / "bin"]
    for search_dir in search_dirs:
        if not search_dir.exists():
            continue
        for item in search_dir.iterdir():
            if item.name in backend_names:
                errors.append(f"Amitia backend bundled in rootfs: {item.relative_to(root)}")
    return errors


def validate_no_user_data(root: Path) -> List[str]:
    errors = []
    sqlite_extensions = {".db", ".sqlite", ".sqlite3", ".wal", ".shm"}
    program_dirs = [root / "opt", root / "usr" / "bin", root / "usr" / "sbin"]
    for program_dir in program_dirs:
        if not program_dir.exists():
            continue
        for item in program_dir.rglob("*"):
            if item.is_file() and item.suffix in sqlite_extensions:
                errors.append(f"Database file in program dir: {item.relative_to(root)}")
    return errors


def validate_machine_id_clean(root: Path, policy: Optional[dict] = None) -> List[str]:
    errors = []
    machine_id_paths = [root / "etc" / "machine-id", root / "var" / "lib" / "dbus" / "machine-id"]
    if policy and "machineIdCleanup" in policy:
        machine_id_paths = [root / p.lstrip("/") for p in policy["machineIdCleanup"]]
    for mid_path in machine_id_paths:
        if mid_path.exists():
            try:
                content = mid_path.read_text(encoding="utf-8").strip()
                if content:
                    errors.append(f"Machine ID not clean: {mid_path.relative_to(root)} = '{content[:20]}...'")
            except OSError:
                pass
    return errors


def validate_ssh_host_keys_clean(root: Path, policy: Optional[dict] = None) -> List[str]:
    errors = []
    ssh_dir = root / "etc" / "ssh"
    if not ssh_dir.exists():
        return errors
    for key_file in ssh_dir.glob("ssh_host_*"):
        if key_file.is_file() and "pub" not in key_file.name:
            errors.append(f"SSH host key present: {key_file.relative_to(root)}")
    return errors


def validate_shell_history_clean(root: Path, policy: Optional[dict] = None) -> List[str]:
    errors = []
    hist_paths = [root / "root" / ".bash_history"]
    amitia_home = root / "home" / "amitia"
    if amitia_home.exists():
        hist_paths.extend(amitia_home.glob(".bash_history"))
        hist_paths.extend(amitia_home.glob(".zsh_history"))
    if policy and "shellHistoryCleanup" in policy:
        hist_paths = [root / p.lstrip("/") for p in policy["shellHistoryCleanup"]]
    for hist_path in hist_paths:
        if hist_path.exists() and hist_path.stat().st_size > 0:
            errors.append(f"Shell history not clean: {hist_path.relative_to(root)}")
    return errors


def validate_secret_scan(root: Path) -> List[str]:
    errors = []
    scan_dirs = [root / "etc", root / "root", root / "home"]
    for scan_dir in scan_dirs:
        if not scan_dir.exists():
            continue
        for item in scan_dir.rglob("*"):
            if not item.is_file() or item.is_symlink():
                continue
            if item.stat().st_size > 1048576:
                continue
            try:
                content = item.read_text(encoding="utf-8", errors="replace")
                for pattern in ROOTFS_SECRET_PATTERNS:
                    if pattern.search(content):
                        errors.append(f"Potential secret in: {item.relative_to(root)}")
                        break
            except OSError:
                pass
    return errors


def validate_compiler_toolchain_absent(root: Path) -> List[str]:
    errors = []
    bin_dirs = [root / "usr" / "bin", root / "usr" / "sbin", root / "bin", root / "sbin"]
    compiler_names = {"gcc", "g++", "clang", "clang++", "make", "cmake", "rustc", "cargo", "go", "javavac"}
    for bin_dir in bin_dirs:
        if not bin_dir.exists():
            continue
        for item in bin_dir.iterdir():
            if item.name in compiler_names:
                errors.append(f"Compiler/toolchain present: {item.relative_to(root)}")
    return errors


def validate_no_local_token(root: Path) -> List[str]:
    errors = []
    token_paths = [
        root / "etc" / "amitia" / "local-token",
        root / "var" / "lib" / "amitia" / "local-token",
        root / "opt" / "amitia" / "security" / "local-token",
    ]
    security_dir = root / "security"
    if security_dir.exists():
        for token_file in security_dir.glob("*token*"):
            if token_file.is_file():
                errors.append(f"Token file in rootfs: {token_file.relative_to(root)}")
    for token_path in token_paths:
        if token_path.exists():
            errors.append(f"Local token bundled: {token_path.relative_to(root)}")
    return errors


def validate_merged_usr(root: Path) -> List[str]:
    errors = []
    merged_links = [
        ("bin", "usr/bin"),
        ("sbin", "usr/sbin"),
        ("lib", "usr/lib"),
    ]
    for link_name, expected_target in merged_links:
        link_path = root / link_name
        if link_path.is_symlink():
            actual_target = os.readlink(link_path)
            if actual_target != expected_target:
                errors.append(f"{link_name} symlink target wrong: {actual_target} != {expected_target}")
        elif not link_path.exists():
            errors.append(f"Missing {link_name} symlink (merged-usr)")
    return errors


def validate_tmp_clean(root: Path) -> List[str]:
    errors = []
    tmp_dirs = [root / "tmp", root / "var" / "tmp"]
    for tmp_dir in tmp_dirs:
        if tmp_dir.exists():
            for item in tmp_dir.iterdir():
                errors.append(f"Temp dir not clean: {tmp_dir.relative_to(root)}/{item.name}")
    return errors


def validate_apt_cache_clean(root: Path) -> List[str]:
    errors = []
    apt_archives = root / "var" / "cache" / "apt" / "archives"
    if apt_archives.exists():
        deb_files = list(apt_archives.glob("*.deb"))
        if deb_files:
            errors.append(f"APT cache not clean: {len(deb_files)} .deb files in archives")
    return errors


def validate_dpkg_database_preserved(root: Path) -> List[str]:
    errors = []
    dpkg_dir = root / "var" / "lib" / "dpkg"
    if not dpkg_dir.exists():
        errors.append("dpkg database missing (expected /var/lib/dpkg)")
    return errors


def validate_symlink_safety(root: Path, root_path: Path) -> List[str]:
    errors = []
    root_resolved = root_path.resolve()
    for dirpath, dirnames, filenames in os.walk(root, followlinks=False):
        dp = Path(dirpath)
        for name in filenames + dirnames:
            fp = dp / name
            if fp.is_symlink():
                target = os.readlink(fp)
                if target.startswith("/"):
                    errors.append(f"Absolute symlink in rootfs: {fp.relative_to(root)} -> {target}")
                else:
                    resolved = (fp.parent / target).resolve()
                    try:
                        resolved.relative_to(root_resolved)
                    except ValueError:
                        errors.append(f"Symlink escapes rootfs tree: {fp.relative_to(root)} -> {target}")
    return errors


def run_rootfs_validation(
    rootfs_path: str,
    lock_path: Optional[str] = None,
    policy_path: Optional[str] = None,
) -> dict:
    root = Path(rootfs_path)
    if not root.exists():
        return {
            "valid": False,
            "errors": [f"Rootfs path does not exist: {rootfs_path}"],
        }

    lock_data = None
    if lock_path and os.path.exists(lock_path):
        with open(lock_path, "r", encoding="utf-8") as f:
            lock_data = json.load(f)

    policy_data = None
    if policy_path and os.path.exists(policy_path):
        with open(policy_path, "r", encoding="utf-8") as f:
            policy_data = json.load(f)

    errors: List[str] = []

    if lock_data:
        lock_errors = validate_lock_format(lock_data)
        errors.extend(lock_errors)

    errors.extend(validate_required_dirs(root))
    errors.extend(validate_required_paths(root))
    errors.extend(validate_guest_layout_dirs(root, policy_data))
    errors.extend(validate_loader_exists(root, lock_data))
    errors.extend(validate_required_libraries(root, policy_data))
    errors.extend(validate_ca_certificates(root, policy_data))
    errors.extend(validate_forbidden_executables(root, policy_data))
    errors.extend(validate_no_node_bundled(root))
    errors.extend(validate_no_qdrant_bundled(root))
    errors.extend(validate_no_backend_bundled(root))
    errors.extend(validate_no_user_data(root))
    errors.extend(validate_machine_id_clean(root, policy_data))
    errors.extend(validate_ssh_host_keys_clean(root, policy_data))
    errors.extend(validate_shell_history_clean(root, policy_data))
    errors.extend(validate_secret_scan(root))
    errors.extend(validate_compiler_toolchain_absent(root))
    errors.extend(validate_no_local_token(root))
    errors.extend(validate_merged_usr(root))
    errors.extend(validate_tmp_clean(root))
    errors.extend(validate_apt_cache_clean(root))
    errors.extend(validate_dpkg_database_preserved(root))
    errors.extend(validate_symlink_safety(root, root))

    return {
        "valid": len(errors) == 0,
        "errors": errors,
        "rootfsPath": str(root),
    }


def parse_args():
    parser = argparse.ArgumentParser(description="Validate Ubuntu ARM64 Rootfs")
    parser.add_argument("--rootfs", required=True, help="Path to rootfs directory")
    parser.add_argument("--lock", help="Path to ubuntu-rootfs-lock.json")
    parser.add_argument("--policy", help="Path to rootfs-policy.json")
    parser.add_argument("--output", help="Output path for validation report JSON")
    parser.add_argument("--compute-hash", action="store_true", help="Compute tree/file hashes")
    parser.add_argument("--fld-manifest", action="store_true", help="Write F/L/D tree manifest")
    return parser.parse_args()


def main():
    args = parse_args()
    result = run_rootfs_validation(args.rootfs, args.lock, args.policy)

    if args.compute_hash:
        tree_sha = compute_tree_sha(args.rootfs)
        files_sha, file_hashes = compute_files_sha256(args.rootfs)
        result["treeSha256"] = tree_sha
        result["filesSha256"] = files_sha
        result["fileCount"] = len(file_hashes)

    if args.fld_manifest:
        fld_sha, fld_bytes = compute_fld_manifest(args.rootfs)
        manifest_out = Path(args.rootfs).parent / "rootfs-files.tsv"
        manifest_out.write_bytes(fld_bytes)
        result["fldManifestSha256"] = fld_sha
        result["fldManifestPath"] = str(manifest_out)

    if args.output:
        with open(args.output, "w", encoding="utf-8", newline="") as f:
            json.dump(result, f, indent=2, ensure_ascii=False)
            f.write("\n")

    if result["valid"]:
        print("[PASS] Rootfs validation passed")
        if "treeSha256" in result:
            print(f"  Tree SHA256: {result['treeSha256']}")
            print(f"  Files SHA256: {result['filesSha256']}")
            print(f"  File count: {result['fileCount']}")
        if "fldManifestSha256" in result:
            print(f"  F/L/D manifest SHA256: {result['fldManifestSha256']}")
            print(f"  F/L/D manifest path: {result['fldManifestPath']}")
        return 0
    else:
        print(f"[FAIL] Rootfs validation failed with {len(result['errors'])} errors:")
        for err in result["errors"]:
            print(f"  - {err}")
        return 1


if __name__ == "__main__":
    sys.exit(main())
