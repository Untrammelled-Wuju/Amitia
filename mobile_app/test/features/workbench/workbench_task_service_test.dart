import 'dart:async';
import 'package:flutter_test/flutter_test.dart';
import 'package:amitia_app/core/backend_transport/backend_service_api.dart';
import 'package:amitia_app/features/workbench/data/workbench_task_service.dart';

class TaskApi extends Fake implements BackendServiceApi {
  Map<String, dynamic>? parameters;
  String? path;
  Object? posted;
  Completer<void>? gate;
  int? failureCode;
  int permissionRevision = 1;
  int coordinationReads = 0;
  String selectedRole = 'role';
  bool changeRoleAfterTask = false;
  bool changePolicyAfterTask = false;
  bool invalidInitialPolicy = false;
  final response = <String, dynamic>{
    'conversationId': 'continuation/hash',
    'tasks': [
      {'coordinationId': 'task-a'},
    ],
    'queryContext': {
      'executionScope': {
        'coreId': 'core-a',
        'spaceId': 'core-a',
        'roleId': 'role',
        'resourceOwnerId': 'core-a',
        'providerEpoch': 1,
        'modeRevision': 1,
        'permissionRevision': 1,
        'coordinated': true,
      },
      'conversationOrigin': {'ownerId': 'device-a', 'id': 'source'},
      'conversationId': 'continuation/hash',
    },
  };

  @override
  Future<T?> post<T>(String target, {Object? data, Map<String, dynamic>? queryParameters, Map<String, String>? headers, T Function(dynamic)? fromJson}) async {
    posted = data;
    return get<T>(target, queryParameters: queryParameters, headers: headers, fromJson: fromJson);
  }

  @override
  Future<T?> get<T>(
    String target, {
    Map<String, dynamic>? queryParameters,
    Map<String, String>? headers,
    T Function(dynamic)? fromJson,
  }) async {
    if (target.endsWith('/coordination/me')) {
      coordinationReads++;
      return {
            'coreId': 'core-a',
            'policy': {
              'providerEpoch': invalidInitialPolicy && coordinationReads == 1 ? 'bad' : 1,
              'modeRevision': 1,
              'permissionRevision': permissionRevision,
              'coordinated': true,
              'selectedRole': selectedRole,
            },
          }
          as T;
    }
    parameters = queryParameters;
    if (changePolicyAfterTask) permissionRevision++;
    if (changeRoleAfterTask) selectedRole = 'other';
    path = target;
    await gate?.future;
    if (failureCode != null) throw ServiceApiException(code: failureCode!, message: 'simulated failure');
    return response as T;
  }
}

