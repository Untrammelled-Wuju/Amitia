import 'package:flutter_test/flutter_test.dart';
import 'package:amitia_app/core/backend_transport/backend_service_api.dart';
import 'package:amitia_app/core/services/continuity_service.dart';

class _PagingApi implements BackendServiceApi {
  final pages = <Map<String, dynamic>>[];
  final cursors = <String>[];
  final writes = <Map<String, dynamic>>[];
  int mode = 1;
  bool wrongCheckpoint = false;
  bool coordinated = false;
  final historyPages = <Map<String, dynamic>>[];

  @override
  Future<T?> get<T>(
    String path, {
    Map<String, dynamic>? queryParameters,
    Map<String, String>? headers,
    T Function(dynamic)? fromJson,
  }) async {
    if (path.endsWith('/coordination/me')) {
      return {
            'coreId': 'core',
            'coordinationAvailable': true,
            'policy': {
              'selectedRole': 'role',
              'modeRevision': mode,
              'permissionRevision': 1,
              'providerEpoch': 1,
              'coordinated': coordinated,
            },
          }
          as T;
    }
    if (path.endsWith('/historical-roles'))
      return {
            'roles': [
              {'id': 'old-role'},
            ],
          }
          as T;
    if (path.endsWith('/data')) return historyPages.removeAt(0) as T;
    if (path.endsWith('/roles')) {
      return {
            'roleOwnerId': 'device',
            'roles': [
              {'id': 'role', 'revision': 1},
            ],
          }
          as T;
    }
    cursors.add(queryParameters?['cursor']?.toString() ?? '');
    return pages.removeAt(0) as T;
  }

  @override
  Future<T?> post<T>(
    String path, {
    dynamic data,
    Map<String, dynamic>? queryParameters,
    Map<String, String>? headers,
    T Function(dynamic)? fromJson,
  }) async {
    final payload = Map<String, dynamic>.from(data as Map);
    writes.add(payload);
    final id = payload['id'] ?? 'continuity/${payload['requestId']}';
    final revision = (payload['expectedRevision'] as int) + 1;
    return {
          'document': {
            'thread': {'id': id, 'characterId': 'role', 'revision': revision},
            'ownerId': 'device',
            'executionScope': {
              ...payload['expectedExecutionScope'] as Map,
              'requestId': payload['requestId'],
            },
          },
          'acknowledgement': {
            'ownerId': 'device',
            'requestId': payload['requestId'],
            'versions': {
              'continuity/$id': revision,
              'checkpoint/continuity-operation/${payload['requestId']}':
                  wrongCheckpoint ? 0 : 1,
            },
          },
        }
        as T;
  }

  @override
  dynamic noSuchMethod(Invocation invocation) => super.noSuchMethod(invocation);
}

Map<String, dynamic> _page(String id, {String cursor = '', int mode = 1}) {
  final scope = {
    'spaceId': 'core',
    'authorizationRealm': 'core',
    'initiatorDeviceId': 'device',
    'targetDeviceId': 'device',
    'providerEpoch': 1,
    'permissionRevision': 1,
    'targetProviderEpoch': 1,
    'targetPermissionRevision': 1,
    'coordinated': false,
    'roleOwnerId': 'device',
    'roleRevision': 1,
    'coreId': 'core',
    'resourceOwnerId': 'device',
    'roleId': 'role',
    'modeRevision': mode,
    'requestId': id,
  };
  return {
    'executionScope': scope,
    'documents': [
      {
        'thread': {
          'id': id,
          'title': id,
          'characterId': 'role',
          'status': 'active',
          'revision': 1,
        },
        'ownerId': 'device',
        'executionScope': {...scope, 'modeRevision': 1},
        'managementExecutionScope': scope,
      },
    ],
    'nextCursor': cursor,
  };
}

