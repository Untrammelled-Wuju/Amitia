import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:go_router/go_router.dart';
import '../../../../app/app_routes.dart';
import '../../../../app/theme/app_colors.dart';
import '../../../../app/theme/app_typography.dart';
import '../../../../app/theme/app_spacing.dart';
import '../../../../app/theme/app_radius.dart';
import '../../../../core/widgets/amitia_scaffold.dart';
import '../../../../core/widgets/amitia_button.dart';
import '../../../../core/widgets/amitia_misc.dart';
import '../../../../core/widgets/amitia_message.dart';
import '../../../../shared/models/models.dart';
import '../../../../shared/mock_data/mock_data.dart';

class AgentPage extends ConsumerStatefulWidget {
  const AgentPage({super.key});

  @override
  ConsumerState<AgentPage> createState() => _AgentPageState();
}

class _AgentPageState extends ConsumerState<AgentPage> {
  int _selectedSegment = 0;

  @override
  Widget build(BuildContext context) {
    return AmitiaScaffold(
      appBar: AmitiaAppBar(
        title: 'Agent',
        centerTitle: true,
      ),
      body: Column(
        children: [
          Padding(
            padding: const EdgeInsets.fromLTRB(
              AppSpacing.pagePadding,
              AppSpacing.sm,
              AppSpacing.pagePadding,
              AppSpacing.md,
            ),
            child: AmitiaSegmentedControl(
              segments: const ['进行中', '等待审批', '已完成'],
              selectedIndex: _selectedSegment,
              onChanged: (i) => setState(() => _selectedSegment = i),
            ),
          ),
          Expanded(
            child: _buildList(),
          ),
          _buildQuickTasks(),
        ],
      ),
    );
  }

  Widget _buildList() {
    switch (_selectedSegment) {
      case 0:
        return _buildRunningList();
      case 1:
        return _buildPendingList();
      case 2:
        return _buildCompletedList();
      default:
        return const SizedBox();
    }
  }

  Widget _buildRunningList() {
    final tasks = MockData.agentTasksRunning;
    if (tasks.isEmpty) {
      return const AmitiaEmptyState(icon: Icons.auto_awesome, title: '暂无进行中的任务');
    }
    return ListView.builder(
      padding: const EdgeInsets.symmetric(horizontal: AppSpacing.pagePadding),
      itemCount: tasks.length,
      itemBuilder: (context, index) {
        final task = tasks[index];
        return _RunningTaskCard(
          task: task,
          onTap: () => context.push(AppRoutes.agentTask(task.id)),
        );
      },
    );
  }

  Widget _buildPendingList() {
    final tasks = MockData.agentTasksPending;
    if (tasks.isEmpty) {
      return const AmitiaEmptyState(icon: Icons.pending_actions, title: '暂无等待审批的任务');
    }
    return ListView.builder(
      padding: const EdgeInsets.symmetric(horizontal: AppSpacing.pagePadding),
      itemCount: tasks.length,
      itemBuilder: (context, index) {
        final task = tasks[index];
        return _PendingTaskCard(
          task: task,
          onTap: () => context.push(AppRoutes.agentTask(task.id)),
          onApprove: () => _showPermissionSheet(task),
        );
      },
    );
  }

  Widget _buildCompletedList() {
    final tasks = MockData.agentTasksCompleted;
    if (tasks.isEmpty) {
      return const AmitiaEmptyState(icon: Icons.task_alt, title: '暂无已完成的任务');
    }
    return ListView.builder(
      padding: const EdgeInsets.symmetric(horizontal: AppSpacing.pagePadding),
      itemCount: tasks.length,
      itemBuilder: (context, index) {
        final task = tasks[index];
        return _CompletedTaskCard(
          task: task,
          onTap: () => context.push(AppRoutes.agentTask(task.id)),
        );
      },
    );
  }

  Widget _buildQuickTasks() {
    return Container(
      padding: const EdgeInsets.fromLTRB(
        AppSpacing.pagePadding,
        AppSpacing.md,
        AppSpacing.pagePadding,
        AppSpacing.lg,
      ),
      decoration: BoxDecoration(
        color: context.surfacePrimary,
        border: Border(top: BorderSide(color: context.borderSecondary, width: 1)),
      ),
      child: SafeArea(
        top: false,
        child: Column(
          crossAxisAlignment: CrossAxisAlignment.start,
          mainAxisSize: MainAxisSize.min,
          children: [
            Text('快捷任务', style: AppTypography.sectionTitle(context)),
            const SizedBox(height: AppSpacing.md),
            GridView.builder(
              shrinkWrap: true,
              physics: const NeverScrollableScrollPhysics(),
              gridDelegate: const SliverGridDelegateWithFixedCrossAxisCount(
                crossAxisCount: 2,
                mainAxisSpacing: AppSpacing.sm,
                crossAxisSpacing: AppSpacing.sm,
                childAspectRatio: 2.8,
              ),
              itemCount: MockData.quickTasks.length,
              itemBuilder: (context, index) {
                final task = MockData.quickTasks[index];
                return _QuickTaskItem(task: task);
              },
            ),
          ],
        ),
      ),
    );
  }

