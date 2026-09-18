import json
import os
import sys
import tempfile
import unittest
from pathlib import Path

SCRIPT_DIR = Path(__file__).resolve().parent
sys.path.insert(0, str(SCRIPT_DIR))

from rootfs_validator import (
    compute_files_sha256,
    compute_tree_sha,
    validate_apt_cache_clean,
    validate_ca_certificates,
    validate_compiler_toolchain_absent,
    validate_dpkg_database_preserved,
    validate_forbidden_executables,
    validate_guest_layout_dirs,
    validate_loader_exists,
    validate_lock_format,
    validate_machine_id_clean,
    validate_merged_usr,
    validate_no_backend_bundled,
    validate_no_local_token,
    validate_no_node_bundled,
    validate_no_qdrant_bundled,
    validate_no_user_data,
    validate_required_dirs,
    validate_required_libraries,
    validate_required_paths,
    validate_secret_scan,
    validate_shell_history_clean,
    validate_ssh_host_keys_clean,
    validate_tmp_clean,
)


def _can_create_symlinks() -> bool:
    """Check if the current platform/user can create symlinks."""
    if os.name != "nt":
        return True
    with tempfile.TemporaryDirectory() as tmpdir:
        test_link = Path(tmpdir) / "test_link"
        test_target = Path(tmpdir) / "test_target"
        test_target.mkdir()
        try:
            os.symlink(str(test_target), str(test_link))
            return test_link.is_symlink()
        except OSError:
            return False
        finally:
            if test_link.is_symlink():
                test_link.unlink()


SYMLINK_SUPPORTED = _can_create_symlinks()


def _create_minimal_valid_rootfs(base: Path) -> None:
    import shutil
    for d in ["boot", "dev", "etc", "home", "media", "mnt",
              "opt", "proc", "root", "run", "srv", "sys", "tmp", "usr", "var"]:
        (base / d).mkdir(parents=True, exist_ok=True)
    (base / "usr" / "bin").mkdir(parents=True, exist_ok=True)
    (base / "usr" / "sbin").mkdir(parents=True, exist_ok=True)
    (base / "usr" / "lib").mkdir(parents=True, exist_ok=True)
    for link_name, target in [("bin", "usr/bin"), ("sbin", "usr/sbin"), ("lib", "usr/lib")]:
        link_path = base / link_name
        if link_path.is_symlink():
            link_path.unlink()
        elif link_path.exists():
            shutil.rmtree(link_path)
        try:
            os.symlink(target, link_path)
        except OSError:
            link_path.mkdir(parents=True, exist_ok=True)
    var_run = base / "var" / "run"
    if var_run.exists():
        if var_run.is_symlink():
            var_run.unlink()
        else:
            shutil.rmtree(var_run)
    try:
        os.symlink("../run", var_run)
    except OSError:
        var_run.mkdir(parents=True, exist_ok=True)
    (base / "usr" / "bin" / "sh").touch()
    (base / "usr" / "bin" / "env").touch()
    (base / "etc" / "ssl" / "certs").mkdir(parents=True, exist_ok=True)
    (base / "etc" / "ssl" / "certs" / "ca-certificates.crt").touch()
    (base / "usr" / "lib" / "ld-linux-aarch64.so.1").touch()
    (base / "usr" / "lib" / "libstdc++.so.6").touch()
    (base / "usr" / "lib" / "libgcc_s.so.1").touch()
    (base / "usr" / "lib" / "libpthread.so.0").touch()
    (base / "usr" / "lib" / "libdl.so.2").touch()
    (base / "usr" / "lib" / "librt.so.1").touch()
    (base / "usr" / "lib" / "libm.so.6").touch()
    (base / "opt" / "amitia").mkdir(parents=True, exist_ok=True)
    (base / "etc" / "amitia").mkdir(parents=True, exist_ok=True)
    (base / "var" / "lib" / "amitia").mkdir(parents=True, exist_ok=True)
    (base / "var" / "cache" / "amitia").mkdir(parents=True, exist_ok=True)
    (base / "var" / "log" / "amitia").mkdir(parents=True, exist_ok=True)
    (base / "run" / "amitia").mkdir(parents=True, exist_ok=True)
    (base / "home" / "amitia").mkdir(parents=True, exist_ok=True)
    (base / "var" / "lib" / "dpkg").mkdir(parents=True, exist_ok=True)
    machine_id = base / "etc" / "machine-id"
    machine_id.write_text("", encoding="utf-8")


