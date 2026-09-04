import 'dart:convert';

import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';

import '../../../../app/app_routes.dart';
import '../../../../app/theme/app_colors.dart';
import '../../../../app/theme/app_radius.dart';
import '../../../../app/theme/app_spacing.dart';
import '../../../../app/theme/app_typography.dart';
import '../../../../core/backend_transport/providers/backend_transport_providers.dart';
import '../../../../core/widgets/amitia_button.dart';
import '../../../../core/widgets/amitia_misc.dart';
import '../../../../core/widgets/amitia_scaffold.dart';

class KernelTasksPage extends ConsumerStatefulWidget {
  const KernelTasksPage({super.key});

  @override
  ConsumerState<KernelTasksPage> createState() => _KernelTasksPageState();
}

class _KernelTasksPageState extends ConsumerState<KernelTasksPage> {
  bool _loading = true;
  bool _working = false;
  String? _error;
  List<Map<String, dynamic>> _definitions = const [];
  List<Map<String, dynamic>> _runs = const [];

  @override
  void initState() {
    super.initState();
    _load();
  }

  Future<void> _load() async {
    setState(() {
      _loading = true;
      _error = null;
    });
    try {
      final api = ref.read(backendServiceProvider);
      final values = await Future.wait<Map<String, dynamic>?>([
        api.get<Map<String, dynamic>>('/api/extensions/task-definitions'),
        api.get<Map<String, dynamic>>(
          '/api/extensions/tasks',
          queryParameters: const {'limit': 200},
        ),
      ]);
      if (!mounted) return;
      setState(() {
        _definitions = _items(values[0]);
        _runs = _items(values[1]);
        _loading = false;
      });
    } catch (e) {
      if (!mounted) return;
      setState(() {
        _error = e.toString();
        _loading = false;
      });
    }
  }

  List<Map<String, dynamic>> _items(Map<String, dynamic>? page) {
    return ((page?['items'] as List?) ?? const [])
        .whereType<Map>()
        .map((item) => Map<String, dynamic>.from(item))
        .toList(growable: false);
  }

  Future<void> _runAction(String successMessage, Future<void> Function() action) async {
    if (_working) return;
    setState(() => _working = true);
    try {
      await action();
      await _load();
      if (mounted) ScaffoldMessenger.of(context).showSnackBar(SnackBar(content: Text(successMessage)));
    } catch (e) {
      if (mounted) ScaffoldMessenger.of(context).showSnackBar(SnackBar(content: Text('操作失败：$e')));
    } finally {
      if (mounted) setState(() => _working = false);
    }
  }

  String _definitionId(Map<String, dynamic> definition) => (definition['taskId'] ?? definition['id'] ?? '').toString();

  String _runId(Map<String, dynamic> run) => (run['taskRunId'] ?? run['id'] ?? '').toString();

  String _status(Map<String, dynamic> run) => (run['status'] ?? 'unknown').toString();

  String _statusLabel(String status) {
    switch (status.toLowerCase()) {
      case 'queued':
        return '排队中';
      case 'leased':
      case 'running':
      case 'checkpointing':
      case 'resuming':
        return '运行中';
      case 'paused':
      case 'pausing':
      case 'pause_requested':
        return '已暂停';
      case 'succeeded':
        return '已完成';
      case 'failed':
      case 'timed_out':
        return '失败';
      case 'cancelled':
      case 'cancelling':
        return '已取消';
      case 'recovery_required':
        return '需要恢复';
      case 'manual_intervention':
        return '需要人工处理';
      default:
        return status.isEmpty ? '未知' : status;
    }
  }

  BadgeType _badgeType(String status) {
    switch (status.toLowerCase()) {
      case 'running':
      case 'leased':
      case 'queued':
      case 'checkpointing':
      case 'resuming':
        return BadgeType.accent;
      case 'succeeded':
        return BadgeType.success;
      case 'failed':
      case 'timed_out':
        return BadgeType.error;
      case 'paused':
      case 'pausing':
      case 'pause_requested':
      case 'recovery_required':
      case 'manual_intervention':
        return BadgeType.warning;
      default:
        return BadgeType.neutral;
    }
  }

