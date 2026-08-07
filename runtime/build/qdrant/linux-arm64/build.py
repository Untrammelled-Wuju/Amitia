import argparse
import hashlib
import json
import lzma
import os
import pathlib
import shutil
import stat
import struct
import sys
import tarfile
import tempfile
import urllib.error
import urllib.request

import elf_inspector

LOCK_FILE_NAME = "qdrant.lock.json"
SCRIPT_DIR = pathlib.Path(__file__).resolve().parent
DEFAULT_CACHE_DIR = SCRIPT_DIR / ".cache"
DEFAULT_WORK_DIR = SCRIPT_DIR / ".work"
DEFAULT_OUTPUT_DIR = SCRIPT_DIR.parent.parent / "out" / "qdrant" / "linux-arm64"
FIXED_MTIME = 0
FIXED_UID = 0
FIXED_GID = 0
FIXED_UNAME = "root"
FIXED_GNAME = "root"
DIR_PERM = 0o755
FILE_PERM = 0o644
BIN_PERM = 0o755
LICENSE_PERM = 0o644
XZ_COMPRESSION_LEVEL = 5
HTTP_TIMEOUT = 120
HTTP_CHUNK_SIZE = 1024 * 1024
DIST_ROOT_NAME = "qdrant"
MAX_SCAN_DEPTH = 2


def load_lock(path=None):
    lock_path = pathlib.Path(path) if path else SCRIPT_DIR / LOCK_FILE_NAME
    with open(lock_path, "r", encoding="utf-8") as f:
        data = json.load(f)
    required = [
        "schemaVersion", "componentId", "name", "version", "releaseTag",
        "releaseCommit", "platform", "architecture", "rustTarget", "libc",
        "assetName", "assetSize", "assetSha256",
        "licenseFile", "licenseSha256",
    ]
    for key in required:
        if key not in data:
            raise ValueError(f"锁文件缺少必填字段: {key}")
    if data.get("platform") != "linux":
        raise ValueError("platform 必须为 linux")
    if data.get("architecture") != "arm64":
        raise ValueError("architecture 必须为 arm64")
    if data.get("rustTarget") != "aarch64-unknown-linux-musl":
        raise ValueError("rustTarget 不匹配")
    if data.get("libc") != "musl":
        raise ValueError("libc 必须为 musl")
    sha = data["assetSha256"]
    if not isinstance(sha, str) or len(sha) != 64:
        raise ValueError("assetSha256 格式无效")
    try:
        int(sha, 16)
    except ValueError:
        raise ValueError("assetSha256 不是有效十六进制")
    lic_sha = data["licenseSha256"]
    if not isinstance(lic_sha, str) or len(lic_sha) != 64:
        raise ValueError("licenseSha256 格式无效")
    try:
        int(lic_sha, 16)
    except ValueError:
        raise ValueError("licenseSha256 不是有效十六进制")
    if not isinstance(data.get("assetSize"), int) or data["assetSize"] <= 0:
        raise ValueError("assetSize 必须为正整数")
    if len(data.get("releaseCommit", "")) != 40:
        raise ValueError("releaseCommit 不完整")
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


def sha256_bytes(data):
    h = hashlib.sha256()
    h.update(data)
    return h.hexdigest()


def download_file(url, tmp_path):
    try:
        req = urllib.request.Request(url, headers={"User-Agent": "AmitiaQdrantBuild/1.0"})
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
    expected_sha = lock["assetSha256"]
    expected_size = lock["assetSize"]
    archive_name = lock["assetName"]
    cache_path = cache_dir / archive_name
    if source_archive:
        src = pathlib.Path(source_archive).resolve()
        if not src.exists():
            raise RuntimeError(f"指定的源归档不存在: {src}")
        st_size = src.stat().st_size
        if st_size != expected_size:
            raise RuntimeError(f"Size 不匹配: {st_size} != {expected_size}")
        actual = sha256_file(src)
        if actual != expected_sha:
            raise RuntimeError(f"SHA 校验失败: 源资产 {actual} != 期望 {expected_sha}")
        return src
    if cache_path.exists():
        cache_size = cache_path.stat().st_size
        actual = sha256_file(cache_path)
        if cache_size == expected_size and actual == expected_sha:
            print(f"[命中] 缓存校验通过: {cache_path}")
            return cache_path
        print("[重建] 缓存 SHA/Size 不符，删除旧缓存")
        cache_path.unlink()
    if offline:
        raise RuntimeError(f"离线模式下缓存不存在或无有效源: {cache_path}")
    url = f"https://github.com/qdrant/qdrant/releases/download/{lock['releaseTag']}/{archive_name}"
    print(f"[下载] {url}")
    tmp_path = cache_path.with_suffix(".partial")
    try:
        download_file(url, tmp_path)
        dl_size = tmp_path.stat().st_size
        if dl_size != expected_size:
            tmp_path.unlink()
            raise RuntimeError(f"Size 不匹配: 下载 {dl_size} != 期望 {expected_size}")
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


