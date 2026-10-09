import 'dart:convert';
import 'package:dio/dio.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:amitia_app/core/backend_transport/backend_service_api.dart';
import 'package:amitia_app/core/services/device_mesh_service.dart';

Map<String, dynamic> callScope() => {
  'spaceId': 'core-b',
  'coreId': 'core-b',
  'initiatorDeviceId': 'caller',
  'targetDeviceId': 'target',
  'resourceOwnerId': 'target',
  'roleOwnerId': 'target',
  'roleId': 'role',
  'roleRevision': 3,
  'providerEpoch': 1,
  'targetProviderEpoch': 1,
  'modeRevision': 1,
  'permissionRevision': 1,
  'targetPermissionRevision': 1,
  'coordinated': false,
  'authorizationRealm': 'core-b',
  'requestId': 'request',
  'turnId': 'turn',
  'executionId': 'execution',
};

class _CallApi implements BackendServiceApi {
  int stateReads = 0;
  bool changedProvider = false;
  bool denied = false;
  Map<String, dynamic>? payload;
  Map<String, String>? requestHeaders;
  bool malformedGrant = false;
  List<Map<String, dynamic>> events = [];

  @override
  Future<T?> get<T>(
    String path, {
    Map<String, dynamic>? queryParameters,
    Map<String, String>? headers,
    T Function(dynamic)? fromJson,
  }) async {
    if (path.endsWith('/coordination/me')) {
      stateReads++;
      return {
            'coreId': changedProvider && stateReads > 1 ? 'core-c' : 'core-b',
            'coordinationAvailable': true,
          }
          as T;
    }
    if (denied) throw StateError('目标设备未授权此调用能力');
    return {
          'roles': [
            {'id': 'role', 'name': '目标角色', 'revision': 3},
          ],
          'roleOwnerId': 'target',
          'executionScope': callScope(),
        }
        as T;
  }

  @override
  Future<Stream<List<int>>> postStream(
    String path, {
    Object? data,
    Map<String, dynamic>? queryParameters,
    Map<String, String>? headers,
    CancelToken? cancelToken,
  }) async {
    payload = Map<String, dynamic>.from(data as Map);
    return Stream.fromIterable(
      events.map(
        (event) =>
            utf8.encode('event: message\ndata: ${jsonEncode(event)}\n\n'),
      ),
    );
  }

  @override
  Future<T?> put<T>(
    String path, {
    Object? data,
    Map<String, dynamic>? queryParameters,
    Map<String, String>? headers,
    T Function(dynamic)? fromJson,
  }) async {
    payload = Map<String, dynamic>.from(data as Map);
    requestHeaders = headers;
    return {
          'grant': {
            'targetId': malformedGrant ? 'other-target' : 'target',
            'callerId': payload!['callerId'],
            'capability': payload!['capability'],
            'allowed': payload!['allowed'],
            'revision': (payload!['expectedRevision'] as int) + 1,
          },
        }
        as T;
  }

  @override
  dynamic noSuchMethod(Invocation invocation) => super.noSuchMethod(invocation);
}

void main() {
  test('手机跨设备准备拒绝在两次请求之间切换 Core', () async {
    final api = _CallApi()..changedProvider = true;
    await expectLater(
      DeviceMeshService(api).prepareCall('target'),
      throwsStateError,
    );
    expect(api.payload, isNull);
  });

  test('未授权目标角色时不发起调用', () async {
    final api = _CallApi()..denied = true;
    await expectLater(
      DeviceMeshService(api).prepareCall('target'),
      throwsStateError,
    );
    expect(api.payload, isNull);
  });

  test('手机跨设备调用携带目标角色和完整权限快照，收到保存确认才结束', () async {
    final api = _CallApi();
    final scope = callScope();
    api.events = [
      {'type': 'started', 'executionScope': scope},
      {'type': 'delta', 'executionScope': scope, 'text': '回复'},
      {
        'type': 'completed',
        'data': {
          'requestId': 'request',
          'executionScope': scope,
          'saved': true,
          'reply': '回复',
        },
      },
    ];
    final service = DeviceMeshService(api);
    final roles = await service.prepareCall('target');
    expect(roles['roleOwnerId'], 'target');
    final events = await service
        .call(
          target: 'target',
          role: 'role',
          core: 'core-b',
          owner: 'target',
          requestId: 'request',
          message: '执行',
          expectedScope: scope,
          cancelToken: CancelToken(),
        )
        .toList();
    expect(api.payload, {
      'targetDeviceId': 'target',
      'characterId': 'role',
      'requestId': 'request',
      'message': '执行',
      'expectedExecutionScope': scope,
    });
    expect(events.last['data']['saved'], true);
  });

  test('手机拒绝其他目标或提供者的流并取消接收', () async {
    for (final field in [
      'coreId',
      'targetDeviceId',
      'resourceOwnerId',
      'roleId',
    ]) {
      final api = _CallApi();
      api.events = [
        {
          'type': 'started',
          'executionScope': {...callScope(), field: 'other'},
        },
      ];
      final cancel = CancelToken();
      await expectLater(
        DeviceMeshService(api)
            .call(
              target: 'target',
              role: 'role',
              core: 'core-b',
              owner: 'target',
              requestId: 'request',
              message: '执行',
              expectedScope: callScope(),
              cancelToken: cancel,
            )
            .toList(),
        throwsStateError,
      );
      expect(cancel.isCancelled, true);
    }
  });

  test('手机能力授权携带当前 Core 和原始授权版本', () async {
    final api = _CallApi();
    final headers = {
      'X-Amitia-Expected-Core-ID': 'core-b',
      'X-Amitia-Expected-Configuration-Policy': '1:2:3',
    };
    await DeviceMeshService(api).setCapabilityGrant(
      target: 'target',
      caller: 'caller',
      capability: 'ai.chat',
      allowed: false,
      expectedRevision: 4,
      expectedCoreId: 'core-b',
      headers: headers,
    );
    expect(api.requestHeaders, headers);
    expect(api.payload, {
      'callerId': 'caller',
      'capability': 'ai.chat',
      'allowed': false,
      'expectedRevision': 4,
      'expectedCoreId': 'core-b',
    });
  });

  test('能力授权拒绝其他目标的成功响应', () async {
    final api = _CallApi()..malformedGrant = true;
    await expectLater(
      DeviceMeshService(api).setCapabilityGrant(
        target: 'target',
        caller: 'caller',
        capability: 'ai.chat',
        allowed: false,
        expectedRevision: 4,
        expectedCoreId: 'core-b',
      ),
      throwsStateError,
    );
  });
}
