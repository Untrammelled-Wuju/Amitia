import 'dart:convert';

import 'package:flutter_riverpod/flutter_riverpod.dart';

import '../../../../core/backend_transport/providers/backend_transport_providers.dart';

enum AgentTaskStatus { pending, waitingApproval, running, paused, completed, failed, cancelled }

class AgentTaskDefinition {
  final String taskId;
  final String extensionId;
  final String moduleId;
  final String name;
  final String description;

  const AgentTaskDefinition({
    required this.taskId,
    required this.extensionId,
    required this.moduleId,
    required this.name,
    required this.description,
  });

  factory AgentTaskDefinition.fromJson(Map<String, dynamic> json) {
    final id = (json['taskId'] ?? json['id'] ?? '').toString();
    final extensionId = (json['extensionId'] ?? '').toString();
    final moduleId = (json['moduleId'] ?? '').toString();
    final name = (json['name'] ?? json['title'] ?? id).toString();
    return AgentTaskDefinition(
      taskId: id,
      extensionId: extensionId,
      moduleId: moduleId,
      name: name.isEmpty ? id : name,
      description: (json['description'] ?? '').toString(),
    );
  }

  String get label {
    final scope = [extensionId, moduleId].where((item) => item.isNotEmpty).join(' · ');
    return scope.isEmpty ? name : '$name · $scope';
  }
}

class AgentTaskItem {
  final String id;
  final String title;
  final String description;
  final List<String> requiredAbilities;
  final List<String> steps;
  final AgentTaskStatus status;
  final int progress;
  final int currentStepIndex;
  final String elapsed;
  final String? result;
  final String? error;
  final DateTime createdAt;
  final int generation;

  AgentTaskItem({
    required this.id,
    required this.title,
    required this.description,
    required this.requiredAbilities,
    required this.steps,
    required this.status,
    required this.progress,
    required this.currentStepIndex,
    required this.elapsed,
    this.result,
    this.error,
    required this.createdAt,
    this.generation = 0,
  });

  factory AgentTaskItem.fromJson(Map<String, dynamic> json) {
    final status = _parseStatus(json['status']?.toString());
    final createdAt = DateTime.tryParse((json['createdAt'] ?? '').toString()) ?? DateTime.now();
    final finishedAt = DateTime.tryParse((json['finishedAt'] ?? '').toString());
    final startedAt = DateTime.tryParse((json['startedAt'] ?? '').toString());
    final end = finishedAt ?? DateTime.now();
    final elapsedDuration = end.difference(startedAt ?? createdAt);
    final elapsed = _formatDuration(elapsedDuration);
    final input = json['input'] is Map ? Map<String, dynamic>.from(json['input'] as Map) : const <String, dynamic>{};
    final taskDefinitionId = (json['taskDefinitionId'] ?? json['taskRunId'] ?? '').toString();
    final extensionId = (json['extensionId'] ?? '').toString();
    final moduleId = (json['moduleId'] ?? '').toString();
    final abilities = input['requiredAbilities'] is List
        ? (input['requiredAbilities'] as List).map((e) => e.toString()).where((e) => e.isNotEmpty).toList(growable: false)
        : [if (extensionId.isNotEmpty) extensionId, if (moduleId.isNotEmpty) moduleId];
    final rawSteps = input['steps'];
    final steps = rawSteps is List
        ? rawSteps.map((e) => e.toString()).where((e) => e.isNotEmpty).toList(growable: false)
        : const <String>[];
    final rawProgress = json['progress'];
    final progress = rawProgress is num ? rawProgress.round().clamp(0, 100).toInt() : _progressFor(status);
    return AgentTaskItem(
      id: (json['taskRunId'] ?? json['id'] ?? '').toString(),
      title: (input['title'] ?? taskDefinitionId).toString().trim().isEmpty ? 'Kernel Task' : (input['title'] ?? taskDefinitionId).toString(),
      description: (input['description'] ?? [extensionId, moduleId].where((e) => e.isNotEmpty).join(' · ')).toString(),
      requiredAbilities: abilities,
      steps: steps,
      status: status,
      progress: progress,
      currentStepIndex: 0,
      elapsed: elapsed,
      result: json['result']?.toString(),
      error: json['errorMessage']?.toString() ?? json['error']?.toString(),
      createdAt: createdAt,
      generation: (json['generation'] as num?)?.toInt() ?? 0,
    );
  }

