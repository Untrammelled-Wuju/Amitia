import 'package:amitia_app/core/backend_transport/backend_service_api.dart';
import 'package:amitia_app/core/models/profile.dart';
import 'package:amitia_app/core/services/device_owned_chat_service.dart';
import 'package:amitia_app/core/services/device_owned_memory_service.dart';
import 'package:amitia_app/core/services/profile_service.dart';
import 'package:flutter_test/flutter_test.dart';

import 'owned_task_submission_test.dart' show TaskTransport;

Map<String, dynamic> scope({String role = 'role', String memoryRole = 'role'}) => {
  'spaceId': 'core',
  'coreId': 'core',
  'authorizationRealm': 'core',
  'initiatorDeviceId': 'phone',
  'targetDeviceId': 'phone',
  'roleId': role,
  'roleRevision': 2,
  'roleOwnerId': 'phone',
  'resourceOwnerId': 'phone',
  'providerEpoch': 1,
  'targetProviderEpoch': 1,
  'modeRevision': 1,
  'permissionRevision': 1,
  'targetPermissionRevision': 1,
  'coordinated': false,
  'memoryRoleId': memoryRole,
  'requestId': 'read',
  'turnId': 'turn',
  'executionId': 'execution',
};

Map<String, dynamic> profileRow(String id, String key, String value) => {
  'kind': 'profile',
  'id': id,
  'ownerId': 'phone',
  'roleId': '@global-memory',
  'revision': 1,
  'updatedAt': '2026-10-26T00:00:00Z',
  'body': {
    'key': 'preference/$key',
    'content': {'value': value, 'memoryType': 'preference'},
  },
};

class ProfileSource extends DeviceOwnedChatService {
  ProfileSource(super.api) {
    enabled = true;
    roles = [
      {'id': 'role', 'name': '角色', 'revision': 2},
    ];
    policy = {'selectedRole': 'role', 'coordinated': false};
    coreId = 'core';
  }

  bool ready = true;
  final kinds = <String>[];
  final views = <String>[];
  List<Map<String, dynamic>> rows = [];

  @override
  Future<bool> refresh() async => ready;

  @override
  Future<Map<String, dynamic>> data(
    String kind, {
    required String characterId,
    String conversationId = '',
    String cursor = '',
    String legacyCursor = '',
    String historicalRoleId = '',
    String historicalCursor = '',
    String historicalLegacyCursor = '',
    String memoryView = '',
  }) async {
    kinds.add(kind);
    views.add(memoryView);
    if (!ready) throw StateError('设备数据服务尚未就绪');
    return {
      'executionScope': scope(),
      'snapshot': {
        'ownerId': 'phone',
        'resources': [
          ...rows,
          {
            'kind': 'memory',
            'id': 'ignored',
            'ownerId': 'phone',
            'roleId': 'role',
            'revision': 1,
            'body': {
              'key': '其他层',
              'content': {'value': '不应出现在画像里'},
            },
          },
        ],
        'nextCursors': {},
      },
    };
  }
}

class ProfileMemory extends DeviceOwnedMemoryService {
  ProfileMemory(super.api) : super(isCurrent: () => true);

  final queries = <String>[];
  final inputs = <Map<String, dynamic>>[];
  final originals = <Map<String, dynamic>>[];
  List<Map<String, dynamic>> conflicts = [];

  @override
  Future<Map<String, dynamic>> query(
    String characterId, {
    bool candidates = false,
    String query = '',
    String mode = 'keyword',
    String memoryType = '',
    String source = '',
    String sort = '',
    int retentionLevel = 0,
    bool? pinned,
    String decayState = '',
    String cursor = '',
    Map<String, dynamic>? original,
    int limit = 128,
    String memoryView = '',
  }) async {
    queries.add(memoryView);
    return {
      'resources': <Map<String, dynamic>>[],
      'executionScope': scope(memoryRole: '@global-memory'),
      'nextCursor': '',
    };
  }

  @override
  Future<Map<String, dynamic>> manage(
    Map<String, dynamic> original,
    Map<String, dynamic> input, {
    bool candidates = false,
    String? operationId,
  }) async {
    originals.add(original);
    inputs.add(input);
    if (conflicts.isNotEmpty) throw OwnedMemoryConflict(conflicts);
    return {
      'resources': [
        {
          ...profileRow(
            'memory-id',
            input['key'].toString(),
            input['value'].toString(),
          ),
          'body': {
            'key': '${input['memoryType']}/${input['key']}',
            'content': {
              'value': input['value'],
              'memoryType': input['memoryType'],
            },
          },
        },
      ],
      'executionScope': {...scope(memoryRole: '@global-memory'), 'requestId': 'write'},
      'saved': true,
      'acknowledgement': {'ownerId': 'phone', 'requestId': 'write', 'versions': {}},
    };
  }
}

