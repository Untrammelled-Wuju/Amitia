import json
import os
import pathlib
import sys
import unittest

sys.path.insert(0, os.path.dirname(os.path.abspath(__file__)))


class TestLockStructure(unittest.TestCase):
    def setUp(self):
        self.lock_path = pathlib.Path(__file__).resolve().parent / "runtime-package.lock.json"
        self.lock = json.loads(self.lock_path.read_text(encoding="utf-8"))

    def test_schema_version(self):
        self.assertEqual(self.lock["schemaVersion"], 1)

    def test_package_id(self):
        self.assertEqual(self.lock["componentId"], "runtime.package")

    def test_host_platform(self):
        target = self.lock["target"]
        self.assertEqual(target["hostPlatform"], "android")
        self.assertEqual(target["hostAbi"], "arm64-v8a")
        self.assertEqual(target["guestArchitecture"], "arm64")
        self.assertEqual(target["distribution"], "ubuntu")

    def test_all_components_present(self):
        components = self.lock["components"]
        required = ["rootfs", "guestLayout", "backend", "node", "nodeScripts",
                    "qdrant", "pluginHost", "taskHost"]
        for r in required:
            self.assertIn(r, components, f"Lock缺少组件: {r}")

    def test_all_components_linux_arm64(self):
        for key in ["rootfs", "backend", "node", "nodeScripts", "qdrant", "guestLayout"]:
            comp = self.lock["components"][key]
            self.assertEqual(comp["platform"], "linux", f"{key} platform错误")
            self.assertEqual(comp["architecture"], "arm64", f"{key} arch错误")

    def test_target_consistency(self):
        target = self.lock["target"]
        self.assertEqual(target["runtimeKind"], "proot")
        self.assertEqual(target["guestPlatform"], "linux")
        self.assertEqual(target["distributionRelease"], "24.04.4")

    def test_locked_hashes_available(self):
        for key in ["guestLayout", "backend", "node", "nodeScripts"]:
            sha = self.lock["components"][key]["sha256"]
            self.assertNotIn("PENDING", sha, f"{key} SHA未锁定")

    def test_plugin_host_has_tree(self):
        ph = self.lock["components"]["pluginHost"]
        self.assertIn("treeSha256", ph)
        self.assertEqual(len(ph["treeSha256"]), 64)

    def test_task_host_has_tree(self):
        th = self.lock["components"]["taskHost"]
        self.assertIn("treeSha256", th)
        self.assertEqual(len(th["treeSha256"]), 64)


if __name__ == "__main__":
    unittest.main()
