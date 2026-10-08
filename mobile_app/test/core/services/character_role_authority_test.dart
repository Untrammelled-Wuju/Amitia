import 'package:amitia_app/core/backend_transport/backend_service_api.dart';
import 'package:amitia_app/core/services/character_service.dart';
import 'package:amitia_app/core/services/character_detail_service.dart';
import 'package:flutter_test/flutter_test.dart';

class _OwnerApi extends Fake implements BackendServiceApi {
  String owner = 'a' * 64;
  final names = <String, String>{'a' * 64: '设备角色', 'b' * 64: 'Core 角色'};
  int writes = 0;
  bool switchAfterRead = false;

  @override
  Future<T?> get<T>(
    String path, {
    Map<String, dynamic>? queryParameters,
    Map<String, String>? headers,
    T Function(dynamic)? fromJson,
  }) async {
    final data = {
      'id': 'same-id',
      'name': names[owner],
      'roleAuthority': owner,
    };
    if (switchAfterRead) owner = 'b' * 64;
    return fromJson != null ? fromJson(data) : data as T;
  }

  @override
  Future<T?> post<T>(
    String path, {
    Object? data,
    Map<String, dynamic>? queryParameters,
    Map<String, String>? headers,
    T Function(dynamic)? fromJson,
  }) async {
    if (headers?['X-Amitia-Role-Authority'] != owner) {
      throw StateError('角色数据归属已变化');
    }
    writes++;
    final result = {
      'id': 'same-id',
      'name': names[owner],
      'roleAuthority': owner,
    };
    return fromJson != null ? fromJson(result) : result as T;
  }

  @override
  Future<T?> put<T>(
    String path, {
    Object? data,
    Map<String, dynamic>? queryParameters,
    Map<String, String>? headers,
    T Function(dynamic)? fromJson,
  }) async {
    if (headers?['X-Amitia-Role-Authority'] != owner)
      throw StateError('角色数据归属已变化');
    writes++;
    names[owner] = (data as Map)['name'] as String;
    final result = {
      'id': 'same-id',
      'name': names[owner],
      'roleAuthority': owner,
    };
    return fromJson != null ? fromJson(result) : result as T;
  }

  @override
  Future<T?> postMultipart<T>(
    String path, {
    Map<String, String> fields = const {},
    Map<String, List<String>> files = const {},
    Map<String, dynamic>? queryParameters,
    T Function(dynamic)? fromJson,
  }) async {
    if (fields['roleAuthority'] != owner) throw StateError('角色数据归属已变化');
    writes++;
    final result = {'avatarUrl': '/avatars/new.png'};
    return fromJson != null ? fromJson(result) : result as T;
  }
}

void main() {
  test('加载设备角色后切换 Core，不会修改同 ID 的 Core 角色', () async {
    final api = _OwnerApi();
    final service = CharacterService(api);
    final original = (await service.getById('same-id'))!;
    api.owner = 'b' * 64;
    await expectLater(
      service.update(original.id, {
        'name': '旧编辑',
      }, roleAuthority: original.roleAuthority),
      throwsStateError,
    );
    expect(api.names[api.owner], 'Core 角色');
    expect(api.writes, 0);
    api.owner = original.roleAuthority;
    await service.update(original.id, {
      'name': '新名称',
    }, roleAuthority: original.roleAuthority);
    expect(api.names[api.owner], '新名称');
  });

  test('旧头像上传携带原数据归属，切换后拒绝', () async {
    final api = _OwnerApi();
    final details = CharacterDetailService(api);
    final role = (await details.character('same-id'))!;
    api.owner = 'b' * 64;
    await expectLater(
      details.uploadAvatar(
        'same-id',
        '/avatar.png',
        roleAuthority: role['roleAuthority'] as String,
      ),
      throwsStateError,
    );
    expect(api.writes, 0);
  });

  test('无归属的旧编辑对象不能发起有意图的更新', () async {
    final api = _OwnerApi();
    await expectLater(
      CharacterService(
        api,
      ).update('same-id', {'name': '旧编辑'}, roleAuthority: ''),
      throwsStateError,
    );
    expect(api.writes, 0);
  });

  test('新建草稿捕获的归属在 Core 切换后仍用于提交', () async {
    final api = _OwnerApi();
    final service = CharacterService(api);
    final authority = await service.authority();
    api.owner = 'b' * 64;
    await expectLater(
      service.create({'name': '设备草稿'}, roleAuthority: authority),
      throwsStateError,
    );
    expect(api.writes, 0);
  });

  test('复制角色读到同 ID 的新归属记录时拒绝复制', () async {
    final api = _OwnerApi();
    final service = CharacterService(api);
    final source = (await service.getById('same-id'))!;
    api.owner = 'b' * 64;
    await expectLater(
      service.duplicate(source.id, roleAuthority: source.roleAuthority),
      throwsStateError,
    );
    expect(api.writes, 0);
  });

  test('复制角色在读取后切换 Core 仍携带原归属', () async {
    final api = _OwnerApi();
    final service = CharacterService(api);
    final source = (await service.getById('same-id'))!;
    api.switchAfterRead = true;
    await expectLater(
      service.duplicate(source.id, roleAuthority: source.roleAuthority),
      throwsStateError,
    );
    expect(api.writes, 0);
  });

  test('激活旧角色不能激活新 Core 的同 ID 角色', () async {
    final api = _OwnerApi();
    final service = CharacterService(api);
    final source = (await service.getById('same-id'))!;
    api.owner = 'b' * 64;
    await expectLater(
      service.setActive(source.id, roleAuthority: source.roleAuthority),
      throwsStateError,
    );
    expect(api.writes, 0);
  });
}
