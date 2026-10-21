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
  final String checkpointId;
  final Map<String, dynamic>? sourceRun;

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
    this.checkpointId = '',
    this.sourceRun,
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
      checkpointSupported:
          json['checkpointSupported'] == true ||
          (json['checkpointSupported'] == null &&
              definition?['checkpoint'] == true),
      retrySupported:
          json['retrySupported'] == true ||
          (json['retrySupported'] == null && definitionRetrySupported),
      checkpointId: (json['checkpointId'] ?? '').toString(),
      sourceRun: json['executionScope'] is Map
          ? Map<String, dynamic>.unmodifiable(json)
          : null,
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
      sourceRun?['readOnly'] != true &&
      checkpointSupported &&
      generation > 0 &&
      (isLocalExecution || executionPlacement == 'device') &&
      (status == AgentTaskStatus.running ||
          status == AgentTaskStatus.checkpointing);

  bool get canResume =>
      sourceRun?['readOnly'] != true &&
      checkpointSupported &&
      checkpointId.isNotEmpty &&
      generation > 0 &&
      (isLocalExecution || executionPlacement == 'device') &&
      status == AgentTaskStatus.paused;

  bool get canCancel {
    if (sourceRun?['readOnly'] == true) return false;
    if (executionPlacement == 'cloud' || executionPlacement == 'device') {
      return status == AgentTaskStatus.queued ||
          status == AgentTaskStatus.running ||
          status == AgentTaskStatus.pausing ||
          status == AgentTaskStatus.paused ||
          status == AgentTaskStatus.cancelling;
    }
    return status == AgentTaskStatus.queued ||
        status == AgentTaskStatus.running ||
        status == AgentTaskStatus.pausing ||
        status == AgentTaskStatus.paused;
  }

  bool get canRecover =>
      sourceRun?['readOnly'] != true &&
      (status == AgentTaskStatus.recoveryRequired ||
          status == AgentTaskStatus.manualIntervention);

  bool get canRetry =>
      sourceRun?['readOnly'] != true &&
      isTerminal &&
      retrySupported &&
      maxAttempts > 0 &&
      attempt < maxAttempts;
}

class AgentTaskNotifier extends AsyncNotifier<List<AgentTaskItem>> {
  Future<List<AgentTaskItem>> _fetch() async {
    final service = ref.read(extensionTaskServiceProvider);
    final runs = await service.listRuns(limit: 200);
    return runs
        .map((row) => AgentTaskItem.fromJson(row))
        .toList(growable: false);
  }

  @override
  Future<List<AgentTaskItem>> build() => _fetch();

  Future<void> refresh() async {
    state = const AsyncValue.loading();
    state = await AsyncValue.guard(_fetch);
  }

  Future<void> createTask(Map<String, dynamic> submission) async {
    final service = ref.read(extensionTaskServiceProvider);
    await service.enqueueOwnedTask(submission);
    await refresh();
  }

  Future<AgentTaskItem> _current(String id) async {
    final items = state.valueOrNull ?? const <AgentTaskItem>[];
    for (final item in items) {
      if (item.id == id) return item;
    }
    final service = ref.read(extensionTaskServiceProvider);
    if (service.bound) throw StateError('请重新加载并选择原任务记录');
    final detail = await service.runtimeDetail(id);
    final run = detail['run'];
    if (run is! Map) throw StateError('任务运行不存在');
    final runMap = Map<String, dynamic>.from(run);
    return AgentTaskItem.fromJson(runMap);
  }

  Future<void> pause(String id, {AgentTaskItem? expectedTask}) async {
    final current = expectedTask ?? await _current(id);
    if (!current.canPause) throw StateError('当前任务状态不允许暂停');
    await ref
        .read(extensionTaskServiceProvider)
        .pause(
          id,
          generation: current.generation,
          expectedRun: current.sourceRun,
        );
    await refresh();
  }

  Future<void> resume(String id, {AgentTaskItem? expectedTask}) async {
    final current = expectedTask ?? await _current(id);
    if (!current.canResume) throw StateError('当前任务状态不允许继续');
    await ref
        .read(extensionTaskServiceProvider)
        .resume(
          id,
          generation: current.generation,
          expectedRun: current.sourceRun,
        );
    await refresh();
  }

