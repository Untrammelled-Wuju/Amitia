import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:go_router/go_router.dart';
import '../../../../app/app_routes.dart';
import '../../../../app/theme/app_colors.dart';
import '../../../../app/theme/app_motion.dart';
import '../../../../app/theme/app_typography.dart';
import '../../../../app/theme/app_spacing.dart';
import '../../../../app/theme/app_radius.dart';
import '../../../../core/widgets/amitia_scaffold.dart';
import '../../../../core/widgets/amitia_drawer.dart';
import '../../../../core/services/providers.dart';
import '../../../../shared/models/models.dart';

final _settingsGroups = <SettingGroup>[
  SettingGroup(title: 'AI 与个性化', items: [
    SettingItem(title: '模型设置', icon: Icons.psychology_outlined, value: 'GPT-4', route: AppRoutes.settingsModels),
    SettingItem(title: 'AI 配置', icon: Icons.smart_toy_outlined, route: AppRoutes.settingsAi),
    SettingItem(title: '语音识别', icon: Icons.transcribe_outlined, route: AppRoutes.settingsAsr),
    SettingItem(title: '外观设置', icon: Icons.palette_outlined, value: '亮色', route: AppRoutes.settingsAppearance),
    SettingItem(title: '主题设置', icon: Icons.color_lens_outlined, route: AppRoutes.settingsTheme),
    SettingItem(title: '界面提供者', icon: Icons.dashboard_customize_outlined, route: AppRoutes.settingsUIProviders),
    SettingItem(title: '用户设置', icon: Icons.person_outline, route: AppRoutes.settingsUser),
    SettingItem(title: '时间感知', icon: Icons.schedule_outlined, route: AppRoutes.settingsTemporal),
  ]),
  SettingGroup(title: '系统与维护', items: [
    SettingItem(title: 'Runtime', icon: Icons.terminal, route: AppRoutes.settingsRuntime),
    SettingItem(title: '运行模式', icon: Icons.hub_outlined, subtitle: '桌面本地 / 私有云部署模式', route: AppRoutes.settingsRuntimeMode),
    SettingItem(title: '长期运行维护', icon: Icons.schedule_send_outlined, subtitle: '长期任务、健康历史与日志维护', route: AppRoutes.settingsLongRunning),
    SettingItem(title: '高级系统', icon: Icons.admin_panel_settings_outlined, subtitle: '账号、审计、Usage、Bridge 与 Voice Session', route: AppRoutes.settingsAdvanced),
    SettingItem(title: 'BDI 决策可视化', icon: Icons.account_tree_outlined, subtitle: 'BehaviorPlan、ExpressionPlan 与降级状态', route: AppRoutes.settingsDecisionViz),
    SettingItem(title: '系统权限', icon: Icons.lock_outlined, route: AppRoutes.settingsPermissions),
    SettingItem(title: 'Android Automation', icon: Icons.smartphone_outlined, subtitle: '执行通道、视觉能力与 Virtual Display 健康状态', route: AppRoutes.settingsAndroidAutomation),
    SettingItem(title: '存储管理', icon: Icons.storage_outlined, route: AppRoutes.settingsStorage),
    SettingItem(title: '安全设置', icon: Icons.security_outlined, route: AppRoutes.settingsSafety),
    SettingItem(title: '维护工具', icon: Icons.build_circle_outlined, route: AppRoutes.settingsMaintenance),
    SettingItem(title: '工具箱', icon: Icons.handyman_outlined, subtitle: '运行日志、状态诊断与开发辅助工具', value: '诊断工具', route: AppRoutes.settingsToolbox),
  ]),
  SettingGroup(title: '部署与隐私', items: [
    SettingItem(title: '部署配置', icon: Icons.cloud_upload_outlined, route: AppRoutes.settingsDeployment),
    SettingItem(title: '隐私扫描', icon: Icons.privacy_tip_outlined, route: AppRoutes.settingsPrivacyScan),
    SettingItem(title: '系统设置', icon: Icons.settings_applications_outlined, route: AppRoutes.settingsSystem),
  ]),
  SettingGroup(title: '关于', items: [
    SettingItem(title: '备份与恢复', icon: Icons.backup_outlined, route: AppRoutes.settingsBackup),
    SettingItem(title: '关于 Amitia', icon: Icons.info_outline, route: AppRoutes.settingsAbout),
  ]),
];

