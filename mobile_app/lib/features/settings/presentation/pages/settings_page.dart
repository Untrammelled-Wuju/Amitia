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
import '../../../../shared/models/models.dart';
import '../../../../shared/mock_data/mock_data.dart';

class SettingsPage extends ConsumerWidget {
  const SettingsPage({super.key});

  @override
  Widget build(BuildContext context, WidgetRef ref) {
    final groups = MockData.mainSettings;
    final isDevMode = ref.watch(isDeveloperModeProvider);
    return AmitiaScaffold(
      appBar: AmitiaAppBar(
        title: '设置',
        navigation: AmitiaAppBarNavigation.back,
      ),
      body: ListView(
        padding: const EdgeInsets.symmetric(vertical: AppSpacing.md),
        children: [
          Padding(
            padding: const EdgeInsets.symmetric(
              horizontal: AppSpacing.pagePadding,
            ),
            child: _UserInfoCard(
              onTap: () => context.push(AppRoutes.settingsUser),
            ),
          ),
          const SizedBox(height: AppSpacing.md),
          for (int i = 0; i < groups.length; i++) ...[
            _SettingGroup(
              group: groups[i],
              leading: groups[i].title == '系统与维护'
                  ? Padding(
                      padding: const EdgeInsets.only(bottom: AppSpacing.sm),
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
              const SizedBox(height: AppSpacing.sectionGap),
          ],
          const SizedBox(height: AppSpacing.xl),
        ],
      ),
    );
  }
}

class _UserInfoCard extends StatelessWidget {
  final VoidCallback onTap;

  const _UserInfoCard({required this.onTap});

  @override
  Widget build(BuildContext context) {
    return GestureDetector(
      onTap: onTap,
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
                  '无',
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
              child: Column(
                crossAxisAlignment: CrossAxisAlignment.start,
                mainAxisSize: MainAxisSize.min,
                children: [
                  Text(
                    '无拘',
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
                        '本地运行',
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
  }
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
                  margin: const EdgeInsets.symmetric(horizontal: 2),
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
          padding: const EdgeInsets.fromLTRB(
            AppSpacing.pagePadding,
            AppSpacing.sm,
            AppSpacing.pagePadding,
            AppSpacing.sm,
          ),
          child: Text(group.title, style: AppTypography.caption(context)),
        ),
        Container(
          margin: const EdgeInsets.symmetric(
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
        padding: const EdgeInsets.symmetric(
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
