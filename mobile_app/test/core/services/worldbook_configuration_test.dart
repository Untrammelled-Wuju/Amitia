import 'package:amitia_app/core/backend_transport/backend_service_api.dart';
import 'package:amitia_app/core/services/device_owned_chat_service.dart';
import 'package:amitia_app/core/services/worldbook_service.dart';
import 'package:flutter_test/flutter_test.dart';

class WorldbookApi extends Fake implements BackendServiceApi {
  final requests = <String>[];
  final payloads = <Map<String, dynamic>>[];
  String ownerId = 'device-a';

  @override
  int get generation => 1;

  @override
  Future<T?> post<T>(
    String path, {
    dynamic data,
    Map<String, dynamic>? queryParameters,
    Map<String, String>? headers,
    T Function(dynamic)? fromJson,
  }) async {
    requests.add(path);
    final payload = Map<String, dynamic>.from(data as Map);
    payloads.add(payload);
    final requestId = payload['requestId'].toString();
    final rule = payload['rule'] is Map
        ? Map<String, dynamic>.from(payload['rule'] as Map)
        : <String, dynamic>{};
    return <String, dynamic>{
          'data': {
            'saved': true,
            'executionScope': {
              ...scope(),
              'requestId': requestId,
              'worldbookRoleId': rule['scope'] == 'global'
                  ? '@global-worldbook'
                  : 'one',
            },
            'acknowledgement': {
              'ownerId': ownerId,
              'requestId': requestId,
              'versions': {'checkpoint/worldbook/$requestId': 1},
            },
            if (payload['action'] != 'delete')
              'rule': {
                ...rule,
                'id': rule['id'] ?? 'created',
                'ownerId': ownerId,
                'roleId': rule['scope'] == 'global'
                    ? '@global-worldbook'
                    : 'one',
                'revision': 1,
              },
            if (path.endsWith('/match')) 'matches': [],
          },
        }
        as T;
  }
}

class FakeOwned extends DeviceOwnedChatService {
  FakeOwned(super.api);

  @override
  bool get enabled => true;

  @override
  String get coreId => 'b';

  @override
  String get notice => '';

  @override
  Future<bool> refresh() async => true;

  @override
  String selectRole(String? requested) => 'one';

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
    return {
      'executionScope': scope(),
      'snapshot': {
        'ownerId': 'device-a',
        'resources': [
          {
            'kind': 'worldbook',
            'id': 'same-id',
            'roleId': 'one',
            'ownerId': 'device-a',
            'revision': 2,
            'body': {
              'scope': 'role',
              'matchType': 'keyword',
              'matchPattern': '晨星',
              'matchScope': 'full_context',
              'injectContent': '原设备内容',
              'priority': 1,
              'enabled': true,
            },
          },
        ],
      },
    };
  }
}

Map<String, dynamic> scope() => {
  'spaceId': 'space',
  'initiatorDeviceId': 'device-a',
  'targetDeviceId': 'b',
  'coreId': 'b',
  'providerEpoch': 1,
  'coordinated': true,
  'modeRevision': 1,
  'permissionRevision': 1,
  'roleOwnerId': 'device-a',
  'resourceOwnerId': 'device-a',
  'roleId': 'one',
  'roleRevision': 1,
};

void main() {
  late WorldbookApi api;
  late WorldBookService service;

  setUp(() {
    api = WorldbookApi();
    service = WorldBookService(FakeOwned(api), api);
  });

  test('手机端读取设备所有者世界书并保留修订', () async {
    final entries = await service.list();
    expect(entries.single.id, 'same-id');
    expect(entries.single.ownerId, 'device-a');
    expect(entries.single.revision, 2);
    expect(api.requests, isEmpty);
  });

  test('手机端写入携带原范围并核验所有者确认', () async {
    await service.create({
      'scope': 'role',
      'matchType': 'keyword',
      'matchPattern': '茶',
      'matchScope': 'full_context',
      'injectContent': '喜欢茶',
    });
    expect(api.requests, ['/api/device-mesh/v1/business/worldbook/manage']);
    expect(api.payloads.single['expectedExecutionScope'], scope());
    expect((api.payloads.single['rule'] as Map)['characterId'], 'one');
  });

  test('所有者确认不匹配时不显示保存成功', () async {
    api.ownerId = 'other';
    await expectLater(
      service.create({
        'scope': 'role',
        'matchType': 'keyword',
        'matchPattern': '茶',
        'injectContent': '喜欢茶',
      }),
      throwsStateError,
    );
  });

  test('手机端世界书不再调用旧接口', () async {
    await service.list();
    await service.testMatch('晨星');
    expect(
      api.requests,
      everyElement('/api/device-mesh/v1/business/worldbook/match'),
    );
    expect(api.requests.any((path) => path == '/api/world-book'), isFalse);
  });
}
