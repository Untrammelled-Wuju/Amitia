import argparse
import hashlib
import json
import os
import pathlib
import shutil
import sys
import urllib.error
import urllib.request

SCRIPT_DIR = pathlib.Path(__file__).resolve().parent
LOCK_FILE = SCRIPT_DIR / "rootfs.lock.json"
PACKAGES_LOCK_FILE = SCRIPT_DIR / "packages.lock.json"
DEFAULT_CACHE_DIR = SCRIPT_DIR / ".cache"
BASE_ARCHIVE_NAME = "ubuntu-base-24.04.4-base-arm64.tar.gz"
BASE_URL = "https://cdimage.ubuntu.com/ubuntu-base/releases/24.04.4/release"
HTTP_TIMEOUT = 300
HTTP_CHUNK_SIZE = 1024 * 1024


def load_json(path):
    with open(path, "r", encoding="utf-8") as f:
        return json.load(f)


def save_json(path, data):
    with open(path, "w", encoding="utf-8", newline="") as f:
        json.dump(data, f, indent=2, sort_keys=True, ensure_ascii=False)
        f.write("\n")


def sha256_file(path):
    h = hashlib.sha256()
    with open(path, "rb") as f:
        while True:
            chunk = f.read(1048576)
            if not chunk:
                break
            h.update(chunk)
    return h.hexdigest()


def sha256_bytes(data):
    return hashlib.sha256(data).hexdigest()


def download_file(url, dest_path, expected_size=None, expected_sha256=None):
    dest_path = pathlib.Path(dest_path)
    dest_path.parent.mkdir(parents=True, exist_ok=True)
    tmp_path = dest_path.with_suffix(".tmp")
    retries = 3
    for attempt in range(retries):
        try:
            print(f"[下载] {url}")
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
            if expected_sha256:
                with open(tmp_path, "rb") as f:
                    actual_sha256 = sha256_bytes(f.read())
                if actual_sha256 != expected_sha256:
                    raise RuntimeError(f"SHA 不匹配: {actual_sha256} != {expected_sha256}")
            os.replace(tmp_path, dest_path)
            print(f"[完成] {dest_path}")
            return dest_path
        except (urllib.error.URLError, urllib.error.HTTPError, OSError) as e:
            if tmp_path.exists():
                tmp_path.unlink()
            if attempt == retries - 1:
                raise RuntimeError(f"下载失败: {e}") from e
    return dest_path


def verify_debs(deb_dir, packages_lock, snapshot_id):
    deb_dir = pathlib.Path(deb_dir)
    manifest_path = deb_dir.parent / "cache-manifest.json"
    if manifest_path.exists():
        manifest = load_json(manifest_path)
        if manifest.get("aptSnapshot") != snapshot_id:
            raise RuntimeError(
                f"APT Lists Snapshot 不匹配: {manifest.get('aptSnapshot')} != {snapshot_id}"
            )
    resolved = packages_lock.get("resolvedPackages", [])
    if not resolved:
        raise RuntimeError("resolvedPackages 为空")
    deb_files = {}
    for deb_path in deb_dir.glob("*.deb"):
        deb_files[deb_path.name] = deb_path
    issues = []
    for pkg in resolved:
        filename = pkg["filename"]
        expected_sha256 = pkg["sha256"]
        expected_size = pkg["size"]
        if filename not in deb_files:
            issues.append(f"缺少 .deb: {filename}")
            continue
        actual_size = deb_files[filename].stat().st_size
        if actual_size != expected_size:
            issues.append(f"大小不匹配: {filename} {actual_size} != {expected_size}")
        actual_sha256 = sha256_file(deb_files[filename])
        if actual_sha256 != expected_sha256:
            issues.append(f"SHA 不匹配: {filename}")
    return issues


