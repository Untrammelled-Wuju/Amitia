import 'package:flutter/material.dart';
import 'package:flutter/cupertino.dart';
import '../../app/theme/app_colors.dart';
import '../../app/theme/app_spacing.dart';
import '../../app/theme/app_radius.dart';
import '../../app/theme/app_typography.dart';
import 'amitia_button.dart';

export 'amitia_button.dart';
export 'amitia_dialogs.dart';

enum BadgeType { success, warning, error, info, accent, neutral }

class AmitiaStatusBadge extends StatelessWidget {
  final String label;
  final BadgeType type;
  final double fontSize;

  const AmitiaStatusBadge({
    super.key,
    required this.label,
    required this.type,
    this.fontSize = 12,
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
  final double iconSize;

  const AmitiaEmptyState({
    super.key,
    required this.icon,
    required this.title,
    this.subtitle,
    this.actionText,
    this.onAction,
    this.iconSize = 56,
  });

  @override
  Widget build(BuildContext context) {
    return _StateViewport(
      children: [
        Icon(icon, size: iconSize, color: context.textTertiary),
        SizedBox(height: AppSpacing.md),
        Text(
          title,
          style: AppTypography.cardTitle(context),
          textAlign: TextAlign.center,
        ),
        if (subtitle != null) ...[
          const SizedBox(height: 4),
          Text(
            subtitle!,
            style: AppTypography.caption(context),
            textAlign: TextAlign.center,
          ),
        ],
        if (actionText != null && onAction != null) ...[
          SizedBox(height: AppSpacing.lg),
          AmitiaButtonOutline(label: actionText!, onPressed: onAction),
        ],
      ],
    );
  }
}

class AmitiaButtonOutline extends StatelessWidget {
  final String label;
  final VoidCallback? onPressed;

  const AmitiaButtonOutline({super.key, required this.label, this.onPressed});

  @override
  Widget build(BuildContext context) {
    return AmitiaButton(label: label, onPressed: onPressed, outlined: true);
  }
}

class AmitiaLoadingState extends StatelessWidget {
  final String? message;

  const AmitiaLoadingState({super.key, this.message});

  @override
  Widget build(BuildContext context) {
    return _StateViewport(
      children: [
        CircularProgressIndicator(
          strokeWidth: 2.5,
          color: context.accentPrimary,
        ),
        if (message != null) ...[
          SizedBox(height: AppSpacing.md),
          Text(message!, style: AppTypography.caption(context)),
        ],
      ],
    );
  }
}

class AmitiaErrorState extends StatelessWidget {
  final String message;
  final VoidCallback? onRetry;

  const AmitiaErrorState({super.key, required this.message, this.onRetry});

  @override
  Widget build(BuildContext context) {
    return _StateViewport(
      children: [
        Icon(Icons.error_outline, size: 48, color: context.error),
        SizedBox(height: AppSpacing.md),
        Text(
          message,
          style: AppTypography.body(context),
          textAlign: TextAlign.center,
        ),
        if (onRetry != null) ...[
          SizedBox(height: AppSpacing.lg),
          AmitiaButton(label: '重试', onPressed: onRetry),
        ],
      ],
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
    return SwitchListTile.adaptive(
      contentPadding: EdgeInsets.symmetric(
        horizontal: AppSpacing.lg,
        vertical: 4,
      ),
      title: Text(title, style: AppTypography.body(context)),
      subtitle: subtitle == null
          ? null
          : Text(subtitle!, style: AppTypography.caption(context)),
      value: value,
      onChanged: onChanged,
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
    if (segments.isEmpty) return const SizedBox.shrink();
    final style = AppTypography.bodySmall(
      context,
    ).copyWith(fontWeight: FontWeight.w500);
    return LayoutBuilder(
      builder: (context, constraints) {
        double requiredWidth = 8;
        for (final segment in segments) {
          final painter = TextPainter(
            text: TextSpan(text: segment, style: style),
            textDirection: Directionality.of(context),
            textScaler: MediaQuery.textScalerOf(context),
            maxLines: 1,
          )..layout();
          final segmentWidth = painter.width + 40;
          if (segmentWidth > requiredWidth) requiredWidth = segmentWidth;
          painter.dispose();
        }
        requiredWidth = requiredWidth * segments.length + 8;
        final availableWidth = constraints.hasBoundedWidth
            ? constraints.maxWidth
            : requiredWidth;
        final width = requiredWidth > availableWidth
            ? requiredWidth
            : availableWidth;
        final children = {
          for (var i = 0; i < segments.length; i++)
            i: ConstrainedBox(
              constraints: const BoxConstraints(minHeight: 44),
              child: Center(
                child: Text(
                  segments[i],
                  style: style.copyWith(
                    color: i == selectedIndex
                        ? context.textPrimary
                        : context.textSecondary,
                  ),
                ),
              ),
            ),
        };
        void select(int value) {
          if (value != selectedIndex) onChanged(value);
        }

        final control = SizedBox(
          width: width,
          child: requiredWidth > availableWidth
              ? CupertinoSegmentedControl<int>(
                  groupValue: selectedIndex.clamp(0, segments.length - 1),
                  borderColor: context.surfaceSecondary,
                  unselectedColor: context.surfaceSecondary,
                  selectedColor: context.surfacePrimary,
                  pressedColor: context.accentSoft,
                  onValueChanged: select,
                  children: children,
                )
              : CupertinoSlidingSegmentedControl<int>(
                  groupValue: selectedIndex.clamp(0, segments.length - 1),
                  backgroundColor: context.surfaceSecondary,
                  thumbColor: context.surfacePrimary,
                  onValueChanged: (value) {
                    if (value != null) select(value);
                  },
                  children: children,
                ),
        );
        return requiredWidth > availableWidth
            ? SingleChildScrollView(
                scrollDirection: Axis.horizontal,
                child: control,
              )
            : control;
      },
    );
  }
}

class _StateViewport extends StatelessWidget {
  const _StateViewport({required this.children});

  final List<Widget> children;

  @override
  Widget build(BuildContext context) {
    return LayoutBuilder(
      builder: (context, constraints) {
        return SingleChildScrollView(
          child: ConstrainedBox(
            constraints: BoxConstraints(
              minHeight: constraints.hasBoundedHeight
                  ? constraints.maxHeight
                  : 0,
            ),
            child: Padding(
              padding: EdgeInsets.all(AppSpacing.xxl),
              child: Column(
                mainAxisSize: MainAxisSize.min,
                mainAxisAlignment: MainAxisAlignment.center,
                children: children,
              ),
            ),
          ),
        );
      },
    );
  }
}
