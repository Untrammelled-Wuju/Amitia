import 'package:flutter_test/flutter_test.dart';
import 'package:amitia_app/core/backend_transport/backend_service_api.dart';
import 'package:amitia_app/core/services/continuity_service.dart';

class _PagingApi implements BackendServiceApi {
  final pages = <Map<String, dynamic>>[];
  final cursors = <String>[];

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
            'policy': {'selectedRole': 'role', 'modeRevision': 1},
          }
          as T;
    }
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
  dynamic noSuchMethod(Invocation invocation) => super.noSuchMethod(invocation);
}

Map<String, dynamic> _page(String id, {String cursor = '', int mode = 1}) => {
  'executionScope': {
    'coreId': 'core',
    'resourceOwnerId': 'device',
    'roleId': 'role',
    'modeRevision': mode,
    'requestId': id,
  },
  'documents': [
    {
      'thread': {
        'id': id,
        'title': id,
        'characterId': 'role',
        'status': 'active',
      },
    },
  ],
  'nextCursor': cursor,
};

void main() {
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
