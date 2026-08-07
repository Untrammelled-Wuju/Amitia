import argparse
import hashlib
import json
import lzma
import os
import pathlib
import shutil
import stat
import sys
import tarfile
import tempfile
import urllib.error
import urllib.request

LOCK_FILE_NAME = "node.lock.json"
SCRIPT_DIR = pathlib.Path(__file__).resolve().parent
DEFAULT_CACHE_DIR = SCRIPT_DIR / ".cache"
DEFAULT_WORK_DIR = SCRIPT_DIR / ".work"
DEFAULT_OUTPUT_DIR = SCRIPT_DIR.parent.parent.parent / "out" / "node" / "linux-arm64"
FIXED_MTIME = 0
FIXED_UID = 0
FIXED_GID = 0
FIXED_UNAME = "root"
FIXED_GNAME = "root"
DIR_PERM = 0o755
FILE_PERM = 0o644
NODE_BIN_PERM = 0o755
XZ_COMPRESSION_LEVEL = 5
HTTP_TIMEOUT = 120
HTTP_CHUNK_SIZE = 1024 * 1024
NODE_ROOT_NAME = "node"


def load_lock(path=None):
    lock_path = pathlib.Path(path) if path else SCRIPT_DIR / LOCK_FILE_NAME
    with open(lock_path, "r", encoding="utf-8") as f:
        data = json.load(f)
    required = [
        "schemaVersion", "componentId", "name", "version", "ltsCodename",
        "npmVersion", "napiVersion", "platform", "architecture",
        "archiveName", "archiveRoot", "archiveSha256",
    ]
    for key in required:
        if key not in data:
            raise ValueError(f"锁文件缺少必填字段: {key}")
    sha = data["archiveSha256"]
    if not isinstance(sha, str) or len(sha) != 64:
        raise ValueError("archiveSha256 格式无效")
    try:
        int(sha, 16)
    except ValueError:
        raise ValueError("archiveSha256 不是有效十六进制")
    if data.get("platform") != "linux":
        raise ValueError("platform 必须为 linux")
    if data.get("architecture") != "arm64":
        raise ValueError("architecture 必须为 arm64")
    archive_name = data.get("archiveName", "")
    if data.get("version") not in archive_name:
        raise ValueError("archiveName 与版本不一致")
    expected_root = f"node-v{data['version']}-linux-{data['architecture']}"
    if data.get("archiveRoot") != expected_root:
        raise ValueError("archiveRoot 与版本/架构不一致")
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


def download_archive(url, tmp_path):
    try:
        req = urllib.request.Request(url, headers={"User-Agent": "AmitiaNodeBuild/1.0"})
        with urllib.request.urlopen(req, timeout=HTTP_TIMEOUT) as resp:
            with open(tmp_path, "wb") as out:
                while True:
                    chunk = resp.read(HTTP_CHUNK_SIZE)
                    if not chunk:
                        break
                    out.write(chunk)
    except (urllib.error.URLError, urllib.error.HTTPError, OSError) as e:
        if os.path.exists(tmp_path):
            os.remove(tmp_path)
        raise RuntimeError(f"下载失败: {e}")


def acquire_archive(lock, cache_dir, work_dir, offline=False, source_archive=None):
    expected_sha = lock["archiveSha256"]
    archive_name = lock["archiveName"]
    cache_path = cache_dir / archive_name
    if source_archive:
        src = pathlib.Path(source_archive).resolve()
        if not src.exists():
            raise RuntimeError(f"指定的源归档不存在: {src}")
        actual = sha256_file(src)
        if actual != expected_sha:
            raise RuntimeError(f"SHA 校验失败: 源归档 {actual} != 期望 {expected_sha}")
        return src
    if cache_path.exists():
        actual = sha256_file(cache_path)
        if actual == expected_sha:
            print(f"[命中] 缓存校验通过: {cache_path}")
            return cache_path
        print("[重建] 缓存 SHA 不符，删除旧缓存")
        cache_path.unlink()
    if offline:
        raise RuntimeError(f"离线模式下缓存不存在: {cache_path}")
    url = f"https://nodejs.org/dist/v{lock['version']}/{archive_name}"
    print(f"[下载] {url}")
    tmp_path = cache_path.with_suffix(".tmp")
    try:
        download_archive(url, tmp_path)
        actual = sha256_file(tmp_path)
        if actual != expected_sha:
            tmp_path.unlink()
            raise RuntimeError(f"SHA 校验失败: 下载 {actual} != 期望 {expected_sha}")
        os.replace(tmp_path, cache_path)
        print(f"[完成] 下载并校验通过: {cache_path}")
    except Exception:
        if tmp_path.exists():
            tmp_path.unlink()
        raise
    return cache_path


