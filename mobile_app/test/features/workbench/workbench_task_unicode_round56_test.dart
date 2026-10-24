import 'package:flutter_test/flutter_test.dart';
import 'package:amitia_app/features/workbench/presentation/pages/workbench_task_snapshot.dart';

Map<String, dynamic> reply(String objective, {String role = 'coding_worker'}) => {
  'protocolVersion': 1,
  'conversationId': 'conv',
  'source': 'durable-checkpoint',
  'truncated': false,
  'summary': {'total': 1, 'active': 1, 'paused': 0, 'succeeded': 0, 'failed': 0, 'runtimeObserved': 0, 'checkpointOnly': 1, 'needsReconciliation': 0, 'codingWorkersRequiringReview': 0},
  'tasks': [{'coordinationId': 'coord', 'status': 'running', 'updatedAt': '2026-10-23T00:00:00Z', 'recoveryState': 'checkpoint-only', 'assignmentCount': 1, 'completedCount': 0, 'failedCount': 0, 'assignments': [{'id': 'child', 'status': 'running', 'role': role, 'objective': objective, 'reviewRequired': false}]}],
};

void main() {
  test('Go rune-length objectives and 100-codepoint roles are supported', () {
    final result = WorkbenchTaskSnapshot.fromResponse(reply('😀' * 220, role: 'r' * 100), 'conv');
    expect(result.tasks.single['assignments'][0]['objective'], '😀' * 220);
  });
  test('out-of-contract objectives and roles are rejected', () {
    expect(() => WorkbenchTaskSnapshot.fromResponse(reply('😀' * 221), 'conv'), throwsFormatException);
    expect(() => WorkbenchTaskSnapshot.fromResponse(reply('ok', role: 'r' * 101), 'conv'), throwsFormatException);
  });
}