  static AgentTaskStatus _parseStatus(String? value) {
    switch (value) {
      case 'running':
      case 'checkpointing':
      case 'resuming':
        return AgentTaskStatus.running;
      case 'paused':
      case 'pausing':
        return AgentTaskStatus.paused;
      case 'succeeded':
        return AgentTaskStatus.completed;
      case 'failed':
      case 'timed_out':
        return AgentTaskStatus.failed;
      case 'cancelled':
      case 'cancelling':
        return AgentTaskStatus.cancelled;
      case 'recovery_required':
      case 'manual_intervention':
        return AgentTaskStatus.waitingApproval;
      default:
        return AgentTaskStatus.pending;
    }
  }

  static int _progressFor(AgentTaskStatus status) {
    switch (status) {
      case AgentTaskStatus.completed:
      case AgentTaskStatus.failed:
      case AgentTaskStatus.cancelled:
        return 100;
      default:
        return 0;
    }
  }

  static String _formatDuration(Duration duration) {
    final safe = duration.isNegative ? Duration.zero : duration;
    final hours = safe.inHours.toString().padLeft(2, '0');
    final minutes = (safe.inMinutes % 60).toString().padLeft(2, '0');
    final seconds = (safe.inSeconds % 60).toString().padLeft(2, '0');
    return '$hours:$minutes:$seconds';
  }
}

class AgentTaskNotifier extends AsyncNotifier<List<AgentTaskItem>> {
  Future<List<AgentTaskItem>> _fetch() async {
    final resp = await ref.read(backendServiceProvider).get<Map<String, dynamic>>(
      '/api/extensions/tasks',
      queryParameters: const {'limit': 200},
    );
    final rows = resp?['items'];
    if (rows is! List) return const [];
    return rows.whereType<Map>().map((e) => AgentTaskItem.fromJson(Map<String, dynamic>.from(e))).toList(growable: false);
  }

  @override
  Future<List<AgentTaskItem>> build() => _fetch();

  Future<void> refresh() async {
    state = const AsyncValue.loading();
    state = await AsyncValue.guard(_fetch);
  }

  Future<void> createTask({
    required String taskDefinitionId,
    required String title,
    required String description,
    required List<String> abilities,
    int stepCount = 3,
  }) async {
    final definitions = await ref.read(agentTaskDefinitionsProvider.future);
    AgentTaskDefinition? definition;
    for (final candidate in definitions) {
      if (candidate.taskId == taskDefinitionId) {
        definition = candidate;
        break;
      }
    }
    if (definition == null) {
      throw StateError('选择的 Kernel Task 定义已不存在，请重新选择');
    }
    final api = ref.read(backendServiceProvider);
    await api.post<Map<String, dynamic>>(
      '/api/extensions/tasks',
      data: {
        'taskDefinitionId': definition.taskId,
        'extensionId': definition.extensionId,
        'moduleId': definition.moduleId,
        'input': {
          'title': title,
          'description': description,
          'requiredAbilities': abilities,
          'stepCount': stepCount,
        },
        'priority': 0,
        'source': 'mobile_agent',
      },
    );
    await refresh();
  }

  Future<void> changeStatus(String id, AgentTaskStatus newStatus) async {
    final api = ref.read(backendServiceProvider);
    final items = state.valueOrNull ?? const <AgentTaskItem>[];
    AgentTaskItem? current;
    for (final item in items) {
      if (item.id == id) {
        current = item;
        break;
      }
    }
    if (newStatus == AgentTaskStatus.cancelled) {
      await api.post<Map<String, dynamic>>('/api/extensions/tasks/$id/cancel', data: const {'reason': 'user_requested'});
    } else if (newStatus == AgentTaskStatus.paused) {
      await api.post<Map<String, dynamic>>('/api/extensions/tasks/$id/pause', data: {'generation': current?.generation ?? 0, 'reason': 'user_requested'});
    } else if (newStatus == AgentTaskStatus.running && current?.status == AgentTaskStatus.paused) {
      await api.post<Map<String, dynamic>>('/api/extensions/tasks/$id/resume', data: {'generation': current?.generation ?? 0, 'resumeKind': 'resume'});
    } else if (newStatus == AgentTaskStatus.running &&
        (current?.status == AgentTaskStatus.failed || current?.status == AgentTaskStatus.cancelled || current?.status == AgentTaskStatus.completed)) {
      await api.post<Map<String, dynamic>>('/api/extensions/tasks/$id/retry');
    } else {
      throw UnsupportedError(
        'Kernel Task 不支持从 ${current?.status.name ?? 'unknown'} 直接切换到 ${newStatus.name}；必须使用真实服务端 Action',
      );
    }
    await refresh();
    ref.invalidate(agentTaskRuntimeDetailProvider(id));
  }

  Future<void> recover(String id) async {
    await ref.read(backendServiceProvider).post<Map<String, dynamic>>(
      '/api/extensions/tasks/$id/recover',
    );
    await refresh();
    ref.invalidate(agentTaskRuntimeDetailProvider(id));
  }
}

