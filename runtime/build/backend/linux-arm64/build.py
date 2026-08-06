import argparse
import hashlib
import json
import os
import pathlib
import re
import shutil
import stat
import subprocess
import sys
import tempfile

import inspect_elf

LOCK_FILE_NAME = "backend-build.lock.json"
SCRIPT_DIR = pathlib.Path(__file__).resolve().parent
DEFAULT_OUTPUT_DIR = SCRIPT_DIR.parent.parent / "out" / "backend" / "linux-arm64"
DEFAULT_CACHE_DIR = SCRIPT_DIR.parent / ".cache" / "backend" / "linux-arm64"
DEFAULT_WORK_DIR = SCRIPT_DIR / ".work"

BUILD_INFO_PACKAGE = "github.com/u-ai/backend/internal/buildinfo"

GO_REQUIRED_VERSION_MAJOR = 1
GO_REQUIRED_VERSION_MINOR = 23

# Forbidden patterns in build declarations
FORBIDDEN_PATTERNS = [
    re.compile(r'import\s+"C"', re.MULTILINE),
    re.compile(r'GOOS\s*=\s*android|^android', re.MULTILINE | re.IGNORECASE),
    re.compile(r'gomobile', re.IGNORECASE),
    re.compile(r'buildmode\s*=\s*c-shared', re.IGNORECASE),
    re.compile(r'buildmode\s*=\s*c-archive', re.IGNORECASE),
    re.compile(r'CGO_ENABLED\s*=\s*1', re.IGNORECASE),
    re.compile(r'CC\s*=|CXX\s*=', re.IGNORECASE),
    re.compile(r'musl-gcc|aarch64-linux-gnu-gcc', re.IGNORECASE),
    re.compile(r'BuildTime|BuildDate|CompiledAt|Timestamp', re.IGNORECASE),
    re.compile(r'GOFLAGS\s*=', re.IGNORECASE),
]

BANNED_BUILD_COMMANDS = [
    "go get",
    "go mod tidy",
    "go mod edit",
    "go work sync",
    "go mod vendor",
]


class BuildError(Exception):
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


def load_lock(path=None):
    lock_path = pathlib.Path(path) if path else SCRIPT_DIR / LOCK_FILE_NAME
    with open(lock_path, "r", encoding="utf-8") as f:
        return json.load(f)


def validate_version(version, development=False):
    if version is None or version == "":
        raise BuildError("version 不能为空")
    v = version.strip()
    if v != version:
        raise BuildError("version 包含前后空白")
    if len(v) > 128:
        raise BuildError(f"version 超过 128 字符: {len(v)}")
    if " " in v:
        raise BuildError("version 包含空格")
    if "/" in v or "\\" in v or os.sep in v:
        raise BuildError("version 包含路径分隔符")
    if "\n" in v or "\r" in v:
        raise BuildError("version 包含换行")
    if not development:
        if v == "dev" or v == "unknown":
            raise BuildError(f"正式版本不得为 '{v}'")
    return v


def validate_commit(commit, development=False):
    if commit is None or commit == "":
        raise BuildError("commit 不能为空")
    c = commit.strip()
    if c != commit:
        raise BuildError("commit 包含前后空白")
    if development and c == "unknown":
        return c
    if len(c) not in (40, 64):
        raise BuildError(f"commit 长度必须为 40 或 64，实际为 {len(c)}")
    if not re.fullmatch(r"[0-9a-f]+", c):
        raise BuildError("commit 只能包含小写十六进制字符")
    if c == "0" * len(c):
        raise BuildError("commit 不能为全零")
    if "dirty" in c.lower():
        raise BuildError("commit 不能包含 'dirty'")
    return c


def find_script_dir():
    return pathlib.Path(__file__).resolve().parent


def resolve_repo_root():
    script_dir = find_script_dir()
    return script_dir.parent.parent.parent.parent


def build_isolated_env(toolchain, cache_dir):
    env = {}
    for key in list(os.environ.keys()):
        upper = key.upper()
        if upper.startswith("GO") or upper in ("CC", "CXX", "PKG_CONFIG", "CGO_ENABLED"):
            continue
        env[key] = os.environ[key]
    env["GOENV"] = "off"
    env["GOTOOLCHAIN"] = "local"
    env["GOWORK"] = "off"
    env["GOOS"] = "linux"
    env["GOARCH"] = "arm64"
    env["GOARM64"] = "v8.0"
    env["CGO_ENABLED"] = "0"
    env["GOCACHE"] = str(pathlib.Path(cache_dir) / "go-build")
    env["GOMODCACHE"] = str(pathlib.Path(cache_dir) / "go-mod")
    pathlib.Path(env["GOCACHE"]).mkdir(parents=True, exist_ok=True)
    pathlib.Path(env["GOMODCACHE"]).mkdir(parents=True, exist_ok=True)
    return env


