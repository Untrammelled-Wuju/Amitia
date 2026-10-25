import 'package:flutter/material.dart';

import '../../app/theme/app_colors.dart';
import '../../app/theme/app_radius.dart';
import '../../app/theme/app_spacing.dart';
import '../../app/theme/app_typography.dart';
import 'amitia_icons.dart';

class CompactEmptyState extends StatelessWidget {
  const CompactEmptyState({
    super.key,
    required this.icon,
    required this.title,
    required this.message,
    this.actionLabel,
    this.onAction,
  });

  final IconData icon;
  final String title;
  final String message;
  final String? actionLabel;
  final VoidCallback? onAction;

  @override
  Widget build(BuildContext context) {
    return Container(
      padding: EdgeInsets.all(AppSpacing.md),
      decoration: BoxDecoration(
        color: context.surfacePrimary,
        borderRadius: AppRadius.brMedium,
        border: Border.all(color: context.borderPrimary, width: 0.5),
      ),
      child: Row(
        crossAxisAlignment: CrossAxisAlignment.start,
        children: [
          AmitiaIcon.adaptive(icon, size: 22, color: context.textSecondary),
          SizedBox(width: AppSpacing.sm),
          Expanded(
            child: Column(
              crossAxisAlignment: CrossAxisAlignment.start,
              children: [
                Text(
                  title,
                  style: AppTypography.body(
                    context,
                  ).copyWith(fontSize: 14.5, fontWeight: FontWeight.w400),
                ),
                const SizedBox(height: 3),
                Text(message, style: AppTypography.caption(context)),
                if (actionLabel != null && onAction != null) ...[
                  SizedBox(height: AppSpacing.xs),
                  TextButton(
                    onPressed: onAction,
                    style: TextButton.styleFrom(
                      minimumSize: const Size(0, 36),
                      padding: const EdgeInsets.symmetric(horizontal: 4),
                      tapTargetSize: MaterialTapTargetSize.shrinkWrap,
                    ),
                    child: Text(actionLabel!),
                  ),
                ],
              ],
            ),
          ),
        ],
      ),
    );
  }
}
