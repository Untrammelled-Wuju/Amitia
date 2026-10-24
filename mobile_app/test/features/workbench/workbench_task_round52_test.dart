import 'package:flutter_test/flutter_test.dart';
import '../../../lib/features/workbench/presentation/pages/workbench_task_snapshot.dart';

void main() {
  final child = <String, dynamic>{'id':'worker','status':'running','role':'coding_worker','objective':'inspect','reviewRequired':true};
  final task = <String,dynamic>{'coordinationId':'coord','status':'running','updatedAt':'2026-10-22T12:00:00Z','recoveryState':'checkpoint-only','assignmentCount':1,'completedCount':0,'failedCount':0,'assignments':[child],'goalId':'goal','goalRevision':3,'executionCoreId':'core-1','deviceId':'device-2','deadlineAt':'2026-10-23T12:00:00Z','cancelRequested':true};
  final summary = <String,dynamic>{'total':1,'active':1,'paused':0,'succeeded':0,'failed':0,'runtimeObserved':0,'checkpointOnly':1,'needsReconciliation':0,'codingWorkersRequiringReview':1};
  Map<String,dynamic> response(Map<String,dynamic> item) => {'protocolVersion':1,'conversationId':'conv','source':'durable-checkpoint','truncated':false,'summary':summary,'tasks':[item]};
  test('M08 durable goal and location projection stays compatible', () {
    final parsed = WorkbenchTaskSnapshot.fromResponse(response(task), 'conv');
    expect(parsed.tasks.single['goalId'], 'goal');
    expect(parsed.tasks.single['deviceId'], 'device-2');
    expect(parsed.tasks.single['cancelRequested'], true);
  });
  test('rejects malformed cancel intention and deadline', () {
    expect(() => WorkbenchTaskSnapshot.fromResponse(response({...task, 'cancelRequested':'yes'}), 'conv'), throwsFormatException);
    expect(() => WorkbenchTaskSnapshot.fromResponse(response({...task, 'deadlineAt':'bad'}), 'conv'), throwsFormatException);
  });
  test('old projection without optional fields is valid', () {
    final legacy = Map<String,dynamic>.from(task)..remove('goalId')..remove('goalRevision')..remove('executionCoreId')..remove('deviceId')..remove('deadlineAt')..remove('cancelRequested');
    expect(WorkbenchTaskSnapshot.fromResponse(response(legacy),'conv').tasks.length,1);
  });
}
