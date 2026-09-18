import 'package:flutter_riverpod/flutter_riverpod.dart';

import '../../../../core/services/providers.dart';

enum AgentTaskStatus {
  created,
  queued,
  starting,
  running,
  checkpointing,
  pausing,
  paused,
  resuming,
  cancelling,
  cancelled,
  succeeded,
  failed,
  timedOut,
  recoveryRequired,
  manualIntervention,
}

class AgentTaskItem {
  final String id;
  final String title;
  final String description;
  final List<String> requiredAbilities;
  final AgentTaskStatus status;
  final double? progress;
  final String elapsed;
  final String? result;
  final String? error;
  final DateTime createdAt;
  final int generation;
  final int attempt;
  final int maxAttempts;
  final String executionPlacement;
  final bool checkpointSupported;
  final bool retrySupported;

  AgentTaskItem({
    required this.id,
    required this.title,
    required this.description,
    required this.requiredAbilities,
    required this.status,
    required this.progress,
    required this.elapsed,
    this.result,
    this.error,
    required this.createdAt,
    this.generation = 0,
    this.attempt = 0,
    this.maxAttempts = 0,
    this.executionPlacement = '',
    this.checkpointSupported = false,
    this.retrySupported = false,
  });

  factory AgentTaskItem.fromJson(
    Map<String, dynamic> json, {
    Map<String, dynamic>? definition,
  }) {
    final status = parseStatus(json['status']?.toString());
    final createdAt =
        DateTime.tryParse((json['createdAt'] ?? '').toString()) ??
        DateTime.now();
    final finishedAt = DateTime.tryParse((json['finishedAt'] ?? '').toString());
    final startedAt = DateTime.tryParse((json['startedAt'] ?? '').toString());
    final end = finishedAt ?? DateTime.now();
    final elapsedDuration = end.difference(startedAt ?? createdAt);
    final title = (json['taskDefinitionId'] ?? json['taskRunId'] ?? '')
        .toString();
    final extensionId = (json['extensionId'] ?? '').toString();
    final moduleId = (json['moduleId'] ?? '').toString();
    final progress = _readProgress(json);
    final idempotency = (definition?['idempotency'] ?? '').toString();
    final definitionRetrySupported = idempotency.isNotEmpty
        ? idempotency != 'non_idempotent'
        : definition?['idempotent'] == true;
    return AgentTaskItem(
      id: (json['taskRunId'] ?? json['id'] ?? '').toString(),
      title: title.isEmpty ? 'Kernel Task' : title,
      description: [
        extensionId,
        moduleId,
      ].where((e) => e.isNotEmpty).join(' · '),
      requiredAbilities: [
        if (extensionId.isNotEmpty) extensionId,
        if (moduleId.isNotEmpty) moduleId,
      ],
      status: status,
      progress: progress,
      elapsed: formatDuration(elapsedDuration),
      result: json['result']?.toString(),
      error: json['errorMessage']?.toString() ?? json['error']?.toString(),
      createdAt: createdAt,
      generation: (json['generation'] as num?)?.toInt() ?? 0,
      attempt: (json['attempt'] as num?)?.toInt() ?? 0,
      maxAttempts: (json['maxAttempts'] as num?)?.toInt() ?? 0,
      executionPlacement: (json['executionPlacement'] ?? '').toString(),
      checkpointSupported: definition?['checkpoint'] == true,
      retrySupported: definitionRetrySupported,
    );
  }

  static double? _readProgress(Map<String, dynamic> json) {
    final direct = json['percentage'];
    if (direct is num) return direct.toDouble().clamp(0.0, 100.0).toDouble();
    final raw = json['progress'];
    if (raw is Map) {
      final percentage = raw['percentage'];
      if (percentage is num)
        return percentage.toDouble().clamp(0.0, 100.0).toDouble();
      final current = raw['current'];
      final total = raw['total'];
      if (current is num && total is num && total > 0) {
        return (current.toDouble() / total.toDouble() * 100)
            .clamp(0.0, 100.0)
            .toDouble();
      }
    }
    return null;
  }

