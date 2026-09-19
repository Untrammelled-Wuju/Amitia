import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:go_router/go_router.dart';
import '../../../../app/app_routes.dart';
import '../../../../app/theme/app_colors.dart';
import '../../../../app/theme/app_typography.dart';
import '../../../../app/theme/app_spacing.dart';
import '../../../../app/theme/app_radius.dart';
import '../../../../core/widgets/profile_avatar.dart';
import '../../../../core/widgets/amitia_scaffold.dart';
import '../../../../core/services/providers.dart';
import '../../../../core/settings/appearance_preferences.dart';
import '../../../../shared/models/models.dart';

const double _settingsOptionFontSize = 15;

List<SettingGroup> _settingsGroups({
  required String modelSummary,
  required String appearanceSummary,
}) => <SettingGroup>[
  SettingGroup(
    title: 'AI 与个性化',
    items: [
      SettingItem(
        title: '模型设置',
        icon: Icons.psychology_outlined,
        value: modelSummary,
        route: AppRoutes.settingsModels,
      ),
      SettingItem(
        title: '外观设置',
        icon: Icons.palette_outlined,
        value: appearanceSummary,
        route: AppRoutes.settingsAppearance,
      ),
      SettingItem(
        title: '界面提供者',
        icon: Icons.dashboard_customize_outlined,
        route: AppRoutes.settingsUIProviders,
      ),
      SettingItem(
        title: '时间感知',
        icon: Icons.schedule_outlined,
        route: AppRoutes.settingsTemporal,
      ),
    ],
  ),
  SettingGroup(
    title: '系统与维护',
    items: [
      SettingItem(
        title: 'Runtime',
        icon: Icons.terminal,
        route: AppRoutes.settingsRuntime,
      ),
      SettingItem(
        title: '运行模式',
        icon: Icons.hub_outlined,
        route: AppRoutes.settingsRuntimeMode,
      ),
      SettingItem(
        title: '长期运行维护',
        icon: Icons.schedule_send_outlined,
        route: AppRoutes.settingsLongRunning,
      ),
      SettingItem(
        title: '高级系统',
        icon: Icons.admin_panel_settings_outlined,
        route: AppRoutes.settingsAdvanced,
      ),
      SettingItem(
        title: 'BDI 决策可视化',
        icon: Icons.account_tree_outlined,
        route: AppRoutes.settingsDecisionViz,
      ),
      SettingItem(
        title: '系统权限',
        icon: Icons.lock_outlined,
        route: AppRoutes.settingsPermissions,
      ),
      SettingItem(
        title: 'Android Automation',
        icon: Icons.smartphone_outlined,
        route: AppRoutes.settingsAndroidAutomation,
      ),
      SettingItem(
        title: '存储管理',
        icon: Icons.storage_outlined,
        route: AppRoutes.settingsStorage,
      ),
      SettingItem(
        title: '安全设置',
        icon: Icons.security_outlined,
        route: AppRoutes.settingsSafety,
      ),
      SettingItem(
        title: '维护工具',
        icon: Icons.build_circle_outlined,
        route: AppRoutes.settingsMaintenance,
      ),
      SettingItem(
        title: '工具箱',
        icon: Icons.handyman_outlined,
        value: '诊断工具',
        route: AppRoutes.settingsToolbox,
      ),
    ],
  ),
  SettingGroup(
    title: '部署与隐私',
    items: [
      SettingItem(
        title: '部署配置',
        icon: Icons.cloud_upload_outlined,
        route: AppRoutes.settingsDeployment,
      ),
      SettingItem(
        title: '隐私扫描',
        icon: Icons.privacy_tip_outlined,
        route: AppRoutes.settingsPrivacyScan,
      ),
      SettingItem(
        title: '隐私政策',
        icon: Icons.policy_outlined,
        route: AppRoutes.settingsPrivacyPolicy,
      ),
      SettingItem(
        title: '用户协议',
        icon: Icons.description_outlined,
        route: AppRoutes.settingsUserAgreement,
      ),
      SettingItem(
        title: '系统设置',
        icon: Icons.settings_applications_outlined,
        route: AppRoutes.settingsSystem,
      ),
    ],
  ),
  SettingGroup(
    title: '关于',
    items: [
      SettingItem(
        title: '更新中心',
        icon: Icons.system_update_outlined,
        route: AppRoutes.settingsAppUpdate,
      ),
      SettingItem(
        title: '备份与恢复',
        icon: Icons.backup_outlined,
        route: AppRoutes.settingsBackup,
      ),
      SettingItem(
        title: '关于 Amitia',
        icon: Icons.info_outline,
        route: AppRoutes.settingsAbout,
      ),
    ],
  ),
];

