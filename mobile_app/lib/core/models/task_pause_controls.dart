class TaskPauseControls {
  final bool pause;
  final bool resume;

  const TaskPauseControls({required this.pause, required this.resume});

  factory TaskPauseControls.fromRun(
    Map<String, dynamic> run,
    Map<String, dynamic>? definition,
  ) {
    final placement = (run['executionPlacement'] ?? 'local').toString();
    final status = (run['status'] ?? '').toString().toLowerCase();
    final generation = run['generation'];
    final supported =
        (placement == 'local' || placement == 'device') &&
        definition?['checkpoint'] == true &&
        generation is num &&
        generation.isFinite &&
        generation == generation.truncateToDouble() &&
        generation > 0 &&
        generation <= 9007199254740991;
    return TaskPauseControls(
      pause: supported && (status == 'running' || status == 'checkpointing'),
      resume:
          supported &&
          status == 'paused' &&
          (run['checkpointId'] ?? '').toString().isNotEmpty,
    );
  }
}
