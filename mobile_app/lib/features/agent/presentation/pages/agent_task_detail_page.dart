import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import '../../../../app/app_routes.dart';
import '../../../../app/theme/app_colors.dart';
import '../../../../app/theme/app_typography.dart';
import '../../../../app/theme/app_spacing.dart';
import '../../../../app/theme/app_radius.dart';
import '../../../../core/widgets/amitia_scaffold.dart';
import '../../../../core/widgets/amitia_misc.dart';
import '../../presentation/providers/agent_tasks_provider.dart';

class AgentTaskDetailPage extends ConsumerWidget {
  final String taskId;

  const AgentTaskDetailPage({super.key, required this.taskId});

  String _statusLabel(AgentTaskStatus s) {
    switch (s) {
      case AgentTaskStatus.pending: return '待开始';
      case AgentTaskStatus.waitingApproval: return '待审批';
      case AgentTaskStatus.running: return '运行中';
      case AgentTaskStatus.paused: return '已暂停';
      case AgentTaskStatus.completed: return '已完成';
      case AgentTaskStatus.failed: return '已失败';
      case AgentTaskStatus.cancelled: return '已取消';
    }
  }

  BadgeType _badgeType(AgentTaskStatus s) {
    switch (s) {
      case AgentTaskStatus.pending: return BadgeType.neutral;
      case AgentTaskStatus.waitingApproval: return BadgeType.warning;
      case AgentTaskStatus.running: return BadgeType.accent;
      case AgentTaskStatus.paused: return BadgeType.neutral;
      case AgentTaskStatus.completed: return BadgeType.success;
      case AgentTaskStatus.failed: return BadgeType.error;
      case AgentTaskStatus.cancelled: return BadgeType.neutral;
    }
  }

  @override
  Widget build(BuildContext context, WidgetRef ref) {
    final taskAsync = ref.watch(agentTaskDetailProvider(taskId));

    return AmitiaScaffold(
      appBar: AmitiaAppBar(
        title: '任务详情',
        showBackButton: true,
        fallbackRoute: AppRoutes.agent,
      ),
      body: taskAsync.when(
        loading: () => const Center(child: CircularProgressIndicator()),
        error: (err, _) => Center(child: Text('加载失败: $err', style: AppTypography.bodySmall(context))),
        data: (task) {
          if (task == null) {
            return _buildNotFound(context);
          }
          return SingleChildScrollView(
            padding: EdgeInsets.fromLTRB(
              AppSpacing.pagePadding,
              AppSpacing.sm,
              AppSpacing.pagePadding,
              AppSpacing.xxxl,
            ),
            child: Column(
              crossAxisAlignment: CrossAxisAlignment.start,
              children: [
                _buildStatusCard(context, task, ref),
                SizedBox(height: AppSpacing.sectionGap),
                _buildExecutionPlan(context, task),
                SizedBox(height: AppSpacing.sectionGap),
                _buildToolCalls(context),
                SizedBox(height: AppSpacing.xxxl),
                _buildBottomActions(context, task, ref),
              ],
            ),
          );
        },
      ),
    );
  }

  Widget _buildNotFound(BuildContext context) {
    return Center(
      child: Column(
        mainAxisAlignment: MainAxisAlignment.center,
        children: [
          Icon(Icons.search_off, size: 56, color: context.textTertiary),
          SizedBox(height: AppSpacing.md),
          Text('任务不存在', style: AppTypography.cardTitle(context)),
          SizedBox(height: AppSpacing.lg),
          AmitiaButton(
            label: '返回任务列表',
            icon: Icons.arrow_back,
            onPressed: () => Navigator.of(context).maybePop(),
          ),
        ],
      ),
    );
  }

