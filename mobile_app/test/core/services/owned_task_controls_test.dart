import 'package:amitia_app/features/agent/presentation/providers/agent_tasks_provider.dart';
import 'package:flutter_test/flutter_test.dart';

AgentTaskItem task(String status, {bool readOnly = false}) =>
    AgentTaskItem.fromJson(
      {
        'taskRunId': 'run',
        'status': status,
        'generation': 2,
        'executionPlacement': 'device',
        'checkpointId': 'checkpoint',
        'executionScope': {'coreId': 'core'},
        'readOnly': readOnly,
      },
      definition: {'checkpoint': true},
    );
void main() {
  test('原设备任务支持真实暂停及检查点恢复', () {
    expect(task('running').canPause, true);
    expect(task('paused').canResume, true);
    expect(task('paused').canCancel, true);
  });
  test('历史原设备任务控制全部只读', () {
    expect(task('running', readOnly: true).canPause, false);
    expect(task('paused', readOnly: true).canResume, false);
    expect(task('paused', readOnly: true).canCancel, false);
    expect(task('recovery_required', readOnly: true).canRecover, false);
  });
}
