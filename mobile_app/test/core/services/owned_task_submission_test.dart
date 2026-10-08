import 'package:amitia_app/core/backend_transport/backend_service_api.dart';
import 'package:amitia_app/core/backend_transport/http/backend_http_request.dart';
import 'package:amitia_app/core/backend_transport/http/backend_http_method.dart';
import 'package:amitia_app/core/backend_transport/http/backend_http_response.dart';
import 'package:amitia_app/core/backend_transport/http/backend_http_transport.dart';
import 'package:amitia_app/core/backend_transport/state/backend_http_state.dart';
import 'package:amitia_app/core/services/extension_task_service.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:amitia_app/core/services/providers.dart';
import 'package:amitia_app/core/backend_transport/providers/backend_transport_providers.dart';

class TaskTransport implements BackendHttpTransport {
  final List<Object> replies;
  final List<BackendHttpRequest> requests = [];
  void Function(BackendHttpRequest)? onSend;
  TaskTransport(this.replies);
  @override
  BackendHttpState get state => BackendHttpState.available;
  @override
  Future<void> close() async {}
  @override
  Future<BackendHttpResponse> send(BackendHttpRequest request) async {
    requests.add(request);
    final reply = replies.removeAt(0);
    onSend?.call(request);
    if (reply is Exception) throw reply;
    return BackendHttpResponse(
      statusCode: 200,
      headers: {},
      data: {'code': 200, 'data': reply},
    );
  }
}

Map<String, dynamic> scope() => {
  'coreId': 'core-b',
  'targetDeviceId': 'phone',
  'roleOwnerId': 'phone',
  'resourceOwnerId': 'phone',
  'modeRevision': 4,
  'roleId': 'role',
  'roleRevision': 2,
};
Map<String, dynamic> options() => {
  'roles': [
    {'id': 'role', 'name': '角色', 'revision': 2},
  ],
  'roleOwnerId': 'phone',
  'executionScope': scope(),
};
Map<String, dynamic> state([String core = 'core-b']) => {
  'coreId': core,
  'coordinationAvailable': true,
};
Map<String, dynamic> submission() => {
  'taskDefinitionId': 'task',
  'targetDeviceId': 'phone',
  'characterId': 'role',
  'input': {},
  'expectedCoreId': 'core-b',
  'expectedModeRevision': 4,
  'expectedRoleRevision': 2,
  'requestId': 'same-request',
  'expectedExecutionScope': scope(),
};

