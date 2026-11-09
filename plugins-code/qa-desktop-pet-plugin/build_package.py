import hashlib
import json
import shutil
import zipfile
import base64
from pathlib import Path


ROOT = Path(__file__).resolve().parent
PROJECT_ROOT = ROOT.parents[1]
STAGE = ROOT / ".build"
OUTPUT = PROJECT_ROOT / "plugin" / "qa" / "qa-desktop-pet-plugin-1.0.0.petx"
PACKAGE_VERSION = "1.0.0"


def digest_bytes(value):
    return hashlib.sha256(value).hexdigest()


def canonical_json(value):
    if isinstance(value, dict):
        return [
            {"k": key, "v": canonical_json(value[key])}
            for key in sorted(value)
        ]
    if isinstance(value, list):
        return [canonical_json(item) for item in value]
    return value


def pet_content_root_hash(manifest):
    clone = json.loads(json.dumps(manifest))
    clone["integrity"]["manifestHash"] = ""
    clone["integrity"]["contentRootHash"] = ""
    data = json.dumps(
        canonical_json(clone),
        ensure_ascii=False,
        separators=(",", ":"),
    ).encode("utf-8")
    manifest_hash = digest_bytes(data)
    entries = [
        (
            item["path"],
            bytes.fromhex(item["sha256"]),
            item["bytes"],
        )
        for item in clone["integrity"]["files"]
    ]
    entries.append(("@manifest", bytes.fromhex(manifest_hash), len(data)))
    tree = hashlib.sha256()
    for path, file_hash, size in sorted(entries, key=lambda item: item[0]):
        tree.update(b"file\0")
        tree.update(path.encode("utf-8"))
        tree.update(b"\0")
        tree.update(str(size).encode("ascii"))
        tree.update(b"\0")
        tree.update(file_hash)
        tree.update(b"\0")
    return manifest_hash, tree.hexdigest()


def collect_files():
    return sorted(
        (
            path
            for path in STAGE.rglob("*")
            if path.is_file()
            and not path.relative_to(STAGE).as_posix().startswith("integrity/")
            and not path.relative_to(STAGE).as_posix().startswith("signatures/")
        ),
        key=lambda path: path.relative_to(STAGE).as_posix(),
    )


