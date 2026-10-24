import 'package:flutter_test/flutter_test.dart';
import 'package:amitia_app/features/workbench/presentation/pages/workbench_task_snapshot.dart';

Map<String, dynamic> taskEvidence(String reason) => <String, dynamic>{
 'protocolVersion': 1, 'conversationId': 'conv', 'source': 'durable-checkpoint', 'truncated': false,
 'tasks': [<String, dynamic>{'coordinationId':'a', 'status':'waiting', 'updatedAt':'2026-10-23T00:00:00Z', 'recoveryState':'needs-reconciliation', 'recoveryReason':reason, 'assignmentCount':1, 'completedCount':0, 'failedCount':0, 'assignments':[<String, dynamic>{'id':'child','status':'waiting','role':'coding_worker','objective':'inspect','reviewRequired':false}]}],
 'summary':<String, dynamic>{'total':1,'active':1,'paused':0,'succeeded':0,'failed':0,'runtimeObserved':0,'checkpointOnly':0,'needsReconciliation':1,'codingWorkersRequiringReview':0},
};

void main() {
 test('known terminal reconciliation reasons survive Flutter strict parsing', () {
  for (final reason in ['terminal-assignment-missing','terminal-child-evidence-missing','terminal-child-reader-unavailable','terminal-child-read-budget-exceeded','terminal-child-evidence-conflict']) {
   final snapshot = WorkbenchTaskSnapshot.fromResponse(taskEvidence(reason), 'conv');
   expect(snapshot.tasks.single['recoveryReason'], reason);
  }
 });
 test('unknown terminal reason is rejected', () {
  expect(() => WorkbenchTaskSnapshot.fromResponse(taskEvidence('terminal-unrecognized'), 'conv'), throwsFormatException);
 });
}
