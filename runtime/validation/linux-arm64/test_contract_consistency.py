import json
import os
import posixpath
import sys
import unittest


BASE_DIR = os.path.dirname(os.path.dirname(os.path.dirname(os.path.dirname(os.path.abspath(__file__)))))
ARTIFACT_RECORD_DIR = os.path.join(BASE_DIR, "runtime", "build", "common")


def load_json(relative_path):
    full_path = os.path.join(BASE_DIR, relative_path)
    with open(full_path, "r", encoding="utf-8") as f:
        return json.load(f)


class TestMountContractReadOnly(unittest.TestCase):
    def test_no_writable_field(self):
        contract = load_json("runtime/contracts/mount-contract.json")
        for bind in contract["binds"]:
            self.assertNotIn("writable", bind)

    def test_readonly_field_present(self):
        contract = load_json("runtime/contracts/mount-contract.json")
        for bind in contract["binds"]:
            self.assertIn("readOnly", bind)
            self.assertIsInstance(bind["readOnly"], bool)

    def test_program_root_readonly(self):
        contract = load_json("runtime/contracts/mount-contract.json")
        program_bind = next(b for b in contract["binds"] if b["target"] == "/opt/amitia")
        self.assertTrue(program_bind["readOnly"])

    def test_data_writable(self):
        contract = load_json("runtime/contracts/mount-contract.json")
        data_bind = next(b for b in contract["binds"] if b["target"] == "/var/lib/amitia")
        self.assertFalse(data_bind["readOnly"])

    def test_cache_writable(self):
        contract = load_json("runtime/contracts/mount-contract.json")
        cache_bind = next(b for b in contract["binds"] if b["target"] == "/var/cache/amitia")
        self.assertFalse(cache_bind["readOnly"])

    def test_logs_writable(self):
        contract = load_json("runtime/contracts/mount-contract.json")
        logs_bind = next(b for b in contract["binds"] if b["target"] == "/var/log/amitia")
        self.assertFalse(logs_bind["readOnly"])

    def test_run_writable(self):
        contract = load_json("runtime/contracts/mount-contract.json")
        run_bind = next(b for b in contract["binds"] if b["target"] == "/run/amitia")
        self.assertFalse(run_bind["readOnly"])


class TestGuestLayoutMountConsistency(unittest.TestCase):
    def test_mount_targets_in_guest_directories(self):
        guest_layout = load_json("runtime/contracts/guest-layout.json")
        mount_contract = load_json("runtime/contracts/mount-contract.json")
        guest_dirs = set(guest_layout["directories"])
        for bind in mount_contract["binds"]:
            self.assertIn(bind["target"], guest_dirs,
                          f"Mount target {bind['target']} not in guest-layout directories")

    def test_guest_dirs_in_mount_targets(self):
        guest_layout = load_json("runtime/contracts/guest-layout.json")
        mount_contract = load_json("runtime/contracts/mount-contract.json")
        mount_targets = {b["target"] for b in mount_contract["binds"]}
        for d in guest_layout["directories"]:
            self.assertTrue(
                d in mount_targets or d == "/tmp",
                f"Guest directory {d} has no corresponding mount bind"
            )

    def test_program_subdirs_cover_required_entries(self):
        guest_layout = load_json("runtime/contracts/guest-layout.json")
        structure = guest_layout["structure"]
        self.assertIn("backend", structure)
        self.assertIn("node", structure)
        self.assertIn("qdrant", structure)
        self.assertIn("pluginHost", structure)
        self.assertIn("taskHost", structure)
        self.assertIn("scripts", structure)
        self.assertIn("manifest", structure)

    def test_backend_has_amitia_server(self):
        guest_layout = load_json("runtime/contracts/guest-layout.json")
        backend = guest_layout["structure"]["backend"]
        self.assertIn("amitia-server", backend["files"])

    def test_node_has_bin_node(self):
        guest_layout = load_json("runtime/contracts/guest-layout.json")
        node = guest_layout["structure"]["node"]
        self.assertIn("bin/node", node["bin"])

    def test_qdrant_has_bin_qdrant(self):
        guest_layout = load_json("runtime/contracts/guest-layout.json")
        qdrant = guest_layout["structure"]["qdrant"]
        self.assertIn("bin/qdrant", qdrant["bin"])

    def test_plugin_host_entry(self):
        guest_layout = load_json("runtime/contracts/guest-layout.json")
        plugin_host = guest_layout["structure"]["pluginHost"]
        self.assertEqual(plugin_host["entry"], "dist/index.js")

    def test_task_host_entry(self):
        guest_layout = load_json("runtime/contracts/guest-layout.json")
        task_host = guest_layout["structure"]["taskHost"]
        self.assertEqual(task_host["entry"], "dist/index.js")

    def test_manifest_files(self):
        guest_layout = load_json("runtime/contracts/guest-layout.json")
        manifest = guest_layout["structure"]["manifest"]
        self.assertIn("guest-layout.json", manifest["files"])
        self.assertIn("mount-contract.json", manifest["files"])

    def test_node_npm_cli_path(self):
        guest_layout = load_json("runtime/contracts/guest-layout.json")
        node = guest_layout["structure"]["node"]
        self.assertEqual(node["npmCli"], "lib/node_modules/npm/bin/npm-cli.js")

    def test_node_npx_cli_path(self):
        guest_layout = load_json("runtime/contracts/guest-layout.json")
        node = guest_layout["structure"]["node"]
        self.assertEqual(node["npxCli"], "lib/node_modules/npm/bin/npx-cli.js")