void main() {
  const reference = 'meshconv1:device-a:source';
  test(
    'task and checkpoint requests use Owned resource identity and preserve the UI reference',
    () async {
      final api = TaskApi();
      final service = WorkbenchTaskService(api, isCurrent: () => true);
      final result = await service.read(reference, 'role');
      expect(api.parameters, {
        'conversationId': 'source',
        'conversationOwnerId': 'device-a',
        'characterId': 'role',
        'expectedCoreId': 'core-a',
      });
      expect(result['conversationId'], reference);
      expect(result['tasks'], hasLength(1));
      await service.read(reference, 'role', coordinationId: 'task-a');
      expect(api.path, '/api/workbench/tasks/checkpoints');
      expect(api.parameters?['coordinationId'], 'task-a');
    },
  );
  test('rejects another owner, role and Core response', () async {
    for (final key in ['ownerId', 'roleId', 'coreId']) {
      final api = TaskApi();
      final context = api.response['queryContext'] as Map;
      (context[key == 'ownerId' ? 'conversationOrigin' : 'executionScope']
              as Map)[key] =
          'other';
      await expectLater(
        WorkbenchTaskService(
          api,
          isCurrent: () => true,
        ).read(reference, 'role'),
        throwsStateError,
      );
    }
  });
  test('discards an old Core response after provider switch', () async {
    final api = TaskApi()..gate = Completer<void>();
    var current = true;
    final request = WorkbenchTaskService(
      api,
      isCurrent: () => current,
    ).read(reference, 'role');
    await Future<void>.delayed(Duration.zero);
    current = false;
    api.gate!.complete();
    await expectLater(request, throwsStateError);
  });
  test('authorization failures invalidate the last trusted snapshot', () async {
    for (final code in [401, 403, 404, 409]) {
      final api = TaskApi()..failureCode = code;
      final error = await WorkbenchTaskService(api, isCurrent: () => true).read(reference, 'role').then<Object?>((_) => null, onError: (Object error) => error);
      expect(error, isA<WorkbenchTaskReadFailure>());
      expect((error as WorkbenchTaskReadFailure).invalidatesSnapshot, isTrue);
    }
  });
  test('transport failures preserve only the last confirmed read-only snapshot', () async {
    for (final code in [500, 503, 10001, 10002]) {
      final api = TaskApi()..failureCode = code;
      final error = await WorkbenchTaskService(api, isCurrent: () => true).read(reference, 'role').then<Object?>((_) => null, onError: (Object error) => error);
      expect(error, isA<WorkbenchTaskReadFailure>());
      expect((error as WorkbenchTaskReadFailure).invalidatesSnapshot, isFalse);
    }
  });
  test('rejects policy revision drift across the task response even when Core is unchanged', () async {
    final api = TaskApi()..changePolicyAfterTask = true;
    final error = await WorkbenchTaskService(api, isCurrent: () => true)
        .read(reference, 'role').then<Object?>((_) => null, onError: (Object e) => e);
    expect(error, isA<WorkbenchTaskReadFailure>());
    expect((error as WorkbenchTaskReadFailure).invalidatesSnapshot, isTrue);
    expect(api.coordinationReads, 2);
  });
  test('rejects role selection drift even when policy revisions are unchanged', () async {
    final api = TaskApi()..changeRoleAfterTask = true;
    final error = await WorkbenchTaskService(api, isCurrent: () => true)
        .read(reference, 'role').then<Object?>((_) => null, onError: (Object e) => e);
    expect(error, isA<WorkbenchTaskReadFailure>());
    expect((error as WorkbenchTaskReadFailure).invalidatesSnapshot, isTrue);
  });
  test('rejects malformed original policy before accessing private tasks', () async {
    final api = TaskApi()..invalidInitialPolicy = true;
    await expectLater(WorkbenchTaskService(api, isCurrent: () => true).read(reference, 'role'), throwsStateError);
    expect(api.path, isNull);
  });
  test('policy authority comparison is strict over all revisions', () {
    final initial = {'providerEpoch':1, 'modeRevision': 2, 'permissionRevision':3, 'coordinated':true};
    expect(workbenchPolicyAuthority(initial), workbenchPolicyAuthority({...initial}));
    for (final key in initial.keys) {
      final modified = {...initial, key: key == 'coordinated' ? false : 100};
      expect(workbenchPolicyAuthority(initial), isNot(workbenchPolicyAuthority(modified)));
    }
    expect(workbenchPolicyIsValid({...initial, 'permissionRevision': '3'}), isFalse);
    expect(workbenchPolicyIsValid({...initial, 'providerEpoch': -1}), isFalse);
  });
  test('budget reads and registration reuse Owned transport and authority fencing', () async {
    final api = TaskApi();
    final service = WorkbenchTaskService(api, isCurrent: () => true);
    await service.readBudget(reference, 'role', coordinationId: 'coord', parentInteractionId: 'parent', kind: 'tool', cursor: {'afterCreatedAt': '2026-10-24T00:00:00Z', 'afterEffectId': 'effect'});
    expect(api.path, '/api/workbench/tasks/budget-reconciliation');
    expect(api.parameters?['parentInteractionId'], 'parent');
    expect(api.parameters?['afterEffectId'], 'effect');
    await service.registerBudgetEvidence(reference, 'role', coordinationId: 'coord', parentInteractionId: 'parent', kind: 'model', recordId: 'record', evidenceRef: 'case:12345678');
    expect(api.path, '/api/workbench/tasks/budget-reconciliation/review');
    expect(api.posted, {'kind': 'model', 'recordId': 'record', 'evidenceRef': 'case:12345678'});
    expect(api.parameters?['conversationOwnerId'], 'device-a');
    api.changePolicyAfterTask = true;
    await expectLater(service.registerBudgetEvidence(reference, 'role', coordinationId: 'coord', parentInteractionId: 'parent', kind: 'model', recordId: 'record', evidenceRef: 'case:12345678'), throwsA(isA<WorkbenchTaskReadFailure>().having((e) => e.invalidatesSnapshot, 'invalidatesSnapshot', true)));
  });
  test('rejects bare or malformed UI conversation identities', () async {
    final service = WorkbenchTaskService(TaskApi(), isCurrent: () => true);
    await expectLater(service.read('source', 'role'), throwsStateError);
    await expectLater(service.read('meshconv1:bad', 'role'), throwsStateError);
    final malformed = await service.read('meshconv1:%ZZ:source', 'role').then<Object?>((_) => null, onError: (Object e) => e);
    expect(malformed, isA<WorkbenchTaskReadFailure>());
    expect((malformed as WorkbenchTaskReadFailure).invalidatesSnapshot, isTrue);
  });
}