def run_go_command(args, env, cwd=None, check=True):
    cmd = ["go"] + args
    result = subprocess.run(cmd, env=env, cwd=cwd, capture_output=True, text=True, check=False)
    if check and result.returncode != 0:
        stderr = result.stderr.strip() if result.stderr else ""
        raise BuildError(f"go 命令失败 ({' '.join(cmd)}): exit={result.returncode}\n{stderr}")
    return result


def verify_go_version(expected_version):
    result = subprocess.run(["go", "version"], capture_output=True, text=True, check=False)
    if result.returncode != 0:
        raise BuildError("无法获取 Go 版本")
    actual = result.stdout.strip()
    if expected_version not in actual:
        raise BuildError(f"Toolchain 版本不匹配: expected '{expected_version}', got '{actual}'")
    parts = actual.split()
    if len(parts) >= 2:
        version_str = parts[1]
        if version_str.startswith("go"):
            version_str = version_str[2:]
        ver_parts = version_str.split(".")
        if len(ver_parts) >= 2:
            try:
                major = int(ver_parts[0])
                minor = int(ver_parts[1])
                if major < GO_REQUIRED_VERSION_MAJOR or (major == GO_REQUIRED_VERSION_MAJOR and minor < GO_REQUIRED_VERSION_MINOR):
                    raise BuildError(f"Go 版本过低: {actual}，需要 >= go{GO_REQUIRED_VERSION_MAJOR}.{GO_REQUIRED_VERSION_MINOR}.0")
            except ValueError:
                pass
    return actual


def check_go_mod_replacements(source_dir):
    go_mod_path = pathlib.Path(source_dir) / "go.mod"
    if not go_mod_path.exists():
        raise BuildError(f"未找到 go.mod: {go_mod_path}")
    content = go_mod_path.read_text(encoding="utf-8")
    lines = content.split("\n")
    in_replace = False
    for i, line in enumerate(lines):
        stripped = line.strip()
        if stripped.startswith("replace ") and ("=>" in stripped or "(" in stripped):
            in_replace = True
        if in_replace:
            if "=>" in stripped:
                match = re.search(r'replace\s+(\S+)\s*=>\s*(\S+)', stripped)
                if match:
                    mod_path = match.group(1)
                    target = match.group(2)
                    if target.startswith(".") or target.startswith("/") or "\\" in target:
                        raise BuildError(
                            f"本地 Module Replace 阻断正式构建: "
                            f"module={mod_path}, target={target}"
                        )
                paren_match = re.search(r'=>\s*(\S+)', stripped)
                if paren_match:
                    target = paren_match.group(1)
                    if target.startswith(".") or target.startswith("/") or "\\" in target:
                        raise BuildError(f"本地 Module Replace 阻断正式构建: target={target}")
            if stripped == ")":
                in_replace = False


def check_build_entries(source_dir):
    go_mod_path = pathlib.Path(source_dir) / "go.mod"
    go_sum_path = pathlib.Path(source_dir) / "go.sum"
    if not go_mod_path.exists():
        raise BuildError("go.mod 不存在")
    if not go_sum_path.exists():
        raise BuildError("go.sum 不存在")
    return sha256_file(str(go_mod_path)), sha256_file(str(go_sum_path))


def compile_check_linux_arm64(env, source_dir):
    run_go_command(
        ["build", "-mod=readonly", "./..."],
        env=env,
        cwd=str(source_dir),
    )


def perform_cgo_audit(env, source_dir):
    result = subprocess.run(
        ["go", "list", "-mod=readonly", "-deps", "-json", "./cmd/server"],
        capture_output=True,
        text=True,
        env=env,
        cwd=str(source_dir),
        check=False,
    )
    if result.returncode != 0:
        raise BuildError(f"依赖审计失败: {result.stderr}")
    packages_text = result.stdout
    decoder = json.JSONDecoder()
    idx = 0
    cgo_detected = []
    while idx < len(packages_text):
        while idx < len(packages_text) and packages_text[idx] in ("\n", " ", "\t", "\r"):
            idx += 1
        if idx >= len(packages_text):
            break
        try:
            obj, end = decoder.raw_decode(packages_text, idx)
        except json.JSONDecodeError:
            break
        import_path = obj.get("ImportPath", "<unknown>")
        cgo_files = obj.get("CgoFiles", [])
        if cgo_files:
            cgo_detected.append({
                "import_path": import_path,
                "cgo_files": cgo_files,
            })
        idx = end
    if cgo_detected:
        raise BuildError(f"检测到 CGO 依赖: {cgo_detected}")
    return True


