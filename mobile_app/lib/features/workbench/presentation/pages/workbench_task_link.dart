Map<String, dynamic>? resolveAuthorizedWorkbenchTaskRun(
  String taskRunId,
  Iterable<Map<String, dynamic>> currentlyAuthorizedRuns,
) {
  if (!RegExp(r'^[A-Za-z0-9_.:-]{1,160}$').hasMatch(taskRunId)) {
    throw const FormatException('TaskRun 关联标识无效');
  }
  Map<String, dynamic>? match;
  for (final run in currentlyAuthorizedRuns) {
    if (run['taskRunId'] != taskRunId) continue;
    if (match != null) throw const FormatException('TaskRun 授权列表存在重复记录');
    match = run;
  }
  return match;
}

bool mayOpenWorkbenchTaskRun({
  required bool live,
  required bool stale,
  required bool loading,
  required String recoveryState,
}) => live && !stale && !loading && recoveryState != 'needs-reconciliation';
