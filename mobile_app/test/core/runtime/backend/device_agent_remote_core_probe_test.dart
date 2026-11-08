import 'dart:io';

import 'package:amitia_app/core/backend_connection/backend_connection_availability.dart';
import 'package:amitia_app/core/backend_connection/backend_connection_config.dart';
import 'package:amitia_app/core/backend_connection/backend_connection_credential.dart';
import 'package:amitia_app/core/backend_connection/backend_connection_endpoint.dart';
import 'package:amitia_app/core/backend_transport/connectivity/backend_connectivity_probe.dart';
import 'package:amitia_app/core/runtime/backend/device_agent_remote_core_probe.dart';
import 'package:flutter_test/flutter_test.dart';

void main() {
  test('cloud probe uses the authenticated local device-agent proxy', () async {
    final token = 'a' * 32;
    final server = await HttpServer.bind(InternetAddress.loopbackIPv4, 0);
    addTearDown(() => server.close(force: true));
    final requests = <String>[];
    final authorization = <String?>[];
    server.listen((request) async {
      requests.add(request.uri.path);
      authorization.add(request.headers.value('x-amitia-local-token'));
      request.response.statusCode = HttpStatus.ok;
      request.response.write('{"status":"ready"}');
      await request.response.close();
    });
    final config = BackendConnectionConfig(
      schemaVersion: 1,
      generation: 1,
      endpoint: BackendConnectionEndpoint(
        host: '127.0.0.1',
        port: server.port,
        httpScheme: 'http',
        webSocketScheme: 'ws',
        livenessPath: '/livez',
        readinessPath: '/readyz',
        pathPrefix: '/internal/device-mesh/provider',
      ),
      authStrategy: BackendAuthStrategy.localToken,
      credential: BackendConnectionCredential.tryCreate(token)!,
    );
    final probe = DeviceAgentRemoteCoreProbe(
      resolveLocal: () async => BackendConnectionAvailable(config),
    );

    final result = await probe.probe(Uri.parse('https://remote.example'));

    expect(result, BackendConnectivityResult.ready);
    expect(requests, <String>['/internal/device-mesh/provider/readyz']);
    expect(authorization, <String?>[token]);
  });

  test(
    'runtime-local probe config still uses the cloud proxy prefix',
    () async {
      final token = 'b' * 32;
      final server = await HttpServer.bind(InternetAddress.loopbackIPv4, 0);
      addTearDown(() => server.close(force: true));
      final requests = <String>[];
      server.listen((request) async {
        requests.add(request.uri.path);
        request.response.statusCode = HttpStatus.ok;
        request.response.write('{"status":"ready"}');
        await request.response.close();
      });
      final config = BackendConnectionConfig(
        schemaVersion: 1,
        generation: 1,
        endpoint: BackendConnectionEndpoint(
          host: '127.0.0.1',
          port: server.port,
          httpScheme: 'http',
          webSocketScheme: 'ws',
          livenessPath: '/livez',
          readinessPath: '/readyz',
        ),
        authStrategy: BackendAuthStrategy.localToken,
        credential: BackendConnectionCredential.tryCreate(token)!,
      );
      final probe = DeviceAgentRemoteCoreProbe(
        resolveLocal: () async => BackendConnectionAvailable(config),
      );

      final result = await probe.probe(Uri.parse('https://remote.example'));

      expect(result, BackendConnectivityResult.ready);
      expect(requests, <String>['/internal/device-mesh/provider/readyz']);
    },
  );
}