def build_backend(env, source_dir, version, commit, output_path):
    ldflags = [
        "-s", "-w",
        "-buildid=",
        "-X", f"{BUILD_INFO_PACKAGE}.Version={version}",
        "-X", f"{BUILD_INFO_PACKAGE}.Commit={commit}",
        "-X", f"{BUILD_INFO_PACKAGE}.Target=linux-arm64",
    ]
    cmd = [
        "go", "build",
        "-mod=readonly",
        "-trimpath",
        "-buildvcs=false",
        "-buildmode=exe",
        "-ldflags", " ".join(ldflags),
        "-o", str(output_path),
        "./cmd/server",
    ]
    result = subprocess.run(cmd, env=env, cwd=str(source_dir), capture_output=True, text=True, check=False)
    if result.returncode != 0:
        stderr = result.stderr.strip() if result.stderr else ""
        raise BuildError(f"go build 失败: exit={result.returncode}\n{stderr}")
    return str(output_path)


def generate_metadata(output_dir, lock, version, commit, binary_path, dependency_modules, development=False):
    binary_sha = sha256_file(str(binary_path))
    binary_size = pathlib.Path(binary_path).stat().st_size

    elf_info = inspect_elf.inspect(str(binary_path))

    build_inputs = {
        "schemaVersion": 1,
        "componentId": lock["componentId"],
        "version": version,
        "commit": commit,
        "modulePath": "github.com/u-ai/backend",
        "entryPackage": "./cmd/server",
        "toolchain": lock["toolchain"]["version"],
        "target": {
            "goos": "linux",
            "goarch": "arm64",
            "goarm64": "v8.0",
            "cgoEnabled": False,
        },
        "flags": {
            "trimPath": True,
            "buildVCS": False,
            "buildMode": "exe",
            "stripSymbols": True,
            "stripDWARF": True,
            "clearBuildID": True,
            "moduleMode": "readonly",
            "workspaceMode": "off",
        },
    }

    artifact = {
        "schemaVersion": 1,
        "componentId": lock["componentId"],
        "name": "amitia-server",
        "version": version,
        "commit": commit,
        "release": not development,
        "platform": "linux",
        "architecture": "arm64",
        "goarm64": "v8.0",
        "cgoEnabled": False,
        "fileName": "amitia-server",
        "installPath": "/opt/amitia/backend/amitia-server",
        "sha256": binary_sha,
        "size": binary_size,
        "elf": {
            "class": elf_info["elfClass"],
            "endianness": elf_info["endianness"],
            "machine": elf_info["machine"],
            "type": elf_info["type"],
            "static": True,
            "interpreter": None,
            "neededLibraries": [],
        },
    }

    manifest_dir = pathlib.Path(output_dir) / "manifest"
    manifest_dir.mkdir(parents=True, exist_ok=True)

    build_inputs_path = manifest_dir / "build-inputs.json"
    artifact_path = manifest_dir / "backend-artifact.json"

    with open(str(build_inputs_path), "w", encoding="utf-8", newline="") as f:
        json.dump(build_inputs, f, indent=2, ensure_ascii=False)
        f.write("\n")

    with open(str(artifact_path), "w", encoding="utf-8", newline="") as f:
        json.dump(artifact, f, indent=2, ensure_ascii=False)
        f.write("\n")

    dep_manifest = {
        "schemaVersion": 1,
        "modulePath": "github.com/u-ai/backend",
        "modules": dependency_modules,
    }
    dep_path = manifest_dir / "dependency-manifest.json"
    with open(str(dep_path), "w", encoding="utf-8", newline="") as f:
        json.dump(dep_manifest, f, indent=2, ensure_ascii=False)
        f.write("\n")

    return build_inputs_path, artifact_path, dep_path