  Widget _buildStatusCard(BuildContext context, AgentTaskItem task, WidgetRef ref) {
    return AmitiaCard(
      child: Column(
        crossAxisAlignment: CrossAxisAlignment.start,
        children: [
          Row(
            children: [
              Text('任务状态', style: AppTypography.sectionTitle(context)),
              const Spacer(),
              AmitiaStatusBadge(
                label: _statusLabel(task.status),
                type: _badgeType(task.status),
              ),
            ],
          ),
          SizedBox(height: AppSpacing.sm),
          Text(task.description, style: AppTypography.caption(context)),
          SizedBox(height: AppSpacing.md),
          Row(
            mainAxisAlignment: MainAxisAlignment.spaceBetween,
            children: [
              Text('执行进度', style: AppTypography.caption(context)),
              Text(
                '${task.progress}%',
                style: AppTypography.caption(context).copyWith(
                  color: context.accentPrimary,
                  fontWeight: FontWeight.w600,
                ),
              ),
            ],
          ),
          SizedBox(height: AppSpacing.xs),
          AmitiaProgressBar(progress: task.progress / 100),
          SizedBox(height: AppSpacing.md),
          Row(
            children: [
              Icon(Icons.timer_outlined, size: 16, color: context.textTertiary),
              const SizedBox(width: 6),
              Text('已运行 ${task.elapsed}', style: AppTypography.label(context)),
              const SizedBox(width: 16),
              Icon(Icons.checklist, size: 16, color: context.textTertiary),
              const SizedBox(width: 6),
              Text('步骤 ${task.currentStepIndex + 1}/${task.steps.length}', style: AppTypography.label(context)),
            ],
          ),
          SizedBox(height: AppSpacing.sm),
          Wrap(
            spacing: 6,
            runSpacing: 4,
            children: task.requiredAbilities.map((a) {
              return Container(
                padding: const EdgeInsets.symmetric(horizontal: 8, vertical: 3),
                decoration: BoxDecoration(
                  color: context.accentSoft,
                  borderRadius: AppRadius.brTag,
                ),
                child: Text(a, style: AppTypography.label(context).copyWith(color: context.accentPrimary)),
              );
            }).toList(),
          ),
          if (task.result != null) ...[
            SizedBox(height: AppSpacing.md),
            Container(
              padding: EdgeInsets.all(AppSpacing.md),
              decoration: BoxDecoration(
                color: context.success.withValues(alpha: 0.08),
                borderRadius: AppRadius.brSmall,
              ),
              child: Row(
                crossAxisAlignment: CrossAxisAlignment.start,
                children: [
                  Icon(Icons.check_circle, size: 18, color: context.success),
                  const SizedBox(width: 8),
                  Expanded(
                    child: Text(task.result!, style: AppTypography.bodySmall(context).copyWith(color: context.success)),
                  ),
                ],
              ),
            ),
          ],
          if (task.error != null) ...[
            SizedBox(height: AppSpacing.md),
            Container(
              padding: EdgeInsets.all(AppSpacing.md),
              decoration: BoxDecoration(
                color: context.error.withValues(alpha: 0.08),
                borderRadius: AppRadius.brSmall,
              ),
              child: Row(
                crossAxisAlignment: CrossAxisAlignment.start,
                children: [
                  Icon(Icons.error_outline, size: 18, color: context.error),
                  const SizedBox(width: 8),
                  Expanded(
                    child: Text(task.error!, style: AppTypography.bodySmall(context).copyWith(color: context.error)),
                  ),
                ],
              ),
            ),
          ],
        ],
      ),
    );
  }

  Widget _buildExecutionPlan(BuildContext context, AgentTaskItem task) {
    return Column(
      crossAxisAlignment: CrossAxisAlignment.start,
      children: [
        Text('执行计划', style: AppTypography.sectionTitle(context)),
        SizedBox(height: AppSpacing.md),
        Container(
          padding: EdgeInsets.all(AppSpacing.cardPadding),
          decoration: BoxDecoration(
            color: context.surfacePrimary,
            borderRadius: AppRadius.brMedium,
            border: Border.all(color: context.borderPrimary, width: 0.5),
          ),
          child: Column(
            children: List.generate(task.steps.length, (index) {
              final stepName = task.steps[index];
              final isLast = index == task.steps.length - 1;
              final isCompleted = task.status == AgentTaskStatus.completed || index < task.currentStepIndex;
              final isRunning = task.status == AgentTaskStatus.running && index == task.currentStepIndex;
              final String stepStatus;
              if (isCompleted) {
                stepStatus = '已完成';
              } else if (isRunning) {
                stepStatus = '执行中';
              } else {
                stepStatus = '等待中';
              }
              return _TimelineStep(name: stepName, status: stepStatus, isLast: isLast);
            }),
          ),
        ),
      ],
    );
  }

