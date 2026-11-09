import 'package:amitia_app/core/backend_transport/errors/backend_transport_error.dart';
import 'package:amitia_app/core/backend_transport/errors/backend_transport_error_code.dart';
import 'package:amitia_app/core/services/providers.dart';
import 'package:flutter_test/flutter_test.dart';

void main() {
  test('统筹权限瞬时传输错误允许有界重试', () {
    expect(
      retryableSharedCoreAccessError(
        BackendTransportError(
          code: BackendTransportErrorCode.serverError,
          path: '/api/device-mesh/v1/coordination/me',
        ),
      ),
      isTrue,
    );
    expect(
      retryableSharedCoreAccessError(
        BackendTransportError(
          code: BackendTransportErrorCode.authenticationFailed,
          path: '/api/device-mesh/v1/coordination/me',
        ),
      ),
      isFalse,
    );
  });
}
