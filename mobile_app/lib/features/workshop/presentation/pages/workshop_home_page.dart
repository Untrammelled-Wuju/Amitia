import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:go_router/go_router.dart';
import '../../../../app/theme/app_colors.dart';
import '../../../../app/theme/app_radius.dart';
import '../../../../app/theme/app_typography.dart';
import '../../../../app/theme/app_spacing.dart';
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
        fallbackRoute: AppRoutes.chat,
      ),
      body: SafeArea(
        top: false,
        child: SingleChildScrollView(
          padding: EdgeInsets.all(AppSpacing.pagePadding),
          child: Column(
            crossAxisAlignment: CrossAxisAlignment.start,
            children: [
              _buildEntryCard(
                context,
                icon: Icons.badge_outlined,
                title: '角色卡工坊',
                description: '创建和编辑角色卡，导入酒馆角色卡并导出 CHARX 角色包',
                onTap: () => context.push(AppRoutes.workshopCharacterCards),
              ),
              _buildEntryCard(
                context,
                icon: Icons.account_tree_outlined,
                title: '工作流',
                description: '创建和管理自动化工作流及其运行配置',
                onTap: () => context.push(AppRoutes.workshopWorkflows),
              ),
              _buildEntryCard(
                context,
                icon: Icons.pets_outlined,
                title: '桌宠中心',
                description: '管理桌宠安装、插件与运行状态',
                onTap: () => context.push(AppRoutes.workshopPet),
              ),
            ],
          ),
        ),
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
    return Material(
      color: Colors.transparent,
      child: InkWell(
        onTap: onTap,
        borderRadius: AppRadius.brMedium,
        child: Container(
          constraints: const BoxConstraints(minHeight: 76),
          margin: const EdgeInsets.only(bottom: 10),
          padding: const EdgeInsets.symmetric(horizontal: 14, vertical: 12),
          decoration: BoxDecoration(
            color: context.surfaceSecondary,
            borderRadius: AppRadius.brMedium,
            border: Border.all(color: context.borderPrimary, width: 0.6),
          ),
          child: Row(
            children: [
              Container(
                width: 38,
                height: 38,
                decoration: BoxDecoration(
                  color: context.accentSoft,
                  borderRadius: AppRadius.brSmall,
                ),
                child: Icon(icon, size: 21, color: context.accentPrimary),
              ),
              const SizedBox(width: 14),
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
              const SizedBox(width: 8),
              Container(
                width: 30,
                height: 30,
                decoration: BoxDecoration(
                  color: context.surfacePrimary,
                  shape: BoxShape.circle,
                ),
                child: Icon(
                  Icons.arrow_forward_rounded,
                  size: 17,
                  color: context.accentPrimary,
                ),
              ),
            ],
          ),
        ),
      ),
    );
  }
}
