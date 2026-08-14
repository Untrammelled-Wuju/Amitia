import 'package:flutter_test/flutter_test.dart';
import 'package:amitia_app/core/backend_transport/backend_service_api.dart';
import 'package:amitia_app/core/services/character_service.dart';

void main() {
  group('CharacterService Read Mapping', () {
    test('list() maps GET /api/characters response to CharacterDto', () async {
      final fakeApi = _FakeBackendServiceApi(responseData: [
        {
          'id': 'char-001',
          'name': 'Amitia',
          'avatar': 'amitia.png',
          'identity': '心理模拟伙伴',
          'personality': '温和、敏锐',
          'speakingStyle': '简洁回应',
          'description': '主角',
          'status': 'enabled',
          'isActive': 1,
          'createdAt': '2026-01-01 10:00:00',
          'voiceType': 'zh_female',
          'voiceSpeed': 1.0,
        },
        {
          'id': 'char-002',
          'name': 'TestCharacter',
          'avatar': '',
          'identity': '测试角色',
          'personality': '测试人格',
          'speakingStyle': '直接',
          'description': '测试',
          'status': 'enabled',
          'isActive': 0,
          'createdAt': '2026-01-02 12:00:00',
          'voiceType': null,
          'voiceSpeed': null,
        },
      ]);

      final service = CharacterService(fakeApi);
      final result = await service.list();

      expect(result, hasLength(2));
      expect(result[0].id, 'char-001');
      expect(result[0].name, 'Amitia');
      expect(result[0].identity, '心理模拟伙伴');
      expect(result[0].isActive, 1);
      expect(result[0].voiceType, 'zh_female');
      expect(result[0].voiceSpeed, 1.0);
      expect(result[1].id, 'char-002');
      expect(result[1].name, 'TestCharacter');
      expect(result[1].isActive, 0);
      expect(result[1].voiceType, isNull);
      expect(result[1].voiceSpeed, isNull);
      expect(fakeApi.requestCount, 1);
      expect(fakeApi.lastPath, '/api/characters');
    });

    test('list() returns empty list when response is null', () async {
      final fakeApi = _FakeBackendServiceApi(responseData: null);
      final service = CharacterService(fakeApi);

      final result = await service.list();

      expect(result, isEmpty);
      expect(fakeApi.requestCount, 1);
    });

    test('getById() maps GET /api/characters/:id to CharacterDto', () async {
      final fakeApi = _FakeBackendServiceApi(responseData: {
        'id': 'char-003',
        'name': 'Specific',
        'avatar': '',
        'identity': '特定角色',
        'personality': '特定人格',
        'speakingStyle': '特定风格',
        'description': '',
        'status': 'enabled',
        'isActive': 0,
        'createdAt': '2026-02-01 08:00:00',
      });

      final service = CharacterService(fakeApi);
      final result = await service.getById('char-003');

      expect(result, isNotNull);
      expect(result!.id, 'char-003');
      expect(result.name, 'Specific');
      expect(result.identity, '特定角色');
      expect(fakeApi.requestCount, 1);
      expect(fakeApi.lastPath, '/api/characters/char-003');
    });
  });

  group('CharacterService Write Mapping', () {
    test('create() sends POST /api/characters with correct payload', () async {
      final fakeApi = _FakeBackendServiceApi(responseData: {
        'id': 'char-new-001',
        'name': 'NewCharacter',
        'avatar': '',
        'identity': '新角色',
        'personality': '新人格',
        'speakingStyle': '新风格',
        'description': '描述',
        'status': 'enabled',
        'isActive': 0,
        'createdAt': '2026-08-14 15:00:00',
      });

      final service = CharacterService(fakeApi);
      final result = await service.create({
        'name': 'NewCharacter',
        'identity': '新角色',
        'personality': '新人格',
        'speakingStyle': '新风格',
        'description': '描述',
      });

      expect(result, isNotNull);
      expect(result!.id, 'char-new-001');
      expect(result.name, 'NewCharacter');
      expect(fakeApi.requestCount, 1);
      expect(fakeApi.lastMethod, 'POST');
      expect(fakeApi.lastPath, '/api/characters');
      expect(fakeApi.lastBody, isNotNull);
      final body = fakeApi.lastBody as Map<String, dynamic>;
      expect(body['name'], 'NewCharacter');
      expect(body['identity'], '新角色');
    });

    test('update() sends PUT /api/characters/:id with correct payload',
        () async {
      final fakeApi = _FakeBackendServiceApi(responseData: {
        'id': 'char-update-001',
        'name': 'UpdatedName',
        'avatar': '',
        'identity': '更新身份',
        'personality': '更新人格',
        'speakingStyle': '更新风格',
        'description': '更新描述',
        'status': 'enabled',
        'isActive': 1,
        'createdAt': '2026-08-14 16:00:00',
      });

      final service = CharacterService(fakeApi);
      final result = await service.update('char-update-001', {
        'name': 'UpdatedName',
        'identity': '更新身份',
      });

      expect(result, isNotNull);
      expect(result!.name, 'UpdatedName');
      expect(result.identity, '更新身份');
      expect(fakeApi.requestCount, 1);
      expect(fakeApi.lastMethod, 'PUT');
      expect(fakeApi.lastPath, '/api/characters/char-update-001');
    });

    test('delete() sends DELETE /api/characters/:id', () async {
      final fakeApi = _FakeBackendServiceApi(responseData: null);
      final service = CharacterService(fakeApi);

      final result = await service.delete('char-delete-001');

      expect(result, isTrue);
      expect(fakeApi.requestCount, 1);
      expect(fakeApi.lastMethod, 'DELETE');
      expect(fakeApi.lastPath, '/api/characters/char-delete-001');
    });
  });

  group('CharacterService Read-After-Write Mapping', () {
    test('read-after-write: list after create shows new character', () async {
      final callCount = [0];
      final characters = <Map<String, dynamic>>[];

      final fakeApi = _DynamicFakeBackendServiceApi(
        onGet: (path) {
          if (path == '/api/characters') {
            return [
              ...characters,
              {
                'id': 'char-existing',
                'name': 'Existing',
                'avatar': '',
                'identity': '',
                'personality': '',
                'speakingStyle': '',
                'description': '',
                'status': 'enabled',
                'isActive': 0,
                'createdAt': '2026-01-01 00:00:00',
              },
            ];
          }
          return null;
        },
        onPost: (path, data) {
          callCount[0]++;
          final body = data is Map ? data as Map<String, dynamic> : const {};
          final created = {
            'id': 'char-e2e-${callCount[0]}',
            'name': body['name'],
            'avatar': '',
            'identity': body['identity'] ?? '',
            'personality': '',
            'speakingStyle': '',
            'description': '',
            'status': 'enabled',
            'isActive': 0,
            'createdAt': '2026-08-14 17:00:00',
          };
          characters.add(created);
          return created;
        },
      );

      final service = CharacterService(fakeApi);

      // Read1: baseline
      final before = await service.list();
      expect(before, hasLength(1));
      expect(before[0].name, 'Existing');

      // Write: create with unique marker
      final created = await service.create({
        'name': 'amitia-runtime-e2e-test',
        'identity': 'E2E测试角色',
      });
      expect(created, isNotNull);
      expect(created!.name, 'amitia-runtime-e2e-test');

      // Read2: should see new character
      final after = await service.list();
      expect(after, hasLength(2));
      expect(any(after, (c) => c.name == 'amitia-runtime-e2e-test'), isTrue);
      expect(any(after, (c) => c.identity == 'E2E测试角色'), isTrue);

      expect(fakeApi.requestCount, 3);
    });
  });
}

