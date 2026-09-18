import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:go_router/go_router.dart';

import '../../../../app/app_routes.dart';
import '../../../../app/theme/app_colors.dart';
import '../../../../app/theme/app_radius.dart';
import '../../../../app/theme/app_spacing.dart';
import '../../../../app/theme/app_typography.dart';
import '../../../../core/services/error_utils.dart';
import '../../../../core/services/providers.dart';
import '../../../../core/widgets/amitia_misc.dart';
import '../../../../core/widgets/amitia_scaffold.dart';

class ExecutionRunsPage extends ConsumerStatefulWidget {
  const ExecutionRunsPage({super.key});

  @override
  ConsumerState<ExecutionRunsPage> createState() => _ExecutionRunsPageState();
}

class _ExecutionRunsPageState extends ConsumerState<ExecutionRunsPage> {
  List<Map<String, dynamic>> _runs = const [];
  bool _loading = true;
  String? _error;
  String _statusFilter = '';

  static const _statuses = <String, String>{
    '': '全部',
    'pending': '等待中',
    'running': '运行中',
    'succeeded': '已完成',
    'failed': '失败',
    'cancelled': '已取消',
  };

  @override
  void initState() {
    super.initState();
    _loadRuns();
  }

  Future<void> _loadRuns() async {
    if (mounted) setState(() { _loading = true; _error = null; });
    try {
      final data = await ref.read(extensionServiceProvider).extensionRuns();
      if (mounted) setState(() { _runs = data; _loading = false; });
    } catch (error) {
      if (mounted) setState(() { _error = safeErrorMessage(error); _loading = false; });
    }
  }

