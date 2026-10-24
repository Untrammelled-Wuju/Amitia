import 'package:flutter_test/flutter_test.dart';
import '../../../lib/features/workbench/presentation/pages/workbench_task_snapshot.dart';

Map<String, dynamic> sample() => {
  'protocolVersion': 1,
  'conversationId': 'conv',
  'source': 'durable-checkpoint',
  'truncated': false,
  'summary': {
    'total': 1, 'active': 1, 'paused': 0, 'succeeded': 0, 'failed': 0,
    'runtimeObserved': 0, 'checkpointOnly': 1, 'needsReconciliation': 0,
    'codingWorkersRequiringReview': 0,
  },
  'tasks': [{
    'coordinationId': 'coord', 'status': 'running', 'updatedAt': '2026-10-23T00:00:00Z',
    'recoveryState': 'checkpoint-only', 'assignmentCount': 2, 'completedCount': 1, 'failedCount': 0,
    'assignments': [
      <String, dynamic>{'id':'a','status':'succeeded','role':'worker','objective':'prepare','reviewRequired':false},
      <String, dynamic>{'id':'b','status':'pending','role':'worker','objective':'review','reviewRequired':false,'dependencies':['a']},
    ],
  }],
};

void main() {
  test('project durable dependencies and preserve legacy fields', () {
    final tasks = WorkbenchTaskSnapshot.fromResponse(sample(), 'conv').tasks;
    final children = tasks.single['assignments'] as List;
    expect(children.first['dependencies'], isEmpty);
    expect(children.last['dependencies'], ['a']);
  });
  for (final kind in ['self', 'cycle', 'unknown', 'duplicate', 'large', 'null']) {
    test('reject unsafe dependency $kind', () {
      final data = sample();
      final children = (data['tasks'] as List).single['assignments'] as List;
      if (kind == 'self') children[1]['dependencies'] = ['b'];
      if (kind == 'cycle') children[0]['dependencies'] = ['b'];
      if (kind == 'unknown') children[1]['dependencies'] = ['foreign'];
      if (kind == 'duplicate') children[1]['dependencies'] = ['a', 'a'];
      if (kind == 'large') children[1]['dependencies'] = List.filled(9, 'a');
      if (kind == 'null') children[1]['dependencies'] = null;
      expect(() => WorkbenchTaskSnapshot.fromResponse(data, 'conv'), throwsFormatException);
    });
  }
}
