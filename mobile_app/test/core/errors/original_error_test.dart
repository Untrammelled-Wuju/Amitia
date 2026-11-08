import 'package:amitia_app/core/backend_transport/errors/backend_transport_error.dart';
import 'package:amitia_app/core/backend_transport/errors/backend_transport_error_code.dart';
import 'package:amitia_app/core/backend_transport/backend_service_api.dart';
import 'package:amitia_app/core/errors/original_error.dart';
import 'package:flutter_test/flutter_test.dart';

void main() {
  test('BackendTransportError keeps the original cause', () {
    final error = BackendTransportError(
      code: BackendTransportErrorCode.serverError,
      cause: StateError('原始服务错误'),
    );
    expect(error.toString(), contains('Bad state: 原始服务错误'));
    expect(originalErrorText(error), 'Bad state: 原始服务错误');
  });

  test('BackendTransportError preserves response body before cause', () {
    final error = BackendTransportError(
      code: BackendTransportErrorCode.conflict,
      statusCode: 409,
      responseBody: {
        'code': 'pairing.already_paired',
        'message': '该设备已添加，无需重复扫码',
      },
      cause: Exception('transport failed'),
    );

    final text = originalErrorText(error);
    expect(text, contains('该设备已添加，无需重复扫码'));
    expect(text, contains('pairing.already_paired'));
    expect(text, contains('transport failed'));
  });

  test('ServiceApiException exposes the original message through toString', () {
    final error = ServiceApiException(code: 409, message: '项目数据版本冲突');
    expect(error.toString(), '项目数据版本冲突');
    expect(originalErrorText(error), '项目数据版本冲突');
  });

  test('map errors keep nested original messages', () {
    expect(
      originalErrorText({
        'message': '请求失败',
        'cause': {'detail': 'revision 3 != 4'},
      }),
      '请求失败\nrevision 3 != 4',
    );
    expect(
      originalErrorText({
        'error': {
          'internalMessage': 'upstream returned 502',
          'rawError': 'upstream returned 502: bad gateway',
        },
      }),
      'upstream returned 502\nupstream returned 502: bad gateway',
    );
  });
}
