import 'package:flutter_test/flutter_test.dart';
import 'package:amitia_app/features/workbench/runtime/remote_browser_failure_policy.dart';

void main() {
  test('expired or denied remote browser observation is terminal', () {
    for (final code in [400, 401, 403, 404, 410, 501]) {
      expect(remoteBrowserFailureIsTerminal(code), isTrue);
    }
  });
  test('explicit nonretryable domain errors stop regardless of HTTP status', () {
    expect(remoteBrowserFailureIsTerminal(503, errorCode: 'capability_revoked'), isTrue);
    expect(remoteBrowserFailureIsTerminal(502, errorCode: 'resource_exceeded', retryable: false), isTrue);
  });
  test('offline and timeout failures remain retryable', () {
    for (final code in [null, 500, 502, 503, 504, 10001, 10002]) {
      expect(remoteBrowserFailureIsTerminal(code), isFalse);
    }
  });
}
