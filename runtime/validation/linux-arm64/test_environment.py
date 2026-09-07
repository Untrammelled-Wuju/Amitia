import os
import sys
import tempfile
import unittest

sys.path.insert(0, os.path.dirname(os.path.abspath(__file__)))


class TestEnvironmentDetection(unittest.TestCase):
    def setUp(self):
        self.tmpdir = tempfile.mkdtemp()

    def tearDown(self):
        import shutil
        shutil.rmtree(self.tmpdir, ignore_errors=True)

    def test_environment_report_fields(self):
        from environment import EnvironmentReport
        report = EnvironmentReport()
        self.assertEqual(report.kernel, "")
        self.assertEqual(report.architecture, "")
        self.assertFalse(report.isLinux)
        self.assertFalse(report.isAarch64)
        self.assertFalse(report.symlinkSupported)

    def test_environment_report_safe_dict(self):
        from environment import EnvironmentReport
        report = EnvironmentReport(
            kernel="Linux",
            architecture="aarch64",
            distribution="ubuntu",
            distributionVersion="24.04.4",
            isLinux=True,
            isAarch64=True,
            symlinkSupported=True,
        )
        safe_dict = report.to_safe_dict()
        self.assertTrue(safe_dict["isLinux"])
        self.assertTrue(safe_dict["isAarch64"])
        self.assertIn("kernel", safe_dict)
        self.assertIn("architecture", safe_dict)
        self.assertIn("distribution", safe_dict)

    def test_linux_required(self):
        from environment import KERNEL_REQUIRED, ARCH_REQUIRED
        self.assertEqual(KERNEL_REQUIRED, "Linux")
        self.assertEqual(ARCH_REQUIRED, "aarch64")

    def test_disk_space_check(self):
        from environment import validate_environment
        report = validate_environment(self.tmpdir, expected_env_type="test")
        self.assertIsInstance(report.diskFreeBytes, int)
        self.assertIsInstance(report.diskSufficient, bool)

    def test_environment_type_emulated(self):
        from environment import validate_environment
        report = validate_environment(self.tmpdir, expected_env_type="emulated-arm64")
        self.assertEqual(report.environmentType, "emulated-arm64")


class TestSymlinkSupport(unittest.TestCase):
    def test_symlink_creation(self):
        if sys.platform == "win32":
            self.skipTest("Unix symlinks not reliably testable on Windows")
        from environment import _test_symlink_support
        with tempfile.TemporaryDirectory() as tmpdir:
            result = _test_symlink_support(tmpdir)
            self.assertTrue(result)

    def test_execute_permission(self):
        if sys.platform == "win32":
            self.skipTest("Unix execute permission not reliably testable on Windows")
        from environment import _test_execute_permission
        with tempfile.TemporaryDirectory() as tmpdir:
            result = _test_execute_permission(tmpdir)
            self.assertTrue(result)


class TestSensitiveEnvFiltering(unittest.TestCase):
    def test_sensitive_keys_redacted(self):
        from environment import SENSITIVE_ENV_KEYS
        self.assertIn("AMITIA_LOCAL_TOKEN", SENSITIVE_ENV_KEYS)
        self.assertIn("GITHUB_TOKEN", SENSITIVE_ENV_KEYS)
        self.assertIn("AWS_SECRET_ACCESS_KEY", SENSITIVE_ENV_KEYS)


if __name__ == "__main__":
    unittest.main()