def safe_extract_member_check(member, extract_dir):
    name = member.name
    if not name:
        raise RuntimeError("归档成员路径为空")
    if name.startswith("/"):
        raise RuntimeError(f"绝对路径成员: {name}")
    if os.name == "nt" and len(name) > 1 and name[1] == ":":
        raise RuntimeError(f"Windows 盘符成员: {name}")
    parts = pathlib.PurePosixPath(name).parts
    if ".." in parts:
        raise RuntimeError(f"路径穿越成员: {name}")
    target = os.path.normpath(os.path.join(extract_dir, name))
    extract_real = os.path.realpath(extract_dir)
    target_real = os.path.realpath(target)
    if not (target_real == extract_real or target_real.startswith(extract_real + os.sep)):
        raise RuntimeError(f"规范化后路径越界: {name}")
    if member.issym():
        link_target = member.linkname
        if link_target.startswith("/"):
            raise RuntimeError(f"绝对符号链接: {name} -> {link_target}")
        resolved = os.path.normpath(os.path.join(extract_dir, os.path.dirname(name), link_target))
        if not (resolved == extract_real or resolved.startswith(extract_real + os.sep)):
            raise RuntimeError(f"符号链接越界: {name} -> {link_target}")
    elif member.islnk():
        link_target = member.linkname
        if not link_target:
            raise RuntimeError(f"空硬链接目标: {name}")
        if link_target.startswith("/"):
            raise RuntimeError(f"绝对硬链接: {name} -> {link_target}")
        resolved = os.path.normpath(os.path.join(extract_dir, os.path.dirname(name), link_target))
        if not (resolved == extract_real or resolved.startswith(extract_real + os.sep)):
            raise RuntimeError(f"硬链接越界: {name} -> {link_target}")
    if member.ischr() or member.isblk() or member.isfifo() or member.isdev():
        raise RuntimeError(f"设备/FIFO/Socket 成员: {name}")


def safe_extract(archive_path, work_dir):
    extract_dir = pathlib.Path(tempfile.mkdtemp(prefix=".extract-", dir=work_dir))
    try:
        with tarfile.open(archive_path, "r:gz") as tf:
            members = tf.getmembers()
            seen_targets = set()
            for m in members:
                safe_extract_member_check(m, str(extract_dir))
                target = os.path.normpath(os.path.join(str(extract_dir), m.name))
                if target in seen_targets:
                    raise RuntimeError(f"重复目标成员: {m.name}")
                seen_targets.add(target)
            for m in members:
                tf.extract(m, str(extract_dir), set_attrs=False)
    except Exception:
        shutil.rmtree(str(extract_dir), ignore_errors=True)
        raise
    return extract_dir


def find_qdrant_binary(extract_dir):
    candidates = []
    root_dir = pathlib.Path(extract_dir)
    direct = root_dir / "qdrant"
    if direct.exists() or direct.is_symlink():
        candidates.append(direct)
    if root_dir.is_dir():
        for child in sorted(root_dir.iterdir()):
            if not child.is_dir():
                continue
            candidate = child / "qdrant"
            if candidate.exists() or candidate.is_symlink():
                candidates.append(candidate)
                if len(candidates) > 1:
                    break
    if len(candidates) == 0:
        raise RuntimeError("未找到 qdrant 二进制")
    if len(candidates) > 1:
        raise RuntimeError(f"找到多个 qdrant 候选: {candidates}")
    binary = candidates[0]
    if binary.is_dir():
        raise RuntimeError(f"候选为目录而非文件: {binary}")
    if binary.is_symlink():
        resolved = binary.resolve()
        if not resolved.exists():
            raise RuntimeError(f"符号链接目标不存在: {binary} -> {resolved}")
        return resolved
    if binary.stat().st_size == 0:
        raise RuntimeError(f"候选二进制为空文件: {binary}")
    return binary


