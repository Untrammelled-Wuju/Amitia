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
import '../../../../shared/mock_data/mock_data.dart';

class MaintenancePage extends ConsumerStatefulWidget {
  const MaintenancePage({super.key});

  @override
  ConsumerState<MaintenancePage> createState() => _MaintenancePageState();
}

class _MaintenancePageState extends ConsumerState<MaintenancePage> {
  bool _diagnosing = false;
  bool _diagnosed = false;
  late List<_CheckItem> _checks;

  static const _recentErrors = [
    ('MCP 连接超时', '2026-07-30 08:15', 'warning'),
    ('向量检索失败', '2026-07-29 14:22', 'error'),
  ];
  static const _exportHistory = [
    ('诊断报告_20260729.zip', '128 KB'),
    ('诊断报告_20260725.zip', '112 KB'),
  ];

  @override
  void initState() {
    super.initState();
    _checks = MockSettings.maintenanceChecks.map((c) => _CheckItem(
      name: c.name,
      status: c.status,
      detail: c.detail,
      isNormal: c.status == '正常',
    )).toList();
  }

  @override
  Widget build(BuildContext context) {
    return AmitiaScaffold(
      appBar: AmitiaAppBar(title: '维护诊断', showBackButton: true, fallbackRoute: AppRoutes.settings),
      body: ListView(
        padding: const EdgeInsets.symmetric(vertical: AppSpacing.md),
        children: [
          _SectionLabel(text: '服务状态'),
          const SizedBox(height: AppSpacing.sm),
          _buildCard(
            _checks.map((c) => Column(children: [
              _buildStatusTile(c),
              if (c != _checks.last) _divider(),
            ])).expand((w) => [w]).toList(),
          ),
          const SizedBox(height: AppSpacing.sectionGap),
          _SectionLabel(text: '诊断操作'),
          const SizedBox(height: AppSpacing.sm),
          Padding(
            padding: const EdgeInsets.symmetric(horizontal: AppSpacing.pagePadding),
            child: Column(
              children: [
                AmitiaButton(
                  label: _diagnosing ? '诊断中...' : '运行诊断检查',
                  icon: Icons.health_and_safety_outlined,
                  isFullWidth: true,
                  onPressed: _diagnosing ? null : _runDiagnostic,
                ),
                const SizedBox(height: AppSpacing.sm),
                Row(
                  children: [
                    Expanded(
                      child: AmitiaButton(
                        label: '修复问题',
                        isSecondary: true,
                        icon: Icons.build_outlined,
                        onPressed: _confirmRepair,
                      ),
                    ),
                    const SizedBox(width: AppSpacing.sm),
                    Expanded(
                      child: AmitiaButton(
                        label: '导出诊断',
                        isSecondary: true,
                        icon: Icons.file_download_outlined,
                        onPressed: _exportDiagnostic,
                      ),
                    ),
                  ],
                ),
              ],
            ),
          ),
          const SizedBox(height: AppSpacing.sectionGap),
          _SectionLabel(text: '数据一致性'),
          const SizedBox(height: AppSpacing.sm),
          _buildCard([
            _buildInfoTile('数据库完整性', '通过', BadgeType.success),
            _divider(),
            _buildInfoTile('索引一致性', '通过', BadgeType.success),
            _divider(),
            _buildInfoTile('缓存状态', '83 MB · 正常', BadgeType.info),
          ]),
          const SizedBox(height: AppSpacing.sectionGap),
          _SectionLabel(text: '最近错误 (${_recentErrors.length})'),
          const SizedBox(height: AppSpacing.sm),
          ..._recentErrors.map((e) => _buildErrorTile(e.$1, e.$2, e.$3)),
          const SizedBox(height: AppSpacing.sectionGap),
          _SectionLabel(text: '导出历史'),
          const SizedBox(height: AppSpacing.sm),
          ..._exportHistory.map((e) => _buildExportTile(e.$1, e.$2)),
          const SizedBox(height: AppSpacing.xl),
        ],
      ),
    );
  }

  Widget _buildCard(List<Widget> children) {
    return Container(
      margin: const EdgeInsets.symmetric(horizontal: AppSpacing.pagePadding),
      decoration: BoxDecoration(
        color: context.surfacePrimary,
        borderRadius: AppRadius.brMedium,
        border: Border.all(color: context.borderPrimary, width: 0.5),
      ),
      child: Column(children: children),
    );
  }

  Widget _divider() {
    return Padding(
      padding: const EdgeInsets.only(left: 56),
      child: Divider(height: 1, thickness: 0.5, color: context.borderSecondary),
    );
  }

