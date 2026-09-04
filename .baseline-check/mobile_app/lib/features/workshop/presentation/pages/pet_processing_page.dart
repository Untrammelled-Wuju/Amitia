import 'package:dio/dio.dart';
import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:go_router/go_router.dart';

import '../../../../app/app_routes.dart';
import '../../../../app/theme/app_colors.dart';
import '../../../../app/theme/app_spacing.dart';
import '../../../../app/theme/app_typography.dart';
import '../../../../core/backend_transport/providers/backend_transport_providers.dart';
import '../../../../core/widgets/amitia_misc.dart';
import '../../../../core/widgets/amitia_scaffold.dart';

class PetProcessingPage extends ConsumerStatefulWidget {
  final String taskId;

  const PetProcessingPage({super.key, required this.taskId});

  @override
  ConsumerState<PetProcessingPage> createState() => _PetProcessingPageState();
}

class _PetProcessingPageState extends ConsumerState<PetProcessingPage> {
  bool _loading = true;
  String? _error;
  bool _isProcessingTask = false;
  Map<String, dynamic> _task = const {};
  List<Map<String, dynamic>> _actions = const [];
  Map<String, dynamic> _qualitySummary = const {};

  @override
  void initState() {
    super.initState();
    _load();
  }

  Future<void> _load() async {
    setState(() { _loading = true; _error = null; });
    final api = ref.read(backendServiceProvider);
    try {
      final response = await api.get<dynamic>('/api/desktop-pets/processing-tasks/${widget.taskId}');
      if (response is Map) {
        final map = Map<String, dynamic>.from(response);
        final task = map['processingTask'];
        final actions = map['actions'];
        if (!mounted) return;
        setState(() {
          _isProcessingTask = true;
          _task = task is Map ? Map<String, dynamic>.from(task) : map;
          _actions = actions is List ? actions.whereType<Map>().map((e) => Map<String, dynamic>.from(e)).toList() : const [];
          _qualitySummary = map['qualitySummary'] is Map ? Map<String, dynamic>.from(map['qualitySummary'] as Map) : const {};
          _loading = false;
        });
        return;
      }
    } catch (_) {
      // The route also accepts a generation-task id from the task list. Fall back below.
    }

    try {
      final response = await api.get<dynamic>('/api/desktop-pets/generation-tasks/${widget.taskId}');
      if (!mounted) return;
      if (response is! Map) throw StateError('未找到桌宠任务');
      final map = Map<String, dynamic>.from(response);
      final actions = map['actions'];
      setState(() {
        _isProcessingTask = false;
        _task = map;
        _actions = actions is List ? actions.whereType<Map>().map((e) => Map<String, dynamic>.from(e)).toList() : const [];
        _qualitySummary = const {};
        _loading = false;
      });
    } catch (e) {
      if (mounted) setState(() { _error = e.toString(); _loading = false; });
    }
  }

  @override
  Widget build(BuildContext context) {
    return AmitiaScaffold(
      appBar: AmitiaAppBar(
        title: _isProcessingTask ? '桌宠处理任务' : '桌宠生成详情',
        showBackButton: true,
        fallbackRoute: AppRoutes.workshopPetTasks,
        actions: [AmitiaIconButton(icon: Icons.refresh, onPressed: _load, color: context.textSecondary)],
      ),
      body: SafeArea(top: false, child: _buildBody(context)),
    );
  }

  Widget _buildBody(BuildContext context) {
    if (_loading) return const AmitiaLoadingState();
    if (_error != null) return AmitiaErrorState(message: _error!, onRetry: _load);
    return RefreshIndicator(
      onRefresh: _load,
      child: ListView(
        padding: EdgeInsets.all(AppSpacing.pagePadding),
        children: [
          _buildSummary(context),
          SizedBox(height: AppSpacing.lg),
          if (_actions.isNotEmpty) ...[
            const AmitiaSectionHeader(title: '动作'),
            SizedBox(height: AppSpacing.sm),
            ..._actions.map((action) => _buildActionCard(context, action)),
          ] else
            const SizedBox(height: 180, child: AmitiaEmptyState(icon: Icons.animation, title: '暂无动作数据')),
          SizedBox(height: AppSpacing.lg),
          _buildTaskActions(context),
          SizedBox(height: AppSpacing.xxl),
        ],
      ),
    );
  }

