import 'package:dio/dio.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:amitia_app/core/backend_transport/backend_service_api.dart';
import 'package:amitia_app/core/backend_transport/routed_backend_service_api.dart';
import 'package:amitia_app/core/backend_transport/role_management_target.dart';

class _RecordingApi implements BackendServiceApi {
  final List<_RoutedCall> calls = [];

  @override
  int get generation => 1;

  @override
  Future<T?> get<T>(
    String path, {
    Map<String, dynamic>? queryParameters,
    Map<String, String>? headers,
    T Function(dynamic)? fromJson,
  }) async {
    calls.add(_RoutedCall('GET', path, headers, null));
    return null;
  }

  @override
  Future<T?> post<T>(
    String path, {
    Object? data,
    Map<String, dynamic>? queryParameters,
    Map<String, String>? headers,
    T Function(dynamic)? fromJson,
  }) async {
    calls.add(_RoutedCall('POST', path, headers, data));
    return null;
  }

  @override
  Future<T?> postPayload<T>(
    String path, {
    Object? data,
    Map<String, String>? headers,
    T Function(dynamic)? fromJson,
  }) async {
    calls.add(_RoutedCall('POST', path, headers, data));
    return null;
  }

  @override
  Future<T?> put<T>(
    String path, {
    Object? data,
    Map<String, dynamic>? queryParameters,
    Map<String, String>? headers,
    T Function(dynamic)? fromJson,
  }) async {
    calls.add(_RoutedCall('PUT', path, headers, data));
    return null;
  }

  @override
  Future<T?> patch<T>(
    String path, {
    Object? data,
    Map<String, dynamic>? queryParameters,
    Map<String, String>? headers,
    T Function(dynamic)? fromJson,
  }) async {
    calls.add(_RoutedCall('PATCH', path, headers, data));
    return null;
  }

  @override
  Future<void> delete(
    String path, {
    Map<String, dynamic>? queryParameters,
    Map<String, String>? headers,
  }) async {
    calls.add(_RoutedCall('DELETE', path, headers, null));
  }

  @override
  Future<T?> deleteWithResponse<T>(
    String path, {
    Object? data,
    Map<String, String>? headers,
    T Function(dynamic)? fromJson,
  }) async {
    calls.add(_RoutedCall('DELETE', path, headers, data));
    return null;
  }

  @override
  Future<Stream<List<int>>> getStream(
    String path, {
    Map<String, dynamic>? queryParameters,
    Map<String, String>? headers,
    CancelToken? cancelToken,
  }) async {
    calls.add(_RoutedCall('GET_STREAM', path, headers, null));
    return const Stream<List<int>>.empty();
  }

  @override
  Future<Stream<List<int>>> postStream(
    String path, {
    Object? data,
    Map<String, dynamic>? queryParameters,
    Map<String, String>? headers,
    CancelToken? cancelToken,
  }) async {
    calls.add(_RoutedCall('POST_STREAM', path, headers, data));
    return const Stream<List<int>>.empty();
  }

  @override
  Future<T?> postMultipart<T>(
    String path, {
    Map<String, String> fields = const {},
    Map<String, List<String>> files = const {},
    Map<String, dynamic>? queryParameters,
    Map<String, String>? headers,
    T Function(dynamic)? fromJson,
  }) async {
    calls.add(
      _RoutedCall('POST_MULTIPART', path, headers, <String, Object?>{
        'fields': fields,
        'files': files,
      }),
    );
    return null;
  }
}

class _RoutedCall {
  final String method;
  final String path;
  final Map<String, String>? headers;
  final Object? body;
  _RoutedCall(this.method, this.path, this.headers, this.body);
}

