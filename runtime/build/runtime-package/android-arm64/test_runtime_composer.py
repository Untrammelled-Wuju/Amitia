import hashlib
import io
import os
import pathlib
import shutil
import sys
import tarfile
import tempfile
import unittest

sys.path.insert(0, os.path.dirname(os.path.abspath(__file__)))
import runtime_composer as rtc


def make_component_tar(files_map):
    tmp = tempfile.NamedTemporaryFile(suffix=".tar.xz", delete=False)
    tmp.close()
    import lzma
    filters = [{"id": lzma.FILTER_LZMA2, "preset": 5}]
    with lzma.open(tmp.name, "wb", format=lzma.FORMAT_XZ, check=lzma.CHECK_SHA256, filters=filters) as xz_f:
        with tarfile.open(fileobj=xz_f, mode="w:") as tf:
            for name, data in files_map.items():
                info = tarfile.TarInfo(name=name)
                if isinstance(data, str):
                    data = data.encode("utf-8")
                info.size = len(data)
                info.mode = 0o755 if name.endswith(".sh") else 0o644
                info.uid = 0
                info.gid = 0
                info.mtime = 0
                if name.endswith("/") or not name:
                    info.type = tarfile.DIRTYPE
                else:
                    info.type = tarfile.REGTYPE
                if info.type == tarfile.REGTYPE:
                    tf.addfile(info, io.BytesIO(data))
    return tmp.name


def make_host_files(base_dir, dist_files):
    results = []
    for name, content in dist_files.items():
        fp = pathlib.Path(base_dir) / name
        fp.parent.mkdir(parents=True, exist_ok=True)
        fp.write_text(content, encoding="utf-8")
        results.append((name, fp))
    return results


