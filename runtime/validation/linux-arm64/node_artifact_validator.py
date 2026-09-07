import argparse
import hashlib
import json
import os
import pathlib
import stat
import struct
import sys

ELF_MAGIC = b"\x7fELF"
ELFCLASS64 = 2
ELFDATA2LSB = 1
EM_AARCH64 = 183

SCRIPT_DIR = pathlib.Path(__file__).resolve().parent
DEFAULT_OUTPUT_DIR = SCRIPT_DIR.parent.parent.parent / "build" / "out" / "node" / "linux-arm64"
DEFAULT_LOCK_FILE = SCRIPT_DIR.parent.parent.parent / "artifacts" / "node" / "linux-arm64" / "node-runtime-lock.json"


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


def compute_tree_sha(manifest_content_bytes):
    return hashlib.sha256(manifest_content_bytes).hexdigest()


def check_lock(lock_file, issues):
    if not lock_file.exists():
        issues.append(f"Lock file not found: {lock_file}")
        return None
    try:
        lock = load_json(lock_file)
    except (json.JSONDecodeError, OSError) as e:
        issues.append(f"Lock file parse error: {e}")
        return None
    required = ["schemaVersion", "component", "version", "platform", "architecture",
                "archiveFileName", "sourceUrl", "sha256", "installSubdir"]
    for key in required:
        if key not in lock:
            issues.append(f"Lock missing field: {key}")
    if lock.get("component") != "node":
        issues.append(f"Lock component != node: {lock.get('component')}")
    if lock.get("platform") != "linux":
        issues.append(f"Lock platform != linux: {lock.get('platform')}")
    if lock.get("architecture") != "arm64":
        issues.append(f"Lock architecture != arm64: {lock.get('architecture')}")
    version = lock.get("version", "")
    if not any(c.isdigit() for c in str(version)):
        issues.append(f"Lock version invalid: {version}")
    sha = lock.get("sha256", "")
    if not isinstance(sha, str) or len(sha) != 64:
        issues.append(f"Lock sha256 invalid format")
    return lock


def check_build_record(output_dir, lock, issues, phase="final"):
    record_path = output_dir / "node-build-record.json"
    if not record_path.exists():
        if phase == "content":
            return None
        issues.append("node-build-record.json not found")
        return None
    try:
        record = load_json(record_path)
    except (json.JSONDecodeError, OSError) as e:
        issues.append(f"Build record parse error: {e}")
        return None
    if record.get("schemaVersion") != 1:
        issues.append(f"Build record schemaVersion != 1: {record.get('schemaVersion')}")
    if record.get("component") != "node":
        issues.append(f"Build record component != node: {record.get('component')}")
    if record.get("version") != lock.get("version"):
        issues.append(f"Build record version mismatch: {record.get('version')} != {lock.get('version')}")
    if record.get("platform") != "linux":
        issues.append(f"Build record platform != linux: {record.get('platform')}")
    if record.get("architecture") != "arm64":
        issues.append(f"Build record architecture != arm64: {record.get('architecture')}")
    source = record.get("source", {})
    expected_sha = lock.get("sha256", "")
    if source.get("expectedSha256") != expected_sha:
        issues.append("Build record source.expectedSha256 != lock.sha256")
    if source.get("actualSha256") != expected_sha:
        issues.append("Build record source.actualSha256 != lock.sha256")
    runtime = record.get("runtime", {})
    for key in ["nodePath", "npmPath", "npxPath"]:
        if key not in runtime:
            issues.append(f"Build record runtime missing: {key}")
    tree_sha = record.get("treeSha256", "")
    if not tree_sha:
        issues.append("Build record treeSha256 missing")
    if phase == "final":
        validation = record.get("validation", {})
        if validation.get("staticValidation") != "PASS":
            issues.append(f"Build record staticValidation is not PASS: {validation.get('staticValidation')}")
    return record


def check_node_exists(output_dir, record, issues):
    node_path = output_dir / "node" / "bin" / "node"
    if not node_path.exists():
        issues.append("node/bin/node not found")
        return False
    if node_path.stat().st_size == 0:
        issues.append("node/bin/node is empty")
        return False
    return True


