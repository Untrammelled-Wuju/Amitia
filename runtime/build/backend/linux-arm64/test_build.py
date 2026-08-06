import json
import os
import pathlib
import re
import shutil
import tempfile
import unittest

import build


class TestValidateVersion(unittest.TestCase):
    def test_valid_version(self):
        self.assertEqual(build.validate_version("1.0.0"), "1.0.0")

    def test_valid_prerelease(self):
        self.assertEqual(build.validate_version("1.0.0-beta.1"), "1.0.0-beta.1")

    def test_valid_build_metadata(self):
        self.assertEqual(build.validate_version("1.0.0+build.1"), "1.0.0+build.1")

    def test_empty_rejected(self):
        with self.assertRaises(build.BuildError):
            build.validate_version("")

    def test_none_rejected(self):
        with self.assertRaises(build.BuildError):
            build.validate_version(None)

    def test_dev_rejected_in_formal(self):
        with self.assertRaises(build.BuildError):
            build.validate_version("dev", development=False)

    def test_unknown_rejected_in_formal(self):
        with self.assertRaises(build.BuildError):
            build.validate_version("unknown", development=False)

    def test_dev_allowed_in_development(self):
        self.assertEqual(build.validate_version("dev", development=True), "dev")

    def test_whitespace_rejected(self):
        with self.assertRaises(build.BuildError):
            build.validate_version(" 1.0.0")

    def test_space_inside_rejected(self):
        with self.assertRaises(build.BuildError):
            build.validate_version("1.0 0")

    def test_path_separator_rejected(self):
        with self.assertRaises(build.BuildError):
            build.validate_version("1.0/0")

    def test_backslash_rejected(self):
        with self.assertRaises(build.BuildError):
            build.validate_version("1.0\\0")

    def test_newline_rejected(self):
        with self.assertRaises(build.BuildError):
            build.validate_version("1.0\n0")

    def test_max_length(self):
        long_ver = "1." + "0" * 126
        self.assertEqual(len(long_ver), 128)
        result = build.validate_version(long_ver)
        self.assertEqual(result, long_ver)

    def test_too_long_rejected(self):
        with self.assertRaises(build.BuildError):
            build.validate_version("1." + "0" * 150)


class TestValidateCommit(unittest.TestCase):
    def test_valid_40_char(self):
        commit = "a" * 40
        self.assertEqual(build.validate_commit(commit), commit)

    def test_valid_64_char(self):
        commit = "a" * 64
        self.assertEqual(build.validate_commit(commit), commit)

    def test_valid_mixed_case(self):
        commit = "abcdef0123456789abcdef0123456789abcdef01"
        self.assertEqual(len(commit), 40)
        self.assertEqual(build.validate_commit(commit), commit)

    def test_empty_rejected(self):
        with self.assertRaises(build.BuildError):
            build.validate_commit("")

    def test_none_rejected(self):
        with self.assertRaises(build.BuildError):
            build.validate_commit(None)

    def test_short_commit_rejected(self):
        with self.assertRaises(build.BuildError):
            build.validate_commit("abc123")

    def test_39_chars_rejected(self):
        with self.assertRaises(build.BuildError):
            build.validate_commit("a" * 39)

    def test_uppercase_rejected(self):
        with self.assertRaises(build.BuildError):
            build.validate_commit("A" * 40)

    def test_all_zeros_rejected(self):
        with self.assertRaises(build.BuildError):
            build.validate_commit("0" * 40)

    def test_dirty_rejected(self):
        with self.assertRaises(build.BuildError):
            build.validate_commit("a" * 39 + "dirty")

    def test_whitespace_rejected(self):
        with self.assertRaises(build.BuildError):
            build.validate_commit(" " + "a" * 39)


class TestLoadLock(unittest.TestCase):
    def setUp(self):
        self.tmp = tempfile.mkdtemp()

    def tearDown(self):
        shutil.rmtree(self.tmp, ignore_errors=True)

    def test_load_valid_lock(self):
        lock_data = {
            "schemaVersion": 1,
            "componentId": "runtime.backend",
            "componentName": "amitia-server",
            "toolchain": {"version": "go1.26.1"},
        }
        lock_path = pathlib.Path(self.tmp) / "test.lock.json"
        with open(lock_path, "w", encoding="utf-8") as f:
            json.dump(lock_data, f)
        result = build.load_lock(str(lock_path))
        self.assertEqual(result["componentId"], "runtime.backend")
        self.assertEqual(result["toolchain"]["version"], "go1.26.1")


class TestResolveRepoRoot(unittest.TestCase):
    def test_resolve_from_build_py(self):
        root = build.resolve_repo_root()
        self.assertTrue((root / "backend").exists())


class TestSha256File(unittest.TestCase):
    def setUp(self):
        self.tmp = tempfile.mkdtemp()

    def tearDown(self):
        shutil.rmtree(self.tmp, ignore_errors=True)

    def test_known_sha(self):
        p = pathlib.Path(self.tmp) / "test.txt"
        p.write_bytes(b"test data")
        result = build.sha256_file(str(p))
        self.assertEqual(len(result), 64)
        self.assertTrue(re.fullmatch(r"[0-9a-f]{64}", result))

    def test_empty_file(self):
        p = pathlib.Path(self.tmp) / "empty"
        p.write_bytes(b"")
        result = build.sha256_file(str(p))
        self.assertEqual(len(result), 64)


if __name__ == "__main__":
    unittest.main()