class AgentTaskRuntimeDetail {
  final Map<String, dynamic> run;
  final Map<String, dynamic> progress;
  final Map<String, dynamic> result;
  final Map<String, dynamic> checkpoint;

  const AgentTaskRuntimeDetail({
    required this.run,
    required this.progress,
    required this.result,
    required this.checkpoint,
  });

  double? get percentage {
    final direct = (progress['percentage'] as num?)?.toDouble();
    if (direct != null) return direct.clamp(0, 100).toDouble();
    final current = (progress['current'] as num?)?.toDouble();
    final total = (progress['total'] as num?)?.toDouble();
    if (current != null && total != null && total > 0) return (current / total * 100).clamp(0, 100).toDouble();
    return null;
  }

  AgentTaskItem get task {
    final base = AgentTaskItem.fromJson(run);
    final input = run['input'] is Map ? Map<String, dynamic>.from(run['input'] as Map) : const <String, dynamic>{};
    final stage = (progress['stage'] ?? '').toString();
    final message = (progress['message'] ?? '').toString();
    final progressValue = percentage?.round() ?? base.progress;
    final resultText = _prettyPayload(result['resultJson'] ?? result['result'] ?? result['payload']);
    return AgentTaskItem(
      id: base.id,
      title: (input['title'] ?? base.title).toString(),
      description: (input['description'] ?? message).toString().trim().isEmpty ? base.description : (input['description'] ?? message).toString(),
      requiredAbilities: base.requiredAbilities,
      steps: stage.isEmpty ? const <String>[] : <String>[stage],
      status: base.status,
      progress: progressValue.clamp(0, 100).toInt(),
      currentStepIndex: 0,
      elapsed: base.elapsed,
      result: resultText.isEmpty ? base.result : resultText,
      error: base.error,
      createdAt: base.createdAt,
      generation: base.generation,
    );
  }

  static String _prettyPayload(dynamic value) {
    if (value == null) return '';
    if (value is String) {
      if (value.trim().isEmpty) return '';
      try {
        return const JsonEncoder.withIndent('  ').convert(jsonDecode(value));
      } catch (_) {
        return value;
      }
    }
    try {
      return const JsonEncoder.withIndent('  ').convert(value);
    } catch (_) {
      return value.toString();
    }
  }
}

Future<Map<String, dynamic>> _optionalTaskMap(Future<Map<String, dynamic>?> request) async {
  try {
    return await request ?? const <String, dynamic>{};
  } catch (_) {
    return const <String, dynamic>{};
  }
}

final agentTaskDefinitionsProvider = FutureProvider.autoDispose<List<AgentTaskDefinition>>((ref) async {
  final api = ref.read(backendServiceProvider);
  final response = await api.get<Map<String, dynamic>>('/api/extensions/task-definitions');
  final rows = response?['items'];
  if (rows is! List) return const <AgentTaskDefinition>[];
  return rows
      .whereType<Map>()
      .map((row) => AgentTaskDefinition.fromJson(Map<String, dynamic>.from(row)))
      .where((definition) => definition.taskId.isNotEmpty)
      .toList(growable: false);
});

final agentTaskRuntimeDetailProvider = FutureProvider.autoDispose.family<AgentTaskRuntimeDetail, String>((ref, taskId) async {
  final api = ref.read(backendServiceProvider);
  final run = await api.get<Map<String, dynamic>>('/api/extensions/tasks/$taskId');
  if (run == null || run.isEmpty) throw StateError('任务不存在');
  final values = await Future.wait<Map<String, dynamic>>([
    _optionalTaskMap(api.get<Map<String, dynamic>>('/api/extensions/tasks/$taskId/progress')),
    _optionalTaskMap(api.get<Map<String, dynamic>>('/api/extensions/tasks/$taskId/result')),
    _optionalTaskMap(api.get<Map<String, dynamic>>('/api/extensions/tasks/$taskId/checkpoint')),
  ]);
  return AgentTaskRuntimeDetail(
    run: run,
    progress: values[0],
    result: values[1],
    checkpoint: values[2],
  );
});

final agentTasksProvider = AsyncNotifierProvider<AgentTaskNotifier, List<AgentTaskItem>>(AgentTaskNotifier.new);

final agentTaskDetailProvider = FutureProvider.autoDispose.family<AgentTaskItem?, String>((ref, taskId) async {
  try {
    return (await ref.watch(agentTaskRuntimeDetailProvider(taskId).future)).task;
  } catch (_) {
    final list = await ref.watch(agentTasksProvider.future);
    for (final task in list) {
      if (task.id == taskId) return task;
    }
    return null;
  }
});
