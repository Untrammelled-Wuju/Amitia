import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import '../../../../app/app_routes.dart';
import '../../../../app/theme/app_colors.dart';
import '../../../../app/theme/app_spacing.dart';
import '../../../../app/theme/app_radius.dart';
import '../../../../app/theme/app_typography.dart';
import '../../../../core/widgets/amitia_scaffold.dart';
import '../../../../core/widgets/amitia_button.dart';
import '../../../../core/widgets/amitia_misc.dart';
import '../../../../core/services/providers.dart';

class ExtensionRunDetailPage extends ConsumerStatefulWidget {
  final String runId;

  const ExtensionRunDetailPage({super.key, required this.runId});

  @override
  ConsumerState<ExtensionRunDetailPage> createState() => _ExtensionRunDetailPageState();
}

class _ExtensionRunDetailPageState extends ConsumerState<ExtensionRunDetailPage> {
  Map<String, dynamic>? _run;
  bool _loading = true;
  String? _error;

  @override
  void initState() {
    super.initState();
    _loadRun();
  }

  Future<void> _loadRun() async {
    setState(() { _loading = true; _error = null; });
    try {
      final svc = ref.read(extensionServiceProvider);
      final data = await svc.getExtensionRun(widget.runId);
      if (mounted) setState(() { _run = data; _loading = false; });
    } catch (e) {
      if (mounted) setState(() { _error = e.toString(); _loading = false; });
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

  @override
  Widget build(BuildContext context) {
    if (_loading) {
      return AmitiaScaffold(
        appBar: AmitiaAppBar(title: '执行详情', showBackButton: true, fallbackRoute: AppRoutes.extensions),
        body: SafeArea(top: false, child: const AmitiaLoadingState(message: '加载中...')),
      );
    }
    if (_error != null || _run == null) {
      return AmitiaScaffold(
        appBar: AmitiaAppBar(title: '执行详情', showBackButton: true, fallbackRoute: AppRoutes.extensions),
        body: SafeArea(top: false, child: AmitiaErrorState(message: _error ?? '未找到该执行记录', onRetry: () {
          Navigator.pop(context);
        })),
      );
    }

    final run = _run!;
    final status = (run['status'] ?? '').toString();
    final name = (run['name'] ?? '').toString();
    final duration = (run['duration'] ?? '').toString();
    final input = (run['input'] ?? '').toString();
    final output = (run['output'] ?? '').toString();
    final error = run['error']?.toString();
    final toolCalls = (run['toolCalls'] as List?)?.cast<Map<String, dynamic>>() ?? [];
    final startTimeStr = (run['startTime'] ?? '').toString();

    return AmitiaScaffold(
      appBar: AmitiaAppBar(
        title: name,
        showBackButton: true,
        fallbackRoute: AppRoutes.extensions,
      ),
      body: SafeArea(
        top: false,
        child: SingleChildScrollView(
          padding: EdgeInsets.fromLTRB(AppSpacing.pagePadding, AppSpacing.sm, AppSpacing.pagePadding, AppSpacing.xxxl),
          child: Column(
            crossAxisAlignment: CrossAxisAlignment.start,
            children: [
              _buildStatusCard(context, status, duration, startTimeStr),
              SizedBox(height: AppSpacing.sectionGap),
              _buildInputSection(context, input),
              SizedBox(height: AppSpacing.sectionGap),
              _buildOutputSection(context, output),
              if (error != null) ...[
                SizedBox(height: AppSpacing.sectionGap),
                _buildErrorSection(context, error),
              ],
              SizedBox(height: AppSpacing.sectionGap),
              _buildToolCallsSection(context, toolCalls),
              SizedBox(height: AppSpacing.xxl),
              if (status == '运行中')
                AmitiaButton(
                  label: '取消任务',
                  isFullWidth: true,
                  isDestructive: true,
                  icon: Icons.stop_circle_outlined,
                  onPressed: () => _showCancelConfirm(context, name),
                ),
            ],
          ),
        ),
      ),
    );
  }

  Widget _buildStatusCard(BuildContext context, String status, String duration, String startTimeStr) {
    return AmitiaCard(
      child: Column(
        crossAxisAlignment: CrossAxisAlignment.start,
        children: [
          Row(
            children: [
              Text('运行状态', style: AppTypography.sectionTitle(context)),
              const Spacer(),
              AmitiaStatusBadge(label: status, type: _statusBadgeType(status)),
            ],
          ),
          SizedBox(height: AppSpacing.md),
          Row(
            children: [
              Icon(Icons.timer_outlined, size: 16, color: context.textTertiary),
              const SizedBox(width: 6),
              Text('耗时 $duration', style: AppTypography.label(context)),
              const SizedBox(width: 16),
              Icon(Icons.schedule, size: 16, color: context.textTertiary),
              const SizedBox(width: 6),
              Text(startTimeStr, style: AppTypography.label(context)),
            ],
          ),
          if (status == '运行中') ...[
            SizedBox(height: AppSpacing.md),
            Row(
              mainAxisAlignment: MainAxisAlignment.spaceBetween,
              children: [
                Text('执行进度', style: AppTypography.caption(context)),
                Text('65%', style: AppTypography.caption(context).copyWith(color: context.accentPrimary, fontWeight: FontWeight.w600)),
              ],
            ),
            SizedBox(height: AppSpacing.xs),
            const AmitiaProgressBar(progress: 0.65),
          ],
        ],
      ),
    );
  }

  Widget _buildInputSection(BuildContext context, String input) {
    return Column(
      crossAxisAlignment: CrossAxisAlignment.start,
      children: [
        Text('输入', style: AppTypography.sectionTitle(context)),
        SizedBox(height: AppSpacing.md),
        AmitiaCard(
          child: Container(
            width: double.infinity,
            padding: const EdgeInsets.all(12),
            decoration: BoxDecoration(
              color: context.surfaceSecondary,
              borderRadius: AppRadius.brSmall,
            ),
            child: Text(input, style: AppTypography.bodySmall(context)),
          ),
        ),
      ],
    );
  }

  Widget _buildOutputSection(BuildContext context, String output) {
    return Column(
      crossAxisAlignment: CrossAxisAlignment.start,
      children: [
        Text('输出', style: AppTypography.sectionTitle(context)),
        SizedBox(height: AppSpacing.md),
        AmitiaCard(
          child: Container(
            width: double.infinity,
            padding: const EdgeInsets.all(12),
            decoration: BoxDecoration(
              color: output.isEmpty ? context.surfaceSecondary : context.accentSoft,
              borderRadius: AppRadius.brSmall,
            ),
            child: Text(
              output.isEmpty ? '(无输出)' : output,
              style: AppTypography.bodySmall(context).copyWith(
                color: output.isEmpty ? context.textTertiary : context.textPrimary,
              ),
            ),
          ),
        ),
      ],
    );
  }

  Widget _buildErrorSection(BuildContext context, String error) {
    return Column(
      crossAxisAlignment: CrossAxisAlignment.start,
      children: [
        Text('错误信息', style: AppTypography.sectionTitle(context).copyWith(color: context.error)),
        SizedBox(height: AppSpacing.md),
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
                child: Text(error, style: AppTypography.bodySmall(context).copyWith(color: context.error)),
              ),
            ],
          ),
        ),
      ],
    );
  }

  Widget _buildToolCallsSection(BuildContext context, List<Map<String, dynamic>> toolCalls) {
    return Column(
      crossAxisAlignment: CrossAxisAlignment.start,
      children: [
        Text('工具调用 (${toolCalls.length})', style: AppTypography.sectionTitle(context)),
        SizedBox(height: AppSpacing.md),
        ...toolCalls.map((call) => Padding(
              padding: EdgeInsets.only(bottom: AppSpacing.sm),
              child: _ToolCallCard(call: call),
            )),
      ],
    );
  }

  void _showCancelConfirm(BuildContext context, String name) {
    showDialog(
      context: context,
      builder: (dialogContext) => AlertDialog(
        backgroundColor: context.surfacePrimary,
        shape: RoundedRectangleBorder(borderRadius: AppRadius.brLarge),
        title: Text('取消任务', style: AppTypography.cardTitle(context)),
        content: Text('确定要取消「$name」吗？正在执行的操作将被中断，已完成的步骤不受影响。', style: AppTypography.bodySmall(context)),
        actions: [
          TextButton(onPressed: () => Navigator.pop(dialogContext), child: Text('继续运行', style: TextStyle(color: context.textSecondary))),
          TextButton(
            onPressed: () {
              Navigator.pop(dialogContext);
              Navigator.pop(context);
              ScaffoldMessenger.of(context).showSnackBar(
                SnackBar(content: Text('$name 已取消'), backgroundColor: context.error),
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
  final Map<String, dynamic> call;

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
    final toolName = (call['toolName'] ?? call['tool_name'] ?? '').toString();
    final status = (call['status'] ?? '').toString();
    final input = (call['input'] ?? '').toString();
    final output = (call['output'] ?? '').toString();
    final duration = (call['duration'] ?? '').toString();

    return Container(
      padding: EdgeInsets.all(AppSpacing.cardPadding),
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
                child: Text(toolName, style: AppTypography.bodySmall(context).copyWith(fontWeight: FontWeight.w600)),
              ),
              AmitiaStatusBadge(label: status, type: _statusBadgeType(status)),
            ],
          ),
          SizedBox(height: AppSpacing.md),
          _InfoRow(label: '输入', value: input),
          SizedBox(height: AppSpacing.sm),
          _InfoRow(label: '输出', value: output),
          SizedBox(height: AppSpacing.sm),
          Row(
            children: [
              Icon(Icons.timer_outlined, size: 14, color: context.textTertiary),
              const SizedBox(width: 4),
              Text('耗时 $duration', style: AppTypography.label(context)),
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
