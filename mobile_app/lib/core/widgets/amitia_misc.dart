import 'package:flutter/material.dart';
import '../../app/theme/app_colors.dart';
import '../../app/theme/app_spacing.dart';
import '../../app/theme/app_radius.dart';
import '../../app/theme/app_typography.dart';
import 'amitia_button.dart';

export 'amitia_button.dart';

enum BadgeType { success, warning, error, info, accent, neutral }

class AmitiaStatusBadge extends StatelessWidget {
  final String label;
  final BadgeType type;
  final double fontSize;

  const AmitiaStatusBadge({
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
      decoration: BoxDecoration(
        color: bgColor,
        borderRadius: AppRadius.brTag,
      ),
      child: Text(
        label,
        style: AppTypography.statusLabel(context).copyWith(color: fgColor, fontSize: fontSize),
      ),
    );
  }
}

class AmitiaProgressBar extends StatelessWidget {
  final double progress;
  final double height;
  final Color? color;

  const AmitiaProgressBar({
    super.key,
    required this.progress,
    this.height = 6,
    this.color,
  });

  @override
  Widget build(BuildContext context) {
    return ClipRRect(
      borderRadius: BorderRadius.circular(height / 2),
      child: LinearProgressIndicator(
        value: progress.clamp(0.0, 1.0),
        minHeight: height,
        backgroundColor: context.accentSoft,
        color: color ?? context.accentPrimary,
      ),
    );
  }
}

class AmitiaEmptyState extends StatelessWidget {
  final IconData icon;
  final String title;
  final String? subtitle;
  final String? actionText;
  final VoidCallback? onAction;

  const AmitiaEmptyState({
    super.key,
    required this.icon,
    required this.title,
    this.subtitle,
    this.actionText,
    this.onAction,
  });

  @override
  Widget build(BuildContext context) {
    return Center(
      child: Padding(
        padding: const EdgeInsets.all(AppSpacing.xxl),
        child: Column(
          mainAxisAlignment: MainAxisAlignment.center,
          children: [
            Icon(icon, size: 56, color: context.textTertiary),
            const SizedBox(height: AppSpacing.md),
            Text(title, style: AppTypography.cardTitle(context)),
            if (subtitle != null) ...[
              const SizedBox(height: 4),
              Text(subtitle!, style: AppTypography.caption(context), textAlign: TextAlign.center),
            ],
            if (actionText != null && onAction != null) ...[
              const SizedBox(height: AppSpacing.lg),
              AmitiaButtonOutline(label: actionText!, onPressed: onAction),
            ],
          ],
        ),
      ),
    );
  }
}

class AmitiaButtonOutline extends StatelessWidget {
  final String label;
  final VoidCallback? onPressed;

  const AmitiaButtonOutline({super.key, required this.label, this.onPressed});

  @override
  Widget build(BuildContext context) {
    return GestureDetector(
      onTap: onPressed,
      child: Container(
        padding: const EdgeInsets.symmetric(horizontal: 20, vertical: 10),
        decoration: BoxDecoration(
          border: Border.all(color: context.accentPrimary, width: 1.5),
          borderRadius: AppRadius.brMedium,
        ),
        child: Text(label, style: TextStyle(color: context.accentPrimary, fontSize: 14, fontWeight: FontWeight.w500)),
      ),
    );
  }
}

class AmitiaLoadingState extends StatelessWidget {
  final String? message;

  const AmitiaLoadingState({super.key, this.message});

  @override
  Widget build(BuildContext context) {
    return Center(
      child: Column(
        mainAxisAlignment: MainAxisAlignment.center,
        children: [
          CircularProgressIndicator(strokeWidth: 2.5, color: context.accentPrimary),
          if (message != null) ...[
            const SizedBox(height: AppSpacing.md),
            Text(message!, style: AppTypography.caption(context)),
          ],
        ],
      ),
    );
  }
}

class AmitiaErrorState extends StatelessWidget {
  final String message;
  final VoidCallback? onRetry;

  const AmitiaErrorState({super.key, required this.message, this.onRetry});

  @override
  Widget build(BuildContext context) {
    return Center(
      child: Padding(
        padding: const EdgeInsets.all(AppSpacing.xxl),
        child: Column(
          mainAxisAlignment: MainAxisAlignment.center,
          children: [
            Icon(Icons.error_outline, size: 48, color: context.error),
            const SizedBox(height: AppSpacing.md),
            Text(message, style: AppTypography.body(context), textAlign: TextAlign.center),
            if (onRetry != null) ...[
              const SizedBox(height: AppSpacing.lg),
              AmitiaButton(label: '重试', onPressed: onRetry),
            ],
          ],
        ),
      ),
    );
  }
}

class AmitiaSwitchTile extends StatelessWidget {
  final String title;
  final String? subtitle;
  final bool value;
  final ValueChanged<bool>? onChanged;

  const AmitiaSwitchTile({
    super.key,
    required this.title,
    this.subtitle,
    required this.value,
    this.onChanged,
  });

  @override
  Widget build(BuildContext context) {
    return Padding(
      padding: const EdgeInsets.symmetric(horizontal: AppSpacing.lg, vertical: 4),
      child: Row(
        children: [
          Expanded(
            child: Column(
              crossAxisAlignment: CrossAxisAlignment.start,
              children: [
                Text(title, style: AppTypography.body(context)),
                if (subtitle != null)
                  Padding(
                    padding: const EdgeInsets.only(top: 2),
                    child: Text(subtitle!, style: AppTypography.label(context)),
                  ),
              ],
            ),
          ),
          Switch(value: value, onChanged: onChanged),
        ],
      ),
    );
  }
}

class AmitiaSegmentedControl extends StatelessWidget {
  final List<String> segments;
  final int selectedIndex;
  final ValueChanged<int> onChanged;

  const AmitiaSegmentedControl({
    super.key,
    required this.segments,
    required this.selectedIndex,
    required this.onChanged,
  });

  @override
  Widget build(BuildContext context) {
    return Container(
      padding: const EdgeInsets.all(4),
      decoration: BoxDecoration(
        color: context.surfaceSecondary,
        borderRadius: AppRadius.brMedium,
      ),
      child: Row(
        children: List.generate(segments.length, (i) {
          final isSelected = i == selectedIndex;
          return Expanded(
            child: GestureDetector(
              onTap: () => onChanged(i),
              child: AnimatedContainer(
                duration: const Duration(milliseconds: 200),
                curve: Curves.easeOutCubic,
                padding: const EdgeInsets.symmetric(vertical: 8),
                decoration: BoxDecoration(
                  color: isSelected ? context.surfacePrimary : Colors.transparent,
                  borderRadius: AppRadius.brSmall,
                  boxShadow: isSelected
                      ? [BoxShadow(color: Colors.black.withValues(alpha: 0.04), blurRadius: 4, offset: const Offset(0, 1))]
                      : null,
                ),
                child: Text(
                  segments[i],
                  textAlign: TextAlign.center,
                  style: TextStyle(
                    fontSize: 13,
                    fontWeight: isSelected ? FontWeight.w600 : FontWeight.w400,
                    color: isSelected ? context.accentPrimary : context.textSecondary,
                  ),
                ),
              ),
            ),
          );
        }),
      ),
    );
  }
}
