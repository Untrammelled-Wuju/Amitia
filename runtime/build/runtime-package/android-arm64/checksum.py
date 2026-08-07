import hashlib
import pathlib
import struct
import tarfile


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


def compute_tree_sha256(file_paths, base_dir):
    base = pathlib.Path(base_dir).resolve()
    entries = []
    for p in file_paths:
        fp = pathlib.Path(p).resolve()
        rel = fp.relative_to(base).as_posix()
        st = fp.stat()
        if fp.is_dir():
            entry = f"d\x00{rel}\x00{st.st_mode}"
            entries.append(entry)
        elif fp.is_symlink():
            target = os.readlink(fp) if hasattr(os, 'readlink') else str(fp)
            entry = f"l\x00{rel}\x00{target}"
            entries.append(entry)
        else:
            file_sha = sha256_file(fp)
            entry = f"f\x00{rel}\x00{st.st_mode}\x00{st.st_size}\x00{file_sha}"
            entries.append(entry)
    entries.sort()
    h = hashlib.sha256()
    for e in entries:
        h.update(e.encode("utf-8"))
        h.update(b"\x00")
    return h.hexdigest()


def compute_archive_tree_sha(archive_path, members):
    sorted_members = sorted(members, key=lambda m: m["path"])
    h = hashlib.sha256()
    for m in sorted_members:
        h.update(m["path"].encode("utf-8"))
        h.update(b"\x00")
        h.update(m["mode"].to_bytes(4, "little"))
        h.update(m["size"].to_bytes(8, "little"))
        h.update(m["sha256"].encode("utf-8"))
        h.update(b"\x00")
    return h.hexdigest()


def extract_regular_files_sha_map(tar_path):
    result = {}
    with tarfile.open(tar_path, "r:*") as tf:
        for m in tf.getmembers():
            if m.isfile():
                data = tf.extractfile(m).read()
                result[m.name] = sha256_bytes(data)
    return result
