import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import '../../../../app/app_routes.dart';
import '../../../../app/theme/app_colors.dart';
import '../../../../app/theme/app_spacing.dart';
import '../../../../app/theme/app_radius.dart';
import '../../../../app/theme/app_typography.dart';
import '../../../../core/widgets/amitia_scaffold.dart';
import '../../../../core/widgets/amitia_button.dart';
import '../../../../core/widgets/amitia_misc.dart';
import '../../../../core/services/providers.dart';

class KernelTasksPage extends ConsumerStatefulWidget {
  const KernelTasksPage({super.key});

  @override
  ConsumerState<KernelTasksPage> createState() => _KernelTasksPageState();
}

class _KernelTasksPageState extends ConsumerState<KernelTasksPage> {
  bool _loading = true;
  String? _error;
  List<Map<String, dynamic>> _tasks = [];

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
      final svc = ref.read(systemServiceProvider);
      final nodes = await svc.graphNodes();
      if (mounted) {
        final tasks = nodes.map((n) {
          return {
            'id': n['id'] ?? '',
            'name': n['label'] ?? n['name'] ?? '任务',
            'status': n['status'] ?? '已完成',
            'output': n['output'],
            'error': n['error'],
            'hasCheckpoint': n['has_checkpoint'] ?? false,
          };
        }).toList();
        setState(() {
          _tasks = tasks;
          _loading = false;
        });
      }
    } catch (e) {
      if (mounted) {
        setState(() {
          _error = e.toString();
          _loading = false;
        });
      }
    }
  }

  @override
  Widget build(BuildContext context) {
    if (_loading) return const AmitiaLoadingState();
    if (_error != null) return AmitiaErrorState(message: _error!, onRetry: _load);

    return AmitiaScaffold(
      appBar: const AmitiaAppBar(
        title: '任务运行时',
        showBackButton: true,
        fallbackRoute: AppRoutes.developer,
      ),
      body: SafeArea(
        top: false,
        child: _tasks.isEmpty
            ? AmitiaEmptyState(
                icon: Icons.play_circle_outline,
                title: '暂无任务',
                subtitle: '任务将在内核运行时自动创建',
              )
            : ListView.builder(
                padding: EdgeInsets.symmetric(vertical: AppSpacing.lg),
                itemCount: _tasks.length,
                itemBuilder: (context, index) => _buildTaskCard(context, _tasks[index]),
              ),
      ),
    );
  }

  Widget _buildTaskCard(BuildContext context, Map<String, dynamic> task) {
    final name = task['name'] as String? ?? '任务';
    final id = task['id'] as String? ?? '';
    final status = task['status'] as String? ?? '';
    final output = task['output'] as String?;
    final error = task['error'] as String?;
    final hasCheckpoint = task['hasCheckpoint'] as bool? ?? false;

    return Padding(
      padding: EdgeInsets.symmetric(horizontal: AppSpacing.pagePadding, vertical: AppSpacing.xs),
      child: AmitiaCard(
        onTap: () => _showTaskDetailSheet(context, task),
        child: Column(
          crossAxisAlignment: CrossAxisAlignment.start,
          children: [
            Row(
              children: [
                Container(
                  width: 40,
                  height: 40,
                  decoration: BoxDecoration(
                    color: _statusColor(context, status).withValues(alpha: 0.1),
                    borderRadius: AppRadius.brSmall,
                  ),
                  child: Icon(Icons.task_alt, size: 22, color: _statusColor(context, status)),
                ),
                const SizedBox(width: 12),
                Expanded(
                  child: Column(
                    crossAxisAlignment: CrossAxisAlignment.start,
                    children: [
                      Text(name, style: AppTypography.cardTitle(context)),
                      const SizedBox(height: 2),
                      Text('ID: $id', style: AppTypography.label(context)),
                    ],
                  ),
                ),
                _buildStatusBadge(status),
              ],
            ),
            if (output != null) ...[
              SizedBox(height: AppSpacing.sm),
              Container(
                width: double.infinity,
                padding: EdgeInsets.all(AppSpacing.sm),
                decoration: BoxDecoration(
                  color: context.surfaceSecondary,
                  borderRadius: AppRadius.brSmall,
                ),
                child: Row(
                  children: [
                    Icon(Icons.text_snippet_outlined, size: 16, color: context.success),
                    const SizedBox(width: 8),
                    Expanded(child: Text(output, style: AppTypography.caption(context))),
                  ],
                ),
              ),
            ],
            if (error != null) ...[
              SizedBox(height: AppSpacing.sm),
              Container(
                width: double.infinity,
                padding: EdgeInsets.all(AppSpacing.sm),
                decoration: BoxDecoration(
                  color: context.error.withValues(alpha: 0.08),
                  borderRadius: AppRadius.brSmall,
                ),
                child: Row(
                  children: [
                    Icon(Icons.error_outline, size: 16, color: context.error),
                    const SizedBox(width: 8),
                    Expanded(child: Text(error, style: AppTypography.caption(context).copyWith(color: context.error))),
                  ],
                ),
              ),
            ],
            if (hasCheckpoint) ...[
              SizedBox(height: AppSpacing.sm),
              Row(
                children: [
                  Icon(Icons.bookmark_outlined, size: 14, color: context.info),
                  const SizedBox(width: 4),
                  Text('已有检查点', style: AppTypography.label(context).copyWith(color: context.info)),
                ],
              ),
            ],
            SizedBox(height: AppSpacing.md),
            Row(
              children: _buildActionButtons(context, task),
            ),
          ],
        ),
      ),
    );
  }

  List<Widget> _buildActionButtons(BuildContext context, Map<String, dynamic> task) {
    final buttons = <Widget>[];
    final status = task['status'] as String? ?? '';

    if (status == '运行中') {
      buttons.add(Expanded(
        child: AmitiaButton(
          label: '取消',
          isSecondary: true,
          icon: Icons.cancel_outlined,
          onPressed: () => _showCancelConfirm(context, task),
        ),
      ));
    } else if (status == '已暂停') {
      buttons.add(Expanded(
        child: AmitiaButton(
          label: '恢复',
          isSecondary: true,
          icon: Icons.play_arrow,
          onPressed: () => _resumeTask(context, task),
        ),
      ));
    } else {
      buttons.add(Expanded(
        child: AmitiaButton(
          label: '手动执行',
          icon: Icons.play_arrow,
          onPressed: () => _showExecuteConfirm(context, task),
        ),
      ));
    }

    buttons.add(SizedBox(width: AppSpacing.sm));
    buttons.add(Expanded(
      child: AmitiaButton(
        label: '详情',
        isSecondary: true,
        icon: Icons.info_outline,
        onPressed: () => _showTaskDetailSheet(context, task),
      ),
    ));

    return buttons;
  }

  Color _statusColor(BuildContext context, String status) {
    switch (status) {
      case '运行中':
        return context.accentPrimary;
      case '已完成':
        return context.success;
      case '已暂停':
        return context.warning;
      case '失败':
        return context.error;
      default:
        return context.textSecondary;
    }
  }

  AmitiaStatusBadge _buildStatusBadge(String status) {
    switch (status) {
      case '运行中':
        return const AmitiaStatusBadge(label: '运行中', type: BadgeType.accent);
      case '已完成':
        return const AmitiaStatusBadge(label: '已完成', type: BadgeType.success);
      case '已暂停':
        return const AmitiaStatusBadge(label: '已暂停', type: BadgeType.warning);
      case '失败':
        return const AmitiaStatusBadge(label: '失败', type: BadgeType.error);
      default:
        return AmitiaStatusBadge(label: status, type: BadgeType.neutral);
    }
  }

  void _showTaskDetailSheet(BuildContext context, Map<String, dynamic> task) {
    final name = task['name'] as String? ?? '任务';
    final id = task['id'] as String? ?? '';
    final status = task['status'] as String? ?? '';
    final output = task['output'] as String?;
    final error = task['error'] as String?;
    final hasCheckpoint = task['hasCheckpoint'] as bool? ?? false;

    showModalBottomSheet(
      context: context,
      backgroundColor: context.surfacePrimary,
      shape: const RoundedRectangleBorder(borderRadius: BorderRadius.vertical(top: Radius.circular(20))),
      builder: (context) {
        return SafeArea(
          child: Padding(
            padding: const EdgeInsets.fromLTRB(20, 0, 20, 34),
            child: Column(
              mainAxisSize: MainAxisSize.min,
              crossAxisAlignment: CrossAxisAlignment.start,
              children: [
                const SizedBox(height: 8),
                Center(
                  child: Container(width: 40, height: 4, decoration: BoxDecoration(color: context.borderPrimary, borderRadius: BorderRadius.circular(2))),
                ),
                const SizedBox(height: 20),
                Text('任务详情', style: AppTypography.pageTitle(context)),
                const SizedBox(height: 16),
                _buildDetailRow(context, '任务名称', name),
                _buildDetailRow(context, '任务 ID', id),
                _buildDetailRow(context, '状态', status),
                if (output != null)
                  _buildDetailRow(context, '输出', output),
                if (error != null)
                  _buildDetailRow(context, '错误', error),
                _buildDetailRow(context, '检查点', hasCheckpoint ? '有' : '无'),
                const SizedBox(height: 20),
                AmitiaButton(
                  label: '关闭',
                  isFullWidth: true,
                  isSecondary: true,
                  onPressed: () => Navigator.pop(context),
                ),
              ],
            ),
          ),
        );
      },
    );
  }

  Widget _buildDetailRow(BuildContext context, String label, String value) {
    return Padding(
      padding: const EdgeInsets.only(bottom: 12),
      child: Row(
        crossAxisAlignment: CrossAxisAlignment.start,
        children: [
          SizedBox(
            width: 70,
            child: Text(label, style: AppTypography.label(context).copyWith(color: context.textTertiary)),
          ),
          Expanded(child: Text(value, style: AppTypography.body(context))),
        ],
      ),
    );
  }

  void _showExecuteConfirm(BuildContext context, Map<String, dynamic> task) {
    showDialog(
      context: context,
      builder: (context) {
        return AlertDialog(
          backgroundColor: context.surfacePrimary,
          shape: RoundedRectangleBorder(borderRadius: AppRadius.brMedium),
          title: Text('手动执行', style: AppTypography.cardTitle(context)),
          content: Text('确定要手动执行任务「${task['name']}」吗？', style: AppTypography.body(context)),
          actions: [
            TextButton(
              onPressed: () => Navigator.pop(context),
              child: Text('取消', style: TextStyle(color: context.textSecondary)),
            ),
            TextButton(
              onPressed: () {
                setState(() {
                  final idx = _tasks.indexWhere((t) => t['id'] == task['id']);
                  if (idx >= 0) {
                    _tasks[idx]['status'] = '运行中';
                    _tasks[idx]['output'] = '处理中...';
                  }
                });
                Navigator.pop(context);
                ScaffoldMessenger.of(context).showSnackBar(SnackBar(content: Text('已开始执行：${task['name']}')));
              },
              child: Text('执行', style: TextStyle(color: context.accentPrimary)),
            ),
          ],
        );
      },
    );
  }

  void _showCancelConfirm(BuildContext context, Map<String, dynamic> task) {
    showDialog(
      context: context,
      builder: (context) {
        return AlertDialog(
          backgroundColor: context.surfacePrimary,
          shape: RoundedRectangleBorder(borderRadius: AppRadius.brMedium),
          title: Text('取消任务', style: AppTypography.cardTitle(context)),
          content: Text('确定要取消任务「${task['name']}」吗？取消后任务将进入暂停状态。', style: AppTypography.body(context)),
          actions: [
            TextButton(
              onPressed: () => Navigator.pop(context),
              child: Text('返回', style: TextStyle(color: context.textSecondary)),
            ),
            TextButton(
              onPressed: () {
                setState(() {
                  final idx = _tasks.indexWhere((t) => t['id'] == task['id']);
                  if (idx >= 0) {
                    _tasks[idx]['status'] = '已暂停';
                    _tasks[idx]['hasCheckpoint'] = true;
                  }
                });
                Navigator.pop(context);
                ScaffoldMessenger.of(context).showSnackBar(SnackBar(content: Text('已取消任务：${task['name']}')));
              },
              child: Text('确认取消', style: TextStyle(color: context.warning)),
            ),
          ],
        );
      },
    );
  }

  void _resumeTask(BuildContext context, Map<String, dynamic> task) {
    setState(() {
      final idx = _tasks.indexWhere((t) => t['id'] == task['id']);
      if (idx >= 0) {
        _tasks[idx]['status'] = '运行中';
        _tasks[idx]['output'] = '处理中...';
      }
    });
    ScaffoldMessenger.of(context).showSnackBar(SnackBar(content: Text('已恢复任务：${task['name']}')));
  }
}
