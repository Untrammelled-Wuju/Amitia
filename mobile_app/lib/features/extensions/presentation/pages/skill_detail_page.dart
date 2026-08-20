import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:go_router/go_router.dart';
import '../../../../app/app_routes.dart';
import '../../../../app/theme/app_colors.dart';
import '../../../../app/theme/app_typography.dart';
import '../../../../app/theme/app_spacing.dart';
import '../../../../app/theme/app_radius.dart';
import '../../../../core/widgets/amitia_scaffold.dart';
import '../../../../core/widgets/amitia_misc.dart';
import '../../../../core/services/providers.dart';

class SkillDetailPage extends ConsumerStatefulWidget {
  final String skillId;

  const SkillDetailPage({super.key, required this.skillId});

  @override
  ConsumerState<SkillDetailPage> createState() => _SkillDetailPageState();
}

class _SkillDetailPageState extends ConsumerState<SkillDetailPage> {
  Map<String, dynamic>? _skill;
  bool _loading = true;
  String? _error;

  @override
  void initState() {
    super.initState();
    _loadSkill();
  }

  Future<void> _loadSkill() async {
    setState(() { _loading = true; _error = null; });
    try {
      final svc = ref.read(extensionServiceProvider);
      final skills = await svc.skills();
      final found = skills.where((s) => (s['id'] ?? '').toString() == widget.skillId).firstOrNull;
      if (mounted) setState(() { _skill = found; _loading = false; });
    } catch (e) {
      if (mounted) setState(() { _error = e.toString(); _loading = false; });
    }
  }

  @override
  Widget build(BuildContext context) {
    if (_loading) {
      return AmitiaScaffold(
        appBar: AmitiaAppBar(title: '技能详情', showBackButton: true, fallbackRoute: AppRoutes.extensions),
        body: SafeArea(top: false, child: const AmitiaLoadingState(message: '加载中...')),
      );
    }
    if (_error != null) {
      return AmitiaScaffold(
        appBar: AmitiaAppBar(title: '技能详情', showBackButton: true, fallbackRoute: AppRoutes.extensions),
        body: SafeArea(top: false, child: AmitiaErrorState(message: _error!, onRetry: _loadSkill)),
      );
    }
    final skill = _skill;
    if (skill == null) {
      return AmitiaScaffold(
        appBar: AmitiaAppBar(title: '技能详情', showBackButton: true, fallbackRoute: AppRoutes.extensions),
        body: SafeArea(top: false, child: Center(child: Text('未找到技能', style: AppTypography.body(context)))),
      );
    }

    final name = (skill['name'] ?? '').toString();
    final description = (skill['description'] ?? '').toString();
    final isInstalled = (skill['isInstalled'] as bool?) ?? ((skill['installed'] as int?) == 1);
    final isEnabled = (skill['isEnabled'] as bool?) ?? ((skill['enabled'] as int?) == 1);

    return AmitiaScaffold(
      appBar: AmitiaAppBar(title: name, showBackButton: true, fallbackRoute: AppRoutes.extensions),
      body: SafeArea(
        top: false,
        child: ListView(
          padding: EdgeInsets.all(AppSpacing.pagePadding),
          children: [
            AmitiaCard(
              child: Column(
                crossAxisAlignment: CrossAxisAlignment.start,
                children: [
                  Row(
                    children: [
                      Container(
                        width: 48,
                        height: 48,
                        decoration: BoxDecoration(color: context.accentSoft, borderRadius: AppRadius.brSmall),
                        child: Icon(Icons.smart_toy_outlined, size: 24, color: context.accentPrimary),
                      ),
                      const SizedBox(width: 12),
                      Expanded(
                        child: Column(
                          crossAxisAlignment: CrossAxisAlignment.start,
                          children: [
                            Text(name, style: AppTypography.cardTitle(context)),
                            Text(description, style: AppTypography.caption(context)),
                          ],
                        ),
                      ),
                    ],
                  ),
                  SizedBox(height: AppSpacing.md),
                  _InfoRow(label: '类型', value: 'Skill'),
                  _InfoRow(label: '状态', value: isInstalled ? '已安装' : '未安装'),
                  _InfoRow(label: '启用', value: isEnabled ? '已启用' : '已禁用'),
                ],
              ),
            ),
            SizedBox(height: AppSpacing.md),
            AmitiaButton(
              label: isInstalled ? '卸载' : '安装',
              isFullWidth: true,
              onPressed: () async {
                try {
                  final svc = ref.read(extensionServiceProvider);
                  if (isInstalled) {
                    await svc.disableSkill(widget.skillId);
                  } else {
                    await svc.enableSkill(widget.skillId);
                  }
                  if (mounted) {
                    ScaffoldMessenger.of(context).showSnackBar(
                      SnackBar(content: Text(isInstalled ? '已卸载' : '已安装'), backgroundColor: context.success),
                    );
                    context.pop();
                  }
                } catch (e) {
                  if (mounted) {
                    ScaffoldMessenger.of(context).showSnackBar(
                      SnackBar(content: Text('操作失败: $e'), backgroundColor: context.error),
                    );
                  }
                }
              },
            ),
          ],
        ),
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
    return Padding(
      padding: const EdgeInsets.symmetric(vertical: 4),
      child: Row(
        mainAxisAlignment: MainAxisAlignment.spaceBetween,
        children: [
          Text(label, style: AppTypography.caption(context)),
          Text(value, style: AppTypography.body(context)),
        ],
      ),
    );
  }
}
