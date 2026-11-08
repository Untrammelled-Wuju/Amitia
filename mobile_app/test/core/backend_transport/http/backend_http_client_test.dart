import 'dart:async';
import 'dart:io';

import 'package:flutter_test/flutter_test.dart';
import 'package:amitia_app/core/backend_connection/backend_connection_config.dart';
import 'package:amitia_app/core/backend_connection/backend_connection_credential.dart';
import 'package:amitia_app/core/backend_connection/backend_connection_endpoint.dart';
import 'package:amitia_app/core/backend_transport/auth/backend_auth_header.dart';
import 'package:amitia_app/core/backend_transport/errors/backend_transport_error.dart';
import 'package:amitia_app/core/backend_transport/errors/backend_transport_error_code.dart';
import 'package:amitia_app/core/backend_transport/http/backend_http_client.dart';
import 'package:amitia_app/core/backend_transport/http/backend_http_method.dart';
import 'package:amitia_app/core/backend_transport/http/backend_http_request.dart';
import 'package:amitia_app/core/backend_transport/core_configuration_intent.dart';

import '../fakes/fake_backend_server.dart';

BackendConnectionConfig _makeConfig(int port, {int generation = 1}) {
  return BackendConnectionConfig(
    schemaVersion: 1,
    generation: generation,
    endpoint: BackendConnectionEndpoint(
      host: '127.0.0.1',
      port: port,
      httpScheme: 'http',
      webSocketScheme: 'ws',
      livenessPath: '/livez',
      readinessPath: '/readyz',
    ),
    authStrategy: BackendAuthStrategy.localToken,
    credential: BackendConnectionCredential.tryCreate(
      'test_token_32chars_long_12345678901234',
    )!,
  );
}