  Widget _buildStatusTile(_CheckItem check) {
    final isNormal = check.isNormal;
    return Padding(
      padding: const EdgeInsets.symmetric(horizontal: AppSpacing.lg, vertical: 13),
      child: Row(
        children: [
          Icon(
            isNormal ? Icons.check_circle : Icons.error_outline,
            size: 22,
            color: isNormal ? context.success : context.warning,
          ),
          const SizedBox(width: 12),
          Expanded(
            child: Column(
              crossAxisAlignment: CrossAxisAlignment.start,
              children: [
                Text(check.name, style: AppTypography.body(context)),
                if (check.detail != null)
                  Text(check.detail!, style: AppTypography.label(context)),
              ],
            ),
          ),
          AmitiaStatusBadge(
            label: check.status,
            type: isNormal ? BadgeType.success : BadgeType.warning,
          ),
        ],
      ),
    );
  }

  Widget _buildInfoTile(String title, String value, BadgeType type) {
    return Padding(
      padding: const EdgeInsets.symmetric(horizontal: AppSpacing.lg, vertical: 13),
      child: Row(
        children: [
          Expanded(child: Text(title, style: AppTypography.body(context))),
          AmitiaStatusBadge(label: value, type: type),
        ],
      ),
    );
  }

  Widget _buildErrorTile(String title, String time, String level) {
    final (color, icon) = switch (level) {
      'warning' => (context.warning, Icons.warning_amber_outlined),
      'error' => (context.error, Icons.error_outline),
      _ => (context.info, Icons.info_outline),
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
                Text(title, style: AppTypography.body(context)),
                Text(time, style: AppTypography.label(context)),
              ],
            ),
          ),
        ],
      ),
    );
  }

  Widget _buildExportTile(String name, String size) {
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
          Icon(Icons.archive_outlined, size: 20, color: context.accentPrimary),
          const SizedBox(width: 12),
          Expanded(
            child: Column(
              crossAxisAlignment: CrossAxisAlignment.start,
              children: [
                Text(name, style: AppTypography.body(context)),
                Text(size, style: AppTypography.label(context)),
              ],
            ),
          ),
          Icon(Icons.download, size: 18, color: context.textTertiary),
        ],
      ),
    );
  }

  Future<void> _runDiagnostic() async {
    setState(() => _diagnosing = true);
    await Future.delayed(const Duration(milliseconds: 2000));
    if (mounted) {
      setState(() {
        _diagnosing = false;
        _diagnosed = true;
      });
      showDialog(
        context: context,
        builder: (ctx) => AlertDialog(
          shape: RoundedRectangleBorder(borderRadius: AppRadius.brMedium),
          title: Row(children: [
            Icon(Icons.check_circle, color: context.success, size: 22),
            const SizedBox(width: 8),
            Text('诊断完成', style: AppTypography.cardTitle(context)),
          ]),
          content: Text('已完成 ${_checks.length} 项检查，发现 1 个异常。', style: AppTypography.body(context)),
          actions: [
            TextButton(onPressed: () => Navigator.pop(ctx), child: const Text('关闭')),
          ],
        ),
      );
    }
  }

  void _confirmRepair() {
    showDialog(
      context: context,
      builder: (ctx) => AlertDialog(
        shape: RoundedRectangleBorder(borderRadius: AppRadius.brMedium),
        title: Text('修复问题', style: AppTypography.cardTitle(context)),
        content: Text('将尝试修复检测到的异常项，是否继续？', style: AppTypography.body(context)),
        actions: [
          TextButton(onPressed: () => Navigator.pop(ctx), child: const Text('取消')),
          TextButton(
            onPressed: () {
              Navigator.pop(ctx);
              setState(() {
                for (int i = 0; i < _checks.length; i++) {
                  _checks[i] = _CheckItem(name: _checks[i].name, status: '正常', isNormal: true);
                }
              });
              ScaffoldMessenger.of(context).showSnackBar(
                const SnackBar(content: Text('修复完成'), duration: Duration(seconds: 1)),
              );
            },
            child: Text('修复', style: TextStyle(color: context.accentPrimary)),
          ),
        ],
      ),
    );
  }

  void _exportDiagnostic() {
    showDialog(
      context: context,
      builder: (ctx) => AlertDialog(
        shape: RoundedRectangleBorder(borderRadius: AppRadius.brMedium),
        title: Text('导出诊断', style: AppTypography.cardTitle(context)),
        content: Text('诊断报告将导出为 ZIP 文件，包含日志、配置和状态信息。', style: AppTypography.body(context)),
        actions: [
          TextButton(onPressed: () => Navigator.pop(ctx), child: const Text('取消')),
          TextButton(
            onPressed: () {
              Navigator.pop(ctx);
              ScaffoldMessenger.of(context).showSnackBar(
                const SnackBar(content: Text('诊断报告已导出'), duration: Duration(seconds: 1)),
              );
            },
            child: Text('导出', style: TextStyle(color: context.accentPrimary)),
          ),
        ],
      ),
    );
  }
}

class _CheckItem {
  final String name;
  final String status;
  final String? detail;
  final bool isNormal;

  _CheckItem({required this.name, required this.status, this.detail, required this.isNormal});
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
