import 'package:flutter/material.dart';

import '../../app/theme/app_colors.dart';
import '../../app/theme/app_radius.dart';
import '../../app/theme/app_typography.dart';
import 'amitia_misc.dart';
import 'amitia_icons.dart';
import 'character_avatar.dart';

class AmitiaCharacterCard extends StatelessWidget {
  const AmitiaCharacterCard({
    super.key,
    required this.name,
    required this.status,
    required this.identity,
    required this.avatarInitial,
    required this.avatarColor,
    this.avatarUrl = '',
    this.characterId = '',
    required this.mood,
    required this.lastActive,
    this.onTap,
  });

  final String name;
  final String status;
  final String identity;
  final String avatarInitial;
  final String avatarColor;
  final String avatarUrl;
  final String characterId;
  final String mood;
  final String lastActive;
  final VoidCallback? onTap;

  @override
  Widget build(BuildContext context) {
    final color = Color(
      int.parse('FF${avatarColor.replaceAll('#', '')}', radix: 16),
    );
    final normalizedStatus = status.toLowerCase();
    final isOnline =
        normalizedStatus == '在线' ||
        normalizedStatus == 'enabled' ||
        normalizedStatus == 'online';
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
            Stack(
              children: [
                CharacterAvatar(
                  characterId: characterId,
                  avatar: avatarUrl,
                  initial: avatarInitial,
                  size: 52,
                  color: color,
                ),
                if (isOnline)
                  Positioned(
                    right: 0,
                    bottom: 0,
                    child: Container(
                      width: 14,
                      height: 14,
                      decoration: BoxDecoration(
                        color: context.success,
                        shape: BoxShape.circle,
                        border: Border.all(
                          color: context.surfacePrimary,
                          width: 2,
                        ),
                      ),
                    ),
                  ),
              ],
            ),
            const SizedBox(width: 14),
            Expanded(
              child: Column(
                crossAxisAlignment: CrossAxisAlignment.start,
                children: [
                  Row(
                    children: [
                      Text(name, style: AppTypography.cardTitle(context)),
                      const SizedBox(width: 8),
                      AmitiaStatusBadge(
                        label: status,
                        type: isOnline ? BadgeType.success : BadgeType.neutral,
                      ),
                    ],
                  ),
                  const SizedBox(height: 3),
                  Text(identity, style: AppTypography.caption(context)),
                  const SizedBox(height: 3),
                  Row(
                    children: [
                      Text('心情：$mood', style: AppTypography.label(context)),
                      const SizedBox(width: 12),
                      Text(lastActive, style: AppTypography.label(context)),
                    ],
                  ),
                ],
              ),
            ),
            AmitiaIcon(
              AmitiaIconGlyph.chevronRight,
              color: context.textTertiary,
              size: 20,
            ),
          ],
        ),
      ),
    );
  }
}