class SettingsPage extends ConsumerWidget {
  const SettingsPage({super.key});

  @override
  Widget build(BuildContext context, WidgetRef ref) {
    final modelConfigs =
        ref.watch(modelConfigListProvider).valueOrNull ?? const [];
    final activeModel = modelConfigs
        .where((item) => item.isActive == 1)
        .firstOrNull;
    final modelSummary = activeModel == null
        ? '未配置'
        : (activeModel.name.trim().isNotEmpty
              ? activeModel.name.trim()
              : (activeModel.model.trim().isNotEmpty
                    ? activeModel.model.trim()
                    : activeModel.provider.trim()));
    final appearance = ref.watch(appearancePreferencesProvider);
    final themeSummary = switch (appearance.themeMode) {
      ThemeMode.dark => '暗色',
      ThemeMode.system => '跟随系统',
      _ => '亮色',
    };
    const accentNames = ['暖棕', '蓝色', '绿色', '琥珀'];
    final appearanceSummary =
        '$themeSummary · ${accentNames[appearance.accentColorIndex]}';
    final groups = _settingsGroups(
      modelSummary: modelSummary,
      appearanceSummary: appearanceSummary,
    );
    return AmitiaScaffold(
      appBar: AmitiaAppBar(
        title: '设置',
        navigation: AmitiaAppBarNavigation.back,
      ),
      body: ListView(
        padding: EdgeInsets.symmetric(vertical: AppSpacing.md),
        children: [
          Padding(
            padding: EdgeInsets.symmetric(horizontal: AppSpacing.pagePadding),
            child: _buildUserInfoCard(context, ref),
          ),
          SizedBox(height: AppSpacing.md),
          for (int i = 0; i < groups.length; i++) ...[
            _SettingGroup(group: groups[i]),
            if (i < groups.length - 1) SizedBox(height: AppSpacing.sectionGap),
          ],
          SizedBox(height: AppSpacing.xl),
        ],
      ),
    );
  }
}