def validate_structure(binary_path):
    size = binary_path.stat().st_size
    if size == 0:
        raise RuntimeError("qdrant 二进制为空")
    with open(binary_path, "rb") as f:
        magic = f.read(4)
    if magic != b"\x7fELF":
        raise RuntimeError("qdrant 非 ELF 文件")


def fix_permissions(root):
    for dirpath, dirnames, filenames in os.walk(root, followlinks=False):
        dirp = pathlib.Path(dirpath)
        try:
            os.chmod(str(dirp), DIR_PERM)
        except OSError:
            pass
        for fn in filenames:
            fp = pathlib.Path(dirp) / fn
            if fp.is_symlink():
                continue
            try:
                relative = fp.relative_to(root)
            except ValueError:
                continue
            parts = relative.parts
            if len(parts) == 2 and parts[-2] == "bin" and parts[-1] == "qdrant":
                os.chmod(str(fp), BIN_PERM)
            elif parts[-1] == "LICENSE":
                os.chmod(str(fp), LICENSE_PERM)
            else:
                os.chmod(str(fp), FILE_PERM)


def build_file_manifest(root):
    entries = []
    root_path = pathlib.Path(root)
    for dirpath, dirnames, filenames in os.walk(root_path, followlinks=False):
        dirnames.sort()
        dirp = pathlib.Path(dirpath)
        rel_dir = dirp.relative_to(root_path)
        for dn in sorted(dirnames):
            dp = dirp / dn
            rel = (rel_dir / dn).as_posix() if str(rel_dir) != "." else dn
            if dp.is_symlink():
                entries.append({
                    "path": rel,
                    "type": "dirsymlink",
                    "size": 0,
                    "sha256": "",
                    "mode": "",
                })
        for fn in sorted(filenames):
            fp = dirp / fn
            rel = (rel_dir / fn).as_posix() if str(rel_dir) != "." else fn
            if fp.is_symlink():
                target = os.readlink(str(fp))
                entries.append({
                    "path": rel,
                    "type": "symlink",
                    "size": 0,
                    "sha256": "",
                    "mode": "",
                    "target": target,
                })
            else:
                st = fp.stat()
                entries.append({
                    "path": rel,
                    "type": "file",
                    "size": st.st_size,
                    "sha256": sha256_file(str(fp)),
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
        "platform": lock["platform"],
        "architecture": lock["architecture"],
        "rustTarget": lock["rustTarget"],
        "libc": lock["libc"],
        "distributionRoot": DIST_ROOT_NAME,
        "entrypoints": {
            "server": f"{DIST_ROOT_NAME}/bin/qdrant",
        },
        "source": {
            "type": "official-qdrant-release",
            "releaseTag": lock["releaseTag"],
            "releaseCommit": lock["releaseCommit"],
            "asset": lock["assetName"],
            "sha256": lock["assetSha256"],
        },
        "compatibility": {
            "guestPlatform": "linux",
            "architecture": "arm64",
            "pageSize4KTested": False,
            "pageSize16KTested": False,
        },
    }
    out = output_dir / "qdrant-runtime.json"
    with open(str(out), "w", encoding="utf-8", newline="") as f:
        json.dump(runtime_json, f, indent=2, sort_keys=True)
        f.write("\n")
    return out


def write_file_manifest(output_dir, root):
    manifest = build_file_manifest(root)
    out = output_dir / "file-manifest.json"
    content = json.dumps(manifest, indent=2, sort_keys=True, ensure_ascii=False) + "\n"
    with open(str(out), "w", encoding="utf-8", newline="") as f:
        f.write(content)
    return out


def write_sha256sums(output_dir, names):
    lines = []
    for name in sorted(names):
        fp = output_dir / name
        digest = sha256_file(str(fp))
        lines.append(f"{digest}  {name}")
    out = output_dir / "SHA256SUMS"
    with open(str(out), "w", encoding="utf-8", newline="") as f:
        f.write("\n".join(lines) + "\n")
    return out


def create_deterministic_tar(source_dir, output_path):
    source_path = pathlib.Path(source_dir)
    output_path = pathlib.Path(output_path)
    members = []
    for dirpath, dirnames, filenames in os.walk(source_path, followlinks=False):
        dirnames.sort()
        for dn in dirnames:
            dp = pathlib.Path(dirpath) / dn
            arcname = dp.relative_to(source_path)
            members.append((arcname, dp))
        for fn in sorted(filenames):
            fp = pathlib.Path(dirpath) / fn
            arcname = fp.relative_to(source_path)
            members.append((arcname, fp))
    members.sort(key=lambda x: x[0].as_posix())
    tmp_out = output_path.with_suffix(output_path.suffix + ".tmp")
    with lzma.open(str(tmp_out), "wb", preset=XZ_COMPRESSION_LEVEL) as xz:
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
                    info.linkname = os.readlink(str(fp))
                    tf.addfile(info)
                elif fp.is_dir():
                    info.type = tarfile.DIRTYPE
                    tf.addfile(info)
                else:
                    with open(str(fp), "rb") as fobj:
                        tf.addfile(info, fobj)
    os.replace(str(tmp_out), str(output_path))


def safe_replace_output(tmp_output, final_output):
    if final_output.exists():
        backup = final_output.with_name(final_output.name + ".old")
        if backup.exists():
            shutil.rmtree(str(backup))
        final_output.rename(str(backup))
        shutil.rmtree(str(backup), ignore_errors=True)
    final_output.parent.mkdir(parents=True, exist_ok=True)
    tmp_output.rename(str(final_output))


def clear_directory(target):
    for child in target.iterdir():
        if child.is_symlink() or child.is_file():
            child.unlink()
        elif child.is_dir():
            shutil.rmtree(str(child))


def run_build(args):
    lock = load_lock()
    print(f"[信息] Qdrant {lock['version']} ({lock['rustTarget']})")
    print(f"[信息] 平台 {lock['platform']}/{lock['architecture']}")
    print(f"[信息] 资产 {lock['assetName']}")
    print(f"[信息] Release {lock['releaseTag']} @ {lock['releaseCommit']}")

    cache_dir = pathlib.Path(args.cache_dir).resolve() if args.cache_dir else DEFAULT_CACHE_DIR
    work_dir = pathlib.Path(args.work_dir).resolve() if args.work_dir else DEFAULT_WORK_DIR
    output_dir = pathlib.Path(args.output_dir).resolve() if args.output_dir else DEFAULT_OUTPUT_DIR
    cache_dir.mkdir(parents=True, exist_ok=True)
    work_dir.mkdir(parents=True, exist_ok=True)

    if args.clean and output_dir.exists():
        print(f"[清理] 删除旧输出: {output_dir}")
        shutil.rmtree(str(output_dir))

    tmp_output = output_dir.with_name(output_dir.name + ".partial")
    if tmp_output.exists():
        shutil.rmtree(str(tmp_output))
    tmp_output.mkdir(parents=True, exist_ok=True)

    work_root = work_dir / "build"
    if work_root.exists():
        shutil.rmtree(str(work_root))
    work_root.mkdir(parents=True, exist_ok=True)

    final_archive_name = f"amitia-qdrant-v{lock['version']}-linux-{lock['architecture']}-musl.tar.xz"

    try:
        license_src = SCRIPT_DIR / lock["licenseFile"]
        if not license_src.exists():
            raise RuntimeError(f"许可证文件不存在: {license_src}")
        if sha256_file(str(license_src)) != lock["licenseSha256"]:
            raise RuntimeError("许可证 SHA 与锁文件不符")

        archive_path = acquire_archive(
            lock, cache_dir, work_dir,
            offline=args.offline, source_archive=args.source_archive,
        )
        print(f"[校验] 源资产 SHA: {lock['assetSha256']}")

        extract_root = safe_extract(archive_path, str(work_root))
        print(f"[解压] 完成: {extract_root}")

        binary = find_qdrant_binary(str(extract_root))
        print(f"[发现] qdrant 二进制: {binary}")

        validate_structure(binary)
        print("[结构] ELF Magic 校验通过")

        elf_info = elf_inspector.inspect(str(binary))
        print(f"[ELF] {elf_info['machine']} elf{elf_info['elfClass']} {elf_info['type']} "
              f"interpreter={'yes' if elf_info['hasInterpreter'] else 'no'} "
              f"dynamic={'yes' if elf_info['hasDynamicSegment'] else 'no'} "
              f"load_alignments={elf_info['loadSegmentAlignments']}")

        if elf_info["machine"] != "aarch64":
            raise RuntimeError(f"ELF Machine 不是 AArch64: {elf_info['machine']}")
        if elf_info["elfClass"] != 64:
            raise RuntimeError(f"ELF Class 不是 64 位: {elf_info['elfClass']}")
        if elf_info["endianness"] != "little":
            raise RuntimeError(f"ELF Byte Order 非小端: {elf_info['endianness']}")
        if elf_info["type"] not in ("executable", "pie"):
            raise RuntimeError(f"ELF Type 不可执行: {elf_info['type']}")
        if not elf_info["loadSegmentAlignments"]:
            raise RuntimeError("缺少 Load Segment Alignment")

        dist_root = tmp_output / DIST_ROOT_NAME
        bin_dir = dist_root / "bin"
        bin_dir.mkdir(parents=True, exist_ok=True)
        shutil.copy2(str(binary), str(bin_dir / "qdrant"))
        shutil.copy2(str(license_src), str(dist_root / "LICENSE"))

        fix_permissions(str(dist_root))
        print("[权限] 已规范化")

        runtime_json_path = write_runtime_json(tmp_output, lock)
        manifest_path = write_file_manifest(tmp_output, str(dist_root))
        print("[元数据] 已生成 qdrant-runtime.json / file-manifest.json")

        archive_out = tmp_output.parent / final_archive_name
        create_deterministic_tar(str(tmp_output), archive_out)
        print(f"[打包] 可复现归档: {archive_out}")

        final_sha = sha256_file(str(archive_out))
        print(f"[SHA] {final_sha}")

        write_sha256sums(
            tmp_output.parent,
            [final_archive_name, runtime_json_path.name, manifest_path.name],
        )
        print("[元数据] 已生成 SHA256SUMS")

        if output_dir.exists():
            clear_directory(str(output_dir))
        else:
            output_dir.parent.mkdir(parents=True, exist_ok=True)
        for child in tmp_output.iterdir():
            dest = output_dir / child.name
            shutil.move(str(child), str(dest))
        if tmp_output.exists():
            shutil.rmtree(str(tmp_output), ignore_errors=True)

        print(f"[发布] 输出目录: {output_dir}")
        print("[完成] Qdrant Linux ARM64 Runtime 构建成功")
        print(f"[产物] {output_dir / final_archive_name}")
        print(f"[元数据] {output_dir / 'qdrant-runtime.json'}")
        print(f"[清单] {output_dir / 'file-manifest.json'}")
        print(f"[校验] {output_dir / 'SHA256SUMS'}")
        print(f"[ELF] machine={elf_info['machine']} class={elf_info['elfClass']} "
              f"type={elf_info['type']} hasInterp={elf_info['hasInterpreter']} "
              f"hasDyn={elf_info['hasDynamicSegment']} "
              f"alignments={elf_info['loadSegmentAlignments']}")
        print("[注意] 4K/16K 页面测试结果由 verify.py --mode runtime 记录，不写入发布元数据")
    except Exception:
        if tmp_output.exists():
            shutil.rmtree(str(tmp_output), ignore_errors=True)
        raise
    finally:
        if work_root.exists():
            shutil.rmtree(str(work_root), ignore_errors=True)


def parse_args():
    parser = argparse.ArgumentParser(description="构建 Linux ARM64 Qdrant Runtime 产物")
    parser.add_argument("--clean", action="store_true", help="清理后重新构建")
    parser.add_argument("--offline", action="store_true", help="离线模式")
    parser.add_argument("--cache-dir", help="自定义缓存目录")
    parser.add_argument("--work-dir", help="自定义临时工作目录")
    parser.add_argument("--output-dir", help="自定义输出目录")
    parser.add_argument("--source-archive", help="指定源资产路径")
    parser.add_argument("--keep-work-dir", action="store_true", help="保留工作目录")
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