def collect_dependency_modules(env, source_dir):
    list_env = dict(env)
    list_env["GOPROXY"] = "https://proxy.golang.org,direct"
    result = run_go_command(
        ["list", "-mod=readonly", "-m", "-json", "all"],
        env=list_env,
        cwd=str(source_dir),
    )
    text = result.stdout.strip()
    modules = []
    decoder = json.JSONDecoder()
    idx = 0
    while idx < len(text):
        while idx < len(text) and text[idx] in ("\n", " ", "\t", "\r"):
            idx += 1
        if idx >= len(text):
            break
        try:
            obj, end = decoder.raw_decode(text, idx)
        except json.JSONDecodeError:
            break
        if isinstance(obj, dict):
            path = obj.get("Path", "")
            version = obj.get("Version", "")
            indirect = obj.get("Indirect", False)
            if path == "github.com/u-ai/backend":
                idx = end
                continue
            sum_val = obj.get("Sum", "")
            go_mod_sum = obj.get("GoModSum", "")
            modules.append({
                "path": path,
                "version": version,
                "sum": sum_val,
                "goModSum": go_mod_sum,
                "indirect": indirect,
            })
        idx = end
    modules.sort(key=lambda m: m["path"])
    return modules


def generate_go_version_metadata(binary_path):
    result = subprocess.run(
        ["go", "version", "-m", str(binary_path)],
        capture_output=True,
        text=True,
        check=False,
    )
    if result.returncode != 0:
        raise BuildError(f"go version -m 失败: {result.stderr}")
    binary_name = pathlib.Path(binary_path).name
    binary_path_str = str(binary_path)
    binary_parent_str = str(binary_path.parent) + os.sep
    lines = result.stdout.splitlines()
    filtered = []
    for line in lines:
        if "vcs.time" in line or "vcs.modified" in line:
            continue
        if line.startswith(binary_path_str):
            line = binary_name + line[len(binary_path_str):]
        elif line.startswith(binary_parent_str):
            continue
        filtered.append(line)
    return "\n".join(filtered) + "\n"


def compute_sha256sums(output_dir, files):
    lines = []
    for name in sorted(files):
        fp = pathlib.Path(output_dir) / name
        digest = sha256_file(str(fp))
        lines.append(f"{digest}  {name}")
    return "\n".join(lines) + "\n"


def safe_replace_directory(tmp_dir, final_dir):
    final_dir = pathlib.Path(final_dir)
    if final_dir.exists():
        backup = final_dir.with_name(final_dir.name + ".old")
        if backup.exists():
            shutil.rmtree(str(backup), ignore_errors=True)
        final_dir.rename(str(backup))
        shutil.rmtree(str(backup), ignore_errors=True)
    final_dir.parent.mkdir(parents=True, exist_ok=True)
    tmp_dir.rename(str(final_dir))


def clear_directory(target):
    target = pathlib.Path(target)
    for child in target.iterdir():
        if child.is_symlink() or child.is_file():
            child.unlink()
        elif child.is_dir():
            shutil.rmtree(str(child), ignore_errors=True)


