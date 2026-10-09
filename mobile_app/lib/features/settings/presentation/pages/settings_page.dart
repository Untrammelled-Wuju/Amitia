import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:go_router/go_router.dart';
import '../../../../app/app_routes.dart';
import '../../../../app/theme/app_colors.dart';
import '../../../../app/theme/app_typography.dart';
import '../../../../app/theme/app_spacing.dart';
import '../../../../app/theme/app_radius.dart';
import '../../../../core/widgets/profile_avatar.dart';
import '../../../../core/widgets/amitia_drawer.dart';
import '../../../../core/widgets/amitia_scaffold.dart';
import '../../../../core/services/providers.dart';
import '../../../../core/settings/appearance_preferences.dart';
import '../../../../shared/models/models.dart';
import '../../../../core/settings/settings_navigation.dart';

const double _settingsOptionFontSize = 16;

class SettingsCategoryPage extends ConsumerWidget {
  const SettingsCategoryPage({super.key, required this.categoryId});

  final String categoryId;

  @override
  Widget build(BuildContext context, WidgetRef ref) {
    final categories = buildSettingsCategories(
      modelSummary: '',
      appearanceSummary: '',
      isDeveloperMode: ref.watch(isDeveloperModeProvider),
    );
    final group = categories.where((category) => category.id == categoryId).firstOrNull?.group ?? categories.first.group;
    return AmitiaScaffold(
      appBar: AmitiaAppBar(
        title: group.title,
        showBackButton: true,
        fallbackRoute: AppRoutes.settings,
      ),
      body: ListView(
        padding: EdgeInsets.symmetric(vertical: AppSpacing.lg),
        children: [
          _SettingGroup(group: group),
          SizedBox(height: AppSpacing.xxxl),
        ],
      ),
    );
  }
}

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
    final categories = buildSettingsCategories(
      modelSummary: modelSummary,
      appearanceSummary: appearanceSummary,
      isDeveloperMode: ref.watch(isDeveloperModeProvider),
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
          _SettingGroup(group: categories.firstWhere((category) => category.id == 'appearance').group),
          SizedBox(height: AppSpacing.sectionGap),
          _SettingGroup(
            group: SettingGroup(
              title: '功能与管理',
              items: [
                for (final category in categories.where((category) => category.id != 'appearance' && category.id != 'about'))
                  SettingItem(
                    title: category.group.title,
                    icon: category.icon,
                    value: category.id == 'ai' ? modelSummary : null,
                    route: '/settings/category/${category.id}',
                  ),
              ],
            ),
          ),
          SizedBox(height: AppSpacing.sectionGap),
          _SettingGroup(group: categories.firstWhere((category) => category.id == 'about').group),
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
                        fontSize: 17,
                        fontWeight: FontWeight.w600,
                        color: context.textPrimary,
                      ),
                    ),
                    const SizedBox(height: 2),
                    Text(
                      spaceId.isEmpty ? '本地个人空间' : spaceId,
                      style: TextStyle(
                        fontSize: 13,
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
        ),
        child: Row(
          children: [
            Icon(Icons.person_outline, color: context.textTertiary),
            const SizedBox(width: 12),
            Expanded(
              child: Text(
                '个人空间资料暂不可用',
                style: AppTypography.body(context).copyWith(fontSize: 17),
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
            style: AppTypography.caption(context).copyWith(fontSize: 13),
          ),
        ),
        Container(
          margin: EdgeInsets.symmetric(horizontal: AppSpacing.pagePadding),
          decoration: BoxDecoration(
            color: context.surfacePrimary,
            borderRadius: AppRadius.brMedium,
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
    return Material(
      color: Colors.transparent,
      child: InkWell(
        onTap: () => context.push(item.route),
        borderRadius: AppRadius.brMedium,
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
                    Text(
                      item.title,
                      style: AppTypography.body(
                        context,
                      ).copyWith(fontSize: _settingsOptionFontSize),
                    ),
                    if (item.value != null && item.value!.isNotEmpty) ...[
                      const SizedBox(height: 3),
                      Text(item.value!, style: AppTypography.caption(context)),
                    ],
                  ],
                ),
              ),

              Icon(Icons.chevron_right, size: 20, color: context.textTertiary),
            ],
          ),
        ),
      ),
    );
  }
}
