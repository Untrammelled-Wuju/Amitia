import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:go_router/go_router.dart';

import '../../../../app/app_routes.dart';
import '../../../../app/theme/app_colors.dart';
import '../../../../app/theme/app_radius.dart';
import '../../../../app/theme/app_spacing.dart';
import '../../../../app/theme/app_typography.dart';
import '../../../../core/services/providers.dart';
import '../../../../core/widgets/amitia_scaffold.dart';
import '../../../../core/widgets/amitia_icons.dart';
import '../../../../core/widgets/hub_entry_card.dart';
import '../../../../core/widgets/profile_avatar.dart';

class ProfileHubPage extends ConsumerWidget {
  const ProfileHubPage({super.key});

  static const _entries = <_ProfileEntry>[
    _ProfileEntry(
      title: '角色',
      icon: AmitiaIconGlyph.characters,
      route: AppRoutes.characters,
    ),
    _ProfileEntry(
      title: '创意工坊',
      icon: AmitiaIconGlyph.workshop,
      route: AppRoutes.workshop,
    ),
    _ProfileEntry(
      title: '游戏模式',
      icon: AmitiaIconGlyph.game,
      route: AppRoutes.gameCenter,
    ),
    _ProfileEntry(
      title: '渠道接入',
      icon: AmitiaIconGlyph.channels,
      route: AppRoutes.channelMessages,
    ),
    _ProfileEntry(
      title: '设置',
      icon: AmitiaIconGlyph.settings,
      route: AppRoutes.settings,
    ),
  ];

  @override
  Widget build(BuildContext context, WidgetRef ref) {
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
                for (var index = 0; index < _entries.length; index++)
                  HubEntryCard(
                    icon: _entries[index].icon,
                    title: _entries[index].title,
                    onTap: () => context.push(_entries[index].route),
                    showDivider: index < _entries.length - 1,
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
    required this.icon,
    required this.route,
  });

  final String title;
  final AmitiaIconGlyph icon;
  final String route;
}
