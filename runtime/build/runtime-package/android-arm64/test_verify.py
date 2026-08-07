import json
import os
import pathlib
import shutil
import sys
import tempfile
import unittest

sys.path.insert(0, os.path.dirname(os.path.abspath(__file__)))
import verify as vf


def _make_fake_host():
    base = tempfile.mkdtemp(prefix="fake_host_")
    dist = pathlib.Path(base) / "dist"
    dist.mkdir()
    (dist / "index.js").write_text("module.exports = {};", encoding="utf-8")
    return base


class TestInputVerification(unittest.TestCase):
    def test_valid_version_accepts(self):
        host_src = _make_fake_host()
        lock = {
            "components": {
                "pluginHost": {"source": host_src},
                "taskHost": {"source": host_src},
            }
        }
        issues = vf.verify_inputs(
            lock,
            "1.0.0",
            "ed7e2f0140b78bd3a5088227882a2e78dd4c98e9",
        )
        self.assertEqual(issues, [])
        shutil.rmtree(host_src, ignore_errors=True)

    def test_invalid_version_rejected(self):
        issues = vf.verify_inputs({"components": {}}, "", "ed7e2f0140b78bd3a5088227882a2e78dd4c98e9")
        self.assertTrue(any("version" in i.lower() for i in issues))

    def test_invalid_commit_rejected(self):
        issues = vf.verify_inputs({"components": {}}, "1.0.0", "abc")
        self.assertTrue(any("commit" in i.lower() for i in issues))


class TestRootfsVerification(unittest.TestCase):
    def test_rejects_program_file(self):
        import io
        import lzma
        import tarfile as tf_mod
        tmpdir = pathlib.Path(tempfile.gettempdir())
        tar_path = str(tmpdir / "bad_rootfs.tar.xz")
        bad_files = {"opt/amitia/backend/amitia-server": b"FAKE"}
        filters = [{"id": lzma.FILTER_LZMA2, "preset": 5}]
        with lzma.open(tar_path, "wb", format=lzma.FORMAT_XZ, check=lzma.CHECK_SHA256, filters=filters) as xz_f:
            with tf_mod.open(fileobj=xz_f, mode="w:") as tf:
                for name, data in bad_files.items():
                    info = tf_mod.TarInfo(name=name)
                    info.size = len(data)
                    tf.addfile(info, io.BytesIO(data))
        issues = vf.verify_rootfs_payload(pathlib.Path(tar_path))
        self.assertTrue(any("禁止" in i for i in issues))
        os.unlink(tar_path)


if __name__ == "__main__":
    unittest.main()
