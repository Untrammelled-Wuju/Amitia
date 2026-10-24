import 'package:flutter_test/flutter_test.dart';
import 'package:amitia_app/features/workbench/presentation/pages/workbench_task_snapshot.dart';

void main() {
  Map<String, dynamic> response(String reason) => {
    'protocolVersion': 1, 'conversationId': 'conv', 'source': 'durable-checkpoint', 'truncated': false,
    'summary': {'total': 1, 'active': 1, 'paused': 0, 'succeeded': 0, 'failed': 0, 'runtimeObserved': 0, 'checkpointOnly': 0, 'needsReconciliation': 1, 'codingWorkersRequiringReview': 0},
    'tasks': [
      {'coordinationId': 'coord', 'status': 'waiting', 'recoveryState': 'needs-reconciliation', 'recoveryReason': reason,
        'updatedAt': '2026-10-23T00:01:00Z', 'deadlineAt': '2026-10-23T00:00:00Z', 'assignmentCount': 1, 'completedCount': 0, 'failedCount': 0,
        'assignments': [{'id': 'child', 'status': 'waiting', 'role': 'review_worker', 'dependencies': <String>[], 'objective': 'verify', 'reviewRequired': false}]},
    ],
  };

  test('unproven child deadline success is shown as reconciliation', () {
    final result = WorkbenchTaskSnapshot.fromResponse(response('terminal-child-deadline-unverified'), 'conv');
    expect(result.tasks.single['recoveryReason'], 'terminal-child-deadline-unverified');
    expect(result.summary.succeeded, 0);
    expect(result.summary.needsReconciliation, 1);
  });

  test('unknown deadline reason is rejected', () {
    expect(() => WorkbenchTaskSnapshot.fromResponse(response('terminal-unknown-proof'), 'conv'), throwsFormatException);
  });
}
