import argparse
import hashlib
import json
import os
import pathlib
import sys
import urllib.error
import urllib.request

DEFAULT_REPO = "qdrant/qdrant"
DEFAULT_TAG = "v1.19.0"
DEFAULT_OUTPUT = None
DEFAULT_LICENSE_FILE = "LICENSE-QDRANT"
ALLOWED_VERSIONS = {"v1.19.0"}
HTTP_TIMEOUT = 30
HTTP_CHUNK_SIZE = 1024 * 1024


def http_json(url):
    req = urllib.request.Request(url, headers={
        "User-Agent": "AmitiaQdrantLock/1.0",
        "Accept": "application/vnd.github+json",
    })
    try:
        with urllib.request.urlopen(req, timeout=HTTP_TIMEOUT) as resp:
            return json.loads(resp.read())
    except (urllib.error.URLError, urllib.error.HTTPError) as e:
        raise RuntimeError(f"请求失败: {url} -> {e}")


def release_commit_sha(owner, repo, tag):
    url = f"https://api.github.com/repos/{owner}/{repo}/git/refs/tags/{tag}"
    data = http_json(url)
    obj = data.get("object", {})
    if obj.get("type") == "tag":
        tag_url = obj.get("url")
        if not tag_url:
            raise RuntimeError("Tag 对象缺少 url 字段")
        tag_data = http_json(tag_url)
        return tag_data.get("object", {}).get("sha")
    return obj.get("sha")


def find_asset(release, asset_name):
    for asset in release.get("assets", []):
        if asset.get("name") == asset_name:
            return asset
    return None


def sha256_file(path):
    h = hashlib.sha256()
    with open(path, "rb") as f:
        while True:
            chunk = f.read(1048576)
            if not chunk:
                break
            h.update(chunk)
    return h.hexdigest()


def download_license(owner, repo, tag, dest_path):
    url = f"https://raw.githubusercontent.com/{owner}/{repo}/{tag}/{DEFAULT_LICENSE_FILE}"
    req = urllib.request.Request(url, headers={"User-Agent": "AmitiaQdrantLock/1.0"})
    try:
        with urllib.request.urlopen(req, timeout=HTTP_TIMEOUT) as resp:
            data = resp.read()
        with open(dest_path, "wb") as f:
            f.write(data)
        return hashlib.sha256(data).hexdigest()
    except (urllib.error.URLError, urllib.error.HTTPError) as e:
        raise RuntimeError(f"下载 LICENSE 失败: {e}")


def build_lock(owner, repo, tag, output_path, license_path):
    print(f"[信息] Repository: {owner}/{repo}")
    print(f"[信息] Tag: {tag}")

    release_url = f"https://api.github.com/repos/{owner}/{repo}/releases/tags/{tag}"
    release = http_json(release_url)

    if release.get("prerelease"):
        raise RuntimeError("不允许锁定 Pre-release")
    if release.get("draft"):
        raise RuntimeError("不允许锁定 Draft Release")

    release_tag = release.get("tag_name")
    if release_tag != tag:
        raise RuntimeError(f"Release Tag 不匹配: {release_tag} != {tag}")

    published_at = release.get("published_at")
    asset_name = "qdrant-aarch64-unknown-linux-musl.tar.gz"
    asset = find_asset(release, asset_name)
    if not asset:
        raise RuntimeError(f"Release 中缺少资产: {asset_name}")

    asset_size = asset.get("size", 0)
    asset_content_type = asset.get("content_type", "application/gzip")
    asset_digest = asset.get("digest", "")
    asset_sha = ""
    if asset_digest.lower().startswith("sha256:"):
        asset_sha = asset_digest[7:]

    if not asset_sha:
        print("[警告] API 未返回 asset digest，使用 browser_download_url 直接下载计算 SHA")
        dl_url = asset.get("browser_download_url")
        if not dl_url:
            raise RuntimeError("无法获取下载 URL")
        tmp = output_path.with_suffix(".tmp-dl")
        try:
            _download_to(dl_url, tmp)
            asset_sha = sha256_file(tmp)
        finally:
            if tmp.exists():
                tmp.unlink()
        print(f"[信息] 下载计算资产 SHA: {asset_sha}")

    commit_sha = release_commit_sha(owner, repo, tag)
    if not commit_sha or len(commit_sha) != 40:
        raise RuntimeError("无法解析完整 Release Commit SHA")

    license_sha = download_license(owner, repo, tag, license_path)
    print(f"[信息] LICENSE SHA: {license_sha}")

    lock = {
        "schemaVersion": 1,
        "componentId": "builtin.qdrant-process",
        "name": "qdrant",
        "version": tag.lstrip("v"),
        "releaseTag": tag,
        "releaseCommit": commit_sha,
        "releasePublishedAt": published_at,
        "platform": "linux",
        "architecture": "arm64",
        "rustTarget": "aarch64-unknown-linux-musl",
        "libc": "musl",
        "assetName": asset_name,
        "assetSize": asset_size,
        "assetContentType": asset_content_type,
        "assetSha256": asset_sha,
        "licenseFile": DEFAULT_LICENSE_FILE,
        "licenseSha256": license_sha,
    }

    content = json.dumps(lock, indent=2, sort_keys=False, ensure_ascii=False) + "\n"
    with open(output_path, "w", encoding="utf-8", newline="") as f:
        f.write(content)

    print(f"[完成] 锁文件已生成: {output_path}")
    print(f"[版本] {lock['version']}")
    print(f"[Commit] {lock['releaseCommit']}")
    print(f"[资产] {lock['assetName']} ({lock['assetSize']} bytes)")
    print(f"[SHA] {lock['assetSha256']}")


def _download_to(url, dest):
    req = urllib.request.Request(url, headers={"User-Agent": "AmitiaQdrantLock/1.0"})
    with urllib.request.urlopen(req, timeout=HTTP_TIMEOUT) as resp:
        with open(dest, "wb") as out:
            while True:
                chunk = resp.read(HTTP_CHUNK_SIZE)
                if not chunk:
                    break
                out.write(chunk)


def parse_args():
    parser = argparse.ArgumentParser(description="生成 Qdrant Linux ARM64 构建锁文件")
    parser.add_argument("--repo", default=DEFAULT_REPO, help="GitHub 仓库")
    parser.add_argument("--tag", default=DEFAULT_TAG, help="Release Tag")
    parser.add_argument("--output", help="锁文件输出路径")
    parser.add_argument("--license-path", help="LICENSE 输出路径")
    parser.add_argument("--allow-version-change", action="store_true",
                        help="允许修改为非预设版本")
    return parser.parse_args()


def main():
    args = parse_args()
    script_dir = pathlib.Path(__file__).resolve().parent
    tag = args.tag
    output_path = pathlib.Path(args.output) if args.output else script_dir / "qdrant.lock.json"
    license_path = pathlib.Path(args.license_path) if args.license_path else script_dir / DEFAULT_LICENSE_FILE

    repo = args.repo
    if "/" not in repo:
        print(f"[错误] 仓库格式无效，预期 owner/repo: {repo}", file=sys.stderr)
        sys.exit(1)
    owner, repo_name = repo.split("/", 1)

    if tag not in ALLOWED_VERSIONS and not args.allow_version_change:
        print(f"[错误] 版本 {tag} 不在允许列表中。使用 --allow-version-change 强制变更。",
              file=sys.stderr)
        sys.exit(1)

    try:
        build_lock(owner, repo_name, tag, output_path, license_path)
    except Exception as e:
        print(f"[错误] {e}", file=sys.stderr)
        sys.exit(1)


if __name__ == "__main__":
    main()