def safe_extract(archive_path, work_dir, expected_root):
    extract_dir = pathlib.Path(tempfile.mkdtemp(prefix=".extract-", dir=work_dir))
    try:
        with tarfile.open(archive_path, "r:xz") as tf:
            members = tf.getmembers()
            seen_roots = set()
            for m in members:
                if not m.name:
                    raise RuntimeError("归档成员路径为空")
                if m.name.startswith("/"):
                    raise RuntimeError(f"绝对路径成员: {m.name}")
                if ":" in m.name:
                    raise RuntimeError(f"Windows 盘符成员: {m.name}")
                parts = pathlib.PurePosixPath(m.name).parts
                if ".." in parts:
                    raise RuntimeError(f"路径穿越成员: {m.name}")
                rel = os.path.normpath(m.name)
                if rel.startswith(".."):
                    raise RuntimeError(f"规范化后路径越界: {m.name}")
                top = parts[0]
                seen_roots.add(top)
                if m.issym():
                    link_target = m.linkname
                    if link_target.startswith("/"):
                        raise RuntimeError(f"绝对符号链接: {m.name} -> {link_target}")
                    target_abs = os.path.normpath(os.path.join(os.path.dirname(m.name), link_target))
                    if target_abs.startswith(".."):
                        raise RuntimeError(f"符号链接越界: {m.name} -> {link_target}")
                elif m.islnk():
                    link_target = m.linkname
                    if link_target.startswith("/"):
                        raise RuntimeError(f"绝对硬链接: {m.name} -> {link_target}")
                    target_abs = os.path.normpath(os.path.join(os.path.dirname(m.name), link_target))
                    if target_abs.startswith(".."):
                        raise RuntimeError(f"硬链接越界: {m.name} -> {link_target}")
                if m.ischr() or m.isblk() or m.isfifo() or m.isdev():
                    raise RuntimeError(f"设备/FIFO 成员: {m.name}")
            if len(seen_roots) != 1:
                raise RuntimeError(f"归档包含多个顶层目录: {seen_roots}")
            root_name = seen_roots.pop()
            if root_name != expected_root:
                raise RuntimeError(f"归档根目录不匹配: {root_name} != {expected_root}")
            for m in members:
                tf.extract(m, extract_dir, set_attrs=False)
    except Exception:
        shutil.rmtree(extract_dir, ignore_errors=True)
        raise
    return extract_dir / expected_root


def validate_structure(root):
    required = [
        "bin/node",
        "lib/node_modules/npm/bin/npm-cli.js",
        "lib/node_modules/npm/bin/npx-cli.js",
        "include/node",
        "LICENSE",
    ]
    missing = []
    for rel in required:
        if not (root / rel).exists():
            missing.append(rel)
    if missing:
        raise RuntimeError(f"缺失关键文件: {missing}")


def prune_tree(root):
    remove_dirs = ["share/man", "share/doc", "share/systemtap"]
    for rel in remove_dirs:
        target = root / rel
        if target.exists():
            shutil.rmtree(target, ignore_errors=True)


def fix_permissions(root):
    for dirpath, dirnames, filenames in os.walk(root):
        dirp = pathlib.Path(dirpath)
        os.chmod(dirp, DIR_PERM)
        for fn in filenames:
            fp = dirp / fn
            if fp.is_symlink():
                continue
            if fp.name == "node" and fp.parent.name == "bin":
                os.chmod(fp, NODE_BIN_PERM)
            else:
                os.chmod(fp, FILE_PERM)


def ensure_relative_symlinks(root):
    bin_dir = root / "bin"
    npm_link = bin_dir / "npm"
    npx_link = bin_dir / "npx"
    if not npm_link.exists():
        os.symlink("../lib/node_modules/npm/bin/npm", npm_link)
    if not npx_link.exists():
        os.symlink("../lib/node_modules/npm/bin/npx", npx_link)


def build_file_manifest(root):
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
        if fp.is_symlink():
            target = os.readlink(fp)
            if fp.is_dir():
                entries.append({
                    "path": rel,
                    "type": "dirsymlink",
                    "target": target,
                    "size": 0,
                    "sha256": "",
                    "mode": "",
                })
            else:
                entries.append({
                    "path": rel,
                    "type": "symlink",
                    "target": target,
                    "size": 0,
                    "sha256": "",
                    "mode": "",
                })
        elif fp.is_dir():
            continue
        else:
            st = fp.stat()
            entries.append({
                "path": rel,
                "type": "file",
                "target": "",
                "size": st.st_size,
                "sha256": sha256_file(fp),
                "mode": oct(stat.S_IMODE(st.st_mode)),
            })
    entries.sort(key=lambda e: e["path"])
    return entries


