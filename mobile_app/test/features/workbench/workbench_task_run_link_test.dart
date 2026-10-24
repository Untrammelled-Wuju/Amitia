import 'package:flutter_test/flutter_test.dart';
import 'package:amitia_app/features/workbench/presentation/pages/workbench_task_snapshot.dart';

void main() {
  test('M08 does not accept an untrusted TaskRun association', () {
    expect(() => WorkbenchTaskSnapshot.fromResponse({
      'protocolVersion': 1, 'conversationId': 'conv', 'source': 'durable-checkpoint',
      'truncated': false,
      'tasks': [{
        'coordinationId': 'coord', 'taskRunId': '../other-owner', 'status': 'running',
        'recoveryState': 'checkpoint-only', 'updatedAt': '2026-10-23T00:00:00Z',
        'assignmentCount': 1, 'completedCount': 0, 'failedCount': 0,
        'assignments': [{'id': 'a', 'status': 'pending', 'role': 'coding_worker', 'objective': 'test', 'reviewRequired': true}]
      }],
      'summary': {'total': 1, 'active': 1, 'paused': 0, 'succeeded': 0, 'failed': 0, 'runtimeObserved': 0, 'checkpointOnly': 1, 'needsReconciliation': 0, 'codingWorkersRequiringReview': 1}
    }, 'conv'), throwsFormatException);
  });
}
