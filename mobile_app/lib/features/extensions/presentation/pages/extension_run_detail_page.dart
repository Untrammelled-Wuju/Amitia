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
import '../../../../shared/mock_data/mock_extensions.dart';

class ExtensionRunDetailPage extends ConsumerWidget {
  final String runId;

  const ExtensionRunDetailPage({super.key, required this.runId});

  ExecutionRun? get _run {
    try {
      return MockExtensions.executionRuns.firstWhere((r) => r.id == runId);
    } catch (_) {
      return null;
    }
  }

  BadgeType _statusBadgeType(String status) {
    switch (status) {
      case '运行中':
        return BadgeType.accent;
      case '已完成':
        return BadgeType.success;
      case '失败':
        return BadgeType.error;
      default:
        return BadgeType.neutral;
    }
  }

  Color _statusColor(String status, BuildContext context) {
    switch (status) {
      case '运行中':
        return context.accentPrimary;
      case '已完成':
        return context.success;
      case '失败':
        return context.error;
      default:
        return context.textTertiary;
    }
  }

  @override
  Widget build(BuildContext context, WidgetRef ref) {
    final run = _run;
    if (run == null) {
      return AmitiaScaffold(
        appBar: AmitiaAppBar(title: '执行详情', showBackButton: true),
        body: AmitiaErrorState(message: '未找到该执行记录', onRetry: () => Navigator.pop(context)),
      );
    }

    return AmitiaScaffold(
      appBar: AmitiaAppBar(
        title: run.name,
        showBackButton: true,
      ),
      body: SafeArea(
        top: false,
        child: SingleChildScrollView(
          padding: const EdgeInsets.fromLTRB(AppSpacing.pagePadding, AppSpacing.sm, AppSpacing.pagePadding, AppSpacing.xxxl),
          child: Column(
            crossAxisAlignment: CrossAxisAlignment.start,
            children: [
              _buildStatusCard(context, run),
              const SizedBox(height: AppSpacing.sectionGap),
              _buildInputSection(context, run),
              const SizedBox(height: AppSpacing.sectionGap),
              _buildOutputSection(context, run),
              if (run.error != null) ...[
                const SizedBox(height: AppSpacing.sectionGap),
                _buildErrorSection(context, run),
              ],
              const SizedBox(height: AppSpacing.sectionGap),
              _buildToolCallsSection(context, run),
              const SizedBox(height: AppSpacing.xxl),
              if (run.status == '运行中')
                AmitiaButton(
                  label: '取消任务',
                  isFullWidth: true,
                  isDestructive: true,
                  icon: Icons.stop_circle_outlined,
                  onPressed: () => _showCancelConfirm(context, run),
                ),
            ],
          ),
        ),
      ),
    );
  }

  Widget _buildStatusCard(BuildContext context, ExecutionRun run) {
    return AmitiaCard(
      child: Column(
        crossAxisAlignment: CrossAxisAlignment.start,
        children: [
          Row(
            children: [
              Text('运行状态', style: AppTypography.sectionTitle(context)),
              const Spacer(),
              AmitiaStatusBadge(label: run.status, type: _statusBadgeType(run.status)),
            ],
          ),
          const SizedBox(height: AppSpacing.md),
          Row(
            children: [
              Icon(Icons.timer_outlined, size: 16, color: context.textTertiary),
              const SizedBox(width: 6),
              Text('耗时 ${run.duration}', style: AppTypography.label(context)),
              const SizedBox(width: 16),
              Icon(Icons.schedule, size: 16, color: context.textTertiary),
              const SizedBox(width: 6),
              Text(
                '${run.startTime.month}/${run.startTime.day} ${run.startTime.hour.toString().padLeft(2, '0')}:${run.startTime.minute.toString().padLeft(2, '0')}',
                style: AppTypography.label(context),
              ),
            ],
          ),
          if (run.status == '运行中') ...[
            const SizedBox(height: AppSpacing.md),
            Row(
              mainAxisAlignment: MainAxisAlignment.spaceBetween,
              children: [
                Text('执行进度', style: AppTypography.caption(context)),
                Text('65%', style: AppTypography.caption(context).copyWith(color: context.accentPrimary, fontWeight: FontWeight.w600)),
              ],
            ),
            const SizedBox(height: AppSpacing.xs),
            const AmitiaProgressBar(progress: 0.65),
          ],
        ],
      ),
    );
  }

  Widget _buildInputSection(BuildContext context, ExecutionRun run) {
    return Column(
      crossAxisAlignment: CrossAxisAlignment.start,
      children: [
        Text('输入', style: AppTypography.sectionTitle(context)),
        const SizedBox(height: AppSpacing.md),
        AmitiaCard(
          child: Container(
            width: double.infinity,
            padding: const EdgeInsets.all(12),
            decoration: BoxDecoration(
              color: context.surfaceSecondary,
              borderRadius: AppRadius.brSmall,
            ),
            child: Text(run.input, style: AppTypography.bodySmall(context)),
          ),
        ),
      ],
    );
  }