class TestRuntimeProgramContract(unittest.TestCase):
    def test_required_entries_complete(self):
        contract = load_json("runtime/contracts/runtime-program-contract.json")
        required = [
            "backend/amitia-server",
            "node/bin/node",
            "node/lib/node_modules/npm/bin/npm-cli.js",
            "node/lib/node_modules/npm/bin/npx-cli.js",
            "qdrant/bin/qdrant",
            "plugin-host/dist/index.js",
            "task-host/dist/index.js",
            "scripts/node/amitia-node-prepare.sh",
            "scripts/node/amitia-node-probe.sh",
            "manifest/guest-layout.json",
            "manifest/mount-contract.json",
        ]
        for entry in required:
            self.assertIn(entry, contract["requiredProgramPaths"],
                          f"Missing required entry: {entry}")

    def test_required_paths_exist_in_guest_layout(self):
        program = load_json("runtime/contracts/runtime-program-contract.json")
        guest_layout = load_json("runtime/contracts/guest-layout.json")
        structure = guest_layout["structure"]
        root = guest_layout["root"]
        resolved_required_paths = set()
        backend_path = structure["backend"]["path"]
        resolved_required_paths.add(posixpath.join(backend_path, "amitia-server"))
        node_path = structure["node"]["path"]
        resolved_required_paths.add(posixpath.join(node_path, "bin", "node"))
        resolved_required_paths.add(posixpath.join(node_path, "lib", "node_modules", "npm", "bin", "npm-cli.js"))
        resolved_required_paths.add(posixpath.join(node_path, "lib", "node_modules", "npm", "bin", "npx-cli.js"))
        qdrant_path = structure["qdrant"]["path"]
        resolved_required_paths.add(posixpath.join(qdrant_path, "bin", "qdrant"))
        plugin_host_path = structure["pluginHost"]["path"]
        resolved_required_paths.add(posixpath.join(plugin_host_path, "dist", "index.js"))
        task_host_path = structure["taskHost"]["path"]
        resolved_required_paths.add(posixpath.join(task_host_path, "dist", "index.js"))
        scripts_path = structure["scripts"]["path"]
        resolved_required_paths.add(posixpath.join(scripts_path, "node", "amitia-node-prepare.sh"))
        resolved_required_paths.add(posixpath.join(scripts_path, "node", "amitia-node-probe.sh"))
        manifest_path = structure["manifest"]["path"]
        resolved_required_paths.add(posixpath.join(manifest_path, "guest-layout.json"))
        resolved_required_paths.add(posixpath.join(manifest_path, "mount-contract.json"))
        for resolved in resolved_required_parts(program["requiredProgramPaths"], root, structure):
            self.assertIn(resolved, resolved_required_paths,
                          f"Required path {resolved} not found in guest-layout structure")

    def test_program_subdirs_match_required_top_dirs(self):
        contract = load_json("runtime/contracts/runtime-program-contract.json")
        expected_dirs = {"backend", "node", "qdrant", "plugin-host", "task-host", "scripts", "manifest"}
        actual_dirs = set(contract["programSubdirs"])
        for d in expected_dirs:
            self.assertIn(d, actual_dirs, f"Missing program subdirectory: {d}")


