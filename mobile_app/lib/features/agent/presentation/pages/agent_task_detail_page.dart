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
import '../../../../shared/models/models.dart';
import '../../../../shared/mock_data/mock_data.dart';
import '../providers/mock_agent_tasks.dart';

class AgentTaskDetailPage extends ConsumerStatefulWidget {
  final String taskId;

  const AgentTaskDetailPage({super.key, required this.taskId});

  @override
  ConsumerState<AgentTaskDetailPage> createState() => _AgentTaskDetailPageState();
}

class _AgentTaskDetailPageState extends ConsumerState<AgentTaskDetailPage> {
  MockAgentTask? _findTask() {
    final tasks = ref.read(agentTasksProvider);
    final idx = tasks.indexWhere((t) => t.id == widget.taskId);
    return idx >= 0 ? tasks[idx] : null;
  }

  void _updateTask(MockAgentTask updated) {
    final tasks = ref.read(agentTasksProvider);
    final next = List<MockAgentTask>.from(tasks);
    final idx = next.indexWhere((t) => t.id == updated.id);
    if (idx >= 0) {
      next[idx] = updated;
      ref.read(agentTasksProvider.notifier).state = next;
    }
  }

  void _changeStatus(MockAgentTask task, MockAgentTaskStatus newStatus, {String? result, String? error}) {
    _updateTask(task.copyWith(status: newStatus, result: result, error: error));
  }

  @override
  Widget build(BuildContext context) {
    final task = _findTask();
    final toolCalls = MockData.toolCallRecords;

    return AmitiaScaffold(
      appBar: AmitiaAppBar(
        title: task?.title ?? '任务详情',
        showBackButton: true,
        fallbackRoute: AppRoutes.agent,
      ),
      body: task == null
          ? _buildNotFound(context)
          : SingleChildScrollView(
              padding: const EdgeInsets.fromLTRB(
                AppSpacing.pagePadding,
                AppSpacing.sm,
                AppSpacing.pagePadding,
                AppSpacing.xxxl,
              ),
              child: Column(
                crossAxisAlignment: CrossAxisAlignment.start,
                children: [
                  _buildStatusCard(context, task),
                  const SizedBox(height: AppSpacing.sectionGap),
                  _buildExecutionPlan(context, task),
                  const SizedBox(height: AppSpacing.sectionGap),
                  _buildToolCalls(context, toolCalls),
                  const SizedBox(height: AppSpacing.xxxl),
                  _buildBottomActions(context, task),
                ],
              ),
            ),
    );
  }

  Widget _buildNotFound(BuildContext context) {
    return Center(
      child: Column(
        mainAxisAlignment: MainAxisAlignment.center,
        children: [
          Icon(Icons.search_off, size: 56, color: context.textTertiary),
          const SizedBox(height: AppSpacing.md),
          Text('任务不存在', style: AppTypography.cardTitle(context)),
          const SizedBox(height: AppSpacing.lg),
          AmitiaButton(
            label: '返回任务列表',
            icon: Icons.arrow_back,
            onPressed: () => context.go(AppRoutes.agent),
          ),
        ],
      ),
    );
  }