def check_npm_exists(output_dir, issues):
    npm_path = output_dir / "node" / "bin" / "npm"
    if not npm_path.exists():
        issues.append("node/bin/npm not found")
        return False
    if npm_path.is_symlink():
        target = os.readlink(npm_path)
        if target.startswith("/"):
            issues.append(f"npm symlink is absolute: {target}")
            return False
    return True


def check_npx_exists(output_dir, issues):
    npx_path = output_dir / "node" / "bin" / "npx"
    if not npx_path.exists():
        issues.append("node/bin/npx not found")
        return False
    if npx_path.is_symlink():
        target = os.readlink(npx_path)
        if target.startswith("/"):
            issues.append(f"npx symlink is absolute: {target}")
            return False
    return True


def check_elf_binary(output_dir, issues):
    node_bin = output_dir / "node" / "bin" / "node"
    if not node_bin.exists():
        return
    stat_result = node_bin.lstat()
    if stat.S_ISLNK(stat_result.st_mode):
        return
    with open(node_bin, "rb") as f:
        header = f.read(64)
    if len(header) < 20:
        issues.append("node binary header too short")
        return
    if header[:4] != ELF_MAGIC:
        issues.append("node is not ELF")
        return
    ei_class = header[4]
    if ei_class != ELFCLASS64:
        issues.append(f"node is not 64-bit ELF: ei_class={ei_class}")
    ei_data = header[5]
    if ei_data != ELFDATA2LSB:
        issues.append(f"node is not little-endian: ei_data={ei_data}")
    machine = struct.unpack_from("<H", header, 18)[0]
    if machine != EM_AARCH64:
        issues.append(f"node machine != AArch64: e_machine={machine}")


def check_tree_manifest(output_dir, record, issues):
    manifest_path = output_dir / "node-files.sha256"
    if not manifest_path.exists():
        issues.append("node-files.sha256 not found")
        return
    raw_bytes = manifest_path.read_bytes()
    if b"\r" in raw_bytes:
        issues.append("node-files.sha256 contains CR characters; LF-only required")
    if raw_bytes.startswith(b"\xef\xbb\xbf"):
        issues.append("node-files.sha256 has UTF-8 BOM; BOM not allowed")
    try:
        content = raw_bytes.decode("utf-8")
    except UnicodeDecodeError:
        issues.append("node-files.sha256 is not valid UTF-8")
        return
    lines = [l for l in content.split("\n") if l.strip()]
    if not lines:
        issues.append("node-files.sha256 is empty")
        return
    file_lines = []
    symlink_lines = []
    for line in lines:
        if line.startswith("L "):
            parts = line.split(" ", 2)
            if len(parts) != 3:
                issues.append(f"Malformed symlink manifest line: {line}")
                continue
            symlink_lines.append(parts)
        else:
            parts = line.split("  ", 1)
            if len(parts) != 2:
                issues.append(f"Malformed manifest line: {line}")
                continue
            digest, name = parts
            if len(digest) != 64:
                issues.append(f"Malformed digest in line: {line}")
                continue
            try:
                int(digest, 16)
            except ValueError:
                issues.append(f"Non-hex digest in line: {line}")
                continue
            file_lines.append((name, digest))

    for name, digest in file_lines:
        fp = output_dir / name
        if not fp.exists():
            issues.append(f"Manifest references missing file: {name}")
            continue
        actual = sha256_file(fp)
        if actual != digest:
            issues.append(f"File SHA mismatch: {name}")

    all_paths = [name for name, _ in file_lines] + [target for _, target, _ in symlink_lines]
    sorted_paths = sorted(all_paths)
    if all_paths != sorted_paths:
        issues.append("node-files.sha256 not sorted by path (ordinal lexical)")

    if record:
        expected_tree = record.get("treeSha256", "")
        if expected_tree:
            actual_tree = compute_tree_sha(raw_bytes)
            if actual_tree != expected_tree:
                issues.append(f"Tree SHA mismatch: actual={actual_tree} expected={expected_tree}")