def main():
    if STAGE.exists():
        shutil.rmtree(STAGE)
    STAGE.mkdir(parents=True)
    manifest = {
        "manifestVersion": 1,
        "extension": {
            "id": "com.amitia.qa/desktop-pet-plugin",
            "name": {"default": "QA Desktop Pet Plugin"},
            "description": {"default": "Amitia desktop pet packaging QA fixture"},
            "version": PACKAGE_VERSION,
            "license": "MIT",
            "categories": ["desktop-pet", "qa"],
        },
        "publisher": {
            "id": "com.amitia.qa",
            "displayName": "Amitia QA",
            "trustLevel": "trusted",
        },
        "compatibility": {
            "minHostVersion": "0.1.0",
            "platforms": ["windows", "linux", "macos", "android", "ios"],
        },
        "modules": [
            {
                "id": "qa-pet-runtime",
                "name": {"default": "QA Desktop Pet Runtime"},
                "type": "javascript",
                "runtime": {
                    "type": "javascript",
                    "entryPoint": "dist/index.js",
                },
                "contributions": [
                    {
                        "id": "qa-pet-plugin",
                        "kind": "petx",
                        "name": {"default": "QA Desktop Pet Plugin"},
                        "description": {"default": "QA desktop pet extension package"},
                        "spec": {
                            "contributionKind": "pet_resource",
                            "displayName": "QA Desktop Pet Resource",
                            "description": "Complete desktop pet resource bundle",
                            "assetKind": "manifest",
                            "resourceRef": "assets/pet/manifest.json",
                        },
                    }
                ],
            }
        ],
        "integrity": {"algorithm": "sha256", "contentTreeHash": ""},
    }
    (STAGE / "manifest.json").write_text(
        json.dumps(manifest, ensure_ascii=False, indent=2, sort_keys=True) + "\n",
        encoding="utf-8",
    )
    module_dir = STAGE / "modules" / "qa-pet-runtime" / "dist"
    module_dir.mkdir(parents=True)
    (module_dir / "index.js").write_text(
        (
            "defineExtension({\n"
            "  async activate(context) {\n"
            "    context.log.info('QA desktop pet plugin activated');\n"
            "  },\n"
            "  async deactivate() {}\n"
            "});\n"
        ),
        encoding="utf-8",
    )
    asset_root = STAGE / "assets" / "pet"
    action_root = asset_root / "actions" / "idle"
    frames_root = action_root / "frames"
    frames_root.mkdir(parents=True)
    frame = base64.b64decode(
        "iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAQAAAC1HAwCAAAAC0lEQVR42mNk"
        "+A8AAQUBAScY42YAAAAASUVORK5CYII="
    )
    (frames_root / "0.png").write_bytes(frame)
    (asset_root / "preview.png").write_bytes(frame)
    frame_hash = digest_bytes(frame)
    action = {
        "actionKey": "idle",
        "anchor": {"coordinateSpace": "normalized_canvas", "x": 0.5, "y": 1.0},
        "cooldownMs": 0,
        "displayName": "Idle",
        "fps": 8,
        "frames": [
            {
                "assetId": "asset_idle_0",
                "contentHash": frame_hash,
                "durationMs": 125,
                "file": "frames/0.png",
                "frameId": "idle_0",
                "index": 0,
            }
        ],
        "interruptible": True,
        "isStableStateCandidate": True,
        "isTransitionOnly": False,
        "maximumPlayMs": None,
        "minimumPlayMs": 0,
        "mutexGroup": None,
        "playbackMode": "loop",
        "priority": 50,
        "returnTo": {"type": "none"},
        "schemaVersion": 1,
        "supportsDefaultIdle": True,
        "version": 1,
    }
    action_data = (
        json.dumps(action, ensure_ascii=False, separators=(",", ":"), sort_keys=True)
        + "\n"
    ).encode("utf-8")
    (action_root / "action.json").write_bytes(action_data)
    files = [
        {
            "bytes": len(action_data),
            "mediaType": "application/json",
            "path": "actions/idle/action.json",
            "role": "action-config",
            "sha256": digest_bytes(action_data),
        },
        {
            "actionKey": "idle",
            "bytes": len(frame),
            "frameId": "idle_0",
            "mediaType": "image/png",
            "path": "actions/idle/frames/0.png",
            "role": "frame",
            "sha256": frame_hash,
        },
        {
            "bytes": len(frame),
            "mediaType": "image/png",
            "path": "preview.png",
            "role": "preview",
            "sha256": frame_hash,
        },
    ]
    pet_manifest = {
        "actions": [
            {
                "config": "actions/idle/action.json",
                "fps": 8,
                "frameCount": 1,
                "isStableStateCandidate": True,
                "isTransitionOnly": False,
                "key": "idle",
                "name": "Idle",
                "playbackMode": "loop",
                "qualityEvaluationId": "qa-eval-v1",
                "qualityVerdict": "accepted",
                "revisionId": "qa-idle-v1",
                "supportsDefaultIdle": True,
            }
        ],
        "author": {"id": "com.amitia.qa", "name": "Amitia QA"},
        "binding": {"policy": "unbound"},
        "canvas": {"coordinateSystem": "top-left", "height": 1, "width": 1},
        "capabilities": {
            "audio": False,
            "frameSequence": True,
            "perFrameDuration": True,
            "transparentBackground": True,
        },
        "compatibility": {
            "minRuntimeVersion": "0.0.0",
            "renderMode": "sprite",
        },
        "defaultAction": "idle",
        "description": "Complete Amitia desktop pet QA plugin",
        "integrity": {
            "algorithm": "amitia-package-sha256-v1",
            "contentRootHash": "",
            "fileCount": len(files),
            "files": files,
            "manifestHash": "",
            "totalBytes": sum(item["bytes"] for item in files),
        },
        "license": {"noticePath": "", "spdx": "MIT"},
        "manifestFormat": "amitia-desktop-pet",
        "name": "QA Desktop Pet",
        "petId": "qa-pet-v1",
        "preview": "preview.png",
        "provenance": {
            "builder": "amitia-qa",
            "builtAt": "2026-11-09T00:00:00Z",
            "generationTaskId": "",
            "processingTaskId": "",
            "sourceType": "qa",
        },
        "releaseId": "qa-pet-v1-release-1",
        "schemaVersion": 1,
        "version": "1.0.0",
    }
    manifest_hash, content_root_hash = pet_content_root_hash(pet_manifest)
    pet_manifest["integrity"]["manifestHash"] = manifest_hash
    pet_manifest["integrity"]["contentRootHash"] = content_root_hash
    (asset_root / "manifest.json").write_text(
        json.dumps(pet_manifest, ensure_ascii=False, indent=2, sort_keys=True) + "\n",
        encoding="utf-8",
    )

    entries = []
    for path in collect_files():
        data = path.read_bytes()
        entries.append(
            {
                "path": path.relative_to(STAGE).as_posix(),
                "size": len(data),
                "hash": digest_bytes(data),
                "modified": "2026-01-01T00:00:00Z",
            }
        )

    integrity = STAGE / "integrity"
    integrity.mkdir()
    files_doc = {
        "algorithm": "sha256",
        "files": {
            entry["path"]: entry
            for entry in entries
        },
        "generatedAt": "2026-01-01T00:00:00Z",
    }
    (integrity / "files.json").write_text(
        json.dumps(files_doc, ensure_ascii=False, indent=2, sort_keys=True) + "\n",
        encoding="utf-8",
    )
    tree = hashlib.sha256()
    for entry in entries:
        tree.update(entry["path"].encode("utf-8"))
        tree.update(b"\0")
        tree.update(entry["hash"].encode("ascii"))
        tree.update(b"\0")
    tree_doc = {
        "algorithm": "sha256",
        "treeHash": tree.hexdigest(),
        "generatedAt": "2026-01-01T00:00:00Z",
    }
    (integrity / "content-tree.json").write_text(
        json.dumps(tree_doc, ensure_ascii=False, indent=2, sort_keys=True) + "\n",
        encoding="utf-8",
    )

    OUTPUT.parent.mkdir(parents=True, exist_ok=True)
    with zipfile.ZipFile(OUTPUT, "w", compression=zipfile.ZIP_DEFLATED) as archive:
        for path in sorted(STAGE.rglob("*"), key=lambda item: item.as_posix()):
            if path.is_file():
                archive.write(path, path.relative_to(STAGE).as_posix())
    print(json.dumps({"output": str(OUTPUT), "sha256": digest_bytes(OUTPUT.read_bytes())}))


if __name__ == "__main__":
    main()
