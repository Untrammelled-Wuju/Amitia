import hashlib
import os
import pathlib
import shutil
import zipfile


FIXED_ZIP_TIME = (1980, 1, 1, 0, 0, 0)
MAX_ZIP_ENTRIES = 1000


def write_zip(payload, output_path):
    out_path = pathlib.Path(output_path)
    out_path.parent.mkdir(parents=True, exist_ok=True)
    work_dir = out_path.parent
    tmp_path = work_dir / (out_path.name + ".tmp")
    payload_entries = sorted(payload, key=lambda x: x["name"])
    if len(payload_entries) > MAX_ZIP_ENTRIES:
        raise RuntimeError(f"ZIP Entry数量超限: {len(payload_entries)}")
    seen = set()
    for entry in payload_entries:
        name = entry["name"]
        if name in seen:
            raise RuntimeError(f"ZIP重复Entry: {name}")
        seen.add(name)
        if name.startswith("/") or "\\" in name or ".." in name.split("/"):
            raise RuntimeError(f"ZIP非法路径: {name}")
    zf = zipfile.ZipFile(str(tmp_path), "w", compression=zipfile.ZIP_STORED, compresslevel=0,
                         allowZip64=False, strict_timestamps=False)
    try:
        for entry in payload_entries:
            name = entry["name"]
            data = entry["data"]
            info = zipfile.ZipInfo(filename=name, date_time=FIXED_ZIP_TIME)
            info.compress_type = zipfile.ZIP_STORED
            info.create_system = 3
            info.external_attr = 0
            info.comment = b""
            info.extra = b""
            zf.writestr(info, data, compress_type=zipfile.ZIP_STORED)
    finally:
        zf.close()
    if out_path.exists():
        out_path.unlink()
    shutil.move(str(tmp_path), str(out_path))
    return str(out_path), hashlib.sha256(out_path.read_bytes()).hexdigest()


def write_sha_file(sha_hex, target_name, output_path):
    out_path = pathlib.Path(output_path)
    out_path.parent.mkdir(parents=True, exist_ok=True)
    tmp = out_path.parent / (out_path.name + ".tmp")
    tmp.write_text(f"{sha_hex}  {target_name}\n", encoding="utf-8")
    if out_path.exists():
        out_path.unlink()
    shutil.move(str(tmp), str(out_path))
    return str(out_path)