  Widget _buildSummary(BuildContext context) {
    final status = (_task['status'] ?? 'unknown').toString();
    final progress = (_task['progress'] as num?)?.toInt() ?? 0;
    final stage = (_task['currentStage'] ?? '').toString();
    final name = (_task['name'] ?? _task['id'] ?? widget.taskId).toString();
    return AmitiaCard(
      child: Column(crossAxisAlignment: CrossAxisAlignment.start, children: [
        Row(children: [
          Expanded(child: Text(name, style: AppTypography.cardTitle(context))),
          AmitiaStatusBadge(label: _statusText(status), type: _statusType(status)),
        ]),
        if (stage.isNotEmpty) ...[
          SizedBox(height: AppSpacing.xs),
          Text(stage, style: AppTypography.caption(context)),
        ],
        SizedBox(height: AppSpacing.md),
        AmitiaProgressBar(progress: (progress / 100).clamp(0.0, 1.0)),
        SizedBox(height: AppSpacing.xs),
        Text('$progress%', style: AppTypography.caption(context)),
        if (_isProcessingTask && _qualitySummary.isNotEmpty) ...[
          SizedBox(height: AppSpacing.md),
          Wrap(spacing: 8, runSpacing: 6, children: [
            _metric(context, '总动作', _qualitySummary['totalActions']),
            _metric(context, '成功', _qualitySummary['succeededActions']),
            _metric(context, '失败', _qualitySummary['failedActions']),
            _metric(context, '警告', _qualitySummary['warningActions']),
          ]),
        ],
        if ((_task['errorMessage'] ?? '').toString().isNotEmpty) ...[
          SizedBox(height: AppSpacing.sm),
          Text(_task['errorMessage'].toString(), style: AppTypography.caption(context).copyWith(color: context.error)),
        ],
      ]),
    );
  }

  Widget _metric(BuildContext context, String label, dynamic value) {
    return Container(
      padding: const EdgeInsets.symmetric(horizontal: 8, vertical: 4),
      decoration: BoxDecoration(color: context.surfaceSecondary, borderRadius: BorderRadius.circular(6)),
      child: Text('$label ${value ?? 0}', style: AppTypography.label(context)),
    );
  }

  Widget _buildActionCard(BuildContext context, Map<String, dynamic> action) {
    final key = (action['actionKey'] ?? action['id'] ?? '').toString();
    final name = (action['actionName'] ?? action['actionNameSnapshot'] ?? key).toString();
    final status = (action['status'] ?? 'unknown').toString();
    final progress = (action['progress'] as num?)?.toInt() ?? 0;
    final excluded = action['excluded'] == true || action['excluded'] == 1;
    final quality = (action['qualityLevel'] ?? '').toString();
    return Padding(
      padding: EdgeInsets.only(bottom: AppSpacing.sm),
      child: AmitiaCard(
        child: Column(crossAxisAlignment: CrossAxisAlignment.start, children: [
          Row(children: [
            Expanded(child: Text(name, style: AppTypography.body(context))),
            if (excluded)
              const AmitiaStatusBadge(label: '已排除', type: BadgeType.neutral)
            else
              AmitiaStatusBadge(label: _statusText(status), type: _statusType(status)),
          ]),
          SizedBox(height: AppSpacing.xs),
          Text(key, style: AppTypography.label(context).copyWith(color: context.textSecondary)),
          if (quality.isNotEmpty) Text('质量：$quality', style: AppTypography.caption(context)),
          SizedBox(height: AppSpacing.sm),
          AmitiaProgressBar(progress: (progress / 100).clamp(0.0, 1.0)),
          if (_isProcessingTask && !excluded) ...[
            SizedBox(height: AppSpacing.sm),
            Wrap(spacing: 8, runSpacing: 8, children: [
              OutlinedButton.icon(
                onPressed: () => context.push(AppRoutes.petActionEditor(widget.taskId, key)),
                icon: const Icon(Icons.edit_outlined, size: 16),
                label: const Text('编辑'),
              ),
              OutlinedButton.icon(
                onPressed: () => _retryAction(key),
                icon: const Icon(Icons.refresh, size: 16),
                label: const Text('重处理'),
              ),
              OutlinedButton.icon(
                onPressed: () => _excludeAction(key),
                icon: const Icon(Icons.block, size: 16),
                label: const Text('排除'),
              ),
            ]),
          ],
        ]),
      ),
    );
  }