bool any<T>(List<T> list, bool Function(T) predicate) {
  for (final item in list) {
    if (predicate(item)) return true;
  }
  return false;
}

class _FakeBackendServiceApi implements BackendServiceApi {
  final dynamic responseData;
  int requestCount = 0;
  String? lastMethod;
  String? lastPath;
  Object? lastBody;

  _FakeBackendServiceApi({required this.responseData});

  void _record(String method, String path, [Object? body]) {
    requestCount++;
    lastMethod = method;
    lastPath = path;
    lastBody = body;
  }

  dynamic _wrapResponse() {
    if (responseData == null) return null;
    return {'code': 200, 'message': 'success', 'data': responseData};
  }

  @override
  int get generation => 1;

  @override
  Future<T?> get<T>(String path,
      {Map<String, dynamic>? queryParameters,
      T Function(dynamic)? fromJson}) async {
    _record('GET', path);
    final wrapped = _wrapResponse();
    if (wrapped == null) return null;
    final data = wrapped['data'];
    if (fromJson != null && data != null) return fromJson(data);
    return data as T?;
  }

  @override
  Future<T?> post<T>(String path,
      {Object? data, T Function(dynamic)? fromJson}) async {
    _record('POST', path, data);
    final wrapped = _wrapResponse();
    if (wrapped == null) return null;
    final responseData = wrapped['data'];
    if (fromJson != null && responseData != null) {
      return fromJson(responseData);
    }
    return responseData as T?;
  }