  static AgentTaskStatus parseStatus(String? value) {
    switch (value) {
      case 'created':
        return AgentTaskStatus.created;
      case 'queued':
        return AgentTaskStatus.queued;
      case 'starting':
        return AgentTaskStatus.starting;
      case 'running':
        return AgentTaskStatus.running;
      case 'checkpointing':
        return AgentTaskStatus.checkpointing;
      case 'pausing':
        return AgentTaskStatus.pausing;
      case 'paused':
        return AgentTaskStatus.paused;
      case 'resuming':
        return AgentTaskStatus.resuming;
      case 'cancelling':
        return AgentTaskStatus.cancelling;
      case 'cancelled':
        return AgentTaskStatus.cancelled;
      case 'succeeded':
        return AgentTaskStatus.succeeded;
      case 'failed':
        return AgentTaskStatus.failed;
      case 'timed_out':
        return AgentTaskStatus.timedOut;
      case 'recovery_required':
        return AgentTaskStatus.recoveryRequired;
      case 'manual_intervention':
        return AgentTaskStatus.manualIntervention;
      default:
        return AgentTaskStatus.created;
    }
  }

  static String formatDuration(Duration duration) {
    final safe = duration.isNegative ? Duration.zero : duration;
    final hours = safe.inHours.toString().padLeft(2, '0');
    final minutes = (safe.inMinutes % 60).toString().padLeft(2, '0');
    final seconds = (safe.inSeconds % 60).toString().padLeft(2, '0');
    return '$hours:$minutes:$seconds';
  }

  bool get isActive => const <AgentTaskStatus>{
    AgentTaskStatus.created,
    AgentTaskStatus.queued,
    AgentTaskStatus.starting,
    AgentTaskStatus.running,
    AgentTaskStatus.checkpointing,
    AgentTaskStatus.pausing,
    AgentTaskStatus.paused,
    AgentTaskStatus.resuming,
    AgentTaskStatus.cancelling,
    AgentTaskStatus.recoveryRequired,
  }.contains(status);

  bool get isTerminal => const <AgentTaskStatus>{
    AgentTaskStatus.succeeded,
    AgentTaskStatus.failed,
    AgentTaskStatus.cancelled,
    AgentTaskStatus.timedOut,
    AgentTaskStatus.manualIntervention,
  }.contains(status);

  bool get needsAttention =>
      status == AgentTaskStatus.recoveryRequired ||
      status == AgentTaskStatus.manualIntervention;

  bool get isLocalExecution =>
      executionPlacement.trim().isEmpty || executionPlacement == 'local';

  bool get canPause =>
      checkpointSupported &&
      isLocalExecution &&
      (status == AgentTaskStatus.running ||
          status == AgentTaskStatus.checkpointing);

  bool get canResume => isLocalExecution && status == AgentTaskStatus.paused;

  bool get canCancel {
    if (executionPlacement == 'cloud' || executionPlacement == 'device') {
      return status == AgentTaskStatus.queued ||
          status == AgentTaskStatus.running ||
          status == AgentTaskStatus.cancelling;
    }
    return status == AgentTaskStatus.queued ||
        status == AgentTaskStatus.running ||
        status == AgentTaskStatus.pausing ||
        status == AgentTaskStatus.paused;
  }

  bool get canRecover =>
      status == AgentTaskStatus.recoveryRequired ||
      status == AgentTaskStatus.manualIntervention;

  bool get canRetry =>
      isTerminal && retrySupported && maxAttempts > 0 && attempt < maxAttempts;
}

