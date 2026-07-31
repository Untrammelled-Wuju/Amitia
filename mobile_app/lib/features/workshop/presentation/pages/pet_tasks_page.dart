import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:go_router/go_router.dart';
import '../../../../app/theme/app_colors.dart';
import '../../../../app/theme/app_typography.dart';
import '../../../../app/theme/app_spacing.dart';
import '../../../../app/theme/app_radius.dart';
import '../../../../core/widgets/amitia_scaffold.dart';
import '../../../../core/widgets/amitia_button.dart';
import '../../../../core/widgets/amitia_misc.dart';
import '../../../../app/app_routes.dart';
import '../../../../shared/models/models.dart';
import '../../../../shared/mock_data/mock_data.dart';

class PetTasksPage extends ConsumerStatefulWidget {
  const PetTasksPage({super.key});

  @override
  ConsumerState<PetTasksPage> createState() => _PetTasksPageState();
}

class _PetTasksPageState extends ConsumerState<PetTasksPage> {
  late List<PetTask> _tasks;

  @override
  void initState() {
    super.initState();
    _tasks = List.from(MockWorkshop.petTasks);
  }

  String _statusLabel(PetTaskStatus status) {
    switch (status) {
      case PetTaskStatus.pending:
        return '待处理';
      case PetTaskStatus.processing:
        return '处理中';
      case PetTaskStatus.completed:
        return '已完成';
      case PetTaskStatus.cancelled:
        return '已取消';
    }
  }

  BadgeType _statusBadgeType(PetTaskStatus status) {
    switch (status) {
      case PetTaskStatus.pending:
        return BadgeType.neutral;
      case PetTaskStatus.processing:
        return BadgeType.accent;
      case PetTaskStatus.completed:
        return BadgeType.success;
      case PetTaskStatus.cancelled:
        return BadgeType.error;
    }
  }

  String _formatDate(DateTime date) {
    return '${date.month}/${date.day}';
  }

  @override
  Widget build(BuildContext context) {
    return AmitiaScaffold(
      appBar: AmitiaAppBar(
        title: '任务列表',
        showBackButton: true,
        actions: [
          Padding(
            padding: const EdgeInsets.only(right: AppSpacing.sm),
            child: AmitiaIconButton(
              icon: Icons.add,
              color: context.accentPrimary,
              onPressed: () => context.push(AppRoutes.workshopPetCreate),
            ),
          ),
        ],
      ),
      body: SafeArea(
        top: false,
        child: _tasks.isEmpty
            ? AmitiaEmptyState(
                icon: Icons.assignment_outlined,
                title: '暂无任务',
                subtitle: '点击右上角创建新桌宠',
                actionText: '创建桌宠',
                onAction: () => context.push(AppRoutes.workshopPetCreate),
              )
            : ListView.builder(
                padding: const EdgeInsets.all(AppSpacing.pagePadding),
                itemCount: _tasks.length,
                itemBuilder: (context, index) => _buildTaskCard(context, _tasks[index]),
              ),
      ),
    );
  }