  String _pretty(dynamic value) {
    try {
      return const JsonEncoder.withIndent('  ').convert(value);
    } catch (_) {
      return value?.toString() ?? '';
    }
  }

  Future<void> _enqueue(Map<String, dynamic> definition) async {
    final definitionId = _definitionId(definition);
    if (definitionId.isEmpty) return;
    final controller = TextEditingController(text: '{}');
    final priorityController = TextEditingController(text: '0');
    final confirmed = await showDialog<bool>(
      context: context,
      builder: (dialogContext) => AlertDialog(
        title: Text('手动入队 · $definitionId'),
        content: SizedBox(
          width: 560,
          child: Column(
            mainAxisSize: MainAxisSize.min,
            children: [
              TextField(
                controller: controller,
                minLines: 7,
                maxLines: 14,
                style: const TextStyle(fontFamily: 'monospace', fontSize: 12),
                decoration: const InputDecoration(labelText: 'Input JSON', border: OutlineInputBorder()),
              ),
              const SizedBox(height: 10),
              TextField(
                controller: priorityController,
                keyboardType: TextInputType.number,
                decoration: const InputDecoration(labelText: 'Priority', border: OutlineInputBorder()),
              ),
            ],
          ),
        ),
        actions: [
          TextButton(onPressed: () => Navigator.pop(dialogContext, false), child: const Text('取消')),
          FilledButton(onPressed: () => Navigator.pop(dialogContext, true), child: const Text('入队')),
        ],
      ),
    );
    if (confirmed != true) {
      controller.dispose();
      priorityController.dispose();
      return;
    }
    try {
      final decoded = jsonDecode(controller.text);
      if (decoded is! Map) throw const FormatException('Input 必须是 JSON Object');
      final priority = int.tryParse(priorityController.text.trim()) ?? 0;
      await _runAction('任务已真实入队', () async {
        await ref.read(backendServiceProvider).post<Map<String, dynamic>>(
          '/api/extensions/tasks',
          data: {
            'taskDefinitionId': definitionId,
            'extensionId': (definition['extensionId'] ?? '').toString(),
            'moduleId': (definition['moduleId'] ?? '').toString(),
            'input': Map<String, dynamic>.from(decoded),
            'priority': priority,
            'source': 'mobile_kernel_tasks',
          },
        );
      });
    } catch (e) {
      if (mounted) ScaffoldMessenger.of(context).showSnackBar(SnackBar(content: Text('入队失败：$e')));
    } finally {
      controller.dispose();
      priorityController.dispose();
    }
  }

  Future<Map<String, dynamic>> _loadRuntimeDetail(String taskRunId) async {
    final api = ref.read(backendServiceProvider);
    final values = await Future.wait<Map<String, dynamic>?>([
      api.get<Map<String, dynamic>>('/api/extensions/tasks/$taskRunId'),
      api.get<Map<String, dynamic>>('/api/extensions/tasks/$taskRunId/progress'),
      api.get<Map<String, dynamic>>('/api/extensions/tasks/$taskRunId/result'),
      api.get<Map<String, dynamic>>('/api/extensions/tasks/$taskRunId/checkpoint'),
    ]);
    return {
      'run': values[0] ?? const <String, dynamic>{},
      'progress': values[1] ?? const <String, dynamic>{},
      'result': values[2] ?? const <String, dynamic>{},
      'checkpoint': values[3] ?? const <String, dynamic>{},
    };
  }

  Future<void> _showRunDetail(Map<String, dynamic> run) async {
    final id = _runId(run);
    if (id.isEmpty) return;
    await showModalBottomSheet<void>(
      context: context,
      isScrollControlled: true,
      backgroundColor: context.surfacePrimary,
      shape: const RoundedRectangleBorder(borderRadius: BorderRadius.vertical(top: Radius.circular(20))),
      builder: (sheetContext) => _KernelTaskRuntimeSheet(
        taskRunId: id,
        loader: () => _loadRuntimeDetail(id),
        onRetry: () async {
          Navigator.pop(sheetContext);
          await _runAction('任务已重新入队', () async {
            await ref.read(backendServiceProvider).post<Map<String, dynamic>>('/api/extensions/tasks/$id/retry');
          });
        },
        onRecover: () async {
          Navigator.pop(sheetContext);
          await _runAction('任务恢复操作已提交', () async {
            await ref.read(backendServiceProvider).post<Map<String, dynamic>>('/api/extensions/tasks/$id/recover');
          });
        },
      ),
    );
  }

