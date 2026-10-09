import 'dart:convert';
import 'dart:io';

import 'package:amitia_app/core/backend_transport/backend_service_api.dart';
import 'package:amitia_app/core/services/onboarding_service.dart';
import 'package:flutter_test/flutter_test.dart';

class PairingProofApi extends Fake implements BackendServiceApi {
  @override
  Future<T?> post<T>(
    String path, {
    Object? data,
    Map<String, dynamic>? queryParameters,
    Map<String, String>? headers,
    T Function(dynamic)? fromJson,
  }) async {
    expect(path, '/internal/device-mesh/identity/sign-claim');
    return {'signature': 'fixture-proof'} as T;
  }
}

void main() {
  for (final message in [
    '不能添加当前设备自身',
    '该设备已添加，无需重复扫码',
    '该设备正在配对，请勿重复添加',
    '配对码已过期，请重新生成',
    '配对码已使用，请重新生成',
  ]) {
    test('真实HTTP配对错误保留中文：$message', () async {
      final server = await HttpServer.bind(InternetAddress.loopbackIPv4, 0);
      addTearDown(() => server.close(force: true));
      var claims = 0;
      server.listen((request) async {
        request.response.headers.contentType = ContentType.json;
        if (request.uri.path.endsWith('/status')) {
          request.response.write(
            jsonEncode({
              'spaceId': 'core-b',
              'firstDeviceSetupRequired': false,
            }),
          );
        } else {
          claims++;
          final body =
              jsonDecode(await utf8.decoder.bind(request).join()) as Map;
          expect(body['deviceId'], 'device-a');
          request.response.statusCode = HttpStatus.conflict;
          request.response.write(
            jsonEncode({'code': 'mesh.pairing_rejected', 'message': message}),
          );
        }
        await request.response.close();
      });
      await expectLater(
        OnboardingService(PairingProofApi()).claimPairingAt(
          'http://127.0.0.1:${server.port}',
          deviceId: 'device-a',
          runtimeId: 'runtime-a',
          platform: 'android',
          offerToken: 'offer',
        ),
        throwsA(
          isA<StateError>().having((error) => error.message, '中文提示', message),
        ),
      );
      expect(claims, 1);
    });
  }

  test('撤销后新码成功返回Bootstrap Ticket，不因旧错误缓存拒绝', () async {
    final server = await HttpServer.bind(InternetAddress.loopbackIPv4, 0);
    addTearDown(() => server.close(force: true));
    server.listen((request) async {
      request.response.headers.contentType = ContentType.json;
      request.response.write(
        jsonEncode(
          request.uri.path.endsWith('/status')
              ? {'spaceId': 'core-b'}
              : {'ticket': 'fresh-bootstrap', 'spaceId': 'core-b'},
        ),
      );
      await request.response.close();
    });
    final result = await OnboardingService(PairingProofApi()).claimPairingAt(
      'http://127.0.0.1:${server.port}',
      deviceId: 'device-a',
      runtimeId: 'runtime-a',
      platform: 'android',
      offerToken: 'new-offer',
    );
    expect(result['ticket'], 'fresh-bootstrap');
  });
}
