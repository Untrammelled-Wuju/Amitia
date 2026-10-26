import 'package:amitia_app/core/backend_transport/backend_service_api.dart';
import 'package:amitia_app/core/services/device_owned_chat_service.dart';
import 'package:amitia_app/core/services/owned_memory_scope.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:shared_preferences/shared_preferences.dart';

class _MemoryApi implements BackendServiceApi {
  final requests = <Map<String, dynamic>>[];
  final resources = <Map<String, dynamic>>[];
  Map<String, dynamic> scope() => {
    'spaceId': 'space',
    'initiatorDeviceId': 'a',
    'targetDeviceId': 'a',
    'coreId': 'core',
    'providerEpoch': 1,
    'targetProviderEpoch': 1,
    'coordinated': false,
    'modeRevision': 1,
    'permissionRevision': 1,
    'targetPermissionRevision': 1,
    'roleId': 'role',
    'roleRevision': 1,
    'authorizationRealm': 'space',
    'roleOwnerId': 'a',
    'resourceOwnerId': 'a',
    'requestId': 'read',
    'turnId': 'turn',
    'executionId': 'execution',
    'memoryRoleId': 'role',
    'memoryViewAll': true,
  };

  @override
  Future<T?> get<T>(
    String path, {
    Map<String, dynamic>? queryParameters,
    Map<String, String>? headers,
    T Function(dynamic)? fromJson,
  }) async {
    if (path.endsWith('/coordination/me'))
      return {
            'coreId': 'core',
            'coordinationAvailable': true,
            'policy': {
              'selectedRole': 'role',
              'coordinated': false,
              'providerEpoch': 1,
              'modeRevision': 1,
              'permissionRevision': 1,
            },
          }
          as T;
    if (path.endsWith('/roles'))
      return {
            'roles': [
              {'id': 'role', 'revision': 1},
            ],
          }
          as T;
    if (path.endsWith('/data'))
      return {
            'executionScope': scope(),
            'snapshot': {
              'ownerId': 'a',
              'resources': [
                {
                  'kind': 'memory',
                  'id': 'mem',
                  'ownerId': 'a',
                  'roleId': 'role',
                  'revision': 1,
                  'body': {
                    'content': {'value': '杭州'},
                  },
                },
              ],
            },
          }
          as T;
    if (path.endsWith('/business/resources')) {
      resources.add(Map<String, dynamic>.from(queryParameters ?? {}));
      return {
            'executionScope': {...scope(), 'memoryViewAll': false},
            'resource': {
              'kind': 'memory',
              'id': 'mem',
              'ownerId': 'a',
              'roleId': 'role',
              'revision': 1,
              'body': {
                'content': {'value': '杭州'},
              },
            },
          }
          as T;
    }
    throw StateError('非预期接口 $path');
  }

  @override
  Future<T?> post<T>(
    String path, {
    Object? data,
    Map<String, dynamic>? queryParameters,
    Map<String, String>? headers,
    T Function(dynamic)? fromJson,
  }) async {
    final request = Map<String, dynamic>.from(data as Map);
    requests.add(request);
    return {
          'ownerId': 'a',
          'requestId': request['requestId'],
          'versions': {'memory/mem': 2},
        }
        as T;
  }

  @override
  dynamic noSuchMethod(Invocation invocation) => super.noSuchMethod(invocation);
}

void main() {
  setUp(() => SharedPreferences.setMockInitialValues({}));

  test('全部记忆视图下的记录可以直接删除且不携带视图选择字段', () async {
    final api = _MemoryApi();
    final service = DeviceOwnedChatService(api, providerKey: () => 'cloud');
    await service.refresh();
    final page = await service.data('memory', characterId: 'role', memoryView: 'all');
    final row = (page['snapshot'] as Map)['resources'] as List;
    final expected = memoryRecordScope(
      page['executionScope'] as Map,
      (row.single as Map)['roleId'].toString(),
    );
    await service.edit(
      'memory',
      'mem',
      deleted: true,
      characterId: 'role',
      expectedScope: expected,
      expectedRevision: 1,
    );
    final payload = api.requests.single;
    expect(
      (payload['expectedExecutionScope'] as Map).containsKey('memoryViewAll'),
      isFalse,
    );
    expect((payload['expectedExecutionScope'] as Map)['memoryRoleId'], 'role');
    expect(api.resources, isEmpty);
  });

  test('归属比较忽略仅由视图派生的 memoryViewAll', () async {
    final api = _MemoryApi();
    final service = DeviceOwnedChatService(api, providerKey: () => 'cloud');
    await service.refresh();
    await service.data('memory', characterId: 'role', memoryView: 'all');
    await service.edit(
      'memory',
      'mem',
      deleted: true,
      characterId: 'role',
      expectedScope: api.scope(),
      expectedRevision: 1,
    );
    expect(api.requests.single['deleted'], isTrue);
  });
}
