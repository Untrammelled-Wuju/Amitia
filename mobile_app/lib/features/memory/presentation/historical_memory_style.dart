import 'package:flutter/material.dart';
import '../../../app/theme/app_colors.dart';
import '../../../core/widgets/amitia_misc.dart' show BadgeType;

class AppRadius {
  AppRadius._();

  static const double small = 12;
  static const double medium = 16;
  static const double large = 22;
  static const double tag = 10;
  static const double extraSmall = 8;

  static const BorderRadius brSmall = BorderRadius.all(Radius.circular(small));
  static const BorderRadius brMedium = BorderRadius.all(
    Radius.circular(medium),
  );
  static const BorderRadius brLarge = BorderRadius.all(Radius.circular(large));
  static const BorderRadius brTag = BorderRadius.all(Radius.circular(tag));
  static const BorderRadius brExtraSmall = BorderRadius.all(
    Radius.circular(extraSmall),
  );
}

class AppSpacing {
  AppSpacing._();

  static const double xs = 4;
  static const double sm = 8;
  static const double md = 12;
  static const double lg = 16;
  static const double xl = 20;
  static const double xxl = 24;
  static const double xxxl = 32;

  static const double pagePadding = 16;
  static const double cardPadding = 16;
  static const double sectionGap = 24;
  static const double componentGap = 12;
  static const double tightGap = 8;

  static const double listItemMinHeight = 56;
  static const double listItemMaxHeight = 68;
  static const double buttonHeight = 48;
  static const double inputMinHeight = 48;
}

class AppTypography {
  AppTypography._();

  static TextStyle pageTitle(BuildContext context) {
    return TextStyle(
      fontSize: 20,
      fontWeight: FontWeight.w600,
      color: context.textPrimary,
      height: 1.3,
    );
  }

  static TextStyle pageLargeTitle(BuildContext context) {
    return TextStyle(
      fontSize: 24,
      fontWeight: FontWeight.w600,
      color: context.textPrimary,
      height: 1.3,
    );
  }

  static TextStyle sectionTitle(BuildContext context) {
    return TextStyle(
      fontSize: 17,
      fontWeight: FontWeight.w600,
      color: context.textPrimary,
      height: 1.35,
    );
  }

  static TextStyle cardTitle(BuildContext context) {
    return TextStyle(
      fontSize: 16,
      fontWeight: FontWeight.w500,
      color: context.textPrimary,
      height: 1.35,
    );
  }

  static TextStyle body(BuildContext context) {
    return TextStyle(
      fontSize: 15,
      fontWeight: FontWeight.w400,
      color: context.textPrimary,
      height: 1.5,
    );
  }

  static TextStyle bodySmall(BuildContext context) {
    return TextStyle(
      fontSize: 14,
      fontWeight: FontWeight.w400,
      color: context.textPrimary,
      height: 1.5,
    );
  }

  static TextStyle caption(BuildContext context) {
    return TextStyle(
      fontSize: 13,
      fontWeight: FontWeight.w400,
      color: context.textSecondary,
      height: 1.4,
    );
  }

  static TextStyle label(BuildContext context) {
    return TextStyle(
      fontSize: 12,
      fontWeight: FontWeight.w400,
      color: context.textTertiary,
      height: 1.4,
    );
  }

  static TextStyle statusLabel(BuildContext context) {
    return TextStyle(fontSize: 11, fontWeight: FontWeight.w500, height: 1.3);
  }

  static TextStyle button(BuildContext context) {
    return TextStyle(
      fontSize: 15,
      fontWeight: FontWeight.w500,
      color: context.textPrimary,
      height: 1.2,
    );
  }
}

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
            padding: padding ?? const EdgeInsets.all(AppSpacing.cardPadding),
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
      padding: const EdgeInsets.symmetric(horizontal: AppSpacing.pagePadding),
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
