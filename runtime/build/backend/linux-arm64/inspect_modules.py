import json
import os
import pathlib
import subprocess
import sys


class ModuleAuditError(Exception):
    pass


def run_go_list(go_bin, source_dir, env):
    cmd = [
        go_bin,
        "list", "-mod=readonly", "-deps", "-json",
        "./cmd/server",
    ]
    try:
        result = subprocess.run(
            cmd,
            cwd=source_dir,
            env=env,
            capture_output=True,
            text=True,
            check=False,
        )
    except OSError as e:
        raise ModuleAuditError(f"无法执行 go list: {e}")
    if result.returncode != 0:
        stderr = result.stderr.strip() if result.stderr else ""
        raise ModuleAuditError(f"go list 失败 (exit={result.returncode}): {stderr}")
    return result.stdout


def parse_packages(stdout):
    packages = []
    decoder = json.JSONDecoder()
    text = stdout.strip()
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
        packages.append(obj)
        idx = end
    return packages


def audit_modules(go_bin, source_dir, env):
    stdout = run_go_list(go_bin, source_dir, env)
    packages = parse_packages(stdout)
    if not packages:
        raise ModuleAuditError("go list 未返回任何 Package")

    issues = []
    modules = {}
    cgo_files_found = []
    local_replaces = []

    for pkg in packages:
        import_path = pkg.get("ImportPath", "<unknown>")
        module_info = pkg.get("Module", {})
        module_path = module_info.get("path", "")
        module_version = module_info.get("version", "")
        is_standard = pkg.get("Standard", False)

        if module_info:
            mod_path = module_info.get("path", "")
            if mod_path and mod_path not in modules:
                modules[mod_path] = {
                    "path": mod_path,
                    "version": module_info.get("version", ""),
                    "replace": module_info.get("Replace"),
                    "dir": module_info.get("Dir", ""),
                }
            replace = module_info.get("Replace")
            if replace:
                replace_target = replace.get("path", "")
                if replace_target and (replace_target.startswith("../") or replace_target.startswith("./") or replace_target.startswith("/") or "\\" in replace_target):
                    local_replaces.append({
                        "module": mod_path,
                        "target": replace_target,
                    })

        if "Error" in pkg and pkg["Error"]:
            issues.append({
                "type": "load_error",
                "import_path": import_path,
                "error": pkg["Error"].get("Err", "unknown error"),
            })

        if pkg.get("InvalidDepsBuild"):
            issues.append({
                "type": "invalid_deps_build",
                "import_path": import_path,
                "error": pkg["InvalidDepsBuild"],
            })

        cgo_files = pkg.get("CgoFiles", [])
        if cgo_files:
            cgo_files_found.append({
                "import_path": import_path,
                "cgo_files": cgo_files,
                "module": module_path,
            })

        ignored = pkg.get("IgnoredGoFiles", [])
        if ignored and not pkg.get("GoFiles") and not pkg.get("CFiles") and not cgo_files:
            issues.append({
                "type": "all_files_excluded",
                "import_path": import_path,
                "ignored": ignored,
            })

    if cgo_files_found:
        for entry in cgo_files_found:
            issues.append({
                "type": "cgo_detected",
                "import_path": entry["import_path"],
                "module": entry["module"],
                "cgo_files": entry["cgo_files"],
            })

    for rep in local_replaces:
        issues.append({
            "type": "local_replace",
            "module": rep["module"],
            "target": rep["target"],
        })

    return modules, issues


def find_go_binary():
    env = os.environ.copy()
    env["GOENV"] = "off"
    env["GOTOOLCHAIN"] = "local"
    result = subprocess.run(["go", "env", "GOROOT"], capture_output=True, text=True, env=env, check=False)
    if result.returncode == 0:
        goroot = result.stdout.strip()
        if goroot:
            candidate = pathlib.Path(goroot) / "bin" / "go"
            if candidate.exists():
                return str(candidate)
    return "go"


def main():
    if len(sys.argv) < 2:
        print("用法: python inspect_modules.py <source_dir>", file=sys.stderr)
        sys.exit(1)
    source_dir = pathlib.Path(sys.argv[1]).resolve()
    go_env = os.environ.copy()
    go_env["GOOS"] = "linux"
    go_env["GOARCH"] = "arm64"
    go_env["GOARM64"] = "v8.0"
    go_env["CGO_ENABLED"] = "0"
    go_env["GOENV"] = "off"
    go_env["GOTOOLCHAIN"] = "local"
    go_env["GOWORK"] = "off"
    go_bin = find_go_binary()
    try:
        modules, issues = audit_modules(go_bin, str(source_dir), go_env)
        report = {
            "modules": {k: v for k, v in sorted(modules.items())},
            "issues": issues,
            "cgo_detected": any(i["type"] == "cgo_detected" for i in issues),
            "local_replace_detected": any(i["type"] == "local_replace" for i in issues),
            "load_errors": [i for i in issues if i["type"] == "load_error"],
        }
        print(json.dumps(report, indent=2, ensure_ascii=False))
        if report["cgo_detected"] or report["local_replace_detected"] or report["load_errors"]:
            sys.exit(2)
    except ModuleAuditError as e:
        print(f"[审计错误] {e}", file=sys.stderr)
        sys.exit(1)


if __name__ == "__main__":
    main()
