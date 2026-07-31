import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import '../../../../app/theme/app_colors.dart';
import '../../../../app/theme/app_typography.dart';
import '../../../../app/theme/app_spacing.dart';
import '../../../../app/theme/app_radius.dart';
import '../../../../core/widgets/amitia_scaffold.dart';
import '../../../../core/widgets/amitia_button.dart';
import '../../../../core/widgets/amitia_misc.dart';
import '../../../../shared/models/models.dart';
import '../../../../shared/mock_data/mock_data.dart';

class AgentTaskDetailPage extends ConsumerWidget {
  final String taskId;

  const AgentTaskDetailPage({super.key, required this.taskId});

  @override
  Widget build(BuildContext context, WidgetRef ref) {
    final task = _findTask();
    final steps = MockData.agentTaskDetailSteps;
    final toolCalls = MockData.toolCallRecords;

    return AmitiaScaffold(
      appBar: AmitiaAppBar(
        title: task?.title ?? '任务详情',
        showBackButton: true,
      ),
      body: SingleChildScrollView(
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
            _buildExecutionPlan(context, steps),
            const SizedBox(height: AppSpacing.sectionGap),
            _buildToolCalls(context, toolCalls),
            const SizedBox(height: AppSpacing.xxxl),
            _buildBottomActions(context),
          ],
        ),
      ),
    );
  }

  AgentTask? _findTask() {
    final all = [
      ...MockData.agentTasksRunning,
      ...MockData.agentTasksPending,
      ...MockData.agentTasksCompleted,
    ];
    final matches = all.where((t) => t.id == taskId);
    return matches.isEmpty ? null : matches.first;
  }

  Widget _buildStatusCard(BuildContext context, AgentTask? task) {
    if (task == null) return const SizedBox();
    return AmitiaCard(
      child: Column(
        crossAxisAlignment: CrossAxisAlignment.start,
        children: [
          Row(
            children: [
              Text('任务状态', style: AppTypography.sectionTitle(context)),
              const Spacer(),
              _buildStatusBadge(context, task.status),
            ],
          ),
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
              Text('已运行 ${task.elapsed ?? '00:00'}', style: AppTypography.label(context)),
              const SizedBox(width: 16),
              Icon(Icons.label_outline, size: 16, color: context.textTertiary),
              const SizedBox(width: 6),
              Text(task.category ?? '未分类', style: AppTypography.label(context)),
            ],
          ),
        ],
      ),
    );
  }

  Widget _buildStatusBadge(BuildContext context, AgentTaskStatus status) {
    return switch (status) {
      AgentTaskStatus.running => const AmitiaStatusBadge(label: '运行中', type: BadgeType.accent),
      AgentTaskStatus.pending => const AmitiaStatusBadge(label: '等待中', type: BadgeType.warning),
      AgentTaskStatus.completed => const AmitiaStatusBadge(label: '已完成', type: BadgeType.success),
      AgentTaskStatus.paused => const AmitiaStatusBadge(label: '已暂停', type: BadgeType.neutral),
    };
  }

  Widget _buildExecutionPlan(BuildContext context, List<AgentTaskStep> steps) {
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
            children: List.generate(steps.length, (index) {
              final step = steps[index];
              final isLast = index == steps.length - 1;
              return _TimelineStep(step: step, isLast: isLast);
            }),
          ),
        ),
      ],
    );
  }

  Widget _buildToolCalls(BuildContext context, List<ToolCallRecord> toolCalls) {
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

  Widget _buildBottomActions(BuildContext context) {
    return Row(
      children: [
        Expanded(
          child: AmitiaButton(
            label: '暂停',
            isSecondary: true,
            icon: Icons.pause,
            onPressed: () => showAmitiaConfirmDialog(context, title: '暂停任务', message: '确定要暂停此任务吗？', confirmLabel: '暂停').then((confirmed) { if (confirmed == true) { amitiaSnackBar(context, '任务已暂停'); } }),
          ),
        ),
        const SizedBox(width: AppSpacing.sm),
        Expanded(
          child: AmitiaButton(
            label: '继续',
            icon: Icons.play_arrow,
            onPressed: () => amitiaSnackBar(context, '任务已继续执行'),
          ),
        ),
        const SizedBox(width: AppSpacing.sm),
        Expanded(
          child: AmitiaButton(
            label: '停止',
            isDestructive: true,
            icon: Icons.stop,
            onPressed: () => showAmitiaConfirmDialog(context, title: '停止任务', message: '确定要停止此任务吗？停止后任务将被终止，此操作不可撤销。', confirmLabel: '停止', isDestructive: true).then((confirmed) { if (confirmed == true) { amitiaSnackBar(context, '任务已停止'); } }),
          ),
        ),
      ],
    );
  }
}

class _TimelineStep extends StatelessWidget {
  final AgentTaskStep step;
  final bool isLast;

  const _TimelineStep({required this.step, required this.isLast});

  Color _statusColor(BuildContext context) {
    switch (step.status) {
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
    final isCompleted = step.status == '已完成';
    final isRunning = step.status == '执行中';

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
              child: Row(
                crossAxisAlignment: CrossAxisAlignment.start,
                children: [
                  Expanded(
                    child: Column(
                      crossAxisAlignment: CrossAxisAlignment.start,
                      children: [
                        Text(
                          step.name,
                          style: AppTypography.bodySmall(context).copyWith(
                            fontWeight: FontWeight.w500,
                            color: step.status == '等待中'
                                ? context.textTertiary
                                : context.textPrimary,
                          ),
                        ),
                        const SizedBox(height: 2),
                        Text(step.status, style: AppTypography.label(context).copyWith(color: color)),
                      ],
                    ),
                  ),
                  if (step.duration != null)
                    Text(step.duration!, style: AppTypography.label(context)),
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
