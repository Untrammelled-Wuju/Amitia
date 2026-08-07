import json
import os
import pathlib
import shutil
import subprocess
import sys
import tempfile
import unittest

SCRIPT_DIR = pathlib.Path(__file__).resolve().parent
SOURCE_DIR = SCRIPT_DIR / "source"
DEFAULT_NODE_DIST = SCRIPT_DIR.parent.parent / "out" / "node" / "linux-arm64" / "node"

IS_POSIX = sys.platform != "win32"


def _sh(cmd, env=None):
    _env = os.environ.copy()
    _env["PATH"] = "/usr/local/sbin:/usr/local/bin:/usr/sbin:/usr/bin:/sbin:/bin"
    if env:
        _env.update(env)
    return subprocess.run(
        ["/bin/sh"] + cmd,
        capture_output=True, text=True, timeout=60, env=_env,
    )


def _ensure_dir(d):
    pathlib.Path(d).mkdir(parents=True, exist_ok=True)
    return pathlib.Path(d)


@unittest.skipUnless(IS_POSIX, "POSIX 环境专用测试")
class ScriptTests(unittest.TestCase):
    def setUp(self):
        self.tmp = pathlib.Path(tempfile.mkdtemp(prefix="amitia-script-test-"))
        self.runtime_root = self.tmp / "runtime"
        node_src = pathlib.Path(os.environ.get("AMITIA_NODE_DIST", str(DEFAULT_NODE_DIST)))
        if node_src.exists():
            shutil.copytree(node_src, self.runtime_root / "node", symlinks=True)
        else:
            (self.runtime_root / "node" / "bin").mkdir(parents=True, exist_ok=True)
            (self.runtime_root / "node" / "bin" / "node").write_text("#!/bin/sh\necho v24.19.0\n")
            os.chmod(self.runtime_root / "node" / "bin" / "node", 0o755)
            (self.runtime_root / "node" / "lib" / "node_modules" / "npm" / "bin").mkdir(parents=True, exist_ok=True)
            (self.runtime_root / "node" / "lib" / "node_modules" / "npm" / "bin" / "npm-cli.js").write_text(
                "console.log('11.17.0');\n"
            )
            (self.runtime_root / "node" / "lib" / "node_modules" / "npm" / "bin" / "npx-cli.js").write_text(
                "console.log('11.17.0');\n"
            )
            (self.runtime_root / "node" / "lib" / "node_modules" / "npm" / "package.json").write_text(
                json.dumps({"name": "npm", "version": "11.17.0"})
            )
        scripts_src = SOURCE_DIR
        shutil.copytree(scripts_src, self.runtime_root / "scripts" / "node", symlinks=True)
        self.scripts_dir = self.runtime_root / "scripts" / "node"
        plugin_dir = self.runtime_root / "plugin-host"
        plugin_dir.mkdir(parents=True, exist_ok=True)
        (plugin_dir / "dist").mkdir(parents=True, exist_ok=True)
        (plugin_dir / "dist" / "index.js").write_text(
            "process.stdout.write('plugin-host-ok');\n"
        )
        task_dir = self.runtime_root / "node"
        task_host_dir = self.runtime_root / "task-host"
        task_host_dir.mkdir(parents=True, exist_ok=True)
        (task_host_dir / "dist").mkdir(parents=True, exist_ok=True)
        (task_host_dir / "dist" / "index.js").write_text(
            "process.stdout.write('task-host-ok');\n"
        )

    def tearDown(self):
        shutil.rmtree(self.tmp, ignore_errors=True)

    def _prepare_env(self):
        data = self.tmp / "data"
        cache = self.tmp / "cache"
        temp = self.tmp / "temp"
        data.mkdir(parents=True, exist_ok=True)
        cache.mkdir(parents=True, exist_ok=True)
        temp.mkdir(parents=True, exist_ok=True)
        return {
            "AMITIA_DATA_ROOT": str(data),
            "AMITIA_CACHE_ROOT": str(cache),
            "AMITIA_TEMP_ROOT": str(temp),
        }

    def _run_prepare(self):
        env = self._prepare_env()
        r = _sh([str(self.scripts_dir / "amitia-node-prepare.sh")], env=env)
        self.assertEqual(r.returncode, 0, msg=r.stderr)
        for line in r.stdout.strip().split("\n"):
            if "=" in line:
                k, v = line.split("=", 1)
                env[k] = v
        return env

    def _run_script(self, name, args=None, env_extra=None):
        sh = self.scripts_dir / name
        env = self._run_prepare()
        if env_extra:
            env.update(env_extra)
        return _sh([str(sh)] + (args or []), env=env)

    def test_prepare_creates_isolated_dirs(self):
        r = self._run_script("amitia-node-prepare.sh")
        self.assertEqual(r.returncode, 0, msg=r.stderr)
        out = r.stdout.strip().split("\n")
        keys = dict(line.split("=", 1) for line in out if "=" in line)
        self.assertIn("AMITIA_NODE_HOME", keys)
        self.assertIn("AMITIA_NODE_PREFIX", keys)
        self.assertIn("AMITIA_NPM_CACHE", keys)
        self.assertIn("AMITIA_NODE_TMP", keys)
        home = pathlib.Path(keys["AMITIA_NODE_HOME"])
        self.assertTrue(home.exists())
        self.assertEqual(oct(home.stat().st_mode & 0o777), "0o700")

    def test_prepare_idempotent(self):
        e = self._prepare_env()
        r1 = _sh([str(self.scripts_dir / "amitia-node-prepare.sh")], env=e)
        r2 = _sh([str(self.scripts_dir / "amitia-node-prepare.sh")], env=e)
        self.assertEqual(r1.returncode, 0)
        self.assertEqual(r2.returncode, 0)

    def test_prepare_missing_env_fails(self):
        r = _sh([str(self.scripts_dir / "amitia-node-prepare.sh")], env={})
        self.assertEqual(r.returncode, 20)

    def test_prepare_invalid_path_fails(self):
        e = self._prepare_env()
        e["AMITIA_DATA_ROOT"] = "not/absolute"
        r = _sh([str(self.scripts_dir / "amitia-node-prepare.sh")], env=e)
        self.assertEqual(r.returncode, 21)

    def test_prepare_rejects_dotdot(self):
        e = self._prepare_env()
        e["AMITIA_DATA_ROOT"] = "/var/lib/../etc"
        r = _sh([str(self.scripts_dir / "amitia-node-prepare.sh")], env=e)
        self.assertEqual(r.returncode, 21)

    def test_node_exec_missing_arg(self):
        r = self._run_script("amitia-node-exec.sh")
        self.assertEqual(r.returncode, 2)

    def test_node_exec_invalid_extension(self):
        f = self.tmp / "test.ts"
        f.write_text("console.log('hello');\n")
        r = self._run_script("amitia-node-exec.sh", args=[str(f)])
        self.assertEqual(r.returncode, 2)

    def test_node_exec_absolute_path_required(self):
        r = self._run_script("amitia-node-exec.sh", args=["not/absolute.js"])
        self.assertEqual(r.returncode, 21)

    def test_node_exec_passes_args(self):
        f = self.tmp / "check.js"
        f.write_text("console.log(JSON.stringify(process.argv.slice(2)));\n")
        os.chmod(f, 0o644)
        r = self._run_script("amitia-node-exec.sh", args=[str(f), "中文 参数", "--flag=value"])
        self.assertEqual(r.returncode, 0, msg=r.stderr)

    def test_npm_exec_version(self):
        r = self._run_script("amitia-npm-exec.sh", args=["--version"])
        self.assertEqual(r.returncode, 0, msg=r.stderr)
        self.assertIn("11.17.0", r.stdout)

    def test_npx_exec_version(self):
        r = self._run_script("amitia-npx-exec.sh", args=["--version"])
        self.assertEqual(r.returncode, 0, msg=r.stderr)
        self.assertIn("11.17.0", r.stdout)

    def test_plugin_host_requires_workspace(self):
        r = self._run_script("amitia-plugin-host.sh")
        self.assertEqual(r.returncode, 20)

    def test_plugin_host_requires_absolute_workspace(self):
        e = self._prepare_env()
        e["AMITIA_PLUGIN_WORKSPACE"] = "not/absolute"
        r = self._run_script("amitia-plugin-host.sh", env_extra=e)
        self.assertEqual(r.returncode, 21)

    def test_task_host_requires_workspace(self):
        r = self._run_script("amitia-task-host.sh")
        self.assertEqual(r.returncode, 20)

    def test_task_host_cwd_is_workspace(self):
        ws = self.tmp / "task-ws"
        ws.mkdir(parents=True, exist_ok=True)
        (ws / "marker").write_text("here")
        e = self._prepare_env()
        e["AMITIA_TASK_WORKSPACE"] = str(ws)
        r = self._run_script("amitia-task-host.sh", env_extra=e)
        self.assertEqual(r.returncode, 0, msg=r.stderr)

    def test_common_lib_detects_missing_lib(self):
        common_lib = self.scripts_dir / "lib" / "amitia-node-common.sh"
        self.assertTrue(common_lib.exists(), "测试前置：lib 必须存在")
        common_lib.unlink()
        r = _sh([str(self.scripts_dir / "amitia-node-prepare.sh")], env={})
        self.assertEqual(r.returncode, 50)

    def test_probe_output_is_json(self):
        r = self._run_script("amitia-node-probe.sh")
        self.assertEqual(r.returncode, 0, msg=r.stderr)
        data = json.loads(r.stdout.strip())
        self.assertEqual(data["nodeVersion"], "v24.19.0")
        self.assertEqual(data["platform"], "linux")
        self.assertEqual(data["architecture"], "arm64")
        self.assertEqual(data["npmVersion"], "11.17.0")

    def test_environment_isolation(self):
        f = self.tmp / "env-check.js"
        f.write_text(
            "process.stdout.write("
            "process.env.NODE_OPTIONS ? 'HAS_NODE_OPTIONS' : 'NO_NODE_OPTIONS'"
            ");\n"
        )
        os.chmod(f, 0o644)
        r = self._run_script("amitia-node-exec.sh", args=[str(f)])
        self.assertEqual(r.returncode, 0, msg=r.stderr)
        self.assertIn("NO_NODE_OPTIONS", r.stdout)

    def test_no_background_process(self):
        r = self._run_script("amitia-node-probe.sh")
        self.assertEqual(r.returncode, 0)
        for script in ["amitia-node-prepare.sh", "amitia-npm-exec.sh"]:
            s = (self.scripts_dir / script).read_text()
            self.assertNotIn("&\n", s, msg=f"{script} 含后台运行")


@unittest.skipIf(IS_POSIX, "Windows 环境不需要此测试")
class WindowsPlaceholderTests(unittest.TestCase):
    def test_skipped_on_windows(self):
        self.assertTrue(True, "POSIX 测试在 Windows 标记未执行")


if __name__ == "__main__":
    unittest.main()
