import 'package:flutter_test/flutter_test.dart';
import 'package:amitia_app/core/models/task_pause_controls.dart';

void main() {
  final definition = <String, dynamic>{'checkpoint': true};
  final task = <String, dynamic>{
    'generation': 2,
    'checkpointId': 'owner-checkpoint',
  };

  for (final placement in ['local', 'device']) {
    test('$placement supports only confirmed pause and resume states', () {
      final running = TaskPauseControls.fromRun({
        ...task,
        'executionPlacement': placement,
        'status': 'running',
      }, definition);
      expect(running.pause, isTrue);
      expect(running.resume, isFalse);
      final paused = TaskPauseControls.fromRun({
        ...task,
        'executionPlacement': placement,
        'status': 'paused',
      }, definition);
      expect(paused.pause, isFalse);
      expect(paused.resume, isTrue);
    });
  }
  for (final status in [
    'pausing',
    'pause_requested',
    'resuming',
    'cancelling',
    'recovery_required',
    'succeeded',
  ]) {
    test('$status cannot resume merely because a checkpoint exists', () {
      final controls = TaskPauseControls.fromRun({
        ...task,
        'executionPlacement': 'device',
        'status': status,
      }, definition);
      expect(controls.pause, isFalse);
      expect(controls.resume, isFalse);
    });
  }
  test('missing definition, generation and checkpoint reject resume', () {
    final paused = {
      ...task,
      'executionPlacement': 'device',
      'status': 'paused',
    };
    expect(TaskPauseControls.fromRun(paused, null).resume, isFalse);
    expect(
      TaskPauseControls.fromRun(paused, {'checkpoint': false}).resume,
      isFalse,
    );
    for (final generation in [0, -1, 1.5, double.nan, double.infinity]) {
      expect(
        TaskPauseControls.fromRun({
          ...paused,
          'generation': generation,
        }, definition).resume,
        isFalse,
      );
    }
    expect(
      TaskPauseControls.fromRun({
        ...paused,
        'checkpointId': '',
      }, definition).resume,
      isFalse,
    );
    expect(
      TaskPauseControls.fromRun({
        ...paused,
        'executionPlacement': 'cloud',
      }, definition).resume,
      isFalse,
    );
  });
}