void main() {
  group('RoutedBackendServiceApiProxy routing', () {
    late _RecordingApi businessApi;
    late _RecordingApi deviceLocalApi;
    late RoutedBackendServiceApiProxy proxy;

    setUp(() {
      businessApi = _RecordingApi();
      deviceLocalApi = _RecordingApi();
      proxy = RoutedBackendServiceApiProxy(
        businessApi: businessApi,
        deviceLocalApi: deviceLocalApi,
      );
    });

    test('regular cloud API does not route to device agent', () async {
      await proxy.get('/api/characters');
      expect(businessApi.calls.length, 1);
      expect(deviceLocalApi.calls.length, 0);
      expect(businessApi.calls.first.path, '/api/characters');
    });

    test('runtime state uses the owned business API', () async {
      await proxy.get(
        '/api/device-mesh/v1/business/runtime-state',
        queryParameters: {'characterId': 'role'},
      );
      expect(businessApi.calls.length, 1);
      expect(deviceLocalApi.calls.length, 0);
      expect(
        businessApi.calls.first.path,
        '/api/device-mesh/v1/business/runtime-state',
      );
    });

    test(
      'knowledge search and diagnostics use the active business Core',
      () async {
        await proxy.post(
          '/api/knowledge/search',
          data: const {'query': '核对文档', 'routingMode': 'chat'},
        );
        await proxy.get('/api/knowledge/runtime');
        await proxy.get('/api/knowledge/vector-orphans');
        expect(businessApi.calls.length, 3);
        expect(deviceLocalApi.calls.length, 0);
        expect(
          businessApi.calls.map((call) => call.path),
          containsAll(<String>[
            '/api/knowledge/search',
            '/api/knowledge/runtime',
            '/api/knowledge/vector-orphans',
          ]),
        );
      },
    );

    test('desktop pet device API routes to device agent', () async {
      await proxy.get('/api/desktop-pets/installations');
      expect(deviceLocalApi.calls.length, 1);
      expect(businessApi.calls.length, 0);
      expect(
        deviceLocalApi.calls.first.path,
        '/api/desktop-pets/installations',
      );
    });

    test('desktop pet package management routes to device agent', () async {
      await proxy.get('/api/extensions/pet/plugins');
      await proxy.post(
        '/api/extensions/pet/plugins/install',
        data: <String, dynamic>{
          'packagePath':
              '/home/amitia/.amitia-imports/desktop-pet/qa-pet-v1.petx',
        },
      );
      expect(deviceLocalApi.calls.length, 2);
      expect(businessApi.calls.length, 0);
      expect(deviceLocalApi.calls.map((call) => call.path), <String>[
        '/api/extensions/pet/plugins',
        '/api/extensions/pet/plugins/install',
      ]);
      expect(deviceLocalApi.calls.last.headers?['Idempotency-Key'], isNotNull);
    });

    test('chat import staging routes to device agent', () async {
      await proxy.post(
        '/api/imports/confirm',
        data: <String, dynamic>{'characterId': 'role'},
      );
      expect(deviceLocalApi.calls.length, 1);
      expect(businessApi.calls.length, 0);
      expect(deviceLocalApi.calls.first.path, '/api/imports/confirm');
      expect(deviceLocalApi.calls.first.headers?['Idempotency-Key'], isNotNull);
    });

    test(
      'data portability migration routes to active data authority',
      () async {
        await proxy.post(
          '/api/data-portability/imports/import-1/commit',
          data: const {'policy': 'duplicate'},
        );
        expect(deviceLocalApi.calls.length, 0);
        expect(businessApi.calls.length, 1);
        expect(
          businessApi.calls.first.path,
          '/api/data-portability/imports/import-1/commit',
        );
      },
    );

    test('physical backup and restore routes to device agent', () async {
      await proxy.get('/api/data-portability/backups');
      await proxy.post(
        '/api/data-portability/restores/restore-1',
        data: const {},
      );
      expect(deviceLocalApi.calls.length, 2);
      expect(businessApi.calls.length, 0);
      expect(deviceLocalApi.calls.last.headers?['Idempotency-Key'], isNotNull);
    });

    test('desktop pet singular prefix routes to device agent', () async {
      await proxy.get('/api/desktop-pet/status');
      expect(deviceLocalApi.calls.length, 1);
      expect(businessApi.calls.length, 0);
    });

    test('device mesh internal API routes to device agent', () async {
      await proxy.get('/internal/device-mesh/status');
      expect(deviceLocalApi.calls.length, 1);
      expect(businessApi.calls.length, 0);
      expect(deviceLocalApi.calls.first.path, '/internal/device-mesh/status');
    });

    test('similar prefix does not mis-route', () async {
      await proxy.get('/api/desktop-petfoo');
      expect(businessApi.calls.length, 1);
      expect(deviceLocalApi.calls.length, 0);
    });

    test('exact prefix match without slash stays on business', () async {
      await proxy.get('/api/desktop-pets');
      expect(deviceLocalApi.calls.length, 1);
      expect(businessApi.calls.length, 0);
    });

    test('path with query string classifies correctly', () async {
      await proxy.get(
        '/api/desktop-pets/installations',
        queryParameters: {'active': 'true'},
      );
      expect(deviceLocalApi.calls.length, 1);
      expect(businessApi.calls.length, 0);
    });

    test('chat command stays on business core', () async {
      await proxy.post(
        '/api/web-chat/messages',
        data: const {'content': 'hello'},
      );
      expect(businessApi.calls.length, 1);
      expect(deviceLocalApi.calls.length, 0);
      expect(businessApi.calls.first.method, 'POST');
      expect(businessApi.calls.first.path, '/api/web-chat/messages');
    });

    test('conversation event stream stays on business core', () async {
      await proxy.getStream('/api/web-chat/conversations/conv-1/events');
      expect(businessApi.calls.length, 1);
      expect(deviceLocalApi.calls.length, 0);
      expect(businessApi.calls.first.method, 'GET_STREAM');
    });

    test('device-local stream still routes to device agent', () async {
      await proxy.getStream('/api/local/workflow-runs/run-1/events');
      expect(deviceLocalApi.calls.length, 1);
      expect(businessApi.calls.length, 0);
      expect(
        deviceLocalApi.calls.first.headers?['X-Amitia-Client-Type'],
        'mobile',
      );
    });

    test(
      'multipart follows the same business/device route classifier',
      () async {
        await proxy.postMultipart(
          '/api/characters/avatar',
          fields: const {'x': '1'},
        );
        await proxy.postMultipart(
          '/api/storage/upload',
          fields: const {'x': '2'},
        );
        expect(businessApi.calls.length, 1);
        expect(deviceLocalApi.calls.length, 1);
        expect(businessApi.calls.first.method, 'POST_MULTIPART');
        expect(deviceLocalApi.calls.first.method, 'POST_MULTIPART');
      },
    );
  });

  group('RoutedBackendServiceApiProxy headers', () {
    late _RecordingApi businessApi;
    late _RecordingApi deviceLocalApi;
    late RoutedBackendServiceApiProxy proxy;

    setUp(() {
      businessApi = _RecordingApi();
      deviceLocalApi = _RecordingApi();
      proxy = RoutedBackendServiceApiProxy(
        businessApi: businessApi,
        deviceLocalApi: deviceLocalApi,
      );
    });

    test('GET to device local sets X-Amitia-Client-Type mobile', () async {
      await proxy.get('/api/desktop-pets/installations');
      final headers = deviceLocalApi.calls.first.headers;
      expect(headers, isNotNull);
      expect(headers!['X-Amitia-Client-Type'], 'mobile');
    });

    test('cloud GET does not set X-Amitia-Client-Type', () async {
      await proxy.get('/api/characters');
      final headers = businessApi.calls.first.headers;
      expect(headers?['X-Amitia-Client-Type'], isNull);
    });

    test('POST to device local auto-generates Idempotency-Key', () async {
      await proxy.post(
        '/api/desktop-pets/installations',
        data: {'action': 'enable'},
      );
      final headers = deviceLocalApi.calls.first.headers;
      expect(headers, isNotNull);
      expect(headers!['Idempotency-Key'], isNotNull);
      expect(headers['Idempotency-Key']!, startsWith('mobile-'));
    });

    test('PUT to device local auto-generates Idempotency-Key', () async {
      await proxy.put(
        '/api/desktop-pets/installations/123',
        data: {'enabled': true},
      );
      final headers = deviceLocalApi.calls.first.headers;
      expect(headers, isNotNull);
      expect(headers!['Idempotency-Key'], isNotNull);
    });

    test('PATCH to device local auto-generates Idempotency-Key', () async {
      await proxy.patch(
        '/api/desktop-pets/installations/123',
        data: {'alpha': 0.5},
      );
      final headers = deviceLocalApi.calls.first.headers;
      expect(headers, isNotNull);
      expect(headers!['Idempotency-Key'], isNotNull);
    });

    test('DELETE to device local auto-generates Idempotency-Key', () async {
      await proxy.delete('/api/desktop-pets/installations/123');
      final headers = deviceLocalApi.calls.first.headers;
      expect(headers, isNotNull);
      expect(headers!['Idempotency-Key'], isNotNull);
    });

    test('GET to device local does not generate Idempotency-Key', () async {
      await proxy.get('/api/desktop-pets/installations');
      final headers = deviceLocalApi.calls.first.headers;
      expect(headers?['Idempotency-Key'], isNull);
    });

    test('caller-supplied Idempotency-Key is not overwritten', () async {
      await proxy.post(
        '/api/desktop-pets/installations',
        data: {'action': 'enable'},
        headers: {'Idempotency-Key': 'caller-supplied-key-123'},
      );
      final headers = deviceLocalApi.calls.first.headers;
      expect(headers!['Idempotency-Key'], 'caller-supplied-key-123');
    });

    test('cloud POST does not get Idempotency-Key', () async {
      await proxy.post('/api/characters', data: {'name': 'test'});
      final headers = businessApi.calls.first.headers;
      expect(headers?['Idempotency-Key'], isNull);
    });

    test('Idempotency-Key is unique per mutation call', () async {
      await proxy.post('/api/desktop-pets/a', data: {});
      await proxy.post('/api/desktop-pets/b', data: {});
      final key1 = deviceLocalApi.calls[0].headers!['Idempotency-Key']!;
      final key2 = deviceLocalApi.calls[1].headers!['Idempotency-Key']!;
      expect(key1, isNot(equals(key2)));
    });
  });

  group('RoutedBackendServiceApiProxy role target', () {
    late _RecordingApi businessApi;
    late _RecordingApi deviceLocalApi;

    RoutedBackendServiceApiProxy buildProxy({required bool cloud}) =>
        RoutedBackendServiceApiProxy(
          businessApi: businessApi,
          deviceLocalApi: deviceLocalApi,
          isCloudDeployment: () => cloud,
        );

    setUp(() {
      businessApi = _RecordingApi();
      deviceLocalApi = _RecordingApi();
    });

    test('未指定来源时角色管理仍走统筹中心', () async {
      await buildProxy(cloud: true).get('/api/characters');
      expect(businessApi.calls.length, 1);
      expect(deviceLocalApi.calls.length, 0);
    });

    test('选择本机时角色与团队写入落在设备本地', () async {
      final proxy = buildProxy(cloud: true);
      await proxy.post(
        '/api/characters',
        data: const {'name': '本机角色'},
        headers: withRoleManagementTarget(null, RoleManagementTarget.local),
      );
      await proxy.post(
        '/api/roles/teams',
        data: const {'name': '本机团队'},
        headers: withRoleManagementTarget(null, RoleManagementTarget.local),
      );
      expect(deviceLocalApi.calls.length, 2);
      expect(businessApi.calls.length, 0);
      expect(deviceLocalApi.calls.first.path, '/api/characters');
      expect(deviceLocalApi.calls.last.path, '/api/roles/teams');
    });

    test('来源请求头不会透传给后端', () async {
      await buildProxy(cloud: true).get(
        '/api/characters',
        headers: {
          ...withRoleManagementTarget(
            const {'X-Amitia-Client-Type': 'mobile'},
            RoleManagementTarget.local,
          )!,
        },
      );
      final headers = deviceLocalApi.calls.first.headers;
      expect(headers?[roleManagementTargetHeader], isNull);
      expect(headers?['X-Amitia-Client-Type'], 'mobile');
    });

    test('选择统筹中心时角色管理走业务核心', () async {
      await buildProxy(cloud: true).get(
        '/api/characters',
        headers: withRoleManagementTarget(null, RoleManagementTarget.center),
      );
      expect(businessApi.calls.length, 1);
      expect(deviceLocalApi.calls.length, 0);
    });

    test('未接入统筹中心时拒绝使用统筹中心角色数据', () async {
      await expectLater(
        buildProxy(cloud: false).get(
          '/api/characters',
          headers: withRoleManagementTarget(null, RoleManagementTarget.center),
        ),
        throwsA(isA<StateError>()),
      );
      expect(businessApi.calls.length, 0);
      expect(deviceLocalApi.calls.length, 0);
    });

    test('MCP 默认停留在设备本地', () async {
      final proxy = buildProxy(cloud: true);
      await proxy.get('/api/mcp/servers');
      await proxy.post(
        '/api/mcp/servers',
        data: const {'name': '本机服务'},
        headers: withRoleManagementTarget(null, RoleManagementTarget.local),
      );
      expect(deviceLocalApi.calls.length, 2);
      expect(businessApi.calls.length, 0);
      expect(deviceLocalApi.calls.first.path, '/api/mcp/servers');
    });

    test('MCP 选择统筹中心时改写到统筹中心控制台路由', () async {
      final proxy = buildProxy(cloud: true);
      await proxy.post(
        '/api/mcp/servers',
        data: const {'name': '中心服务'},
        headers: withRoleManagementTarget(null, RoleManagementTarget.center),
      );
      await proxy.get(
        '/api/mcp/servers/center-1/tools',
        headers: withRoleManagementTarget(null, RoleManagementTarget.center),
      );
      expect(businessApi.calls.length, 2);
      expect(deviceLocalApi.calls.length, 0);
      expect(
        businessApi.calls.first.path,
        '/api/device-mesh/v1/business/console/mcp/servers',
      );
      expect(
        businessApi.calls.last.path,
        '/api/device-mesh/v1/business/console/mcp/servers/center-1/tools',
      );
      expect(
        businessApi.calls.last.headers?[roleManagementTargetHeader],
        isNull,
      );
      expect(businessApi.calls.last.headers?['Idempotency-Key'], isNull);
    });

    test('未接入统筹中心时拒绝使用统筹中心 MCP', () async {
      await expectLater(
        buildProxy(cloud: false).get(
          '/api/mcp/servers',
          headers: withRoleManagementTarget(null, RoleManagementTarget.center),
        ),
        throwsA(isA<StateError>()),
      );
      expect(businessApi.calls.length, 0);
      expect(deviceLocalApi.calls.length, 0);
    });

    test('skill 默认停留在设备本地', () async {
      final proxy = buildProxy(cloud: true);
      await proxy.get('/api/extensions/kernel/agent-skills');
      await proxy.post(
        '/api/extensions/kernel/agent-skills/import/preview',
        data: const {'path': '/tmp/skill'},
        headers: withRoleManagementTarget(null, RoleManagementTarget.local),
      );
      expect(deviceLocalApi.calls.length, 2);
      expect(businessApi.calls.length, 0);
      expect(
        deviceLocalApi.calls.first.path,
        '/api/extensions/kernel/agent-skills',
      );
      expect(deviceLocalApi.calls.last.headers?['Idempotency-Key'], isNotNull);
    });

    test('skill 选择统筹中心时改写到统筹中心控制台路由', () async {
      final proxy = buildProxy(cloud: true);
      await proxy.get(
        '/api/extensions/kernel/agent-skills',
        headers: withRoleManagementTarget(null, RoleManagementTarget.center),
      );
      await proxy.post(
        '/api/extensions/kernel/agent-skills/center-1/enable',
        data: const {},
        headers: withRoleManagementTarget(null, RoleManagementTarget.center),
      );
      expect(businessApi.calls.length, 2);
      expect(deviceLocalApi.calls.length, 0);
      expect(
        businessApi.calls.first.path,
        '/api/device-mesh/v1/business/console/skills',
      );
      expect(
        businessApi.calls.last.path,
        '/api/device-mesh/v1/business/console/skills/center-1/enable',
      );
      expect(
        businessApi.calls.last.headers?[roleManagementTargetHeader],
        isNull,
      );
      expect(businessApi.calls.last.headers?['Idempotency-Key'], isNull);
    });

    test('未接入统筹中心时拒绝使用统筹中心 skill', () async {
      await expectLater(
        buildProxy(cloud: false).get(
          '/api/extensions/kernel/agent-skills',
          headers: withRoleManagementTarget(null, RoleManagementTarget.center),
        ),
        throwsA(isA<StateError>()),
      );
      expect(businessApi.calls.length, 0);
      expect(deviceLocalApi.calls.length, 0);
    });

    test('任务默认停留在设备本地', () async {
      final proxy = buildProxy(cloud: true);
      await proxy.get('/api/reminders?kind=user_reminder');
      await proxy.post(
        '/api/reminders',
        data: const {'title': '喝水'},
        headers: withRoleManagementTarget(null, RoleManagementTarget.local),
      );
      expect(deviceLocalApi.calls.length, 2);
      expect(businessApi.calls.length, 0);
      expect(deviceLocalApi.calls.first.path, '/api/reminders?kind=user_reminder');
    });

    test('任务选择统筹中心时走统筹中心业务核心', () async {
      final proxy = buildProxy(cloud: true);
      await proxy.get(
        '/api/reminders',
        queryParameters: const {'kind': 'user_reminder'},
        headers: withRoleManagementTarget(null, RoleManagementTarget.center),
      );
      await proxy.put(
        '/api/reminders/task-1',
        data: const {'title': '喝水'},
        headers: withRoleManagementTarget(null, RoleManagementTarget.center),
      );
      expect(businessApi.calls.length, 2);
      expect(deviceLocalApi.calls.length, 0);
      expect(businessApi.calls.first.path, '/api/reminders');
      expect(businessApi.calls.last.path, '/api/reminders/task-1');
      expect(
        businessApi.calls.last.headers?[roleManagementTargetHeader],
        isNull,
      );
    });

    test('未接入统筹中心时拒绝使用统筹中心任务', () async {
      await expectLater(
        buildProxy(cloud: false).get(
          '/api/reminders',
          headers: withRoleManagementTarget(null, RoleManagementTarget.center),
        ),
        throwsA(isA<StateError>()),
      );
      expect(businessApi.calls.length, 0);
      expect(deviceLocalApi.calls.length, 0);
    });
  });
}