def check_symlink_safety(output_dir, issues):
    node_root = output_dir / "node"
    if not node_root.exists():
        return
    node_root_resolved = node_root.resolve()
    for dirpath, dirnames, filenames in os.walk(node_root, followlinks=False):
        dp = pathlib.Path(dirpath)
        for name in filenames + dirnames:
            fp = dp / name
            if fp.is_symlink():
                target = os.readlink(fp)
                if target.startswith("/"):
                    issues.append(f"Absolute symlink: {fp.relative_to(output_dir)} -> {target}")
                else:
                    resolved = (fp.parent / target).resolve()
                    try:
                        resolved.relative_to(node_root_resolved)
                    except ValueError:
                        issues.append(f"Symlink escapes node root: {fp.relative_to(output_dir)} -> {target}")


def check_executable_mode(output_dir, issues):
    node_bin = output_dir / "node" / "bin" / "node"
    if not node_bin.exists() or node_bin.is_symlink():
        return
    if sys.platform != "win32":
        mode = node_bin.stat().st_mode & 0o777
        if mode != 0o755:
            issues.append(f"node bin mode != 0755: {oct(mode)}")


def check_build_record_consistency(output_dir, lock, record, issues):
    if record is None:
        return
    manifest_path = output_dir / "node-files.sha256"
    if not manifest_path.exists():
        return
    raw_bytes = manifest_path.read_bytes()
    actual_tree = compute_tree_sha(raw_bytes)
    record_tree = record.get("treeSha256", "")
    if actual_tree != record_tree:
        issues.append(f"Build record treeSha256 does not match actual manifest SHA: record={record_tree} actual={actual_tree}")
    record_version = record.get("version", "")
    if record_version != lock.get("version", ""):
        issues.append(f"Build record version != lock version: {record_version}")
    record_arch = record.get("architecture", "")
    if record_arch != "arm64":
        issues.append(f"Build record architecture != arm64: {record_arch}")
    record_platform = record.get("platform", "")
    if record_platform != "linux":
        issues.append(f"Build record platform != linux: {record_platform}")


def run_validation(output_dir, lock_file, phase="final"):
    issues = []
    lock = check_lock(lock_file, issues)
    if lock is None:
        return issues
    record = check_build_record(output_dir, lock, issues, phase=phase)
    if phase == "content":
        if not check_node_exists(output_dir, record, issues):
            return issues
        check_elf_binary(output_dir, issues)
        check_npm_exists(output_dir, issues)
        check_npx_exists(output_dir, issues)
        check_tree_manifest(output_dir, record, issues)
        check_symlink_safety(output_dir, issues)
        check_executable_mode(output_dir, issues)
        return issues
    if not check_node_exists(output_dir, record, issues):
        return issues
    check_elf_binary(output_dir, issues)
    check_npm_exists(output_dir, issues)
    check_npx_exists(output_dir, issues)
    check_tree_manifest(output_dir, record, issues)
    check_symlink_safety(output_dir, issues)
    check_executable_mode(output_dir, issues)
    check_build_record_consistency(output_dir, lock, record, issues)
    return issues


def parse_args():
    parser = argparse.ArgumentParser(description="Validate Linux ARM64 Node frozen artifact")
    parser.add_argument("--output-dir", type=pathlib.Path, default=DEFAULT_OUTPUT_DIR,
                        help="Path to frozen node output directory")
    parser.add_argument("--lock-file", type=pathlib.Path, default=DEFAULT_LOCK_FILE,
                        help="Path to node-runtime-lock.json")
    parser.add_argument("--phase", choices=["content", "final"], default="final",
                        help="Validation phase: content (pre-build-record) or final (full)")
    return parser.parse_args()


def main():
    args = parse_args()
    output_dir = args.output_dir.resolve()
    lock_file = args.lock_file.resolve()
    phase = args.phase
    print(f"[VALIDATE] Output: {output_dir}")
    print(f"[VALIDATE] Lock:   {lock_file}")
    print(f"[VALIDATE] Phase:  {phase}")
    issues = run_validation(output_dir, lock_file, phase=phase)
    if issues:
        print(f"\n[FAIL] {len(issues)} issue(s):")
        for i in issues:
            print(f"  - {i}")
        sys.exit(1)
    print("\n[PASS] All checks passed")


if __name__ == "__main__":
    main()