def resolved_required_parts(required_paths, root, structure):
    result = set()
    subdir_to_path = {}
    for key in structure:
        entry = structure[key]
        subdirs = entry.get("subdirs", [])
        for sub in subdirs:
            subdir_to_path[sub] = posixpath.join(entry["path"], sub)
    for path in required_paths:
        parts = path.split("/")
        top_dir = parts[0]
        subpath = "/".join(parts[1:])
        if top_dir in subdir_to_path:
            base = subdir_to_path[top_dir]
            result.add(posixpath.join(base, subpath))
    return result


class TestPackageIndexSchema(unittest.TestCase):
    def test_schema_is_valid_json_schema(self):
        schema = load_json("runtime/contracts/package-index.schema.json")
        self.assertEqual(schema["$schema"], "http://json-schema.org/draft-07/schema#")
        self.assertEqual(schema["type"], "object")

    def test_target_host_platform(self):
        schema = load_json("runtime/contracts/package-index.schema.json")
        target = schema["properties"]["target"]
        self.assertIn("hostPlatform", target["properties"])
        self.assertEqual(target["properties"]["hostPlatform"]["enum"], ["android"])

    def test_target_host_abi(self):
        schema = load_json("runtime/contracts/package-index.schema.json")
        target = schema["properties"]["target"]
        self.assertIn("hostAbi", target["properties"])
        self.assertEqual(target["properties"]["hostAbi"]["enum"], ["arm64-v8a"])

    def test_target_runtime_kind(self):
        schema = load_json("runtime/contracts/package-index.schema.json")
        target = schema["properties"]["target"]
        self.assertIn("runtimeKind", target["properties"])
        self.assertEqual(target["properties"]["runtimeKind"]["enum"], ["embedded-proot"])

    def test_target_guest_platform(self):
        schema = load_json("runtime/contracts/package-index.schema.json")
        target = schema["properties"]["target"]
        self.assertIn("guestPlatform", target["properties"])
        self.assertEqual(target["properties"]["guestPlatform"]["enum"], ["linux"])

    def test_target_guest_architecture(self):
        schema = load_json("runtime/contracts/package-index.schema.json")
        target = schema["properties"]["target"]
        self.assertIn("guestArchitecture", target["properties"])
        self.assertEqual(target["properties"]["guestArchitecture"]["enum"], ["arm64"])

    def test_payloads_roles(self):
        schema = load_json("runtime/contracts/package-index.schema.json")
        payloads = schema["properties"]["payloads"]
        self.assertEqual(payloads["items"]["properties"]["role"]["enum"], ["rootfs", "runtime"])

    def test_metadata_roles(self):
        schema = load_json("runtime/contracts/package-index.schema.json")
        metadata = schema["properties"]["metadata"]
        self.assertEqual(metadata["items"]["properties"]["role"]["enum"], ["guest-layout", "mount-contract", "sha256sums"])

    def test_sha256_pattern(self):
        schema = load_json("runtime/contracts/package-index.schema.json")
        sha256_pattern = "^[a-fA-F0-9]{64}$"
        self.assertEqual(schema["properties"]["payloads"]["items"]["properties"]["sha256"]["pattern"], sha256_pattern)
        self.assertEqual(schema["properties"]["metadata"]["items"]["properties"]["sha256"]["pattern"], sha256_pattern)