def run_build(args):
    lock = load_lock()
    development = args.development

    tool_version = lock["toolchain"]["version"]
    version = args.version
    commit = args.commit

    if args.download:
        version = "_download_"
        commit = "_download_"
    else:
        if version is None:
            if development:
                version = "dev"
            else:
                raise BuildError("正式模式必须提供 --version")
        if commit is None:
            if development:
                commit = "unknown"
            else:
                raise BuildError("正式模式必须提供 --commit")

    version = validate_version(version, development)
    commit = validate_commit(commit, development)

    cache_dir = pathlib.Path(args.cache_dir).resolve() if args.cache_dir else DEFAULT_CACHE_DIR
    work_dir = pathlib.Path(args.work_dir).resolve() if args.work_dir else DEFAULT_WORK_DIR
    output_dir = pathlib.Path(args.output_dir).resolve() if args.output_dir else DEFAULT_OUTPUT_DIR
    source_dir = pathlib.Path(args.source_dir).resolve() if args.source_dir else resolve_repo_root() / "backend"

    cache_dir.mkdir(parents=True, exist_ok=True)
    work_dir.mkdir(parents=True, exist_ok=True)
    output_dir.mkdir(parents=True, exist_ok=True)

    if args.clean:
        work_dir_clean = work_dir / "build"
        if work_dir_clean.exists():
            shutil.rmtree(str(work_dir_clean), ignore_errors=True)
        out_partial = output_dir.with_name(output_dir.name + ".partial")
        if out_partial.exists():
            shutil.rmtree(str(out_partial), ignore_errors=True)

    tmp_output = output_dir.with_name(output_dir.name + ".partial")
    if tmp_output.exists():
        shutil.rmtree(str(tmp_output), ignore_errors=True)
    tmp_output.mkdir(parents=True, exist_ok=True)

    work_root = work_dir / "build"
    if work_root.exists():
        shutil.rmtree(str(work_root), ignore_errors=True)
    work_root.mkdir(parents=True, exist_ok=True)

    try:
        print(f"[信息] 锁定 Toolchain: {tool_version}")
        print(f"[信息] 目标: linux/arm64 (GOARM64=v8.0, CGO_ENABLED=0)")
        print(f"[信息] 版本: {version}, 提交: {commit}")
        print(f"[信息] {'开发构建' if development else '正式构建'}")

        actual_go_version = verify_go_version(tool_version)
        print(f"[Toolchain] {actual_go_version}")

        if not (source_dir / "go.mod").exists():
            raise BuildError(f"源目录不包含 go.mod: {source_dir}")
        if not (source_dir / "cmd" / "server").exists():
            raise BuildError(f"源目录不包含 cmd/server: {source_dir}")

        check_go_mod_replacements(source_dir)

        pre_go_mod_hash, pre_go_sum_hash = check_build_entries(str(source_dir))
        print(f"[校验] go.mod SHA: {pre_go_mod_hash[:16]}...")
        print(f"[校验] go.sum SHA: {pre_go_sum_hash[:16]}...")

        post_go_mod_hash = sha256_file(str(source_dir / "go.mod"))
        post_go_sum_hash = sha256_file(str(source_dir / "go.sum"))
        if post_go_mod_hash != pre_go_mod_hash or post_go_sum_hash != pre_go_sum_hash:
            raise BuildError("go.mod/go.sum 在构建前已被修改")

        env = build_isolated_env(tool_version, cache_dir)

        if args.download:
            print("[准备] 下载模块...")
            run_go_command(["mod", "download"], env=env, cwd=str(source_dir))
            run_go_command(["mod", "verify"], env=env, cwd=str(source_dir))
            print("[准备] 模块下载完成")
            return 0

        if args.offline:
            env["GOPROXY"] = "off"
            print("[模式] 离线构建")
        else:
            env["GOPROXY"] = "off"
            print("[模式] 离线构建 (默认)")

        print("[准备] 填充专用 Module Cache...")
        download_env = dict(env)
        download_env["GOPROXY"] = "https://proxy.golang.org,direct"
        run_go_command(["mod", "download"], env=download_env, cwd=str(source_dir))

        print("[校验] go mod verify...")
        run_go_command(["mod", "verify"], env=env, cwd=str(source_dir))

        print("[审计] CGO 依赖审计...")
        perform_cgo_audit(env, source_dir)
        print("[审计] 通过: 无 CGO 依赖")

        print("[编译] Linux ARM64 全量编译检查...")
        compile_check_linux_arm64(env, source_dir)
        print("[编译] 全量编译检查通过")

        print("[构建] 编译 amitia-server...")
        binary_path = tmp_output / "backend" / "amitia-server"
        binary_path.parent.mkdir(parents=True, exist_ok=True)
        build_backend(env, source_dir, version, commit, binary_path)

        mode = stat.S_IMODE(binary_path.stat().st_mode)
        if not (mode & 0o111):
            os.chmod(str(binary_path), 0o755)
            mode = 0o755
        print(f"[构建] 成功: {binary_path} (权限: {oct(mode)})")

        print("[检查] ELF 结构验证...")
        elf_info = inspect_elf.inspect(str(binary_path))
        print(f"[ELF] machine={elf_info['machine']} class={elf_info['elfClass']} "
              f"type={elf_info['type']}")

        print("[检查] 源码路径泄漏扫描...")
        final_source_root = str(resolve_repo_root()).encode("utf-8")
        with open(str(binary_path), "rb") as f:
            binary_content = f.read()
        if final_source_root in binary_content:
            raise BuildError("二进制包含源码绝对路径，启用 -trimpath 后不应出现")
        print("[检查] 通过: 未检测到源码路径泄漏")

        dep_modules = collect_dependency_modules(env, source_dir)
        print(f"[信息] 依赖模块数量: {len(dep_modules)}")

        print("[元数据] 生成组件元数据...")
        build_inputs_path, artifact_path, dep_path = generate_metadata(
            str(tmp_output), lock, version, commit, binary_path, dep_modules, development
        )

        print("[元数据] 生成 go version 信息...")
        go_ver_meta = generate_go_version_metadata(binary_path)
        go_ver_path = pathlib.Path(tmp_output) / "manifest" / "go-version-metadata.txt"
        with open(str(go_ver_path), "w", encoding="utf-8", newline="") as f:
            f.write(go_ver_meta)

        print("[归档] 创建确定性归档...")
        archive_name = f"amitia-backend-{version}-linux-arm64.tar.xz"
        archive_path = tmp_output / archive_name
        create_archive_script = SCRIPT_DIR / "archive.py"
        result = subprocess.run(
            [sys.executable, str(create_archive_script), "create", str(tmp_output), str(archive_path)],
            capture_output=True, text=True, check=False
        )
        if result.returncode != 0:
            raise BuildError(f"归档创建失败: {result.stderr}")
        print(f"[归档] 成功: {archive_path}")

        print("[SHA] 生成 SHA256SUMS...")
        files_for_sums = [
            "backend/amitia-server",
            "manifest/backend-artifact.json",
            "manifest/build-inputs.json",
            "manifest/dependency-manifest.json",
            "manifest/go-version-metadata.txt",
            archive_name,
        ]
        sums_content = compute_sha256sums(str(tmp_output), files_for_sums)
        sums_path = pathlib.Path(tmp_output) / "SHA256SUMS"
        with open(str(sums_path), "w", encoding="utf-8", newline="") as f:
            f.write(sums_content)

        print("[发布] 写入正式输出...")
        if output_dir.exists():
            clear_directory(str(output_dir))
        else:
            output_dir.parent.mkdir(parents=True, exist_ok=True)
        for child in tmp_output.iterdir():
            dest = output_dir / child.name
            if dest.exists():
                if dest.is_dir():
                    shutil.rmtree(str(dest), ignore_errors=True)
                else:
                    dest.unlink()
            shutil.move(str(child), str(dest))
        if tmp_output.exists():
            shutil.rmtree(str(tmp_output), ignore_errors=True)

        print("")
        print("=" * 50)
        print(f"[完成] Amitia Backend Linux ARM64 构建成功")
        print(f"[输出] {output_dir}")
        print(f"[二进制] {output_dir / 'backend' / 'amitia-server'}")
        print(f"[归档] {output_dir / archive_name}")
        print(f"[SHA256] {sha256_file(str(output_dir / 'backend' / 'amitia-server'))}")
        print("=" * 50)

        return 0

    except BuildError:
        if tmp_output.exists():
            shutil.rmtree(str(tmp_output), ignore_errors=True)
        raise
    except Exception as e:
        if tmp_output.exists():
            shutil.rmtree(str(tmp_output), ignore_errors=True)
        raise BuildError(f"构建失败: {e}")
    finally:
        if not args.keep_work_dir and work_root.exists():
            shutil.rmtree(str(work_root), ignore_errors=True)


