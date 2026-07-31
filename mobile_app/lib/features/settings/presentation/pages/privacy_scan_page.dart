import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
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

class PrivacyScanPage extends ConsumerStatefulWidget {
  const PrivacyScanPage({super.key});

  @override
  ConsumerState<PrivacyScanPage> createState() => _PrivacyScanPageState();
}

class _PrivacyScanPageState extends ConsumerState<PrivacyScanPage> {
  int _scanRangeIndex = 0;
  bool _scanning = false;
  double _scanProgress = 0;
  List<PrivacyScanResult> _results = [];

  static const _scanRanges = ['全部数据', '对话记录', '记忆数据', '用户画像'];
  static const _history = [
    ('2026-07-29 16:00', '发现 6 项风险', 'warning'),
    ('2026-07-25 10:30', '未发现风险', 'success'),
    ('2026-07-20 14:00', '发现 3 项风险', 'warning'),
  ];

  @override
  void initState() {
    super.initState();
    _results = List.from(MockSettings.privacyScanResults);
  }

  @override
  Widget build(BuildContext context) {
    return AmitiaScaffold(
      appBar: AmitiaAppBar(title: '隐私扫描', showBackButton: true, fallbackRoute: AppRoutes.settings),
      body: ListView(
        padding: const EdgeInsets.symmetric(vertical: AppSpacing.md),
        children: [
          _SectionLabel(text: '扫描范围'),
          const SizedBox(height: AppSpacing.sm),
          Padding(
            padding: const EdgeInsets.symmetric(horizontal: AppSpacing.pagePadding),
            child: AmitiaSegmentedControl(
              segments: _scanRanges,
              selectedIndex: _scanRangeIndex,
              onChanged: (i) => setState(() => _scanRangeIndex = i),
            ),
          ),
          const SizedBox(height: AppSpacing.sectionGap),
          Padding(
            padding: const EdgeInsets.symmetric(horizontal: AppSpacing.pagePadding),
            child: AmitiaButton(
              label: _scanning ? '扫描中...' : '开始扫描',
              icon: Icons.radar,
              isFullWidth: true,
              onPressed: _scanning ? null : _startScan,
            ),
          ),
          if (_scanning || _scanProgress > 0) ...[
            const SizedBox(height: AppSpacing.md),
            Padding(
              padding: const EdgeInsets.symmetric(horizontal: AppSpacing.pagePadding),
              child: Column(
                children: [
                  AmitiaProgressBar(progress: _scanProgress, height: 8),
                  const SizedBox(height: 4),
                  Text('${(_scanProgress * 100).round()}%', style: AppTypography.label(context)),
                ],
              ),
            ),
          ],
          const SizedBox(height: AppSpacing.sectionGap),
          AmitiaSectionHeader(
            title: '风险结果 (${_results.length})',
            actionText: '脱敏',
            onAction: _confirmDesensitize,
          ),
          const SizedBox(height: AppSpacing.sm),
          ..._results.map((r) => _buildResultTile(r)),
          const SizedBox(height: AppSpacing.sectionGap),
          _SectionLabel(text: '扫描历史'),
          const SizedBox(height: AppSpacing.sm),
          ..._history.map((h) => _buildHistoryTile(h.$1, h.$2, h.$3)),
          const SizedBox(height: AppSpacing.xl),
        ],
      ),
    );
  }