  Future<void> _pause(Map<String, dynamic> run) async {
    final id = _runId(run);
    await _runAction('任务已暂停', () async {
      await ref.read(backendServiceProvider).post<Map<String, dynamic>>(
        '/api/extensions/tasks/$id/pause',
        data: {
          'generation': (run['generation'] as num?)?.toInt() ?? 0,
          'reason': 'mobile_user',
        },
      );
    });
  }

  Future<void> _resume(Map<String, dynamic> run) async {
    final id = _runId(run);
    await _runAction('任务已继续', () async {
      await ref.read(backendServiceProvider).post<Map<String, dynamic>>(
        '/api/extensions/tasks/$id/resume',
        data: {
          'generation': (run['generation'] as num?)?.toInt() ?? 0,
          'resumeKind': 'resume',
        },
      );
    });
  }

  Future<void> _cancel(Map<String, dynamic> run) async {
    final id = _runId(run);
    await _runAction('任务已请求取消', () async {
      await ref.read(backendServiceProvider).post<Map<String, dynamic>>(
        '/api/extensions/tasks/$id/cancel',
        data: const {'reason': 'mobile_user'},
      );
    });
  }

  Future<void> _retry(Map<String, dynamic> run) async {
    final id = _runId(run);
    await _runAction('任务已重新入队', () async {
      await ref.read(backendServiceProvider).post<Map<String, dynamic>>('/api/extensions/tasks/$id/retry');
    });
  }

  Future<void> _recover(Map<String, dynamic> run) async {
    final id = _runId(run);
    await _runAction('任务恢复操作已提交', () async {
      await ref.read(backendServiceProvider).post<Map<String, dynamic>>('/api/extensions/tasks/$id/recover');
    });
  }

  @override
  Widget build(BuildContext context) {
    if (_loading) return const AmitiaLoadingState(message: '加载 Kernel Task Runtime...');
    if (_error != null) return AmitiaErrorState(message: _error!, onRetry: _load);

    return AmitiaScaffold(
      appBar: AmitiaAppBar(
        title: '任务运行时',
        showBackButton: true,
        fallbackRoute: AppRoutes.developer,
        actions: [AmitiaIconButton(icon: Icons.refresh, tooltip: '刷新', onPressed: _working ? null : _load)],
      ),
      body: SafeArea(
        top: false,
        child: RefreshIndicator(
          onRefresh: _load,
          child: ListView(
            padding: EdgeInsets.fromLTRB(AppSpacing.pagePadding, AppSpacing.sm, AppSpacing.pagePadding, AppSpacing.xxxl),
            children: [
              AmitiaSectionHeader(title: 'Task Definitions', actionText: '${_definitions.length} 个定义'),
              SizedBox(height: AppSpacing.sm),
              if (_definitions.isEmpty)
                const AmitiaEmptyState(icon: Icons.schema_outlined, title: '暂无任务定义', subtitle: 'Kernel 扩展尚未注册 Task Definition')
              else
                ..._definitions.map(_definitionCard),
              SizedBox(height: AppSpacing.sectionGap),
              AmitiaSectionHeader(title: '运行实例', actionText: '${_runs.length} 个运行实例'),
              SizedBox(height: AppSpacing.sm),
              if (_runs.isEmpty)
                const AmitiaEmptyState(icon: Icons.play_circle_outline, title: '暂无运行实例', subtitle: '可从上方任务定义手动入队')
              else
                ..._runs.map(_runCard),
            ],
          ),
        ),
      ),
    );
  }

