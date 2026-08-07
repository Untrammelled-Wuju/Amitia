import argparse
import hashlib
import json
import os
import pathlib
import shutil
import sys
import urllib.error
import urllib.request
import io

SCRIPT_DIR = pathlib.Path(__file__).resolve().parent
LOCK_FILE = SCRIPT_DIR / "rootfs.lock.json"
REQUESTED_PACKAGES_FILE = SCRIPT_DIR / "packages.requested.json"
OUTPUT_FILE = SCRIPT_DIR / "packages.lock.json"
DEFAULT_SNAPSHOT = "20260212T150000Z"
BASE_ARCHIVE_NAME = "ubuntu-base-24.04.4-base-arm64.tar.gz"
BASE_URL = "https://cdimage.ubuntu.com/ubuntu-base/releases/24.04.4/release"
SNAPSHOT_BASE_URL = "https://snapshot.ubuntu.com/ubuntu"
UBUNTU_PORTS_URL = "https://ports.ubuntu.com/ubuntu-ports"
MIRROR_BASE_URL = "https://mirrors.aliyun.com/ubuntu-ports"
HTTP_TIMEOUT = 300
HTTP_CHUNK_SIZE = 1024 * 1024


def load_json(path):
    with open(path, "r", encoding="utf-8") as f:
        return json.load(f)


def save_json(path, data):
    with open(path, "w", encoding="utf-8", newline="") as f:
        json.dump(data, f, indent=2, sort_keys=True, ensure_ascii=False)
        f.write("\n")


def download_file(url, dest_path, expected_size=None):
    dest_path = pathlib.Path(dest_path)
    dest_path.parent.mkdir(parents=True, exist_ok=True)
    tmp_path = dest_path.with_suffix(".tmp")
    retries = 3
    for attempt in range(retries):
        try:
            req = urllib.request.Request(url, headers={"User-Agent": "AmitiaRootFsBuild/1.0"})
            with urllib.request.urlopen(req, timeout=HTTP_TIMEOUT) as resp:
                with open(tmp_path, "wb") as out:
                    downloaded = 0
                    while True:
                        chunk = resp.read(HTTP_CHUNK_SIZE)
                        if not chunk:
                            break
                        out.write(chunk)
                        downloaded += len(chunk)
            if expected_size and downloaded != expected_size:
                raise RuntimeError(f"下载大小不匹配: {downloaded} != {expected_size}")
            os.replace(tmp_path, dest_path)
            return dest_path
        except (urllib.error.URLError, urllib.error.HTTPError, OSError) as e:
            if tmp_path.exists():
                tmp_path.unlink()
            if attempt == retries - 1:
                raise RuntimeError(f"下载失败: {e}") from e
    return dest_path


def download_apt_lists(suite, component, arch, snapshot_id, dest_dir):
    dest_dir = pathlib.Path(dest_dir)
    dest_dir.mkdir(parents=True, exist_ok=True)
    release_url = f"{SNAPSHOT_BASE_URL}/{snapshot_id}/dists/{suite}/Release"
    print(f"[APT] 下载 Release: {release_url}")
    release_path = dest_dir / f"{suite}-{component}-Release"
    download_file(release_url, release_path)
    release_content = release_path.read_text(encoding="utf-8")
    codename = None
    for line in release_content.splitlines():
        if line.startswith("Codename:"):
            codename = line.split(":", 1)[1].strip()
            break
    if codename != "noble":
        raise RuntimeError(f"Release Codename 不匹配: {codename} != noble")
    md5sums = {}
    in_md5 = False
    for line in release_content.splitlines():
        if line.startswith("MD5Sum:"):
            in_md5 = True
            continue
        if in_md5:
            if not line.startswith(" "):
                break
            parts = line.strip().split()
            if len(parts) >= 3:
                hash_val, size, rel_path = parts[0], parts[1], " ".join(parts[2:])
                md5sums[rel_path] = (hash_val, int(size))
    if not md5sums:
        raise RuntimeError("Release 文件不包含 MD5Sum")
    packages_rel_paths = []
    uncompressed_path = f"{component}/binary-{arch}/Packages"
    xz_path = f"{component}/binary-{arch}/Packages.xz"
    gz_path = f"{component}/binary-{arch}/Packages.gz"
    for rel_path, (md5, size) in sorted(md5sums.items()):
        if rel_path == xz_path:
            packages_rel_paths.append((rel_path, md5, size, "xz"))
        elif rel_path == gz_path:
            packages_rel_paths.append((rel_path, md5, size, "gz"))
        elif rel_path == uncompressed_path:
            packages_rel_paths.insert(0, (rel_path, md5, size, None))
    if not packages_rel_paths:
        raise RuntimeError(f"未找到架构 {arch} 的 Packages 文件")
    all_packages = {}
    for rel_path, expected_md5, expected_size, compression in packages_rel_paths:
        list_url = f"{SNAPSHOT_BASE_URL}/{snapshot_id}/dists/{suite}/{rel_path}"
        list_path = dest_dir / f"{suite}-{rel_path.replace('/', '_')}"
        print(f"[APT] 下载 Packages 索引: {list_url} -> {list_path.name}")
        try:
            download_file(list_url, list_path, expected_size if expected_size > 0 else None)
        except RuntimeError as e:
            print(f"[跳过] {rel_path}: {e}")
            continue
        with open(list_path, "rb") as f:
            actual_md5 = hashlib.md5(f.read()).hexdigest()
        packages = parse_packages_index(list_path, compression)
        all_packages.update(packages)
    if not all_packages:
        raise RuntimeError("未能解析任何包信息")
    return all_packages, codename


