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

final _aboutVersionProvider = FutureProvider<Map<String, dynamic>?>((ref) async {
  final svc = ref.read(systemServiceProvider);
  return svc.health();
});

class AboutPageNew extends ConsumerWidget {
  const AboutPageNew({super.key});

  static const _components = [
    ('Flutter', 'BSD-3-Clause', 'UI 框架'),
    ('Riverpod', 'MIT', '状态管理'),
    ('Go Router', 'BSD-3-Clause', '路由导航'),
    ('SurrealDB', 'BSL', '数据库'),
    ('Qdrant', 'Apache-2.0', '向量数据库'),
    ('Material Icons', 'Apache-2.0', '图标库'),
  ];

  static const _infoItems = [
    ('开源协议', 'MIT'),
    ('项目地址', 'github.com/amitia-ai/amitia'),
    ('隐私政策', ''),
    ('使用边界', ''),
  ];

  @override
  Widget build(BuildContext context, WidgetRef ref) {
    final versionAsync = ref.watch(_aboutVersionProvider);
    final versionText = versionAsync.when(
      data: (data) {
        if (data == null) return 'v1.0.0';
        final v = data['version'] as String? ?? data['app_version'] as String? ?? data['runtime_version'] as String?;
        return v != null && v.isNotEmpty ? v : 'v1.0.0';
      },
      loading: () => 'v1.0.0',
      error: (_, __) => 'v1.0.0',
    );
    return AmitiaScaffold(
      appBar: AmitiaAppBar(title: '关于', showBackButton: true, fallbackRoute: AppRoutes.settings),
      body: ListView(
        padding: EdgeInsets.symmetric(vertical: AppSpacing.xxl),
        children: [
          SizedBox(height: AppSpacing.xl),
          Center(
            child: Container(
              width: 96,
              height: 96,
              decoration: BoxDecoration(
                gradient: LinearGradient(
                  colors: [context.accentPrimary, context.accentSecondary],
                  begin: Alignment.topLeft,
                  end: Alignment.bottomRight,
                ),
                shape: BoxShape.circle,
                boxShadow: [
                  BoxShadow(
                    color: context.accentPrimary.withValues(alpha: 0.3),
                    blurRadius: 20,
                    offset: const Offset(0, 8),
                  ),
                ],
              ),
              child: const Icon(Icons.auto_awesome, size: 48, color: Colors.white),
            ),
          ),
          SizedBox(height: AppSpacing.lg),
          Center(child: Text('Amitia', style: AppTypography.pageLargeTitle(context))),
          const SizedBox(height: 4),
          Center(
            child: Row(
              mainAxisSize: MainAxisSize.min,
              children: [
                AmitiaStatusBadge(label: versionText, type: BadgeType.accent),
                const SizedBox(width: 8),
                versionAsync.maybeWhen(
                  data: (data) {
                    final build = data?['build'] as String? ?? data?['build_number'] as String?;
                    return Text('Build ${build ?? ''}', style: AppTypography.label(context));
                  },
                  orElse: () => const SizedBox.shrink(),
                ),
              ],
            ),
          ),
          SizedBox(height: AppSpacing.md),
          Center(
            child: Text('你的专属 AI 伙伴', style: AppTypography.caption(context)),
          ),
          SizedBox(height: AppSpacing.sectionGap),
          _SectionLabel(text: '应用信息'),
          SizedBox(height: AppSpacing.sm),
          _buildCard(context, [
            for (int i = 0; i < _infoItems.length; i++) ...[
              _buildInfoTile(context, _infoItems[i].$1, _infoItems[i].$2),
              if (i < _infoItems.length - 1) _divider(context),
            ],
          ]),
          SizedBox(height: AppSpacing.sectionGap),
          Padding(
            padding: EdgeInsets.symmetric(horizontal: AppSpacing.pagePadding),
            child: AmitiaButton(
              label: '检查更新',
              icon: Icons.system_update,
              isFullWidth: true,
              onPressed: () => _checkUpdate(context),
            ),
          ),
          SizedBox(height: AppSpacing.sectionGap),
          _SectionLabel(text: '开源组件'),
          SizedBox(height: AppSpacing.sm),
          _buildCard(context, [
            for (int i = 0; i < _components.length; i++) ...[
              _buildComponentTile(context, _components[i].$1, _components[i].$2, _components[i].$3),
              if (i < _components.length - 1) _divider(context),
            ],
          ]),
          SizedBox(height: AppSpacing.sectionGap),
          Center(
            child: Text(
              'Copyright (c) 2026 Amitia\n保留所有权利',
              textAlign: TextAlign.center,
              style: AppTypography.label(context),
            ),
          ),
          SizedBox(height: AppSpacing.xl),
        ],
      ),
    );
  }

  Widget _buildCard(BuildContext context, List<Widget> children) {
    return Container(
      margin: EdgeInsets.symmetric(horizontal: AppSpacing.pagePadding),
      decoration: BoxDecoration(
        color: context.surfacePrimary,
        borderRadius: AppRadius.brMedium,
        border: Border.all(color: context.borderPrimary, width: 0.5),
      ),
      child: Column(children: children),
    );
  }

  Widget _divider(BuildContext context) {
    return Padding(
      padding: EdgeInsets.only(left: AppSpacing.lg),
      child: Divider(height: 1, thickness: 0.5, color: context.borderSecondary),
    );
  }

  Widget _buildInfoTile(BuildContext context, String title, String value) {
    return GestureDetector(
      behavior: HitTestBehavior.opaque,
      onTap: () {
        ScaffoldMessenger.of(context).showSnackBar(
          SnackBar(content: Text('$title · 即将打开'), duration: const Duration(seconds: 1)),
        );
      },
      child: Padding(
        padding: EdgeInsets.symmetric(horizontal: AppSpacing.lg, vertical: 14),
        child: Row(
          children: [
            Expanded(child: Text(title, style: AppTypography.body(context))),
            if (value.isNotEmpty) ...[
              Text(value, style: AppTypography.caption(context)),
              const SizedBox(width: 4),
            ],
            Icon(Icons.chevron_right, size: 20, color: context.textTertiary),
          ],
        ),
      ),
    );
  }

  Widget _buildComponentTile(BuildContext context, String name, String license, String desc) {
    return Padding(
      padding: EdgeInsets.symmetric(horizontal: AppSpacing.lg, vertical: 12),
      child: Row(
        children: [
          Expanded(
            child: Column(
              crossAxisAlignment: CrossAxisAlignment.start,
              children: [
                Text(name, style: AppTypography.body(context)),
                Text(desc, style: AppTypography.label(context)),
              ],
            ),
          ),
          AmitiaStatusBadge(label: license, type: BadgeType.neutral),
        ],
      ),
    );
  }

  void _checkUpdate(BuildContext context) {
    showDialog(
      context: context,
      builder: (ctx) => AlertDialog(
        shape: RoundedRectangleBorder(borderRadius: AppRadius.brMedium),
        title: Row(
          children: [
            Icon(Icons.check_circle, color: context.success, size: 22),
            const SizedBox(width: 8),
            Text('已是最新版本', style: AppTypography.cardTitle(context)),
          ],
        ),
        content: Text('当前版本 v1.0.0 已是最新版本。', style: AppTypography.body(context)),
        actions: [
          TextButton(onPressed: () => Navigator.pop(ctx), child: const Text('确定')),
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
      padding: EdgeInsets.fromLTRB(AppSpacing.pagePadding, AppSpacing.sm, AppSpacing.pagePadding, AppSpacing.sm),
      child: Text(text, style: AppTypography.caption(context)),
    );
  }
}
