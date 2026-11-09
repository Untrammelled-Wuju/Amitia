import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:go_router/go_router.dart';

import '../../../../app/app_routes.dart';
import '../../../../app/theme/app_colors.dart';
import '../../../../app/theme/app_radius.dart';
import '../../../../app/theme/app_spacing.dart';
import '../../../../app/theme/app_typography.dart';
import '../../../../core/services/providers.dart';
import '../../../../core/ui_runtime/ui_navigation_registry.dart';
import '../../../../core/ui_runtime/ui_runtime_controller.dart';
import '../../../../core/widgets/amitia_scaffold.dart';
import '../../../../core/widgets/amitia_icons.dart';
import '../../../../core/widgets/hub_entry_card.dart';
import '../../../../core/widgets/profile_avatar.dart';

class ProfileHubPage extends ConsumerWidget {
  const ProfileHubPage({super.key});

  static const _coreEntries = <_ProfileEntry>[
    _ProfileEntry(
      title: '角色',
      glyph: AmitiaIconGlyph.characters,
      route: AppRoutes.meCharacters,
    ),
    _ProfileEntry(
      title: '创意工坊',
      glyph: AmitiaIconGlyph.workshop,
      route: AppRoutes.workshop,
    ),
    _ProfileEntry(
      title: '游戏模式',
      glyph: AmitiaIconGlyph.game,
      route: AppRoutes.gameCenter,
    ),
    _ProfileEntry(
      title: '归档对话',
      glyph: AmitiaIconGlyph.archive,
      route: AppRoutes.chatLogs,
    ),
    _ProfileEntry(
      title: '设置',
      glyph: AmitiaIconGlyph.settings,
      route: AppRoutes.settings,
    ),
  ];

  List<_ProfileEntry> _entries(WidgetRef ref) {
    final snapshot = ref.watch(effectiveUIRuntimeSnapshotProvider);
    final extensionEntries = UINavigationRegistry.resolve(snapshot)
        .where((item) => item.profile)
        .map(
          (item) => _ProfileEntry(
            title: item.label,
            iconData: item.icon,
            route: item.route,
          ),
        )
        .where(
          (entry) => !_coreEntries.any(
            (core) => core.route == entry.route || core.title == entry.title,
          ),
        )
        .toList(growable: false);
    return <_ProfileEntry>[..._coreEntries, ...extensionEntries];
  }

  @override
  Widget build(BuildContext context, WidgetRef ref) {
    final entries = _entries(ref);
    final profile = ref.watch(currentSpaceProfileProvider).valueOrNull;
    final name = (profile?.displayName ?? '').trim().isEmpty
        ? '我的空间'
        : profile!.displayName.trim();
    final spaceId = (profile?.spaceId ?? '').trim();
    final initial = name.characters.first;
    return AmitiaScaffold(
      handlesSystemBack: false,
      appBar: AmitiaAppBar(
        title: '我的',
        backgroundColor: context.surfacePrimary,
        toolbarHeight: AmitiaAppBar.rootPageToolbarHeight,
      ),
      body: ListView(
        padding: EdgeInsets.fromLTRB(
          AppSpacing.pagePadding,
          AppSpacing.md,
          AppSpacing.pagePadding,
          AppSpacing.xl,
        ),
        children: [
          Material(
            color: Colors.transparent,
            child: InkWell(
              onTap: () => context.push(AppRoutes.settingsUser),
              borderRadius: AppRadius.brMedium,
              child: Container(
                padding: EdgeInsets.all(AppSpacing.md),
                decoration: BoxDecoration(
                  color: context.surfacePrimary,
                  borderRadius: AppRadius.brMedium,
                  border: Border.all(color: context.borderPrimary, width: 0.6),
                ),
                child: Row(
                  children: [
                    ProfileAvatar(
                      avatar: profile?.avatar ?? '',
                      initial: initial,
                      size: 48,
                      borderRadius: BorderRadius.circular(14),
                    ),
                    SizedBox(width: AppSpacing.md),
                    Expanded(
                      child: Column(
                        crossAxisAlignment: CrossAxisAlignment.start,
                        children: [
                          Text(
                            name,
                            style: AppTypography.body(context).copyWith(
                              fontSize: 16,
                              fontWeight: FontWeight.w400,
                            ),
                            maxLines: 1,
                            overflow: TextOverflow.ellipsis,
                          ),
                          const SizedBox(height: 4),
                          Text(
                            spaceId.isEmpty ? '本地个人空间' : spaceId,
                            style: AppTypography.caption(context),
                            maxLines: 1,
                            overflow: TextOverflow.ellipsis,
                          ),
                        ],
                      ),
                    ),
                    AmitiaIcon(
                      AmitiaIconGlyph.chevronRight,
                      color: context.textTertiary,
                      size: 18,
                    ),
                  ],
                ),
              ),
            ),
          ),
          SizedBox(height: AppSpacing.lg),
          Text('功能入口', style: AppTypography.label(context)),
          SizedBox(height: AppSpacing.sm),
          Container(
            clipBehavior: Clip.antiAlias,
            decoration: BoxDecoration(
              color: context.surfacePrimary,
              borderRadius: AppRadius.brMedium,
              border: Border.all(color: context.borderPrimary, width: 0.5),
            ),
            child: Column(
              children: [
                for (var index = 0; index < entries.length; index++)
                  HubEntryCard(
                    icon: entries[index].glyph ?? AmitiaIconGlyph.extensions,
                    iconData: entries[index].iconData,
                    title: entries[index].title,
                    onTap: () => context.push(entries[index].route),
                    showDivider: index < entries.length - 1,
                  ),
              ],
            ),
          ),
        ],
      ),
    );
  }
}

class _ProfileEntry {
  const _ProfileEntry({
    required this.title,
    required this.route,
    this.glyph,
    this.iconData,
  });

  final String title;
  final String route;
  final AmitiaIconGlyph? glyph;
  final IconData? iconData;
}
