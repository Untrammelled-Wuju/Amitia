import 'package:amitia_app/core/backend_transport/backend_service_api.dart';
import 'package:amitia_app/core/services/extension_task_service.dart';
import 'package:flutter_test/flutter_test.dart';
import 'owned_task_submission_test.dart' show TaskTransport;

Map<String, dynamic> policy([int epoch = 1]) => {
  'coreId': 'core',
  'coordinationAvailable': true,
  'policy': {
    'providerEpoch': epoch,
    'modeRevision': 1,
    'permissionRevision': 1,
    'coordinated': false,
  },
};
Map<String, dynamic> scope([int epoch = 1]) => {
  'coreId': 'core',
  'callerDeviceId': 'phone',
  'targetDeviceId': 'source',
  'resourceOwnerId': 'phone',
  'roleOwnerId': 'source',
  'roleId': 'role',
  'roleRevision': 1,
  'providerEpoch': epoch,
  'modeRevision': 1,
  'permissionRevision': 1,
  'coordinated': false,
  'realmRevision': 1,
};
Map<String, dynamic> run({bool readOnly = false, int epoch = 1}) => {
  'taskRunId': 'run',
  'ownerId': 'phone',
  'readOnly': readOnly,
  'executionScope': scope(epoch),
  'managementExecutionScope': scope(epoch),
};
ExtensionTaskService service(TaskTransport transport) => ExtensionTaskService(
  BackendServiceApi(transport, 1),
  isBound: () => true,
  taskIsCurrent: () => true,
);
void main() {
  test('本机 owned 任务继续使用原本机接口', () async {
    final transport = TaskTransport([run()]);
    await ExtensionTaskService(
      BackendServiceApi(transport, 1),
    ).cancel('run', expectedRun: run());
    expect(transport.requests.single.path, '/api/extensions/tasks/run/cancel');
  });
  test('控制携带原列表范围并验证响应', () async {
    final transport = TaskTransport([
      policy(),
      {
        'items': [run()],
      },
      policy(),
      policy(),
      run(),
      policy(),
    ]);
    final api = service(transport);
    final row = (await api.listRuns()).single;
    expect(
      () => (row['executionScope'] as Map)['providerEpoch'] = 2,
      throwsUnsupportedError,
    );
    await api.cancel('run', expectedRun: row);
    expect(
      (transport.requests[4].body as Map)['expectedExecutionScope'],
      scope(),
    );
  });
  test('同 Core 权限 ABA 不允许旧任务控制', () async {
    final transport = TaskTransport([
      policy(),
      {
        'items': [run()],
      },
      policy(),
      policy(3),
    ]);
    final api = service(transport);
    final row = (await api.listRuns()).single;
    await expectLater(api.cancel('run', expectedRun: row), throwsStateError);
    expect(transport.requests.length, 4);
  });
  test('列表加载中切换权限丢弃迟到列表', () async {
    final transport = TaskTransport([
      policy(),
      {
        'items': [run()],
      },
      policy(2),
    ]);
    await expectLater(service(transport).listRuns(), throwsStateError);
  });
  test('历史任务禁止写入', () async {
    final transport = TaskTransport([
      policy(),
      {
        'items': [run(readOnly: true)],
      },
      policy(),
      policy(),
    ]);
    final api = service(transport);
    final row = (await api.listRuns()).single;
    await expectLater(api.cancel('run', expectedRun: row), throwsStateError);
    expect(transport.requests.length, 4);
  });
  test('原任务范围不能用于另一 ID', () async {
    final transport = TaskTransport([
      policy(),
      {
        'items': [run()],
      },
      policy(),
    ]);
    final api = service(transport);
    final row = (await api.listRuns()).single;
    await expectLater(
      api.runtimeDetail('other', expectedRun: row),
      throwsStateError,
    );
    await expectLater(api.cancel('other', expectedRun: row), throwsStateError);
    expect(transport.requests.length, 3);
  });
  test('混合 owner 的详情结果不可展示', () async {
    final foreign = run();
    foreign['executionScope'] = {...scope(), 'resourceOwnerId': 'other'};
    final transport = TaskTransport([
      policy(),
      {
        'items': [run()],
      },
      policy(),
      policy(),
      run(),
      run(),
      foreign,
      run(),
      policy(),
    ]);
    final api = service(transport);
    final row = (await api.listRuns()).single;
    await expectLater(
      api.runtimeDetail('run', expectedRun: row),
      throwsStateError,
    );
  });
  test('操作完成后身份变化不得显示成功', () async {
    final transport = TaskTransport([
      policy(),
      {
        'items': [run()],
      },
      policy(),
      policy(),
      run(),
      policy(2),
    ]);
    final api = service(transport);
    final row = (await api.listRuns()).single;
    await expectLater(api.cancel('run', expectedRun: row), throwsStateError);
  });
  test('新服务不接受旧 Core 的同 ID 行', () async {
    final transport = TaskTransport([
      policy(),
      {
        'items': [run()],
      },
      policy(),
    ]);
    final row = (await service(transport).listRuns()).single;
    final other = TaskTransport([]);
    await expectLater(
      service(other).cancel('run', expectedRun: row),
      throwsStateError,
    );
    expect(other.requests, isEmpty);
  });
}