  Widget _buildResultTile(PrivacyScanResult result) {
    final (type, color) = switch (result.riskLevel) {
      '高风险' => (BadgeType.error, context.error),
      '中风险' => (BadgeType.warning, context.warning),
      '低风险' => (BadgeType.info, context.info),
      _ => (BadgeType.success, context.success),
    };
    return Container(
      margin: const EdgeInsets.only(left: AppSpacing.pagePadding, right: AppSpacing.pagePadding, bottom: AppSpacing.sm),
      padding: const EdgeInsets.symmetric(horizontal: AppSpacing.lg, vertical: 12),
      decoration: BoxDecoration(
        color: context.surfacePrimary,
        borderRadius: AppRadius.brMedium,
        border: Border.all(color: context.borderPrimary, width: 0.5),
      ),
      child: Row(
        children: [
          Container(
            width: 36,
            height: 36,
            decoration: BoxDecoration(
              color: color.withValues(alpha: 0.12),
              borderRadius: AppRadius.brSmall,
            ),
            child: Icon(Icons.privacy_tip_outlined, size: 18, color: color),
          ),
          const SizedBox(width: 12),
          Expanded(
            child: Column(
              crossAxisAlignment: CrossAxisAlignment.start,
              children: [
                Text(result.category, style: AppTypography.body(context)),
                Text('发现 ${result.riskCount} 项风险', style: AppTypography.label(context)),
              ],
            ),
          ),
          AmitiaStatusBadge(label: result.riskLevel, type: type),
        ],
      ),
    );
  }

  Widget _buildHistoryTile(String time, String desc, String level) {
    final (color, icon) = switch (level) {
      'warning' => (context.warning, Icons.warning_amber_outlined),
      'error' => (context.error, Icons.error_outline),
      _ => (context.success, Icons.check_circle_outline),
    };
    return Container(
      margin: const EdgeInsets.only(left: AppSpacing.pagePadding, right: AppSpacing.pagePadding, bottom: AppSpacing.sm),
      padding: const EdgeInsets.symmetric(horizontal: AppSpacing.lg, vertical: 12),
      decoration: BoxDecoration(
        color: context.surfacePrimary,
        borderRadius: AppRadius.brMedium,
        border: Border.all(color: context.borderPrimary, width: 0.5),
      ),
      child: Row(
        children: [
          Icon(icon, size: 20, color: color),
          const SizedBox(width: 12),
          Expanded(
            child: Column(
              crossAxisAlignment: CrossAxisAlignment.start,
              children: [
                Text(desc, style: AppTypography.body(context)),
                Text(time, style: AppTypography.label(context)),
              ],
            ),
          ),
        ],
      ),
    );
  }

  Future<void> _startScan() async {
    setState(() {
      _scanning = true;
      _scanProgress = 0;
    });
    for (int i = 1; i <= 10; i++) {
      await Future.delayed(const Duration(milliseconds: 200));
      if (mounted) setState(() => _scanProgress = i / 10);
    }
    if (mounted) {
      setState(() => _scanning = false);
      ScaffoldMessenger.of(context).showSnackBar(
        const SnackBar(content: Text('扫描完成'), duration: Duration(seconds: 1)),
      );
    }
  }

  void _confirmDesensitize() {
    showDialog(
      context: context,
      builder: (ctx) => AlertDialog(
        shape: RoundedRectangleBorder(borderRadius: AppRadius.brMedium),
        title: Text('脱敏操作', style: AppTypography.cardTitle(context)),
        content: Text('将对 ${_results.where((r) => r.riskCount > 0).length} 项风险数据进行脱敏处理，处理后的数据将无法还原。是否继续？', style: AppTypography.body(context)),
        actions: [
          TextButton(onPressed: () => Navigator.pop(ctx), child: const Text('取消')),
          TextButton(
            onPressed: () {
              Navigator.pop(ctx);
              setState(() {
                _results = _results.map((r) => PrivacyScanResult(category: r.category, riskCount: 0, riskLevel: '安全')).toList();
              });
              ScaffoldMessenger.of(context).showSnackBar(
                const SnackBar(content: Text('脱敏完成'), duration: Duration(seconds: 1)),
              );
            },
            child: Text('确认脱敏', style: TextStyle(color: context.accentPrimary)),
          ),
        ],
      ),
    );
  }
}

class _SectionLabel extends StatelessWidget {
  final String text;
  const _SectionLabel({required this.text});

  @override
  Widget build(BuildContext context) {
    return Padding(
      padding: const EdgeInsets.fromLTRB(AppSpacing.pagePadding, AppSpacing.sm, AppSpacing.pagePadding, AppSpacing.sm),
      child: Text(text, style: AppTypography.caption(context)),
    );
  }
}