  Widget _definitionCard(Map<String, dynamic> definition) {
    final id = _definitionId(definition);
    final extensionId = (definition['extensionId'] ?? '').toString();
    final moduleId = (definition['moduleId'] ?? '').toString();
    final runtimeType = (definition['runtimeType'] ?? '').toString();
    final checkpoint = definition['checkpoint'] == true || definition['recoverable'] == true;
    return Padding(
      padding: EdgeInsets.only(bottom: AppSpacing.sm),
      child: AmitiaCard(
        child: Column(
          crossAxisAlignment: CrossAxisAlignment.start,
          children: [
            Row(
              children: [
                Expanded(child: Text(id.isEmpty ? 'Unnamed Task Definition' : id, style: AppTypography.cardTitle(context))),
                if (checkpoint) const AmitiaStatusBadge(label: '可恢复', type: BadgeType.success),
              ],
            ),
            const SizedBox(height: 6),
            Text([extensionId, moduleId, runtimeType].where((v) => v.isNotEmpty).join(' · '), style: AppTypography.caption(context)),
            const SizedBox(height: 10),
            Row(
              children: [
                Expanded(child: Text('Entry: ${(definition['entry'] ?? '').toString()}', style: AppTypography.label(context), maxLines: 2, overflow: TextOverflow.ellipsis)),
                const SizedBox(width: 10),
                AmitiaButton(
                  label: '手动入队',
                  icon: Icons.add_task,
                  isSecondary: true,
                  onPressed: _working || id.isEmpty ? null : () => _enqueue(definition),
                ),
              ],
            ),
          ],
        ),
      ),
    );
  }

  Widget _runCard(Map<String, dynamic> run) {
    final id = _runId(run);
    final definitionId = (run['taskDefinitionId'] ?? '').toString();
    final status = _status(run);
    final error = (run['errorMessage'] ?? '').toString();
    final checkpointId = (run['checkpointId'] ?? '').toString();
    final resultArtifactId = (run['resultArtifactId'] ?? '').toString();
    final isRunning = {'running', 'leased', 'checkpointing'}.contains(status.toLowerCase());
    final isPaused = {'paused', 'pausing', 'pause_requested'}.contains(status.toLowerCase());
    final canRetry = {'failed', 'timed_out', 'cancelled', 'succeeded'}.contains(status.toLowerCase());
    final canRecover = {'failed', 'timed_out', 'recovery_required', 'manual_intervention'}.contains(status.toLowerCase()) || checkpointId.isNotEmpty;

    return Padding(
      padding: EdgeInsets.only(bottom: AppSpacing.sm),
      child: AmitiaCard(
        onTap: id.isEmpty ? null : () => _showRunDetail(run),
        child: Column(
          crossAxisAlignment: CrossAxisAlignment.start,
          children: [
            Row(
              children: [
                Container(
                  width: 38,
                  height: 38,
                  decoration: BoxDecoration(color: context.accentSoft, borderRadius: AppRadius.brSmall),
                  child: Icon(Icons.task_alt, color: context.accentPrimary, size: 20),
                ),
                const SizedBox(width: 10),
                Expanded(
                  child: Column(
                    crossAxisAlignment: CrossAxisAlignment.start,
                    children: [
                      Text(definitionId.isEmpty ? id : definitionId, style: AppTypography.cardTitle(context)),
                      const SizedBox(height: 2),
                      Text(id, style: AppTypography.caption(context)),
                    ],
                  ),
                ),
                AmitiaStatusBadge(label: _statusLabel(status), type: _badgeType(status)),
              ],
            ),
            if (error.isNotEmpty) ...[
              const SizedBox(height: 8),
              Text(error, style: AppTypography.caption(context).copyWith(color: context.error), maxLines: 3, overflow: TextOverflow.ellipsis),
            ],
            if (checkpointId.isNotEmpty || resultArtifactId.isNotEmpty) ...[
              const SizedBox(height: 8),
              Wrap(
                spacing: 8,
                runSpacing: 6,
                children: [
                  if (checkpointId.isNotEmpty) _RunMetaChip(icon: Icons.bookmark_outline, label: 'Checkpoint $checkpointId'),
                  if (resultArtifactId.isNotEmpty) _RunMetaChip(icon: Icons.inventory_2_outlined, label: 'Result $resultArtifactId'),
                ],
              ),
            ],
            const SizedBox(height: 10),
            Wrap(
              spacing: 8,
              runSpacing: 8,
              children: [
                OutlinedButton.icon(onPressed: id.isEmpty ? null : () => _showRunDetail(run), icon: const Icon(Icons.info_outline), label: const Text('真实详情')),
                if (isRunning) OutlinedButton.icon(onPressed: _working ? null : () => _pause(run), icon: const Icon(Icons.pause), label: const Text('暂停')),
                if (isPaused) OutlinedButton.icon(onPressed: _working ? null : () => _resume(run), icon: const Icon(Icons.play_arrow), label: const Text('继续')),
                if (isRunning || isPaused || status == 'queued') OutlinedButton.icon(onPressed: _working ? null : () => _cancel(run), icon: const Icon(Icons.cancel_outlined), label: const Text('取消')),
                if (canRetry) OutlinedButton.icon(onPressed: _working ? null : () => _retry(run), icon: const Icon(Icons.replay), label: const Text('Retry')),
                if (canRecover) OutlinedButton.icon(onPressed: _working ? null : () => _recover(run), icon: const Icon(Icons.settings_backup_restore), label: const Text('Recover')),
              ],
            ),
          ],
        ),
      ),
    );
  }
}