ProfileService service(ProfileSource source, ProfileMemory memory) =>
    ProfileService(source, memory: () => memory);

void main() {
  test('画像列表只走统一设备数据通道并按分类与关键词过滤', () async {
    final source = ProfileSource(BackendServiceApi(TaskTransport([]), 1))
      ..rows = [
        profileRow('one', '饮品', '喜欢茶'),
        profileRow('two', '城市', '住在杭州'),
      ];
    final memory = ProfileMemory(BackendServiceApi(TaskTransport([]), 1));
    final rows = await service(source, memory).list();
    expect(source.kinds, ['profile']);
    expect(source.views.toSet(), {'global'});
    expect(rows.map((row) => row.id), ['one', 'two']);
    expect(rows.every((row) => row.category == 'preference'), isTrue);
    expect(rows.map((row) => row.attributeName), ['饮品', '城市']);
    final filtered = await service(source, memory).list(keyword: '杭州');
    expect(filtered.map((row) => row.id), ['two']);
    expect(await service(source, memory).list(category: 'fact'), isEmpty);
  });

  test('画像分页按页码截断而不是整表返回', () async {
    final source = ProfileSource(BackendServiceApi(TaskTransport([]), 1))
      ..rows = [
        profileRow('one', '饮品', '茶'),
        profileRow('two', '城市', '杭州'),
        profileRow('three', '宠物', '猫'),
      ];
    final memory = ProfileMemory(BackendServiceApi(TaskTransport([]), 1));
    final first = await service(source, memory).list(page: 1, pageSize: 2);
    final second = await service(source, memory).list(page: 2, pageSize: 2);
    expect(first.map((row) => row.id), ['one', 'two']);
    expect(second.map((row) => row.id), ['three']);
  });

  test('画像写入使用全局视图取得的真实范围而不是响应外层结构', () async {
    final source = ProfileSource(BackendServiceApi(TaskTransport([]), 1));
    final memory = ProfileMemory(BackendServiceApi(TaskTransport([]), 1));
    final dto = await service(source, memory).create({
      'characterId': 'role',
      'category': 'memory',
      'attributeName': '初始记忆',
      'attributeValue': '用户喜欢喝茶',
    });
    expect(memory.queries, ['global']);
    expect(memory.inputs.single['partition'], 'profile');
    expect(memory.inputs.single['key'], '初始记忆');
    final original = memory.originals.single;
    expect(original['executionScope'], isA<Map>());
    expect(
      (original['executionScope'] as Map)['resourceOwnerId'],
      'phone',
    );
    expect((original['executionScope'] as Map)['memoryRoleId'], '@global-memory');
    expect(dto, isA<ProfileDto>());
    expect(dto!.attributeValue, '用户喜欢喝茶');
    expect(dto.category, 'memory');
    expect(dto.attributeName, '初始记忆');
  });

  test('同键画像已存在时视为已保存，其他冲突继续抛出', () async {
    final source = ProfileSource(BackendServiceApi(TaskTransport([]), 1));
    final memory = ProfileMemory(BackendServiceApi(TaskTransport([]), 1))
      ..conflicts = [profileRow('existing', '初始记忆', '用户喜欢喝茶')];
    final dto = await service(source, memory).create({
      'characterId': 'role',
      'attributeName': '初始记忆',
      'attributeValue': '用户喜欢喝茶',
    });
    expect(dto!.id, 'existing');

    memory.conflicts = [profileRow('other', '别的键', '内容')];
    await expectLater(
      service(source, memory).create({
        'characterId': 'role',
        'attributeName': '初始记忆',
        'attributeValue': '用户喜欢喝茶',
      }),
      throwsA(isA<OwnedMemoryConflict>()),
    );
  });

  test('服务未就绪时画像读取与写入都直接失败且不产生写入请求', () async {
    final source = ProfileSource(BackendServiceApi(TaskTransport([]), 1))
      ..ready = false;
    final memory = ProfileMemory(BackendServiceApi(TaskTransport([]), 1));
    await expectLater(service(source, memory).list(), throwsStateError);
    await expectLater(
      service(source, memory).create({
        'characterId': 'role',
        'attributeName': '初始记忆',
        'attributeValue': '内容',
      }),
      throwsStateError,
    );
    expect(memory.inputs, isEmpty);
  });

  test('画像写入内容为空时拒绝提交', () async {
    final source = ProfileSource(BackendServiceApi(TaskTransport([]), 1));
    final memory = ProfileMemory(BackendServiceApi(TaskTransport([]), 1));
    await expectLater(
      service(source, memory).create({
        'characterId': 'role',
        'attributeName': ' ',
        'attributeValue': '内容',
      }),
      throwsStateError,
    );
    expect(memory.queries, isEmpty);
  });
}
