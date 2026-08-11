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
  });

  AgentTaskItem copyWith({
    AgentTaskStatus? status,
    int? progress,
    int? currentStepIndex,
    String? elapsed,
    String? result,
    String? error,
  }) {
    return AgentTaskItem(
      id: id,
      title: title,
      description: description,
      requiredAbilities: requiredAbilities,
      steps: steps,
      status: status ?? this.status,
      progress: progress ?? this.progress,
      currentStepIndex: currentStepIndex ?? this.currentStepIndex,
      elapsed: elapsed ?? this.elapsed,
      result: result ?? this.result,
      error: error ?? this.error,
      createdAt: createdAt,
    );
  }

  factory AgentTaskItem.fromJson(Map<String, dynamic> json) {
    return AgentTaskItem(
      id: json['id']?.toString() ?? '',
      title: json['title']?.toString() ?? '',
      description: json['description']?.toString() ?? '',
      requiredAbilities: (json['requiredAbilities'] as List?)?.map((e) => e.toString()).toList() ?? [],
      steps: (json['steps'] as List?)?.map((e) => e.toString()).toList() ?? [],
      status: _parseStatus(json['status']?.toString()),
      progress: (json['progress'] as num?)?.toInt() ?? 0,
      currentStepIndex: (json['currentStepIndex'] as num?)?.toInt() ?? 0,
      elapsed: json['elapsed']?.toString() ?? '00:00',
      result: json['result']?.toString(),
      error: json['error']?.toString(),
      createdAt: json['createdAt'] != null ? DateTime.tryParse(json['createdAt'].toString()) ?? DateTime.now() : DateTime.now(),
    );
  }

  static AgentTaskStatus _parseStatus(String? s) {
    switch (s) {
      case 'pending': return AgentTaskStatus.pending;
      case 'waitingApproval': return AgentTaskStatus.waitingApproval;
      case 'running': return AgentTaskStatus.running;
      case 'paused': return AgentTaskStatus.paused;
      case 'completed': return AgentTaskStatus.completed;
      case 'failed': return AgentTaskStatus.failed;
      case 'cancelled': return AgentTaskStatus.cancelled;
      default: return AgentTaskStatus.pending;
    }
  }
}

class AgentTaskNotifier extends AsyncNotifier<List<AgentTaskItem>> {
  @override
  Future<List<AgentTaskItem>> build() async {
    final api = ref.watch(backendServiceProvider);
    if (api == null) return [];
    final resp = await api.get<List<dynamic>>('/api/agent/tasks');
    if (resp == null) return [];
    return resp.map((e) => AgentTaskItem.fromJson(e as Map<String, dynamic>)).toList();
  }

  Future<void> refresh() async {
    state = const AsyncValue.loading();
    state = await AsyncValue.guard(() async {
      final api = ref.watch(backendServiceProvider);
      if (api == null) return <AgentTaskItem>[];
      final resp = await api.get<List<dynamic>>('/api/agent/tasks');
      if (resp == null) return <AgentTaskItem>[];
      return resp.map((e) => AgentTaskItem.fromJson(e as Map<String, dynamic>)).toList();
    });
  }

  Future<void> createTask({
    required String title,
    required String description,
    required List<String> abilities,
    int stepCount = 3,
  }) async {
    final api = ref.watch(backendServiceProvider);
    if (api == null) return;
    await api.post('/api/agent/tasks', data: {
      'title': title,
      'description': description,
      'requiredAbilities': abilities,
      'stepCount': stepCount,
    });
    await refresh();
  }

  Future<void> changeStatus(String id, AgentTaskStatus newStatus) async {
    final api = ref.watch(backendServiceProvider);
    if (api == null) return;
    await api.post('/api/agent/tasks/$id/status', data: {
      'status': newStatus.name,
    });
    await refresh();
  }
}

final agentTasksProvider = AsyncNotifierProvider<AgentTaskNotifier, List<AgentTaskItem>>(AgentTaskNotifier.new);

final agentTaskDetailProvider = FutureProvider.autoDispose.family<AgentTaskItem?, String>((ref, taskId) async {
  final list = ref.watch(agentTasksProvider).maybeWhen(
    data: (items) => items,
    orElse: () => <AgentTaskItem>[],
  );
  return list.where((t) => t.id == taskId).firstOrNull;
});
