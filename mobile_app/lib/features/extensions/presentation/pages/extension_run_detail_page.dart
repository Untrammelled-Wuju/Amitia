import 'dart:convert';

import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';

import '../../../../app/app_routes.dart';
import '../../../../app/theme/app_colors.dart';
import '../../../../app/theme/app_radius.dart';
import '../../../../app/theme/app_spacing.dart';
import '../../../../app/theme/app_typography.dart';
import '../../../../core/services/providers.dart';
import '../../../../core/widgets/amitia_misc.dart';
import '../../../../core/widgets/amitia_scaffold.dart';

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
    if (mounted) setState(() { _loading = true; _error = null; });
    try {
      final data = await ref.read(extensionServiceProvider).getExtensionRun(widget.runId);
      if (mounted) setState(() { _run = data; _loading = false; });
    } catch (error) {
      if (mounted) setState(() { _error = error.toString(); _loading = false; });
    }
  }

  @override
  Widget build(BuildContext context) {
    return AmitiaScaffold(
      appBar: AmitiaAppBar(
        title: '执行详情',
        showBackButton: true,
        fallbackRoute: AppRoutes.extensionsRuns,
        actions: [AmitiaIconButton(icon: Icons.refresh, onPressed: _loadRun)],
      ),
      body: SafeArea(
        top: false,
        child: _loading
            ? const AmitiaLoadingState(message: '加载中…')
            : _error != null || _run == null
                ? AmitiaErrorState(message: _error ?? '未找到该执行记录', onRetry: _loadRun)
                : _buildContent(_run!),
      ),
    );
  }

  Widget _buildContent(Map<String, dynamic> run) {
    final status = (run['status'] ?? '').toString();
    final sideEffects = ((run['sideEffects'] as List?) ?? const [])
        .whereType<Map>()
        .map((e) => Map<String, dynamic>.from(e))
        .toList(growable: false);
    return ListView(
      padding: EdgeInsets.all(AppSpacing.pagePadding),
      children: [
        AmitiaCard(
          child: Column(
            crossAxisAlignment: CrossAxisAlignment.start,
            children: [
              Row(
                children: [
                  Expanded(child: Text((run['skillId'] ?? run['extensionId'] ?? widget.runId).toString(), style: AppTypography.sectionTitle(context))),
                  AmitiaStatusBadge(label: _statusLabel(status), type: _statusBadge(status)),
                ],
              ),
              SizedBox(height: AppSpacing.md),
              _row('Run ID', (run['runId'] ?? widget.runId).toString()),
              _row('扩展', (run['extensionId'] ?? '').toString()),
              _row('版本', (run['extensionVersion'] ?? '').toString()),
              _row('角色', (run['characterId'] ?? '').toString()),
              _row('会话', (run['conversationId'] ?? '').toString()),
              _row('渠道', (run['channel'] ?? '').toString()),
              _row('触发器', (run['trigger'] ?? '').toString()),
              _row('开始时间', (run['startedAt'] ?? '').toString()),
              _row('结束时间', (run['finishedAt'] ?? '').toString()),
              _row('耗时', '${(run['durationMs'] as num?)?.toInt() ?? 0} ms'),
              _row('Trace ID', (run['traceId'] ?? '').toString()),
              _row('幂等键', (run['idempotencyKey'] ?? '').toString()),
            ],
          ),
        ),
        SizedBox(height: AppSpacing.md),
        _textSection('输入摘要', (run['inputSummary'] ?? '').toString()),
        SizedBox(height: AppSpacing.md),
        _textSection('输出摘要', (run['outputSummary'] ?? '').toString()),
        if ((run['errorCode'] ?? '').toString().isNotEmpty || (run['errorDetail'] ?? '').toString().isNotEmpty) ...[
          SizedBox(height: AppSpacing.md),
          _errorSection((run['errorCode'] ?? '').toString(), (run['errorDetail'] ?? '').toString()),
        ],
        if (sideEffects.isNotEmpty) ...[
          SizedBox(height: AppSpacing.md),
          Text('副作用 (${sideEffects.length})', style: AppTypography.sectionTitle(context)),
          SizedBox(height: AppSpacing.sm),
          ...sideEffects.map((effect) => Padding(
                padding: EdgeInsets.only(bottom: AppSpacing.sm),
                child: AmitiaCard(
                  child: Row(
                    children: [
                      Icon(effect['confirmed'] == true ? Icons.verified_outlined : Icons.warning_amber_outlined, color: effect['confirmed'] == true ? context.success : context.warning),
                      SizedBox(width: AppSpacing.sm),
                      Expanded(
                        child: Column(
                          crossAxisAlignment: CrossAxisAlignment.start,
                          children: [
                            Text((effect['type'] ?? 'side_effect').toString(), style: AppTypography.cardTitle(context)),
                            if ((effect['targetId'] ?? '').toString().isNotEmpty)
                              Text('目标：${effect['targetId']}', style: AppTypography.caption(context)),
                          ],
                        ),
                      ),
                      AmitiaStatusBadge(label: effect['confirmed'] == true ? '已确认' : '未确认', type: effect['confirmed'] == true ? BadgeType.success : BadgeType.warning),
                    ],
                  ),
                ),
              )),
        ],
        SizedBox(height: AppSpacing.md),
        ExpansionTile(
          tilePadding: EdgeInsets.zero,
          title: Text('原始 RunView', style: AppTypography.cardTitle(context)),
          children: [
            Container(
              width: double.infinity,
              padding: EdgeInsets.all(AppSpacing.md),
              decoration: BoxDecoration(color: context.surfaceSecondary, borderRadius: AppRadius.brMedium),
              child: SelectableText(const JsonEncoder.withIndent('  ').convert(run), style: AppTypography.caption(context)),
            ),
          ],
        ),
        SizedBox(height: AppSpacing.xxl),
      ],
    );
  }

  Widget _row(String label, String value) {
    if (value.trim().isEmpty) return const SizedBox.shrink();
    return Padding(
      padding: const EdgeInsets.only(bottom: 7),
      child: Row(
        crossAxisAlignment: CrossAxisAlignment.start,
        children: [
          SizedBox(width: 86, child: Text(label, style: AppTypography.label(context))),
          Expanded(child: SelectableText(value, style: AppTypography.bodySmall(context))),
        ],
      ),
    );
  }

  Widget _textSection(String title, String value) => Column(
        crossAxisAlignment: CrossAxisAlignment.start,
        children: [
          Text(title, style: AppTypography.sectionTitle(context)),
          SizedBox(height: AppSpacing.sm),
          AmitiaCard(
            child: SizedBox(
              width: double.infinity,
              child: SelectableText(value.isEmpty ? '—' : value, style: AppTypography.bodySmall(context).copyWith(height: 1.5)),
            ),
          ),
        ],
      );

  Widget _errorSection(String code, String detail) => Container(
        padding: EdgeInsets.all(AppSpacing.md),
        decoration: BoxDecoration(
          color: context.error.withValues(alpha: 0.08),
          borderRadius: AppRadius.brMedium,
          border: Border.all(color: context.error.withValues(alpha: 0.3)),
        ),
        child: Column(
          crossAxisAlignment: CrossAxisAlignment.start,
          children: [
            Text(code.isEmpty ? '执行失败' : code, style: AppTypography.cardTitle(context).copyWith(color: context.error)),
            if (detail.isNotEmpty) ...[
              const SizedBox(height: 4),
              SelectableText(detail, style: AppTypography.bodySmall(context)),
            ],
          ],
        ),
      );

  String _statusLabel(String status) {
    switch (status) {
      case 'pending': return '等待中';
      case 'running': return '运行中';
      case 'succeeded': return '已完成';
      case 'failed': return '失败';
      case 'cancelled': return '已取消';
      default: return status.isEmpty ? '未知' : status;
    }
  }

  BadgeType _statusBadge(String status) {
    switch (status) {
      case 'succeeded': return BadgeType.success;
      case 'failed': return BadgeType.error;
      case 'running': return BadgeType.accent;
      case 'pending': return BadgeType.warning;
      default: return BadgeType.neutral;
    }
  }
}