class SettingsPage extends ConsumerWidget {
  const SettingsPage({super.key});

  @override
  Widget build(BuildContext context, WidgetRef ref) {
    final groups = _settingsGroups;
    final isDevMode = ref.watch(isDeveloperModeProvider);
    return AmitiaScaffold(
      appBar: AmitiaAppBar(
        title: '设置',
        navigation: AmitiaAppBarNavigation.back,
      ),
      body: ListView(
        padding: EdgeInsets.symmetric(vertical: AppSpacing.md),
        children: [
          Padding(
            padding: EdgeInsets.symmetric(
              horizontal: AppSpacing.pagePadding,
            ),
            child: _buildUserInfoCard(context, ref),
          ),
          SizedBox(height: AppSpacing.md),
          for (int i = 0; i < groups.length; i++) ...[
            _SettingGroup(
              group: groups[i],
              leading: groups[i].title == '系统与维护'
                  ? Padding(
                      padding: EdgeInsets.only(bottom: AppSpacing.sm),
                      child: _DevModeToggle(
                        isDevMode: isDevMode,
                        onTap: () {
                          ref.read(isDeveloperModeProvider.notifier).state =
                              !isDevMode;
                        },
                      ),
                    )
                  : null,
            ),
            if (i < groups.length - 1)
              SizedBox(height: AppSpacing.sectionGap),
          ],
          SizedBox(height: AppSpacing.xl),
        ],
      ),
    );
  }
}

