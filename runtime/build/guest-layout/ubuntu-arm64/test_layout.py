import json
import os
import pathlib
import unittest

SCRIPT_DIR = pathlib.Path(__file__).resolve().parent
LOCK_FILE = SCRIPT_DIR / "guest-layout.lock.json"

try:
    from verify import (
        validate_linux_path,
        validate_directories,
        validate_mount_contract,
        validate_lock_structure,
        validate_paths_section,
        AMITIA_UID,
        AMITIA_GID,
        ALLOWED_PERSISTENCE,
        ALLOWED_PURPOSE,
    )
    _HAS_VERIFY = True
except ImportError:
    _HAS_VERIFY = False


def load_lock():
    with open(LOCK_FILE, "r", encoding="utf-8") as f:
        return json.load(f)


@unittest.skipUnless(_HAS_VERIFY, "verify import failed")
class TestLockStructure(unittest.TestCase):
    def test_file_parses(self):
        data = load_lock()
        errors = validate_lock_structure(data)
        self.assertEqual(errors, [])

    def test_all_required_roots_present(self):
        data = load_lock()
        paths = {d["path"] for d in data["directories"]}
        required_roots = {
            "/opt/amitia", "/etc/amitia", "/var/lib/amitia",
            "/var/cache/amitia", "/var/log/amitia", "/run/amitia",
            "/var/lib/amitia/workspaces",
        }
        for r in required_roots:
            self.assertIn(r, paths, f"missing root: {r}")

    def test_component_paths(self):
        data = load_lock()
        comp = data["components"]
        self.assertEqual(comp["backend"]["binary"], "/opt/amitia/backend/amitia-server")
        self.assertEqual(comp["node"]["binary"], "/opt/amitia/node/bin/node")
        self.assertEqual(comp["node"]["npmCli"], "/opt/amitia/node/lib/node_modules/npm/bin/npm-cli.js")
        self.assertEqual(comp["node"]["npxCli"], "/opt/amitia/node/lib/node_modules/npm/bin/npx-cli.js")
        self.assertEqual(comp["qdrant"]["binary"], "/opt/amitia/qdrant/bin/qdrant")
        self.assertEqual(comp["pluginHost"]["entry"], "/opt/amitia/plugin-host/dist/index.js")
        self.assertEqual(comp["taskHost"]["entry"], "/opt/amitia/task-host/dist/index.js")

    def test_qdrant_paths(self):
        data = load_lock()
        q = data["components"]["qdrant"]
        self.assertEqual(q["config"], "/etc/amitia/providers/qdrant/config.yaml")
        self.assertEqual(q["storage"], "/var/lib/amitia/providers/qdrant/storage")
        self.assertEqual(q["snapshots"], "/var/lib/amitia/providers/qdrant/snapshots")
        self.assertEqual(q["migration"], "/var/lib/amitia/providers/qdrant/migration")

    def test_environment_present(self):
        data = load_lock()
        env = data.get("environment", {})
        self.assertEqual(env.get("AMITIA_RUNTIME_ROOT"), "/opt/amitia")
        self.assertEqual(env.get("HOME"), "/home/amitia")
        self.assertEqual(env.get("AMITIA_NODE_HOME"), "/var/lib/amitia/node/home")
        self.assertEqual(env.get("AMITIA_NPM_CACHE"), "/var/cache/amitia/node/npm")
        self.assertEqual(env.get("AMITIA_NODE_TMP"), "/run/amitia/tmp/node")


@unittest.skipUnless(_HAS_VERIFY, "verify import failed")
class TestPathValidation(unittest.TestCase):
    def test_valid_absolute(self):
        self.assertIsNone(validate_linux_path("/opt/amitia"))
        self.assertIsNone(validate_linux_path("/run/amitia/tmp"))

    def test_rejects_relative(self):
        self.assertIsNotNone(validate_linux_path("opt/amitia"))

    def test_rejects_backslash(self):
        self.assertIsNotNone(validate_linux_path("/opt\\amitia"))

    def test_rejects_traversal(self):
        self.assertIsNotNone(validate_linux_path("/opt/../etc"))
        self.assertIsNotNone(validate_linux_path("/foo/../bar"))

    def test_rejects_dot_segment(self):
        self.assertIsNotNone(validate_linux_path("/opt/./amitia"))

    def test_rejects_double_slash(self):
        self.assertIsNotNone(validate_linux_path("/opt//amitia"))

    def test_rejects_fs_root(self):
        self.assertIsNotNone(validate_linux_path("/"))

    def test_rejects_android_paths(self):
        for p in ("/data", "/sdcard", "/storage", "/storage/emulated", "/data/user", "/data/data"):
            self.assertIsNotNone(validate_linux_path(p), p)

    def test_rejects_forbidden_aliases(self):
        for p in ("/runtime", "/data", "/workspace", "/amitia"):
            self.assertIsNotNone(validate_linux_path(p), p)

    def test_rejects_trailing_slash(self):
        self.assertIsNotNone(validate_linux_path("/opt/"))


