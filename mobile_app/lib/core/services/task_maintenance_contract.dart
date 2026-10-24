Map<String, dynamic> requireAppliedTaskConfig(Map<String, dynamic>? value) {
  final timeout = value?['timeout'];
  if (value == null ||
      value['source'] != 'task-runtime' ||
      value['applied'] != true ||
      value['revision'] is! int ||
      value['revision'] < 1 ||
      value['timeoutRevision'] is! int ||
      value['timeoutRevision'] < 0 ||
      value['maxTasks'] is! int ||
      value['maxTasks'] < 1 ||
      value['maxTasks'] > 20 ||
      timeout is! Map ||
      timeout['disabled'] is! bool ||
      timeout['seconds'] is! int ||
      timeout['seconds'] < 30 ||
      timeout['seconds'] > 1800 ||
      timeout['seconds'] % 30 != 0) {
    throw StateError('任务执行器未确认配置生效');
  }
  return value;
}

void requireSavedTaskConfig(
  Map<String, dynamic> value,
  Map<String, dynamic> request,
) {
  requireAppliedTaskConfig(value);
  if (value['maxTasks'] != request['maxTasks'] ||
      value['timeout']['disabled'] != request['timeout']['disabled'] ||
      value['timeout']['seconds'] != request['timeout']['seconds'] ||
      value['revision'] != request['expectedRevision'] + 1 ||
      value['timeoutRevision'] != request['expectedTimeoutRevision'] + 1) {
    throw StateError('执行器返回的配置与本次保存不一致，请重新加载');
  }
}
