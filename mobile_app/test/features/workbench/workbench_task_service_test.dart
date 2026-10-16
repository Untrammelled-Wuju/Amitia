import 'dart:async';
import 'package:flutter_test/flutter_test.dart';
import 'package:amitia_app/core/backend_transport/backend_service_api.dart';
import 'package:amitia_app/features/workbench/data/workbench_task_service.dart';

class TaskApi extends Fake implements BackendServiceApi {
  Map<String, dynamic>? parameters;
  String? path;
  Completer<void>? gate;
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
  Future<T?> get<T>(
    String target, {
    Map<String, dynamic>? queryParameters,
    Map<String, String>? headers,
    T Function(dynamic)? fromJson,
  }) async {
    if (target.endsWith('/coordination/me')) {
      return {
            'coreId': 'core-a',
            'policy': {
              'providerEpoch': 1,
              'modeRevision': 1,
              'permissionRevision': 1,
              'coordinated': true,
            },
          }
          as T;
    }
    parameters = queryParameters;
    path = target;
    await gate?.future;
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
  test('rejects bare or malformed UI conversation identities', () async {
    final service = WorkbenchTaskService(TaskApi(), isCurrent: () => true);
    await expectLater(service.read('source', 'role'), throwsStateError);
    await expectLater(service.read('meshconv1:bad', 'role'), throwsStateError);
  });
}
