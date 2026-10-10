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
    struct.pack_into("<H", data, 16, 2)
    struct.pack_into("<H", data, 18, machine)
    struct.pack_into("<I", data, 20, 1)
    struct.pack_into("<H", data, 52, 64)
    struct.pack_into("<Q", data, 32, 64)
    struct.pack_into("<HH", data, 54, 56, 2 if interpreter else 1)
    struct.pack_into("<II", data, 64, 1, 5)
    struct.pack_into("<QQ", data, 96, len(data), len(data))
    if interpreter:
        struct.pack_into("<I", data, 120, 3)
        struct.pack_into("<Q", data, 128, 192)
        struct.pack_into("<Q", data, 152, len(interpreter) + 1)
        data[192:192 + len(interpreter)] = interpreter
    return bytes(data)


def elf_with_dependency(name):
    data = bytearray(1024)
    data[:6] = b"\x7fELF\x02\x01"
    struct.pack_into("<H", data, 16, 2)
    struct.pack_into("<H", data, 18, 183)
    struct.pack_into("<I", data, 20, 1)
    struct.pack_into("<H", data, 52, 64)
    struct.pack_into("<Q", data, 32, 64)
    struct.pack_into("<HH", data, 54, 56, 3)
    struct.pack_into("<II", data, 64, 1, 5)
    struct.pack_into("<QQ", data, 72, 0, 0)
    struct.pack_into("<Q", data, 96, len(data))
    struct.pack_into("<Q", data, 104, len(data))
    struct.pack_into("<I", data, 120, 2)
    struct.pack_into("<QQ", data, 128, 300, 300)
    struct.pack_into("<Q", data, 152, 48)
    loader = b"/lib/ld-musl-aarch64.so.1\0"
    struct.pack_into("<I", data, 176, 3)
    struct.pack_into("<QQ", data, 184, 256, 256)
    struct.pack_into("<Q", data, 208, len(loader))
    data[256:256 + len(loader)] = loader
    struct.pack_into("<qQqQ", data, 300, 5, 600, 1, 0)
    data[600:600 + len(name) + 1] = name.encode() + b"\0"
    return bytes(data)


class RootfsInputsTests(unittest.TestCase):
    def prepare_variant(self, node_binary, qdrant_binary, library=None):
        with tempfile.TemporaryDirectory() as directory:
            root = pathlib.Path(directory)
            with tarfile.open(root / "alpine.tar.gz", "w:gz") as archive:
                MODULE.add_bytes(archive, "lib/ld-musl-aarch64.so.1", elf())
                if library is not None:
                    MODULE.add_bytes(archive, "usr/lib/librequired.so", library)
            with tarfile.open(root / "node.tar.gz", "w:gz") as archive:
                MODULE.add_bytes(archive, "node/bin/node", node_binary, 0o755)
            with tarfile.open(root / "qdrant.tar.gz", "w:gz") as archive:
                if qdrant_binary is not None:
                    MODULE.add_bytes(archive, "qdrant", qdrant_binary, 0o755)
            (root / "core").write_bytes(elf())
            (root / "sources").write_bytes(b"{}")
            MODULE.prepare(root / "alpine.tar.gz", root / "node.tar.gz", root / "core", root / "sources", "3.21.0", root / "guest.tar.gz", root / "manifest", root / "qdrant.tar.gz")

    def test_missing_qdrant_and_wrong_arch_refuse_package(self):
        node = elf(b"/lib/ld-musl-aarch64.so.1")
        with self.assertRaisesRegex(ValueError, "Qdrant binary missing"):
            self.prepare_variant(node, None)
        with self.assertRaisesRegex(ValueError, "ARM64"):
            self.prepare_variant(node, elf(machine=62))

    def test_runtime_dependency_must_resolve_to_real_arm64_library(self):
        node = elf_with_dependency("librequired.so")
        self.assertEqual(MODULE.elf_dependencies(node), ["librequired.so"])
        with self.assertRaisesRegex(ValueError, "dependency missing"):
            self.prepare_variant(node, elf())
        with self.assertRaisesRegex(ValueError, "ARM64"):
            self.prepare_variant(node, elf(), elf(machine=62))
        self.prepare_variant(node, elf(), elf())

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
                MODULE.add_bytes(archive, "lib/ld-musl-aarch64.so.1", elf())
                link = tarfile.TarInfo("lib/libc.musl-aarch64.so.1")
                link.type, link.linkname = tarfile.SYMTYPE, "ld-musl-aarch64.so.1"
                archive.addfile(link)
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
            stored = next(item for item in manifest["runtimeLibraries"] if item["path"] == "/lib/libc.musl-aarch64.so.1")
            self.assertEqual(stored["hostStoredSha256"], MODULE.digest(b"ld-musl-aarch64.so.1"))
            self.assertEqual(stored["sha256"], MODULE.digest(elf()))
            self.assertEqual(manifest["sourceManifest"]["size"], (root / "sources").stat().st_size)

    def test_runtime_input_requires_real_hash_and_existing_file(self):
        with tempfile.TemporaryDirectory() as directory:
            path = pathlib.Path(directory) / "input"
            path.write_bytes(b"runtime archive")
            MODULE.verify_input(path, MODULE.digest(path.read_bytes()))
            for expected in ("", "0" * 64, "not-sha"):
                with self.assertRaises(ValueError):
                    MODULE.verify_input(path, expected)
            with self.assertRaises(FileNotFoundError):
                MODULE.verify_input(path.with_name("missing"), "0" * 64)

    def test_dependency_parser_rejects_glibc_runtime(self):
        with self.assertRaises(ValueError):
            MODULE.elf_dependencies(elf(b"/lib/ld-linux-aarch64.so.1"))
        self.assertEqual(MODULE.elf_dependencies(elf()), [])

    def test_package_hash_matches_final_zip_without_self_reference(self):
        with tempfile.TemporaryDirectory() as directory:
            root = pathlib.Path(directory)
            (root / "fake/data").mkdir(parents=True)
            (root / "fake/meta.db").write_bytes(b"test metadata")
            inputs = {"alpineSha256": "1" * 64, "nodeArchiveSha256": "2" * 64, "qdrantArchiveSha256": "3" * 64}
            (root / "manifest").write_text(json.dumps({"guestArch": "arm64", "runtimeInputs": inputs}))
            (root / "sources").write_text("{}")
            target = str(root / "bundle.zip")
            MODULE.package(root / "fake", root / "manifest", root / "sources", target, root / "release")
            release = json.loads((root / "release").read_text())
            self.assertEqual(release["sha256"], MODULE.digest(pathlib.Path(target).read_bytes()))
            self.assertEqual(release["runtimeInputs"], inputs)
            with zipfile.ZipFile(target) as archive:
                self.assertNotIn("packageSha256", json.loads(archive.read("rootfs.manifest.json")))
                self.assertIn("data/", archive.namelist())

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
