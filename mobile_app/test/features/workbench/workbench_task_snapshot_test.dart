import 'package:flutter_test/flutter_test.dart';
import '../../../lib/features/workbench/presentation/pages/workbench_task_snapshot.dart';

Map<String, dynamic> response() => {
  'protocolVersion': 1,
  'conversationId': 'conv-1',
  'source': 'durable-checkpoint',
  'truncated': false,
  'liveObservationTruncated': false,
  'summary': {'total': 1, 'active': 1, 'paused': 0, 'succeeded': 0, 'failed': 0, 'runtimeObserved': 1, 'checkpointOnly': 0, 'needsReconciliation': 0, 'codingWorkersRequiringReview': 1},
  'tasks': [{
    'coordinationId': 'coord-1', 'status': 'running', 'updatedAt': '2026-10-15T00:01:02Z', 'recoveryState': 'runtime-observed',
    'assignmentCount': 1, 'completedCount': 0, 'failedCount': 0,
    'assignments': [{'id': 'assignment-1', 'status': 'running', 'role': 'coding_worker', 'objective': 'Code', 'reviewRequired': true}],
  }],
};

void main() {
  test('valid response preserves task review requirements', () {
    final parsed = WorkbenchTaskSnapshot.fromResponse(response(), 'conv-1');
    expect(parsed.tasks.single['assignments'][0]['reviewRequired'], true);
    expect(parsed.summary.total, 1);
  });
  test('wrong conversation and protocol are rejected', () {
    expect(() => WorkbenchTaskSnapshot.fromResponse(response(), 'conv-2'), throwsFormatException);
    final data = response()..['protocolVersion'] = 0;
    expect(() => WorkbenchTaskSnapshot.fromResponse(data, 'conv-1'), throwsFormatException);
  });
  test('incomplete or forged task counts are rejected', () {
    final data = response();
    (data['tasks'] as List).first['assignmentCount'] = 2;
    expect(() => WorkbenchTaskSnapshot.fromResponse(data, 'conv-1'), throwsFormatException);
    final second = response();
    (second['summary'] as Map)['total'] = 2;
    expect(() => WorkbenchTaskSnapshot.fromResponse(second, 'conv-1'), throwsFormatException);
  });
  test('large and duplicate lists are rejected', () {
    final data = response();
    data['tasks'] = List.filled(31, (data['tasks'] as List).first);
    expect(() => WorkbenchTaskSnapshot.fromResponse(data, 'conv-1'), throwsFormatException);
    final second = response();
    (second['tasks'] as List).add((second['tasks'] as List).first);
    (second['summary'] as Map)['total'] = 2;
    expect(() => WorkbenchTaskSnapshot.fromResponse(second, 'conv-1'), throwsFormatException);
  });
  test('forged summary categories are rejected', () {
    for (final field in ['active', 'runtimeObserved', 'codingWorkersRequiringReview']) {
      final data = response();
      (data['summary'] as Map)[field] = 0;
      expect(() => WorkbenchTaskSnapshot.fromResponse(data, 'conv-1'), throwsFormatException);
    }
  });
  test('child terminal totals must match child states', () {
    final data = response();
    (data['tasks'] as List).first['completedCount'] = 1;
    expect(() => WorkbenchTaskSnapshot.fromResponse(data, 'conv-1'), throwsFormatException);
    final failed = response();
    (failed['tasks'] as List).first['completedCount'] = 1;
    (failed['tasks'] as List).first['failedCount'] = 1;
    expect(() => WorkbenchTaskSnapshot.fromResponse(failed, 'conv-1'), throwsFormatException);
  });
  test('consistent terminal state is accepted', () {
    final data = response();
    final task = (data['tasks'] as List).first;
    task['status'] = 'succeeded';
    task['completedCount'] = 1;
    (task['assignments'] as List).first['status'] = 'succeeded';
    (data['summary'] as Map)['active'] = 0;
    (data['summary'] as Map)['succeeded'] = 1;
    expect(WorkbenchTaskSnapshot.fromResponse(data, 'conv-1').summary.succeeded, 1);
  });
  test('invalid state and unbounded objective are rejected', () {
    final data = response();
    (data['tasks'] as List).first['status'] = 'unexpected';
    expect(() => WorkbenchTaskSnapshot.fromResponse(data, 'conv-1'), throwsFormatException);
    final second = response();
    ((second['tasks'] as List).first['assignments'] as List).first['objective'] = 'x' * 221;
    expect(() => WorkbenchTaskSnapshot.fromResponse(second, 'conv-1'), throwsFormatException);
  });
}