class TestLockValidation(unittest.TestCase):
    def test_valid_lock(self):
        lock = {
            "schemaVersion": 1,
            "component": "ubuntu-rootfs",
            "distribution": "ubuntu",
            "release": "24.04.4",
            "architecture": "arm64",
            "archiveFileName": "ubuntu-base-24.04.4-base-arm64.tar.gz",
            "sourceUrl": "https://cdimage.ubuntu.com/ubuntu-base/releases/24.04.4/release/ubuntu-base-24.04.4-base-arm64.tar.gz",
            "sha256": "04207713ece899c3740823d33690441ad3a7f0ded1101aca744e2b0f37ac7ff2",
        }
        errors = validate_lock_format(lock)
        self.assertEqual(errors, [])

    def test_missing_lock_fails(self):
        errors = validate_lock_format({})
        self.assertTrue(len(errors) > 0)

    def test_invalid_sha_fails(self):
        lock = {
            "schemaVersion": 1,
            "component": "ubuntu-rootfs",
            "distribution": "ubuntu",
            "release": "24.04.4",
            "architecture": "arm64",
            "archiveFileName": "test.tar.gz",
            "sourceUrl": "https://cdimage.ubuntu.com/test.tar.gz",
            "sha256": "invalid",
        }
        errors = validate_lock_format(lock)
        self.assertTrue(any("sha256" in e.lower() for e in errors))

    def test_latest_in_url_fails(self):
        lock = {
            "schemaVersion": 1,
            "component": "ubuntu-rootfs",
            "distribution": "ubuntu",
            "release": "24.04.4",
            "architecture": "arm64",
            "archiveFileName": "latest.tar.gz",
            "sourceUrl": "https://cdimage.ubuntu.com/ubuntu-base/releases/latest/release/ubuntu-base-latest-arm64.tar.gz",
            "sha256": "04207713ece899c3740823d33690441ad3a7f0ded1101aca744e2b0f37ac7ff2",
        }
        errors = validate_lock_format(lock)
        self.assertTrue(any("latest" in e.lower() for e in errors))


class TestRootfsStructureValidation(unittest.TestCase):
    def test_missing_required_dir_fails(self):
        with tempfile.TemporaryDirectory() as tmpdir:
            root = Path(tmpdir)
            errors = validate_required_dirs(root)
            self.assertTrue(len(errors) > 0)
            self.assertTrue(any("bin" in e for e in errors))

    def test_missing_required_path_fails(self):
        with tempfile.TemporaryDirectory() as tmpdir:
            root = Path(tmpdir)
            (root / "bin").mkdir(parents=True, exist_ok=True)
            errors = validate_required_paths(root)
            self.assertTrue(any("/bin/sh" in e for e in errors))

    def test_missing_loader_fails(self):
        with tempfile.TemporaryDirectory() as tmpdir:
            root = Path(tmpdir)
            errors = validate_loader_exists(root)
            self.assertTrue(any("loader" in e.lower() for e in errors))

    def test_missing_ca_certificates_fails(self):
        with tempfile.TemporaryDirectory() as tmpdir:
            root = Path(tmpdir)
            errors = validate_ca_certificates(root)
            self.assertTrue(any("CA" in e for e in errors))

    def test_missing_guest_dir_fails(self):
        with tempfile.TemporaryDirectory() as tmpdir:
            root = Path(tmpdir)
            errors = validate_guest_layout_dirs(root)
            self.assertTrue(len(errors) > 0)

    @unittest.skipUnless(SYMLINK_SUPPORTED, "Symlinks not supported - cannot build valid merged-usr rootfs")
    def test_happy_rootfs_passes(self):
        with tempfile.TemporaryDirectory() as tmpdir:
            root = Path(tmpdir)
            _create_minimal_valid_rootfs(root)
            self.assertEqual(validate_required_dirs(root), [])
            self.assertEqual(validate_required_paths(root), [])
            self.assertEqual(validate_loader_exists(root), [])
            self.assertEqual(validate_guest_layout_dirs(root), [])
            self.assertEqual(validate_ca_certificates(root), [])


