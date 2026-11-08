import 'dart:convert';
import 'dart:io';
import 'dart:typed_data';

import 'package:crypto/crypto.dart';
import 'package:amitia_app/core/backend_transport/backend_service_api.dart';
import 'package:amitia_app/core/security/certificate_pin.dart';
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

class PinnedPairingApi extends Fake implements BackendServiceApi {
  var claims = 0;

  @override
  Future<T?> post<T>(
    String path, {
    Object? data,
    Map<String, dynamic>? queryParameters,
    Map<String, String>? headers,
    T Function(dynamic)? fromJson,
  }) async {
    expect(path, '/internal/device-mesh/pairing/claim');
    final body = data as Map<String, dynamic>;
    claims++;
    if (claims == 1) {
      expect(body['offerToken'], '');
      return {
            'pending': true,
            'requestId': 'request-pinned',
            'offerToken': 'pinned-direct-token',
          }
          as T;
    }
    expect(body['offerToken'], 'pinned-direct-token');
    return {'ticket': 'pinned-bootstrap', 'spaceId': 'core-b'} as T;
  }

  @override
  Future<Stream<List<int>>> getStream(
    String path, {
    Map<String, dynamic>? queryParameters,
    Map<String, String>? headers,
    dynamic cancelToken,
  }) async {
    expect(
      path,
      '/internal/device-mesh/pairing/requests/request-pinned/events',
    );
    return Stream<List<int>>.value(
      utf8.encode('event: pairing_approved\ndata: {"state":"approved"}\n\n'),
    );
  }
}

class RevokedPinnedPairingApi extends Fake implements BackendServiceApi {
  var claims = 0;
  var deletedCredential = false;

  @override
  Future<T?> post<T>(
    String path, {
    Object? data,
    Map<String, dynamic>? queryParameters,
    Map<String, String>? headers,
    T Function(dynamic)? fromJson,
  }) async {
    expect(path, '/internal/device-mesh/pairing/claim');
    claims++;
    if (claims == 1) {
      throw StateError('pairing.already_paired\n该设备已添加，无需重复扫码');
    }
    if (claims == 2) {
      return {
            'pending': true,
            'requestId': 'request-revoked',
            'offerToken': 'revoked-direct-token',
          }
          as T;
    }
    return {'ticket': 'revoked-bootstrap', 'spaceId': 'core-c'} as T;
  }

  @override
  Future<T?> get<T>(
    String path, {
    Map<String, dynamic>? queryParameters,
    Map<String, String>? headers,
    T Function(dynamic)? fromJson,
  }) async {
    expect(path, '/internal/device-mesh/status');
    return {'state': 'revoked'} as T;
  }

  @override
  Future<void> delete(
    String path, {
    Map<String, dynamic>? queryParameters,
    Map<String, String>? headers,
  }) async {
    expect(path, '/internal/device-mesh/credential');
    deletedCredential = true;
  }

  @override
  Future<Stream<List<int>>> getStream(
    String path, {
    Map<String, dynamic>? queryParameters,
    Map<String, String>? headers,
    dynamic cancelToken,
  }) async {
    expect(
      path,
      '/internal/device-mesh/pairing/requests/request-revoked/events',
    );
    return Stream<List<int>>.value(
      utf8.encode('event: pairing_approved\ndata: {"state":"approved"}\n\n'),
    );
  }
}

