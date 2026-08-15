import json
import os
import tempfile
import unittest

from .program_tree_validator import validate_program_tree, load_program_contract


class TestProgramTreeValidator(unittest.TestCase):
    def setUp(self):
        self.tmpdir = tempfile.mkdtemp()

    def tearDown(self):
        import shutil
        shutil.rmtree(self.tmpdir, ignore_errors=True)

    def _create_entry(self, rel_path: str, is_dir: bool = False):
        full = os.path.join(self.tmpdir, rel_path)
        if is_dir:
            os.makedirs(full, exist_ok=True)
        else:
            os.makedirs(os.path.dirname(full), exist_ok=True)
            with open(full, "w") as f:
                f.write("x")

    def test_load_contract(self):
        contract = load_program_contract()
        self.assertIsInstance(contract, dict)
        self.assertIn("requiredProgramPaths", contract)
        self.assertIn("programSubdirs", contract)

    def test_valid_program_tree(self):
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
        subdirs = [
            "backend", "node", "qdrant", "sidecar", "qq-sidecar",
            "plugin-host", "task-host", "scripts", "manifest", "licenses",
        ]
        for r in required:
            self._create_entry(r)
        for s in subdirs:
            self._create_entry(s + "/", is_dir=True)
        os.makedirs(os.path.join(self.tmpdir, "sidecar"), exist_ok=True)
        os.makedirs(os.path.join(self.tmpdir, "qq-sidecar"), exist_ok=True)
        os.makedirs(os.path.join(self.tmpdir, "licenses"), exist_ok=True)

        result = validate_program_tree(self.tmpdir)
        self.assertTrue(result.valid, f"Errors: {result.errors}")

    def test_missing_required_path(self):
        subdirs = ["backend", "node"]
        for s in subdirs:
            os.makedirs(os.path.join(self.tmpdir, s), exist_ok=True)
        self._create_entry("backend/amitia-server")

        result = validate_program_tree(self.tmpdir)
        self.assertFalse(result.valid)
        self.assertTrue(len(result.missing_required) > 0 or len(result.missing_subdirs) > 0)

    def test_missing_subdirs(self):
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
        for r in required:
            self._create_entry(r)
        os.makedirs(os.path.join(self.tmpdir, "backend"), exist_ok=True)
        os.makedirs(os.path.join(self.tmpdir, "node"), exist_ok=True)

        result = validate_program_tree(self.tmpdir)
        self.assertFalse(result.valid)
        self.assertTrue(len(result.missing_subdirs) > 0)

    def test_nonexistent_root(self):
        result = validate_program_tree("/nonexistent/path")
        self.assertFalse(result.valid)
        self.assertTrue(len(result.errors) > 0)


if __name__ == "__main__":
    unittest.main()
