import json
import os
import pathlib
import sys
import tempfile
import unittest
from unittest.mock import MagicMock, patch

SCRIPT_DIR = pathlib.Path(__file__).resolve().parent
sys.path.insert(0, str(SCRIPT_DIR))

from proot_runner import (
    detect_qemu_needed,
    build_command,
    FIXED_ENV,
    BLOCKED_ENV_KEYS,
    _validate_bindings,
    _validate_env,
    architecture,
    supports_arm64_execution,
)


class FakeProcessRunner:
    def __init__(self):
        self.calls = []
        self.results = {}

    def set_result(self, command_key, returncode=0, stdout=b"", stderr=b""):
        self.results[command_key] = (returncode, stdout, stderr)

    def run(self, command, env=None, binds=None, working_dir="/", timeout=None,
            rootfs_path=None, capture_output=False):
        self.calls.append({
            "command": command,
            "env": env,
            "binds": binds,
            "working_dir": working_dir,
            "timeout": timeout,
            "rootfs_path": rootfs_path,
        })
        key = tuple(command) if isinstance(command, list) else command
        rc, so, se = self.results.get(key, (0, b"", ""))
        return type("Result", (), {
            "returncode": rc,
            "stdout": so,
            "stderr": se,
        })()


class TestArchitecture(unittest.TestCase):
    def test_detect_qemu_needed(self):
        if sys.platform == "linux":
            import platform
            host = platform.machine().lower()
            if host in ("aarch64", "arm64"):
                self.assertFalse(detect_qemu_needed("arm64"))
            else:
                self.assertTrue(detect_qemu_needed("arm64"))

    def test_current_architecture(self):
        arch = architecture()
        self.assertIn(arch, ["x86_64", "amd64", "aarch64", "arm64"])

    def test_supports_arm64_execution(self):
        result = supports_arm64_execution()
        import platform
        host = platform.machine().lower()
        if host in ("aarch64", "arm64"):
            self.assertTrue(result)
        else:
            self.assertFalse(result)


class TestEnvironment(unittest.TestCase):
    def test_fixed_env_completeness(self):
        required_keys = [
            "DEBIAN_FRONTEND", "LANG", "LC_ALL", "TZ", "HOME", "PATH",
            "APT_LISTCHANGES_FRONTEND", "UCF_FORCE_CONFFOLD",
        ]
        for key in required_keys:
            self.assertIn(key, FIXED_ENV, f"缺少环境变量: {key}")
        self.assertEqual(FIXED_ENV["DEBIAN_FRONTEND"], "noninteractive")
        self.assertEqual(FIXED_ENV["LANG"], "C.UTF-8")
        self.assertEqual(FIXED_ENV["LC_ALL"], "C.UTF-8")
        self.assertEqual(FIXED_ENV["TZ"], "Etc/UTC")

    def test_blocked_env_keys(self):
        blocked = {"HOME", "USER", "PATH", "GOPATH", "GOROOT", "NODE_PATH", "ANDROID_HOME"}
        for key in blocked:
            self.assertIn(key, BLOCKED_ENV_KEYS)

    def test_validate_env_removes_blocked(self):
        env = {
            "HOME": "/home/test",
            "GOPATH": "/go",
            "LANG": "en_US.UTF-8",
            "TZ": "Asia/Shanghai",
            "http_proxy": "http://proxy:8080",
        }
        clean = _validate_env(env)
        self.assertNotIn("HOME", clean)
        self.assertNotIn("GOPATH", clean)
        self.assertNotIn("http_proxy", clean)
        self.assertIn("LANG", clean)


class TestBindings(unittest.TestCase):
    def test_validate_bindings_nonexistent_source(self):
        with self.assertRaises(RuntimeError):
            _validate_bindings([("/nonexistent/path/12345", "/target")])

    def test_validate_bindings_absolute_target_required(self):
        with tempfile.TemporaryDirectory() as tmpdir:
            test_file = pathlib.Path(tmpdir) / "test.txt"
            test_file.write_text("test\n")
            with self.assertRaises(RuntimeError):
                _validate_bindings([(test_file, "relative/path")])

    def test_validate_bindings_valid(self):
        with tempfile.TemporaryDirectory() as tmpdir:
            test_file = pathlib.Path(tmpdir) / "test.txt"
            test_file.write_text("test\n")
            result = _validate_bindings([(test_file, "/container/test")])
            self.assertEqual(len(result), 1)
            self.assertEqual(result[0][1], "/container/test")


