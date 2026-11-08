import 'package:flutter/material.dart';
import 'package:go_router/go_router.dart';

import '../../../../app/app_routes.dart';
import '../../../../app/theme/app_colors.dart';
import '../../../../app/theme/app_radius.dart';
import '../../../../app/theme/app_spacing.dart';
import '../../../../app/theme/app_typography.dart';
import '../../../../core/widgets/amitia_scaffold.dart';

class RuntimeModePage extends StatelessWidget {
  const RuntimeModePage({super.key});

  @override
  Widget build(BuildContext context) {
    return AmitiaScaffold(
      appBar: AmitiaAppBar(
        title: '运行模式',
        navigation: AmitiaAppBarNavigation.back,
        fallbackRoute: AppRoutes.settings,
      ),
      body: ListView(
        padding: EdgeInsets.symmetric(vertical: AppSpacing.md),
        children: [
          _EntryCard(
            icon: Icons.tune_outlined,
            title: '运行模式',
            description: '选择以本机作为统筹中心，或连接远程统筹中心。',
            onTap: () => context.push(AppRoutes.settingsDeployment),
          ),
          _EntryCard(
            icon: Icons.devices_outlined,
            title: '已绑定设备',
            description: '查看中心或当前设备已经绑定的设备、权限与运行状态。',
            onTap: () => context.push(AppRoutes.settingsBoundDevices),
          ),
        ],
      ),
    );
  }
}

class _EntryCard extends StatelessWidget {
  final IconData icon;
  final String title;
  final String description;
  final VoidCallback onTap;

  const _EntryCard({
    required this.icon,
    required this.title,
    required this.description,
    required this.onTap,
  });

  @override
  Widget build(BuildContext context) {
    return Padding(
      padding: EdgeInsets.fromLTRB(
        AppSpacing.pagePadding,
        0,
        AppSpacing.pagePadding,
        AppSpacing.md,
      ),
      child: Material(
        color: context.surfacePrimary,
        borderRadius: AppRadius.brMedium,
        child: InkWell(
          borderRadius: AppRadius.brMedium,
          onTap: onTap,
          child: Container(
            padding: EdgeInsets.all(AppSpacing.cardPadding),
            decoration: BoxDecoration(
              borderRadius: AppRadius.brMedium,
              border: Border.all(color: context.borderPrimary, width: 0.6),
            ),
            child: Row(
              children: [
                Container(
                  width: 46,
                  height: 46,
                  decoration: BoxDecoration(
                    color: context.accentSoft,
                    borderRadius: AppRadius.brSmall,
                  ),
                  child: Icon(icon, color: context.accentPrimary),
                ),
                SizedBox(width: AppSpacing.md),
                Expanded(
                  child: Column(
                    crossAxisAlignment: CrossAxisAlignment.start,
                    children: [
                      Text(title, style: AppTypography.cardTitle(context)),
                      SizedBox(height: 4),
                      Text(description, style: AppTypography.caption(context)),
                    ],
                  ),
                ),
                Icon(Icons.chevron_right, color: context.textTertiary),
              ],
            ),
          ),
        ),
      ),
    );
  }
}