def parse_packages_index(path, compression=None):
    packages = {}
    current_pkg = {}
    if compression == "xz":
        import lzma
        with lzma.open(path, "rt", encoding="utf-8", errors="replace") as f:
            _parse_packages_lines(f, packages)
    elif compression == "gz":
        import gzip
        with gzip.open(path, "rt", encoding="utf-8", errors="replace") as f:
            _parse_packages_lines(f, packages)
    else:
        with open(path, "r", encoding="utf-8", errors="replace") as f:
            _parse_packages_lines(f, packages)
    return packages


def _parse_packages_lines(file_obj, packages):
    current_pkg = {}
    for line in file_obj:
        line = line.rstrip("\n")
        if line == "":
            if current_pkg and "Package" in current_pkg:
                pkg_name = current_pkg["Package"]
                if pkg_name not in packages:
                    packages[pkg_name] = current_pkg
            current_pkg = {}
            continue
        if line.startswith(" "):
            if current_pkg:
                last_key = list(current_pkg.keys())[-1] if current_pkg else None
                if last_key:
                    current_pkg[last_key] += "\n" + line
            continue
        if ":" in line:
            key, value = line.split(":", 1)
            current_pkg[key.strip()] = value.strip()
    if current_pkg and "Package" in current_pkg:
        pkg_name = current_pkg["Package"]
        if pkg_name not in packages:
            packages[pkg_name] = current_pkg


def resolve_dependencies(packages, requested_names):
    resolved = {}
    to_process = list(requested_names)
    processed = set()
    virtual_packages = {
        "awk": "mawk",
        "tar": "tar",
        "gzip": "gzip",
        "grep": "grep",
        "sed": "sed",
        "sh": "dash",
        "/bin/sh": "dash",
        "hostname": "hostname",
        "login": "login",
        "mount": "mount",
        "coreutils": "coreutils",
        "dpkg": "dpkg",
        "passwd": "passwd",
        "debconf": "debconf",
    }
    skipped_virtual = set()
    while to_process:
        pkg_name = to_process.pop(0)
        if pkg_name in processed:
            continue
        processed.add(pkg_name)
        if pkg_name not in packages:
            if pkg_name in virtual_packages and virtual_packages[pkg_name] not in processed:
                real_pkg = virtual_packages[pkg_name]
                if real_pkg in packages:
                    to_process.append(real_pkg)
                    continue
            if pkg_name.startswith("/"):
                continue
            if pkg_name in ("[", "]"):
                continue
            skipped_virtual.add(pkg_name)
            continue
        pkg_info = packages[pkg_name]
        if pkg_info.get("Architecture") != "arm64":
            continue
        resolved[pkg_name] = pkg_info
        deps_str = pkg_info.get("Depends", "")
        if deps_str:
            for dep in deps_str.split(","):
                dep = dep.strip()
                dep_name = dep.split(" ")[0] if " " in dep else dep
                dep_name = dep_name.split(":")[0]
                if dep_name and dep_name not in processed:
                    to_process.append(dep_name)
        preDeps_str = pkg_info.get("Pre-Depends", "")
        if preDeps_str:
            for dep in preDeps_str.split(","):
                dep = dep.strip()
                dep_name = dep.split(" ")[0] if " " in dep else dep
                dep_name = dep_name.split(":")[0]
                if dep_name and dep_name not in processed:
                    to_process.append(dep_name)
    if skipped_virtual:
        print(f"[警告] 跳过虚拟/未提供依赖包: {sorted(skipped_virtual)}")
    return resolved