class TestPackageIndexMountSemanticConsistency(unittest.TestCase):
    def test_guest_platform_matches_mount_linux(self):
        schema = load_json("runtime/contracts/package-index.schema.json")
        mount_contract = load_json("runtime/contracts/mount-contract.json")
        guest_platform = schema["properties"]["target"]["properties"]["guestPlatform"]["enum"][0]
        self.assertEqual(guest_platform, "linux")

    def test_guest_arch_matches_frozen_record_arm64(self):
        schema = load_json("runtime/contracts/package-index.schema.json")
        guest_arch = schema["properties"]["target"]["properties"]["guestArchitecture"]["enum"][0]
        artifact_schema = load_json("runtime/contracts/frozen-artifact-record.schema.json")
        arch_enum = artifact_schema["properties"]["architecture"]["enum"]
        self.assertIn(guest_arch, arch_enum)

    def test_host_abi_matches_frozen_record_arm64_v8a(self):
        schema = load_json("runtime/contracts/package-index.schema.json")
        host_abi = schema["properties"]["target"]["properties"]["hostAbi"]["enum"][0]
        artifact_schema = load_json("runtime/contracts/frozen-artifact-record.schema.json")
        arch_enum = artifact_schema["properties"]["architecture"]["enum"]
        self.assertIn(host_abi, arch_enum)

    def test_mount_targets_align_with_guest_layout(self):
        mount_contract = load_json("runtime/contracts/mount-contract.json")
        guest_layout = load_json("runtime/contracts/guest-layout.json")
        mount_targets = {b["target"] for b in mount_contract["binds"]}
        guest_dirs = set(guest_layout["directories"])
        for target in mount_targets:
            self.assertIn(target, guest_dirs,
                          f"Mount target {target} missing from guest-layout directories")


class TestArtifactRecordPackageIndexConsistency(unittest.TestCase):
    def test_artifact_platform_values_subset_of_package_index(self):
        artifact_schema = load_json("runtime/contracts/frozen-artifact-record.schema.json")
        package_schema = load_json("runtime/contracts/package-index.schema.json")
        artifact_platforms = set(artifact_schema["properties"]["platform"]["enum"])
        package_guest_platforms = set(package_schema["properties"]["target"]["properties"]["guestPlatform"]["enum"])
        package_host_platforms = set(package_schema["properties"]["target"]["properties"]["hostPlatform"]["enum"])
        combined = package_guest_platforms | package_host_platforms
        for platform in artifact_platforms:
            self.assertIn(platform, combined,
                          f"Artifact platform '{platform}' not in package-index target platforms")

    def test_package_index_architecture_values_subset_of_artifact_record(self):
        artifact_schema = load_json("runtime/contracts/frozen-artifact-record.schema.json")
        package_schema = load_json("runtime/contracts/package-index.schema.json")
        artifact_archs = set(artifact_schema["properties"]["architecture"]["enum"])
        package_archs = set()
        package_archs.add(package_schema["properties"]["target"]["properties"]["guestArchitecture"]["enum"][0])
        package_archs.add(package_schema["properties"]["target"]["properties"]["hostAbi"]["enum"][0])
        for arch in package_archs:
            self.assertIn(arch, artifact_archs,
                          f"Package-index architecture '{arch}' not in artifact-record architectures")

    def test_artifact_record_validate_accepts_valid_record(self):
        sys.path.insert(0, ARTIFACT_RECORD_DIR)
        from artifact_record import FrozenArtifactRecord, validate
        record = FrozenArtifactRecord(
            componentId="test-component",
            version="1.0.0",
            platform="linux",
            architecture="arm64",
            artifactType="executable",
            artifactRelativePath="bin/test",
            artifactSha256="a" * 64,
            treeSha256="b" * 64,
            sourceRevision="abc1234",
            buildMode="release"
        )
        errors = validate(record)
        self.assertEqual(errors, [])

    def test_artifact_record_validate_rejects_invalid_platform(self):
        sys.path.insert(0, ARTIFACT_RECORD_DIR)
        from artifact_record import FrozenArtifactRecord, validate
        record = FrozenArtifactRecord(
            componentId="test-component",
            version="1.0.0",
            platform="invalid",
            architecture="arm64",
            artifactType="executable",
            artifactRelativePath="bin/test",
            artifactSha256="a" * 64,
            treeSha256="b" * 64,
            sourceRevision="abc1234",
            buildMode="release"
        )
        errors = validate(record)
        self.assertTrue(any("platform" in e for e in errors))

    def test_artifact_record_validate_rejects_invalid_sha256(self):
        sys.path.insert(0, ARTIFACT_RECORD_DIR)
        from artifact_record import FrozenArtifactRecord, validate
        record = FrozenArtifactRecord(
            componentId="test-component",
            version="1.0.0",
            platform="linux",
            architecture="arm64",
            artifactType="executable",
            artifactRelativePath="bin/test",
            artifactSha256="invalid",
            treeSha256="b" * 64,
            sourceRevision="abc1234",
            buildMode="release"
        )
        errors = validate(record)
        self.assertTrue(any("artifactSha256" in e for e in errors))


if __name__ == "__main__":
    unittest.main()
