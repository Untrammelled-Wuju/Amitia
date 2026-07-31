import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:go_router/go_router.dart';
import '../../../../app/app_routes.dart';
import '../../../../app/theme/app_colors.dart';
import '../../../../app/theme/app_typography.dart';
import '../../../../app/theme/app_spacing.dart';
import '../../../../app/theme/app_radius.dart';
import '../../../../core/widgets/amitia_scaffold.dart';

class ModelSettingsPage extends ConsumerWidget {
  const ModelSettingsPage({super.key});

  static const _types = <(IconData, String, String, String)>[
    (Icons.chat_outlined, '文本模型', '对话与文本生成', 'text'),
    (Icons.visibility_outlined, '视觉模型', '图像理解与描述', 'vision'),
    (Icons.record_voice_over_outlined, '语音模型', '语音合成与识别', 'voice'),
    (Icons.scatter_plot_outlined, '向量模型', '文本向量化与检索', 'vector'),
    (Icons.image_outlined, '图像生成模型', '文生图与图像编辑', 'image'),
  ];

  @override
  Widget build(BuildContext context, WidgetRef ref) {
    return AmitiaScaffold(
      appBar: AmitiaAppBar(title: '模型设置', showBackButton: true, fallbackRoute: AppRoutes.settings),
      body: ListView(
        padding: const EdgeInsets.all(AppSpacing.pagePadding),
        children: [
          Text('选择模型类型进行配置', style: AppTypography.caption(context)),
          const SizedBox(height: AppSpacing.lg),
          ..._types.map((t) => Padding(
                padding: const EdgeInsets.only(bottom: AppSpacing.md),
                child: _ModelTypeCard(
                  icon: t.$1,
                  title: t.$2,
                  subtitle: t.$3,
                  onTap: () => context.push(AppRoutes.modelConfig(t.$4)),
                ),
              )),
          const SizedBox(height: AppSpacing.xl),
        ],
      ),
    );
  }
}

class _ModelTypeCard extends StatelessWidget {
  final IconData icon;
  final String title;
  final String subtitle;
  final VoidCallback onTap;

  const _ModelTypeCard({
    required this.icon,
    required this.title,
    required this.subtitle,
    required this.onTap,
  });

  @override
  Widget build(BuildContext context) {
    return GestureDetector(
      onTap: onTap,
      child: Container(
        padding: const EdgeInsets.all(AppSpacing.cardPadding),
        decoration: BoxDecoration(
          color: context.surfacePrimary,
          borderRadius: AppRadius.brMedium,
          border: Border.all(color: context.borderPrimary, width: 0.5),
        ),
        child: Row(
          children: [
            Container(
              width: 44,
              height: 44,
              decoration: BoxDecoration(
                color: context.accentSoft,
                borderRadius: AppRadius.brSmall,
              ),
              child: Icon(icon, size: 22, color: context.accentPrimary),
            ),
            const SizedBox(width: 12),
            Expanded(
              child: Column(
                crossAxisAlignment: CrossAxisAlignment.start,
                children: [
                  Text(title, style: AppTypography.cardTitle(context)),
                  const SizedBox(height: 2),
                  Text(subtitle, style: AppTypography.caption(context)),
                ],
              ),
            ),
            Icon(Icons.chevron_right, size: 20, color: context.textTertiary),
          ],
        ),
      ),
    );
  }
}
