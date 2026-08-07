import unittest
import json
import tempfile
from pathlib import Path


class TestBuildLockFile(unittest.TestCase):

    def test_lock_file_exists(self):
        lock_file = Path(__file__).parent / "proot.lock.json"
        self.assertTrue(lock_file.exists())

    def test_lock_file_valid_json(self):
        lock_file = Path(__file__).parent / "proot.lock.json"
        with open(lock_file, "r") as f:
            data = json.load(f)
        self.assertIsInstance(data, dict)

    def test_lock_mandatory_fields(self):
        lock_file = Path(__file__).parent / "proot.lock.json"
        with open(lock_file, "r") as f:
            data = json.load(f)
        self.assertIn("componentId", data)
        self.assertEqual(data["componentId"], "runtime.proot")
        self.assertIn("upstream", data)
        self.assertIn("tag", data["upstream"])
        self.assertEqual(data["upstream"]["tag"], "v5.4.0")
        self.assertIn("target", data)
        self.assertEqual(data["target"]["abi"], "arm64-v8a")
        self.assertEqual(data["target"]["architecture"], "aarch64")

    def test_lock_version_format(self):
        lock_file = Path(__file__).parent / "proot.lock.json"
        with open(lock_file, "r") as f:
            data = json.load(f)
        version = data.get("version", "")
        self.assertTrue(version.startswith("5.4.0"))


class TestPythonScripts(unittest.TestCase):

    def test_update_lock_syntax(self):
        import py_compile
        script = Path(__file__).parent / "update_lock.py"
        try:
            py_compile.compile(str(script), doraise=True)
        except py_compile.PyCompileError as e:
            self.fail(f"Syntax error: {e}")

    def test_prepare_source_syntax(self):
        import py_compile
        script = Path(__file__).parent / "prepare_source.py"
        try:
            py_compile.compile(str(script), doraise=True)
        except py_compile.PyCompileError as e:
            self.fail(f"Syntax error: {e}")

    def test_build_syntax(self):
        import py_compile
        script = Path(__file__).parent / "build.py"
        try:
            py_compile.compile(str(script), doraise=True)
        except py_compile.PyCompileError as e:
            self.fail(f"Syntax error: {e}")

    def test_verify_syntax(self):
        import py_compile
        script = Path(__file__).parent / "verify.py"
        try:
            py_compile.compile(str(script), doraise=True)
        except py_compile.PyCompileError as e:
            self.fail(f"Syntax error: {e}")


class TestPlaceholderDetection(unittest.TestCase):

    def test_placeholder_detected(self):
        lock_file = Path(__file__).parent / "proot.lock.json"
        with open(lock_file, "r") as f:
            content = f.read()
        self.assertTrue("placeholder" in content)


if __name__ == "__main__":
    unittest.main()
