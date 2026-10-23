import 'package:amitia_app/core/backend_transport/backend_service_api.dart';
import 'package:amitia_app/core/services/temporal_service.dart';
import 'package:flutter_test/flutter_test.dart';

class _Api extends Fake implements BackendServiceApi {
  String owner = 'phone';
  String? path;
  Map<String, dynamic>? saved;
  Map<String, dynamic> view() => {
    'ownerId': owner,
    'roleId': 'role',
    'executionScope': {'resourceOwnerId': owner, 'roleId': 'role'},
    'settings': {'characterId': 'role', 'enabled': true, 'updatedAt': 'v1'},
    'state': {'interactionCount': 2},
    'episodes': [],
    'episode': {'id': 'old-episode'},
  };

  @override
  Future<T?> get<T>(
    String path, {
    Map<String, dynamic>? queryParameters,
    Map<String, String>? headers,
    T Function(dynamic)? fromJson,
  }) async {
    this.path = path;
    return view() as T;
  }

  @override
  Future<T?> put<T>(
    String path, {
    Object? data,
    Map<String, dynamic>? queryParameters,
    Map<String, String>? headers,
    T Function(dynamic)? fromJson,
  }) async {
    this.path = path;
    saved = Map<String, dynamic>.from(data as Map);
    return view() as T;
  }
}

void main() {
  test('关系时间使用 Owner 接口并保存原权限', () async {
    final api = _Api();
    final service = TemporalService(api);
    final settings = await service.relationshipTimeSettings('role');
    expect(
      (await service.relationshipTimeState('role'))?['interactionCount'],
      2,
    );
    await service.updateRelationshipTimeSettings('role', settings!);
    expect(api.path, '/api/device-mesh/v1/business/relationship-time');
    expect(
      (api.saved?['expectedExecutionScope'] as Map)['resourceOwnerId'],
      'phone',
    );
    expect(
      (await service.reunionEpisode('role', 'old-episode'))?['id'],
      'old-episode',
    );
  });

  test('刷新状态后仍阻止向不同 Owner 确认设置保存', () async {
    final api = _Api();
    final service = TemporalService(api);
    final settings = await service.relationshipTimeSettings('role');
    api.owner = 'core';
    await service.relationshipTimeState('role');
    await expectLater(
      service.updateRelationshipTimeSettings('role', settings!),
      throwsStateError,
    );
    expect(
      (api.saved?['expectedExecutionScope'] as Map)['resourceOwnerId'],
      'phone',
    );
  });
}
