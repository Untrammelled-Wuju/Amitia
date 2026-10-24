import 'package:flutter_test/flutter_test.dart';
import 'package:amitia_app/features/workbench/presentation/pages/workbench_task_snapshot.dart';

void main() {
  test('parent terminal with unsettled child remains unverified on mobile', () {
    final input = <String, dynamic>{
      'protocolVersion': 1, 'conversationId': 'conv', 'source': 'durable-checkpoint', 'truncated': false,
      'summary': {'total': 1, 'active': 1, 'paused': 0, 'succeeded': 0, 'failed': 0, 'runtimeObserved': 0, 'checkpointOnly': 0, 'needsReconciliation': 1, 'codingWorkersRequiringReview': 0},
      'tasks': [
        {'coordinationId': 'coord', 'status': 'waiting', 'recoveryState': 'needs-reconciliation', 'recoveryReason': 'parent-terminal-child-pending', 'updatedAt': '2026-10-23T00:00:00Z', 'assignmentCount': 1, 'completedCount': 0, 'failedCount': 0,
          'assignments': [{'id': 'child', 'status': 'waiting', 'role': 'worker', 'dependencies': <String>[], 'objective': 'verify', 'reviewRequired': false}]}
      ],
    };
    final result = WorkbenchTaskSnapshot.fromResponse(input, 'conv');
    expect(result.summary.succeeded, 0);
    expect(result.summary.needsReconciliation, 1);
    expect(result.tasks.single['recoveryReason'], 'parent-terminal-child-pending');
  });
}
