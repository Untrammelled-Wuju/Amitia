import 'package:flutter/material.dart';
import 'package:flutter/services.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';

import '../../../../app/app_routes.dart';
import '../../../../app/theme/app_colors.dart';
import '../../../../app/theme/app_radius.dart';
import '../../../../app/theme/app_spacing.dart';
import '../../../../app/theme/app_typography.dart';
import '../../../../core/services/providers.dart';
import '../../../../core/widgets/amitia_button.dart';
import '../../../../core/widgets/amitia_misc.dart';
import '../../../../core/widgets/amitia_scaffold.dart';

final _aboutInfoProvider = FutureProvider<Map<String, dynamic>>((ref) async {
  final service = ref.read(systemServiceProvider);
  final values = await Future.wait([service.version(), service.about()]);
  return {'version': values[0] ?? <String, dynamic>{}, 'about': values[1] ?? <String, dynamic>{}};
});

class AboutPageNew extends ConsumerWidget {
  const AboutPageNew({super.key});

  @override
  Widget build(BuildContext context, WidgetRef ref) {
    final info = ref.watch(_aboutInfoProvider);
    return AmitiaScaffold(
      appBar: AmitiaAppBar(
        title: '关于',
        showBackButton: true,
        fallbackRoute: AppRoutes.settings,
        actions: [AmitiaIconButton(icon: Icons.refresh, tooltip: '刷新', onPressed: () => ref.invalidate(_aboutInfoProvider))],
      ),
      body: info.when(
        loading: () => const Center(child: CircularProgressIndicator()),
        error: (error, _) => Center(child: Text('加载版本信息失败：$error')),
        data: (value) => _content(context, value),
      ),
    );
  }

  Widget _content(BuildContext context, Map<String, dynamic> value) {
    final version = value['version'] as Map<String, dynamic>? ?? const {};
    final about = value['about'] as Map<String, dynamic>? ?? const {};
    final versionText = (version['version'] ?? about['version'] ?? 'unknown').toString();
    final sourceUrl = (about['sourceCodeUrl'] ?? '').toString();
    final license = (about['license'] ?? 'AGPL-3.0-only').toString();
    return ListView(
      padding: EdgeInsets.symmetric(vertical: AppSpacing.xxl),
      children: [
        Center(
          child: Container(
            width: 96,
            height: 96,
            decoration: BoxDecoration(
              gradient: LinearGradient(colors: [context.accentPrimary, context.accentSecondary], begin: Alignment.topLeft, end: Alignment.bottomRight),
              shape: BoxShape.circle,
            ),
            child: const Icon(Icons.auto_awesome, size: 48, color: Colors.white),
          ),
        ),
        SizedBox(height: AppSpacing.lg),
        Center(child: Text((about['name'] ?? 'Amitia').toString(), style: AppTypography.pageLargeTitle(context))),
        const SizedBox(height: 6),
        Center(child: AmitiaStatusBadge(label: 'v$versionText', type: BadgeType.accent)),
        SizedBox(height: AppSpacing.sectionGap),
        _label(context, '应用信息'),
        AmitiaCard(
          margin: EdgeInsets.symmetric(horizontal: AppSpacing.pagePadding),
          child: Column(children: [
            _row(context, '显示名称', about['displayName'] ?? 'Amitia'),
            _row(context, '版本', versionText),
            _row(context, '构建时间', version['buildTime'] ?? '—'),
            _row(context, 'Go 版本', version['goVersion'] ?? '—'),
            _row(context, 'Git Commit', (about['gitCommit'] ?? '').toString().isEmpty ? '未注入' : about['gitCommit']),
            _row(context, '开源协议', license),
            _row(context, '版权', about['copyright'] ?? 'Copyright (C) 2026 彭旭'),
          ]),
        ),
        if (sourceUrl.isNotEmpty) ...[
          SizedBox(height: AppSpacing.md),
          Padding(
            padding: EdgeInsets.symmetric(horizontal: AppSpacing.pagePadding),
            child: Column(children: [
              AmitiaButton(
                label: '复制项目地址',
                icon: Icons.content_copy,
                isSecondary: true,
                isFullWidth: true,
                onPressed: () async {
                  await Clipboard.setData(ClipboardData(text: sourceUrl));
                  if (context.mounted) ScaffoldMessenger.of(context).showSnackBar(const SnackBar(content: Text('项目地址已复制')));
                },
              ),
            ]),
          ),
        ],
        SizedBox(height: AppSpacing.xl),
      ],
    );
  }

  Widget _row(BuildContext context, String label, Object? value) => Padding(
        padding: EdgeInsets.symmetric(vertical: AppSpacing.sm),
        child: Row(crossAxisAlignment: CrossAxisAlignment.start, children: [
          Expanded(child: Text(label, style: AppTypography.body(context))),
          Flexible(child: Text('${value ?? '—'}', textAlign: TextAlign.right, style: AppTypography.caption(context))),
        ]),
      );

  Widget _label(BuildContext context, String text) => Padding(
        padding: EdgeInsets.fromLTRB(AppSpacing.pagePadding, AppSpacing.sm, AppSpacing.pagePadding, AppSpacing.sm),
        child: Text(text, style: AppTypography.caption(context)),
      );
}