class TestCommandBuilding(unittest.TestCase):
    def test_command_uses_array(self):
        with tempfile.TemporaryDirectory() as tmpdir:
            rootfs = pathlib.Path(tmpdir) / "rootfs"
            rootfs.mkdir()
            cmd = build_command(str(rootfs), qemu_needed=False, proot_path="")
            self.assertIsInstance(cmd, list)
            self.assertTrue(len(cmd) > 0)
            for arg in cmd:
                self.assertIsInstance(arg, str)

    def test_command_includes_rootfs(self):
        with tempfile.TemporaryDirectory() as tmpdir:
            rootfs = pathlib.Path(tmpdir) / "rootfs"
            rootfs.mkdir()
            cmd = build_command(str(rootfs), qemu_needed=False, proot_path="")
            self.assertIn("-S", cmd)
            s_idx = cmd.index("-S")
            self.assertEqual(cmd[s_idx + 1], str(rootfs))

    def test_command_includes_proc_dev_sys(self):
        with tempfile.TemporaryDirectory() as tmpdir:
            rootfs = pathlib.Path(tmpdir) / "rootfs"
            rootfs.mkdir()
            cmd = build_command(str(rootfs), qemu_needed=False, proot_path="")
            self.assertIn("-b", cmd)
            binds = []
            for i, arg in enumerate(cmd):
                if arg == "-b" and i + 1 < len(cmd):
                    binds.append(cmd[i + 1])
            self.assertTrue(any("proc" in b for b in binds))
            self.assertTrue(any("dev" in b for b in binds))
            self.assertTrue(any("sys" in b for b in binds))

    def test_qemu_command_building(self):
        with tempfile.TemporaryDirectory() as tmpdir:
            rootfs = pathlib.Path(tmpdir) / "rootfs"
            rootfs.mkdir()
            fake_qemu = pathlib.Path(tmpdir) / "qemu-aarch64-static"
            fake_qemu.write_text("#!/bin/sh\n")
            fake_qemu.chmod(0o755)
            cmd = build_command(str(rootfs), qemu_needed=True, qemu_path=str(fake_qemu), proot_path="")
            self.assertIn("-q", cmd)
            q_idx = cmd.index("-q")
            self.assertEqual(cmd[q_idx + 1], str(fake_qemu))

    def test_no_shell_concatenation(self):
        with tempfile.TemporaryDirectory() as tmpdir:
            rootfs = pathlib.Path(tmpdir) / "rootfs"
            rootfs.mkdir()
            cmd = build_command(str(rootfs), qemu_needed=False, proot_path="")
            cmd_str = " ".join(cmd)
            self.assertNotIn("$", cmd_str.split("-S")[0])
            self.assertNotIn("&&", cmd_str)
            self.assertNotIn(";", cmd_str.split("-S")[0])


class TestFakeProcessRunner(unittest.TestCase):
    def test_fake_runner_records_calls(self):
        runner = FakeProcessRunner()
        runner.set_result(("/bin/bash", "--version"), returncode=0, stdout=b"bash 5.1\n")
        result = runner.run(
            ["/bin/bash", "--version"],
            rootfs_path="/tmp/test",
        )
        self.assertEqual(result.returncode, 0)
        self.assertEqual(len(runner.calls), 1)
        self.assertEqual(runner.calls[0]["command"], ["/bin/bash", "--version"])

    def test_fake_runner_returns_configured_result(self):
        runner = FakeProcessRunner()
        runner.set_result(("cat", "/etc/hostname"), returncode=0, stdout=b"test-host\n")
        result = runner.run(
            ["cat", "/etc/hostname"],
            rootfs_path="/tmp/test",
        )
        self.assertEqual(result.stdout, b"test-host\n")

    def test_fake_runner_returns_default_result(self):
        runner = FakeProcessRunner()
        result = runner.run(
            ["unknown", "command"],
            rootfs_path="/tmp/test",
        )
        self.assertEqual(result.returncode, 0)


class TestRunValidation(unittest.TestCase):
    def test_run_requires_rootfs_path(self):
        from proot_runner import run
        with self.assertRaises(RuntimeError):
            run(["/bin/bash"], rootfs_path=None)

    def test_run_requires_command_array(self):
        from proot_runner import run
        with tempfile.TemporaryDirectory() as tmpdir:
            rootfs = pathlib.Path(tmpdir) / "rootfs"
            rootfs.mkdir()
            with self.assertRaises(RuntimeError):
                run("string command", rootfs_path=str(rootfs))

    def test_run_rejects_shell_concatenation(self):
        from proot_runner import run
        with tempfile.TemporaryDirectory() as tmpdir:
            rootfs = pathlib.Path(tmpdir) / "rootfs"
            rootfs.mkdir()
            with self.assertRaises(RuntimeError):
                run("ls -la && echo done", rootfs_path=str(rootfs))


if __name__ == "__main__":
    unittest.main()