void main() {
  test('历史 Source 持续事项同 ID 保留独立只读快照且不能恢复执行', () async {
    final page = _page('same');
    (page['executionScope'] as Map)['coordinated'] = true;
    ((page['documents'] as List).single as Map)['managementExecutionScope'] =
        page['executionScope'];
    final original = _page('same')['documents'][0] as Map;
    final api = _PagingApi()
      ..coordinated = true
      ..pages.add(page)
      ..historyPages.add({
        'executionScope': page['executionScope'],
        'historicalSnapshot': {
          'ownerId': 'source',
          'resources': [
            {'kind': 'continuity', 'id': 'same', 'body': original},
          ],
          'nextCursors': {},
        },
      });
    final service = ContinuityService(api);
    final rows = await service.list();
    expect(rows.length, 2);
    final old = rows.last.sourceDocument!;
    expect(old['readOnly'], isTrue);
    final detail = await service.get('same', expectedDocument: old);
    expect(detail.thread.sourceDocument!['ownerId'], 'source');
    await expectLater(
      service.update('same', {'status': 'active'}, expectedDocument: old),
      throwsStateError,
    );
    expect(api.writes, isEmpty);
  });
  test('历史 Source 读取期间 scope 变化拒绝混入当前列表', () async {
    final page = _page('same');
    (page['executionScope'] as Map)['coordinated'] = true;
    ((page['documents'] as List).single as Map)['managementExecutionScope'] =
        page['executionScope'];
    final api = _PagingApi()
      ..coordinated = true
      ..pages.add(page)
      ..historyPages.add({
        'executionScope': {
          ...page['executionScope'] as Map,
          'permissionRevision': 2,
        },
        'historicalSnapshot': {
          'ownerId': 'source',
          'resources': [],
          'nextCursors': {},
        },
      });
    await expectLater(ContinuityService(api).list(), throwsStateError);
  });
  test('重新打开旧授权事项使用当前管理范围并保留原执行范围', () async {
    final api = _PagingApi()
      ..mode = 2
      ..pages.add(_page('original', mode: 2));
    final service = ContinuityService(api);
    final rows = await service.list();
    final document = rows.single.sourceDocument!;
    expect(document['executionScope']['modeRevision'], 2);
    expect(document['persistedExecutionScope']['modeRevision'], 1);
    await service.update('original', {
      'status': 'active',
    }, expectedDocument: document);
    expect(api.writes.single['expectedExecutionScope']['modeRevision'], 2);
  });

  test('旧页面同设备模式ABA拒绝发送写请求', () async {
    final api = _PagingApi()..pages.add(_page('original'));
    final service = ContinuityService(api);
    final original = (await service.list()).single.sourceDocument!;
    api.mode = 3;
    await expectLater(
      service.update('original', {
        'status': 'active',
      }, expectedDocument: original),
      throwsStateError,
    );
    expect(api.writes, isEmpty);
  });

  test('缺少持续事项checkpoint保存回执不报告成功', () async {
    final api = _PagingApi()
      ..wrongCheckpoint = true
      ..pages.add(_page('original'));
    final service = ContinuityService(api);
    final original = (await service.list()).single.sourceDocument!;
    await expectLater(
      service.update('original', {
        'title': 'modified',
      }, expectedDocument: original),
      throwsStateError,
    );
  });
  test('持续事项读取所有分页并去重', () async {
    final api = _PagingApi()
      ..pages.addAll([_page('first', cursor: 'next'), _page('second')]);
    final rows = await ContinuityService(api).list();
    expect(rows.map((row) => row.id), ['first', 'second']);
    expect(api.cursors, ['', 'next']);
  });

  test('持续事项分页期间模式变化拒绝混入结果', () async {
    final api = _PagingApi()
      ..pages.addAll([
        _page('first', cursor: 'next'),
        _page('second', mode: 2),
      ]);
    await expectLater(ContinuityService(api).list(), throwsStateError);
  });

  test('持续事项重复游标不会无限加载', () async {
    final api = _PagingApi()
      ..pages.addAll([
        _page('first', cursor: 'next'),
        _page('second', cursor: 'next'),
      ]);
    await expectLater(ContinuityService(api).list(), throwsStateError);
    expect(api.cursors.length, 2);
  });
}