void main() {
  test('已执行的单次授权可以按当前版本在本机撤销', () async {
    final item = {
      'id': 'task-approval',
      'revision': 3,
      'status': 'claimed',
      'binding': {'inputHash': 'a' * 64},
    };
    final transport = TaskTransport([
      {...item, 'revision': 4, 'status': 'revoked'},
    ]);
    await ExtensionTaskService(
      BackendServiceApi(transport, 1),
    ).revokeSourceTaskApproval(item);
    expect(
      transport.requests.single.path,
      '/internal/device-mesh/task-approvals/task-approval/revoke',
    );
    expect(transport.requests.single.body, {'expectedRevision': 3});
  });
  Map<String, dynamic> approval() => {
    'id': 'task-approval-test',
    'revision': 1,
    'status': 'pending',
    'binding': {
      'executionScope': {'coreId': 'core', 'targetDeviceId': 'source'},
      'taskRunId': 'run',
      'taskGeneration': 1,
      'executionTarget': {
        'spaceId': 'core',
        'deviceId': 'source',
        'runtimeId': 'runtime',
        'runtimeSessionId': 'session',
        'connectionGeneration': 1,
      },
      'inputHash': 'a' * 64,
      'target': {'taskId': 'task', 'installedGeneration': 2},
    },
    'permissions': [
      {'permissionId': 'filesystem.read'},
    ],
  };
  test('绑定模式本机审批 provider 使用独立 local transport，不使用远端 Core transport', () async {
    final local = TaskTransport([
      [approval()],
    ]);
    final remote = TaskTransport([]);
    final container = ProviderContainer(
      overrides: [
        rawDeviceLocalBackendServiceApiProvider.overrideWithValue(
          BackendServiceApi(local, 7),
        ),
        rawBackendServiceApiProvider.overrideWithValue(
          BackendServiceApi(remote, 7),
        ),
      ],
    );
    final service = container.read(sourceTaskApprovalServiceProvider);
    final rows = await service.listSourceTaskApprovals();
    expect(rows.single['id'], 'task-approval-test');
    expect(local.requests.single.path, '/internal/device-mesh/task-approvals');
    expect(remote.requests, isEmpty);
    container.dispose();
  });
  test('本机 runtime 切换后旧审批对话框不能提交给新 generation', () async {
    var current = true;
    final transport = TaskTransport([
      [approval()],
    ]);
    final service = ExtensionTaskService(
      BackendServiceApi(transport, 1),
      sourceIsCurrent: () => current,
    );
    final selected = (await service.listSourceTaskApprovals()).single;
    current = false;
    await expectLater(
      service.decideSourceTaskApproval(selected, true),
      throwsStateError,
    );
    expect(transport.requests.length, 1);
  });
  test('本机配对失效后审批消失，不能发送决定', () async {
    final transport = TaskTransport([
      [approval()],
      <dynamic>[],
    ]);
    final service = ExtensionTaskService(
      BackendServiceApi(transport, 1),
      sourceIsCurrent: () => true,
    );
    final selected = (await service.listSourceTaskApprovals()).single;
    await expectLater(
      service.decideSourceTaskApproval(selected, true),
      throwsStateError,
    );
    expect(
      transport.requests.every(
        (request) => request.method == BackendHttpMethod.get,
      ),
      isTrue,
    );
  });
  test('本机审批写请求进行中 runtime 改变，拒绝迟到成功结果', () async {
    var current = true;
    final item = approval();
    final transport = TaskTransport([
      [item],
      [item],
      {...item, 'revision': 2, 'status': 'approved'},
    ]);
    transport.onSend = (_) {
      if (transport.requests.length == 3) current = false;
    };
    final service = ExtensionTaskService(
      BackendServiceApi(transport, 1),
      sourceIsCurrent: () => current,
    );
    final selected = (await service.listSourceTaskApprovals()).single;
    await expectLater(
      service.decideSourceTaskApproval(selected, true),
      throwsStateError,
    );
    expect(transport.requests.length, 3);
  });
  test('资源审批始终读取和决定本机数据', () async {
    final item = approval();
    final transport = TaskTransport([
      [item],
      {...item, 'revision': 2, 'status': 'approved'},
    ]);
    final service = ExtensionTaskService(BackendServiceApi(transport, 1));
    expect(await service.listSourceTaskApprovals(), [item]);
    await service.decideSourceTaskApproval(item, true);
    expect(transport.requests[0].path, '/internal/device-mesh/task-approvals');
    expect(
      transport.requests[1].path,
      '/internal/device-mesh/task-approvals/task-approval-test/decision',
    );
    expect(transport.requests[1].body, {
      'expectedRevision': 1,
      'approved': true,
    });
  });
  test('不接受其他请求的资源审批确认', () async {
    final item = approval();
    final transport = TaskTransport([
      {
        ...item,
        'revision': 2,
        'status': 'approved',
        'binding': {...item['binding'], 'inputHash': 'b' * 64},
      },
    ]);
    await expectLater(
      ExtensionTaskService(
        BackendServiceApi(transport, 1),
      ).decideSourceTaskApproval(item, true),
      throwsStateError,
    );
  });
  test('错误资源声明不能呈现为可批准任务', () async {
    final transport = TaskTransport([
      [
        {...approval(), 'permissions': null},
      ],
    ]);
    await expectLater(
      ExtensionTaskService(
        BackendServiceApi(transport, 1),
      ).listSourceTaskApprovals(),
      throwsStateError,
    );
  });
  test('已消费的审批不再发送决定', () async {
    final transport = TaskTransport([]);
    await expectLater(
      ExtensionTaskService(
        BackendServiceApi(transport, 1),
      ).decideSourceTaskApproval({...approval(), 'status': 'claimed'}, true),
      throwsStateError,
    );
    expect(transport.requests, isEmpty);
  });
  for (final field in [
    'generation',
    'session',
    'connection',
    'source',
    'core',
  ]) {
    test('拒绝不完整或被替换的审批执行身份：$field', () async {
      final item = approval();
      final binding = item['binding'] as Map<String, dynamic>;
      final target = binding['executionTarget'] as Map<String, dynamic>;
      if (field == 'generation') binding['taskGeneration'] = 0;
      if (field == 'session') target['runtimeSessionId'] = '';
      if (field == 'connection') target['connectionGeneration'] = 0;
      if (field == 'source') target['deviceId'] = 'other';
      if (field == 'core') target['spaceId'] = 'other';
      final transport = TaskTransport([
        [item],
      ]);
      await expectLater(
        ExtensionTaskService(
          BackendServiceApi(transport, 1),
        ).listSourceTaskApprovals(),
        throwsStateError,
      );
    });
  }
  Map<String, dynamic> catalog() => {
    'executionScope': scope(),
    'revision': 'a' * 64,
    'entries': [
      {
        'reference': {
          'catalogId': 'mesh-task-${'b' * 64}',
          'coreId': 'core-b',
          'deviceId': 'phone',
          'sourceTaskId': 'task',
          'portableFingerprint': 'c' * 64,
        },
        'definition': {'taskId': 'task'},
        'target': {
          'deviceId': 'phone',
          'taskId': 'task',
          'portableFingerprint': 'c' * 64,
          'definitionFingerprint': 'd' * 64,
          'installedGeneration': 2,
        },
      },
    ],
  };
  test('目录请求固定角色和设备授权且不导入定义', () async {
    final page = catalog();
    final transport = TaskTransport([page]);
    final service = ExtensionTaskService(BackendServiceApi(transport, 1));
    expect(await service.listOwnedDeviceTaskCatalog(options(), 'role'), page);
    expect(
      transport.requests.single.path,
      '/api/device-mesh/v1/business/tasks/catalog',
    );
  });
  for (final kind in [
    'core',
    'device',
    'task',
    'fingerprint',
    'installation',
    'duplicate',
    'scope',
  ]) {
    test('目录拒绝 $kind 混用', () async {
      final page = catalog();
      final entry = page['entries'][0] as Map;
      if (kind == 'core') entry['reference']['coreId'] = 'core-c';
      if (kind == 'device') entry['target']['deviceId'] = 'other';
      if (kind == 'task') entry['reference']['sourceTaskId'] = 'other';
      if (kind == 'fingerprint')
        entry['target']['portableFingerprint'] = 'e' * 64;
      if (kind == 'installation') entry['target']['installedGeneration'] = 0;
      if (kind == 'duplicate') page['entries'].add(entry);
      if (kind == 'scope') page['executionScope']['modeRevision'] = 5;
      final transport = TaskTransport([page]);
      await expectLater(
        ExtensionTaskService(
          BackendServiceApi(transport, 1),
        ).listOwnedDeviceTaskCatalog(options(), 'role'),
        throwsStateError,
      );
    });
  }
  test('只读取任务权限角色并检查同一 Core', () async {
    final transport = TaskTransport([state(), options(), state()]);
    final result = await ExtensionTaskService(
      BackendServiceApi(transport, 1),
    ).prepareOwnedTask('phone');
    expect(result, options());
    expect(
      transport.requests[1].path,
      '/api/device-mesh/v1/business/tasks/roles',
    );
    expect(transport.requests[1].queryParameters, {'targetDeviceId': 'phone'});
  });
  for (final kind in ['provider', 'owner', 'target', 'revision']) {
    test('拦截 $kind 变化', () async {
      final changed = options();
      final current = changed['executionScope'] as Map;
      if (kind == 'owner') current['resourceOwnerId'] = 'other';
      if (kind == 'target') current['targetDeviceId'] = 'other';
      if (kind == 'revision') changed['roles'][0]['revision'] = 0;
      final transport = TaskTransport([
        state(),
        changed,
        state(kind == 'provider' ? 'core-c' : 'core-b'),
      ]);
      await expectLater(
        ExtensionTaskService(
          BackendServiceApi(transport, 1),
        ).prepareOwnedTask('phone'),
        throwsStateError,
      );
    });
  }
  test('未就绪时不读取角色', () async {
    final transport = TaskTransport([
      {'coreId': 'core-b', 'coordinationAvailable': false},
    ]);
    await expectLater(
      ExtensionTaskService(
        BackendServiceApi(transport, 1),
      ).prepareOwnedTask('phone'),
      throwsStateError,
    );
    expect(transport.requests.length, 1);
  });
  test('网络重试保留请求编号并返回完成状态', () async {
    final task = {
      'taskRunId': 'owned-run',
      'status': 'succeeded',
      'queued': false,
    };
    final transport = TaskTransport([
      Exception('网络中断'),
      {'task': task, 'executionScope': scope()},
    ]);
    final service = ExtensionTaskService(BackendServiceApi(transport, 1));
    await expectLater(service.enqueueOwnedTask(submission()), throwsException);
    expect(await service.enqueueOwnedTask(submission()), task);
    expect(transport.requests[0].body, transport.requests[1].body);
    expect(transport.requests[0].path, '/api/device-mesh/v1/business/tasks');
  });
  for (final field in [
    'coreId',
    'targetDeviceId',
    'roleId',
    'roleRevision',
    'modeRevision',
  ]) {
    test('拒绝不匹配的 $field 确认', () async {
      final transport = TaskTransport([
        {
          'task': {'taskRunId': 'run'},
          'executionScope': {...scope(), field: 'changed'},
        },
      ]);
      await expectLater(
        ExtensionTaskService(
          BackendServiceApi(transport, 1),
        ).enqueueOwnedTask(submission()),
        throwsStateError,
      );
    });
  }
}
