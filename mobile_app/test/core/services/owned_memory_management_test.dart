import 'package:amitia_app/core/backend_transport/backend_service_api.dart';
import 'package:amitia_app/core/services/device_owned_memory_service.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:dio/dio.dart';
import 'package:amitia_app/core/backend_transport/errors/backend_transport_error.dart';
import 'package:amitia_app/core/backend_transport/errors/backend_transport_error_code.dart';
import 'owned_task_submission_test.dart' show TaskTransport;

const operation = '55a50b42-033c-4c49-a1d5-544bbd18739c';
Map<String, dynamic> scope({int epoch = 1, String request = 'read'}) => {
  'spaceId': 'core',
  'coreId': 'core',
  'authorizationRealm': 'core',
  'initiatorDeviceId': 'phone',
  'targetDeviceId': 'phone',
  'roleId': 'role',
  'roleRevision': 2,
  'roleOwnerId': 'phone',
  'resourceOwnerId': 'phone',
  'providerEpoch': epoch,
  'targetProviderEpoch': 1,
  'modeRevision': 1,
  'permissionRevision': 1,
  'targetPermissionRevision': 1,
  'coordinated': false,
  'requestId': request,
  'turnId': 'turn',
  'executionId': 'execution',
};
Map<String, dynamic> state([int epoch = 1]) => {
  'coreId': 'core',
  'coordinationAvailable': true,
  'policy': {
    'providerEpoch': epoch,
    'modeRevision': 1,
    'permissionRevision': 1,
    'coordinated': false,
  },
};
Map<String, dynamic> resource({
  String kind = 'memory',
  String owner = 'phone',
}) => {
  'id': 'item',
  'ownerId': owner,
  'roleId': 'role',
  'kind': kind,
  'revision': 1,
  'body': {
    'key': 'fact/key',
    'content': {'value': '内容'},
  },
};
Map<String, dynamic> page({
  List<Map<String, dynamic>> rows = const [],
  int epoch = 1,
}) => {
  'resources': rows,
  'executionScope': scope(epoch: epoch),
  'nextCursor': '',
};
Map<String, dynamic> saved({
  bool generated = false,
  bool candidates = false,
}) => {
  'resources': [],
  'executionScope': scope(request: operation),
  'saved': true,
  'acknowledgement': {
    'ownerId': 'phone',
    'requestId': generated ? '$operation|candidate-result' : operation,
    'versions': {
      'checkpoint/${candidates ? 'memory-candidate-operation' : 'memory-management'}/$operation':
          generated ? 2 : 1,
    },
  },
};
void main() {
  test('统筹开启由 Core 同一份数据保存并确认', () async {
    final coordinatedState = state();
    (coordinatedState['policy'] as Map)['coordinated'] = true;
    final coreScope = {
      ...scope(),
      'coordinated': true,
      'roleOwnerId': 'core',
      'resourceOwnerId': 'core',
    };
    final currentPage = {...page(), 'executionScope': coreScope};
    final result = saved();
    result['executionScope'] = {...coreScope, 'requestId': operation};
    (result['acknowledgement'] as Map)['ownerId'] = 'core';
    final transport = TaskTransport([
      coordinatedState,
      currentPage,
      coordinatedState,
      coordinatedState,
      result,
      coordinatedState,
    ]);
    final service = DeviceOwnedMemoryService(
      BackendServiceApi(transport, 1),
      isCurrent: () => true,
    );
    final original = await service.query('role');
    final confirmed = await service.manage(original, {
      'action': 'create',
    }, operationId: operation);
    expect(confirmed['acknowledgement']['ownerId'], 'core');
    expect(
      (transport.requests[4].body
          as Map)['expectedExecutionScope']['resourceOwnerId'],
      'core',
    );
  });
  test('冲突保存原 owner 和 CAS 版本，不重新取同 ID', () async {
    final options = RequestOptions(path: '/manage');
    final error = BackendTransportError(
      code: BackendTransportErrorCode.conflict,
      statusCode: 409,
      cause: DioException(
        requestOptions: options,
        response: Response(
          requestOptions: options,
          statusCode: 409,
          data: {
            'conflicts': [resource()],
          },
        ),
      ),
    );
    final transport = TaskTransport([
      state(),
      page(),
      state(),
      state(),
      error,
      state(),
    ]);
    final service = DeviceOwnedMemoryService(
      BackendServiceApi(transport, 1),
      isCurrent: () => true,
    );
    final original = await service.query('role');
    try {
      await service.manage(original, {
        'action': 'create',
      }, operationId: operation);
      fail('应返回冲突');
    } on OwnedMemoryConflict catch (conflict) {
      expect(conflict.resources.single['ownerId'], 'phone');
      expect(conflict.resources.single['revision'], 1);
      expect(
        conflict.resources.single['executionScope'],
        original['executionScope'],
      );
    }
    expect(transport.requests.length, 6);
  });
  test('候选修改必须使用原记录 ID 和版本', () async {
    final transport = TaskTransport([
      state(),
      page(rows: [resource(kind: 'checkpoint')]),
      state(),
    ]);
    final service = DeviceOwnedMemoryService(
      BackendServiceApi(transport, 1),
      isCurrent: () => true,
    );
    final original =
        (await service.query('role', candidates: true))['resources'][0]
            as Map<String, dynamic>;
    await expectLater(
      service.manage(original, {
        'action': 'update',
        'id': 'other',
        'expectedRevision': 1,
      }, candidates: true),
      throwsStateError,
    );
    await expectLater(
      service.manage(original, {
        'action': 'update',
        'id': 'item',
        'expectedRevision': 2,
      }, candidates: true),
      throwsStateError,
    );
    expect(transport.requests.length, 3);
  });
  test('网络重试保留原请求编号和所有者意图', () async {
    final error = BackendTransportError(
      code: BackendTransportErrorCode.requestTimeout,
    );
    final transport = TaskTransport([
      state(),
      page(),
      state(),
      state(),
      error,
      state(),
      state(),
      saved(),
      state(),
    ]);
    final service = DeviceOwnedMemoryService(
      BackendServiceApi(transport, 1),
      isCurrent: () => true,
    );
    final original = await service.query('role');
    await expectLater(
      service.manage(original, {'action': 'create'}, operationId: operation),
      throwsA(isA<BackendTransportError>()),
    );
    await service.manage(original, {
      'action': 'create',
    }, operationId: operation);
    expect(transport.requests[4].body, transport.requests[7].body);
  });
  test('查询过滤条件和分页保留原完整范围', () async {
    final transport = TaskTransport([
      state(),
      page(),
      state(),
      state(),
      page(),
      state(),
    ]);
    final service = DeviceOwnedMemoryService(
      BackendServiceApi(transport, 1),
      isCurrent: () => true,
    );
    final original = await service.query('role', query: '关键词', mode: 'hybrid');
    await service.query('role', cursor: 'cursor', original: original);
    expect(transport.requests[1].queryParameters?['query'], '关键词');
    expect(transport.requests[1].queryParameters?['mode'], 'hybrid');
    expect(
      () => (original['executionScope'] as Map)['providerEpoch'] = 2,
      throwsUnsupportedError,
    );
  });
  test('手动记忆提交冻结 scope 并精确验证 ACK', () async {
    final transport = TaskTransport([
      state(),
      page(),
      state(),
      state(),
      saved(),
      state(),
    ]);
    final service = DeviceOwnedMemoryService(
      BackendServiceApi(transport, 1),
      isCurrent: () => true,
    );
    final original = await service.query('role');
    final result = await service.manage(original, {
      'action': 'create',
      'key': 'key',
      'value': 'value',
      'importance': 5,
    }, operationId: operation);
    expect(result['saved'], true);
    expect(
      (transport.requests[4].body as Map)['expectedExecutionScope'],
      original['executionScope'],
    );
  });
  test('候选生成要求外部编号派生 ACK 与 proof2', () async {
    final transport = TaskTransport([
      state(),
      page(),
      state(),
      state(),
      saved(generated: true, candidates: true),
      state(),
    ]);
    final service = DeviceOwnedMemoryService(
      BackendServiceApi(transport, 1),
      isCurrent: () => true,
    );
    final original = await service.query('role', candidates: true);
    await service.manage(
      original,
      {
        'action': 'generate',
        'conversationId': 'conversation',
        'conversationOrigin': {'ownerId': 'phone', 'id': 'conversation'},
      },
      candidates: true,
      operationId: operation,
    );
    expect((transport.requests[4].body as Map)['conversationOrigin'], {
      'ownerId': 'phone',
      'id': 'conversation',
    });
  });
  test('错误候选 ACK 不能显示保存成功', () async {
    final result = saved(generated: true, candidates: true);
    (result['acknowledgement'] as Map)['requestId'] = operation;
    final transport = TaskTransport([
      state(),
      page(),
      state(),
      state(),
      result,
      state(),
    ]);
    final service = DeviceOwnedMemoryService(
      BackendServiceApi(transport, 1),
      isCurrent: () => true,
    );
    final original = await service.query('role', candidates: true);
    await expectLater(
      service.manage(
        original,
        {'action': 'generate', 'conversationId': 'conversation'},
        candidates: true,
        operationId: operation,
      ),
      throwsStateError,
    );
  });
  test('ACK 缺资源版本拒绝保存成功', () async {
    final result = saved();
    result['resources'] = [resource()];
    final transport = TaskTransport([
      state(),
      page(),
      state(),
      state(),
      result,
      state(),
    ]);
    final service = DeviceOwnedMemoryService(
      BackendServiceApi(transport, 1),
      isCurrent: () => true,
    );
    final original = await service.query('role');
    await expectLater(
      service.manage(original, {'action': 'create'}, operationId: operation),
      throwsStateError,
    );
  });
  test('同 Core 权限 ABA 旧表单不能写入', () async {
    final transport = TaskTransport([state(), page(), state(), state(3)]);
    final service = DeviceOwnedMemoryService(
      BackendServiceApi(transport, 1),
      isCurrent: () => true,
    );
    final original = await service.query('role');
    await expectLater(
      service.manage(original, {'action': 'create'}, operationId: operation),
      throwsStateError,
    );
    expect(transport.requests.length, 4);
  });
  test('混合 owner 查询结果不能展示', () async {
    final transport = TaskTransport([
      state(),
      page(rows: [resource(owner: 'other')]),
      state(),
    ]);
    await expectLater(
      DeviceOwnedMemoryService(
        BackendServiceApi(transport, 1),
        isCurrent: () => true,
      ).query('role'),
      throwsStateError,
    );
  });
  test('旧服务意图不能交给新服务同 ID', () async {
    final transport = TaskTransport([state(), page(), state()]);
    final original = await DeviceOwnedMemoryService(
      BackendServiceApi(transport, 1),
      isCurrent: () => true,
    ).query('role');
    final other = TaskTransport([]);
    await expectLater(
      DeviceOwnedMemoryService(
        BackendServiceApi(other, 2),
        isCurrent: () => true,
      ).manage(original, {'action': 'create'}),
      throwsStateError,
    );
    expect(other.requests, isEmpty);
  });
  test('操作迟到成功在 provider 更换后丢弃', () async {
    var current = true;
    final transport = TaskTransport([
      state(),
      page(),
      state(),
      state(),
      saved(),
    ]);
    transport.onSend = (request) {
      if (request.path.endsWith('/manage')) current = false;
    };
    final service = DeviceOwnedMemoryService(
      BackendServiceApi(transport, 1),
      isCurrent: () => current,
    );
    final original = await service.query('role');
    await expectLater(
      service.manage(original, {'action': 'create'}, operationId: operation),
      throwsStateError,
    );
  });
}
