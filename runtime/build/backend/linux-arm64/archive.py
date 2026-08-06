import hashlib
import io
import json
import lzma
import os
import pathlib
import struct
import sys
import tarfile
import tempfile

FIXED_MTIME = 0
FIXED_UID = 0
FIXED_GID = 0
FIXED_UNAME = ""
FIXED_GNAME = ""

DIR_PERM = 0o755
FILE_PERM = 0o644
BIN_PERM = 0o755

XZ_COMPRESSION_LEVEL = 5


class ArchiveError(Exception):
    pass


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


def validate_member_path(name):
    if not name:
        raise ArchiveError("归档成员路径为空")
    if name.startswith("/"):
        raise ArchiveError(f"绝对路径成员: {name}")
    if os.name == "nt" and len(name) > 1 and name[1] == ":":
        raise ArchiveError(f"Windows 盘符成员: {name}")
    parts = pathlib.PurePosixPath(name).parts
    if ".." in parts:
        raise ArchiveError(f"路径穿越成员: {name}")


def sort_key(name):
    parts = pathlib.PurePosixPath(name).parts
    return tuple(p.lower() for p in parts)


def collect_members(source_dir):
    source_path = pathlib.Path(source_dir)
    if not source_path.exists():
        raise ArchiveError(f"源目录不存在: {source_dir}")
    members = []
    for dirpath, dirnames, filenames in os.walk(source_path, followlinks=False):
        dirnames.sort()
        dirp = pathlib.Path(dirpath)
        for dn in sorted(dirnames):
            dp = dirp / dn
            arcname = dp.relative_to(source_path)
            try:
                validate_member_path(str(arcname))
            except ArchiveError as e:
                raise ArchiveError(f"目录成员违规: {e}")
            members.append((arcname, dp, "dir"))
        for fn in sorted(filenames):
            fp = dirp / fn
            arcname = fp.relative_to(source_path)
            try:
                validate_member_path(str(arcname))
            except ArchiveError as e:
                raise ArchiveError(f"文件成员违规: {e}")
            members.append((arcname, fp, "file"))
    members.sort(key=lambda x: sort_key(x[0].as_posix()))
    return members


def get_file_mode(arcname, fp):
    name_str = arcname.as_posix()
    if name_str.startswith("backend/") and name_str.endswith("/amitia-server"):
        return BIN_PERM
    if fp.name == "amitia-server":
        try:
            st = fp.stat()
            if st.st_mode & 0o111:
                return BIN_PERM
        except OSError:
            pass
    return FILE_PERM


def create_archive(source_dir, output_path):
    source_path = pathlib.Path(source_dir)
    output_path = pathlib.Path(output_path)
    members = collect_members(str(source_path))
    tmp_out = output_path.with_suffix(output_path.suffix + ".tmp")

    with lzma.open(str(tmp_out), "wb", preset=XZ_COMPRESSION_LEVEL) as xz:
        with tarfile.open(fileobj=xz, mode="w") as tf:
            for arcname, fp, entry_type in members:
                info = tarfile.TarInfo(name=arcname.as_posix())
                info.uid = FIXED_UID
                info.gid = FIXED_GID
                info.uname = FIXED_UNAME
                info.gname = FIXED_GNAME
                info.mtime = FIXED_MTIME
                if entry_type == "dir":
                    info.type = tarfile.DIRTYPE
                    info.mode = DIR_PERM
                    tf.addfile(info)
                elif fp.is_symlink():
                    info.type = tarfile.SYMTYPE
                    info.linkname = os.readlink(str(fp))
                    tf.addfile(info)
                else:
                    info.mode = get_file_mode(arcname, fp)
                    info.size = fp.stat().st_size
                    with open(str(fp), "rb") as fobj:
                        tf.addfile(info, fobj)

    os.replace(str(tmp_out), str(output_path))
    return sha256_file(str(output_path))


def compute_sha256sums(output_dir, files):
    lines = []
    for name in sorted(files):
        fp = pathlib.Path(output_dir) / name
        if fp.exists():
            digest = sha256_file(str(fp))
            lines.append(f"{digest}  {name}")
    return "\n".join(lines) + "\n"


def verify_archive(archive_path):
    issues = []
    errors = []
    archive_path = pathlib.Path(archive_path)
    if not archive_path.exists():
        return ["归档不存在"], []
    try:
        with lzma.open(str(archive_path), "rb") as xz:
            with tarfile.open(fileobj=xz, mode="r") as tf:
                members = tf.getmembers()
                if not members:
                    errors.append("归档为空")
                    return issues, errors
                names = [m.name for m in members]
                sorted_names = sorted(names, key=lambda x: tuple(p.lower() for p in pathlib.PurePosixPath(x).parts))
                if names != sorted_names:
                    errors.append("归档成员未排序")
                for m in members:
                    if m.uid != 0 or m.gid != 0:
                        issues.append(f"uid/gid 非 0: {m.name}")
                    if m.uname != "" or m.gname != "":
                        issues.append(f"uname/gname 非空: {m.name}")
                    if m.mtime != 0:
                        issues.append(f"mtime 非 0: {m.name}")
                    validate_member_path(m.name)
    except ArchiveError as e:
        errors.append(str(e))
    except Exception as e:
        errors.append(f"归档解析失败: {e}")
    return issues, errors


def main():
    if len(sys.argv) < 3:
        print("用法: python archive.py <create|verify> <source_dir|archive_path> [output_path]", file=sys.stderr)
        sys.exit(1)
    action = sys.argv[1]
    if action == "create" and len(sys.argv) >= 4:
        source = sys.argv[2]
        output = sys.argv[3]
        try:
            sha = create_archive(source, output)
            print(f"归档创建成功: {output}")
            print(f"SHA-256: {sha}")
        except ArchiveError as e:
            print(f"[错误] {e}", file=sys.stderr)
            sys.exit(1)
    elif action == "verify":
        archive = sys.argv[2]
        issues, errors = verify_archive(archive)
        if errors:
            for e in errors:
                print(f"[错误] {e}", file=sys.stderr)
            sys.exit(1)
        if issues:
            for i in issues:
                print(f"[警告] {i}")
            sys.exit(1)
        print("归档验证通过")
    else:
        print("无效操作", file=sys.stderr)
        sys.exit(1)


if __name__ == "__main__":
    main()
