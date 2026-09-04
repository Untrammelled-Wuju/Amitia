import 'dart:convert';
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
    final detailAsync = ref.watch(agentTaskRuntimeDetailProvider(taskId));

    return AmitiaScaffold(
      appBar: AmitiaAppBar(
        title: '任务详情',
        showBackButton: true,
        fallbackRoute: AppRoutes.agent,
        actions: [
          AmitiaIconButton(
            icon: Icons.refresh,
            tooltip: '刷新真实运行状态',
            onPressed: () => ref.invalidate(agentTaskRuntimeDetailProvider(taskId)),
          ),
        ],
      ),
      body: detailAsync.when(
        loading: () => const Center(child: CircularProgressIndicator()),
        error: (err, _) => Center(
          child: Padding(
            padding: EdgeInsets.all(AppSpacing.pagePadding),
            child: Column(
              mainAxisSize: MainAxisSize.min,
              children: [
                Icon(Icons.error_outline, size: 48, color: context.error),
                SizedBox(height: AppSpacing.md),
                Text('运行详情加载失败', style: AppTypography.cardTitle(context)),
                SizedBox(height: AppSpacing.sm),
                Text('$err', style: AppTypography.caption(context), textAlign: TextAlign.center),
                SizedBox(height: AppSpacing.lg),
                AmitiaButton(
                  label: '重新加载',
                  icon: Icons.refresh,
                  onPressed: () => ref.invalidate(agentTaskRuntimeDetailProvider(taskId)),
                ),
              ],
            ),
          ),
        ),
        data: (detail) {
          final task = detail.task;
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
                _buildStatusCard(context, task, detail),
                SizedBox(height: AppSpacing.sectionGap),
                _buildRuntimeDetails(context, detail),
                SizedBox(height: AppSpacing.sectionGap),
                _buildToolCalls(context, detail),
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

  Widget _buildStatusCard(BuildContext context, AgentTaskItem task, AgentTaskRuntimeDetail detail) {
    final stage = (detail.progress['stage'] ?? '').toString();
    final message = (detail.progress['message'] ?? '').toString();
    final current = (detail.progress['current'] as num?)?.toInt();
    final total = (detail.progress['total'] as num?)?.toInt();
    return AmitiaCard(
      child: Column(
        crossAxisAlignment: CrossAxisAlignment.start,
        children: [
          Row(
            children: [
              Expanded(child: Text(task.title, style: AppTypography.sectionTitle(context))),
              AmitiaStatusBadge(label: _statusLabel(task.status), type: _badgeType(task.status)),
            ],
          ),
          if (task.description.trim().isNotEmpty) ...[
            SizedBox(height: AppSpacing.sm),
            Text(task.description, style: AppTypography.caption(context)),
          ],
          SizedBox(height: AppSpacing.md),
          Row(
            mainAxisAlignment: MainAxisAlignment.spaceBetween,
            children: [
              Text(stage.isEmpty ? '执行进度' : stage, style: AppTypography.caption(context)),
              Text(
                '${task.progress}%',
                style: AppTypography.caption(context).copyWith(color: context.accentPrimary, fontWeight: FontWeight.w600),
              ),
            ],
          ),
          SizedBox(height: AppSpacing.xs),
          AmitiaProgressBar(progress: task.progress / 100),
          if (message.isNotEmpty) ...[
            SizedBox(height: AppSpacing.sm),
            Text(message, style: AppTypography.bodySmall(context)),
          ],
          SizedBox(height: AppSpacing.md),
          Wrap(
            spacing: 14,
            runSpacing: 8,
            children: [
              _meta(context, Icons.timer_outlined, '已运行 ${task.elapsed}'),
              if (current != null && total != null && total > 0) _meta(context, Icons.checklist, '进度 $current/$total'),
              _meta(context, Icons.layers_outlined, 'Generation ${task.generation}'),
            ],
          ),
          if (task.requiredAbilities.isNotEmpty) ...[
            SizedBox(height: AppSpacing.md),
            Wrap(
              spacing: 6,
              runSpacing: 4,
              children: task.requiredAbilities
                  .map((a) => Container(
                        padding: const EdgeInsets.symmetric(horizontal: 8, vertical: 3),
                        decoration: BoxDecoration(color: context.accentSoft, borderRadius: AppRadius.brTag),
                        child: Text(a, style: AppTypography.label(context).copyWith(color: context.accentPrimary)),
                      ))
                  .toList(growable: false),
            ),
          ],
          if (task.error != null && task.error!.trim().isNotEmpty) ...[
            SizedBox(height: AppSpacing.md),
            _payloadBox(context, '错误', task.error!, tone: context.error),
          ],
        ],
      ),
    );
  }

  Widget _meta(BuildContext context, IconData icon, String label) {
    return Row(
      mainAxisSize: MainAxisSize.min,
      children: [
        Icon(icon, size: 16, color: context.textTertiary),
        const SizedBox(width: 6),
        Text(label, style: AppTypography.label(context)),
      ],
    );
  }

  Widget _buildRuntimeDetails(BuildContext context, AgentTaskRuntimeDetail detail) {
    final sections = <MapEntry<String, Map<String, dynamic>>>[
      MapEntry('任务输入', detail.run['input'] is Map ? Map<String, dynamic>.from(detail.run['input'] as Map) : const <String, dynamic>{}),
      MapEntry('实时进度', detail.progress),
      MapEntry('执行结果', detail.result),
      MapEntry('检查点', detail.checkpoint),
    ].where((entry) => entry.value.isNotEmpty).toList(growable: false);

    return Column(
      crossAxisAlignment: CrossAxisAlignment.start,
      children: [
        Text('运行时数据', style: AppTypography.sectionTitle(context)),
        SizedBox(height: AppSpacing.md),
        if (sections.isEmpty)
          Text('运行时尚未返回进度、结果或检查点数据', style: AppTypography.caption(context))
        else
          ...sections.map((entry) => Padding(
                padding: EdgeInsets.only(bottom: AppSpacing.sm),
                child: _payloadBox(context, entry.key, _pretty(entry.value)),
              )),
      ],
    );
  }

  Widget _payloadBox(BuildContext context, String title, String payload, {Color? tone}) {
    final color = tone ?? context.textPrimary;
    return Container(
      width: double.infinity,
      padding: EdgeInsets.all(AppSpacing.md),
      decoration: BoxDecoration(
        color: tone == null ? context.surfaceSecondary : color.withValues(alpha: 0.08),
        borderRadius: AppRadius.brSmall,
        border: Border.all(color: tone == null ? context.borderPrimary : color.withValues(alpha: 0.24), width: 0.5),
      ),
      child: Column(
        crossAxisAlignment: CrossAxisAlignment.start,
        children: [
          Text(title, style: AppTypography.label(context).copyWith(fontWeight: FontWeight.w600, color: color)),
          const SizedBox(height: 6),
          SelectableText(payload, style: AppTypography.caption(context).copyWith(color: color)),
        ],
      ),
    );
  }

  Widget _buildToolCalls(BuildContext context, AgentTaskRuntimeDetail detail) {
    final calls = <dynamic>[];
    _collectToolCalls(detail.progress, calls);
    _collectToolCalls(detail.result, calls);
    _collectToolCalls(detail.checkpoint, calls);
    return Column(
      crossAxisAlignment: CrossAxisAlignment.start,
      children: [
        Text('工具调用记录', style: AppTypography.sectionTitle(context)),
        SizedBox(height: AppSpacing.md),
        if (calls.isEmpty)
          Text('当前运行时没有返回工具调用明细；页面不会再用固定空态覆盖真实数据。', style: AppTypography.caption(context))
        else
          ...calls.asMap().entries.map((entry) => Padding(
                padding: EdgeInsets.only(bottom: AppSpacing.sm),
                child: _payloadBox(context, '调用 ${entry.key + 1}', _pretty(entry.value)),
              )),
      ],
    );
  }

  void _collectToolCalls(dynamic value, List<dynamic> output) {
    if (value is Map) {
      for (final entry in value.entries) {
        final key = entry.key.toString().toLowerCase().replaceAll('_', '');
        if (key == 'toolcalls' || key == 'toolcall' || key == 'calls') {
          if (entry.value is List) {
            output.addAll(entry.value as List);
          } else if (entry.value != null) {
            output.add(entry.value);
          }
        } else {
          _collectToolCalls(entry.value, output);
        }
      }
    } else if (value is List) {
      for (final item in value) {
        _collectToolCalls(item, output);
      }
    }
  }

  String _pretty(dynamic value) {
    if (value is String) {
      try {
        return const JsonEncoder.withIndent('  ').convert(jsonDecode(value));
      } catch (_) {
        return value;
      }
    }
    try {
      return const JsonEncoder.withIndent('  ').convert(value);
    } catch (_) {
      return value.toString();
    }
  }

  Widget _buildBottomActions(BuildContext context, AgentTaskItem task, WidgetRef ref) {
    Future<void> changeStatus(AgentTaskStatus newStatus, String successMessage) async {
      try {
        await ref.read(agentTasksProvider.notifier).changeStatus(task.id, newStatus);
        ref.invalidate(agentTaskRuntimeDetailProvider(task.id));
        if (context.mounted) amitiaSnackBar(context, successMessage);
      } catch (e) {
        if (context.mounted) amitiaSnackBar(context, '操作失败：$e');
      }
    }

    Future<void> recover() async {
      try {
        await ref.read(agentTasksProvider.notifier).recover(task.id);
        ref.invalidate(agentTaskRuntimeDetailProvider(task.id));
        if (context.mounted) amitiaSnackBar(context, '已通过 Kernel recover 提交恢复');
      } catch (e) {
        if (context.mounted) amitiaSnackBar(context, '恢复失败：$e');
      }
    }

    switch (task.status) {
      case AgentTaskStatus.pending:
        return Row(
          children: [
            Expanded(
              child: AmitiaButton(
                label: '等待 Kernel 调度',
                icon: Icons.schedule,
                onPressed: null,
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
                  message: 'Kernel Task 不提供手动 start 接口；当前任务由队列自动调度。确定取消此排队任务吗？',
                  confirmLabel: '取消任务',
                  onConfirm: () => changeStatus(AgentTaskStatus.cancelled, '任务已由服务端取消'),
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
                label: '允许并恢复',
                icon: Icons.settings_backup_restore,
                onPressed: recover,
              ),
            ),
            SizedBox(width: AppSpacing.sm),
            Expanded(
              child: AmitiaButton(
                label: '拒绝并取消',
                isSecondary: true,
                isDestructive: true,
                icon: Icons.close,
                onPressed: () => changeStatus(AgentTaskStatus.cancelled, '已拒绝并由服务端取消任务'),
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
                    changeStatus(AgentTaskStatus.paused, '任务已由服务端暂停');
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
                    changeStatus(AgentTaskStatus.cancelled, '任务已由服务端停止');
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
                  changeStatus(AgentTaskStatus.running, '任务已由服务端继续执行');
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
                    changeStatus(AgentTaskStatus.cancelled, '任务已由服务端停止');
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
                onPressed: () => changeStatus(AgentTaskStatus.running, '任务已通过 Retry 重新入队'),
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
                onPressed: () => changeStatus(AgentTaskStatus.running, '任务已通过 Retry 重新入队'),
              ),
            ),
          ],
        );
      case AgentTaskStatus.cancelled:
        return AmitiaButton(
          label: '再次执行',
          icon: Icons.refresh,
          isFullWidth: true,
          onPressed: () => changeStatus(AgentTaskStatus.running, '任务已通过 Retry 重新入队'),
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
