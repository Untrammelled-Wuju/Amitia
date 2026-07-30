import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import '../../../../app/theme/app_colors.dart';
import '../../../../app/theme/app_typography.dart';
import '../../../../app/theme/app_spacing.dart';
import '../../../../app/theme/app_radius.dart';
import '../../../../core/widgets/amitia_scaffold.dart';
import '../../../../core/widgets/amitia_misc.dart';
import '../../../../shared/models/models.dart';
import '../../../../shared/mock_data/mock_data.dart';

class PermissionsPage extends ConsumerStatefulWidget {
  const PermissionsPage({super.key});

  @override
  ConsumerState<PermissionsPage> createState() => _PermissionsPageState();
}

class _PermissionsPageState extends ConsumerState<PermissionsPage> {
  late List<PermissionItem> _permissions;

  @override
  void initState() {
    super.initState();
    _permissions = List.of(MockData.permissions);
  }

  BadgeType _badgeType(String status) {
    switch (status) {
      case '已授权':
        return BadgeType.success;
      case '需要设置':
        return BadgeType.warning;
      case '不可用':
        return BadgeType.error;
      default:
        return BadgeType.neutral;
    }
  }

  void _showGuide(PermissionItem item) {
    showModalBottomSheet(
      context: context,
      backgroundColor: context.surfacePrimary,
      shape: const RoundedRectangleBorder(
        borderRadius: BorderRadius.vertical(
          top: Radius.circular(AppRadius.large),
        ),
      ),
      builder: (ctx) => _PermissionGuideSheet(item: item),
    );
  }

  @override
  Widget build(BuildContext context) {
    return AmitiaScaffold(
      appBar: AmitiaAppBar(title: '系统权限', showBackButton: true),
      body: ListView.separated(
        padding: const EdgeInsets.symmetric(
          vertical: AppSpacing.md,
          horizontal: AppSpacing.pagePadding,
        ),
        itemCount: _permissions.length,
        separatorBuilder: (_, _) => const SizedBox(height: AppSpacing.sm),
        itemBuilder: (context, index) {
          final item = _permissions[index];
          return _PermissionCard(
            item: item,
            badgeType: _badgeType(item.status),
            onTap: () => _showGuide(item),
          );
        },
      ),
    );
  }
}

class _PermissionCard extends StatelessWidget {
  final PermissionItem item;
  final BadgeType badgeType;
  final VoidCallback onTap;

  const _PermissionCard({
    required this.item,
    required this.badgeType,
    required this.onTap,
  });

  @override
  Widget build(BuildContext context) {
    return GestureDetector(
      onTap: onTap,
      child: Container(
        padding: const EdgeInsets.all(AppSpacing.cardPadding),
        decoration: BoxDecoration(
          color: context.surfacePrimary,
          borderRadius: AppRadius.brMedium,
          border: Border.all(color: context.borderPrimary, width: 0.5),
        ),
        child: Row(
          children: [
            Container(
              width: 40,
              height: 40,
              decoration: BoxDecoration(
                color: context.accentSoft,
                shape: BoxShape.circle,
              ),
              child: Icon(item.icon, size: 20, color: context.accentPrimary),
            ),
            const SizedBox(width: AppSpacing.md),
            Expanded(
              child: Column(
                crossAxisAlignment: CrossAxisAlignment.start,
                children: [
                  Text(item.name, style: AppTypography.cardTitle(context)),
                  const SizedBox(height: 2),
                  Text(
                    item.description,
                    style: AppTypography.label(context),
                    maxLines: 2,
                    overflow: TextOverflow.ellipsis,
                  ),
                ],
              ),
            ),
            const SizedBox(width: AppSpacing.sm),
            AmitiaStatusBadge(label: item.status, type: badgeType),
          ],
        ),
      ),
    );
  }
}

class _PermissionGuideSheet extends StatelessWidget {
  final PermissionItem item;

  const _PermissionGuideSheet({required this.item});

  @override
  Widget build(BuildContext context) {
    final steps = <String>[
      '打开系统设置',
      '找到「应用管理」',
      '选择 Amitia',
      '找到「${item.name}」并开启',
    ];
    return SafeArea(
      child: Padding(
        padding: const EdgeInsets.fromLTRB(
          AppSpacing.lg,
          AppSpacing.lg,
          AppSpacing.lg,
          AppSpacing.xxl,
        ),
        child: Column(
          mainAxisSize: MainAxisSize.min,
          crossAxisAlignment: CrossAxisAlignment.start,
          children: [
            Center(
              child: Container(
                width: 36,
                height: 4,
                decoration: BoxDecoration(
                  color: context.borderPrimary,
                  borderRadius: BorderRadius.circular(2),
                ),
              ),
            ),
            const SizedBox(height: AppSpacing.lg),
            Text(item.name, style: AppTypography.sectionTitle(context)),
            const SizedBox(height: AppSpacing.sm),
            Text(item.description, style: AppTypography.caption(context)),
            const SizedBox(height: AppSpacing.lg),
            ...steps.asMap().entries.map(
              (entry) {
                return Padding(
                  padding: const EdgeInsets.only(bottom: AppSpacing.md),
                  child: Row(
                    crossAxisAlignment: CrossAxisAlignment.start,
                    children: [
                      Container(
                        width: 22,
                        height: 22,
                        decoration: BoxDecoration(
                          color: context.accentSoft,
                          shape: BoxShape.circle,
                        ),
                        child: Center(
                          child: Text(
                            '${entry.key + 1}',
                            style: TextStyle(
                              fontSize: 12,
                              color: context.accentPrimary,
                              fontWeight: FontWeight.w600,
                            ),
                          ),
                        ),
                      ),
                      const SizedBox(width: AppSpacing.md),
                      Expanded(
                        child: Text(
                          entry.value,
                          style: AppTypography.bodySmall(context),
                        ),
                      ),
                    ],
                  ),
                );
              },
            ),
          ],
        ),
      ),
    );
  }
}