void main() {
  group('BackendHttpClient', () {
    late FakeBackendServer server;
    late BackendHttpClient client;

    setUp(() async {
      server = FakeBackendServer();
      await server.start();
      server.requireToken = false;
      client = BackendHttpClient(_makeConfig(server.port));
    });

    tearDown(() async {
      await client.close();
      await server.stop();
    });

    test('GET successful returns 200', () async {
      final resp = await client.send(
        BackendHttpRequest(
          method: BackendHttpMethod.get,
          path: '/api/characters',
        ),
      );
      expect(resp.statusCode, 200);
      expect(server.requests.length, 1);
      expect(server.requests.first.method, 'GET');
      expect(server.requests.first.path, '/api/characters');
    });

    test('配置请求携带原始 Core ID 且覆盖伪造请求头', () async {
      final intent = CoreConfigurationIntent(
        generation: 1,
        coreId: 'core-b',
        policyRevision: '1:2:3',
        canConfigure: true,
        isCurrent: () => true,
      );
      await intent.run(
        () => client.send(
          BackendHttpRequest(
            method: BackendHttpMethod.put,
            path: '/api/model-configs/1',
            body: {'name': 'old-form'},
            headers: {'X-Amitia-Expected-Core-ID': 'core-c'},
          ),
        ),
      );
      expect(
        server.requests.single.headers['x-amitia-expected-core-id'],
        'core-b',
      );
      expect(
        server
            .requests
            .single
            .headers['x-amitia-expected-configuration-policy'],
        '1:2:3',
      );
    });

    test('旧 generation 配置在 HTTP 发出前拒绝', () async {
      final intent = CoreConfigurationIntent(
        generation: 2,
        coreId: 'core-b',
        canConfigure: true,
        isCurrent: () => true,
      );
      await expectLater(
        intent.run(
          () => client.send(
            BackendHttpRequest(
              method: BackendHttpMethod.put,
              path: '/api/model-configs/1',
              body: {'name': 'old-form'},
            ),
          ),
        ),
        throwsStateError,
      );
      expect(server.requests, isEmpty);
    });

    test('POST JSON sends correct body and headers', () async {
      final resp = await client.send(
        BackendHttpRequest(
          method: BackendHttpMethod.post,
          path: '/api/characters',
          body: {'name': 'test'},
        ),
      );
      expect(resp.statusCode, 200);
      final req = server.requests.first;
      expect(req.headers['content-type'], 'application/json');
      expect(req.body, contains('test'));
    });

    test('PUT sends correct method', () async {
      final resp = await client.send(
        BackendHttpRequest(
          method: BackendHttpMethod.put,
          path: '/api/characters/1',
          body: {'name': 'updated'},
        ),
      );
      expect(resp.statusCode, 200);
      expect(server.requests.first.method, 'PUT');
    });

    test('DELETE sends correct method', () async {
      final resp = await client.send(
        BackendHttpRequest(
          method: BackendHttpMethod.delete,
          path: '/api/characters/1',
        ),
      );
      expect(resp.statusCode, 200);
      expect(server.requests.first.method, 'DELETE');
    });

    test('HEAD does not set content-type', () async {
      final resp = await client.send(
        BackendHttpRequest(
          method: BackendHttpMethod.head,
          path: '/api/characters',
        ),
      );
      expect(resp.statusCode, 200);
      final contentType = server.requests.first.headers['content-type'];
      expect(contentType, isNull);
    });

    test('query parameters are sent', () async {
      final resp = await client.send(
        BackendHttpRequest(
          method: BackendHttpMethod.get,
          path: '/api/items',
          queryParameters: {'q': 'test', 'page': '1'},
        ),
      );
      expect(resp.statusCode, 200);
      expect(server.requests.first.queryParameters['q'], 'test');
      expect(server.requests.first.queryParameters['page'], '1');
    });

    test('User-Agent Amitia-Mobile is set', () async {
      final resp = await client.send(
        BackendHttpRequest(method: BackendHttpMethod.get, path: '/api/items'),
      );
      expect(resp.statusCode, 200);
      expect(server.requests.first.headers['user-agent'], 'Amitia-Mobile');
    });

    test('X-Amitia-Local-Token is injected', () async {
      server.requireToken = true;
      server.validToken = 'test_token_32chars_long_12345678901234';
      final resp = await client.send(
        BackendHttpRequest(method: BackendHttpMethod.get, path: '/api/items'),
      );
      expect(resp.statusCode, 200);
      expect(
        server.requests.first.headers['x-amitia-local-token'],
        'test_token_32chars_long_12345678901234',
      );
    });

    test('cannot override protected headers', () async {
      await expectLater(
        client.send(
          BackendHttpRequest(
            method: BackendHttpMethod.get,
            path: '/api/items',
            headers: {BackendAuthHeader.localToken: 'bad_token'},
          ),
        ),
        throwsA(isA<Exception>()),
      );
    });

    test('401 maps to authentication failed', () async {
      server.requireToken = true;
      server.validToken = 'different_token';
      await expectLater(
        client.send(
          BackendHttpRequest(method: BackendHttpMethod.get, path: '/api/items'),
        ),
        throwsA(isA<Exception>()),
      );
      expect(server.authFailures, 1);
    });

    test('404 throws BackendTransportError with notFound code', () async {
      server.requireToken = false;
      await expectLater(
        client.send(
          BackendHttpRequest(
            method: BackendHttpMethod.get,
            path: '/nonexistent',
          ),
        ),
        throwsA(isA<BackendTransportError>()),
      );
      try {
        await client.send(
          BackendHttpRequest(
            method: BackendHttpMethod.get,
            path: '/nonexistent',
          ),
        );
      } catch (e) {
        expect(e, isA<BackendTransportError>());
        expect(
          (e as BackendTransportError).code,
          BackendTransportErrorCode.notFound,
        );
        expect(e.statusCode, 404);
      }
    });

    test('409 preserves the complete response body', () async {
      final server = await HttpServer.bind(InternetAddress.loopbackIPv4, 0);
      addTearDown(() => server.close(force: true));
      server.listen((request) async {
        request.response.statusCode = HttpStatus.conflict;
        request.response.headers.contentType = ContentType.json;
        request.response.write(
          '{"code":"pairing.already_paired","message":"该设备已添加，无需重复扫码","detail":"same origin"}',
        );
        await request.response.close();
      });
      final conflictClient = BackendHttpClient(_makeConfig(server.port));
      addTearDown(conflictClient.close);

      try {
        await conflictClient.send(
          BackendHttpRequest(
            method: BackendHttpMethod.post,
            path: '/internal/device-mesh/pairing/claim',
            body: const <String, dynamic>{},
          ),
        );
        fail('409 must throw');
      } on BackendTransportError catch (error) {
        expect(error.statusCode, HttpStatus.conflict);
        expect(error.responseBody, isA<Map>());
        expect((error.responseBody as Map)['message'], '该设备已添加，无需重复扫码');
        expect(error.toString(), contains('pairing.already_paired'));
        expect(error.toString(), contains('该设备已添加，无需重复扫码'));
      }
    });

    test('streaming 409 preserves the complete response body', () async {
      final server = await HttpServer.bind(InternetAddress.loopbackIPv4, 0);
      addTearDown(() => server.close(force: true));
      server.listen((request) async {
        request.response.statusCode = HttpStatus.conflict;
        request.response.headers.contentType = ContentType.json;
        request.response.write(
          '{"code":"mesh.continuity_unavailable","message":"持续事项解析失败","detail":"revision 3 != 4"}',
        );
        await request.response.close();
      });
      final conflictClient = BackendHttpClient(_makeConfig(server.port));
      addTearDown(conflictClient.close);

      try {
        await conflictClient.send(
          BackendHttpRequest(
            method: BackendHttpMethod.post,
            path: '/api/device-mesh/v1/business/messages',
            body: const <String, dynamic>{'message': 'test409'},
            headers: const {'Accept': 'text/event-stream'},
            streamResponse: true,
          ),
        );
        fail('409 must throw');
      } on BackendTransportError catch (error) {
        expect(error.statusCode, HttpStatus.conflict);
        expect(error.responseBody, isA<Map>());
        expect(
          (error.responseBody as Map)['code'],
          'mesh.continuity_unavailable',
        );
        expect((error.responseBody as Map)['message'], '持续事项解析失败');
        expect((error.responseBody as Map)['detail'], 'revision 3 != 4');
        expect(error.toString(), isNot(contains("Instance of 'ResponseBody'")));
      }
    });

    test('close makes transport state closed', () async {
      await client.close();
      expect(client.state.name, 'closed');
    });

    test('close is idempotent', () async {
      await client.close();
      await client.close();
      expect(client.state.name, 'closed');
    });

    test('request after close throws transport closed', () async {
      await client.close();
      await expectLater(
        client.send(
          BackendHttpRequest(method: BackendHttpMethod.get, path: '/api/items'),
        ),
        throwsA(isA<Exception>()),
      );
    });

    test('单次请求可覆盖全局超时且控制头不会发送到服务端', () async {
      final scoped = await HttpServer.bind(InternetAddress.loopbackIPv4, 0);
      final requestHeaders = Completer<HttpHeaders>();
      final requestHandled = () async {
        final request = await scoped.first;
        requestHeaders.complete(request.headers);
        await Future<void>.delayed(const Duration(milliseconds: 180));
        request.response
          ..statusCode = HttpStatus.ok
          ..write('{}');
        await request.response.close();
      }();
      final delayedClient = BackendHttpClient(
        _makeConfig(scoped.port),
        operationTimeout: const Duration(milliseconds: 50),
      );
      try {
        final response = await delayedClient.send(
          BackendHttpRequest(
            method: BackendHttpMethod.post,
            path: '/internal/device-mesh/bootstrap',
            body: const <String, dynamic>{},
            headers: const {'X-Amitia-Request-Timeout-Seconds': '1'},
          ),
        );
        await requestHandled;
        expect(response.statusCode, 200);
        expect(
          (await requestHeaders.future).value(
            'x-amitia-request-timeout-seconds',
          ),
          isNull,
        );
      } finally {
        await delayedClient.close();
        await scoped.close(force: true);
      }
    });

    test('重复超时响应头不会阻断正常响应', () async {
      final scoped = await HttpServer.bind(InternetAddress.loopbackIPv4, 0);
      final requestHandled = () async {
        final request = await scoped.first;
        request.response
          ..statusCode = HttpStatus.ok
          ..headers.add('x-amitia-timeout-disabled', 'false')
          ..headers.add('x-amitia-timeout-disabled', 'false')
          ..headers.add('x-amitia-timeout-seconds', '180')
          ..headers.add('x-amitia-timeout-seconds', '180')
          ..write('{}');
        await request.response.close();
      }();
      final duplicateHeaderClient = BackendHttpClient(_makeConfig(scoped.port));
      try {
        final response = await duplicateHeaderClient.send(
          BackendHttpRequest(method: BackendHttpMethod.get, path: '/api/space'),
        );
        await requestHandled;
        expect(response.statusCode, 200);
      } finally {
        await duplicateHeaderClient.close();
        await scoped.close(force: true);
      }
    });
  });

  group('BackendGeneration', () {
    test('generation changes propagate', () async {
      final server = FakeBackendServer();
      await server.start();
      server.requireToken = false;
      try {
        final config1 = _makeConfig(server.port, generation: 1);
        final config2 = _makeConfig(server.port, generation: 2);
        expect(config1.generation, 1);
        expect(config2.generation, 2);
      } finally {
        await server.stop();
      }
    });
  });

  group('request model', () {
    test('BackendHttpRequest can be created', () {
      final req = BackendHttpRequest(
        method: BackendHttpMethod.get,
        path: '/api/items',
      );
      expect(req.method, BackendHttpMethod.get);
      expect(req.path, '/api/items');
    });
  });

  group('generation', () {
    test('config with generation <= 0 throws', () async {
      final server = FakeBackendServer();
      await server.start();
      try {
        expect(
          () => _makeConfig(server.port, generation: 0),
          throwsArgumentError,
        );
        expect(
          () => _makeConfig(server.port, generation: -1),
          throwsArgumentError,
        );
      } finally {
        await server.stop();
      }
    });
  });
}
