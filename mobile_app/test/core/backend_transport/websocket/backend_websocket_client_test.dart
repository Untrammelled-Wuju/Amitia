import 'dart:async';

import 'package:flutter_test/flutter_test.dart';
import 'package:amitia_app/core/backend_connection/backend_connection_config.dart';
import 'package:amitia_app/core/backend_connection/backend_connection_credential.dart';
import 'package:amitia_app/core/backend_connection/backend_connection_endpoint.dart';
import 'package:amitia_app/core/backend_transport/auth/backend_auth_header.dart';
import 'package:amitia_app/core/backend_transport/websocket/backend_websocket_client.dart';
import 'package:amitia_app/core/backend_transport/websocket/backend_websocket_message.dart';
import 'package:amitia_app/core/backend_transport/websocket/backend_websocket_session.dart';
import 'package:amitia_app/core/backend_transport/websocket/backend_websocket_transport.dart';

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
    credential: BackendConnectionCredential.tryCreate(
      'test_token_32chars_long_12345678901234',
    )!,
  );
}

void main() {
  group('BackendWebSocketClient', () {
    late FakeBackendServer server;
    late BackendWebSocketClient client;

    setUp(() async {
      server = FakeBackendServer();
      await server.start();
      server.requireToken = true;
      server.validToken = 'test_token_32chars_long_12345678901234';
      client = BackendWebSocketClient(_makeConfig(server.port));
    });

    tearDown(() async {
      await client.close();
      await server.stop();
    });

    test('connects and receives echo', () async {
      final session = await client.connect('/ws/chat');
      expect(session, isA<BackendWebSocketSession>());
      expect(session.generation, 1);
      expect(client.state.name, 'connected');

      final completer = Completer<String>();
      final sub = session.messages.listen((msg) {
        if (msg is WebSocketTextMessage && !completer.isCompleted) {
          completer.complete(msg.data);
        }
      });

      await session.send(WebSocketTextMessage('hello'));
      final echoed = await completer.future.timeout(const Duration(seconds: 5));
      expect(echoed, 'echo: hello');

      await sub.cancel();
    });

    test('X-Amitia-Local-Token is injected', () async {
      final session = await client.connect('/ws/chat');
      expect(server.requests.length, 1);
      expect(
        server.requests.first.headers['x-amitia-local-token'],
        'test_token_32chars_long_12345678901234',
      );
      await session.close();
    });

    test('invalid token causes auth failure', () async {
      server.validToken = 'different_token_32chars_long_1234567890';
      await expectLater(
        client.connect('/ws/chat'),
        throwsA(isA<Exception>()),
      );
      expect(server.authFailures, 1);
    });

    test('skip auth when requireToken=false', () async {
      server.requireToken = false;
      final session = await client.connect('/ws/chat');
      expect(session, isA<BackendWebSocketSession>());
      expect(server.authFailures, 0);
      await session.close();
    });

    test('user-agent header contains Amitia-Mobile', () async {
      final session = await client.connect('/ws/chat');
      expect(server.requests.first.headers['user-agent'], contains('Amitia-Mobile'));
      await session.close();
    });

    test('generation is carried in session', () async {
      final gen2Client = BackendWebSocketClient(
        _makeConfig(server.port, generation: 2),
      );
      final session = await gen2Client.connect('/ws/chat');
      expect(session.generation, 2);
      await session.close();
      await gen2Client.close();
    });

    test('close makes transport state closed', () async {
      await client.connect('/ws/chat');
      await client.close();
      expect(client.state.name, 'closed');
    });

    test('close is idempotent', () async {
      await client.connect('/ws/chat');
      await client.close();
      await client.close();
      expect(client.state.name, 'closed');
    });

    test('connect after close throws transport closed', () async {
      await client.close();
      await expectLater(
        client.connect('/ws/chat'),
        throwsA(isA<Exception>()),
      );
    });
  });
}