  Widget _buildToolCalls(BuildContext context) {
    return Column(
      crossAxisAlignment: CrossAxisAlignment.start,
      children: [
        Text('工具调用记录', style: AppTypography.sectionTitle(context)),
        SizedBox(height: AppSpacing.md),
        Text('暂无工具调用记录', style: AppTypography.caption(context)),
      ],
    );
  }

  Widget _buildBottomActions(BuildContext context, AgentTaskItem task, WidgetRef ref) {
    void changeStatus(AgentTaskStatus newStatus) {
      ref.read(agentTasksProvider.notifier).changeStatus(task.id, newStatus);
    }

    switch (task.status) {
      case AgentTaskStatus.pending:
        return Row(
          children: [
            Expanded(
              child: AmitiaButton(
                label: '开始',
                icon: Icons.play_arrow,
                onPressed: () {
                  changeStatus(AgentTaskStatus.running);
                  amitiaSnackBar(context, '任务已开始执行');
                },
              ),
            ),
            SizedBox(width: AppSpacing.sm),
            Expanded(
              child: AmitiaButton(
                label: '取消',
                isSecondary: true,
                isDestructive: true,
                icon: Icons.cancel_outlined,
                onPressed: () => _confirmDestructive(
                  context,
                  title: '取消任务',
                  message: '确定要取消此任务吗？取消后需重新创建。',
                  confirmLabel: '取消任务',
                  onConfirm: () {
                    changeStatus(AgentTaskStatus.cancelled);
                    amitiaSnackBar(context, '任务已取消');
                  },
                ),
              ),
            ),
          ],
        );
      case AgentTaskStatus.waitingApproval:
        return Row(
          children: [
            Expanded(
              child: AmitiaButton(
                label: '允许',
                icon: Icons.check,
                onPressed: () {
                  changeStatus(AgentTaskStatus.running);
                  amitiaSnackBar(context, '已批准，任务开始执行');
                },
              ),
            ),
            SizedBox(width: AppSpacing.sm),
            Expanded(
              child: AmitiaButton(
                label: '拒绝',
                isSecondary: true,
                isDestructive: true,
                icon: Icons.close,
                onPressed: () {
                  changeStatus(AgentTaskStatus.cancelled);
                  amitiaSnackBar(context, '已拒绝，任务已取消');
                },
              ),
            ),
          ],
        );
      case AgentTaskStatus.running:
        return Row(
          children: [
            Expanded(
              child: AmitiaButton(
                label: '暂停',
                isSecondary: true,
                icon: Icons.pause,
                onPressed: () => _confirmDestructive(
                  context,
                  title: '暂停任务',
                  message: '确定要暂停此任务吗？',
                  confirmLabel: '暂停',
                  onConfirm: () {
                    changeStatus(AgentTaskStatus.paused);
                    amitiaSnackBar(context, '任务已暂停');
                  },
                ),
              ),
            ),
            SizedBox(width: AppSpacing.sm),
            Expanded(
              child: AmitiaButton(
                label: '停止',
                isDestructive: true,
                icon: Icons.stop,
                onPressed: () => _confirmDestructive(
                  context,
                  title: '停止任务',
                  message: '确定要停止此任务吗？此操作不可撤销。',
                  confirmLabel: '停止',
                  onConfirm: () {
                    changeStatus(AgentTaskStatus.cancelled);
                    amitiaSnackBar(context, '任务已停止');
                  },
                ),
              ),
            ),
          ],
        );
      case AgentTaskStatus.paused:
        return Row(
          children: [
            Expanded(
              child: AmitiaButton(
                label: '继续',
                icon: Icons.play_arrow,
                onPressed: () {
                  changeStatus(AgentTaskStatus.running);
                  amitiaSnackBar(context, '任务已继续执行');
                },
              ),
            ),
            SizedBox(width: AppSpacing.sm),
            Expanded(
              child: AmitiaButton(
                label: '停止',
                isDestructive: true,
                icon: Icons.stop,
                onPressed: () => _confirmDestructive(
                  context,
                  title: '停止任务',
                  message: '确定要停止此任务吗？此操作不可撤销。',
                  confirmLabel: '停止',
                  onConfirm: () {
                    changeStatus(AgentTaskStatus.cancelled);
                    amitiaSnackBar(context, '任务已停止');
                  },
                ),
              ),
            ),
          ],
        );
      case AgentTaskStatus.completed:
        return Row(
          children: [
            Expanded(
              child: AmitiaButton(
                label: '查看结果',
                isSecondary: true,
                icon: Icons.description_outlined,
                onPressed: () {
                  amitiaSnackBar(context, task.result ?? '任务已完成');
                },
              ),
            ),
            SizedBox(width: AppSpacing.sm),
            Expanded(
              child: AmitiaButton(
                label: '再次执行',
                icon: Icons.refresh,
                onPressed: () {
                  changeStatus(AgentTaskStatus.running);
                  amitiaSnackBar(context, '任务已重新开始');
                },
              ),
            ),
          ],
        );
      case AgentTaskStatus.failed:
        return Row(
          children: [
            Expanded(
              child: AmitiaButton(
                label: '查看错误',
                isSecondary: true,
                icon: Icons.error_outline,
                onPressed: () {
                  amitiaSnackBar(context, task.error ?? '任务执行失败');
                },
              ),
            ),
            SizedBox(width: AppSpacing.sm),
            Expanded(
              child: AmitiaButton(
                label: '重试',
                icon: Icons.refresh,
                onPressed: () {
                  changeStatus(AgentTaskStatus.running);
                  amitiaSnackBar(context, '任务已重新开始');
                },
              ),
            ),
          ],
        );
      case AgentTaskStatus.cancelled:
        return AmitiaButton(
          label: '再次执行',
          icon: Icons.refresh,
          isFullWidth: true,
          onPressed: () {
            changeStatus(AgentTaskStatus.running);
            amitiaSnackBar(context, '任务已重新开始');
          },
        );
    }
  }

