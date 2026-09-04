import 'package:flutter_riverpod/flutter_riverpod.dart';

import '../../../../core/backend_transport/providers/backend_transport_providers.dart';

enum AgentTaskStatus { pending, waitingApproval, running, paused, completed, failed, cancelled }

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
    final title = (json['taskDefinitionId'] ?? json['taskRunId'] ?? '').toString();
    final extensionId = (json['extensionId'] ?? '').toString();
    final moduleId = (json['moduleId'] ?? '').toString();
    return AgentTaskItem(
      id: (json['taskRunId'] ?? json['id'] ?? '').toString(),
      title: title.isEmpty ? 'Kernel Task' : title,
      description: [extensionId, moduleId].where((e) => e.isNotEmpty).join(' · '),
      requiredAbilities: [if (extensionId.isNotEmpty) extensionId, if (moduleId.isNotEmpty) moduleId],
      steps: [
        '排队',
        '启动运行时',
        '执行任务',
        '持久化结果',
      ],
      status: status,
      progress: _progressFor(status),
      currentStepIndex: _stepFor(status),
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

  static int _stepFor(AgentTaskStatus status) {
    switch (status) {
      case AgentTaskStatus.pending:
        return 0;
      case AgentTaskStatus.waitingApproval:
      case AgentTaskStatus.running:
      case AgentTaskStatus.paused:
        return 2;
      case AgentTaskStatus.completed:
      case AgentTaskStatus.failed:
      case AgentTaskStatus.cancelled:
        return 3;
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
    required String title,
    required String description,
    required List<String> abilities,
    int stepCount = 3,
  }) async {
    final api = ref.read(backendServiceProvider);
    final defs = await api.get<Map<String, dynamic>>('/api/extensions/task-definitions');
    final rows = defs?['items'];
    if (rows is! List || rows.isEmpty) {
      throw StateError('当前没有可执行的 Kernel Task 定义');
    }
    final definition = Map<String, dynamic>.from(rows.whereType<Map>().first);
    final taskDefinitionId = (definition['taskId'] ?? '').toString();
    if (taskDefinitionId.isEmpty) throw StateError('任务定义缺少 taskId');
    await api.post<Map<String, dynamic>>(
      '/api/extensions/tasks',
      data: {
        'taskDefinitionId': taskDefinitionId,
        'extensionId': (definition['extensionId'] ?? '').toString(),
        'moduleId': (definition['moduleId'] ?? '').toString(),
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
  }

  Future<void> recover(String id) async {
    await ref.read(backendServiceProvider).post<Map<String, dynamic>>(
      '/api/extensions/tasks/$id/recover',
    );
    await refresh();
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

  double? get percentage => (progress['percentage'] as num?)?.toDouble();
}

final agentTaskRuntimeDetailProvider = FutureProvider.autoDispose.family<AgentTaskRuntimeDetail, String>((ref, taskId) async {
  final api = ref.read(backendServiceProvider);
  final values = await Future.wait<Map<String, dynamic>?>([
    api.get<Map<String, dynamic>>('/api/extensions/tasks/$taskId'),
    api.get<Map<String, dynamic>>('/api/extensions/tasks/$taskId/progress'),
    api.get<Map<String, dynamic>>('/api/extensions/tasks/$taskId/result'),
    api.get<Map<String, dynamic>>('/api/extensions/tasks/$taskId/checkpoint'),
  ]);
  return AgentTaskRuntimeDetail(
    run: values[0] ?? const <String, dynamic>{},
    progress: values[1] ?? const <String, dynamic>{},
    result: values[2] ?? const <String, dynamic>{},
    checkpoint: values[3] ?? const <String, dynamic>{},
  );
});

final agentTasksProvider = AsyncNotifierProvider<AgentTaskNotifier, List<AgentTaskItem>>(AgentTaskNotifier.new);

final agentTaskDetailProvider = FutureProvider.autoDispose.family<AgentTaskItem?, String>((ref, taskId) async {
  final list = await ref.watch(agentTasksProvider.future);
  for (final task in list) {
    if (task.id == taskId) return task;
  }
  return null;
});
