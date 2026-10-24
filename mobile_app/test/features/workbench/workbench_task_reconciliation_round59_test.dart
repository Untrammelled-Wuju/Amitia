import 'package:flutter_test/flutter_test.dart';
import 'package:amitia_app/features/workbench/presentation/pages/workbench_task_snapshot.dart';

void main() {
  Map<String, dynamic> response({bool verified = false}) => {
    'protocolVersion': 1,
    'conversationId': 'conv',
    'source': 'durable-checkpoint',
    'truncated': false,
    'summary': {'total': 1, 'active': verified ? 0 : 1, 'paused': 0, 'succeeded': verified ? 1 : 0, 'failed': 0, 'runtimeObserved': 0, 'checkpointOnly': verified ? 1 : 0, 'needsReconciliation': verified ? 0 : 1, 'codingWorkersRequiringReview': 1},
    'tasks': [{
      'coordinationId': 'coord', 'status': 'succeeded', 'updatedAt': '2026-10-23T01:00:00Z', 'recoveryState': verified ? 'checkpoint-only' : 'needs-reconciliation',
      'assignmentCount': 1, 'completedCount': 1, 'failedCount': 0,
      'assignments': [{'id': 'child', 'status': 'succeeded', 'role': 'coding_worker', 'objective': 'verify', 'reviewRequired': true}],
    }],
  };
  test('unverified success is represented as unresolved', () {
    final parsed = WorkbenchTaskSnapshot.fromResponse(response(), 'conv');
    expect(parsed.summary.active, 1);
    expect(parsed.summary.succeeded, 0);
    expect(parsed.summary.needsReconciliation, 1);
  });
  test('forged confirmed terminal counts are refused', () {
    final forged = response();
    (forged['summary'] as Map)['succeeded'] = 1;
    expect(() => WorkbenchTaskSnapshot.fromResponse(forged, 'conv'), throwsFormatException);
  });
  test('confirmed terminal is accepted', () {
    expect(WorkbenchTaskSnapshot.fromResponse(response(verified: true), 'conv').summary.succeeded, 1);
  });
}