class _KernelTaskRuntimeSheet extends StatefulWidget {
  final String taskRunId;
  final Future<Map<String, dynamic>> Function() loader;
  final Future<void> Function() onRetry;
  final Future<void> Function() onRecover;

  const _KernelTaskRuntimeSheet({
    required this.taskRunId,
    required this.loader,
    required this.onRetry,
    required this.onRecover,
  });

  @override
  State<_KernelTaskRuntimeSheet> createState() => _KernelTaskRuntimeSheetState();
}

class _KernelTaskRuntimeSheetState extends State<_KernelTaskRuntimeSheet> {
  late Future<Map<String, dynamic>> _future;

  @override
  void initState() {
    super.initState();
    _future = widget.loader();
  }

  String _pretty(dynamic value) {
    try {
      return const JsonEncoder.withIndent('  ').convert(value);
    } catch (_) {
      return value?.toString() ?? '';
    }
  }

  @override
  Widget build(BuildContext context) {
    return SafeArea(
      top: false,
      child: SizedBox(
        height: MediaQuery.sizeOf(context).height * 0.86,
        child: Padding(
          padding: const EdgeInsets.fromLTRB(20, 10, 20, 22),
          child: Column(
            crossAxisAlignment: CrossAxisAlignment.start,
            children: [
              Center(child: Container(width: 40, height: 4, decoration: BoxDecoration(color: context.borderPrimary, borderRadius: BorderRadius.circular(2)))),
              const SizedBox(height: 14),
              Row(
                children: [
                  Expanded(child: Text('Runtime Detail', style: AppTypography.pageTitle(context))),
                  IconButton(onPressed: () => setState(() => _future = widget.loader()), icon: const Icon(Icons.refresh)),
                  IconButton(onPressed: () => Navigator.pop(context), icon: const Icon(Icons.close)),
                ],
              ),
              Text(widget.taskRunId, style: AppTypography.caption(context)),
              const SizedBox(height: 10),
              Expanded(
                child: FutureBuilder<Map<String, dynamic>>(
                  future: _future,
                  builder: (context, snapshot) {
                    if (snapshot.connectionState != ConnectionState.done) return const Center(child: CircularProgressIndicator());
                    if (snapshot.hasError) return Center(child: Text('加载失败：${snapshot.error}'));
                    final data = snapshot.data ?? const <String, dynamic>{};
                    final run = Map<String, dynamic>.from(data['run'] as Map? ?? const {});
                    final progress = Map<String, dynamic>.from(data['progress'] as Map? ?? const {});
                    final result = Map<String, dynamic>.from(data['result'] as Map? ?? const {});
                    final checkpoint = Map<String, dynamic>.from(data['checkpoint'] as Map? ?? const {});
                    final percentage = (progress['percentage'] as num?)?.toDouble();
                    final current = (progress['current'] as num?)?.toDouble();
                    final total = (progress['total'] as num?)?.toDouble();
                    final stage = (progress['stage'] ?? '').toString();
                    final message = (progress['message'] ?? '').toString();
                    final status = (run['status'] ?? '').toString().toLowerCase();
                    final checkpointId = (checkpoint['checkpointId'] ?? '').toString();
                    final canRetry = {'failed', 'timed_out', 'cancelled', 'succeeded'}.contains(status);
                    final canRecover = {'failed', 'timed_out', 'recovery_required', 'manual_intervention'}.contains(status) || checkpointId.isNotEmpty;
                    return ListView(
                      children: [
                        if (percentage != null) ...[
                          Text('真实进度 ${percentage.clamp(0, 100).toStringAsFixed(1)}%', style: AppTypography.cardTitle(context)),
                          const SizedBox(height: 6),
                          LinearProgressIndicator(value: percentage.clamp(0, 100) / 100),
                          const SizedBox(height: 6),
                        ] else if (current != null && total != null && total > 0) ...[
                          Text('真实进度 ${current.toStringAsFixed(0)} / ${total.toStringAsFixed(0)}', style: AppTypography.cardTitle(context)),
                          const SizedBox(height: 6),
                          LinearProgressIndicator(value: (current / total).clamp(0, 1)),
                          const SizedBox(height: 6),
                        ] else
                          Text('后端尚未上报 percentage/current/total，不显示虚构百分比', style: AppTypography.caption(context)),
                        if (stage.isNotEmpty || message.isNotEmpty) ...[
                          const SizedBox(height: 8),
                          Text([stage, message].where((v) => v.isNotEmpty).join(' · '), style: AppTypography.bodySmall(context)),
                        ],
                        const SizedBox(height: 14),
                        _RuntimeBlock(title: 'Run', text: _pretty(run)),
                        _RuntimeBlock(title: 'Progress', text: _pretty(progress)),
                        _RuntimeBlock(title: 'Result', text: _pretty(result)),
                        _RuntimeBlock(title: 'Checkpoint', text: _pretty(checkpoint)),
                        Wrap(
                          spacing: 8,
                          runSpacing: 8,
                          children: [
                            if (canRetry) FilledButton.icon(onPressed: widget.onRetry, icon: const Icon(Icons.replay), label: const Text('Retry')),
                            if (canRecover) OutlinedButton.icon(onPressed: widget.onRecover, icon: const Icon(Icons.settings_backup_restore), label: const Text('Recover')),
                          ],
                        ),
                      ],
                    );
                  },
                ),
              ),
            ],
          ),
        ),
      ),
    );
  }
}