  Widget _buildTaskActions(BuildContext context) {
    if (!_isProcessingTask) {
      final status = (_task['status'] ?? '').toString();
      final complete = status == 'completed' || status == 'succeeded';
      return AmitiaButton(
        label: complete ? '创建处理任务' : '生成任务尚未完成',
        icon: Icons.auto_fix_high,
        isFullWidth: true,
        onPressed: complete ? _createProcessingTask : null,
      );
    }
    final status = (_task['status'] ?? '').toString();
    final done = status == 'completed' || status == 'succeeded';
    return Column(children: [
      AmitiaButton(
        label: '生成可安装资源包',
        icon: Icons.archive_outlined,
        isFullWidth: true,
        onPressed: done ? _createPackage : null,
      ),
      SizedBox(height: AppSpacing.sm),
      AmitiaButton(
        label: '查看已安装桌宠',
        isSecondary: true,
        icon: Icons.install_desktop,
        isFullWidth: true,
        onPressed: () => context.push(AppRoutes.workshopPetInstallations),
      ),
      if (!done && !{'failed', 'cancelled'}.contains(status)) ...[
        SizedBox(height: AppSpacing.sm),
        AmitiaButton(
          label: '取消处理任务',
          isSecondary: true,
          isFullWidth: true,
          onPressed: _cancelProcessing,
        ),
      ],
    ]);
  }

  Future<void> _createProcessingTask() async {
    try {
      final form = FormData.fromMap({
        'outputWidth': (_task['outputWidth'] ?? 512).toString(),
        'outputHeight': (_task['outputHeight'] ?? 512).toString(),
        'targetCharacterHeightRatio': '0.82',
        'anchorMode': 'bottom_center',
        'backgroundMode': 'transparent',
        'outputFormat': 'png',
        'defaultFps': '12',
      });
      final response = await ref.read(backendServiceProvider).post<dynamic>(
        '/api/desktop-pets/generation-tasks/${widget.taskId}/process',
        data: form,
      );
      final id = response is Map ? (response['id'] ?? '').toString() : '';
      if (!mounted) return;
      if (id.isNotEmpty) context.replace(AppRoutes.petProcessing(id));
      await _load();
    } catch (e) {
      if (mounted) ScaffoldMessenger.of(context).showSnackBar(SnackBar(content: Text('创建处理任务失败：$e')));
    }
  }

  Future<void> _retryAction(String key) async {
    try {
      await ref.read(backendServiceProvider).post<dynamic>('/api/desktop-pets/processing-tasks/${widget.taskId}/actions/$key/retry');
      await _load();
    } catch (e) {
      if (mounted) ScaffoldMessenger.of(context).showSnackBar(SnackBar(content: Text('重处理失败：$e')));
    }
  }

  Future<void> _excludeAction(String key) async {
    try {
      await ref.read(backendServiceProvider).post<dynamic>('/api/desktop-pets/processing-tasks/${widget.taskId}/actions/$key/exclude');
      await _load();
    } catch (e) {
      if (mounted) ScaffoldMessenger.of(context).showSnackBar(SnackBar(content: Text('排除失败：$e')));
    }
  }

  Future<void> _cancelProcessing() async {
    try {
      await ref.read(backendServiceProvider).post<dynamic>('/api/desktop-pets/processing-tasks/${widget.taskId}/cancel');
      await _load();
    } catch (e) {
      if (mounted) ScaffoldMessenger.of(context).showSnackBar(SnackBar(content: Text('取消失败：$e')));
    }
  }

  Future<void> _createPackage() async {
    final included = _actions
        .where((a) => !(a['excluded'] == true || a['excluded'] == 1))
        .map((a) => (a['actionKey'] ?? '').toString())
        .where((e) => e.isNotEmpty)
        .toList();
    if (included.isEmpty) return;
    try {
      final response = await ref.read(backendServiceProvider).post<dynamic>(
        '/api/desktop-pets/processing-tasks/${widget.taskId}/package',
        data: {
          'defaultAction': included.first,
          'includedActions': included,
          'userDefaultAction': included.first,
        },
      );
      final packageId = response is Map ? (response['packageId'] ?? '').toString() : '';
      if (!mounted) return;
      ScaffoldMessenger.of(context).showSnackBar(SnackBar(content: Text(packageId.isEmpty ? '资源包已生成' : '资源包已生成：$packageId')));
      context.push(AppRoutes.workshopPetInstallations);
    } catch (e) {
      if (mounted) ScaffoldMessenger.of(context).showSnackBar(SnackBar(content: Text('打包失败：$e')));
    }
  }

  String _statusText(String status) {
    switch (status) {
      case 'pending': return '待处理';
      case 'queued': return '排队中';
      case 'processing':
      case 'running': return '处理中';
      case 'completed':
      case 'succeeded': return '已完成';
      case 'failed': return '失败';
      case 'cancelled': return '已取消';
      default: return status;
    }
  }

  BadgeType _statusType(String status) {
    if (status == 'completed' || status == 'succeeded') return BadgeType.success;
    if (status == 'failed' || status == 'cancelled') return BadgeType.error;
    if (status == 'processing' || status == 'running' || status == 'queued') return BadgeType.accent;
    return BadgeType.neutral;
  }
}
