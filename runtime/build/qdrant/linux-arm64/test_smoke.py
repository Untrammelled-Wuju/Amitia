import json
import os
import pathlib
import socket
import struct
import sys
import tempfile
import unittest

SCRIPT_DIR = pathlib.Path(__file__).resolve().parent
sys.path.insert(0, str(SCRIPT_DIR))

import smoke_test


def free_port():
    with socket.socket(socket.AF_INET, socket.SOCK_STREAM) as s:
        s.bind(("127.0.0.1", 0))
        return s.getsockname()[1]


class TestEnvironmentCheck(unittest.TestCase):
    def test_returns_list(self):
        errs = smoke_test.check_env_errors()
        self.assertIsInstance(errs, list)


class TestConfigBuilding(unittest.TestCase):
    def test_minimal_config(self):
        config = smoke_test.build_config(6333, "/tmp/storage", "/tmp/snapshots")
        self.assertEqual(config["service"]["http_port"], 6333)
        self.assertEqual(config["service"]["host"], "127.0.0.1")
        self.assertEqual(config["storage"]["storage_path"], "/tmp/storage")
        self.assertEqual(config["storage"]["snapshots_path"], "/tmp/snapshots")
        self.assertTrue(config["service"]["enable_cors"] is False)
        self.assertTrue(config["telemetry_disabled"])


class TestPortSelection(unittest.TestCase):
    def test_port_valid(self):
        port = free_port()
        self.assertIsInstance(port, int)
        self.assertGreater(port, 0)
        self.assertLess(port, 65536)


class TestSmokeTestWithFakeServer(unittest.TestCase):
    def setUp(self):
        self.tmp = tempfile.mkdtemp()

    def tearDown(self):
        import shutil
        shutil.rmtree(self.tmp, ignore_errors=True)

    def test_build_config_json_serializable(self):
        config = smoke_test.build_config(6333, "/tmp/storage", "/tmp/snapshots")
        json_str = json.dumps(config)
        parsed = json.loads(json_str)
        self.assertEqual(parsed["service"]["http_port"], 6333)


if __name__ == "__main__":
    unittest.main()