  Widget _buildOutputSection(BuildContext context, ExecutionRun run) {
    return Column(
      crossAxisAlignment: CrossAxisAlignment.start,
      children: [
        Text('输出', style: AppTypography.sectionTitle(context)),
        const SizedBox(height: AppSpacing.md),
        AmitiaCard(
          child: Container(
            width: double.infinity,
            padding: const EdgeInsets.all(12),
            decoration: BoxDecoration(
              color: run.output.isEmpty ? context.surfaceSecondary : context.accentSoft,
              borderRadius: AppRadius.brSmall,
            ),
            child: Text(
              run.output.isEmpty ? '(无输出)' : run.output,
              style: AppTypography.bodySmall(context).copyWith(
                color: run.output.isEmpty ? context.textTertiary : context.textPrimary,
              ),
            ),
          ),
        ),
      ],
    );
  }

  Widget _buildErrorSection(BuildContext context, ExecutionRun run) {
    return Column(
      crossAxisAlignment: CrossAxisAlignment.start,
      children: [
        Text('错误信息', style: AppTypography.sectionTitle(context).copyWith(color: context.error)),
        const SizedBox(height: AppSpacing.md),
        Container(
          padding: const EdgeInsets.all(12),
          decoration: BoxDecoration(
            color: context.error.withValues(alpha: 0.08),
            borderRadius: AppRadius.brMedium,
            border: Border.all(color: context.error.withValues(alpha: 0.3), width: 1),
          ),
          child: Row(
            crossAxisAlignment: CrossAxisAlignment.start,
            children: [
              Icon(Icons.error_outline, size: 18, color: context.error),
              const SizedBox(width: 8),
              Expanded(
                child: Text(run.error!, style: AppTypography.bodySmall(context).copyWith(color: context.error)),
              ),
            ],
          ),
        ),
      ],
    );
  }

  Widget _buildToolCallsSection(BuildContext context, ExecutionRun run) {
    return Column(
      crossAxisAlignment: CrossAxisAlignment.start,
      children: [
        Text('工具调用 (${run.toolCalls.length})', style: AppTypography.sectionTitle(context)),
        const SizedBox(height: AppSpacing.md),
        ...run.toolCalls.map((call) => Padding(
              padding: const EdgeInsets.only(bottom: AppSpacing.sm),
              child: _ToolCallCard(call: call),
            )),
      ],
    );
  }

  void _showCancelConfirm(BuildContext context, ExecutionRun run) {
    showDialog(
      context: context,
      builder: (dialogContext) => AlertDialog(
        backgroundColor: context.surfacePrimary,
        shape: RoundedRectangleBorder(borderRadius: AppRadius.brLarge),
        title: Text('取消任务', style: AppTypography.cardTitle(context)),
        content: Text('确定要取消「${run.name}」吗？正在执行的操作将被中断，已完成的步骤不受影响。', style: AppTypography.bodySmall(context)),
        actions: [
          TextButton(onPressed: () => Navigator.pop(dialogContext), child: Text('继续运行', style: TextStyle(color: context.textSecondary))),
          TextButton(
            onPressed: () {
              Navigator.pop(dialogContext);
              Navigator.pop(context);
              ScaffoldMessenger.of(context).showSnackBar(
                SnackBar(content: Text('${run.name} 已取消'), backgroundColor: context.error),
              );
            },
            child: Text('取消任务', style: TextStyle(color: context.error)),
          ),
        ],
      ),
    );
  }
}

class _ToolCallCard extends StatelessWidget {
  final ToolCallEntry call;

  const _ToolCallCard({required this.call});

  BadgeType _statusBadgeType(String status) {
    switch (status) {
      case '成功':
        return BadgeType.success;
      case '运行中':
        return BadgeType.accent;
      case '失败':
        return BadgeType.error;
      default:
        return BadgeType.neutral;
    }
  }

  @override
  Widget build(BuildContext context) {
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
                child: Text(call.toolName, style: AppTypography.bodySmall(context).copyWith(fontWeight: FontWeight.w600)),
              ),
              AmitiaStatusBadge(label: call.status, type: _statusBadgeType(call.status)),
            ],
          ),
          const SizedBox(height: AppSpacing.md),
          _InfoRow(label: '输入', value: call.input),
          const SizedBox(height: AppSpacing.sm),
          _InfoRow(label: '输出', value: call.output),
          const SizedBox(height: AppSpacing.sm),
          Row(
            children: [
              Icon(Icons.timer_outlined, size: 14, color: context.textTertiary),
              const SizedBox(width: 4),
              Text('耗时 ${call.duration}', style: AppTypography.label(context)),
            ],
          ),
        ],
      ),
    );
  }
}

class _InfoRow extends StatelessWidget {
  final String label;
  final String value;

  const _InfoRow({required this.label, required this.value});

  @override
  Widget build(BuildContext context) {
    return Row(
      crossAxisAlignment: CrossAxisAlignment.start,
      children: [
        SizedBox(width: 40, child: Text(label, style: AppTypography.label(context))),
        Expanded(child: Text(value, style: AppTypography.bodySmall(context).copyWith(color: context.textSecondary))),
      ],
    );
  }
}
