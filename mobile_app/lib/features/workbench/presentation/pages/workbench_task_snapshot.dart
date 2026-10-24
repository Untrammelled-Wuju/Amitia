class WorkbenchTaskSummary {
  const WorkbenchTaskSummary({required this.total, required this.active, required this.succeeded, required this.failed, required this.needsReconciliation});
  final int total;
  final int active;
  final int succeeded;
  final int failed;
  final int needsReconciliation;

  factory WorkbenchTaskSummary.fromJson(dynamic value) {
    final source = value is Map ? value : const {};
    int count(String key) {
      final number = source[key];
      return number is int && number >= 0 ? number : 0;
    }
    return WorkbenchTaskSummary(total: count('total'), active: count('active'), succeeded: count('succeeded'), failed: count('failed'), needsReconciliation: count('needsReconciliation'));
  }
}

class WorkbenchTaskSnapshot {
  const WorkbenchTaskSnapshot({required this.tasks, required this.summary, required this.truncated, required this.liveObservationTruncated});
  final List<Map<String,dynamic>> tasks;
  final WorkbenchTaskSummary summary;
  final bool truncated;
  final bool liveObservationTruncated;

  factory WorkbenchTaskSnapshot.fromJson(Map<String,dynamic>? value) {
    final data = value ?? const {};
    final raw = data['tasks'];
    final tasks = raw is List ? raw.whereType<Map>().take(30).map((item) => Map<String,dynamic>.from(item)).toList(growable: false) : <Map<String,dynamic>>[];
    return WorkbenchTaskSnapshot(tasks: tasks, summary: WorkbenchTaskSummary.fromJson(data['summary']), truncated: data['truncated'] == true, liveObservationTruncated: data['liveObservationTruncated'] == true);
  }

  static String _text(Object? value, int max, {bool nonEmpty = true}) {
    if (value is! String || value.runes.length > max || (nonEmpty && value.isEmpty)) throw const FormatException('任务响应文本无效');
    return value;
  }

  static int _count(Object? value, int min, int max) {
    if (value is! int || value < min || value > max) throw const FormatException('任务响应统计无效');
    return value;
  }

  static Map<String, dynamic> _assignment(Object? value) {
    if (value is! Map) throw const FormatException('子任务格式无效');
    final state = _text(value['status'], 24);
    if (!const {'pending', 'running', 'waiting', 'succeeded', 'failed', 'cancelled', 'paused'}.contains(state) || value['reviewRequired'] is! bool) throw const FormatException('子任务状态无效');
    final rawDependencies = value['dependencies'];
    if (value.containsKey('dependencies') && (rawDependencies is! List || rawDependencies.length > 8)) throw const FormatException('子任务依赖无效');
    final dependencies = rawDependencies == null ? <String>[] : (rawDependencies as List).map((item) => _text(item, 160)).toList(growable: false);
    return <String, dynamic>{
      'id': _text(value['id'], 160),
      'dependencies': dependencies,
      'status': state,
      'role': _text(value['role'], 100),
      'objective': _text(value['objective'], 220, nonEmpty: false),
      'reviewRequired': value['reviewRequired'],
    };
  }

  static String _linkedRunId(Object? value) {
    final id = _text(value, 160);
    if (!RegExp(r'^[A-Za-z0-9_.:-]{1,160}$').hasMatch(id)) throw const FormatException('TaskRun 关联标识无效');
    return id;
  }

