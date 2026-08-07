import hashlib
import json
import pathlib
import re


SEMVER_RE = re.compile(r"^[0-9]+\.[0-9]+\.[0-9]+(-[0-9A-Za-z.]+)?(\+[0-9A-Za-z.]+)?$")
COMMIT_RE = re.compile(r"^[0-9a-f]{40}$")


def validate_runtime_version(version):
    if not version:
        raise ValueError("runtime-version为空")
    if len(version) > 128:
        raise ValueError("runtime-version超过128字符")
    if " " in version or "/" in version or "\\" in version or "\n" in version:
        raise ValueError("runtime-version包含非法字符")
    if version in ("latest", "stable", "dev", "unknown", "current"):
        raise ValueError(f"runtime-version不能是保留字: {version}")
    if not SEMVER_RE.match(version):
        raise ValueError(f"runtime-version格式非法: {version}")
    return True


def validate_commit(commit):
    if not commit:
        raise ValueError("commit为空")
    if len(commit) not in (40, 64):
        raise ValueError(f"commit长度非法: {len(commit)}")
    if not COMMIT_RE.match(commit.lower()):
        raise ValueError(f"commit格式非法: {commit}")
    if commit == "0" * len(commit):
        raise ValueError("commit不能为全零")
    return True


def validate_component_metadata(component_id, metadata, lock_sha=None):
    required = ["componentId", "version", "platform", "architecture"]
    for key in required:
        if key not in metadata:
            raise ValueError(f"组件元数据缺少字段 {key}: {component_id}")
    if metadata["componentId"] != component_id:
        raise ValueError(f"ComponentId不匹配: {metadata['componentId']} != {component_id}")
    if metadata["platform"] != "linux":
        raise ValueError(f"Platform错误: {metadata['platform']} for {component_id}")
    if metadata["architecture"] != "arm64":
        raise ValueError(f"Architecture错误: {metadata['architecture']} for {component_id}")
    if "sha256" in metadata and lock_sha:
        if metadata["sha256"] != lock_sha:
            raise ValueError(f"SHA校验失败: {component_id}")
    if component_id == "runtime.backend":
        if metadata.get("version") == "dev":
            pass
    if component_id == "runtime.node":
        if "nodeVersion" in metadata:
            ver = metadata.get("nodeVersion", "")
            if not ver.startswith("24.19"):
                raise ValueError(f"Node版本需要 24.19.x: {ver}")
    if component_id == "runtime.qdrant":
        ver = metadata.get("version", "")
        if not ver.startswith("1.19"):
            pass
    return True


def validate_lock(data, runtime_version, commit):
    if data.get("schemaVersion") != 1:
        raise ValueError("Lock schemaVersion错误")
    if data.get("componentId") != "runtime.package":
        raise ValueError("Lock componentId错误")
    target = data.get("target", {})
    required_target = {
        "hostPlatform": "android",
        "hostAbi": "arm64-v8a",
        "runtimeKind": "proot",
        "guestPlatform": "linux",
        "guestArchitecture": "arm64",
        "distribution": "ubuntu",
        "distributionRelease": "24.04.4",
    }
    for k, v in required_target.items():
        if target.get(k) != v:
            raise ValueError(f"Target字段不匹配 {k}: {target.get(k)} != {v}")
    components = data.get("components", {})
    required_components = ["rootfs", "guestLayout", "backend", "node", "nodeScripts",
                          "qdrant", "pluginHost", "taskHost"]
    for rc in required_components:
        if rc not in components:
            raise ValueError(f"Lock缺少组件: {rc}")
    for cid, comp in components.items():
        if "componentId" not in comp:
            raise ValueError(f"组件缺少componentId: {cid}")
        if comp.get("platform") and comp.get("platform") != "linux":
            raise ValueError(f"组件Platform错误: {cid}")
        if comp.get("architecture") and comp.get("architecture") != "arm64":
            raise ValueError(f"组件Architecture错误: {cid}")
    for cid in ["pluginHost", "taskHost"]:
        if commit:
            comp_commit = components[cid].get("commit", "")
            if comp_commit and comp_commit != commit:
                raise ValueError(f"组件Commit不一致 {cid}: {comp_commit} != {commit}")
    return True


def validate_inputs(lock_data, runtime_version, commit, artifacts):
    issues = []
    if not validate_runtime_version(runtime_version):
        issues.append("runtime-version非法")
    if not validate_commit(commit):
        issues.append("commit非法")
    for key in ["rootfs", "backend", "node", "nodeScripts", "qdrant", "guestLayout"]:
        comp = lock_data["components"].get(key, {})
        artifact_field = comp.get("artifact", "")
        sha_field = comp.get("sha256", "")
        if artifact_field and sha_field and sha_field != "PENDING_BUILD" and sha_field != "PENDING_COMPUTE":
            art_path = pathlib.Path(artifact_field)
            if not art_path.exists():
                issues.append(f"{key} Artifact不存在: {artifact_field}")
            else:
                actual = hashlib.sha256(art_path.read_bytes()).hexdigest()
                if actual != sha_field:
                    issues.append(f"{key} SHA不匹配: 实际={actual} 期望={sha_field}")
    return issues
