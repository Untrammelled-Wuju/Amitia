import hashlib
import json
import os
import pathlib
import shutil
import sys
import tarfile
import tempfile
import unittest

sys.path.insert(0, os.path.dirname(os.path.abspath(__file__)))
import build as bld
import verify_component as vc


class TestBuildInputValidation(unittest.TestCase):
    def test_valid_runtime_version(self):
        bld.validate_version("1.0.0")
        bld.validate_version("1.0.0-beta.1")
        bld.validate_version("2.1.3+build.1")

    def test_invalid_versions(self):
        bad = ["latest", "stable", "dev", "unknown", "", "current"]
        for v in bad:
            with self.assertRaises(ValueError, msg=f"Should fail: {v}"):
                bld.validate_version(v)

    def test_version_with_spaces(self):
        with self.assertRaises(ValueError):
            bld.validate_version("1.0 0")

    def test_version_with_path_sep(self):
        with self.assertRaises(ValueError):
            bld.validate_version("1.0/0")

    def test_long_version_fails(self):
        with self.assertRaises(ValueError):
            bld.validate_version("1.0.0" + "x" * 200)

    def test_valid_commit(self):
        valid = "ed7e2f0140b78bd3a5088227882a2e78dd4c98e9"
        bld.validate_commit(valid)

    def test_invalid_commits(self):
        bad = ["", "abc", "ghijkl", "z" * 40, "0" * 40]
        for c in bad:
            with self.assertRaises(ValueError, msg=f"Should fail: {c[:10]}"):
                bld.validate_commit(c)


class TestComponentValidation(unittest.TestCase):
    def test_backend_metadata_valid(self):
        meta = {
            "componentId": "runtime.backend",
            "version": "dev",
            "platform": "linux",
            "architecture": "arm64",
            "sha256": "abc123",
        }
        vc.validate_component_metadata("runtime.backend", meta, "abc123")

    def test_wrong_platform_fails(self):
        meta = {
            "componentId": "runtime.backend",
            "version": "dev",
            "platform": "windows",
            "architecture": "arm64",
        }
        with self.assertRaises(ValueError):
            vc.validate_component_metadata("runtime.backend", meta)

    def test_wrong_arch_fails(self):
        meta = {
            "componentId": "runtime.backend",
            "version": "dev",
            "platform": "linux",
            "architecture": "amd64",
        }
        with self.assertRaises(ValueError):
            vc.validate_component_metadata("runtime.backend", meta)

    def test_sha_mismatch_fails(self):
        meta = {
            "componentId": "runtime.backend",
            "version": "dev",
            "platform": "linux",
            "architecture": "arm64",
            "sha256": "abc",
        }
        with self.assertRaises(ValueError):
            vc.validate_component_metadata("runtime.backend", meta, "xyz")


class TestLockValidation(unittest.TestCase):
    def test_valid_lock(self):
        lock = bld.load_lock()
        vc.validate_lock(lock, "1.0.0", "ed7e2f0140b78bd3a5088227882a2e78dd4c98e9")

    def test_wrong_target_abi(self):
        lock = bld.load_lock()
        lock["target"]["hostAbi"] = "armeabi-v7a"
        with self.assertRaises(ValueError):
            vc.validate_lock(lock, "1.0.0", "ed7e2f0140b78bd3a5088227882a2e78dd4c98e9")

    def test_missing_component(self):
        lock = bld.load_lock()
        del lock["components"]["backend"]
        with self.assertRaises(ValueError):
            vc.validate_lock(lock, "1.0.0", "ed7e2f0140b78bd3a5088227882a2e78dd4c98e9")


if __name__ == "__main__":
    unittest.main()