class AgentTaskNotifier extends AsyncNotifier<List<AgentTaskItem>> {
  Future<List<AgentTaskItem>> _fetch() async {
    final service = ref.read(extensionTaskServiceProvider);
    final values = await Future.wait<List<Map<String, dynamic>>>([
      service.listRuns(limit: 200),
      service.listDefinitions(),
    ]);
    final definitions = <String, Map<String, dynamic>>{
      for (final definition in values[1])
        if ((definition['taskId'] ?? '').toString().trim().isNotEmpty)
          (definition['taskId'] ?? '').toString(): definition,
    };
    return values[0]
        .map(
          (row) => AgentTaskItem.fromJson(
            row,
            definition: definitions[(row['taskDefinitionId'] ?? '').toString()],
          ),
        )
        .toList(growable: false);
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
    String? deviceId,
  }) async {
    final service = ref.read(extensionTaskServiceProvider);
    final selectedTaskDefinitionId = taskDefinitionId.trim();
    if (selectedTaskDefinitionId.isEmpty) {
      throw StateError('请选择可执行的 Kernel Task 定义');
    }
    final rows = await service.listDefinitions();
    Map<String, dynamic>? definition;
    for (final row in rows) {
      if ((row['taskId'] ?? '').toString() == selectedTaskDefinitionId) {
        definition = row;
        break;
      }
    }
    if (definition == null) {
      throw StateError('选定的 Kernel Task 定义不存在或已被移除');
    }
    await service.enqueue(
      taskDefinitionId: selectedTaskDefinitionId,
      extensionId: (definition['extensionId'] ?? '').toString(),
      moduleId: (definition['moduleId'] ?? '').toString(),
      input: {
        'title': title,
        'description': description,
        'requiredAbilities': abilities,
        'stepCount': stepCount,
      },
      priority: 0,
      source: 'mobile_agent',
      deviceId: deviceId,
    );
    await refresh();
  }

  Future<AgentTaskItem> _current(String id) async {
    final items = state.valueOrNull ?? const <AgentTaskItem>[];
    for (final item in items) {
      if (item.id == id) return item;
    }
    final service = ref.read(extensionTaskServiceProvider);
    final detail = await service.runtimeDetail(id);
    final run = detail['run'];
    if (run is! Map) throw StateError('任务运行不存在');
    final runMap = Map<String, dynamic>.from(run);
    final definitions = await service.listDefinitions();
    Map<String, dynamic>? definition;
    final definitionId = (runMap['taskDefinitionId'] ?? '').toString();
    for (final row in definitions) {
      if ((row['taskId'] ?? '').toString() == definitionId) {
        definition = row;
        break;
      }
    }
    return AgentTaskItem.fromJson(runMap, definition: definition);
  }

  Future<void> pause(String id) async {
    final current = await _current(id);
    if (!current.canPause) throw StateError('当前任务状态不允许暂停');
    await ref
        .read(extensionTaskServiceProvider)
        .pause(id, generation: current.generation);
    await refresh();
  }

  Future<void> resume(String id) async {
    final current = await _current(id);
    if (!current.canResume) throw StateError('当前任务状态不允许继续');
    await ref
        .read(extensionTaskServiceProvider)
        .resume(id, generation: current.generation);
    await refresh();
  }

  Future<void> cancel(String id) async {
    final current = await _current(id);
    if (!current.canCancel) throw StateError('当前任务状态不允许取消');
    await ref.read(extensionTaskServiceProvider).cancel(id);
    await refresh();
  }

  Future<void> retry(String id) async {
    final current = await _current(id);
    if (!current.canRetry) throw StateError('当前任务状态不允许重试');
    await ref.read(extensionTaskServiceProvider).retry(id);
    await refresh();
  }

  Future<void> recover(String id) async {
    final current = await _current(id);
    if (!current.canRecover) throw StateError('当前任务状态不允许恢复');
    await ref.read(extensionTaskServiceProvider).recover(id);
    await refresh();
  }
}

class AgentTaskDefinitionOption {
  final String taskId;
  final String extensionId;
  final String moduleId;
  final String executionPlacement;
  final bool checkpointSupported;
  final bool retrySupported;

  const AgentTaskDefinitionOption({
    required this.taskId,
    required this.extensionId,
    required this.moduleId,
    required this.executionPlacement,
    this.checkpointSupported = false,
    this.retrySupported = false,
  });

  String get label {
    final owner = [
      extensionId,
      moduleId,
    ].where((part) => part.isNotEmpty).join(' / ');
    final placement = executionPlacement.isEmpty ? 'local' : executionPlacement;
    return owner.isEmpty
        ? '$taskId · $placement'
        : '$taskId · $owner · $placement';
  }

