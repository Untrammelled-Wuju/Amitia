import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import '../../../../app/theme/app_colors.dart';
import '../../../../app/theme/app_typography.dart';
import '../../../../app/theme/app_spacing.dart';
import '../../../../app/theme/app_radius.dart';
import '../../../../core/widgets/amitia_scaffold.dart';

class AboutPage extends ConsumerWidget {
  const AboutPage({super.key});

  @override
  Widget build(BuildContext context, WidgetRef ref) {
    final items = <(String, String)>[
      ('开源协议', 'MIT'),
      ('GitHub', 'amitia-ai/amitia'),
      ('隐私政策', ''),
      ('用户协议', ''),
      ('检查更新', ''),
      ('开源组件', ''),
    ];

    return AmitiaScaffold(
      appBar: AmitiaAppBar(title: '关于', showBackButton: true),
      body: ListView(
        padding: const EdgeInsets.symmetric(vertical: AppSpacing.xxl),
        children: [
          const SizedBox(height: AppSpacing.xl),
          Center(
            child: Container(
              width: 88,
              height: 88,
              decoration: BoxDecoration(
                color: context.accentSoft,
                shape: BoxShape.circle,
              ),
              child: Icon(
                Icons.auto_awesome,
                size: 44,
                color: context.accentPrimary,
              ),
            ),
          ),
          const SizedBox(height: AppSpacing.lg),
          Center(
            child: Text('Amitia', style: AppTypography.pageLargeTitle(context)),
          ),
          const SizedBox(height: 4),
          Center(
            child: Text('v1.0.0', style: AppTypography.caption(context)),
          ),
          const SizedBox(height: AppSpacing.sectionGap),
          Container(
            margin: const EdgeInsets.symmetric(horizontal: AppSpacing.pagePadding),
            decoration: BoxDecoration(
              color: context.surfacePrimary,
              borderRadius: AppRadius.brMedium,
              border: Border.all(color: context.borderPrimary, width: 0.5),
            ),
            child: Column(
              children: [
                for (int i = 0; i < items.length; i++) ...[
                  _AboutTile(title: items[i].$1, value: items[i].$2),
                  if (i < items.length - 1)
                    Padding(
                      padding: const EdgeInsets.only(left: AppSpacing.lg),
                      child: Divider(
                        height: 1,
                        thickness: 0.5,
                        color: context.borderSecondary,
                      ),
                    ),
                ],
              ],
            ),
          ),
          const SizedBox(height: AppSpacing.sectionGap),
          Center(
            child: Text(
              'Copyright © 2026 Amitia\n保留所有权利',
              textAlign: TextAlign.center,
              style: AppTypography.label(context),
            ),
          ),
        ],
      ),
    );
  }
}

class _AboutTile extends StatelessWidget {
  final String title;
  final String value;

  const _AboutTile({required this.title, required this.value});

  @override
  Widget build(BuildContext context) {
    return GestureDetector(
      behavior: HitTestBehavior.opaque,
      onTap: () {
        ScaffoldMessenger.of(context).showSnackBar(
          SnackBar(
            content: Text('$title · 即将打开'),
            duration: const Duration(seconds: 1),
          ),
        );
      },
      child: Padding(
        padding: const EdgeInsets.symmetric(
          horizontal: AppSpacing.lg,
          vertical: 14,
        ),
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
}
