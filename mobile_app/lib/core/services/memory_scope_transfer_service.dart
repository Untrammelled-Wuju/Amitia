import '../backend_transport/backend_service_api.dart';
import 'device_owned_memory_service.dart';

Future<void> transferMemoryScope(
  BackendServiceApi api,
  Map record,
  String targetScope,
  String targetRole,
) async {
  final scope = record['executionScope'] as Map;
  final generation = api.generation;
  final requestId = DeviceOwnedMemoryService.requestId();
  final kind = record['kind'].toString();
  final id = record['id'].toString();
  final revision = record['revision'] as int;
  final state = await api.get<Map<String, dynamic>>(
    '/api/device-mesh/v1/coordination/me',
  );
  if (api.generation != generation ||
      state?['coreId'] != scope['coreId'] ||
      const [
        'providerEpoch',
        'modeRevision',
        'permissionRevision',
        'coordinated',
      ].any((key) => state?['policy']?[key] != scope[key]))
    throw StateError('记忆服务归属已变化，请重新加载');
  final ack = await api.post<Map<String, dynamic>>(
    '/api/device-mesh/v1/business/memory-scope/transfer',
    data: {
      'requestId': requestId,
      'characterId': scope['roleId'],
      'targetDeviceId': scope['targetDeviceId'],
      'expectedExecutionScope': scope,
      'kind': kind,
      'id': id,
      'expectedRevision': revision,
      'targetScope': targetScope,
      if (targetScope == 'character') 'targetRoleId': targetRole,
    },
  );
  if (api.generation != generation ||
      ack?['ownerId'] != record['ownerId'] ||
      ack?['requestId'] != requestId ||
      ack?['versions']?['$kind/$id'] != revision + 1)
    throw StateError('数据所有者尚未确认迁移，请重新加载核对');
}