  static Map<String, dynamic> _task(Object? value) {
    if (value is! Map || value['assignments'] is! List) throw const FormatException('任务格式无效');
    final state = _text(value['status'], 24);
    final recovery = _text(value['recoveryState'], 48);
    if (!const {'planning', 'running', 'waiting', 'aggregating', 'succeeded', 'failed', 'cancelled', 'paused'}.contains(state) ||
        !const {'runtime-observed', 'checkpoint-only', 'needs-reconciliation'}.contains(recovery)) throw const FormatException('任务状态无效');
    final rawAssignments = value['assignments'] as List;
    if (rawAssignments.length > 8) throw const FormatException('子任务数量过大');
    final assignments = rawAssignments.map(_assignment).toList(growable: false);
    final assignmentCount = _count(value['assignmentCount'], 1, 8);
    final completedCount = _count(value['completedCount'], 0, assignmentCount);
    final failedCount = _count(value['failedCount'], 0, completedCount);
    if (assignments.length != assignmentCount || assignments.map((row) => row['id']).toSet().length != assignments.length) throw const FormatException('子任务身份不一致');
    final byId = <String, Map<String, dynamic>>{for (final row in assignments) row['id'] as String: row};
    final visited = <String, int>{};
    void visit(String id) {
      if (visited[id] == 1) throw const FormatException('子任务依赖存在循环');
      if (visited[id] == 2) return;
      visited[id] = 1;
      final seen = <String>{};
      for (final dependency in (byId[id]!['dependencies'] as List<String>)) {
        if (!byId.containsKey(dependency) || !seen.add(dependency)) throw const FormatException('子任务依赖引用无效');
        visit(dependency);
      }
      visited[id] = 2;
    }
    for (final id in byId.keys) { visit(id); }
    final actualCompleted = assignments.where((row) => const {'succeeded', 'failed', 'cancelled'}.contains(row['status'])).length;
    final actualFailed = assignments.where((row) => const {'failed', 'cancelled'}.contains(row['status'])).length;
    if (completedCount != actualCompleted || failedCount != actualFailed) throw const FormatException('子任务终态统计不一致');
    final updatedAt = _text(value['updatedAt'], 80);
    if (DateTime.tryParse(updatedAt) == null) throw const FormatException('任务更新时间无效');
    if (value['cancelRequested'] != null && value['cancelRequested'] is! bool) throw const FormatException('取消状态无效');
    if (value['deadlineAt'] != null && DateTime.tryParse(_text(value['deadlineAt'], 80)) == null) throw const FormatException('任务截止时间无效');
    final reason = value['recoveryReason'];
    if (reason != null && !const {'parent-state-conflict', 'parent-terminal-child-pending', 'runtime-checkpoint-conflict', 'terminal-assignment-missing', 'terminal-child-evidence-missing', 'terminal-child-reader-unavailable', 'terminal-child-read-budget-exceeded', 'terminal-child-evidence-conflict', 'terminal-child-deadline-unverified'}.contains(reason)) throw const FormatException('任务对账原因无效');
    return <String, dynamic>{
      'coordinationId': _text(value['coordinationId'], 160),
      if (value['parentInteractionId'] != null) 'parentInteractionId': _text(value['parentInteractionId'], 512),
      if (value['taskRunId'] != null) 'taskRunId': _linkedRunId(value['taskRunId']),
      if (value['goalId'] != null) 'goalId': _text(value['goalId'], 160),
      if (value['goalRevision'] != null) 'goalRevision': _count(value['goalRevision'], 0, 0x1fffffffffffff),
      if (value['executionCoreId'] != null) 'executionCoreId': _text(value['executionCoreId'], 160),
      if (value['deviceId'] != null) 'deviceId': _text(value['deviceId'], 160),
      if (value['deadlineAt'] != null) 'deadlineAt': _text(value['deadlineAt'], 80),
      if (value['cancelRequested'] != null) 'cancelRequested': value['cancelRequested'],
      'status': state,
      'updatedAt': updatedAt,
      'recoveryState': recovery,
      if (reason != null) 'recoveryReason': reason,
      'assignmentCount': assignmentCount,
      'completedCount': completedCount,
      'failedCount': failedCount,
      'assignments': assignments,
    };
  }

  factory WorkbenchTaskSnapshot.fromResponse(Map<String,dynamic>? value, String conversationId) {
    if (value == null || value['protocolVersion'] != 1 || value['conversationId'] != conversationId || value['source'] != 'durable-checkpoint' ||
        value['truncated'] is! bool || (value['liveObservationTruncated'] != null && value['liveObservationTruncated'] is! bool) || value['tasks'] is! List || value['summary'] is! Map) {
      throw const FormatException('任务响应与当前对话或协议不匹配');
    }
    final raw = value['tasks'] as List;
    if (raw.length > 30) throw const FormatException('任务列表超出限制');
    final summary = value['summary'] as Map;
    for (final field in const ['total', 'active', 'paused', 'succeeded', 'failed', 'runtimeObserved', 'checkpointOnly', 'needsReconciliation', 'codingWorkersRequiringReview']) {
      _count(summary[field], 0, 0x1fffffffffffff);
    }
    final tasks = raw.map(_task).toList(growable: false);
    if (tasks.map((task) => task['coordinationId']).toSet().length != tasks.length) throw const FormatException('任务列表身份不一致');
    final derived = <String, int>{
      'total': tasks.length, 'active': 0, 'paused': 0, 'succeeded': 0, 'failed': 0,
      'runtimeObserved': 0, 'checkpointOnly': 0, 'needsReconciliation': 0, 'codingWorkersRequiringReview': 0,
    };
    for (final task in tasks) {
      final status = task['status'];
      if (task['recoveryState'] == 'needs-reconciliation') { derived['active'] = derived['active']! + 1; }
      else if (status == 'succeeded') { derived['succeeded'] = derived['succeeded']! + 1; }
      else if (status == 'failed' || status == 'cancelled') { derived['failed'] = derived['failed']! + 1; }
      else if (status == 'paused') { derived['paused'] = derived['paused']! + 1; }
      else { derived['active'] = derived['active']! + 1; }
      final recovery = task['recoveryState'];
      final recoveryKey = recovery == 'runtime-observed' ? 'runtimeObserved' : recovery == 'needs-reconciliation' ? 'needsReconciliation' : 'checkpointOnly';
      derived[recoveryKey] = derived[recoveryKey]! + 1;
      final assignments = task['assignments'] as List;
      derived['codingWorkersRequiringReview'] = derived['codingWorkersRequiringReview']! + assignments.where((child) => child['reviewRequired'] == true).length;
    }
    for (final entry in derived.entries) {
      if (summary[entry.key] != entry.value) throw const FormatException('任务分类统计与明细不一致');
    }
    return WorkbenchTaskSnapshot(
      tasks: tasks,
      summary: WorkbenchTaskSummary.fromJson(summary),
      truncated: value['truncated'] == true,
      liveObservationTruncated: value['liveObservationTruncated'] == true,
    );
  }
}
