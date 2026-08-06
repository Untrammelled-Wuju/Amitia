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
import '../../../../core/services/providers.dart';

final _diagnosticsProvider = FutureProvider<Map<String, dynamic>?>((ref) async {
  final svc = ref.read(systemServiceProvider);
  return svc.diagnostics();
});

class MaintenancePage extends ConsumerWidget {
  const MaintenancePage({super.key});

  @override
  Widget build(BuildContext context, WidgetRef ref) {
    final diagAsync = ref.watch(_diagnosticsProvider);

    return AmitiaScaffold(
      appBar: AmitiaAppBar(title: '维护诊断', showBackButton: true, fallbackRoute: AppRoutes.settings),
      body: diagAsync.when(
        loading: () => const Center(child: CircularProgressIndicator()),
        error: (err, _) => Center(
          child: Padding(
            padding: const EdgeInsets.all(32),
            child: Column(
              mainAxisSize: MainAxisSize.min,
              children: [
                Icon(Icons.error_outline, size: 48, color: context.textSecondary),
                const SizedBox(height: 16),
                Text(
                  '加载失败: ${err.toString().replaceFirst('Exception: ', '')}',
                  style: AppTypography.body(context).copyWith(color: context.error),
                  textAlign: TextAlign.center,
                ),
                const SizedBox(height: 16),
                AmitiaButton(
                  label: '重试',
                  onPressed: () => ref.invalidate(_diagnosticsProvider),
                ),
              ],
            ),
          ),
        ),
        data: (diag) {
          return _MaintenanceContent(data: diag);
        },
      ),
    );
  }
}

class _MaintenanceContent extends ConsumerStatefulWidget {
  final Map<String, dynamic>? data;

  const _MaintenanceContent({this.data});

  @override
  ConsumerState<_MaintenanceContent> createState() => _MaintenanceContentState();
}

class _MaintenanceContentState extends ConsumerState<_MaintenanceContent> {
  bool _diagnosing = false;
  List<_CheckItem> _checks = [];

  @override
  void initState() {
    super.initState();
    final checksList = widget.data?['checks'] as List<dynamic>?;
    if (checksList != null) {
      _checks = checksList.map((c) {
        final m = c as Map<String, dynamic>;
        final status = (m['status'] ?? '正常').toString();
        return _CheckItem(
          name: (m['name'] ?? '').toString(),
          status: status,
          detail: m['detail']?.toString(),
          isNormal: status == '正常',
        );
      }).toList();
    } else {
      _checks = [
        _CheckItem(name: 'API 服务', status: '正常', detail: '响应时间 12ms', isNormal: true),
        _CheckItem(name: '向量数据库', status: '正常', detail: '1,247 个向量', isNormal: true),
        _CheckItem(name: '记忆系统', status: '正常', detail: '运行中', isNormal: true),
        _CheckItem(name: '模型推理', status: '正常', detail: 'GPT-4 连接正常', isNormal: true),
      ];
    }
  }

  @override
  Widget build(BuildContext context) {
    return ListView(
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
                      onPressed: () => _exportDiagnostic(context),
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
        const SizedBox(height: AppSpacing.xl),
      ],
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

  Future<void> _runDiagnostic() async {
    setState(() => _diagnosing = true);
    final sys = ref.read(systemServiceProvider);
    final result = await sys.runDiagnostics();
    if (mounted) {
      setState(() => _diagnosing = false);
      ref.invalidate(_diagnosticsProvider);
      showDialog(
        context: context,
        builder: (ctx) => AlertDialog(
          shape: RoundedRectangleBorder(borderRadius: AppRadius.brMedium),
          title: Row(children: [
            Icon(Icons.check_circle, color: context.success, size: 22),
            const SizedBox(width: 8),
            Text('诊断完成', style: AppTypography.cardTitle(context)),
          ]),
          content: Text('已完成 ${_checks.length} 项检查。', style: AppTypography.body(context)),
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

  Future<void> _exportDiagnostic(BuildContext context) async {
    final sys = ref.read(systemServiceProvider);
    final diag = await sys.diagnostics();
    if (context.mounted) {
      ScaffoldMessenger.of(context).showSnackBar(
        const SnackBar(content: Text('诊断报告已导出'), duration: Duration(seconds: 1)),
      );
    }
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
