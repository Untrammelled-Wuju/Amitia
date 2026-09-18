import os
import sys
import tempfile
import unittest

sys.path.insert(0, os.path.dirname(os.path.abspath(__file__)))


class TestCheck(unittest.TestCase):
    def test_check_passed(self):
        from report import Check, CheckStatus
        check = Check(id="test.check", status=CheckStatus.PASSED.value)
        self.assertEqual(check.status, "passed")
        data = check.to_safe_dict()
        self.assertEqual(data["status"], "passed")

    def test_check_failed(self):
        from report import Check, CheckStatus
        check = Check(id="test.fail", status=CheckStatus.FAILED.value, errorMessage="something broke")
        self.assertEqual(check.status, "failed")
        data = check.to_safe_dict()
        self.assertEqual(data["errorMessage"], "something broke")

    def test_check_skipped(self):
        from report import Check, CheckStatus
        check = Check(id="test.skip", status=CheckStatus.SKIPPED.value)
        self.assertEqual(check.status, "skipped")

    def test_check_not_applicable(self):
        from report import Check, CheckStatus
        check = Check(id="test.na", status=CheckStatus.NOT_APPLICABLE.value)
        self.assertEqual(check.status, "not_applicable")

    def test_check_serialization(self):
        from report import Check
        check = Check(id="serialization.test", status="passed", durationMs=123, details={"foo": "bar"})
        data = check.to_safe_dict()
        self.assertEqual(data["id"], "serialization.test")
        self.assertEqual(data["durationMs"], 123)
        self.assertEqual(data["details"]["foo"], "bar")


class TestValidationReport(unittest.TestCase):
    def test_all_passed(self):
        from report import ValidationReport, Check, CheckStatus
        report = ValidationReport(runtimeVersion="0.1.0")
        report.add_check(Check(id="a", status=CheckStatus.PASSED.value))
        report.add_check(Check(id="b", status=CheckStatus.PASSED.value))
        report.finalize()
        self.assertEqual(report.result, "passed")

    def test_one_failed_overall_failed(self):
        from report import ValidationReport, Check, CheckStatus
        report = ValidationReport(runtimeVersion="0.1.0")
        report.add_check(Check(id="a", status=CheckStatus.PASSED.value))
        report.add_check(Check(id="b", status=CheckStatus.FAILED.value))
        report.finalize()
        self.assertEqual(report.result, "failed")

    def test_skipped_does_not_fail(self):
        from report import ValidationReport, Check, CheckStatus
        report = ValidationReport(runtimeVersion="0.1.0")
        report.add_check(Check(id="a", status=CheckStatus.SKIPPED.value))
        report.add_check(Check(id="b", status=CheckStatus.PASSED.value))
        report.finalize()
        self.assertEqual(report.result, "passed")

    def test_not_applicable_does_not_fail(self):
        from report import ValidationReport, Check, CheckStatus
        report = ValidationReport(runtimeVersion="0.1.0")
        report.add_check(Check(id="a", status=CheckStatus.NOT_APPLICABLE.value))
        report.add_check(Check(id="b", status=CheckStatus.PASSED.value))
        report.finalize()
        self.assertEqual(report.result, "passed")

    def test_commit_not_exposed(self):
        from report import ValidationReport, Check, CheckStatus
        report = ValidationReport(runtimeVersion="0.1.0")
        report.add_check(Check(id="a", status=CheckStatus.PASSED.value))
        report.finalize()
        data = report.to_safe_dict()
        self.assertEqual(data["sourceCommit"], "")

    def test_deterministic_order(self):
        from report import ValidationReport, Check, CheckStatus
        report = ValidationReport(runtimeVersion="0.1.0")
        for i in range(5):
            report.add_check(Check(id=f"check.{i}", status=CheckStatus.PASSED.value))
        data = report.to_safe_dict()
        ids = [c["id"] for c in data["checks"]]
        self.assertEqual(ids, ["check.0", "check.1", "check.2", "check.3", "check.4"])

    def test_sensitive_field_redaction(self):
        from report import Check
        check = Check(id="sensitive.test", status="passed",
                      details={"api_key": "secret123", "normal": "visible"})
        data = check.to_safe_dict()
        self.assertEqual(data["details"]["api_key"], "[REDACTED]")
        self.assertEqual(data["details"]["normal"], "visible")

    def test_token_value_redacted(self):
        from report import Check
        check = Check(id="token.test", status="passed",
                      details={"X-Amitia-Local-Token": "supersecrettoken"})
        data = check.to_safe_dict()
        self.assertEqual(data["details"]["X-Amitia-Local-Token"], "[REDACTED]")

    def test_password_in_key_redacted(self):
        from report import Check
        check = Check(id="password.test", status="passed",
                      details={"user_password": "hunter2"})
        data = check.to_safe_dict()
        self.assertEqual(data["details"]["user_password"], "[REDACTED]")


class TestReportWrite(unittest.TestCase):
    def test_write_json(self):
        from report import ValidationReport, Check, CheckStatus
        report = ValidationReport(runtimeVersion="0.1.0")
        report.add_check(Check(id="a", status=CheckStatus.PASSED.value))
        report.finalize()
        with tempfile.TemporaryDirectory() as tmpdir:
            path = os.path.join(tmpdir, "report.json")
            report.write_json(path)
            self.assertTrue(os.path.exists(path))
            import json
            with open(path, "r", encoding="utf-8") as f:
                loaded = json.load(f)
            self.assertEqual(loaded["result"], "passed")
            self.assertEqual(len(loaded["checks"]), 1)

    def test_write_summary(self):
        from report import ValidationReport, Check, CheckStatus
        report = ValidationReport(runtimeVersion="0.1.0")
        report.add_check(Check(id="a", status=CheckStatus.PASSED.value))
        report.finalize()
        with tempfile.TemporaryDirectory() as tmpdir:
            path = os.path.join(tmpdir, "summary.txt")
            report.write_summary(path)
            self.assertTrue(os.path.exists(path))
            with open(path, "r", encoding="utf-8") as f:
                content = f.read()
            self.assertIn("PASSED", content)


if __name__ == "__main__":
    unittest.main()
