import 'package:flutter_test/flutter_test.dart';
import 'package:amitia_app/core/backend_connection/backend_connection_config.dart';
import 'package:amitia_app/core/backend_connection/backend_connection_credential.dart';
import 'package:amitia_app/core/backend_connection/backend_connection_endpoint.dart';
import 'package:amitia_app/core/backend_transport/auth/backend_auth_header.dart';
import 'package:amitia_app/core/backend_transport/http/backend_http_client.dart';
import 'package:amitia_app/core/backend_transport/http/backend_http_method.dart';
import 'package:amitia_app/core/backend_transport/http/backend_http_request.dart';
import 'package:amitia_app/core/backend_transport/state/backend_transport_state.dart';

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
      final resp = await client.send(BackendHttpRequest(
        method: BackendHttpMethod.get,
        path: '/api/characters',
      ));
      expect(resp.statusCode, 200);
      expect(server.requests.length, 1);
      expect(server.requests.first.method, 'GET');
      expect(server.requests.first.path, '/api/characters');
    });

    test('POST JSON sends correct body and headers', () async {
      final resp = await client.send(BackendHttpRequest(
        method: BackendHttpMethod.post,
        path: '/api/characters',
        body: {'name': 'test'},
      ));
      expect(resp.statusCode, 200);
      final req = server.requests.first;
      expect(req.headers['content-type'], 'application/json');
      expect(req.body, contains('test'));
    });

    test('PUT sends correct method', () async {
      final resp = await client.send(BackendHttpRequest(
        method: BackendHttpMethod.put,
        path: '/api/characters/1',
        body: {'name': 'updated'},
      ));
      expect(resp.statusCode, 200);
      expect(server.requests.first.method, 'PUT');
    });

    test('DELETE sends correct method', () async {
      final resp = await client.send(BackendHttpRequest(
        method: BackendHttpMethod.delete,
        path: '/api/characters/1',
      ));
      expect(resp.statusCode, 200);
      expect(server.requests.first.method, 'DELETE');
    });

    test('HEAD does not set content-type', () async {
      final resp = await client.send(BackendHttpRequest(
        method: BackendHttpMethod.head,
        path: '/api/characters',
      ));
      expect(resp.statusCode, 200);
      final contentType = server.requests.first.headers['content-type'];
      expect(contentType, isNull);
    });

    test('query parameters are sent', () async {
      final resp = await client.send(BackendHttpRequest(
        method: BackendHttpMethod.get,
        path: '/api/items',
        queryParameters: {'q': 'test', 'page': '1'},
      ));
      expect(resp.statusCode, 200);
      expect(server.requests.first.queryParameters['q'], 'test');
      expect(server.requests.first.queryParameters['page'], '1');
    });

    test('User-Agent Amitia-Mobile is set', () async {
      final resp = await client.send(BackendHttpRequest(
        method: BackendHttpMethod.get,
        path: '/api/items',
      ));
      expect(resp.statusCode, 200);
      expect(server.requests.first.headers['user-agent'], 'Amitia-Mobile');
    });

    test('X-Amitia-Local-Token is injected', () async {
      server.requireToken = true;
      server.validToken = 'test_token_32chars_long_12345678901234';
      final resp = await client.send(BackendHttpRequest(
        method: BackendHttpMethod.get,
        path: '/api/items',
      ));
      expect(resp.statusCode, 200);
      expect(
        server.requests.first.headers['x-amitia-local-token'],
        'test_token_32chars_long_12345678901234',
      );
    });

    test('cannot override protected headers', () async {
      await expectLater(
        client.send(BackendHttpRequest(
          method: BackendHttpMethod.get,
          path: '/api/items',
          headers: {BackendAuthHeader.localToken: 'bad_token'},
        )),
        throwsA(isA<Exception>()),
      );
    });

    test('401 maps to authentication failed', () async {
      server.requireToken = true;
      server.validToken = 'different_token';
      await expectLater(
        client.send(BackendHttpRequest(
          method: BackendHttpMethod.get,
          path: '/api/items',
        )),
        throwsA(isA<Exception>()),
      );
      expect(server.authFailures, 1);
    });

    test('404 does not throw but returns status', () async {
      final resp = await client.send(BackendHttpRequest(
        method: BackendHttpMethod.get,
        path: '/nonexistent',
      ));
      expect(resp.statusCode, 404);
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
        client.send(BackendHttpRequest(
          method: BackendHttpMethod.get,
          path: '/api/items',
        )),
        throwsA(isA<Exception>()),
      );
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
