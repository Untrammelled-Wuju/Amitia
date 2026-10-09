import importlib.util
import io
import json
import pathlib
import struct
import tarfile
import tempfile
import unittest
import zipfile

SPEC = importlib.util.spec_from_file_location("rootfs_inputs", pathlib.Path(__file__).with_name("rootfs_inputs.py"))
MODULE = importlib.util.module_from_spec(SPEC)
SPEC.loader.exec_module(MODULE)


def elf(interpreter=None, machine=183):
    data = bytearray(256)
    data[:6] = b"\x7fELF\x02\x01"
    struct.pack_into("<H", data, 18, machine)
    struct.pack_into("<Q", data, 32, 64)
    struct.pack_into("<HH", data, 54, 56, 1 if interpreter else 0)
    if interpreter:
        struct.pack_into("<I", data, 64, 3)
        struct.pack_into("<Q", data, 72, 128)
        struct.pack_into("<Q", data, 96, len(interpreter) + 1)
        data[128:128 + len(interpreter)] = interpreter
    return bytes(data)


class RootfsInputsTests(unittest.TestCase):
    def test_rejects_wrong_arch_and_glibc_node(self):
        with self.assertRaises(ValueError):
            MODULE.verify_elf(elf(machine=62))
        with self.assertRaises(ValueError):
            MODULE.verify_elf(elf(b"/lib/ld-linux-aarch64.so.1"), musl=True)
        MODULE.verify_elf(elf(b"/lib/ld-musl-aarch64.so.1"), musl=True)

    def test_refuses_traversal_and_absolute_paths(self):
        for path in ("../outside", "/etc/passwd", "a/../../bad"):
            with self.assertRaises(ValueError):
                MODULE.safe_path(path)

    def test_guest_payload_has_real_binaries_and_distinct_ownership(self):
        with tempfile.TemporaryDirectory() as directory:
            root = pathlib.Path(directory)
            with tarfile.open(root / "alpine.tar.gz", "w:gz") as archive:
                MODULE.add_bytes(archive, "etc/passwd", b"root:x:0:0:root:/root:/bin/sh\n")
            with tarfile.open(root / "node.tar.gz", "w:gz") as archive:
                MODULE.add_bytes(archive, "node/bin/node", elf(b"/lib/ld-musl-aarch64.so.1"), 0o755)
            with tarfile.open(root / "qdrant.tar.gz", "w:gz") as archive:
                MODULE.add_bytes(archive, "qdrant", elf(), 0o755)
            (root / "core").write_bytes(elf())
            (root / "sources").write_bytes(b'{"inputs":[]}\n')
            MODULE.prepare(root / "alpine.tar.gz", root / "node.tar.gz", root / "core", root / "sources", "3.21.0", root / "guest.tar.gz", root / "manifest", root / "qdrant.tar.gz")
            with tarfile.open(root / "guest.tar.gz") as archive:
                self.assertEqual(archive.getmember("home/amitia").uid, 1000)
                self.assertEqual(archive.getmember("var/lib/amitia/data").mode, 0o700)
                self.assertEqual(archive.getmember("var/lib/amitia/data").uid, 0)
                self.assertIn(b"v3.21/main", archive.extractfile("etc/apk/repositories").read())
                self.assertEqual(archive.extractfile(MODULE.CORE_PATH[1:]).read(), elf())
            manifest = json.loads((root / "manifest").read_text())
            self.assertEqual(manifest["guestArch"], "arm64")
            self.assertNotIn("packageSha256", manifest)
            self.assertEqual(manifest["qdrant"]["path"], MODULE.QDRANT_PATH)

    def test_package_hash_matches_final_zip_without_self_reference(self):
        with tempfile.TemporaryDirectory() as directory:
            root = pathlib.Path(directory)
            (root / "fake/data").mkdir(parents=True)
            (root / "fake/meta.db").write_bytes(b"test metadata")
            (root / "manifest").write_text('{"guestArch":"arm64"}')
            (root / "sources").write_text("{}")
            target = str(root / "bundle.zip")
            MODULE.package(root / "fake", root / "manifest", root / "sources", target, root / "release")
            release = json.loads((root / "release").read_text())
            self.assertEqual(release["sha256"], MODULE.digest(pathlib.Path(target).read_bytes()))
            with zipfile.ZipFile(target) as archive:
                self.assertNotIn("packageSha256", json.loads(archive.read("rootfs.manifest.json")))

    def test_source_manifest_tracks_embedded_files_and_detects_changes(self):
        with tempfile.TemporaryDirectory() as directory:
            root = pathlib.Path(directory)
            for name in ("go.mod", "go.sum", "main.go", "baseline.sql"):
                (root / name).write_text(name)
            package = {"Dir": directory, "ImportPath": "amitia/core", "GoFiles": ["main.go"], "EmbedFiles": ["baseline.sql"]}
            (root / "packages").write_text(json.dumps(package))
            MODULE.source_manifest(root / "packages", root, root / "before")
            (root / "baseline.sql").write_text("changed")
            MODULE.source_manifest(root / "packages", root, root / "after")
            self.assertNotEqual((root / "before").read_bytes(), (root / "after").read_bytes())
            self.assertEqual(len(json.loads((root / "before").read_text())["inputs"]), 4)


if __name__ == "__main__":
    unittest.main()
