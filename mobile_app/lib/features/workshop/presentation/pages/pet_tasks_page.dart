import 'package:dio/dio.dart';
import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:go_router/go_router.dart';

import '../../../../app/app_routes.dart';
import '../../../../app/theme/app_colors.dart';
import '../../../../app/theme/app_radius.dart';
import '../../../../app/theme/app_spacing.dart';
import '../../../../app/theme/app_typography.dart';
import '../../../../core/backend_transport/providers/backend_transport_providers.dart';
import '../../../../core/widgets/amitia_misc.dart';
import '../../../../core/widgets/amitia_scaffold.dart';

class PetTasksPage extends ConsumerStatefulWidget {
  const PetTasksPage({super.key});

  @override
  ConsumerState<PetTasksPage> createState() => _PetTasksPageState();
}

class _PetTasksPageState extends ConsumerState<PetTasksPage> {
  List<Map<String, dynamic>> _tasks = const [];
  bool _loading = true;
  String? _error;

  @override
  void initState() {
    super.initState();
    _load();
  }

  Future<void> _load() async {
    setState(() { _loading = true; _error = null; });
    try {
      final api = ref.read(backendServiceProvider);
      final response = await api.get<dynamic>('/api/desktop-pets/generation-tasks', queryParameters: {'page': 1, 'pageSize': 100});
      final items = response is Map ? response['items'] : null;
      if (!mounted) return;
      setState(() {
        _tasks = items is List ? items.whereType<Map>().map((e) => Map<String, dynamic>.from(e)).toList() : const [];
        _loading = false;
      });
    } catch (e) {
      if (mounted) setState(() { _error = e.toString(); _loading = false; });
    }
  }

  String _statusLabel(String? status) {
    switch (status) {
      case 'pending': return '待开始';
      case 'queued': return '排队中';
      case 'running':
      case 'processing': return '生成中';
      case 'succeeded':
      case 'completed': return '已完成';
      case 'failed': return '失败';
      case 'cancelling': return '取消中';
      case 'cancelled': return '已取消';
      default: return status?.isNotEmpty == true ? status! : '未知';
    }
  }

  BadgeType _statusBadgeType(String? status) {
    if (status == 'succeeded' || status == 'completed') return BadgeType.success;
    if (status == 'failed' || status == 'cancelled') return BadgeType.error;
    if (status == 'running' || status == 'processing' || status == 'queued') return BadgeType.accent;
    return BadgeType.neutral;
  }

  bool _canCreateProcessing(String? status) => status == 'succeeded' || status == 'completed';
  bool _canCancel(String? status) => !{'succeeded', 'completed', 'failed', 'cancelled'}.contains(status);

  @override
  Widget build(BuildContext context) {
    return AmitiaScaffold(
      appBar: AmitiaAppBar(
        title: '桌宠生成任务',
        showBackButton: true,
        fallbackRoute: AppRoutes.workshop,
        actions: [
          AmitiaIconButton(icon: Icons.refresh, onPressed: _load, color: context.textSecondary),
          Padding(
            padding: EdgeInsets.only(right: AppSpacing.sm),
            child: AmitiaIconButton(icon: Icons.add, color: context.accentPrimary, onPressed: () => context.push(AppRoutes.workshopPetCreate)),
          ),
        ],
      ),
      body: SafeArea(top: false, child: _buildBody(context)),
    );
  }

  Widget _buildBody(BuildContext context) {
    if (_loading) return const AmitiaLoadingState();
    if (_error != null) return AmitiaErrorState(message: _error!, onRetry: _load);
    if (_tasks.isEmpty) {
      return AmitiaEmptyState(
        icon: Icons.assignment_outlined,
        title: '暂无桌宠生成任务',
        subtitle: '创建任务后，生成、处理、打包和安装都将使用桌宠后端链路。',
        actionText: '创建桌宠',
        onAction: () => context.push(AppRoutes.workshopPetCreate),
      );
    }
    return RefreshIndicator(
      onRefresh: _load,
      child: ListView.builder(
        padding: EdgeInsets.all(AppSpacing.pagePadding),
        itemCount: _tasks.length,
        itemBuilder: (context, index) => _buildTaskCard(context, _tasks[index]),
      ),
    );
  }