Widget _buildUserInfoCard(BuildContext context, WidgetRef ref) {
  final userAsync = ref.watch(currentUserProvider);
  return userAsync.when(
    data: (user) {
      final username = user?.username ?? '未登录';
      final initial = username.isNotEmpty ? username.characters.first : '?';
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
              Container(
                width: 44,
                height: 44,
                decoration: BoxDecoration(
                  borderRadius: BorderRadius.circular(16),
                  gradient: LinearGradient(
                    begin: Alignment.topLeft,
                    end: Alignment.bottomRight,
                    colors: [context.accentPrimary, context.accentSecondary],
                  ),
                ),
                child: Center(
                  child: Text(
                    initial,
                    style: const TextStyle(
                      color: Colors.white,
                      fontSize: 15,
                      fontWeight: FontWeight.w600,
                    ),
                  ),
                ),
              ),
              const SizedBox(width: 12),
              Expanded(
                child: Column(
                  crossAxisAlignment: CrossAxisAlignment.start,
                  mainAxisSize: MainAxisSize.min,
                  children: [
                    Text(
                      username,
                      style: TextStyle(
                        fontSize: 14,
                        fontWeight: FontWeight.w600,
                        color: context.textPrimary,
                      ),
                    ),
                    const SizedBox(height: 2),
                    Row(
                      mainAxisSize: MainAxisSize.min,
                      children: [
                        Container(
                          width: 6,
                          height: 6,
                          decoration: BoxDecoration(
                            color: context.success,
                            shape: BoxShape.circle,
                          ),
                        ),
                        const SizedBox(width: 4),
                        Text(
                          '已登录',
                          style: TextStyle(
                            fontSize: 11,
                            color: context.textTertiary,
                          ),
                        ),
                      ],
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
            Container(
              width: 44,
              height: 44,
              decoration: BoxDecoration(
                borderRadius: BorderRadius.circular(16),
                gradient: LinearGradient(
                  begin: Alignment.topLeft,
                  end: Alignment.bottomRight,
                  colors: [context.accentPrimary, context.accentSecondary],
                ),
              ),
              child: const Center(
                child: Text(
                  '?',
                  style: TextStyle(
                    color: Colors.white,
                    fontSize: 15,
                    fontWeight: FontWeight.w600,
                  ),
                ),
              ),
            ),
            const SizedBox(width: 12),
            Expanded(
              child: Text(
                '未登录',
                style: TextStyle(
                  fontSize: 14,
                  fontWeight: FontWeight.w600,
                  color: context.textPrimary,
                ),
              ),
            ),
            Icon(Icons.chevron_right, size: 20, color: context.textTertiary),
          ],
        ),
      ),
    ),
  );
}

class _DevModeToggle extends StatelessWidget {
  final bool isDevMode;
  final VoidCallback onTap;

  const _DevModeToggle({required this.isDevMode, required this.onTap});

  @override
  Widget build(BuildContext context) {
    return GestureDetector(
      onTap: onTap,
      child: Container(
        padding: const EdgeInsets.symmetric(horizontal: 14, vertical: 10),
        decoration: BoxDecoration(
          color: isDevMode
              ? context.accentPrimary.withValues(alpha: 0.12)
              : context.surfacePrimary,
          borderRadius: AppRadius.brMedium,
          border: Border.all(color: context.borderPrimary, width: 0.5),
        ),
        child: Row(
          children: [
            Icon(
              Icons.developer_mode,
              size: 20,
              color: isDevMode ? context.accentPrimary : context.textTertiary,
            ),
            const SizedBox(width: 10),
            Expanded(
              child: Text(
                '开发者模式',
                style: TextStyle(
                  fontSize: 14,
                  fontWeight: FontWeight.w500,
                  color: isDevMode
                      ? context.accentPrimary
                      : context.textPrimary,
                ),
              ),
            ),
            Container(
              width: 36,
              height: 20,
              decoration: BoxDecoration(
                color: isDevMode
                    ? context.accentPrimary
                    : context.borderPrimary,
                borderRadius: BorderRadius.circular(10),
              ),
              child: AnimatedAlign(
                duration: AppMotion.standard,
                curve: AppMotion.standardCurve,
                alignment: isDevMode
                    ? Alignment.centerRight
                    : Alignment.centerLeft,
                child: Container(
                  width: 16,
                  height: 16,
                  margin: EdgeInsets.symmetric(horizontal: 2),
                  decoration: const BoxDecoration(
                    color: Colors.white,
                    shape: BoxShape.circle,
                  ),
                ),
              ),
            ),
          ],
        ),
      ),
    );
  }
}

class _SettingGroup extends StatelessWidget {
  final SettingGroup group;
  final Widget? leading;

  const _SettingGroup({required this.group, this.leading});

  @override
  Widget build(BuildContext context) {
    return Column(
      crossAxisAlignment: CrossAxisAlignment.start,
      children: [
        if (leading != null) leading!,
        Padding(
          padding: EdgeInsets.fromLTRB(
            AppSpacing.pagePadding,
            AppSpacing.sm,
            AppSpacing.pagePadding,
            AppSpacing.sm,
          ),
          child: Text(group.title, style: AppTypography.caption(context)),
        ),
        Container(
          margin: EdgeInsets.symmetric(
            horizontal: AppSpacing.pagePadding,
          ),
          decoration: BoxDecoration(
            color: context.surfacePrimary,
            borderRadius: AppRadius.brMedium,
            border: Border.all(color: context.borderPrimary, width: 0.5),
          ),
          child: Column(
            children: [
              for (int i = 0; i < group.items.length; i++) ...[
                _SettingTile(item: group.items[i]),
                if (i < group.items.length - 1)
                  Padding(
                    padding: const EdgeInsets.only(left: 56),
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
        padding: EdgeInsets.symmetric(
          horizontal: AppSpacing.lg,
          vertical: 13,
        ),
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
                  Text(item.title, style: AppTypography.body(context)),
                  if (item.subtitle != null)
                    Padding(
                      padding: const EdgeInsets.only(top: 2),
                      child: Text(
                        item.subtitle!,
                        style: AppTypography.caption(context),
                        maxLines: 1,
                        overflow: TextOverflow.ellipsis,
                      ),
                    ),
                ],
              ),
            ),
            if (item.value != null) ...[
              Text(item.value!, style: AppTypography.caption(context)),
              const SizedBox(width: 4),
            ],
            Icon(Icons.chevron_right, size: 20, color: context.textTertiary),
          ],
        ),
      ),
    );
  }
}
