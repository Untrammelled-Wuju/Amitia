import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import '../../../../app/theme/app_colors.dart';
import '../../../../app/theme/app_spacing.dart';
import '../../../../app/theme/app_radius.dart';
import '../../../../app/theme/app_typography.dart';
import '../../../../core/widgets/amitia_scaffold.dart';
import '../../../../core/widgets/amitia_button.dart';
import '../../../../core/widgets/amitia_misc.dart';
import '../../../../shared/models/models.dart';
import '../../../../shared/mock_data/mock_data.dart';

class KernelTasksPage extends ConsumerStatefulWidget {
  const KernelTasksPage({super.key});

  @override
  ConsumerState<KernelTasksPage> createState() => _KernelTasksPageState();
}

class _KernelTasksPageState extends ConsumerState<KernelTasksPage> {
  late List<KernelTask> _tasks;

  @override
  void initState() {
    super.initState();
    _tasks = List.from(MockKernel.kernelTasks);
  }

  @override
  Widget build(BuildContext context) {
    return AmitiaScaffold(
      appBar: const AmitiaAppBar(
        title: '任务运行时',
        showBackButton: true,
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
                padding: const EdgeInsets.symmetric(vertical: AppSpacing.lg),
                itemCount: _tasks.length,
                itemBuilder: (context, index) => _buildTaskCard(context, _tasks[index]),
              ),
      ),
    );
  }

  Widget _buildTaskCard(BuildContext context, KernelTask task) {
    return Padding(
      padding: const EdgeInsets.symmetric(horizontal: AppSpacing.pagePadding, vertical: AppSpacing.xs),
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
                    color: _statusColor(context, task.status).withValues(alpha: 0.1),
                    borderRadius: AppRadius.brSmall,
                  ),
                  child: Icon(Icons.task_alt, size: 22, color: _statusColor(context, task.status)),
                ),
                const SizedBox(width: 12),
                Expanded(
                  child: Column(
                    crossAxisAlignment: CrossAxisAlignment.start,
                    children: [
                      Text(task.name, style: AppTypography.cardTitle(context)),
                      const SizedBox(height: 2),
                      Text('ID: ${task.id}', style: AppTypography.label(context)),
                    ],
                  ),
                ),
                _buildStatusBadge(task.status),
              ],
            ),
            if (task.output != null) ...[
              const SizedBox(height: AppSpacing.sm),
              Container(
                width: double.infinity,
                padding: const EdgeInsets.all(AppSpacing.sm),
                decoration: BoxDecoration(
                  color: context.surfaceSecondary,
                  borderRadius: AppRadius.brSmall,
                ),
                child: Row(
                  children: [
                    Icon(Icons.text_snippet_outlined, size: 16, color: context.success),
                    const SizedBox(width: 8),
                    Expanded(child: Text(task.output!, style: AppTypography.caption(context))),
                  ],
                ),
              ),
            ],
            if (task.error != null) ...[
              const SizedBox(height: AppSpacing.sm),
              Container(
                width: double.infinity,
                padding: const EdgeInsets.all(AppSpacing.sm),
                decoration: BoxDecoration(
                  color: context.error.withValues(alpha: 0.08),
                  borderRadius: AppRadius.brSmall,
                ),
                child: Row(
                  children: [
                    Icon(Icons.error_outline, size: 16, color: context.error),
                    const SizedBox(width: 8),
                    Expanded(child: Text(task.error!, style: AppTypography.caption(context).copyWith(color: context.error))),
                  ],
                ),
              ),
            ],
            if (task.hasCheckpoint) ...[
              const SizedBox(height: AppSpacing.sm),
              Row(
                children: [
                  Icon(Icons.bookmark_outlined, size: 14, color: context.info),
                  const SizedBox(width: 4),
                  Text('已有检查点', style: AppTypography.label(context).copyWith(color: context.info)),
                ],
              ),
            ],
            const SizedBox(height: AppSpacing.md),
            Row(
              children: _buildActionButtons(context, task),
            ),
          ],
        ),
      ),
    );
  }

  List<Widget> _buildActionButtons(BuildContext context, KernelTask task) {
    final buttons = <Widget>[];

    if (task.status == '运行中') {
      buttons.add(Expanded(
        child: AmitiaButton(
          label: '取消',
          isSecondary: true,
          icon: Icons.cancel_outlined,
          onPressed: () => _showCancelConfirm(context, task),
        ),
      ));
    } else if (task.status == '已暂停') {
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

    buttons.add(const SizedBox(width: AppSpacing.sm));
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

  void _showTaskDetailSheet(BuildContext context, KernelTask task) {
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
                _buildDetailRow(context, '任务名称', task.name),
                _buildDetailRow(context, '任务 ID', task.id),
                _buildDetailRow(context, '状态', task.status),
                if (task.output != null)
                  _buildDetailRow(context, '输出', task.output!),
                if (task.error != null)
                  _buildDetailRow(context, '错误', task.error!),
                _buildDetailRow(context, '检查点', task.hasCheckpoint ? '有' : '无'),
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

  void _showExecuteConfirm(BuildContext context, KernelTask task) {
    showDialog(
      context: context,
      builder: (context) {
        return AlertDialog(
          backgroundColor: context.surfacePrimary,
          shape: RoundedRectangleBorder(borderRadius: AppRadius.brMedium),
          title: Text('手动执行', style: AppTypography.cardTitle(context)),
          content: Text('确定要手动执行任务「${task.name}」吗？', style: AppTypography.body(context)),
          actions: [
            TextButton(
              onPressed: () => Navigator.pop(context),
              child: Text('取消', style: TextStyle(color: context.textSecondary)),
            ),
            TextButton(
              onPressed: () {
                setState(() {
                  final idx = _tasks.indexWhere((t) => t.id == task.id);
                  if (idx >= 0) {
                    _tasks[idx] = KernelTask(
                      id: task.id,
                      name: task.name,
                      status: '运行中',
                      output: '处理中...',
                      hasCheckpoint: task.hasCheckpoint,
                    );
                  }
                });
                Navigator.pop(context);
                ScaffoldMessenger.of(context).showSnackBar(SnackBar(content: Text('已开始执行：${task.name}')));
              },
              child: Text('执行', style: TextStyle(color: context.accentPrimary)),
            ),
          ],
        );
      },
    );
  }

  void _showCancelConfirm(BuildContext context, KernelTask task) {
    showDialog(
      context: context,
      builder: (context) {
        return AlertDialog(
          backgroundColor: context.surfacePrimary,
          shape: RoundedRectangleBorder(borderRadius: AppRadius.brMedium),
          title: Text('取消任务', style: AppTypography.cardTitle(context)),
          content: Text('确定要取消任务「${task.name}」吗？取消后任务将进入暂停状态。', style: AppTypography.body(context)),
          actions: [
            TextButton(
              onPressed: () => Navigator.pop(context),
              child: Text('返回', style: TextStyle(color: context.textSecondary)),
            ),
            TextButton(
              onPressed: () {
                setState(() {
                  final idx = _tasks.indexWhere((t) => t.id == task.id);
                  if (idx >= 0) {
                    _tasks[idx] = KernelTask(
                      id: task.id,
                      name: task.name,
                      status: '已暂停',
                      hasCheckpoint: true,
                    );
                  }
                });
                Navigator.pop(context);
                ScaffoldMessenger.of(context).showSnackBar(SnackBar(content: Text('已取消任务：${task.name}')));
              },
              child: Text('确认取消', style: TextStyle(color: context.warning)),
            ),
          ],
        );
      },
    );
  }

  void _resumeTask(BuildContext context, KernelTask task) {
    setState(() {
      final idx = _tasks.indexWhere((t) => t.id == task.id);
      if (idx >= 0) {
        _tasks[idx] = KernelTask(
          id: task.id,
          name: task.name,
          status: '运行中',
          output: '处理中...',
          hasCheckpoint: task.hasCheckpoint,
        );
      }
    });
    ScaffoldMessenger.of(context).showSnackBar(SnackBar(content: Text('已恢复任务：${task.name}')));
  }
}
