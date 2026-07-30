import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:go_router/go_router.dart';
import '../../../../app/theme/app_colors.dart';
import '../../../../app/theme/app_typography.dart';
import '../../../../app/theme/app_spacing.dart';
import '../../../../app/theme/app_radius.dart';
import '../../../../core/widgets/amitia_scaffold.dart';
import '../../../../app/app_routes.dart';

class WorkshopHomePage extends ConsumerWidget {
  const WorkshopHomePage({super.key});

  @override
  Widget build(BuildContext context, WidgetRef ref) {
    return AmitiaScaffold(
      appBar: AmitiaAppBar(
        title: '创意工坊',
        showBackButton: true,
      ),
      body: SafeArea(
        top: false,
        child: SingleChildScrollView(
          padding: const EdgeInsets.all(AppSpacing.pagePadding),
          child: Column(
            crossAxisAlignment: CrossAxisAlignment.start,
            children: [
              _buildHeaderBanner(context),
              const SizedBox(height: AppSpacing.xl),
              _buildEntryCard(
                context,
                icon: Icons.psychology_outlined,
                title: '技能制作',
                description: '创建和管理 AI 技能，定义输入输出 Schema、测试并安装到系统',
                onTap: () => context.push(AppRoutes.workshopSkills),
              ),
              const SizedBox(height: AppSpacing.md),
              _buildEntryCard(
                context,
                icon: Icons.pets_outlined,
                title: '桌宠制作',
                description: '生成自定义桌宠角色，处理动作帧、审核质量并安装到桌面',
                onTap: () => context.push(AppRoutes.workshopPet),
              ),
              const SizedBox(height: AppSpacing.xxl),
              Text('快速开始', style: AppTypography.sectionTitle(context)),
              const SizedBox(height: AppSpacing.sm),
              _buildTipCard(context),
            ],
          ),
        ),
      ),
    );
  }

  Widget _buildHeaderBanner(BuildContext context) {
    return Container(
      padding: const EdgeInsets.all(AppSpacing.xl),
      decoration: BoxDecoration(
        gradient: LinearGradient(
          begin: Alignment.topLeft,
          end: Alignment.bottomRight,
          colors: [
            context.accentPrimary,
            context.accentSecondary,
          ],
        ),
        borderRadius: AppRadius.brLarge,
      ),
      child: Column(
        crossAxisAlignment: CrossAxisAlignment.start,
        children: [
          Row(
            children: [
              Icon(Icons.auto_awesome, size: 28, color: Colors.white),
              const SizedBox(width: AppSpacing.sm),
              Text(
                '创意工坊',
                style: TextStyle(
                  fontSize: 22,
                  fontWeight: FontWeight.w600,
                  color: Colors.white,
                ),
              ),
            ],
          ),
          const SizedBox(height: AppSpacing.xs),
          Text(
            '在这里制作专属技能和桌宠，释放你的创造力',
            style: TextStyle(
              fontSize: 14,
              color: Colors.white.withValues(alpha: 0.85),
            ),
          ),
        ],
      ),
    );
  }

  Widget _buildEntryCard(
    BuildContext context, {
    required IconData icon,
    required String title,
    required String description,
    required VoidCallback onTap,
  }) {
    return GestureDetector(
      onTap: onTap,
      child: Container(
        padding: const EdgeInsets.all(AppSpacing.xl),
        decoration: BoxDecoration(
          color: context.surfacePrimary,
          borderRadius: AppRadius.brLarge,
          border: Border.all(color: context.borderPrimary, width: 0.5),
        ),
        child: Row(
          children: [
            Container(
              width: 56,
              height: 56,
              decoration: BoxDecoration(
                color: context.accentSoft,
                borderRadius: AppRadius.brMedium,
              ),
              child: Icon(icon, size: 28, color: context.accentPrimary),
            ),
            const SizedBox(width: AppSpacing.md),
            Expanded(
              child: Column(
                crossAxisAlignment: CrossAxisAlignment.start,
                children: [
                  Text(title, style: AppTypography.cardTitle(context)),
                  const SizedBox(height: 4),
                  Text(description, style: AppTypography.caption(context)),
                ],
              ),
            ),
            Icon(Icons.chevron_right, size: 22, color: context.textTertiary),
          ],
        ),
      ),
    );
  }

  Widget _buildTipCard(BuildContext context) {
    final tips = [
      (Icons.lightbulb_outline, '技能制作', '通过描述自动生成结构化 Draft，支持测试和安装'),
      (Icons.animation, '桌宠制作', '上传角色图后自动生成动作帧，支持逐帧审核和编辑'),
    ];
    return Container(
      decoration: BoxDecoration(
        color: context.surfacePrimary,
        borderRadius: AppRadius.brMedium,
        border: Border.all(color: context.borderPrimary, width: 0.5),
      ),
      child: Column(
        children: List.generate(tips.length, (i) {
          final tip = tips[i];
          return Padding(
            padding: const EdgeInsets.symmetric(horizontal: AppSpacing.cardPadding, vertical: 12),
            child: Row(
              crossAxisAlignment: CrossAxisAlignment.start,
              children: [
                Icon(tip.$1, size: 20, color: context.accentPrimary),
                const SizedBox(width: AppSpacing.md),
                Expanded(
                  child: Column(
                    crossAxisAlignment: CrossAxisAlignment.start,
                    children: [
                      Text(tip.$2, style: AppTypography.body(context)),
                      const SizedBox(height: 2),
                      Text(tip.$3, style: AppTypography.caption(context)),
                    ],
                  ),
                ),
              ],
            ),
          );
        }),
      ),
    );
  }
}