  Widget _buildTaskCard(BuildContext context, PetTask task) {
    final canCancel = task.status == PetTaskStatus.pending || task.status == PetTaskStatus.processing;
    final canProcess = task.status == PetTaskStatus.pending || task.status == PetTaskStatus.processing;

    return Padding(
      padding: const EdgeInsets.only(bottom: AppSpacing.sm),
      child: AmitiaCard(
        onTap: () => context.push(AppRoutes.petProcessing(task.id)),
        child: Column(
          crossAxisAlignment: CrossAxisAlignment.start,
          children: [
            Row(
              children: [
                Container(
                  width: 44,
                  height: 44,
                  decoration: BoxDecoration(
                    color: context.accentSoft,
                    borderRadius: AppRadius.brSmall,
                  ),
                  child: Icon(Icons.pets_outlined, size: 22, color: context.accentPrimary),
                ),
                const SizedBox(width: AppSpacing.md),
                Expanded(
                  child: Column(
                    crossAxisAlignment: CrossAxisAlignment.start,
                    children: [
                      Text(task.name, style: AppTypography.cardTitle(context)),
                      const SizedBox(height: 2),
                      Text(
                        '${task.characterName} · ${_formatDate(task.createdAt)}',
                        style: AppTypography.caption(context),
                      ),
                    ],
                  ),
                ),
                AmitiaStatusBadge(label: _statusLabel(task.status), type: _statusBadgeType(task.status)),
              ],
            ),
            const SizedBox(height: AppSpacing.md),
            Row(
              children: [
                Text('${task.completedActions}/${task.totalActions} 动作', style: AppTypography.caption(context)),
                const SizedBox(width: AppSpacing.md),
                Expanded(child: AmitiaProgressBar(progress: task.progress / 100.0)),
                const SizedBox(width: AppSpacing.sm),
                Text('${task.progress}%', style: AppTypography.caption(context)),
              ],
            ),
            const SizedBox(height: AppSpacing.md),
            Row(
              children: [
                if (canProcess)
                  Expanded(
                    child: AmitiaButton(
                      label: '继续处理',
                      isSecondary: true,
                      height: 38,
                      icon: Icons.play_arrow,
                      onPressed: () => context.push(AppRoutes.petProcessing(task.id)),
                    ),
                  )
                else
                  Expanded(
                    child: AmitiaButton(
                      label: '查看详情',
                      isSecondary: true,
                      height: 38,
                      onPressed: () => context.push(AppRoutes.petProcessing(task.id)),
                    ),
                  ),
                const SizedBox(width: AppSpacing.sm),
                Expanded(
                  child: AmitiaButton(
                    label: '创建处理任务',
                    height: 38,
                    icon: Icons.add,
                    onPressed: () => _showCreateProcessingConfirm(task),
                  ),
                ),
                const SizedBox(width: AppSpacing.sm),
                if (canCancel)
                  SizedBox(
                    width: 38,
                    height: 38,
                    child: AmitiaIconButton(
                      icon: Icons.close,
                      color: context.error,
                      backgroundColor: context.error.withValues(alpha: 0.1),
                      onPressed: () => _showCancelConfirm(task),
                    ),
                  ),
              ],
            ),
          ],
        ),
      ),
    );
  }

  void _showCancelConfirm(PetTask task) {
    showDialog(
      context: context,
      builder: (dialogContext) {
        return AlertDialog(
          shape: RoundedRectangleBorder(borderRadius: AppRadius.brMedium),
          title: Text('取消任务', style: AppTypography.cardTitle(context)),
          content: Text(
            '确认取消任务「${task.name}」？取消后无法恢复。',
            style: AppTypography.body(context),
          ),
          actions: [
            TextButton(
              onPressed: () => Navigator.pop(dialogContext),
              child: Text('保留', style: TextStyle(color: context.textSecondary)),
            ),
            TextButton(
              onPressed: () {
                Navigator.pop(dialogContext);
                setState(() {
                  final idx = _tasks.indexWhere((t) => t.id == task.id);
                  if (idx >= 0) {
                    _tasks[idx] = PetTask(
                      id: task.id,
                      name: task.name,
                      characterName: task.characterName,
                      totalActions: task.totalActions,
                      completedActions: task.completedActions,
                      status: PetTaskStatus.cancelled,
                      progress: task.progress,
                      createdAt: task.createdAt,
                    );
                  }
                });
                ScaffoldMessenger.of(context).showSnackBar(
                  SnackBar(content: Text('任务「${task.name}」已取消')),
                );
              },
              child: Text('取消任务', style: TextStyle(color: context.error)),
            ),
          ],
        );
      },
    );
  }

  void _showCreateProcessingConfirm(PetTask task) {
    showDialog(
      context: context,
      builder: (dialogContext) {
        return AlertDialog(
          shape: RoundedRectangleBorder(borderRadius: AppRadius.brMedium),
          title: Text('创建处理任务', style: AppTypography.cardTitle(context)),
          content: Text(
            '确认为任务「${task.name}」创建处理任务？将为每个动作生成处理子任务。',
            style: AppTypography.body(context),
          ),
          actions: [
            TextButton(
              onPressed: () => Navigator.pop(dialogContext),
              child: Text('取消', style: TextStyle(color: context.textSecondary)),
            ),
            TextButton(
              onPressed: () {
                Navigator.pop(dialogContext);
                ScaffoldMessenger.of(context).showSnackBar(
                  SnackBar(content: Text('已为「${task.name}」创建处理任务')),
                );
                context.push(AppRoutes.petProcessing(task.id));
              },
              child: Text('创建', style: TextStyle(color: context.accentPrimary)),
            ),
          ],
        );
      },
    );
  }
}
