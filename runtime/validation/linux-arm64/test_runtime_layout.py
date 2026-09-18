import os
import shutil
import tempfile
import unittest
import sys

sys.path.insert(0, os.path.dirname(os.path.abspath(__file__)))


class TestRuntimeLayout(unittest.TestCase):
    def setUp(self):
        self.tmpdir = tempfile.mkdtemp()

    def tearDown(self):
        shutil.rmtree(self.tmpdir, ignore_errors=True)

    def test_required_entries(self):
        from runtime_layout import RUNTIME_REQUIRED_ENTRIES
        self.assertIn("backend", RUNTIME_REQUIRED_ENTRIES)
        self.assertIn("node", RUNTIME_REQUIRED_ENTRIES)
        self.assertIn("qdrant", RUNTIME_REQUIRED_ENTRIES)
        self.assertIn("plugin-host", RUNTIME_REQUIRED_ENTRIES)
        self.assertIn("task-host", RUNTIME_REQUIRED_ENTRIES)
        self.assertIn("scripts", RUNTIME_REQUIRED_ENTRIES)
        self.assertIn("manifest", RUNTIME_REQUIRED_ENTRIES)
        self.assertIn("licenses", RUNTIME_REQUIRED_ENTRIES)

    def test_forbidden_paths(self):
        from runtime_layout import RUNTIME_FORBIDDEN_PATHS
        self.assertIn("config", RUNTIME_FORBIDDEN_PATHS)
        self.assertIn("data", RUNTIME_FORBIDDEN_PATHS)
        self.assertIn("log", RUNTIME_FORBIDDEN_PATHS)

    def test_guest_layout_defaults(self):
        from runtime_layout import GuestLayout
        layout = GuestLayout()
        self.assertEqual(layout.runtimeRoot, "/opt/amitia")
        self.assertEqual(layout.configRoot, "/etc/amitia")
        self.assertEqual(layout.dataRoot, "/var/lib/amitia")
        self.assertEqual(layout.cacheRoot, "/var/cache/amitia")
        self.assertEqual(layout.logRoot, "/var/log/amitia")
        self.assertEqual(layout.runRoot, "/run/amitia")
        self.assertEqual(layout.tempRoot, "/run/amitia/tmp")
        self.assertEqual(layout.workspaceRoot, "/var/lib/amitia/workspaces")

    def test_ensure_directory_structure(self):
        from runtime_layout import GuestLayout, ensure_directory_structure
        work_root = os.path.join(self.tmpdir, "runtime")
        runtime_root = os.path.join(work_root, "opt", "amitia")
        config_root = os.path.join(work_root, "etc", "amitia")
        data_root = os.path.join(work_root, "var", "lib", "amitia")
        cache_root = os.path.join(work_root, "var", "cache", "amitia")
        log_root = os.path.join(work_root, "var", "log", "amitia")
        run_root = os.path.join(work_root, "run", "amitia")
        temp_root = os.path.join(run_root, "tmp")
        workspace_root = os.path.join(data_root, "workspaces")

        layout = GuestLayout(
            runtimeRoot=runtime_root,
            configRoot=config_root,
            dataRoot=data_root,
            cacheRoot=cache_root,
            logRoot=log_root,
            runRoot=run_root,
            tempRoot=temp_root,
            workspaceRoot=workspace_root,
        )
        ensure_directory_structure(layout)
        self.assertTrue(os.path.isdir(runtime_root))
        self.assertTrue(os.path.isdir(config_root))
        self.assertTrue(os.path.isdir(data_root))
        self.assertTrue(os.path.isdir(cache_root))
        self.assertTrue(os.path.isdir(log_root))
        self.assertTrue(os.path.isdir(run_root))
        self.assertTrue(os.path.isdir(temp_root))
        self.assertTrue(os.path.isdir(workspace_root))


class TestTreeScan(unittest.TestCase):
    def setUp(self):
        self.tmpdir = tempfile.mkdtemp()

    def tearDown(self):
        shutil.rmtree(self.tmpdir, ignore_errors=True)

    def test_scan_empty_tree(self):
        from runtime_layout import scan_tree
        root = os.path.join(self.tmpdir, "empty")
        os.makedirs(root)
        manifest = scan_tree(root)
        self.assertEqual(manifest.root, root)
        self.assertEqual(len(manifest.entries), 0)

    def test_scan_tree_with_files(self):
        from runtime_layout import scan_tree
        root = os.path.join(self.tmpdir, "tree")
        os.makedirs(root)
        with open(os.path.join(root, "file1.txt"), "w") as f:
            f.write("content1")
        os.makedirs(os.path.join(root, "subdir"))
        with open(os.path.join(root, "subdir", "file2.txt"), "w") as f:
            f.write("content2")
        manifest = scan_tree(root)
        paths = {entry.path for entry in manifest.entries}
        self.assertIn("file1.txt", paths)
        self.assertIn("subdir", paths)
        self.assertIn(os.path.join("subdir", "file2.txt"), paths)

    def test_compare_tree_manifests(self):
        from runtime_layout import scan_tree, compare_tree_manifests
        root = os.path.join(self.tmpdir, "compare")
        os.makedirs(root)
        with open(os.path.join(root, "a.txt"), "w") as f:
            f.write("original")
        original = scan_tree(root)
        with open(os.path.join(root, "b.txt"), "w") as f:
            f.write("new")
        current = scan_tree(root)
        diff = compare_tree_manifests(original, current)
        self.assertTrue(diff["changed"])
        self.assertIn("b.txt", diff["added"])

    def test_compare_tree_unchanged(self):
        from runtime_layout import scan_tree, compare_tree_manifests
        root = os.path.join(self.tmpdir, "unchanged")
        os.makedirs(root)
        with open(os.path.join(root, "a.txt"), "w") as f:
            f.write("same")
        manifest1 = scan_tree(root)
        manifest2 = scan_tree(root)
        diff = compare_tree_manifests(manifest1, manifest2)
        self.assertFalse(diff["changed"])
        self.assertEqual(len(diff["added"]), 0)
        self.assertEqual(len(diff["removed"]), 0)
        self.assertEqual(len(diff["modified"]), 0)


class TestPathClassification(unittest.TestCase):
    def test_classify_data_root(self):
        from runtime_layout import GuestLayout, classify_path_against_layout, DirectoryRole
        layout = GuestLayout()
        role = classify_path_against_layout(layout.dataRoot + "/test", layout)
        self.assertEqual(role, DirectoryRole.DATA)

    def test_classify_program_root(self):
        from runtime_layout import GuestLayout, classify_path_against_layout, DirectoryRole
        layout = GuestLayout()
        role = classify_path_against_layout(layout.runtimeRoot + "/backend/amitia-server", layout)
        self.assertEqual(role, DirectoryRole.PROGRAM)

    def test_classify_config_root(self):
        from runtime_layout import GuestLayout, classify_path_against_layout, DirectoryRole
        layout = GuestLayout()
        role = classify_path_against_layout(layout.configRoot + "/config.yaml", layout)
        self.assertEqual(role, DirectoryRole.CONFIG)

    def test_classify_unknown(self):
        from runtime_layout import GuestLayout, classify_path_against_layout
        layout = GuestLayout()
        role = classify_path_against_layout("/some/random/path", layout)
        self.assertIsNone(role)


if __name__ == "__main__":
    unittest.main()
