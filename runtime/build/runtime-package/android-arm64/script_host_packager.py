import hashlib
import os
import pathlib
import re


FORBIDDEN_DIRS = {"src", "node_modules", "coverage", "tests", "__tests__", ".vscode", ".idea", ".git"}
FORBIDDEN_EXTS = {".test.js", ".spec.js", ".d.ts"}
ENTRY = "dist/index.js"
SECRET_PATTERNS = [
    re.compile(r"\bsk-[A-Za-z0-9]{20,}\b"),
    re.compile(r"Bearer\s+[A-Za-z0-9._~+/]+=*", re.IGNORECASE),
    re.compile(r"Authorization:\s*\S+", re.IGNORECASE),
    re.compile(r"api_key\s*=\s*[\"'][^\"']{16,}[\"']", re.IGNORECASE),
    re.compile(r"apiKey\s*:\s*[\"'][^\"']{16,}[\"']", re.IGNORECASE),
    re.compile(r"password\s*:\s*[\"'][^\"']{8,}[\"']", re.IGNORECASE),
    re.compile(r"secret\s*:\s*[\"'][^\"']{16,}[\"']", re.IGNORECASE),
]


def find_source_absolute_paths(text):
    patterns = [
        re.compile(r'''(?:^|["'\s=])(/(?:[a-zA-Z0-9_\-]+/){2,}[a-zA-Z0-9_\-./]*)'''),
        re.compile(r"([A-Z]:\\[^\"'\\\s]+)"),
    ]
    results = []
    for pat in patterns:
        for m in pat.finditer(text):
            candidate = m.group(1) if m.lastindex else m.group(0)
            candidate = candidate.lstrip("'\").:= ")
            if candidate and candidate.startswith(("/", "C:\\", "D:\\", "c:\\", "d:\\")) and "node_modules" not in candidate and ".js:" not in candidate:
                results.append(candidate)
    return list(set(results))


def package_host(host_dir):
    base = pathlib.Path(host_dir).resolve()
    dist_dir = base / "dist"
    if not dist_dir.is_dir():
        raise RuntimeError(f"dist 目录不存在: {dist_dir}")
    entry = dist_dir / ENTRY.split("/")[-1]
    if not entry.is_file():
        raise RuntimeError(f"入口文件缺失: {entry}")
    if entry.stat().st_size == 0:
        raise RuntimeError(f"入口文件为空: {entry}")
    with open(entry, "rb") as f:
        raw = f.read()
    try:
        raw.decode("utf-8")
    except UnicodeDecodeError:
        raise RuntimeError(f"入口文件非UTF-8: {entry}")
    dist_files = []
    for root, dirs, files in os.walk(str(dist_dir)):
        for d in list(dirs):
            if d in FORBIDDEN_DIRS:
                dirs.remove(d)
        for fname in files:
            full = pathlib.Path(root) / fname
            rel = full.relative_to(base).as_posix()
            _, ext = os.path.splitext(fname)
            full_name = full.name
            is_test = any(full_name.endswith(s) for s in FORBIDDEN_EXTS)
            if is_test:
                continue
            dist_files.append(full)
    if not dist_files:
        raise RuntimeError(f"dist 目录为空: {dist_dir}")
    tree_entries = []
    for fp in dist_files:
        rel = fp.relative_to(base).as_posix()
        st = fp.stat()
        sha = hashlib.sha256(fp.read_bytes()).hexdigest()
        tree_entries.append((rel, st.st_mode, st.st_size, sha))
    tree_entries.sort(key=lambda x: x[0])
    h = hashlib.sha256()
    for rel, mode, size, sha in tree_entries:
        h.update(rel.encode("utf-8"))
        h.update(b"\x00")
        h.update(mode.to_bytes(4, "little"))
        h.update(size.to_bytes(8, "little"))
        h.update(sha.encode("utf-8"))
        h.update(b"\x00")
    tree_sha = h.hexdigest()
    package_json = base / "package.json"
    extra_files = []
    if package_json.is_file():
        extra_files.append(package_json)
    results = []
    for fp in dist_files:
        rel = fp.relative_to(base).as_posix()
        results.append((rel, fp))
    for fp in extra_files:
        rel = fp.relative_to(base).as_posix()
        results.append((rel, fp))
    results.sort(key=lambda x: x[0])
    scan_file = str(base)
    for rel, fp in results:
        if fp.is_dir():
            continue
        if fp.suffix in (".js", ".mjs", ".json"):
            text = fp.read_text(encoding="utf-8", errors="ignore")
            for pat in SECRET_PATTERNS:
                m = pat.search(text)
                if m:
                    context = m.group(0)[:20] + "..." if len(m.group(0)) > 20 else m.group(0)
                    raise RuntimeError(f"安全告警 [{context}]: {scan_file} -> {rel}")
            if ".js" in fp.suffix or fp.suffix == ".mjs":
                paths = find_source_absolute_paths(text)
                if paths:
                    raise RuntimeError(f"源码绝对路径告警 ({paths[0]}): {rel}")
    return {
        "entry": ENTRY,
        "tree_sha256": tree_sha,
        "files": results,
        "dist_dir": str(dist_dir),
        "base_dir": str(base),
    }