  void _confirmDestructive(
    BuildContext context, {
    required String title,
    required String message,
    required String confirmLabel,
    required VoidCallback onConfirm,
  }) {
    showAmitiaConfirmDialog(
      context,
      title: title,
      message: message,
      confirmLabel: confirmLabel,
      isDestructive: true,
    ).then((confirmed) {
      if (confirmed == true) onConfirm();
    });
  }
}

class _TimelineStep extends StatelessWidget {
  final String name;
  final String status;
  final bool isLast;

  const _TimelineStep({required this.name, required this.status, required this.isLast});

  Color _statusColor(BuildContext context) {
    switch (status) {
      case '已完成':
        return context.success;
      case '执行中':
        return context.accentPrimary;
      case '等待中':
        return context.textTertiary;
      default:
        return context.textTertiary;
    }
  }

  @override
  Widget build(BuildContext context) {
    final color = _statusColor(context);
    final isCompleted = status == '已完成';
    final isRunning = status == '执行中';

    return IntrinsicHeight(
      child: Row(
        crossAxisAlignment: CrossAxisAlignment.start,
        children: [
          SizedBox(
            width: 24,
            child: Column(
              children: [
                Container(
                  width: 20,
                  height: 20,
                  decoration: BoxDecoration(
                    color: isCompleted ? color : color.withValues(alpha: 0.15),
                    shape: BoxShape.circle,
                    border: isRunning ? Border.all(color: color, width: 2) : null,
                  ),
                  child: isCompleted
                      ? const Icon(Icons.check, size: 12, color: Colors.white)
                      : isRunning
                          ? Center(
                              child: Container(
                                width: 8,
                                height: 8,
                                decoration: BoxDecoration(
                                  color: color,
                                  shape: BoxShape.circle,
                                ),
                              ),
                            )
                          : null,
                ),
                if (!isLast)
                  Expanded(
                    child: Container(
                      width: 1.5,
                      color: context.borderPrimary,
                    ),
                  ),
              ],
            ),
          ),
          const SizedBox(width: 12),
          Expanded(
            child: Padding(
              padding: EdgeInsets.only(bottom: isLast ? 0 : AppSpacing.md),
              child: Column(
                crossAxisAlignment: CrossAxisAlignment.start,
                children: [
                  Text(
                    name,
                    style: AppTypography.bodySmall(context).copyWith(
                      fontWeight: FontWeight.w500,
                      color: status == '等待中'
                          ? context.textTertiary
                          : context.textPrimary,
                    ),
                  ),
                  const SizedBox(height: 2),
                  Text(status, style: AppTypography.label(context).copyWith(color: color)),
                ],
              ),
            ),
          ),
        ],
      ),
    );
  }
}
