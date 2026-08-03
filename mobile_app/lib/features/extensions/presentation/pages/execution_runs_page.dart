import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:go_router/go_router.dart';
import '../../../../app/app_routes.dart';
import '../../../../app/theme/app_colors.dart';
import '../../../../app/theme/app_spacing.dart';
import '../../../../app/theme/app_radius.dart';
import '../../../../app/theme/app_typography.dart';
import '../../../../core/widgets/amitia_scaffold.dart';
import '../../../../core/widgets/amitia_misc.dart';
import '../../../../shared/models/models.dart';
import '../../../../shared/mock_data/mock_extensions.dart';
import 'extension_run_detail_page.dart';

class ExecutionRunsPage extends ConsumerStatefulWidget {
  const ExecutionRunsPage({super.key});

  @override
  ConsumerState<ExecutionRunsPage> createState() => _ExecutionRunsPageState();
}

class _ExecutionRunsPageState extends ConsumerState<ExecutionRunsPage> {
  late List<ExecutionRun> _runs;
  String _selectedFilter = '全部';
  final _filters = ['全部', '运行中', '已完成', '失败'];

  @override
  void initState() {
    super.initState();
    _runs = List.from(MockExtensions.executionRuns);
  }

  List<ExecutionRun> get _filteredRuns {
    if (_selectedFilter == '全部') return _runs;
    return _runs.where((r) => r.status == _selectedFilter).toList();
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

  IconData _statusIcon(String status) {
    switch (status) {
      case '运行中':
        return Icons.play_circle_fill;
      case '已完成':
        return Icons.check_circle;
      case '失败':
        return Icons.cancel;
      default:
        return Icons.circle_outlined;
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

  String _formatTime(DateTime time) {
    return '${time.month.toString().padLeft(2, '0')}-${time.day.toString().padLeft(2, '0')} ${time.hour.toString().padLeft(2, '0')}:${time.minute.toString().padLeft(2, '0')}';
  }

  @override
  Widget build(BuildContext context) {
    return AmitiaScaffold(
      appBar: AmitiaAppBar(
        title: '执行记录',
        showBackButton: true,
        fallbackRoute: AppRoutes.extensions,
      ),
      body: SafeArea(
        top: false,
        child: Column(
          children: [
            _buildFilterBar(context),
            const SizedBox(height: AppSpacing.sm),
            Expanded(
              child: _filteredRuns.isEmpty
                  ? AmitiaEmptyState(
                      icon: Icons.history,
                      title: '暂无执行记录',
                      subtitle: '$_selectedFilter 状态下没有记录',
                    )
                  : ListView.separated(
                      padding: const EdgeInsets.fromLTRB(AppSpacing.pagePadding, 0, AppSpacing.pagePadding, AppSpacing.xxxl),
                      itemCount: _filteredRuns.length,
                      separatorBuilder: (_, _) => const SizedBox(height: AppSpacing.sm),
                      itemBuilder: (context, index) => _buildRunCard(context, _filteredRuns[index]),
                    ),
            ),
          ],
        ),
      ),
    );
  }

  Widget _buildFilterBar(BuildContext context) {
    return SizedBox(
      height: 38,
      child: ListView.separated(
        scrollDirection: Axis.horizontal,
        padding: const EdgeInsets.symmetric(horizontal: AppSpacing.pagePadding),
        itemCount: _filters.length,
        separatorBuilder: (_, _) => const SizedBox(width: AppSpacing.sm),
        itemBuilder: (context, index) {
          final isSelected = _selectedFilter == _filters[index];
          return GestureDetector(
            onTap: () => setState(() => _selectedFilter = _filters[index]),
            child: Container(
              padding: const EdgeInsets.symmetric(horizontal: 14, vertical: 8),
              decoration: BoxDecoration(
                color: isSelected ? context.accentPrimary : context.surfaceSecondary,
                borderRadius: AppRadius.brTag,
              ),
              child: Center(
                child: Text(
                  _filters[index],
                  style: TextStyle(
                    fontSize: 13,
                    fontWeight: isSelected ? FontWeight.w600 : FontWeight.w400,
                    color: isSelected ? Colors.white : context.textSecondary,
                  ),
                ),
              ),
            ),
          );
        },
      ),
    );
  }

  Widget _buildRunCard(BuildContext context, ExecutionRun run) {
    final color = _statusColor(run.status, context);
    return AmitiaCard(
      onTap: () => context.push('/extensions/runs/${run.id}'),
      child: Column(
        crossAxisAlignment: CrossAxisAlignment.start,
        children: [
          Row(
            children: [
              Icon(_statusIcon(run.status), size: 22, color: color),
              const SizedBox(width: 10),
              Expanded(
                child: Column(
                  crossAxisAlignment: CrossAxisAlignment.start,
                  children: [
                    Text(run.name, style: AppTypography.cardTitle(context)),
                    const SizedBox(height: 2),
                    Row(
                      children: [
                        Text(_formatTime(run.startTime), style: AppTypography.label(context)),
                        const SizedBox(width: 8),
                        Icon(Icons.timer_outlined, size: 12, color: context.textTertiary),
                        const SizedBox(width: 3),
                        Text(run.duration, style: AppTypography.label(context)),
                      ],
                    ),
                  ],
                ),
              ),
              AmitiaStatusBadge(label: run.status, type: _statusBadgeType(run.status)),
            ],
          ),
          const SizedBox(height: AppSpacing.md),
          Row(
            children: [
              Icon(Icons.build_outlined, size: 14, color: context.textTertiary),
              const SizedBox(width: 4),
              Text('${run.toolCalls.length} 个工具调用', style: AppTypography.label(context)),
              const Spacer(),
              if (run.status == '运行中')
                GestureDetector(
                  onTap: () => _showCancelConfirm(run),
                  child: Container(
                    padding: const EdgeInsets.symmetric(horizontal: 10, vertical: 5),
                    decoration: BoxDecoration(
                      color: context.error.withValues(alpha: 0.1),
                      borderRadius: AppRadius.brTag,
                    ),
                    child: Text('取消任务', style: TextStyle(fontSize: 12, color: context.error, fontWeight: FontWeight.w500)),
                  ),
                ),
              const SizedBox(width: 8),
              Icon(Icons.chevron_right, color: context.textTertiary, size: 20),
            ],
          ),
        ],
      ),
    );
  }

  void _showCancelConfirm(ExecutionRun run) {
    showDialog(
      context: context,
      builder: (context) => AlertDialog(
        backgroundColor: context.surfacePrimary,
        shape: RoundedRectangleBorder(borderRadius: AppRadius.brLarge),
        title: Text('取消任务', style: AppTypography.cardTitle(context)),
        content: Text('确定要取消「${run.name}」吗？正在执行的操作将被中断。', style: AppTypography.bodySmall(context)),
        actions: [
          TextButton(onPressed: () => Navigator.pop(context), child: Text('继续运行', style: TextStyle(color: context.textSecondary))),
          TextButton(
            onPressed: () {
              Navigator.pop(context);
              setState(() {
                final index = _runs.indexWhere((r) => r.id == run.id);
                _runs[index] = ExecutionRun(
                  id: run.id,
                  name: run.name,
                  status: '失败',
                  duration: run.duration,
                  input: run.input,
                  output: run.output,
                  error: '用户取消任务',
                  toolCalls: run.toolCalls,
                  startTime: run.startTime,
                );
              });
              ScaffoldMessenger.of(this.context).showSnackBar(
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