Widget _buildUserInfoCard(BuildContext context, WidgetRef ref) {
  final profileAsync = ref.watch(currentSpaceProfileProvider);
  return profileAsync.when(
    data: (profile) {
      final displayName = (profile?.displayName ?? '').trim().isEmpty
          ? '我的空间'
          : profile!.displayName.trim();
      final initial = displayName.characters.first;
      final spaceId = (profile?.spaceId ?? '').trim();
      return GestureDetector(
        onTap: () => context.push(AppRoutes.settingsUser),
        child: Container(
          padding: const EdgeInsets.all(14),
          decoration: BoxDecoration(
            color: context.surfacePrimary,
            borderRadius: AppRadius.brMedium,
            border: Border.all(color: context.borderPrimary, width: 0.5),
          ),
          child: Row(
            children: [
              ProfileAvatar(
                avatar: profile?.avatar ?? '',
                initial: initial,
                size: 44,
                borderRadius: BorderRadius.circular(16),
              ),
              const SizedBox(width: 12),
              Expanded(
                child: Column(
                  crossAxisAlignment: CrossAxisAlignment.start,
                  mainAxisSize: MainAxisSize.min,
                  children: [
                    Text(
                      displayName,
                      style: TextStyle(
                        fontSize: 14.5,
                        fontWeight: FontWeight.w600,
                        color: context.textPrimary,
                      ),
                    ),
                    const SizedBox(height: 2),
                    Text(
                      spaceId.isEmpty ? '本地个人空间' : spaceId,
                      style: TextStyle(
                        fontSize: 11.5,
                        color: context.textTertiary,
                      ),
                      maxLines: 1,
                      overflow: TextOverflow.ellipsis,
                    ),
                  ],
                ),
              ),
              Icon(Icons.chevron_right, size: 20, color: context.textTertiary),
            ],
          ),
        ),
      );
    },
    loading: () => Container(
      padding: const EdgeInsets.all(14),
      decoration: BoxDecoration(
        color: context.surfacePrimary,
        borderRadius: AppRadius.brMedium,
        border: Border.all(color: context.borderPrimary, width: 0.5),
      ),
      child: const Center(child: CircularProgressIndicator(strokeWidth: 2)),
    ),
    error: (_, __) => GestureDetector(
      onTap: () => context.push(AppRoutes.settingsUser),
      child: Container(
        padding: const EdgeInsets.all(14),
        decoration: BoxDecoration(
          color: context.surfacePrimary,
          borderRadius: AppRadius.brMedium,
          border: Border.all(color: context.borderPrimary, width: 0.5),
        ),
        child: Row(
          children: [
            Icon(Icons.person_outline, color: context.textTertiary),
            const SizedBox(width: 12),
            Expanded(
              child: Text(
                '个人空间资料暂不可用',
                style: AppTypography.body(
                  context,
                ).copyWith(fontSize: 14.5),
              ),
            ),
            Icon(Icons.chevron_right, size: 20, color: context.textTertiary),
          ],
        ),
      ),
    ),
  );
}

class _SettingGroup extends StatelessWidget {
  final SettingGroup group;

  const _SettingGroup({required this.group});

  @override
  Widget build(BuildContext context) {
    return Column(
      crossAxisAlignment: CrossAxisAlignment.start,
      children: [
        Padding(
          padding: EdgeInsets.fromLTRB(
            AppSpacing.pagePadding,
            AppSpacing.sm,
            AppSpacing.pagePadding,
            AppSpacing.sm,
          ),
          child: Text(
            group.title,
            style: AppTypography.caption(
              context,
            ).copyWith(fontSize: 12.5),
          ),
        ),
        Container(
          margin: EdgeInsets.symmetric(horizontal: AppSpacing.pagePadding),
          decoration: BoxDecoration(
            color: context.surfacePrimary,
            borderRadius: AppRadius.brMedium,
            border: Border.all(color: context.borderPrimary, width: 0.5),
          ),
          child: Column(
            children: [
              for (final item in group.items) _SettingTile(item: item),
            ],
          ),
        ),
      ],
    );
  }
}

class _SettingTile extends StatelessWidget {
  final SettingItem item;

  const _SettingTile({required this.item});

  @override
  Widget build(BuildContext context) {
    return GestureDetector(
      behavior: HitTestBehavior.opaque,
      onTap: () => context.push(item.route),
      child: Padding(
        padding: EdgeInsets.symmetric(horizontal: AppSpacing.lg, vertical: 13),
        child: Row(
          children: [
            Container(
              width: 32,
              height: 32,
              decoration: BoxDecoration(
                color: context.accentSoft,
                shape: BoxShape.circle,
              ),
              child: Icon(item.icon, size: 17, color: context.accentPrimary),
            ),
            const SizedBox(width: 12),
            Expanded(
              child: Column(
                crossAxisAlignment: CrossAxisAlignment.start,
                children: [
                  Text(
                    item.title,
                    style: AppTypography.body(
                      context,
                    ).copyWith(fontSize: _settingsOptionFontSize),
                  ),
                ],
              ),
            ),
            if (item.value != null) ...[
              Text(
                item.value!,
                style: AppTypography.caption(
                  context,
                ).copyWith(fontSize: 13.5),
              ),
              const SizedBox(width: 4),
            ],
            Icon(Icons.chevron_right, size: 20, color: context.textTertiary),
          ],
        ),
      ),
    );
  }
}