def run_prepare(args):
    lock = load_json(LOCK_FILE)
    cache_dir = pathlib.Path(args.cache_dir).resolve() if args.cache_dir else DEFAULT_CACHE_DIR
    apt_cache_dir = cache_dir / "apt"
    base_cache_dir = cache_dir / "base"
    apt_cache_dir.mkdir(parents=True, exist_ok=True)
    base_cache_dir.mkdir(parents=True, exist_ok=True)
    offline = args.offline
    source_dir = pathlib.Path(args.source_dir).resolve() if args.source_dir else None
    base_archive_name = lock["baseArchiveName"]
    base_archive_sha256 = lock["baseArchiveSha256"]
    base_archive_url = f"{BASE_URL}/{base_archive_name}"
    base_archive_path = base_cache_dir / base_archive_name
    if source_dir and (source_dir / base_archive_name).exists():
        src_path = source_dir / base_archive_name
        actual_sha = sha256_file(src_path)
        if actual_sha != base_archive_sha256:
            raise RuntimeError(f"Base 归档 SHA 不匹配: {actual_sha} != {base_archive_sha256}")
        if not base_archive_path.exists() or sha256_file(base_archive_path) != base_archive_sha256:
            shutil.copy2(str(src_path), str(base_archive_path))
    elif base_archive_path.exists():
        actual = sha256_file(base_archive_path)
        if actual == base_archive_sha256:
            print(f"[命中] Base 归档: {base_archive_path}")
        else:
            print("[重建] Base 归档 SHA 不匹配，重新下载")
            base_archive_path.unlink()
            if offline:
                raise RuntimeError("离线模式下 Base 归档 SHA 不匹配")
            download_file(base_archive_url, base_archive_path, expected_sha256=base_archive_sha256)
    else:
        if offline:
            raise RuntimeError("离线模式下 Base 归档不存在")
        download_file(base_archive_url, base_archive_path, expected_sha256=base_archive_sha256)
    print(f"[校验] Base 归档: {base_archive_path}")
    manifest_path = apt_cache_dir / "cache-manifest.json"
    packages_lock = None
    if PACKAGES_LOCK_FILE.exists():
        packages_lock = load_json(PACKAGES_LOCK_FILE)
    if manifest_path.exists():
        manifest = load_json(manifest_path)
        if packages_lock is None:
            packages_lock = {
                "schemaVersion": 1,
                "distribution": lock["distribution"],
                "codename": lock["codename"],
                "architecture": lock["architecture"],
                "aptSnapshot": manifest.get("aptSnapshot", lock["aptSnapshot"]),
                "requestedPackages": [],
                "resolvedPackages": [
                    {
                        "name": f["name"],
                        "version": "",
                        "architecture": manifest.get("architecture", "arm64"),
                        "filename": f["name"],
                        "size": f["size"],
                        "sha256": f["sha256"],
                    }
                    for f in manifest.get("debFiles", [])
                ],
            }
    deb_archive_dir = apt_cache_dir / "archives"
    deb_archive_dir.mkdir(parents=True, exist_ok=True)
    if source_dir:
        src_deb_dir = source_dir / "deb"
        if src_deb_dir.exists():
            print(f"[复制] 从源目录复制 .deb 文件: {src_deb_dir}")
            for deb_path in src_deb_dir.glob("*.deb"):
                dest = deb_archive_dir / deb_path.name
                if not dest.exists():
                    shutil.copy2(str(deb_path), str(dest))
        else:
            print(f"[信息] 源目录无 .deb 文件: {src_deb_dir}")
    if packages_lock is None:
        raise RuntimeError(
            "packages.lock.json 不存在且无可用缓存。请先运行 update_lock.py"
        )
    snapshot_id = packages_lock.get("aptSnapshot", lock["aptSnapshot"])
    print(f"[校验] 验证 .deb 文件完整性")
    issues = verify_debs(deb_archive_dir, packages_lock, snapshot_id)
    if issues:
        for issue in issues:
            print(f"[问题] {issue}")
        raise RuntimeError(f"发现 {len(issues)} 个问题")
    print(f"[校验] 所有 .deb 文件通过验证")
    print(f"[完成] APT 缓存准备完成")
    print(f"[统计] .deb 数量: {len(packages_lock.get('resolvedPackages', []))}")
    print(f"[统计] Base 归档: {base_archive_path}")


def parse_args():
    parser = argparse.ArgumentParser(description="准备 APT 离线缓存")
    parser.add_argument("--cache-dir", help="自定义缓存目录")
    parser.add_argument("--offline", action="store_true", help="离线模式（禁止网络访问）")
    parser.add_argument("--source-dir", help="本地源目录（包含 Base 归档和 .deb）")
    return parser.parse_args()


def main():
    args = parse_args()
    try:
        run_prepare(args)
    except Exception as e:
        print(f"[错误] {e}", file=sys.stderr)
        sys.exit(1)


if __name__ == "__main__":
    main()