class _RuntimeBlock extends StatelessWidget {
  final String title;
  final String text;

  const _RuntimeBlock({required this.title, required this.text});

  @override
  Widget build(BuildContext context) {
    return Padding(
      padding: const EdgeInsets.only(bottom: 14),
      child: Column(
        crossAxisAlignment: CrossAxisAlignment.start,
        children: [
          Text(title, style: AppTypography.cardTitle(context)),
          const SizedBox(height: 6),
          Container(
            width: double.infinity,
            constraints: const BoxConstraints(maxHeight: 230),
            padding: const EdgeInsets.all(10),
            decoration: BoxDecoration(color: context.surfaceSecondary, borderRadius: AppRadius.brSmall),
            child: SingleChildScrollView(child: SelectableText(text, style: const TextStyle(fontFamily: 'monospace', fontSize: 11))),
          ),
        ],
      ),
    );
  }
}

class _RunMetaChip extends StatelessWidget {
  final IconData icon;
  final String label;

  const _RunMetaChip({required this.icon, required this.label});

  @override
  Widget build(BuildContext context) {
    return Container(
      padding: const EdgeInsets.symmetric(horizontal: 8, vertical: 4),
      decoration: BoxDecoration(color: context.surfaceSecondary, borderRadius: AppRadius.brTag),
      child: Row(
        mainAxisSize: MainAxisSize.min,
        children: [
          Icon(icon, size: 13, color: context.textSecondary),
          const SizedBox(width: 4),
          ConstrainedBox(
            constraints: const BoxConstraints(maxWidth: 220),
            child: Text(label, style: AppTypography.caption(context), maxLines: 1, overflow: TextOverflow.ellipsis),
          ),
        ],
      ),
    );
  }
}
