import 'package:flutter_test/flutter_test.dart';
import 'package:amitia_app/core/backend_connection/backend_connection_config.dart';
import 'package:amitia_app/core/backend_connection/backend_connection_credential.dart';
import 'package:amitia_app/core/backend_connection/backend_connection_endpoint.dart';
import 'package:amitia_app/core/backend_transport/default_backend_transport.dart';

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
  group('BackendTransport WebSocket Generation', () {
    late FakeBackendServer server;

    setUp(() async {
      server = FakeBackendServer();
      await server.start();
      server.requireToken = false;
    });

    tearDown(() async {
      await server.stop();
    });

    test('WebSocket session carries generation from config', () async {
      final transport = DefaultBackendTransport.create(
        _makeConfig(server.port, generation: 1),
      );
      final session = await transport.webSocket.connect('/ws/chat');
      expect(session.generation, 1);
      await session.close();
      await transport.close();
    });

    test('different configs produce different generation transports', () async {
      final t1 = DefaultBackendTransport.create(
        _makeConfig(server.port, generation: 1),
      );
      final t2 = DefaultBackendTransport.create(
        _makeConfig(server.port, generation: 2),
      );
      expect(t1.generation, 1);
      expect(t2.generation, 2);
      await t1.close();
      await t2.close();
    });

    test('close transport closes all WebSocket sessions', () async {
      final transport = DefaultBackendTransport.create(
        _makeConfig(server.port, generation: 1),
      );
      await transport.webSocket.connect('/ws/chat');
      await transport.webSocket.connect('/ws/events');
      expect(transport.webSocket.state.name, 'connected');
      await transport.close();
      expect(transport.webSocket.state.name, 'closed');
    });
  });
}
