import 'package:flutter/material.dart';
import 'package:go_router/go_router.dart';

import '../../../../app/app_routes.dart';
import '../../../../app/theme/app_colors.dart';
import '../../../../app/theme/app_radius.dart';
import '../../../../app/theme/app_spacing.dart';
import '../../../../app/theme/app_typography.dart';
import '../../../../core/widgets/amitia_icons.dart';
import '../../../../core/widgets/amitia_scaffold.dart';
import '../../../../core/widgets/hub_entry_card.dart';

class CharacterWorldBookHubPage extends StatelessWidget {
  const CharacterWorldBookHubPage({super.key});

  static const _entries = <_CharacterEntry>[
    _CharacterEntry(
      title: '角色管理',
      icon: AmitiaIconGlyph.characters,
      route: AppRoutes.characters,
    ),
    _CharacterEntry(
      title: '世界书',
      icon: AmitiaIconGlyph.worldBook,
      route: AppRoutes.memoryWorldBook,
    ),
  ];

  @override
  Widget build(BuildContext context) {
    return AmitiaScaffold(
      appBar: AmitiaAppBar(
        title: '角色',
        backgroundColor: context.surfacePrimary,
      ),
      body: ListView(
        padding: EdgeInsets.fromLTRB(
          AppSpacing.pagePadding,
          AppSpacing.md,
          AppSpacing.pagePadding,
          AppSpacing.xl,
        ),
        children: [
          Text('角色功能', style: AppTypography.label(context)),
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

class _CharacterEntry {
  const _CharacterEntry({
    required this.title,
    required this.icon,
    required this.route,
  });

  final String title;
  final AmitiaIconGlyph icon;
  final String route;
}
