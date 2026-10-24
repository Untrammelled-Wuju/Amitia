import 'package:flutter_test/flutter_test.dart';
import '../../../lib/features/workbench/presentation/pages/workbench_task_budget.dart';

void main() {
  final now = DateTime.utc(2026, 10, 23, 12);
  test('deadline is not proof that a running worker stopped', () {
    expect(WorkbenchTaskBudget.notice({'deadlineAt': '2026-10-23T12:00:00Z', 'status': 'running'}, now: now, stale: false), contains('正式回执'));
  });
  test('a terminal result is not proof of on-time completion', () {
    expect(WorkbenchTaskBudget.notice({'deadlineAt': '2026-10-23T11:00:00Z', 'status': 'succeeded'}, now: now, stale: false), contains('不能证明'));
  });
  test('missing hard budget and invalid evidence are described without inventing enforcement', () {
    expect(WorkbenchTaskBudget.notice({'status': 'running'}, now: now, stale: false), contains('预算账本'));
    expect(WorkbenchTaskBudget.notice({'recoveryState': 'needs-reconciliation', 'status': 'running'}, now: now, stale: false), contains('不可确认'));
    expect(WorkbenchTaskBudget.notice({'status': 'running'}, now: now, stale: true), contains('不可确认'));
  });
}
