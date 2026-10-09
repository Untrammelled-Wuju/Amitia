import 'package:flutter_test/flutter_test.dart';
import 'package:amitia_app/core/services/device_management_intent.dart';
import 'package:amitia_app/core/services/device_mesh_service.dart';
import 'package:amitia_app/core/backend_transport/backend_service_api.dart';

class _DeviceManagementApi implements BackendServiceApi {
  final List<String> requests = [];

  @override
  Future<T?> get<T>(
    String path, {
    Map<String, dynamic>? queryParameters,
    Map<String, String>? headers,
    T Function(dynamic)? fromJson,
  }) async {
    requests.add(path);
    if (path.endsWith('/coordination/me')) return state(available: false) as T;
    if (path.endsWith('/devices'))
      return {
            'devices': [
              {'deviceId': 'device-a', 'label': '测试设备'},
            ],
          }
          as T;
    throw StateError('unexpected request');
  }

  @override
  dynamic noSuchMethod(Invocation invocation) => super.noSuchMethod(invocation);
}

Map<String, dynamic> state({
  bool admin = true,
  bool coordinated = true,
  bool available = true,
  String core = 'core-b',
  int permission = 1,
}) => {
  'coordinationAvailable': available,
  'coreId': core,
  'canAdminister': admin,
  'policy': <String, dynamic>{
    'deviceId': 'device-a',
    'coordinated': coordinated,
    'providerEpoch': 2,
    'modeRevision': 3,
    'permissionRevision': permission,
  },
};

void main() {
  test('设备列表加载链路在统筹业务未就绪时仍能完成', () async {
    final api = _DeviceManagementApi();
    final service = DeviceMeshService(api);
    final intent = DeviceManagementIntent(
      await service.coordination(),
      isCurrent: () => true,
    );
    final devices = await service.devices();
    intent.validate(await service.coordination());
    expect(devices.single['deviceId'], 'device-a');
    expect(api.requests, [
      '/api/device-mesh/v1/coordination/me',
      '/api/device-mesh/v1/devices',
      '/api/device-mesh/v1/coordination/me',
    ]);
  });
  test('原管理员意图携带 Core 与完整权限版本', () {
    final intent = DeviceManagementIntent(state(), isCurrent: () => true);
    intent.requireAdministrator();
    intent.requireTarget('device-c');
    expect(intent.headers, {
      'X-Amitia-Expected-Core-ID': 'core-b',
      'X-Amitia-Expected-Configuration-Policy': '2:3:1',
    });
    intent.validate(state());
  });

  test('普通设备只可管理自身授权', () {
    final intent = DeviceManagementIntent(
      state(admin: false, coordinated: false),
      isCurrent: () => true,
    );
    intent.requireTarget('device-a');
    expect(() => intent.requireTarget('device-c'), throwsStateError);
    expect(intent.requireAdministrator, throwsStateError);
  });

  test('统筹关闭即使声明管理员也不能操作管理员入口', () {
    final intent = DeviceManagementIntent(
      state(coordinated: false),
      isCurrent: () => true,
    );
    expect(intent.requireAdministrator, throwsStateError);
    intent.validate(state(coordinated: false));
    intent.requireTarget('device-a');
    expect(() => intent.requireTarget('device-c'), throwsStateError);
  });

  test('统筹业务未就绪时仍能确认设备管理身份权限', () {
    final intent = DeviceManagementIntent(
      state(available: false),
      isCurrent: () => true,
    );
    intent.validate(state(available: false));
    intent.validate(state(available: true));
    expect(intent.headers['X-Amitia-Expected-Core-ID'], 'core-b');
  });

  test('统筹关闭时原始管理员权限变化仍使快照失效', () {
    final intent = DeviceManagementIntent(
      state(coordinated: false, available: false),
      isCurrent: () => true,
    );
    intent.validate(state(coordinated: false, available: false));
    expect(
      () => intent.validate(
        state(admin: false, coordinated: false, available: false),
      ),
      throwsStateError,
    );
  });

  test('业务未就绪不放宽身份字段校验', () {
    final invalidStates = [
      state(available: false, core: ''),
      {...state(available: false), 'canAdminister': null},
      {...state(available: false), 'policy': null},
    ];
    for (final field in ['deviceId', 'coordinated']) {
      final invalid = state(available: false);
      (invalid['policy'] as Map)[field] = null;
      invalidStates.add(invalid);
    }
    for (final invalid in invalidStates) {
      expect(
        () => DeviceManagementIntent(invalid, isCurrent: () => true),
        throwsStateError,
      );
    }
  });

  test('统筹业务未就绪仍拒绝 Core 与权限版本变化', () {
    final intent = DeviceManagementIntent(
      state(available: false),
      isCurrent: () => true,
    );
    expect(
      () => intent.validate(state(available: false, core: 'other-core')),
      throwsStateError,
    );
    for (final field in [
      'providerEpoch',
      'modeRevision',
      'permissionRevision',
    ]) {
      final changed = state(available: false);
      (changed['policy'] as Map)[field] += 1;
      expect(() => intent.validate(changed), throwsStateError);
    }
    expect(
      () => intent.validate(state(available: false, coordinated: false)),
      throwsStateError,
    );
  });

  test('同 Core 权限撤销重新授予使原意图失效', () {
    final intent = DeviceManagementIntent(state(), isCurrent: () => true);
    expect(() => intent.validate(state(permission: 3)), throwsStateError);
    expect(() => intent.validate(state(core: 'core-c')), throwsStateError);
  });

  test('自身授权确认仅允许精确权限版本递增一次', () {
    final intent = DeviceManagementIntent(
      state(admin: false),
      isCurrent: () => true,
    );
    intent.validate(state(admin: false, permission: 2), permissionIncrement: 1);
    expect(
      () => intent.validate(
        state(admin: false, permission: 3),
        permissionIncrement: 1,
      ),
      throwsStateError,
    );
    final switched = state(admin: false, permission: 2);
    (switched['policy'] as Map)['providerEpoch'] = 3;
    expect(
      () => intent.validate(switched, permissionIncrement: 1),
      throwsStateError,
    );
  });

  test('原连接变化后不接受新 Core 同 ID 权限', () {
    var current = true;
    final intent = DeviceManagementIntent(state(), isCurrent: () => current);
    current = false;
    expect(() => intent.validate(state()), throwsStateError);
    expect(intent.requireAdministrator, throwsStateError);
  });

  test('缺失或无效权限版本拒绝读取权限上下文', () {
    for (final value in [null, 0, -1, 1.5, 9007199254740992]) {
      final malformed = state();
      (malformed['policy'] as Map)['permissionRevision'] = value;
      expect(
        () => DeviceManagementIntent(malformed, isCurrent: () => true),
        throwsStateError,
      );
    }
  });

  test('权限快照不会被后续原对象修改覆盖', () {
    final original = state();
    final intent = DeviceManagementIntent(original, isCurrent: () => true);
    (original['policy'] as Map)['permissionRevision'] = 3;
    expect(intent.headers['X-Amitia-Expected-Configuration-Policy'], '2:3:1');
    expect(() => intent.validate(original), throwsStateError);
  });
}
