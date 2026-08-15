import os
import shutil
import tempfile
import unittest

from common.atomic_publish import atomic_publish_directory, AtomicPublishResult


class TestAtomicPublishDirectory(unittest.TestCase):
    def setUp(self):
        self.tmpdir = tempfile.mkdtemp()
        self.source = os.path.join(self.tmpdir, "source")
        self.target_base = os.path.join(self.tmpdir, "output")
        os.makedirs(self.source)
        with open(os.path.join(self.source, "artifact.txt"), "w") as f:
            f.write("artifact content")

    def tearDown(self):
        shutil.rmtree(self.tmpdir, ignore_errors=True)

    def test_creates_version_directory(self):
        result = atomic_publish_directory(self.source, self.target_base, "v1.0.0")
        self.assertTrue(result.success)
        expected_dir = os.path.join(self.target_base, "v1.0.0")
        self.assertEqual(result.published_dir, expected_dir)
        self.assertTrue(os.path.isdir(expected_dir))
        self.assertTrue(os.path.isfile(os.path.join(expected_dir, "artifact.txt")))

    def test_version_directory_contains_source_files(self):
        sub = os.path.join(self.source, "subdir")
        os.makedirs(sub)
        with open(os.path.join(sub, "nested.txt"), "w") as f:
            f.write("nested")
        result = atomic_publish_directory(self.source, self.target_base, "v2.0.0")
        self.assertTrue(result.success)
        nested_path = os.path.join(self.target_base, "v2.0.0", "subdir", "nested.txt")
        self.assertTrue(os.path.isfile(nested_path))

    def test_failure_does_not_pollute_existing(self):
        existing_dir = os.path.join(self.target_base, "v1.0.0")
        os.makedirs(existing_dir)
        with open(os.path.join(existing_dir, "existing.txt"), "w") as f:
            f.write("pre-existing")
        result = atomic_publish_directory(self.source, self.target_base, "v1.0.0")
        self.assertFalse(result.success)
        self.assertTrue(len(result.errors) > 0)
        self.assertTrue(os.path.isfile(os.path.join(existing_dir, "existing.txt")))

    def test_empty_source_fails(self):
        empty_source = os.path.join(self.tmpdir, "empty")
        os.makedirs(empty_source)
        result = atomic_publish_directory(empty_source, self.target_base, "v3.0.0")
        self.assertFalse(result.success)
        self.assertTrue(any("empty" in e.lower() for e in result.errors))

    def test_nonexistent_source_fails(self):
        result = atomic_publish_directory(
            os.path.join(self.tmpdir, "nonexistent"),
            self.target_base,
            "v4.0.0",
        )
        self.assertFalse(result.success)
        self.assertTrue(any("does not exist" in e for e in result.errors))

    def test_staging_dir_cleaned_on_success(self):
        result = atomic_publish_directory(self.source, self.target_base, "v5.0.0")
        self.assertTrue(result.success)
        entries = os.listdir(self.target_base)
        staging = [e for e in entries if e.startswith(".publish-")]
        self.assertEqual(staging, [])

    def test_result_has_errors_on_failure(self):
        result = atomic_publish_directory(
            os.path.join(self.tmpdir, "nonexistent"),
            self.target_base,
            "v6.0.0",
        )
        self.assertFalse(result.success)
        self.assertIsInstance(result.errors, list)
        self.assertTrue(len(result.errors) > 0)


if __name__ == "__main__":
    unittest.main()
