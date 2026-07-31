import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:go_router/go_router.dart';
import '../../../../app/theme/app_colors.dart';
import '../../../../app/theme/app_typography.dart';
import '../../../../app/theme/app_spacing.dart';
import '../../../../app/theme/app_radius.dart';
import '../../../../core/widgets/amitia_scaffold.dart';
import '../../../../shared/models/models.dart';
import '../../../../shared/mock_data/mock_data.dart';

class SettingsPage extends ConsumerWidget {
  const SettingsPage({super.key});

  @override
  Widget build(BuildContext context, WidgetRef ref) {
    final groups = MockData.mainSettings;
    return AmitiaScaffold(
      appBar: AmitiaAppBar(title: '设置', navigation: AmitiaAppBarNavigation.drawer),
      body: ListView(
        padding: const EdgeInsets.symmetric(vertical: AppSpacing.md),
        children: [
          for (int i = 0; i < groups.length; i++) ...[
            _SettingGroup(group: groups[i]),
            if (i < groups.length - 1) const SizedBox(height: AppSpacing.sectionGap),
          ],
          const SizedBox(height: AppSpacing.xl),
        ],
      ),
    );
  }
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
          padding: const EdgeInsets.fromLTRB(
            AppSpacing.pagePadding,
            AppSpacing.sm,
            AppSpacing.pagePadding,
            AppSpacing.sm,
          ),
          child: Text(group.title, style: AppTypography.caption(context)),
        ),
        Container(
          margin: const EdgeInsets.symmetric(horizontal: AppSpacing.pagePadding),
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
              Text(
                item.value!,
                style: AppTypography.caption(context),
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
