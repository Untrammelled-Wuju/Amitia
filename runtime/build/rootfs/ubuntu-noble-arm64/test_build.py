import json
import os
import pathlib
import sys
import tempfile
import unittest

SCRIPT_DIR = pathlib.Path(__file__).resolve().parent
sys.path.insert(0, str(SCRIPT_DIR))

from filesystem import load_lock, safe_extract_archive, apply_overlay, cleanup_cache, create_mount_points


class TestRootFsLock(unittest.TestCase):
    def test_valid_lock(self):
        lock = load_lock()
        self.assertEqual(lock["distribution"], "ubuntu")
        self.assertEqual(lock["release"], "24.04.4")
        self.assertEqual(lock["codename"], "noble")
        self.assertEqual(lock["architecture"], "arm64")
        self.assertEqual(lock["runtimeKind"], "proot")
        self.assertEqual(lock["aptSnapshot"], "20260212T150000Z")

    def test_invalid_distribution(self):
        import filesystem as fs_module
        original_lock_path = fs_module.LOCK_FILE
        try:
            invalid_lock = {
                "schemaVersion": 1,
                "componentId": "builtin.ubuntu-proot-rootfs",
                "distribution": "debian",
                "flavor": "ubuntu-base",
                "release": "24.04.4",
                "codename": "noble",
                "architecture": "arm64",
                "guestPlatform": "linux",
                "runtimeKind": "proot",
                "baseArchiveName": "ubuntu-base-24.04.4-base-arm64.tar.gz",
                "baseArchiveSha256": "04207713ece899c3740823d33690441ad3a7f0ded1101aca744e2b0f37ac7ff2",
                "aptSnapshot": "20260212T150000Z",
                "aptComponents": ["main", "universe"],
                "aptSuites": ["noble", "noble-updates", "noble-security"],
                "defaultLocale": "C.UTF-8",
                "defaultTimezone": "Etc/UTC",
            }
            with tempfile.NamedTemporaryFile(mode='w', suffix='.json', delete=False) as f:
                json.dump(invalid_lock, f)
                temp_path = f.name
            fs_module.LOCK_FILE = pathlib.Path(temp_path)
            with self.assertRaises(ValueError):
                fs_module.load_lock()
        finally:
            fs_module.LOCK_FILE = original_lock_path
            if os.path.exists(temp_path):
                os.unlink(temp_path)


class TestPackageRequest(unittest.TestCase):
    def test_load_requested_packages(self):
        req_path = SCRIPT_DIR / "packages.requested.json"
        with open(req_path, "r", encoding="utf-8") as f:
            data = json.load(f)
        self.assertIn("packages", data)
        self.assertIn("bash", data["packages"])
        self.assertIn("ca-certificates", data["packages"])
        forbidden_packages = [
            "systemd", "snapd", "dbus", "udev", "cron", "rsyslog",
            "openssh-server", "sudo", "docker", "podman", "gcc", "g++",
            "make", "python3-dev",
        ]
        for pkg in forbidden_packages:
            self.assertNotIn(pkg, data["packages"], f"不应包含包: {pkg}")


class TestOverlayRules(unittest.TestCase):
    def test_overlay_blocked_paths(self):
        with tempfile.TemporaryDirectory() as tmpdir:
            overlay_dir = pathlib.Path(tmpdir) / "overlays"
            etc_dir = overlay_dir / "etc"
            etc_dir.mkdir(parents=True, exist_ok=True)
            with open(etc_dir / "hostname", "w") as f:
                f.write("test\n")
            with tempfile.TemporaryDirectory() as target:
                target_path = pathlib.Path(target)
                (target_path / "etc").mkdir(parents=True, exist_ok=True)
                apply_overlay(overlay_dir, target_path)
                self.assertTrue((target_path / "etc" / "hostname").exists())

    def test_overlay_rejects_amitia_dirs(self):
        with tempfile.TemporaryDirectory() as tmpdir:
            overlay_dir = pathlib.Path(tmpdir) / "overlays"
            opt_amitia = overlay_dir / "opt" / "amitia"
            opt_amitia.mkdir(parents=True, exist_ok=True)
            with open(opt_amitia / "test.conf", "w") as f:
                f.write("test\n")
            with tempfile.TemporaryDirectory() as target:
                target_path = pathlib.Path(target)
                with self.assertRaises(RuntimeError):
                    apply_overlay(overlay_dir, target_path)

    def test_overlay_rejects_absolute_target(self):
        with tempfile.TemporaryDirectory() as tmpdir:
            overlay_dir = pathlib.Path(tmpdir) / "overlays"
            with tempfile.TemporaryDirectory() as target:
                target_path = pathlib.Path(target)
                try:
                    overlay_dir.symlink_to("/etc")
                    apply_overlay(overlay_dir, target_path)
                except (RuntimeError, OSError):
                    pass


