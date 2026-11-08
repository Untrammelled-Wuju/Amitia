import 'dart:convert';
import 'dart:io';

import 'package:amitia_app/core/backend_connection/backend_connection_config.dart';
import 'package:amitia_app/core/backend_connection/backend_connection_credential.dart';
import 'package:amitia_app/core/backend_connection/backend_connection_endpoint.dart';
import 'package:amitia_app/core/backend_transport/backend_service_api.dart';
import 'package:amitia_app/core/backend_transport/errors/backend_transport_error.dart';
import 'package:amitia_app/core/backend_transport/http/backend_http_client.dart';
import 'package:amitia_app/core/errors/original_error.dart';
import 'package:amitia_app/core/services/device_owned_memory_service.dart';
import 'package:flutter_test/flutter_test.dart';

void main() {
  test('真实 Core 上的角色数 0 → 1 → 0 不再阻断最近记忆', () async {
    final root = Platform.environment['AMITIA_QA_DATA_DIR'];
    if (root == null || !File('$root/.qa-restore-isolated').existsSync()) {
      throw StateError('必须指定带标记的隔离 AMITIA_QA_DATA_DIR');
    }
    final endpoint = jsonDecode(
      File('$root/run/endpoint.json').readAsStringSync(),
    ) as Map<String, dynamic>;
    final uri = Uri.parse(endpoint['httpBaseURL'] as String);
    if (uri.host != '127.0.0.1') throw StateError('必须使用环回 Core');
    final credential = BackendConnectionCredential.tryCreate(
      File('$root/security/local-token').readAsStringSync().trim(),
    )!;
    final transport = BackendHttpClient(
      BackendConnectionConfig(
        schemaVersion: 1,
        generation: 1,
        endpoint: BackendConnectionEndpoint(
          host: uri.host,
          port: uri.port,
          httpScheme: 'http',
          webSocketScheme: 'ws',
          livenessPath: '/livez',
          readinessPath: '/readyz',
        ),
        authStrategy: BackendAuthStrategy.localToken,
        credential: credential,
      ),
    );
    addTearDown(transport.close);
    final api = BackendServiceApi(transport, 1);
    final service = DeviceOwnedMemoryService(api, isCurrent: () => true);
    const rolesPath = '/api/device-mesh/v1/business/roles';
    final before = await api.get<Map<String, dynamic>>(rolesPath);
    expect(before?['roles'], isEmpty);
    expect((await service.pageMemories()).items, isEmpty);
    await expectLater(
      service.query('', memoryView: 'all'),
      throwsA(
        isA<BackendTransportError>()
            .having((error) => error.statusCode, 'status', 409)
            .having(originalErrorText, 'original error', contains('没有可用角色')),
      ),
    );
    final created = await api.post<Map<String, dynamic>>(
      '/api/characters',
      data: {'name': 'QA 真 Core 最近记忆'},
    );
    final id = created!['id'] as String;
    try {
      final available = await api.get<Map<String, dynamic>>(rolesPath);
      expect(available?['roles'], hasLength(1));
      final page = await service.pageMemories();
      expect(page.items, isEmpty);
      expect(page.nextCursor, isEmpty);
    } finally {
      await api.delete('/api/characters/$id');
    }
    final after = await api.get<Map<String, dynamic>>(rolesPath);
    expect(after?['roles'], isEmpty);
    expect((await service.pageMemories()).items, isEmpty);
  });
}