  factory AgentTaskDefinitionOption.fromJson(Map<String, dynamic> json) {
    return AgentTaskDefinitionOption(
      taskId: (json['taskId'] ?? '').toString().trim(),
      extensionId: (json['extensionId'] ?? '').toString().trim(),
      moduleId: (json['moduleId'] ?? '').toString().trim(),
      executionPlacement: (json['executionPlacement'] ?? '').toString().trim(),
      checkpointSupported: json['checkpointSupported'] == true,
      retrySupported: json['retrySupported'] == true,
    );
  }
}

class AgentTaskDeviceOption {
  final String deviceId;
  final String label;
  final String platform;

  const AgentTaskDeviceOption({
    required this.deviceId,
    required this.label,
    required this.platform,
  });

  String get displayLabel {
    final display = label.isNotEmpty
        ? label
        : (platform.isNotEmpty ? platform : 'device');
    return '$display · $deviceId';
  }
}

final agentTaskDefinitionsProvider =
    FutureProvider.autoDispose<List<AgentTaskDefinitionOption>>((ref) async {
      final rows = await ref
          .read(extensionTaskServiceProvider)
          .listDefinitions();
      return rows
          .map(AgentTaskDefinitionOption.fromJson)
          .where((item) => item.taskId.isNotEmpty)
          .toList(growable: false);
    });

final agentTaskDevicesProvider =
    FutureProvider.autoDispose<List<AgentTaskDeviceOption>>((ref) async {
      final rows = await ref.read(extensionServiceProvider).workflowDevices();
      return rows
          .where(
            (row) =>
                row['online'] == true &&
                (row['deviceId'] ?? '').toString().trim().isNotEmpty,
          )
          .map(
            (row) => AgentTaskDeviceOption(
              deviceId: (row['deviceId'] ?? '').toString().trim(),
              label: (row['label'] ?? '').toString().trim(),
              platform: (row['platform'] ?? '').toString().trim(),
            ),
          )
          .toList(growable: false);
    });

class AgentTaskRuntimeDetail {
  final Map<String, dynamic> run;
  final Map<String, dynamic> progress;
  final Map<String, dynamic> result;
  final Map<String, dynamic> checkpoint;
  final Map<String, dynamic>? definition;

  const AgentTaskRuntimeDetail({
    required this.run,
    required this.progress,
    required this.result,
    required this.checkpoint,
    this.definition,
  });

  AgentTaskItem get task => AgentTaskItem.fromJson(run, definition: definition);

  double? get percentage {
    final value = progress['percentage'];
    if (value is num) return value.toDouble().clamp(0.0, 100.0).toDouble();
    final current = progress['current'];
    final total = progress['total'];
    if (current is num && total is num && total > 0) {
      return (current.toDouble() / total.toDouble() * 100)
          .clamp(0.0, 100.0)
          .toDouble();
    }
    return null;
  }
}

final agentTaskRuntimeDetailProvider = FutureProvider.autoDispose
    .family<AgentTaskRuntimeDetail, String>((ref, taskId) async {
      final service = ref.read(extensionTaskServiceProvider);
      final detail = await service.runtimeDetail(taskId);
      Map<String, dynamic> part(String key) {
        final value = detail[key];
        return value is Map
            ? Map<String, dynamic>.from(value)
            : const <String, dynamic>{};
      }

      final run = part('run');
      final definitionId = (run['taskDefinitionId'] ?? '').toString();
      Map<String, dynamic>? definition;
      if (definitionId.isNotEmpty) {
        final definitions = await service.listDefinitions();
        for (final row in definitions) {
          if ((row['taskId'] ?? '').toString() == definitionId) {
            definition = row;
            break;
          }
        }
      }
      return AgentTaskRuntimeDetail(
        run: run,
        progress: part('progress'),
        result: part('result'),
        checkpoint: part('checkpoint'),
        definition: definition,
      );
    });

final agentTasksProvider =
    AsyncNotifierProvider<AgentTaskNotifier, List<AgentTaskItem>>(
      AgentTaskNotifier.new,
    );

final agentTaskDetailProvider = FutureProvider.autoDispose
    .family<AgentTaskItem?, String>((ref, taskId) async {
      final list = await ref.watch(agentTasksProvider.future);
      for (final task in list) {
        if (task.id == taskId) return task;
      }
      return null;
    });
