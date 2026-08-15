import json
import os
import shutil
import tempfile
import unittest

from common.version_policy import (
    resolve_version,
    VersionResolution,
    VersionResolutionKind,
)
from common.artifact_record import FrozenArtifactRecord


def _make_record(**overrides):
    base = dict(
        schemaVersion=1,
        componentId="backend",
        version="1.0.0",
        platform="linux",
        architecture="arm64",
        artifactType="executable",
        artifactRelativePath="backend/linux-arm64/1.0.0/amitia-server",
        artifactSha256="a" * 64,
        treeSha256="b" * 64,
        sourceRevision="abc123",
        buildMode="release",
        createdAt="2024-01-01T00:00:00Z",
    )
    base.update(overrides)
    return FrozenArtifactRecord(**base)


def _write_record(directory, record):
    os.makedirs(directory, exist_ok=True)
    path = os.path.join(directory, "build-record.json")
    record.save(path)
    return path


class TestResolveVersion(unittest.TestCase):
    def setUp(self):
        self.tmpdir = tempfile.mkdtemp()

    def tearDown(self):
        shutil.rmtree(self.tmpdir, ignore_errors=True)

    def test_no_existing_dir_returns_publish(self):
        nonexistent = os.path.join(self.tmpdir, "does-not-exist")
        record = _make_record()
        result = resolve_version(nonexistent, record)
        self.assertEqual(result.kind, VersionResolutionKind.PUBLISH)
        self.assertIn("no existing directory", result.reason)

    def test_existing_dir_no_record_returns_publish(self):
        empty_dir = os.path.join(self.tmpdir, "empty-dir")
        os.makedirs(empty_dir)
        record = _make_record()
        result = resolve_version(empty_dir, record)
        self.assertEqual(result.kind, VersionResolutionKind.PUBLISH)
        self.assertIn("no build-record.json", result.reason)

    def test_same_version_same_identity_returns_reuse(self):
        existing_dir = os.path.join(self.tmpdir, "existing")
        record = _make_record(version="1.0.0")
        _write_record(existing_dir, record)
        new_record = _make_record(version="1.0.0")
        result = resolve_version(existing_dir, new_record)
        self.assertEqual(result.kind, VersionResolutionKind.REUSE)
        self.assertEqual(result.existing_dir, existing_dir)
        self.assertIn("reuse", result.reason.lower())

    def test_different_version_same_identity_returns_publish(self):
        existing_dir = os.path.join(self.tmpdir, "existing")
        old_record = _make_record(version="1.0.0")
        _write_record(existing_dir, old_record)
        new_record = _make_record(version="2.0.0")
        result = resolve_version(existing_dir, new_record)
        self.assertEqual(result.kind, VersionResolutionKind.PUBLISH)
        self.assertIn("version changed", result.reason)

    def test_identity_mismatch_component_id_returns_fail(self):
        existing_dir = os.path.join(self.tmpdir, "existing")
        old_record = _make_record(componentId="backend")
        _write_record(existing_dir, old_record)
        new_record = _make_record(componentId="qdrant")
        result = resolve_version(existing_dir, new_record)
        self.assertEqual(result.kind, VersionResolutionKind.FAIL)
        self.assertIn("identity mismatch", result.reason)

    def test_identity_mismatch_platform_returns_fail(self):
        existing_dir = os.path.join(self.tmpdir, "existing")
        old_record = _make_record(platform="linux")
        _write_record(existing_dir, old_record)
        new_record = _make_record(platform="android")
        result = resolve_version(existing_dir, new_record)
        self.assertEqual(result.kind, VersionResolutionKind.FAIL)
        self.assertIn("identity mismatch", result.reason)

    def test_identity_mismatch_architecture_returns_fail(self):
        existing_dir = os.path.join(self.tmpdir, "existing")
        old_record = _make_record(architecture="arm64")
        _write_record(existing_dir, old_record)
        new_record = _make_record(architecture="x86_64")
        result = resolve_version(existing_dir, new_record)
        self.assertEqual(result.kind, VersionResolutionKind.FAIL)
        self.assertIn("identity mismatch", result.reason)

    def test_identity_mismatch_artifact_type_returns_fail(self):
        existing_dir = os.path.join(self.tmpdir, "existing")
        old_record = _make_record(artifactType="executable")
        _write_record(existing_dir, old_record)
        new_record = _make_record(artifactType="archive")
        result = resolve_version(existing_dir, new_record)
        self.assertEqual(result.kind, VersionResolutionKind.FAIL)
        self.assertIn("identity mismatch", result.reason)

    def test_corrupt_build_record_returns_publish(self):
        existing_dir = os.path.join(self.tmpdir, "corrupt")
        os.makedirs(existing_dir)
        with open(os.path.join(existing_dir, "build-record.json"), "w") as f:
            f.write("{not valid json")
        record = _make_record()
        result = resolve_version(existing_dir, record)
        self.assertEqual(result.kind, VersionResolutionKind.PUBLISH)
        self.assertIn("failed to read", result.reason)

    def test_resolution_has_existing_dir(self):
        existing_dir = os.path.join(self.tmpdir, "existing")
        record = _make_record(version="1.0.0")
        _write_record(existing_dir, record)
        new_record = _make_record(version="1.0.0")
        result = resolve_version(existing_dir, new_record)
        self.assertEqual(result.existing_dir, existing_dir)

    def test_resolution_has_reason(self):
        nonexistent = os.path.join(self.tmpdir, "nope")
        record = _make_record()
        result = resolve_version(nonexistent, record)
        self.assertTrue(len(result.reason) > 0)


if __name__ == "__main__":
    unittest.main()