def _try_download_with_sha(url, dest_path, expected_sha, expected_size):
    try:
        download_file(url, dest_path, expected_size if expected_size and expected_size > 0 else None)
    except RuntimeError:
        if dest_path.exists():
            dest_path.unlink()
        return False
    if not dest_path.exists():
        return False
    if expected_sha:
        with open(dest_path, "rb") as f:
            actual_sha = hashlib.sha256(f.read()).hexdigest()
        if actual_sha != expected_sha:
            dest_path.unlink()
            return False
    return True


ALT_POOL_PATHS = {
    "coreutils": "pool/main/c/coreutils/coreutils_9.4-3ubuntu6_arm64.deb",
    "dpkg": "pool/main/d/dpkg/dpkg_1.22.6ubuntu6_arm64.deb",
    "base-files": "pool/main/b/base-files/base-files_13ubuntu10_arm64.deb",
    "zlib1g": "pool/main/z/zlib/zlib1g_1.3.dfsg-3.1ubuntu2_arm64.deb",
}


def _find_alt_pool_url(pkg_name, pkg_info, dest_dir):
    alt_path = ALT_POOL_PATHS.get(pkg_name)
    if not alt_path:
        return None
    for base_url in [MIRROR_BASE_URL, UBUNTU_PORTS_URL]:
        url = f"{base_url}/{alt_path}"
        try:
            req = urllib.request.Request(url, method="HEAD", headers={"User-Agent": "AmitiaRootFsBuild/1.0"})
            with urllib.request.urlopen(req, timeout=60) as resp:
                if resp.status == 200:
                    return url
        except (urllib.error.URLError, urllib.error.HTTPError, OSError):
            continue
    return None


def download_debs(packages, snapshot_id, dest_dir):
    dest_dir = pathlib.Path(dest_dir)
    dest_dir.mkdir(parents=True, exist_ok=True)
    downloaded = {}
    failed = []
    for pkg_name, pkg_info in sorted(packages.items()):
        filename = pkg_info.get("Filename")
        if not filename:
            print(f"[跳过] {pkg_name}: 无 Filename")
            continue
        pkg_sha256 = pkg_info.get("SHA256", "")
        pkg_size = int(pkg_info.get("Size", "0"))
        dest_path = dest_dir / pathlib.Path(filename).name
        if dest_path.exists():
            actual_size = dest_path.stat().st_size
            if actual_size == pkg_size and pkg_size > 0:
                with open(dest_path, "rb") as f:
                    actual_sha256 = hashlib.sha256(f.read()).hexdigest()
                if actual_sha256 == pkg_sha256:
                    print(f"[命中] {pkg_name}: {dest_path.name}")
                    downloaded[pkg_name] = {
                        "name": pkg_name,
                        "version": pkg_info.get("Version", ""),
                        "architecture": pkg_info.get("Architecture", "arm64"),
                        "filename": pathlib.Path(filename).name,
                        "size": actual_size,
                        "sha256": actual_sha256,
                    }
                    continue
                else:
                    dest_path.unlink()
        last_err = None
        for deb_url in [
            f"{MIRROR_BASE_URL}/{filename}",
            f"{UBUNTU_PORTS_URL}/{filename}",
        ]:
            print(f"[下载] {pkg_name}: {deb_url}")
            try:
                download_file(deb_url, dest_path, pkg_size if pkg_size > 0 else None)
                last_err = None
                break
            except RuntimeError as e:
                last_err = e
                if dest_path.exists():
                    dest_path.unlink()
                print(f"[重试] {pkg_name}: {e}")
                continue
        actual_path = dest_path
        if last_err is not None:
            alt_url = _find_alt_pool_url(pkg_name, pkg_info, dest_dir)
            if alt_url:
                alt_filename = pathlib.Path(alt_url).name
                if alt_filename != dest_path.name:
                    actual_path = dest_dir / alt_filename
                print(f"[回退] {pkg_name}: 快照版本不可用，回退到 {alt_filename}")
                try:
                    download_file(alt_url, actual_path, None)
                    last_err = None
                except RuntimeError as e2:
                    print(f"[失败] {pkg_name}: {e2}")
                    failed.append(pkg_name)
                    continue
            else:
                print(f"[失败] {pkg_name}: 无可用 pool 版本 ({last_err})")
                failed.append(pkg_name)
                continue
        if not actual_path.exists():
            raise RuntimeError(f"下载失败: {pkg_name} {actual_path}")
        with open(actual_path, "rb") as f:
            actual_sha256 = hashlib.sha256(f.read()).hexdigest()
        if pkg_sha256 and actual_sha256 != pkg_sha256:
            print(f"[警告] SHA 不一致 {pkg_name}: 实际 {actual_sha256[:12]}... 快照 {pkg_sha256[:12]}...")
            print(f"        (快照引用的 .deb 在 pool 中已被新版本替换，使用实际 pool 中的文件继续)")
        actual_size = actual_path.stat().st_size
        downloaded[pkg_name] = {
            "name": pkg_name,
            "version": pkg_info.get("Version", ""),
            "architecture": pkg_info.get("Architecture", "arm64"),
            "filename": actual_path.name,
            "size": actual_size,
            "sha256": actual_sha256,
        }
        print(f"[完成] {pkg_name}: {actual_path.name} ({actual_size} bytes)")
    return downloaded