class TestForbiddenComponentScanning(unittest.TestCase):
    def test_bundled_node_fails(self):
        with tempfile.TemporaryDirectory() as tmpdir:
            root = Path(tmpdir)
            (root / "usr" / "bin").mkdir(parents=True, exist_ok=True)
            (root / "usr" / "bin" / "node").touch()
            errors = validate_no_node_bundled(root)
            self.assertTrue(any("Node" in e for e in errors))

    def test_bundled_qdrant_fails(self):
        with tempfile.TemporaryDirectory() as tmpdir:
            root = Path(tmpdir)
            (root / "opt" / "amitia" / "qdrant").mkdir(parents=True, exist_ok=True)
            errors = validate_no_qdrant_bundled(root)
            self.assertTrue(any("Qdrant" in e for e in errors))

    def test_bundled_backend_fails(self):
        with tempfile.TemporaryDirectory() as tmpdir:
            root = Path(tmpdir)
            (root / "opt").mkdir(parents=True, exist_ok=True)
            (root / "opt" / "amitia-server").touch()
            errors = validate_no_backend_bundled(root)
            self.assertTrue(any("backend" in e.lower() for e in errors))

    def test_sshd_fails(self):
        with tempfile.TemporaryDirectory() as tmpdir:
            root = Path(tmpdir)
            (root / "usr" / "sbin").mkdir(parents=True, exist_ok=True)
            (root / "usr" / "sbin" / "sshd").touch()
            errors = validate_forbidden_executables(root)
            self.assertTrue(any("sshd" in e for e in errors))

    def test_sudo_fails(self):
        with tempfile.TemporaryDirectory() as tmpdir:
            root = Path(tmpdir)
            (root / "usr" / "bin").mkdir(parents=True, exist_ok=True)
            (root / "usr" / "bin" / "sudo").touch()
            errors = validate_forbidden_executables(root)
            self.assertTrue(any("sudo" in e for e in errors))

    def test_compiler_fails(self):
        with tempfile.TemporaryDirectory() as tmpdir:
            root = Path(tmpdir)
            (root / "usr" / "bin").mkdir(parents=True, exist_ok=True)
            (root / "usr" / "bin" / "gcc").touch()
            errors = validate_compiler_toolchain_absent(root)
            self.assertTrue(any("gcc" in e for e in errors))

    def test_secret_fixture_fails(self):
        with tempfile.TemporaryDirectory() as tmpdir:
            root = Path(tmpdir)
            (root / "etc").mkdir(parents=True, exist_ok=True)
            (root / "etc" / "test.conf").write_text("token=abc123secret\n", encoding="utf-8")
            errors = validate_secret_scan(root)
            self.assertTrue(len(errors) > 0)

    def test_machine_id_not_clean_fails(self):
        with tempfile.TemporaryDirectory() as tmpdir:
            root = Path(tmpdir)
            (root / "etc").mkdir(parents=True, exist_ok=True)
            (root / "etc" / "machine-id").write_text("abc123def456\n", encoding="utf-8")
            errors = validate_machine_id_clean(root)
            self.assertTrue(any("Machine ID" in e for e in errors))

    def test_ssh_host_keys_present_fails(self):
        with tempfile.TemporaryDirectory() as tmpdir:
            root = Path(tmpdir)
            (root / "etc" / "ssh").mkdir(parents=True, exist_ok=True)
            (root / "etc" / "ssh" / "ssh_host_rsa_key").touch()
            errors = validate_ssh_host_keys_clean(root)
            self.assertTrue(any("ssh_host" in e for e in errors))

    def test_shell_history_not_clean_fails(self):
        with tempfile.TemporaryDirectory() as tmpdir:
            root = Path(tmpdir)
            (root / "root").mkdir(parents=True, exist_ok=True)
            (root / "root" / ".bash_history").write_text("ls\ncd ~\n", encoding="utf-8")
            errors = validate_shell_history_clean(root)
            self.assertTrue(any("history" in e.lower() for e in errors))


class TestCleanupAndIsolation(unittest.TestCase):
    def test_tmp_not_clean_fails(self):
        with tempfile.TemporaryDirectory() as tmpdir:
            root = Path(tmpdir)
            (root / "tmp").mkdir(parents=True, exist_ok=True)
            (root / "tmp" / "garbage.txt").touch()
            errors = validate_tmp_clean(root)
            self.assertTrue(any("Temp" in e for e in errors))

    def test_apt_cache_not_clean_fails(self):
        with tempfile.TemporaryDirectory() as tmpdir:
            root = Path(tmpdir)
            (root / "var" / "cache" / "apt" / "archives").mkdir(parents=True, exist_ok=True)
            (root / "var" / "cache" / "apt" / "archives" / "test.deb").touch()
            errors = validate_apt_cache_clean(root)
            self.assertTrue(any("APT" in e for e in errors))

    def test_dpkg_database_missing_fails(self):
        with tempfile.TemporaryDirectory() as tmpdir:
            root = Path(tmpdir)
            errors = validate_dpkg_database_preserved(root)
            self.assertTrue(any("dpkg" in e.lower() for e in errors))