  void _showPermissionSheet(AgentTask task) {
    showModalBottomSheet(
      context: context,
      backgroundColor: context.surfacePrimary,
      shape: const RoundedRectangleBorder(
        borderRadius: BorderRadius.vertical(top: Radius.circular(AppRadius.large)),
      ),
      builder: (sheetContext) {
        return SingleChildScrollView(
          child: AmitiaPermissionSheet(
            taskTitle: task.title,
            permissions: task.requiredPermissions ?? [],
            onAllowOnce: () => Navigator.pop(sheetContext),
            onAllowAlways: () => Navigator.pop(sheetContext),
            onDeny: () => Navigator.pop(sheetContext),
          ),
        );
      },
    );
  }
}

class _RunningTaskCard extends StatelessWidget {
  final AgentTask task;
  final VoidCallback onTap;

  const _RunningTaskCard({required this.task, required this.onTap});

  @override
  Widget build(BuildContext context) {
    return Container(
      margin: const EdgeInsets.only(bottom: AppSpacing.sm),
      child: AmitiaCard(
        onTap: onTap,
        child: Column(
          crossAxisAlignment: CrossAxisAlignment.start,
          children: [
            Row(
              children: [
                Container(
                  width: 36,
                  height: 36,
                  decoration: BoxDecoration(
                    color: context.accentSoft,
                    borderRadius: AppRadius.brSmall,
                  ),
                  child: Icon(Icons.auto_awesome, size: 18, color: context.accentPrimary),
                ),
                const SizedBox(width: 12),
                Expanded(
                  child: Column(
                    crossAxisAlignment: CrossAxisAlignment.start,
                    children: [
                      Text(task.title, style: AppTypography.cardTitle(context)),
                      const SizedBox(height: 2),
                      Text(task.currentStep, style: AppTypography.caption(context)),
                    ],
                  ),
                ),
                const AmitiaStatusBadge(label: '运行中', type: BadgeType.accent),
              ],
            ),
            const SizedBox(height: AppSpacing.md),
            AmitiaProgressBar(progress: task.progress / 100),
            const SizedBox(height: AppSpacing.sm),
            Row(
              children: [
                Text('${task.progress}%', style: AppTypography.label(context)),
                const SizedBox(width: 12),
                Text('已运行 ${task.elapsed ?? '00:00'}', style: AppTypography.label(context)),
                const Spacer(),
                GestureDetector(
                  onTap: () => showAmitiaConfirmDialog(context, title: '暂停任务', message: '确定要暂停此任务吗？暂停后可以继续执行。', confirmLabel: '暂停').then((confirmed) { if (confirmed == true) { amitiaSnackBar(context, '任务已暂停'); } }),
                  child: Container(
                    padding: const EdgeInsets.symmetric(horizontal: 10, vertical: 4),
                    decoration: BoxDecoration(
                      color: context.surfaceSecondary,
                      borderRadius: AppRadius.brTag,
                    ),
                    child: Row(
                      mainAxisSize: MainAxisSize.min,
                      children: [
                        Icon(Icons.pause, size: 14, color: context.textSecondary),
                        const SizedBox(width: 4),
                        Text('暂停', style: TextStyle(fontSize: 12, color: context.textSecondary)),
                      ],
                    ),
                  ),
                ),
              ],
            ),
          ],
        ),
      ),
    );
  }
}

class _PendingTaskCard extends StatelessWidget {
  final AgentTask task;
  final VoidCallback onTap;
  final VoidCallback onApprove;

  const _PendingTaskCard({
    required this.task,
    required this.onTap,
    required this.onApprove,
  });