def write_runtime_json(output_dir, lock):
    runtime_json = {
        "schemaVersion": 1,
        "componentId": lock["componentId"],
        "name": lock["name"],
        "version": lock["version"],
        "npmVersion": lock["npmVersion"],
        "napiVersion": lock["napiVersion"],
        "platform": lock["platform"],
        "architecture": lock["architecture"],
        "distributionRoot": NODE_ROOT_NAME,
        "entrypoints": {
            "node": "node/bin/node",
            "npmCli": "node/lib/node_modules/npm/bin/npm-cli.js",
            "npxCli": "node/lib/node_modules/npm/bin/npx-cli.js",
        },
        "source": {
            "type": "official-node-release",
            "archive": lock["archiveName"],
            "sha256": lock["archiveSha256"],
        },
    }
    out = output_dir / "node-runtime.json"
    with open(out, "w", encoding="utf-8", newline="") as f:
        json.dump(runtime_json, f, indent=2, sort_keys=True)
        f.write("\n")
    return out


def write_file_manifest(output_dir, root):
    manifest = build_file_manifest(root)
    out = output_dir / "file-manifest.json"
    content = json.dumps(manifest, indent=2, sort_keys=True, ensure_ascii=False) + "\n"
    with open(out, "w", encoding="utf-8", newline="") as f:
        f.write(content)
    return out


def write_sha256sums(output_dir, names):
    lines = []
    for name in sorted(names):
        fp = output_dir / name
        digest = sha256_file(fp)
        lines.append(f"{digest}  {name}")
    out = output_dir / "SHA256SUMS"
    with open(out, "w", encoding="utf-8", newline="") as f:
        f.write("\n".join(lines) + "\n")
    return out


def create_deterministic_tar(source_dir, archive_dir, output_path):
    source_path = source_dir
    members = []
    for dirpath, dirnames, filenames in os.walk(source_path, followlinks=False):
        dirnames.sort()
        for dn in dirnames:
            dp = pathlib.Path(dirpath) / dn
            arcname = dp.relative_to(archive_dir)
            members.append((arcname, dp))
        for fn in sorted(filenames):
            fp = pathlib.Path(dirpath) / fn
            arcname = fp.relative_to(archive_dir)
            members.append((arcname, fp))
    members.sort(key=lambda x: x[0].as_posix())
    tmp_out = output_path.with_suffix(".tmp")
    with lzma.open(tmp_out, "wb", preset=XZ_COMPRESSION_LEVEL) as xz:
        with tarfile.open(fileobj=xz, mode="w") as tf:
            for arcname, fp in members:
                info = tf.gettarinfo(str(fp), arcname=str(arcname))
                info.uid = FIXED_UID
                info.gid = FIXED_GID
                info.uname = FIXED_UNAME
                info.gname = FIXED_GNAME
                info.mtime = FIXED_MTIME
                if fp.is_symlink():
                    info.type = tarfile.SYMTYPE
                    info.linkname = os.readlink(fp)
                    tf.addfile(info)
                elif fp.is_dir():
                    info.type = tarfile.DIRTYPE
                    info.mode = DIR_PERM
                    tf.addfile(info)
                else:
                    with open(fp, "rb") as fobj:
                        tf.addfile(info, fobj)
    os.replace(tmp_out, output_path)


def safe_replace_output(tmp_output, final_output):
    if final_output.exists():
        backup = final_output.with_name(final_output.name + ".old")
        if backup.exists():
            shutil.rmtree(backup)
        final_output.rename(backup)
        shutil.rmtree(backup, ignore_errors=True)
    final_output.parent.mkdir(parents=True, exist_ok=True)
    tmp_output.rename(final_output)