  @override
  Future<T?> put<T>(String path,
      {Object? data, T Function(dynamic)? fromJson}) async {
    _record('PUT', path, data);
    final wrapped = _wrapResponse();
    if (wrapped == null) return null;
    final responseData = wrapped['data'];
    if (fromJson != null && responseData != null) {
      return fromJson(responseData);
    }
    return responseData as T?;
  }

  @override
  Future<void> delete(String path) async {
    _record('DELETE', path);
  }

  @override
  Future<T?> deleteWithResponse<T>(String path,
      {Object? data, T Function(dynamic)? fromJson}) async {
    _record('DELETE', path, data);
    final wrapped = _wrapResponse();
    if (wrapped == null) return null;
    final responseData = wrapped['data'];
    if (fromJson != null && responseData != null) {
      return fromJson(responseData);
    }
    return responseData as T?;
  }
}

class _DynamicFakeBackendServiceApi implements BackendServiceApi {
  final dynamic Function(String path)? onGet;
  final dynamic Function(String path, Object? data)? onPost;
  int requestCount = 0;
  String? lastMethod;
  String? lastPath;
  Object? lastBody;

  _DynamicFakeBackendServiceApi({this.onGet, this.onPost});

  @override
  int get generation => 1;

  @override
  Future<T?> get<T>(String path,
      {Map<String, dynamic>? queryParameters,
      T Function(dynamic)? fromJson}) async {
    requestCount++;
    lastMethod = 'GET';
    lastPath = path;
    final data = onGet?.call(path);
    if (data == null) return null;
    if (fromJson != null) return fromJson(data);
    return data as T?;
  }

  @override
  Future<T?> post<T>(String path,
      {Object? data, T Function(dynamic)? fromJson}) async {
    requestCount++;
    lastMethod = 'POST';
    lastPath = path;
    lastBody = data;
    final responseData = onPost?.call(path, data);
    if (responseData == null) return null;
    if (fromJson != null) return fromJson(responseData);
    return responseData as T?;
  }

  @override
  Future<T?> put<T>(String path,
      {Object? data, T Function(dynamic)? fromJson}) async {
    requestCount++;
    lastMethod = 'PUT';
    lastPath = path;
    lastBody = data;
    return null;
  }

  @override
  Future<void> delete(String path) async {
    requestCount++;
    lastMethod = 'DELETE';
    lastPath = path;
  }

  @override
  Future<T?> deleteWithResponse<T>(String path,
      {Object? data, T Function(dynamic)? fromJson}) async {
    requestCount++;
    lastMethod = 'DELETE';
    lastPath = path;
    return null;
  }
}