class TestCleanup(unittest.TestCase):
    def test_cleanup_apt_cache(self):
        with tempfile.TemporaryDirectory() as tmpdir:
            root = pathlib.Path(tmpdir)
            apt_archives = root / "var" / "cache" / "apt" / "archives"
            apt_archives.mkdir(parents=True, exist_ok=True)
            with open(apt_archives / "test.deb", "w") as f:
                f.write("fake deb\n")
            cleanup_cache(root)
            self.assertFalse((apt_archives / "test.deb").exists())

    def test_cleanup_machine_id(self):
        with tempfile.TemporaryDirectory() as tmpdir:
            root = pathlib.Path(tmpdir)
            machine_id = root / "etc" / "machine-id"
            machine_id.parent.mkdir(parents=True, exist_ok=True)
            with open(machine_id, "w") as f:
                f.write("some-machine-id\n")
            cleanup_cache(root)
            if machine_id.exists():
                content = machine_id.read_text()
                self.assertEqual(content, "")

    def test_cleanup_ssh_keys(self):
        with tempfile.TemporaryDirectory() as tmpdir:
            root = pathlib.Path(tmpdir)
            ssh_dir = root / "etc" / "ssh"
            ssh_dir.mkdir(parents=True, exist_ok=True)
            for key_name in ["ssh_host_rsa_key", "ssh_host_ecdsa_key"]:
                with open(ssh_dir / key_name, "w") as f:
                    f.write("fake key\n")
            cleanup_cache(root)


class TestMountPoints(unittest.TestCase):
    def test_create_mount_points(self):
        with tempfile.TemporaryDirectory() as tmpdir:
            root = pathlib.Path(tmpdir)
            create_mount_points(root)
            self.assertTrue((root / "dev").exists())
            self.assertTrue((root / "dev" / "pts").exists())
            self.assertTrue((root / "proc").exists())
            self.assertTrue((root / "sys").exists())
            self.assertTrue((root / "run").exists())
            self.assertTrue((root / "tmp").exists())
            self.assertTrue((root / "var" / "tmp").exists())
            var_run = root / "var" / "run"
            if var_run.is_symlink():
                self.assertEqual(os.readlink(var_run), "../run")
            else:
                self.assertTrue(var_run.exists())

    def test_tmp_permissions(self):
        with tempfile.TemporaryDirectory() as tmpdir:
            root = pathlib.Path(tmpdir)
            create_mount_points(root)
            tmp_stat = (root / "tmp").stat()
            if sys.platform != "win32":
                self.assertEqual(stat.S_IMODE(tmp_stat.st_mode), 0o1777)


class TestPackageLock(unittest.TestCase):
    def test_package_lock_structure(self):
        lock_path = SCRIPT_DIR / "packages.lock.json"
        if not lock_path.exists():
            self.skipTest("packages.lock.json 不存在")
        with open(lock_path, "r", encoding="utf-8") as f:
            lock = json.load(f)
        self.assertIn("schemaVersion", lock)
        self.assertIn("resolvedPackages", lock)
        self.assertEqual(lock["architecture"], "arm64")
        self.assertEqual(lock.get("aptSnapshot"), "20260212T150000Z")
        for pkg in lock["resolvedPackages"]:
            self.assertIn("name", pkg)
            self.assertIn("version", pkg)
            self.assertIn("sha256", pkg)
            self.assertEqual(len(pkg["sha256"]), 64)
            self.assertIsInstance(pkg.get("size", 0), int)
            self.assertGreater(pkg.get("size", 1), 0)


if __name__ == "__main__":
    unittest.main()