class TestRuntimeComposer(unittest.TestCase):
    def setUp(self):
        self.tmpdir = tempfile.mkdtemp(prefix="rtc_test_")

    def tearDown(self):
        shutil.rmtree(self.tmpdir, ignore_errors=True)

    def _setup_manifest(self):
        manifest_dir = pathlib.Path(self.tmpdir) / "manifest"
        manifest_dir.mkdir(parents=True, exist_ok=True)
        (manifest_dir / "guest-layout.json").write_text("{}", encoding="utf-8")
        (manifest_dir / "mount-contract.json").write_text("{}", encoding="utf-8")
        return manifest_dir

    def test_basic_compose(self):
        backend_tar = make_component_tar({"amitia-server": b"FAKEBIN"})
        node_tar = make_component_tar({
            "bin/node": b"NODEBIN",
            "lib/node_modules/npm/bin/npm-cli.js": b"NPM",
            "lib/node_modules/npm/bin/npx-cli.js": b"NPX",
        })
        scripts_tar = make_component_tar({
            "node/amitia-node-prepare.sh": b"#!/bin/sh\necho ok",
            "node/amitia-node-probe.sh": b"#!/bin/sh\necho probe",
            "node/amitia-node-exec.sh": b"#!/bin/sh\necho exec",
            "node/amitia-npm-exec.sh": b"#!/bin/sh\necho npm",
            "node/amitia-npx-exec.sh": b"#!/bin/sh\necho npx",
            "node/amitia-plugin-host.sh": b"#!/bin/sh\necho plugin",
            "node/amitia-task-host.sh": b"#!/bin/sh\necho task",
        })
        qdrant_tar = make_component_tar({"bin/qdrant": b"QBIN"})
        plugin_dir = tempfile.mkdtemp(prefix="plugin_")
        make_host_files(plugin_dir, {"dist/index.js": "module.exports={};"})
        task_dir = tempfile.mkdtemp(prefix="task_")
        make_host_files(task_dir, {"dist/index.js": "module.exports={};"})
        manifest_dir = self._setup_manifest()
        plugin_files = [("dist/index.js", pathlib.Path(plugin_dir) / "dist" / "index.js")]
        task_files = [("dist/index.js", pathlib.Path(task_dir) / "dist" / "index.js")]
        manifest_files = [
            ("guest-layout.json", manifest_dir / "guest-layout.json"),
            ("mount-contract.json", manifest_dir / "mount-contract.json"),
        ]
        out_path = os.path.join(self.tmpdir, "runtime.tar.xz")
        try:
            result_path, result_sha, members = rtc.compose_runtime_root(
                backend_tar, node_tar, scripts_tar, qdrant_tar,
                plugin_files, task_files, manifest_files,
                out_path
            )
            self.assertEqual(result_path, out_path)
            self.assertIsNotNone(result_sha)
            with tarfile.open(out_path, "r:xz") as tf:
                names = {m.name for m in tf.getmembers()}
            self.assertIn("backend/amitia-server", names)
            self.assertIn("node/bin/node", names)
            self.assertIn("node/lib/node_modules/npm/bin/npm-cli.js", names)
            self.assertIn("qdrant/bin/qdrant", names)
            self.assertIn("plugin-host/dist/index.js", names)
            self.assertIn("task-host/dist/index.js", names)
            self.assertIn("manifest/guest-layout.json", names)
            self.assertIn("manifest/mount-contract.json", names)
        finally:
            for p in [backend_tar, node_tar, scripts_tar, qdrant_tar]:
                if os.path.exists(p):
                    os.unlink(p)
            shutil.rmtree(plugin_dir, ignore_errors=True)
            shutil.rmtree(task_dir, ignore_errors=True)

    def test_missing_required_entry_fails(self):
        backend_tar = make_component_tar({"amitia-server": b"FAKEBIN"})
        node_tar = make_component_tar({"bin/node": b"NODEBIN"})
        scripts_tar = make_component_tar({})
        qdrant_tar = make_component_tar({"bin/qdrant": b"QBIN"})
        plugin_dir = tempfile.mkdtemp(prefix="plugin_")
        make_host_files(plugin_dir, {"dist/index.js": "ok"})
        task_dir = tempfile.mkdtemp(prefix="task_")
        make_host_files(task_dir, {"dist/index.js": "ok"})
        manifest_dir = self._setup_manifest()
        out_path = os.path.join(self.tmpdir, "runtime.tar.xz")
        try:
            with self.assertRaises(RuntimeError):
                rtc.compose_runtime_root(
                    backend_tar, node_tar, scripts_tar, qdrant_tar,
                    [("dist/index.js", pathlib.Path(plugin_dir)/"dist"/"index.js")],
                    [("dist/index.js", pathlib.Path(task_dir)/"dist"/"index.js")],
                    [("guest-layout.json", manifest_dir / "guest-layout.json")],
                    out_path
                )
        finally:
            for p in [backend_tar, node_tar, scripts_tar, qdrant_tar]:
                if os.path.exists(p):
                    os.unlink(p)
            shutil.rmtree(plugin_dir, ignore_errors=True)
            shutil.rmtree(task_dir, ignore_errors=True)

    def test_deterministic_output(self):
        backend_tar = make_component_tar({"amitia-server": b"FAKEBIN"})
        node_tar = make_component_tar({
            "bin/node": b"NODEBIN",
            "lib/node_modules/npm/bin/npm-cli.js": b"NPM",
            "lib/node_modules/npm/bin/npx-cli.js": b"NPX",
        })
        scripts_tar = make_component_tar({
            "node/amitia-node-prepare.sh": "#!/bin/sh\necho ok",
            "node/amitia-node-probe.sh": "#!/bin/sh\necho probe",
            "node/amitia-node-exec.sh": "#!/bin/sh\necho exec",
            "node/amitia-npm-exec.sh": "#!/bin/sh\necho npm",
            "node/amitia-npx-exec.sh": "#!/bin/sh\necho npx",
            "node/amitia-plugin-host.sh": "#!/bin/sh\necho plugin",
            "node/amitia-task-host.sh": "#!/bin/sh\necho task",
        })
        qdrant_tar = make_component_tar({"bin/qdrant": b"QBIN"})
        plugin_dir = tempfile.mkdtemp(prefix="plugin_")
        make_host_files(plugin_dir, {"dist/index.js": "module.exports={};"})
        task_dir = tempfile.mkdtemp(prefix="task_")
        make_host_files(task_dir, {"dist/index.js": "module.exports={};"})
        manifest_dir = self._setup_manifest()
        plugin_files = [("dist/index.js", pathlib.Path(plugin_dir)/"dist"/"index.js")]
        task_files = [("dist/index.js", pathlib.Path(task_dir)/"dist"/"index.js")]
        manifest_files = [
            ("guest-layout.json", manifest_dir / "guest-layout.json"),
            ("mount-contract.json", manifest_dir / "mount-contract.json"),
        ]
        out1 = os.path.join(self.tmpdir, "runtime1.tar.xz")
        out2 = os.path.join(self.tmpdir, "runtime2.tar.xz")
        try:
            p1, s1, _ = rtc.compose_runtime_root(
                backend_tar, node_tar, scripts_tar, qdrant_tar,
                plugin_files, task_files, manifest_files, out1
            )
            p2, s2, _ = rtc.compose_runtime_root(
                backend_tar, node_tar, scripts_tar, qdrant_tar,
                plugin_files, task_files, manifest_files, out2
            )
            self.assertEqual(s1, s2)
        finally:
            for p in [backend_tar, node_tar, scripts_tar, qdrant_tar]:
                if os.path.exists(p):
                    os.unlink(p)
            shutil.rmtree(plugin_dir, ignore_errors=True)
            shutil.rmtree(task_dir, ignore_errors=True)

    def test_forbidden_root_rejected(self):
        backend_tar = make_component_tar({"amitia-server": b"FAKEBIN"})
        node_tar = make_component_tar({
            "bin/node": b"NODEBIN",
            "lib/node_modules/npm/bin/npm-cli.js": b"NPM",
            "lib/node_modules/npm/bin/npx-cli.js": b"NPX",
        })
        scripts_tar = make_component_tar({
            "node/amitia-node-prepare.sh": "ok",
            "node/amitia-node-probe.sh": "ok",
            "node/amitia-node-exec.sh": "ok",
            "node/amitia-npm-exec.sh": "ok",
            "node/amitia-npx-exec.sh": "ok",
            "node/amitia-plugin-host.sh": "ok",
            "node/amitia-task-host.sh": "ok",
        })
        qdrant_tar = make_component_tar({"bin/qdrant": b"QBIN"})
        plugin_dir = tempfile.mkdtemp(prefix="plugin_")
        make_host_files(plugin_dir, {"dist/index.js": "ok"})
        task_dir = tempfile.mkdtemp(prefix="task_")
        make_host_files(task_dir, {"dist/index.js": "ok"})
        manifest_dir = self._setup_manifest()
        plugin_files = [("dist/index.js", pathlib.Path(plugin_dir)/"dist"/"index.js")]
        task_files = [("dist/index.js", pathlib.Path(task_dir)/"dist"/"index.js")]
        manifest_files = [
            ("guest-layout.json", manifest_dir / "guest-layout.json"),
            ("mount-contract.json", manifest_dir / "mount-contract.json"),
        ]
        out_path = os.path.join(self.tmpdir, "runtime.tar.xz")
        try:
            p, s, _ = rtc.compose_runtime_root(
                backend_tar, node_tar, scripts_tar, qdrant_tar,
                plugin_files, task_files, manifest_files, out_path
            )
            with tarfile.open(out_path, "r:xz") as tf:
                names = {m.name for m in tf.getmembers()}
            forbidden = {"config", "data", "cache", "logs", "run", "workspace", "workspaces", "tmp"}
            for name in names:
                first = name.split("/")[0]
                if first in forbidden:
                    self.fail(f"Forbidden root in runtime: {first}")
        finally:
            for p in [backend_tar, node_tar, scripts_tar, qdrant_tar]:
                if os.path.exists(p):
                    os.unlink(p)
            shutil.rmtree(plugin_dir, ignore_errors=True)
            shutil.rmtree(task_dir, ignore_errors=True)


if __name__ == "__main__":
    unittest.main()
