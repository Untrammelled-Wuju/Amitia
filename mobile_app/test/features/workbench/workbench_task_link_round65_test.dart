import 'package:flutter_test/flutter_test.dart';
import 'package:amitia_app/features/workbench/presentation/pages/workbench_task_link.dart';

void main() {
  test('only a current authorized TaskRun entry can be opened', () {
    final rows = <Map<String, dynamic>>[
      {'taskRunId': 'run-001', 'executionScope': {'roleId': 'r1'}},
      {'taskRunId': 'run-002', 'readOnly': true},
    ];
    expect(resolveAuthorizedWorkbenchTaskRun('run-001', rows), same(rows.first));
    expect(resolveAuthorizedWorkbenchTaskRun('run-unknown', rows), isNull);
    expect(() => resolveAuthorizedWorkbenchTaskRun('../other-owner', rows), throwsFormatException);
    expect(() => resolveAuthorizedWorkbenchTaskRun('run-001', [rows.first, rows.first]), throwsFormatException);
  });

  test('stale, reconciling or inactive Workbench cannot open controls', () {
    expect(mayOpenWorkbenchTaskRun(live: true, stale: false, loading: false, recoveryState: 'runtime-observed'), isTrue);
    expect(mayOpenWorkbenchTaskRun(live: true, stale: true, loading: false, recoveryState: 'runtime-observed'), isFalse);
    expect(mayOpenWorkbenchTaskRun(live: false, stale: false, loading: false, recoveryState: 'checkpoint-only'), isFalse);
    expect(mayOpenWorkbenchTaskRun(live: true, stale: false, loading: true, recoveryState: 'checkpoint-only'), isFalse);
    expect(mayOpenWorkbenchTaskRun(live: true, stale: false, loading: false, recoveryState: 'needs-reconciliation'), isFalse);
  });
}