class TestHashComputation(unittest.TestCase):
    def test_tree_sha_deterministic(self):
        with tempfile.TemporaryDirectory() as tmpdir:
            root = Path(tmpdir)
            (root / "etc").mkdir(parents=True, exist_ok=True)
            (root / "etc" / "test.txt").write_text("hello\n", encoding="utf-8")
            sha1 = compute_tree_sha(str(root))
            sha2 = compute_tree_sha(str(root))
            self.assertEqual(sha1, sha2)

    def test_files_sha_deterministic(self):
        with tempfile.TemporaryDirectory() as tmpdir:
            root = Path(tmpdir)
            (root / "etc").mkdir(parents=True, exist_ok=True)
            (root / "etc" / "test.txt").write_text("hello\n", encoding="utf-8")
            sha1, _ = compute_files_sha256(str(root))
            sha2, _ = compute_files_sha256(str(root))
            self.assertEqual(sha1, sha2)

    def test_different_content_different_sha(self):
        with tempfile.TemporaryDirectory() as tmpdir1:
            with tempfile.TemporaryDirectory() as tmpdir2:
                root1 = Path(tmpdir1)
                root2 = Path(tmpdir2)
                (root1 / "etc").mkdir(parents=True, exist_ok=True)
                (root2 / "etc").mkdir(parents=True, exist_ok=True)
                (root1 / "etc" / "test.txt").write_text("hello\n", encoding="utf-8")
                (root2 / "etc" / "test.txt").write_text("world\n", encoding="utf-8")
                sha1 = compute_tree_sha(str(root1))
                sha2 = compute_tree_sha(str(root2))
                self.assertNotEqual(sha1, sha2)


class TestMergedUsr(unittest.TestCase):
    def test_missing_merged_usr_fails(self):
        with tempfile.TemporaryDirectory() as tmpdir:
            root = Path(tmpdir)
            errors = validate_merged_usr(root)
            self.assertTrue(len(errors) > 0)

    @unittest.skipUnless(SYMLINK_SUPPORTED, "Symlinks not supported on this platform")
    def test_correct_merged_usr_passes(self):
        with tempfile.TemporaryDirectory() as tmpdir:
            root = Path(tmpdir)
            (root / "usr" / "bin").mkdir(parents=True, exist_ok=True)
            (root / "usr" / "sbin").mkdir(parents=True, exist_ok=True)
            (root / "usr" / "lib").mkdir(parents=True, exist_ok=True)
            for link_name, target in [("bin", "usr/bin"), ("sbin", "usr/sbin"), ("lib", "usr/lib")]:
                link_path = root / link_name
                if link_path.is_symlink():
                    link_path.unlink()
                os.symlink(target, link_path)
            errors = validate_merged_usr(root)
            self.assertEqual(errors, [])


class TestRequiredLibraries(unittest.TestCase):
    def test_missing_libstdcxx_fails(self):
        with tempfile.TemporaryDirectory() as tmpdir:
            root = Path(tmpdir)
            (root / "usr" / "lib").mkdir(parents=True, exist_ok=True)
            errors = validate_required_libraries(root)
            self.assertTrue(any("libstdc++" in e for e in errors))

    def test_all_required_libs_present_passes(self):
        with tempfile.TemporaryDirectory() as tmpdir:
            root = Path(tmpdir)
            (root / "usr" / "lib").mkdir(parents=True, exist_ok=True)
            (root / "usr" / "lib" / "libstdc++.so.6").touch()
            (root / "usr" / "lib" / "libgcc_s.so.1").touch()
            (root / "usr" / "lib" / "libpthread.so.0").touch()
            (root / "usr" / "lib" / "libdl.so.2").touch()
            (root / "usr" / "lib" / "librt.so.1").touch()
            (root / "usr" / "lib" / "libm.so.6").touch()
            errors = validate_required_libraries(root)
            self.assertEqual(errors, [])


if __name__ == "__main__":
    unittest.main()