def run_build(args):
    lock = load_lock()
    print(f"[信息] Node {lock['version']} ({lock['ltsCodename']})")
    print(f"[信息] 平台 {lock['platform']}/{lock['architecture']}")
    print(f"[信息] 归档 {lock['archiveName']}")
    cache_dir = pathlib.Path(args.cache_dir).resolve() if args.cache_dir else DEFAULT_CACHE_DIR
    work_dir = pathlib.Path(args.work_dir).resolve() if args.work_dir else DEFAULT_WORK_DIR
    output_dir = pathlib.Path(args.output_dir).resolve() if args.output_dir else DEFAULT_OUTPUT_DIR
    cache_dir.mkdir(parents=True, exist_ok=True)
    work_dir.mkdir(parents=True, exist_ok=True)
    if args.clean and output_dir.exists():
        print(f"[清理] 删除旧输出: {output_dir}")
        shutil.rmtree(output_dir)
    final_output = output_dir
    tmp_output = output_dir.with_name(output_dir.name + ".partial")
    if tmp_output.exists():
        shutil.rmtree(tmp_output)
    tmp_output.mkdir(parents=True, exist_ok=True)
    final_archive_name = f"amitia-node-v{lock['version']}-linux-{lock['architecture']}.tar.xz"
    work_root = work_dir / "build"
    if work_root.exists():
        shutil.rmtree(work_root)
    work_root.mkdir(parents=True, exist_ok=True)
    try:
        archive_path = acquire_archive(
            lock, cache_dir, work_dir,
            offline=args.offline, source_archive=args.source_archive,
        )
        print(f"[校验] 源归档 SHA: {lock['archiveSha256']}")
        extracted = safe_extract(archive_path, work_root, lock["archiveRoot"])
        print(f"[解压] 完成: {extracted}")
        validate_structure(extracted)
        print("[结构] 关键文件校验通过")
        prune_tree(extracted)
        print("[裁剪] 移除非必要目录")
        fix_permissions(extracted)
        print("[权限] 已规范化")
        ensure_relative_symlinks(extracted)
        print("[符号链接] 已确保相对符号链接")
        dist_root = tmp_output / NODE_ROOT_NAME
        if dist_root.exists():
            shutil.rmtree(dist_root)
        shutil.copytree(extracted, dist_root, symlinks=True)
        print(f"[组装] 分发目录准备完成: {dist_root}")
        write_runtime_json(tmp_output, lock)
        write_file_manifest(tmp_output, dist_root)
        print("[元数据] 已生成 node-runtime.json / file-manifest.json")
        build_staging = tmp_output.parent / ("staging-" + tmp_output.name)
        if build_staging.exists():
            shutil.rmtree(build_staging)
        build_staging.mkdir(parents=True)
        shutil.copytree(str(dist_root), str(build_staging / NODE_ROOT_NAME), symlinks=True)
        shutil.copy2(str(tmp_output / "node-runtime.json"), str(build_staging / "node-runtime.json"))
        shutil.copy2(str(tmp_output / "file-manifest.json"), str(build_staging / "file-manifest.json"))
        archive_out = tmp_output / final_archive_name
        create_deterministic_tar(build_staging, build_staging, archive_out)
        shutil.rmtree(build_staging)
        print(f"[打包] 可复现归档: {archive_out}")
        final_sha = sha256_file(archive_out)
        print(f"[SHA] {final_sha}")
        write_sha256sums(
            tmp_output,
            [final_archive_name, "node-runtime.json", "file-manifest.json"],
        )
        print("[元数据] 已生成 SHA256SUMS")
        safe_replace_output(tmp_output, final_output)
        print(f"[发布] 输出目录: {final_output}")
        print("[完成] Linux ARM64 Node Runtime 构建成功")
        print(f"[产物] {final_output / final_archive_name}")
        print(f"[元数据] {final_output / 'node-runtime.json'}")
        print(f"[清单] {final_output / 'file-manifest.json'}")
        print(f"[校验] {final_output / 'SHA256SUMS'}")
    except Exception:
        if tmp_output.exists():
            shutil.rmtree(tmp_output, ignore_errors=True)
        raise
    finally:
        if work_root.exists():
            shutil.rmtree(work_root, ignore_errors=True)


def parse_args():
    parser = argparse.ArgumentParser(description="构建 Linux ARM64 Node Runtime 产物")
    parser.add_argument("--clean", action="store_true", help="清理后重新构建")
    parser.add_argument("--offline", action="store_true", help="离线模式")
    parser.add_argument("--cache-dir", help="自定义缓存目录")
    parser.add_argument("--work-dir", help="自定义临时工作目录")
    parser.add_argument("--output-dir", help="自定义输出目录")
    parser.add_argument("--source-archive", help="指定源归档路径")
    parser.add_argument("--skip-runtime-test", action="store_true", help="跳过运行时测试")
    return parser.parse_args()


def main():
    args = parse_args()
    try:
        run_build(args)
    except Exception as e:
        print(f"[错误] {e}", file=sys.stderr)
        sys.exit(1)


if __name__ == "__main__":
    main()