void main() {
  test('证书指纹与二维码使用SubjectPublicKeyInfo哈希', () {
    Uint8List tlv(int tag, List<int> content) {
      final length = content.length;
      final encodedLength = length < 128
          ? <int>[length]
          : length < 256
          ? <int>[0x81, length]
          : <int>[0x82, length >> 8, length & 0xff];
      return Uint8List.fromList([tag, ...encodedLength, ...content]);
    }

    final publicKeyInfo = tlv(0x30, [
      ...tlv(0x30, const []),
      ...tlv(0x03, const [0, 1, 2, 3]),
    ]);
    final tbs = tlv(0x30, [
      ...tlv(0xa0, tlv(0x02, const [2])),
      ...tlv(0x02, const [1]),
      ...tlv(0x30, const []),
      ...tlv(0x30, const []),
      ...tlv(0x30, const []),
      ...tlv(0x30, const []),
      ...publicKeyInfo,
    ]);
    final certificate = tlv(0x30, [
      ...tbs,
      ...tlv(0x30, const []),
      ...tlv(0x03, const [0, 4, 5, 6]),
    ]);
    expect(
      certificatePublicKeyInfoSha256(certificate),
      sha256.convert(publicKeyInfo).toString(),
    );
    expect(
      certificatePublicKeyInfoSha256(certificate),
      isNot(sha256.convert(certificate).toString()),
    );
  });

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

  test('地址配对审批后复用服务端短暂配对凭据领取票据', () async {
    final server = await HttpServer.bind(InternetAddress.loopbackIPv4, 0);
    addTearDown(() => server.close(force: true));
    var claims = 0;
    server.listen((request) async {
      if (request.uri.path.endsWith('/status')) {
        request.response.headers.contentType = ContentType.json;
        request.response.write(
          jsonEncode({'spaceId': 'core-b', 'firstDeviceSetupRequired': false}),
        );
      } else if (request.uri.path.endsWith('/events')) {
        request.response.headers.contentType = ContentType(
          'text',
          'event-stream',
        );
        request.response.write(
          'event: pairing_approved\ndata: {"state":"approved"}\n\n',
        );
      } else {
        claims++;
        request.response.headers.contentType = ContentType.json;
        final body = jsonDecode(await utf8.decoder.bind(request).join()) as Map;
        if (claims == 1) {
          expect(body.containsKey('offerToken'), isFalse);
          request.response.statusCode = HttpStatus.accepted;
          request.response.write(
            jsonEncode({
              'pending': true,
              'requestId': 'request-a',
              'offerToken': 'direct-pairing-token',
            }),
          );
        } else {
          expect(body['offerToken'], 'direct-pairing-token');
          request.response.write(
            jsonEncode({'ticket': 'direct-bootstrap', 'spaceId': 'core-b'}),
          );
        }
      }
      await request.response.close();
    });
    final result = await OnboardingService(PairingProofApi()).claimPairingAt(
      'http://127.0.0.1:${server.port}',
      deviceId: 'device-a',
      runtimeId: 'runtime-a',
      platform: 'android',
    );
    expect(result['ticket'], 'direct-bootstrap');
    expect(claims, 2);
  });

  test('审批事件流中断后重新订阅同一请求并完成配对', () async {
    final server = await HttpServer.bind(InternetAddress.loopbackIPv4, 0);
    addTearDown(() => server.close(force: true));
    var claims = 0;
    var eventStreams = 0;
    server.listen((request) async {
      if (request.uri.path.endsWith('/status')) {
        request.response.headers.contentType = ContentType.json;
        request.response.write(
          jsonEncode({'spaceId': 'core-b', 'firstDeviceSetupRequired': false}),
        );
      } else if (request.uri.path.endsWith('/events')) {
        eventStreams++;
        expect(
          request.uri.path,
          '/api/public/device-mesh/v1/pairing/requests/request-a/events',
        );
        request.response.headers.contentType = ContentType(
          'text',
          'event-stream',
        );
        if (eventStreams == 1) {
          request.response.write(
            'event: pairing_pending\n'
            'data: {"state":"pending","expiresAt":"${DateTime.now().toUtc().add(const Duration(minutes: 1)).toIso8601String()}"}\n\n',
          );
        } else {
          request.response.write(
            'event: pairing_approved\ndata: {"state":"approved"}\n\n',
          );
        }
      } else {
        claims++;
        request.response.headers.contentType = ContentType.json;
        final body = jsonDecode(await utf8.decoder.bind(request).join()) as Map;
        if (claims == 1) {
          expect(body.containsKey('offerToken'), isFalse);
          request.response.statusCode = HttpStatus.accepted;
          request.response.write(
            jsonEncode({
              'pending': true,
              'requestId': 'request-a',
              'offerToken': 'direct-pairing-token',
            }),
          );
        } else {
          expect(body['offerToken'], 'direct-pairing-token');
          request.response.write(
            jsonEncode({'ticket': 'direct-bootstrap', 'spaceId': 'core-b'}),
          );
        }
      }
      await request.response.close();
    });
    final result =
        await OnboardingService(
          PairingProofApi(),
          pairingApprovalWait: const Duration(seconds: 6),
        ).claimPairingAt(
          'http://127.0.0.1:${server.port}',
          deviceId: 'device-a',
          runtimeId: 'runtime-a',
          platform: 'android',
        );
    expect(result['ticket'], 'direct-bootstrap');
    expect(claims, 2);
    expect(eventStreams, 2);
  });

  test('远程凭据被撤销后清理本地旧凭据并允许重新配对', () async {
    final api = RevokedPinnedPairingApi();
    final result = await OnboardingService(api).claimPairingAt(
      'https://10.0.0.2:7443',
      deviceId: 'device-c',
      runtimeId: 'runtime-c',
      platform: 'android',
      offerToken: 'fresh-offer',
      fingerprint: List.filled(64, 'b').join(),
      coreId: 'core-c',
    );

    expect(api.deletedCredential, isTrue);
    expect(api.claims, 3);
    expect(result['ticket'], 'revoked-bootstrap');
  });

  test('手动地址发现证书后使用钉扎配对并复用审批凭据', () async {
    final api = PinnedPairingApi();
    final result = await OnboardingService(api).claimPairingAt(
      'https://10.0.0.2:7443',
      deviceId: 'device-a',
      runtimeId: 'runtime-a',
      platform: 'android',
      fingerprint: List.filled(64, 'a').join(),
      coreId: 'core-b',
    );
    expect(result['ticket'], 'pinned-bootstrap');
    expect(api.claims, 2);
  });
}
