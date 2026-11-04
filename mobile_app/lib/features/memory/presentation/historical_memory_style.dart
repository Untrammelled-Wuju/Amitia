import 'package:flutter/material.dart';
import '../../../app/theme/app_colors.dart';
import '../../../app/theme/app_radius.dart';
import '../../../app/theme/app_spacing.dart';
import '../../../app/theme/app_typography.dart';
import '../../../core/widgets/amitia_misc.dart' show BadgeType;

class HistoricalMemoryBadge extends StatelessWidget {
  final String label;
  final BadgeType type;
  final double fontSize;

  const HistoricalMemoryBadge({
    super.key,
    required this.label,
    required this.type,
    this.fontSize = 11,
  });

  @override
  Widget build(BuildContext context) {
    Color bgColor;
    Color fgColor;
    switch (type) {
      case BadgeType.success:
        bgColor = context.success.withValues(alpha: 0.12);
        fgColor = context.success;
      case BadgeType.warning:
        bgColor = context.warning.withValues(alpha: 0.12);
        fgColor = context.warning;
      case BadgeType.error:
        bgColor = context.error.withValues(alpha: 0.12);
        fgColor = context.error;
      case BadgeType.info:
        bgColor = context.info.withValues(alpha: 0.12);
        fgColor = context.info;
      case BadgeType.accent:
        bgColor = context.accentSoft;
        fgColor = context.accentPrimary;
      case BadgeType.neutral:
        bgColor = context.borderPrimary;
        fgColor = context.textSecondary;
    }
    return Container(
      padding: const EdgeInsets.symmetric(horizontal: 8, vertical: 3),
      decoration: BoxDecoration(color: bgColor, borderRadius: AppRadius.brTag),
      child: Text(
        label,
        style: AppTypography.statusLabel(
          context,
        ).copyWith(color: fgColor, fontSize: fontSize),
      ),
    );
  }
}

class HistoricalMemorySurface extends StatelessWidget {
  final Widget child;
  final EdgeInsets? padding;
  final EdgeInsets? margin;
  final VoidCallback? onTap;
  final Color? backgroundColor;
  final Border? border;
  final BorderRadius? borderRadius;

  const HistoricalMemorySurface({
    super.key,
    required this.child,
    this.padding,
    this.margin,
    this.onTap,
    this.backgroundColor,
    this.border,
    this.borderRadius,
  });

  @override
  Widget build(BuildContext context) {
    return GestureDetector(
      onTap: onTap,
      child: Container(
        margin: margin,
        child: Material(
          color: backgroundColor ?? context.surfacePrimary,
          borderRadius: borderRadius ?? AppRadius.brMedium,
          clipBehavior: Clip.antiAlias,
          child: Container(
            padding: padding ?? EdgeInsets.all(AppSpacing.cardPadding),
            decoration: BoxDecoration(
              borderRadius: borderRadius ?? AppRadius.brMedium,
              border:
                  border ??
                  Border.all(color: context.borderPrimary, width: 0.5),
            ),
            child: child,
          ),
        ),
      ),
    );
  }
}

class HistoricalMemorySectionHeader extends StatelessWidget {
  final String title;
  final String? actionText;
  final VoidCallback? onAction;

  const HistoricalMemorySectionHeader({
    super.key,
    required this.title,
    this.actionText,
    this.onAction,
  });

  @override
  Widget build(BuildContext context) {
    return Padding(
      padding: EdgeInsets.symmetric(horizontal: AppSpacing.pagePadding),
      child: Row(
        mainAxisAlignment: MainAxisAlignment.spaceBetween,
        children: [
          Text(title, style: AppTypography.sectionTitle(context)),
          if (actionText != null)
            GestureDetector(
              onTap: onAction,
              child: Text(
                actionText!,
                style: TextStyle(
                  fontSize: 13,
                  color: context.accentPrimary,
                  fontWeight: FontWeight.w500,
                ),
              ),
            ),
        ],
      ),
    );
  }
}