  List<Map<String, dynamic>> get _visible => _statusFilter.isEmpty
      ? _runs
      : _runs.where((run) => (run['status'] ?? '').toString() == _statusFilter).toList(growable: false);

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
            _buildFilters(),
            SizedBox(height: AppSpacing.sm),
            Expanded(
              child: _loading
                  ? const AmitiaLoadingState(message: '加载中…')
                  : _error != null
                      ? AmitiaErrorState(message: '加载失败：$_error', onRetry: _loadRuns)
                      : _visible.isEmpty
                          ? AmitiaEmptyState(
                              icon: Icons.history,
                              title: '暂无执行记录',
                              subtitle: _statusFilter.isEmpty ? '尚未产生扩展执行记录' : '当前状态没有记录',
                            )
                          : RefreshIndicator(
                              onRefresh: _loadRuns,
                              child: ListView.separated(
                                padding: EdgeInsets.fromLTRB(AppSpacing.pagePadding, 0, AppSpacing.pagePadding, AppSpacing.xxxl),
                                itemCount: _visible.length,
                                separatorBuilder: (_, _) => SizedBox(height: AppSpacing.sm),
                                itemBuilder: (context, index) => _buildRunCard(_visible[index]),
                              ),
                            ),
            ),
          ],
        ),
      ),
    );
  }

  Widget _buildFilters() {
    final entries = _statuses.entries.toList(growable: false);
    return SizedBox(
      height: 38,
      child: ListView.separated(
        scrollDirection: Axis.horizontal,
        padding: EdgeInsets.symmetric(horizontal: AppSpacing.pagePadding),
        itemCount: entries.length,
        separatorBuilder: (_, _) => SizedBox(width: AppSpacing.sm),
        itemBuilder: (context, index) {
          final entry = entries[index];
          final selected = entry.key == _statusFilter;
          return GestureDetector(
            onTap: () => setState(() => _statusFilter = entry.key),
            child: Container(
              alignment: Alignment.center,
              padding: const EdgeInsets.symmetric(horizontal: 14, vertical: 8),
              decoration: BoxDecoration(
                color: selected ? context.accentPrimary : context.surfaceSecondary,
                borderRadius: AppRadius.brTag,
              ),
              child: Text(entry.value, style: TextStyle(fontSize: 13, color: selected ? Colors.white : context.textSecondary)),
            ),
          );
        },
      ),
    );
  }

  Widget _buildRunCard(Map<String, dynamic> run) {
    final runId = (run['runId'] ?? '').toString();
    final skillId = (run['skillId'] ?? '').toString();
    final extensionId = (run['extensionId'] ?? '').toString();
    final status = (run['status'] ?? '').toString();
    final durationMs = (run['durationMs'] as num?)?.toInt() ?? 0;
    final startedAt = (run['startedAt'] ?? '').toString();
    final inputSummary = (run['inputSummary'] ?? '').toString();
    final traceId = (run['traceId'] ?? '').toString();
    final sideEffects = (run['sideEffects'] as List?) ?? const [];
    return AmitiaCard(
      onTap: runId.isEmpty ? null : () => context.push('/extensions/runs/$runId'),
      child: Column(
        crossAxisAlignment: CrossAxisAlignment.start,
        children: [
          Row(
            children: [
              Icon(_statusIcon(status), size: 22, color: _statusColor(status)),
              const SizedBox(width: 10),
              Expanded(
                child: Column(
                  crossAxisAlignment: CrossAxisAlignment.start,
                  children: [
                    Text(skillId.isNotEmpty ? skillId : (extensionId.isNotEmpty ? extensionId : runId), style: AppTypography.cardTitle(context)),
                    const SizedBox(height: 2),
                    Text('${_formatTime(startedAt)} · ${_formatDuration(durationMs)}', style: AppTypography.label(context)),
                  ],
                ),
              ),
              AmitiaStatusBadge(label: _statusLabel(status), type: _statusBadge(status)),
            ],
          ),
          if (inputSummary.isNotEmpty) ...[
            SizedBox(height: AppSpacing.sm),
            Text(inputSummary, style: AppTypography.bodySmall(context), maxLines: 2, overflow: TextOverflow.ellipsis),
          ],
          SizedBox(height: AppSpacing.sm),
          Wrap(
            spacing: AppSpacing.sm,
            runSpacing: AppSpacing.xs,
            children: [
              if (extensionId.isNotEmpty) AmitiaStatusBadge(label: extensionId, type: BadgeType.neutral),
              if (sideEffects.isNotEmpty) AmitiaStatusBadge(label: '副作用 ${sideEffects.length}', type: BadgeType.warning),
              if (traceId.isNotEmpty) AmitiaStatusBadge(label: 'Trace ${_short(traceId)}', type: BadgeType.info),
            ],
          ),
        ],
      ),
    );
  }

  String _statusLabel(String status) => _statuses[status] ?? (status.isEmpty ? '未知' : status);

  BadgeType _statusBadge(String status) {
    switch (status) {
      case 'succeeded': return BadgeType.success;
      case 'failed': return BadgeType.error;
      case 'running': return BadgeType.accent;
      case 'pending': return BadgeType.warning;
      default: return BadgeType.neutral;
    }
  }

  IconData _statusIcon(String status) {
    switch (status) {
      case 'succeeded': return Icons.check_circle;
      case 'failed': return Icons.cancel;
      case 'running': return Icons.play_circle_fill;
      case 'pending': return Icons.schedule;
      case 'cancelled': return Icons.block;
      default: return Icons.circle_outlined;
    }
  }

  Color _statusColor(String status) {
    switch (status) {
      case 'succeeded': return context.success;
      case 'failed': return context.error;
      case 'running': return context.accentPrimary;
      case 'pending': return context.warning;
      default: return context.textTertiary;
    }
  }

  static String _formatTime(String raw) {
    if (raw.isEmpty) return '未知时间';
    final date = DateTime.tryParse(raw.replaceFirst(' ', 'T'));
    if (date == null) return raw;
    return '${date.month.toString().padLeft(2, '0')}-${date.day.toString().padLeft(2, '0')} ${date.hour.toString().padLeft(2, '0')}:${date.minute.toString().padLeft(2, '0')}';
  }

  static String _formatDuration(int ms) {
    if (ms < 1000) return '${ms}ms';
    return '${(ms / 1000).toStringAsFixed(ms < 10000 ? 1 : 0)}s';
  }

  static String _short(String value) => value.length <= 10 ? value : '${value.substring(0, 8)}…';
}