def create_archive(source_dir, output_path):
    result = subprocess.run(
        [sys.executable, str(SCRIPT_DIR / "archive.py"), "create", str(source_dir), str(output_path)],
        capture_output=True, text=True, check=False
    )
    if result.returncode != 0:
        raise BuildError(f"归档失败: {result.stderr}")


def parse_args():
    parser = argparse.ArgumentParser(description="构建 Linux ARM64 Go 后端产物")
    parser.add_argument("--version", help="正式版本号")
    parser.add_argument("--commit", help="完整 Git Commit (40 或 64 字符)")
    parser.add_argument("--clean", action="store_true", help="清理后重新构建")
    parser.add_argument("--offline", action="store_true", help="离线模式")
    parser.add_argument("--download", action="store_true", help="仅下载模块")
    parser.add_argument("--development", action="store_true", help="开发构建模式")
    parser.add_argument("--source-dir", help="源码目录")
    parser.add_argument("--output-dir", help="输出目录")
    parser.add_argument("--cache-dir", help="缓存目录")
    parser.add_argument("--work-dir", help="临时工作目录")
    parser.add_argument("--keep-work-dir", action="store_true", help="保留工作目录")
    parser.add_argument("--skip-host-tests", action="store_true", help="跳过 Host 测试")
    parser.add_argument("--skip-arm64-smoke-test", action="store_true", help="跳过 ARM64 冒烟测试")
    return parser.parse_args()


def main():
    args = parse_args()
    if args.offline and args.download:
        print("[错误] --offline 与 --download 互斥", file=sys.stderr)
        sys.exit(1)
    if not args.offline and not args.download:
        args.offline = True
    try:
        sys.exit(run_build(args))
    except BuildError as e:
        print(f"[错误] {e}", file=sys.stderr)
        sys.exit(1)


if __name__ == "__main__":
    main()