  @override
  Widget build(BuildContext context) {
    return Container(
      margin: const EdgeInsets.only(bottom: AppSpacing.sm),
      child: AmitiaCard(
        onTap: onTap,
        child: Column(
          crossAxisAlignment: CrossAxisAlignment.start,
          children: [
            Row(
              children: [
                Container(
                  width: 36,
                  height: 36,
                  decoration: BoxDecoration(
                    color: context.warning.withValues(alpha: 0.12),
                    borderRadius: AppRadius.brSmall,
                  ),
                  child: Icon(Icons.hourglass_top, size: 18, color: context.warning),
                ),
                const SizedBox(width: 12),
                Expanded(
                  child: Column(
                    crossAxisAlignment: CrossAxisAlignment.start,
                    children: [
                      Text(task.title, style: AppTypography.cardTitle(context)),
                      const SizedBox(height: 2),
                      Text(task.currentStep, style: AppTypography.caption(context)),
                    ],
                  ),
                ),
                const AmitiaStatusBadge(label: '待审批', type: BadgeType.warning),
              ],
            ),
            if (task.requiredPermissions != null && task.requiredPermissions!.isNotEmpty) ...[
              const SizedBox(height: AppSpacing.md),
              Text('需要权限', style: AppTypography.label(context)),
              const SizedBox(height: AppSpacing.xs),
              Wrap(
                spacing: 6,
                runSpacing: 6,
                children: task.requiredPermissions!.map((p) {
                  return Container(
                    padding: const EdgeInsets.symmetric(horizontal: 8, vertical: 3),
                    decoration: BoxDecoration(
                      color: context.warning.withValues(alpha: 0.08),
                      borderRadius: AppRadius.brTag,
                    ),
                    child: Text(p, style: TextStyle(fontSize: 11, color: context.warning)),
                  );
                }).toList(),
              ),
            ],
            const SizedBox(height: AppSpacing.md),
            AmitiaButton(
              label: '审批',
              isFullWidth: true,
              isSecondary: true,
              icon: Icons.shield_outlined,
              onPressed: onApprove,
            ),
          ],
        ),
      ),
    );
  }
}

class _CompletedTaskCard extends StatelessWidget {
  final AgentTask task;
  final VoidCallback onTap;

  const _CompletedTaskCard({required this.task, required this.onTap});

  @override
  Widget build(BuildContext context) {
    return Container(
      margin: const EdgeInsets.only(bottom: AppSpacing.sm),
      child: AmitiaCard(
        onTap: onTap,
        child: Column(
          crossAxisAlignment: CrossAxisAlignment.start,
          children: [
            Row(
              children: [
                Container(
                  width: 36,
                  height: 36,
                  decoration: BoxDecoration(
                    color: context.success.withValues(alpha: 0.12),
                    borderRadius: AppRadius.brSmall,
                  ),
                  child: Icon(Icons.check_circle_outline, size: 18, color: context.success),
                ),
                const SizedBox(width: 12),
                Expanded(child: Text(task.title, style: AppTypography.cardTitle(context))),
                const AmitiaStatusBadge(label: '已完成', type: BadgeType.success),
              ],
            ),
            if (task.result != null) ...[
              const SizedBox(height: AppSpacing.sm),
              Text(task.result!, style: AppTypography.caption(context)),
            ],
            const SizedBox(height: AppSpacing.xs),
            Row(
              children: [
                if (task.category != null) ...[
                  Text(task.category!, style: AppTypography.label(context)),
                  const SizedBox(width: 12),
                ],
                Text('耗时 ${task.elapsed ?? '--'}', style: AppTypography.label(context)),
              ],
            ),
          ],
        ),
      ),
    );
  }
}

class _QuickTaskItem extends StatelessWidget {
  final QuickTask task;

  const _QuickTaskItem({required this.task});

  @override
  Widget build(BuildContext context) {
    return GestureDetector(
      onTap: () => amitiaComingSoon(context, task.title),
      child: Container(
        padding: const EdgeInsets.symmetric(horizontal: 12, vertical: 10),
        decoration: BoxDecoration(
          color: context.surfaceSecondary,
          borderRadius: AppRadius.brMedium,
        ),
        child: Row(
          children: [
            Container(
              width: 32,
              height: 32,
              decoration: BoxDecoration(
                color: context.accentSoft,
                borderRadius: AppRadius.brSmall,
              ),
              child: Icon(task.icon, size: 18, color: context.accentPrimary),
            ),
            const SizedBox(width: 10),
            Expanded(
              child: Column(
                crossAxisAlignment: CrossAxisAlignment.start,
                mainAxisAlignment: MainAxisAlignment.center,
                mainAxisSize: MainAxisSize.min,
                children: [
                  Flexible(
                    child: Text(
                      task.title,
                      style: AppTypography.bodySmall(context).copyWith(
                        fontWeight: FontWeight.w500,
                      ),
                      maxLines: 1,
                      overflow: TextOverflow.ellipsis,
                    ),
                  ),
                  Text(task.category, style: AppTypography.label(context)),
                ],
              ),
            ),
          ],
        ),
      ),
    );
  }
}