@unittest.skipUnless(_HAS_VERIFY, "verify import failed")
class TestDirectoryContract(unittest.TestCase):
    def test_directories_valid(self):
        data = load_lock()
        errors = validate_directories(data["directories"])
        self.assertEqual(errors, [])

    def test_immutable_dir_owned_by_root(self):
        data = load_lock()
        for d in data["directories"]:
            if d["persistence"] == "immutable":
                self.assertEqual(d["ownerUid"], 0)
                self.assertEqual(d["ownerGid"], 0)
                self.assertIn(d["mode"], ("0755",))

    def test_mutable_dir_owned_by_amitia(self):
        data = load_lock()
        mutable_persistences = {"persistent-critical", "persistent-diagnostic", "rebuildable", "ephemeral"}
        for d in data["directories"]:
            if d["persistence"] in mutable_persistences:
                self.assertEqual(d["ownerUid"], AMITIA_UID, d["path"])
                self.assertEqual(d["ownerGid"], AMITIA_GID, d["path"])

    def test_no_0777(self):
        data = load_lock()
        for d in data["directories"]:
            self.assertNotIn(d["mode"], ("0777", "0666"), d["path"])

    def test_no_unknown_persistence(self):
        data = load_lock()
        for d in data["directories"]:
            self.assertIn(d["persistence"], ALLOWED_PERSISTENCE, d["path"])

    def test_no_unknown_purpose(self):
        data = load_lock()
        for d in data["directories"]:
            self.assertIn(d["purpose"], ALLOWED_PURPOSE, d["path"])

    def test_no_forbidden_opt_subdirs(self):
        data = load_lock()
        forbidden = {"/opt/amitia/data", "/opt/amitia/config", "/opt/amitia/cache", "/opt/amitia/logs"}
        paths = {d["path"] for d in data["directories"]}
        for f in forbidden:
            self.assertNotIn(f, paths)

    def test_roots_non_overlapping(self):
        data = load_lock()
        roots = {"/opt/amitia", "/etc/amitia", "/var/lib/amitia", "/var/cache/amitia", "/var/log/amitia", "/run/amitia"}
        rlist = list(roots)
        for i in range(len(rlist)):
            for j in range(i + 1, len(rlist)):
                a, b = rlist[i], rlist[j]
                if a != b and not a.startswith(b + "/") and not b.startswith(a + "/"):
                    continue
                elif a != b:
                    self.fail(f"roots overlap: {a} {b}")

    def test_workspace_under_data_root(self):
        data = load_lock()
        ws = next((d for d in data["directories"] if d["purpose"] == "workspaces"), None)
        self.assertIsNotNone(ws)
        self.assertTrue(ws["path"].startswith("/var/lib/amitia/"))

    def test_temp_under_run_root(self):
        data = load_lock()
        tmp = next((d for d in data["directories"] if d["purpose"] == "temp"), None)
        self.assertIsNotNone(tmp)
        self.assertEqual(tmp["path"], "/run/amitia/tmp")
        self.assertTrue(tmp["path"].startswith("/run/amitia/"))


@unittest.skipUnless(_HAS_VERIFY, "verify import failed")
class TestMountContract(unittest.TestCase):
    def test_contract_valid(self):
        data = load_lock()
        errors = validate_mount_contract(data["mountContract"])
        self.assertEqual(errors, [])

    def test_all_required_mounts(self):
        data = load_lock()
        ids = [m["id"] for m in data["mountContract"]]
        for r in ("runtime", "config", "data", "cache", "logs", "run"):
            self.assertIn(r, ids)

    def test_runtime_first(self):
        data = load_lock()
        mounts = data["mountContract"]
        self.assertEqual(mounts[0]["id"], "runtime")

    def test_no_root_mount(self):
        data = load_lock()
        for m in data["mountContract"]:
            self.assertNotEqual(m["guestTarget"], "/")

    def test_no_android_mount(self):
        data = load_lock()
        for m in data["mountContract"]:
            self.assertFalse(m["guestTarget"].startswith(("/data", "/sdcard", "/storage")))

    def test_no_host_source(self):
        data = load_lock()
        for m in data["mountContract"]:
            self.assertNotIn("hostSource", m)
            self.assertNotIn("host_source", m)

    def test_id_unique(self):
        data = load_lock()
        ids = [m["id"] for m in data["mountContract"]]
        self.assertEqual(len(ids), len(set(ids)))

    def test_target_unique(self):
        data = load_lock()
        targets = [m["guestTarget"] for m in data["mountContract"]]
        self.assertEqual(len(targets), len(set(targets)))


class TestForbiddenRootsNotInLock(unittest.TestCase):
    def test_no_aliases_in_dirs(self):
        data = load_lock()
        paths = {d["path"] for d in data["directories"]}
        for alias in ("/runtime", "/data", "/workspace", "/amitia"):
            self.assertNotIn(alias, paths)


if __name__ == "__main__":
    unittest.main()