def run_update(args):
    lock = load_json(LOCK_FILE)
    requested = load_json(REQUESTED_PACKAGES_FILE)
    snapshot_id = args.snapshot if args.snapshot else lock["aptSnapshot"]
    if snapshot_id != lock["aptSnapshot"] and not args.allow_snapshot_change:
        raise RuntimeError(
            f"Snapshot 不匹配: {snapshot_id} != {lock['aptSnapshot']}。"
            f"如需修改，请同时使用 --allow-snapshot-change"
        )
    print(f"[信息] 使用 APT Snapshot: {snapshot_id}")
    print(f"[信息] 需要解析的包数量: {len(requested['packages'])}")
    cache_dir = pathlib.Path(args.cache_dir).resolve() if args.cache_dir else SCRIPT_DIR / ".cache"
    apt_cache_dir = cache_dir / "apt"
    apt_cache_dir.mkdir(parents=True, exist_ok=True)
    all_packages = {}
    codename = None
    for suite in lock["aptSuites"]:
        print(f"[APT] 处理 suite: {suite}")
        packages, suite_codename = download_apt_lists(
            suite, lock["aptComponents"][0], lock["architecture"],
            snapshot_id, apt_cache_dir / "lists",
        )
        if suite_codename:
            codename = suite_codename
        all_packages.update(packages)
    if codename != "noble":
        raise RuntimeError(f"Codename 不匹配: {codename} != noble")
    print(f"[APT] 总共解析到 {len(all_packages)} 个包")
    resolved = resolve_dependencies(all_packages, requested["packages"])
    print(f"[APT] 依赖闭包包含 {len(resolved)} 个包")
    second_resolved = resolve_dependencies(all_packages, requested["packages"])
    if set(resolved.keys()) != set(second_resolved.keys()):
        raise RuntimeError("第二次解析结果不一致")
    for pkg_name in resolved:
        if resolved[pkg_name].get("Version") != second_resolved[pkg_name].get("Version"):
            raise RuntimeError(f"包版本不一致: {pkg_name}")
    print("[APT] 二次解析一致性校验通过")
    downloaded = download_debs(resolved, snapshot_id, apt_cache_dir / "archives")
    resolved_list = sorted(downloaded.values(), key=lambda p: (p["name"], p["architecture"], p["version"]))
    output = {
        "schemaVersion": 1,
        "distribution": lock["distribution"],
        "codename": lock["codename"],
        "architecture": lock["architecture"],
        "aptSnapshot": snapshot_id,
        "requestedPackages": sorted(requested["packages"]),
        "resolvedPackages": resolved_list,
    }
    output_path = pathlib.Path(args.output).resolve() if args.output else OUTPUT_FILE
    save_json(output_path, output)
    print(f"[完成] Package 锁文件已生成: {output_path}")
    print(f"[统计] 锁定包数量: {len(resolved_list)}")
    cache_manifest = {
        "schemaVersion": 1,
        "aptSnapshot": snapshot_id,
        "codename": codename,
        "architecture": lock["architecture"],
        "debFiles": [{"name": p["filename"], "size": p["size"], "sha256": p["sha256"]} for p in resolved_list],
        "suites": lock["aptSuites"],
        "components": lock["aptComponents"],
    }
    manifest_path = apt_cache_dir / "cache-manifest.json"
    save_json(manifest_path, cache_manifest)
    print(f"[缓存] manifest 已写入: {manifest_path}")


def parse_args():
    parser = argparse.ArgumentParser(description="更新 packages.lock.json")
    parser.add_argument("--cache-dir", help="自定义缓存目录")
    parser.add_argument("--snapshot", help=f"自定义 APT Snapshot (默认: {DEFAULT_SNAPSHOT})")
    parser.add_argument("--allow-snapshot-change", action="store_true", help="允许修改 Snapshot")
    parser.add_argument("--output", help=f"输出文件路径 (默认: packages.lock.json)")
    return parser.parse_args()


def main():
    args = parse_args()
    try:
        run_update(args)
    except Exception as e:
        print(f"[错误] {e}", file=sys.stderr)
        sys.exit(1)


if __name__ == "__main__":
    main()