  Widget _buildTaskCard(BuildContext context, Map<String, dynamic> task) {
    final id = (task['id'] ?? '').toString();
    final name = (task['name'] ?? id).toString();
    final characterName = (task['characterName'] ?? '').toString();
    final status = task['status']?.toString();
    final progress = (task['progress'] as num?)?.toInt() ?? 0;
    final actions = (task['selectedActionCount'] as num?)?.toInt() ?? 0;
    final stage = (task['currentStage'] ?? '').toString();
    final canProcess = _canCreateProcessing(status);
    return Padding(
      padding: EdgeInsets.only(bottom: AppSpacing.sm),
      child: AmitiaCard(
        onTap: () => context.push(AppRoutes.petProcessing(id)),
        child: Column(crossAxisAlignment: CrossAxisAlignment.start, children: [
          Row(children: [
            Container(
              width: 44,
              height: 44,
              decoration: BoxDecoration(color: context.accentSoft, borderRadius: AppRadius.brSmall),
              child: Icon(Icons.pets_outlined, size: 22, color: context.accentPrimary),
            ),
            SizedBox(width: AppSpacing.md),
            Expanded(child: Column(crossAxisAlignment: CrossAxisAlignment.start, children: [
              Text(name, style: AppTypography.cardTitle(context)),
              if (characterName.isNotEmpty) Text(characterName, style: AppTypography.caption(context)),
              if (stage.isNotEmpty) Text(stage, style: AppTypography.label(context).copyWith(color: context.textSecondary)),
            ])),
            AmitiaStatusBadge(label: _statusLabel(status), type: _statusBadgeType(status)),
          ]),
          SizedBox(height: AppSpacing.md),
          Row(children: [
            Text('$actions 个动作', style: AppTypography.caption(context)),
            SizedBox(width: AppSpacing.md),
            Expanded(child: AmitiaProgressBar(progress: (progress / 100).clamp(0.0, 1.0))),
            SizedBox(width: AppSpacing.sm),
            Text('$progress%', style: AppTypography.caption(context)),
          ]),
          SizedBox(height: AppSpacing.md),
          Row(children: [
            Expanded(
              child: AmitiaButton(
                label: canProcess ? '创建处理任务' : '查看详情',
                height: 38,
                isSecondary: !canProcess,
                icon: canProcess ? Icons.auto_fix_high : Icons.open_in_new,
                onPressed: canProcess ? () => _createProcessingTask(id, name) : () => context.push(AppRoutes.petProcessing(id)),
              ),
            ),
            if (_canCancel(status)) ...[
              SizedBox(width: AppSpacing.sm),
              AmitiaIconButton(icon: Icons.close, color: context.error, onPressed: () => _cancel(id, name)),
            ],
          ]),
        ]),
      ),
    );
  }

  Future<void> _cancel(String taskId, String name) async {
    final confirmed = await showDialog<bool>(
      context: context,
      builder: (dialogContext) => AlertDialog(
        title: const Text('取消生成任务'),
        content: Text('确认取消「$name」？'),
        actions: [
          TextButton(onPressed: () => Navigator.pop(dialogContext, false), child: const Text('保留')),
          TextButton(onPressed: () => Navigator.pop(dialogContext, true), child: const Text('取消任务')),
        ],
      ),
    );
    if (confirmed != true) return;
    try {
      await ref.read(backendServiceProvider).post<dynamic>('/api/desktop-pets/generation-tasks/$taskId/cancel');
      await _load();
    } catch (e) {
      if (mounted) ScaffoldMessenger.of(context).showSnackBar(SnackBar(content: Text('取消失败：$e')));
    }
  }

  Future<void> _createProcessingTask(String generationTaskId, String name) async {
    try {
      final form = FormData.fromMap({
        'outputWidth': '512',
        'outputHeight': '512',
        'targetCharacterHeightRatio': '0.82',
        'anchorMode': 'bottom_center',
        'backgroundMode': 'transparent',
        'outputFormat': 'png',
        'defaultFps': '12',
      });
      final response = await ref.read(backendServiceProvider).post<dynamic>(
        '/api/desktop-pets/generation-tasks/$generationTaskId/process',
        data: form,
      );
      final result = response is Map ? Map<String, dynamic>.from(response) : const <String, dynamic>{};
      final processingId = (result['id'] ?? '').toString();
      if (!mounted) return;
      ScaffoldMessenger.of(context).showSnackBar(SnackBar(content: Text('已为「$name」创建处理任务')));
      context.push(AppRoutes.petProcessing(processingId.isEmpty ? generationTaskId : processingId));
    } catch (e) {
      if (mounted) ScaffoldMessenger.of(context).showSnackBar(SnackBar(content: Text('创建处理任务失败：$e')));
    }
  }
}
