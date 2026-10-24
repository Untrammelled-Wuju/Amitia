import 'package:flutter_test/flutter_test.dart';
import 'package:amitia_app/features/workbench/presentation/pages/workbench_task_dependency_view.dart';

Map<String, Object> item(String id, String status, [List<String> dependencies = const []]) => {'id': id, 'status': status, 'dependencies': dependencies};

void main() {
  test('prerequisites appear before dependents', () {
    final result = WorkbenchTaskDependencyView.project([item('third', 'pending', ['second']), item('second', 'pending', ['first']), item('first', 'succeeded')]);
    expect(result.map((row) => (row['assignment'] as Map)['id']).toList(), ['first', 'second', 'third']);
    expect(result.map((row) => row['depth']).toList(), [0, 1, 2]);
    expect(result.last['notice'], contains('等待'));
  });
  test('failed prerequisites and false success are visible', () {
    final result = WorkbenchTaskDependencyView.project([item('a', 'failed'), item('b', 'pending', ['a']), item('c', 'succeeded', ['a'])]);
    expect(result[1]['notice'], contains('失败'));
    expect(result[2]['inconsistent'], true);
  });
  test('broken DAG is rejected', () {
    expect(() => WorkbenchTaskDependencyView.project([item('a', 'pending', ['missing'])]), throwsFormatException);
    expect(() => WorkbenchTaskDependencyView.project([item('a', 'pending', ['b']), item('b', 'pending', ['a'])]), throwsFormatException);
  });
}