  Widget _buildStatusCard(BuildContext context, MockAgentTask task) {
    return AmitiaCard(
      child: Column(
        crossAxisAlignment: CrossAxisAlignment.start,
        children: [
          Row(
            children: [
              Text('任务状态', style: AppTypography.sectionTitle(context)),
              const Spacer(),
              AmitiaStatusBadge(
                label: mockAgentTaskStatusLabel(task.status),
                type: mockAgentTaskBadgeType(task.status),
              ),
            ],
          ),
          const SizedBox(height: AppSpacing.sm),
          Text(task.description, style: AppTypography.caption(context)),
          const SizedBox(height: AppSpacing.md),
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
          const SizedBox(height: AppSpacing.xs),
          AmitiaProgressBar(progress: task.progress / 100),
          const SizedBox(height: AppSpacing.md),
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
          const SizedBox(height: AppSpacing.sm),
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
            const SizedBox(height: AppSpacing.md),
            Container(
              padding: const EdgeInsets.all(AppSpacing.md),
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
            const SizedBox(height: AppSpacing.md),
            Container(
              padding: const EdgeInsets.all(AppSpacing.md),
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

  Widget _buildExecutionPlan(BuildContext context, MockAgentTask task) {
    return Column(
      crossAxisAlignment: CrossAxisAlignment.start,
      children: [
        Text('执行计划', style: AppTypography.sectionTitle(context)),
        const SizedBox(height: AppSpacing.md),
        Container(
          padding: const EdgeInsets.all(AppSpacing.cardPadding),
          decoration: BoxDecoration(
            color: context.surfacePrimary,
            borderRadius: AppRadius.brMedium,
            border: Border.all(color: context.borderPrimary, width: 0.5),
          ),
          child: Column(
            children: List.generate(task.steps.length, (index) {
              final stepName = task.steps[index];
              final isLast = index == task.steps.length - 1;
              final isCompleted = task.status == MockAgentTaskStatus.completed ||
                  index < task.currentStepIndex;
              final isRunning = task.status == MockAgentTaskStatus.running &&
                  index == task.currentStepIndex;
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

  Widget _buildToolCalls(BuildContext context, List toolCalls) {
    return Column(
      crossAxisAlignment: CrossAxisAlignment.start,
      children: [
        Text('工具调用记录', style: AppTypography.sectionTitle(context)),
        const SizedBox(height: AppSpacing.md),
        ...toolCalls.map((record) => Padding(
              padding: const EdgeInsets.only(bottom: AppSpacing.sm),
              child: _ToolCallCard(record: record),
            )),
      ],
    );
  }

  Widget _buildBottomActions(BuildContext context, MockAgentTask task) {
    switch (task.status) {
      case MockAgentTaskStatus.pending:
        return Row(
          children: [
            Expanded(
              child: AmitiaButton(
                label: '开始',
                icon: Icons.play_arrow,
                onPressed: () {
                  _changeStatus(task, MockAgentTaskStatus.running);
                  amitiaSnackBar(context, '任务已开始执行');
                },
              ),
            ),
            const SizedBox(width: AppSpacing.sm),
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
                    _changeStatus(task, MockAgentTaskStatus.cancelled);
                    amitiaSnackBar(context, '任务已取消');
                  },
                ),
              ),
            ),
          ],
        );
      case MockAgentTaskStatus.waitingApproval:
        return Row(
          children: [
            Expanded(
              child: AmitiaButton(
                label: '允许',
                icon: Icons.check,
                onPressed: () {
                  _changeStatus(task, MockAgentTaskStatus.running);
                  amitiaSnackBar(context, '已批准，任务开始执行');
                },
              ),
            ),
            const SizedBox(width: AppSpacing.sm),
            Expanded(
              child: AmitiaButton(
                label: '拒绝',
                isSecondary: true,
                isDestructive: true,
                icon: Icons.close,
                onPressed: () {
                  _changeStatus(task, MockAgentTaskStatus.cancelled);
                  amitiaSnackBar(context, '已拒绝，任务已取消');
                },
              ),
            ),
          ],
        );
      case MockAgentTaskStatus.running:
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
                    _changeStatus(task, MockAgentTaskStatus.paused);
                    amitiaSnackBar(context, '任务已暂停');
                  },
                ),
              ),
            ),
            const SizedBox(width: AppSpacing.sm),
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
                    _changeStatus(task, MockAgentTaskStatus.cancelled);
                    amitiaSnackBar(context, '任务已停止');
                  },
                ),
              ),
            ),
          ],
        );
      case MockAgentTaskStatus.paused:
        return Row(
          children: [
            Expanded(
              child: AmitiaButton(
                label: '继续',
                icon: Icons.play_arrow,
                onPressed: () {
                  _changeStatus(task, MockAgentTaskStatus.running);
                  amitiaSnackBar(context, '任务已继续执行');
                },
              ),
            ),
            const SizedBox(width: AppSpacing.sm),
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
                    _changeStatus(task, MockAgentTaskStatus.cancelled);
                    amitiaSnackBar(context, '任务已停止');
                  },
                ),
              ),
            ),
          ],
        );
      case MockAgentTaskStatus.completed:
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
            const SizedBox(width: AppSpacing.sm),
            Expanded(
              child: AmitiaButton(
                label: '再次执行',
                icon: Icons.refresh,
                onPressed: () {
                  _updateTask(task.copyWith(
                    status: MockAgentTaskStatus.running,
                    progress: 0,
                    currentStepIndex: 0,
                    elapsed: '00:00',
                    result: null,
                    error: null,
                  ));
                  amitiaSnackBar(context, '任务已重新开始');
                },
              ),
            ),
          ],
        );
      case MockAgentTaskStatus.failed:
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
            const SizedBox(width: AppSpacing.sm),
            Expanded(
              child: AmitiaButton(
                label: '重试',
                icon: Icons.refresh,
                onPressed: () {
                  _updateTask(task.copyWith(
                    status: MockAgentTaskStatus.running,
                    progress: 0,
                    currentStepIndex: 0,
                    elapsed: '00:00',
                    error: null,
                  ));
                  amitiaSnackBar(context, '任务已重新开始');
                },
              ),
            ),
          ],
        );
      case MockAgentTaskStatus.cancelled:
        return AmitiaButton(
          label: '再次执行',
          icon: Icons.refresh,
          isFullWidth: true,
          onPressed: () {
            _updateTask(task.copyWith(
              status: MockAgentTaskStatus.running,
              progress: 0,
              currentStepIndex: 0,
              elapsed: '00:00',
              result: null,
              error: null,
            ));
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

class _ToolCallCard extends StatelessWidget {
  final ToolCallRecord record;

  const _ToolCallCard({required this.record});

  @override
  Widget build(BuildContext context) {
    final isSuccess = record.status == '成功';
    final isRunning = record.status == '运行中';

    return Container(
      padding: const EdgeInsets.all(AppSpacing.cardPadding),
      decoration: BoxDecoration(
        color: context.surfacePrimary,
        borderRadius: AppRadius.brMedium,
        border: Border.all(color: context.borderPrimary, width: 0.5),
      ),
      child: Column(
        crossAxisAlignment: CrossAxisAlignment.start,
        children: [
          Row(
            children: [
              Icon(Icons.build_outlined, size: 16, color: context.accentPrimary),
              const SizedBox(width: 8),
              Expanded(
                child: Text(
                  record.toolName,
                  style: AppTypography.bodySmall(context).copyWith(
                    fontWeight: FontWeight.w600,
                  ),
                ),
              ),
              if (isSuccess)
                const AmitiaStatusBadge(label: '成功', type: BadgeType.success)
              else if (isRunning)
                const AmitiaStatusBadge(label: '运行中', type: BadgeType.accent)
              else
                AmitiaStatusBadge(label: record.status, type: BadgeType.neutral),
            ],
          ),
          const SizedBox(height: AppSpacing.md),
          _buildInfoRow(context, '输入', record.input),
          const SizedBox(height: AppSpacing.sm),
          _buildInfoRow(context, '输出', record.output),
          const SizedBox(height: AppSpacing.sm),
          Row(
            children: [
              Icon(Icons.timer_outlined, size: 14, color: context.textTertiary),
              const SizedBox(width: 4),
              Text('耗时 ${record.duration}', style: AppTypography.label(context)),
            ],
          ),
        ],
      ),
    );
  }

  Widget _buildInfoRow(BuildContext context, String label, String value) {
    return Row(
      crossAxisAlignment: CrossAxisAlignment.start,
      children: [
        SizedBox(
          width: 36,
          child: Text(label, style: AppTypography.label(context)),
        ),
        Expanded(
          child: Text(
            value,
            style: AppTypography.bodySmall(context).copyWith(color: context.textSecondary),
          ),
        ),
      ],
    );
  }
}
