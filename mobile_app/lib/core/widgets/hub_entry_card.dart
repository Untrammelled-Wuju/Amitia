import 'package:flutter/material.dart';

import '../../app/theme/app_colors.dart';
import '../../app/theme/app_radius.dart';
import '../../app/theme/app_spacing.dart';
import '../../app/theme/app_typography.dart';
import 'amitia_icons.dart';

class HubSectionPlaceholder extends StatelessWidget {
  const HubSectionPlaceholder({super.key, required this.message});

  final String message;

  @override
  Widget build(BuildContext context) {
    return Padding(
      padding: EdgeInsets.symmetric(vertical: AppSpacing.lg),
      child: Center(
        child: Text(
          message,
          style: AppTypography.caption(
            context,
          ).copyWith(color: context.textTertiary),
        ),
      ),
    );
  }
}

class HubEntryCard extends StatelessWidget {
  const HubEntryCard({
    super.key,
    required this.icon,
    required this.title,
    required this.onTap,
    this.description = '',
    this.showDivider = false,
  });

  final AmitiaIconGlyph icon;
  final String title;
  final String description;
  final VoidCallback onTap;
  final bool showDivider;

  @override
  Widget build(BuildContext context) {
    final hasDescription = description.trim().isNotEmpty;
    return Material(
      color: Colors.transparent,
      child: InkWell(
        onTap: onTap,
        borderRadius: AppRadius.brMedium,
        child: Container(
          height: hasDescription ? 60 : 56,
          padding: EdgeInsets.symmetric(
            horizontal: AppSpacing.md,
            vertical: AppSpacing.sm,
          ),
          decoration: BoxDecoration(
            border: showDivider
                ? Border(
                    bottom: BorderSide(
                      color: context.borderSecondary,
                      width: 0.5,
                    ),
                  )
                : null,
          ),
          child: Row(
            children: [
              AmitiaIcon(icon, color: context.textSecondary, size: 20),
              SizedBox(width: AppSpacing.sm),
              Expanded(
                child: hasDescription
                    ? Column(
                        crossAxisAlignment: CrossAxisAlignment.start,
                        mainAxisSize: MainAxisSize.min,
                        children: [
                          Text(
                            title,
                            style: AppTypography.body(context).copyWith(
                              fontSize: 14.5,
                              fontWeight: FontWeight.w400,
                            ),
                          ),
                          const SizedBox(height: 2),
                          Text(
                            description,
                            style: AppTypography.caption(context),
                            maxLines: 1,
                            overflow: TextOverflow.ellipsis,
                          ),
                        ],
                      )
                    : Align(
                        alignment: Alignment.centerLeft,
                        child: Text(
                          title,
                          style: AppTypography.body(context).copyWith(
                            fontSize: 14.5,
                            fontWeight: FontWeight.w400,
                          ),
                        ),
                      ),
              ),
              SizedBox(width: AppSpacing.sm),
              AmitiaIcon(
                AmitiaIconGlyph.chevronRight,
                color: context.textTertiary,
                size: 18,
              ),
            ],
          ),
        ),
      ),
    );
  }
}
