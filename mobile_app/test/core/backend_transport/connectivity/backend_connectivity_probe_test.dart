import 'package:flutter_test/flutter_test.dart';
import 'package:amitia_app/core/backend_connection/backend_connection_config.dart';
import 'package:amitia_app/core/backend_connection/backend_connection_credential.dart';
import 'package:amitia_app/core/backend_connection/backend_connection_endpoint.dart';
import 'package:amitia_app/core/backend_transport/connectivity/backend_connectivity_probe.dart';
import 'package:amitia_app/core/backend_transport/http/backend_http_client.dart';
import 'package:amitia_app/core/backend_transport/http/backend_http_request.dart';
import 'package:amitia_app/core/backend_transport/http/backend_http_response.dart';
import 'package:amitia_app/core/backend_transport/http/backend_http_transport.dart';
import 'package:amitia_app/core/backend_transport/state/backend_http_state.dart';

import '../fakes/fake_backend_server.dart';

BackendConnectionConfig _makeConfig(int port) {
  return BackendConnectionConfig(
    schemaVersion: 1,
    generation: 1,
    endpoint: BackendConnectionEndpoint(
      host: '127.0.0.1',
      port: port,
      httpScheme: 'http',
      webSocketScheme: 'ws',
      livenessPath: '/livez',
      readinessPath: '/readyz',
    ),
    authStrategy: BackendAuthStrategy.localToken,
    credential:
        BackendConnectionCredential.tryCreate('test_token_32chars_long_12345678901234')!,
  );
}

class _FailingHttpTransport implements BackendHttpTransport {
  @override
  BackendHttpState get state => BackendHttpState.available;

  @override
  Future<BackendHttpResponse> send(BackendHttpRequest request) async {
    throw Exception('Network error');
  }

  @override
  Future<void> close() async {}
}

void main() {
  group('BackendConnectivityProbe', () {
    late FakeBackendServer server;

    setUp(() async {
      server = FakeBackendServer();
      await server.start();
      server.requireToken = false;
    });

    tearDown(() async {
      await server.stop();
    });

    test('returns ready when /readyz returns 200', () async {
      final client = BackendHttpClient(_makeConfig(server.port));
      final probe = BackendConnectivityProbe(client);
      final result = await probe.probe();
      expect(result, BackendConnectivityResult.ready);
      await client.close();
    });

    test('returns unreachable when both fail', () async {
      final probe = BackendConnectivityProbe(
        _FailingHttpTransport(),
      );
      final result = await probe.probe();
      expect(result, BackendConnectivityResult.unreachable);
    });

    test('returns live when /readyz fails but /livez succeeds', () async {
      final probe = BackendConnectivityProbe(
        _ReadyFailsLiveSucceedsTransport(),
      );
      final result = await probe.probe();
      expect(result, BackendConnectivityResult.live);
    });
  });
}

class _ReadyFailsLiveSucceedsTransport implements BackendHttpTransport {
  @override
  BackendHttpState get state => BackendHttpState.available;

  @override
  Future<BackendHttpResponse> send(BackendHttpRequest request) async {
    if (request.path == '/readyz') {
      throw Exception('Not ready');
    }
    if (request.path == '/livez') {
      return BackendHttpResponse(statusCode: 200, headers: {});
    }
    throw Exception('unexpected');
  }

  @override
  Future<void> close() async {}
}