  Future<void> cancel(String id, {AgentTaskItem? expectedTask}) async {
    final current = expectedTask ?? await _current(id);
    if (!current.canCancel) throw StateError('当前任务状态不允许取消');
    await ref
        .read(extensionTaskServiceProvider)
        .cancel(id, expectedRun: current.sourceRun);
    await refresh();
  }

  Future<void> retry(String id, {AgentTaskItem? expectedTask}) async {
    final current = expectedTask ?? await _current(id);
    if (!current.canRetry) throw StateError('当前任务状态不允许重试');
    await ref
        .read(extensionTaskServiceProvider)
        .retry(id, expectedRun: current.sourceRun);
    await refresh();
  }

  Future<void> recover(String id, {AgentTaskItem? expectedTask}) async {
    final current = expectedTask ?? await _current(id);
    if (!current.canRecover) throw StateError('当前任务状态不允许恢复');
    await ref
        .read(extensionTaskServiceProvider)
        .recover(id, expectedRun: current.sourceRun);
    await refresh();
  }
}

class AgentTaskDefinitionOption {
  final String optionId;
  final Map<String, dynamic> definition;
  final Map<String, dynamic>? taskCatalogReference;
  final String executionPlacement;

  const AgentTaskDefinitionOption({
    required this.optionId,
    required this.definition,
    required this.taskCatalogReference,
    required this.executionPlacement,
  });

  String get taskId => taskCatalogReference == null
      ? (definition['taskId'] ?? '').toString()
      : (taskCatalogReference!['catalogId'] ?? '').toString();

  String get label {
    final extensionId = (definition['extensionId'] ?? '').toString().trim();
    final moduleId = (definition['moduleId'] ?? '').toString().trim();
    final sourceTaskId = taskCatalogReference == null
        ? taskId
        : (taskCatalogReference!['sourceTaskId'] ?? taskId).toString();
    final owner = [
      extensionId,
      moduleId,
    ].where((part) => part.isNotEmpty).join(' / ');
    final placement = executionPlacement.isEmpty ? 'local' : executionPlacement;
    return owner.isEmpty
        ? '$sourceTaskId · $placement'
        : '$sourceTaskId · $owner · $placement';
  }

  factory AgentTaskDefinitionOption.fromLocal(Map<String, dynamic> json) {
    return AgentTaskDefinitionOption(
      optionId: (json['taskId'] ?? '').toString().trim(),
      definition: Map<String, dynamic>.from(json),
      taskCatalogReference: null,
      executionPlacement:
          (json['executionPlacement'] ?? '').toString().trim().isEmpty
          ? 'local'
          : (json['executionPlacement'] ?? '').toString().trim(),
    );
  }

  factory AgentTaskDefinitionOption.fromCatalog(Map<String, dynamic> entry) {
    final reference = entry['reference'];
    final definition = entry['definition'];
    if (reference is! Map || definition is! Map)
      throw StateError('设备任务目录条目缺少定义或来源');
    final normalizedReference = Map<String, dynamic>.from(reference);
    return AgentTaskDefinitionOption(
      optionId: (normalizedReference['catalogId'] ?? '').toString(),
      definition: Map<String, dynamic>.from(definition),
      taskCatalogReference: normalizedReference,
      executionPlacement: 'device',
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

  const AgentTaskRuntimeDetail({
    required this.run,
    required this.progress,
    required this.result,
    required this.checkpoint,
  });

  AgentTaskItem get task => AgentTaskItem.fromJson(run);

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
    .family<AgentTaskRuntimeDetail, (String, Map<String, dynamic>?)>((
      ref,
      selection,
    ) async {
      final service = ref.read(extensionTaskServiceProvider);
      final detail = await service.runtimeDetail(
        selection.$1,
        expectedRun: selection.$2,
      );
      Map<String, dynamic> part(String key) {
        final value = detail[key];
        return value is Map
            ? Map<String, dynamic>.from(value)
            : const <String, dynamic>{};
      }

      return AgentTaskRuntimeDetail(
        run: part('run'),
        progress: part('progress'),
        result: part('result'),
        checkpoint: part('checkpoint'),
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
