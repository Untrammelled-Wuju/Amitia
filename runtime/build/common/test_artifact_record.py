import json
import os
import tempfile
import unittest

from common.artifact_record import (
    FrozenArtifactRecord,
    SHA256_HEX_PATTERN,
    load,
    save,
    validate,
)


def _make_valid_record(**overrides):
    base = dict(
        schemaVersion=1,
        componentId="backend",
        version="1.2.3",
        platform="linux",
        architecture="arm64",
        artifactType="executable",
        artifactRelativePath="backend/linux-arm64/1.2.3/amitia-server",
        artifactSha256="a" * 64,
        treeSha256="b" * 64,
        sourceRevision="abc123def456",
        buildMode="release",
        createdAt="2024-01-01T00:00:00Z",
    )
    base.update(overrides)
    return FrozenArtifactRecord(**base)


class TestFrozenArtifactRecordRoundTrip(unittest.TestCase):
    def setUp(self):
        self.tmpdir = tempfile.mkdtemp()

    def tearDown(self):
        import shutil
        shutil.rmtree(self.tmpdir, ignore_errors=True)

    def test_serialize_deserialize_integrity(self):
        record = _make_valid_record()
        path = os.path.join(self.tmpdir, "build-record.json")
        save(record, path)
        loaded = load(path)
        self.assertEqual(loaded.schemaVersion, record.schemaVersion)
        self.assertEqual(loaded.componentId, record.componentId)
        self.assertEqual(loaded.version, record.version)
        self.assertEqual(loaded.platform, record.platform)
        self.assertEqual(loaded.architecture, record.architecture)
        self.assertEqual(loaded.artifactType, record.artifactType)
        self.assertEqual(loaded.artifactRelativePath, record.artifactRelativePath)
        self.assertEqual(loaded.artifactSha256, record.artifactSha256)
        self.assertEqual(loaded.treeSha256, record.treeSha256)
        self.assertEqual(loaded.sourceRevision, record.sourceRevision)
        self.assertEqual(loaded.buildMode, record.buildMode)
        self.assertEqual(loaded.createdAt, record.createdAt)

    def test_json_roundtrip_preserves_all_fields(self):
        record = _make_valid_record()
        path = os.path.join(self.tmpdir, "roundtrip.json")
        record.save(path)
        with open(path, "r", encoding="utf-8") as f:
            raw = json.load(f)
        reconstructed = FrozenArtifactRecord(**raw)
        self.assertEqual(reconstructed.to_dict(), record.to_dict())

    def test_load_ignores_unknown_fields(self):
        record = _make_valid_record()
        path = os.path.join(self.tmpdir, "extra.json")
        record.save(path)
        with open(path, "r", encoding="utf-8") as f:
            data = json.load(f)
        data["extraField"] = "should-be-ignored"
        data["anotherExtra"] = 42
        with open(path, "w", encoding="utf-8") as f:
            json.dump(data, f)
        loaded = load(path)
        self.assertFalse(hasattr(loaded, "extraField"))
        self.assertEqual(loaded.componentId, "backend")


class TestValidate(unittest.TestCase):
    def test_valid_record_no_errors(self):
        record = _make_valid_record()
        errors = validate(record)
        self.assertEqual(errors, [])

    def test_missing_component_id(self):
        record = _make_valid_record(componentId="")
        errors = validate(record)
        self.assertIn("componentId is required", errors)

    def test_missing_version(self):
        record = _make_valid_record(version="")
        errors = validate(record)
        self.assertIn("version is required", errors)

    def test_missing_artifact_relative_path(self):
        record = _make_valid_record(artifactRelativePath="")
        errors = validate(record)
        self.assertIn("artifactRelativePath is required", errors)

    def test_missing_source_revision(self):
        record = _make_valid_record(sourceRevision="")
        errors = validate(record)
        self.assertIn("sourceRevision is required", errors)

    def test_invalid_sha256_too_short(self):
        record = _make_valid_record(artifactSha256="abc123")
        errors = validate(record)
        self.assertIn("artifactSha256 must be a valid 64-character hex SHA-256 digest", errors)

    def test_invalid_sha256_non_hex(self):
        record = _make_valid_record(artifactSha256="g" * 64)
        errors = validate(record)
        self.assertIn("artifactSha256 must be a valid 64-character hex SHA-256 digest", errors)

    def test_invalid_tree_sha256(self):
        record = _make_valid_record(treeSha256="not-a-hash")
        errors = validate(record)
        self.assertIn("treeSha256 must be a valid 64-character hex SHA-256 digest", errors)

    def test_invalid_platform(self):
        record = _make_valid_record(platform="windows")
        errors = validate(record)
        self.assertTrue(any("platform" in e for e in errors))

    def test_invalid_architecture(self):
        record = _make_valid_record(architecture="i386")
        errors = validate(record)
        self.assertTrue(any("architecture" in e for e in errors))

    def test_invalid_artifact_type(self):
        record = _make_valid_record(artifactType="library")
        errors = validate(record)
        self.assertTrue(any("artifactType" in e for e in errors))

    def test_invalid_build_mode(self):
        record = _make_valid_record(buildMode="profile")
        errors = validate(record)
        self.assertTrue(any("buildMode" in e for e in errors))

    def test_multiple_errors_reported(self):
        record = _make_valid_record(
            componentId="",
            version="",
            artifactSha256="bad",
            treeSha256="bad",
        )
        errors = validate(record)
        self.assertTrue(len(errors) >= 4)

    def test_sha256_pattern_uppercase_accepted(self):
        record = _make_valid_record(artifactSha256="A" * 64)
        errors = validate(record)
        self.assertNotIn("artifactSha256 must be a valid 64-character hex SHA-256 digest", errors)


class TestIdentityKey(unittest.TestCase):
    def test_identity_key_format(self):
        record = _make_valid_record(
            componentId="backend",
            platform="linux",
            architecture="arm64",
            artifactType="executable",
        )
        self.assertEqual(record.identity_key, "backend:linux:arm64:executable")

    def test_identity_key_differs_by_component(self):
        r1 = _make_valid_record(componentId="backend")
        r2 = _make_valid_record(componentId="qdrant")
        self.assertNotEqual(r1.identity_key, r2.identity_key)

    def test_identity_key_differs_by_platform(self):
        r1 = _make_valid_record(platform="linux")
        r2 = _make_valid_record(platform="android")
        self.assertNotEqual(r1.identity_key, r2.identity_key)

    def test_identity_key_differs_by_architecture(self):
        r1 = _make_valid_record(architecture="arm64")
        r2 = _make_valid_record(architecture="x86_64")
        self.assertNotEqual(r1.identity_key, r2.identity_key)

    def test_identity_key_differs_by_artifact_type(self):
        r1 = _make_valid_record(artifactType="executable")
        r2 = _make_valid_record(artifactType="archive")
        self.assertNotEqual(r1.identity_key, r2.identity_key)

    def test_identity_key_same_for_identical_fields(self):
        r1 = _make_valid_record()
        r2 = _make_valid_record()
        self.assertEqual(r1.identity_key, r2.identity_key)


if __name__ == "__main__":
    unittest.main()
