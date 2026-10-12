import 'package:flutter/material.dart';
import '../../app/theme/app_colors.dart';
import '../../app/theme/app_spacing.dart';
import '../../app/theme/app_radius.dart';
import '../../app/theme/app_typography.dart';
import '../../app/theme/design_tokens.dart';
import '../ui_runtime/mobile_style_runtime.dart';
import 'amitia_visual_scope.dart';

class AmitiaButton extends StatelessWidget {
  final String label;
  final VoidCallback? onPressed;
  final bool isPrimary;
  final bool isSecondary;
  final bool isDestructive;
  final bool isFullWidth;
  final IconData? icon;
  final double? width;
  final double height;
  final bool outlined;
  final bool round;
  final String? styleTarget;

  AmitiaButton({
    super.key,
    required this.label,
    this.onPressed,
    this.isPrimary = true,
    this.isSecondary = false,
    this.isDestructive = false,
    this.isFullWidth = false,
    this.icon,
    this.width,
    double? height,
    this.outlined = false,
    this.round = false,
    this.styleTarget,
  }) : height = height ?? AppSpacing.buttonHeight;

  @override
  Widget build(BuildContext context) {
    final variant = context.uiComponentVariant('button');
    double number(String key, double fallback) =>
        variant[key] is num ? (variant[key] as num).toDouble() : fallback;
    final effectiveHeight = number('height', number('minHeight', height));
    final radius = number('radius', AppRadius.medium);
    final iconSize = number('iconSize', 18);
    final gap = number('gap', 8);
    final fontSize = number('fontSize', context.uiTypography.buttonSize);
    final rawWeight = variant['fontWeight'];
    final fontWeight = rawWeight is num
        ? designFontWeight(rawWeight.toInt())
        : designFontWeight(context.uiTypography.buttonWeight);

    Color bgColor;
    Color fgColor;
    if (isDestructive) {
      bgColor = onPressed == null
          ? context.error.withValues(alpha: 0.3)
          : context.error;
      fgColor = Colors.white;
    } else if (isSecondary) {
      bgColor = Color.alphaBlend(
        context.accentSecondary.withValues(alpha: 0.14),
        context.surfacePrimary,
      );
      fgColor = context.accentSecondary;
    } else {
      bgColor = onPressed == null
          ? context.accentPrimary.withValues(alpha: 0.4)
          : context.accentPrimary;
      fgColor = Theme.of(context).colorScheme.onPrimary;
    }
    if (outlined) {
      fgColor = isDestructive ? context.error : context.accentPrimary;
      bgColor = Colors.transparent;
      if (onPressed == null) fgColor = fgColor.withValues(alpha: 0.4);
    }
    final resolvedVariant = isDestructive
        ? 'destructive'
        : isSecondary
        ? 'secondary'
        : outlined
        ? 'outlined'
        : 'primary';
    final resolvedTarget = styleTarget ?? 'button.$resolvedVariant';
    final resolved = context.amitiaStyleFor(
      resolvedTarget,
      states: [onPressed == null ? 'disabled' : 'default'],
    );
    final resolvedRadius = resolved.number('borderRadius');
    final resolvedPadding = resolved.edgeInsets('padding');
    final resolvedHeight = resolved.number('height');
    final resolvedWidth = resolved.number('width');
    final resolvedOpacity = resolved.number('opacity');
    final resolvedFontSize = resolved.number('fontSize');
    final resolvedFontWeight = resolved.number('fontWeight');
    final resolvedBorderWidth = resolved.number('borderWidth');
    fgColor =
        resolved.color('foregroundColor') ??
        resolved.color('textColor') ??
        fgColor;
    bgColor =
        resolved.color('backgroundColor') ??
        resolved.color('background') ??
        bgColor;
    final effectiveRadius = round
        ? effectiveHeight / 2
        : resolvedRadius ?? radius;
    final effectiveWidth = resolvedWidth ?? width;
    final effectiveResolvedHeight = resolvedHeight ?? effectiveHeight;
    final effectiveFontSize = resolvedFontSize ?? fontSize;
    final effectiveFontWeight = resolvedFontWeight == null
        ? fontWeight
        : designFontWeight(resolvedFontWeight.toInt());
    final effectiveBorderWidth =
        resolvedBorderWidth ?? context.uiComponents.borderWidth;
    final preservesComposer = AmitiaVisualScope.preservesComposer(context);
    final effectivePadding =
        resolvedPadding ??
        EdgeInsets.symmetric(
          horizontal: number('paddingX', preservesComposer ? 0 : 16),
          vertical: number('paddingY', preservesComposer ? 0 : 12),
        );

    if (!preservesComposer) {
      final button = SizedBox(
        width: isFullWidth ? double.infinity : effectiveWidth,
        child: TextButton(
          onPressed: onPressed,
          style: TextButton.styleFrom(
            foregroundColor: fgColor,
            backgroundColor: bgColor,
            disabledForegroundColor: context.textDisabled,
            disabledBackgroundColor: outlined
                ? Colors.transparent
                : context.surfaceSecondary,
            minimumSize: Size(
              48,
              effectiveResolvedHeight.clamp(48, double.infinity),
            ),
            padding: effectivePadding,
            shape: RoundedRectangleBorder(
              borderRadius: BorderRadius.circular(effectiveRadius),
              side: outlined
                  ? BorderSide(
                      color: resolved.color('borderColor') ?? fgColor,
                      width: effectiveBorderWidth,
                    )
                  : BorderSide.none,
            ),
          ),
          child: Row(
            mainAxisSize: MainAxisSize.min,
            mainAxisAlignment: MainAxisAlignment.center,
            children: [
              if (icon != null) ...[
                Icon(icon, size: iconSize),
                SizedBox(width: gap),
              ],
              Flexible(
                child: Text(
                  label,
                  textAlign: TextAlign.center,
                  style: AppTypography.button(context).copyWith(
                    color: onPressed == null ? context.textDisabled : fgColor,
                    fontSize: effectiveFontSize,
                    fontWeight: effectiveFontWeight,
                  ),
                ),
              ),
            ],
          ),
        ),
      );
      final opacity = resolvedOpacity ?? variant['opacity'];
      return opacity is num
          ? Opacity(opacity: opacity.toDouble().clamp(0.0, 1.0), child: button)
          : button;
    }

    final button = GestureDetector(
      onTap: onPressed,
      child: Container(
        width: isFullWidth ? double.infinity : effectiveWidth,
        height: effectiveResolvedHeight,
        padding: effectivePadding,
        decoration: BoxDecoration(
          color: bgColor,
          borderRadius: BorderRadius.circular(effectiveRadius),
          border: outlined
              ? Border.all(
                  color: resolved.color('borderColor') ?? fgColor,
                  width: effectiveBorderWidth,
                )
              : null,
        ),
        child: Row(
          mainAxisAlignment: MainAxisAlignment.center,
          children: [
            if (icon != null) ...[
              Icon(icon, size: iconSize, color: fgColor),
              SizedBox(width: gap),
            ],
            Text(
              label,
              style: AppTypography.button(context).copyWith(
                color: fgColor,
                fontSize: effectiveFontSize,
                fontWeight: effectiveFontWeight,
              ),
            ),
          ],
        ),
      ),
    );
    final opacity = resolvedOpacity ?? variant['opacity'];
    return opacity is num
        ? Opacity(
            opacity: opacity.toDouble().clamp(0.0, 1.0).toDouble(),
            child: button,
          )
        : button;
  }
}

class AmitiaIconButton extends StatelessWidget {
  final IconData icon;
  final VoidCallback? onPressed;
  final double size;
  final Color? color;
  final Color? backgroundColor;
  final String? tooltip;
  final String? styleTarget;

  const AmitiaIconButton({
    super.key,
    required this.icon,
    this.onPressed,
    this.size = 22,
    this.color,
    this.backgroundColor,
    this.tooltip,
    this.styleTarget,
  });

  @override
  Widget build(BuildContext context) {
    final variant = context.uiComponentVariant('iconButton');
    double number(String key, double fallback) =>
        variant[key] is num ? (variant[key] as num).toDouble() : fallback;
    final buttonSize = number('height', number('minHeight', 40));
    final style = context.amitiaStyleFor(
      styleTarget ?? 'component.icon_button',
      states: <String>[onPressed == null ? 'disabled' : 'default'],
    );
    final resolvedButtonSize =
        style.number('height') ?? style.number('width') ?? buttonSize;
    final resolvedForeground =
        style.color('iconColor') ??
        style.color('foregroundColor') ??
        color ??
        context.textSecondary;
    final resolvedBackground =
        style.color('backgroundColor') ??
        style.color('background') ??
        backgroundColor;
    final resolvedRadius =
        style.borderRadius('borderRadius') ??
        BorderRadius.circular(number('radius', buttonSize / 2));
    final resolvedIconSize = style.number('iconSize') ?? number('iconSize', size);
    final resolvedPadding = style.edgeInsets('padding');
    if (!AmitiaVisualScope.preservesComposer(context)) {
      final targetSize = resolvedButtonSize.clamp(48.0, double.infinity);
      final button = IconButton(
        onPressed: onPressed,
        tooltip: tooltip,
        constraints: BoxConstraints(
          minWidth: targetSize,
          minHeight: targetSize,
        ),
        padding: resolvedPadding,
        style: IconButton.styleFrom(
          backgroundColor: resolvedBackground,
          foregroundColor: resolvedForeground,
          disabledForegroundColor: context.textDisabled,
          shape: RoundedRectangleBorder(borderRadius: resolvedRadius),
        ),
        icon: Icon(icon, size: resolvedIconSize),
      );
      final opacity = style.number('opacity');
      return opacity == null
          ? button
          : Opacity(
              opacity: opacity.clamp(0.0, 1.0).toDouble(),
              child: button,
            );
    }
    final btn = GestureDetector(
      onTap: onPressed,
      behavior: HitTestBehavior.opaque,
      child: Tooltip(
        message: tooltip ?? '',
        child: Container(
          width: resolvedButtonSize,
          height: resolvedButtonSize,
          padding: resolvedPadding,
          decoration: BoxDecoration(
            color: resolvedBackground ?? Colors.transparent,
            borderRadius: resolvedRadius,
            border: style.has('borderColor') || style.has('borderWidth')
                ? Border.all(
                    color: style.color('borderColor') ?? context.borderPrimary,
                    width: style.number('borderWidth') ?? 1,
                  )
                : null,
          ),
          child: Icon(
            icon,
            size: resolvedIconSize,
            color: resolvedForeground,
          ),
        ),
      ),
    );
    final opacity = style.number('opacity');
    return opacity == null
        ? btn
        : Opacity(
            opacity: opacity.clamp(0.0, 1.0).toDouble(),
            child: btn,
          );
  }
}

class AmitiaTextField extends StatelessWidget {
  final String? hintText;
  final TextEditingController? controller;
  final bool obscureText;
  final int maxLines;
  final Widget? prefixIcon;
  final Widget? suffixIcon;
  final VoidCallback? onTap;
  final bool readOnly;
  final FocusNode? focusNode;
  final ValueChanged<String>? onChanged;
  final TextInputType? keyboardType;
  final int? maxLength;
  final bool showCounter;
  final String? errorText;
  final String? styleTarget;

  const AmitiaTextField({
    super.key,
    this.hintText,
    this.controller,
    this.obscureText = false,
    this.maxLines = 1,
    this.prefixIcon,
    this.suffixIcon,
    this.onTap,
    this.readOnly = false,
    this.focusNode,
    this.onChanged,
    this.keyboardType,
    this.maxLength,
    this.showCounter = false,
    this.errorText,
    this.styleTarget,
  });

  @override
  Widget build(BuildContext context) {
    final variant = context.uiComponentVariant('input');
    double number(String key, double fallback) =>
        variant[key] is num ? (variant[key] as num).toDouble() : fallback;
    final style = context.amitiaStyleFor(
      styleTarget ?? 'text_field',
      states: <String>[focusNode?.hasFocus == true ? 'focus' : 'default'],
    );
    final radius =
        style.borderRadius('borderRadius') ??
        BorderRadius.circular(number('radius', AppRadius.medium));
    final hasBorderStyle =
        style.has('borderColor') ||
        style.has('borderWidth') ||
        style.has('border') ||
        style.has('borderSide');
    final normalBorderFallback = hasBorderStyle
        ? BorderSide(
            color: context.borderPrimary,
            width: style.number('borderWidth') ?? 1,
          )
        : BorderSide.none;
    final borderSide =
        style.borderSide(fallback: normalBorderFallback) ??
        normalBorderFallback;
    final focusedBorderSide =
        style.borderSide(
          fallback: BorderSide(
            color: style.color('borderColor') ?? context.accentPrimary,
            width: number('borderWidth', 1.5),
          ),
        ) ??
        BorderSide(
          color: style.color('borderColor') ?? context.accentPrimary,
          width: number('borderWidth', 1.5),
        );
    final normalBorder = OutlineInputBorder(
      borderRadius: radius,
      borderSide: borderSide,
    );
    final focusedBorder = OutlineInputBorder(
      borderRadius: radius,
      borderSide: focusedBorderSide,
    );
    final textStyle = style.applyTextStyle(
      AppTypography.body(
        context,
      ).copyWith(fontSize: number('fontSize', context.uiTypography.bodySize)),
    );
    final placeholderColor =
        style.color('placeholderColor') ??
        style.color('foregroundColor') ??
        context.textTertiary;
    final hintStyle = style
        .applyTextStyle(TextStyle(color: placeholderColor))
        .copyWith(color: placeholderColor);
    final background =
        style.color('backgroundColor') ?? style.color('background');
    final field = TextField(
      controller: controller,
      obscureText: obscureText,
      maxLines: maxLines,
      readOnly: readOnly,
      focusNode: focusNode,
      onTap: onTap,
      onChanged: onChanged,
      keyboardType: keyboardType,
      maxLength: maxLength,
      cursorColor: style.color('foregroundColor'),
      buildCounter: showCounter
          ? null
          : (_, {required currentLength, required isFocused, maxLength}) =>
                null,
      style: textStyle,
      decoration: InputDecoration(
        hintText: hintText,
        errorText: errorText,
        hintStyle: hintStyle,
        prefixIcon: prefixIcon,
        suffixIcon: suffixIcon,
        isDense: true,
        filled: background != null,
        fillColor: background ?? Colors.transparent,
        contentPadding:
            style.edgeInsets('padding') ??
            EdgeInsets.symmetric(
              horizontal: number('paddingX', AppSpacing.lg),
              vertical: number('paddingY', AppSpacing.md),
            ),
        border: normalBorder,
        enabledBorder: normalBorder,
        focusedBorder: focusedBorder,
        disabledBorder: normalBorder,
        errorBorder: normalBorder,
        focusedErrorBorder: focusedBorder,
      ),
    );
    final minHeight = style.number('height') ?? variant['minHeight'];
    Widget result = minHeight is num
        ? ConstrainedBox(
            constraints: BoxConstraints(minHeight: minHeight.toDouble()),
            child: field,
          )
        : field;
    final width = style.number('width');
    if (width != null) result = SizedBox(width: width, child: result);
    final opacity = style.number('opacity');
    return opacity == null
        ? result
        : Opacity(
            opacity: opacity.toDouble().clamp(0.0, 1.0).toDouble(),
            child: result,
          );
  }
}

class AmitiaSearchField extends StatelessWidget {
  final String hintText;
  final TextEditingController? controller;
  final ValueChanged<String>? onChanged;
  final bool autofocus;

  const AmitiaSearchField({
    super.key,
    this.hintText = '搜索',
    this.controller,
    this.onChanged,
    this.autofocus = false,
  });

  @override
  Widget build(BuildContext context) {
    final variant = context.uiComponentVariant('input');
    double number(String key, double fallback) =>
        variant[key] is num ? (variant[key] as num).toDouble() : fallback;
    final style = context.amitiaStyleFor(
      'text_field.search',
      states: <String>['default'],
    );
    final background =
        style.color('backgroundColor') ?? style.color('background');
    final hasBorderStyle =
        style.has('borderColor') ||
        style.has('borderWidth') ||
        style.has('border') ||
        style.has('borderSide');
    final normalBorderFallback = hasBorderStyle
        ? BorderSide(
            color: context.borderPrimary,
            width: style.number('borderWidth') ?? 1,
          )
        : BorderSide.none;
    final borderSide =
        style.borderSide(fallback: normalBorderFallback) ??
        normalBorderFallback;
    final radius =
        style.borderRadius('borderRadius') ??
        BorderRadius.circular(number('radius', AppRadius.medium));
    final textStyle = style.applyTextStyle(
      AppTypography.bodySmall(context).copyWith(
        fontSize: number('fontSize', context.uiTypography.bodySmallSize),
      ),
    );
    final placeholderColor =
        style.color('placeholderColor') ??
        style.color('foregroundColor') ??
        context.textTertiary;
    final field = ConstrainedBox(
      constraints: BoxConstraints(
        minHeight:
            style.number('height') ?? number('height', number('minHeight', 48)),
      ),
      child: TextField(
        controller: controller,
        onChanged: onChanged,
        autofocus: autofocus,
        cursorColor: style.color('foregroundColor'),
        style: textStyle,
        decoration: InputDecoration(
          hintText: hintText,
          hintStyle: style
              .applyTextStyle(TextStyle(color: placeholderColor, fontSize: 14))
              .copyWith(color: placeholderColor),
          prefixIcon: Icon(
            Icons.search,
            size: number('iconSize', context.uiIcons.medium),
            color: style.color('iconColor') ?? context.textTertiary,
          ),
          isDense: true,
          filled: background != null,
          fillColor: background ?? Colors.transparent,
          contentPadding:
              style.edgeInsets('padding') ??
              EdgeInsets.symmetric(
                horizontal: number('paddingX', 12),
                vertical: number('paddingY', 12),
              ),
          border: OutlineInputBorder(
            borderRadius: radius,
            borderSide: borderSide,
          ),
          enabledBorder: OutlineInputBorder(
            borderRadius: radius,
            borderSide: borderSide,
          ),
          focusedBorder: OutlineInputBorder(
            borderRadius: radius,
            borderSide:
                style.borderSide(
                  fallback: BorderSide(
                    color: style.color('borderColor') ?? context.accentPrimary,
                    width: number('borderWidth', 1.5),
                  ),
                ) ??
                BorderSide(
                  color: style.color('borderColor') ?? context.accentPrimary,
                  width: number('borderWidth', 1.5),
                ),
          ),
        ),
      ),
    );
    final opacity = style.number('opacity');
    return opacity == null
        ? field
        : Opacity(
            opacity: opacity.toDouble().clamp(0.0, 1.0).toDouble(),
            child: field,
          );
  }
}

class AmitiaAvatar extends StatelessWidget {
  final String initial;
  final String colorHex;
  final double size;
  final bool showOnlineStatus;

  const AmitiaAvatar({
    super.key,
    required this.initial,
    required this.colorHex,
    this.size = 48,
    this.showOnlineStatus = false,
  });

  Color get _bgColor => _parseColor(colorHex);

  static Color _parseColor(String hex) {
    final cleaned = hex.replaceAll('#', '');
    return Color(int.parse('FF$cleaned', radix: 16));
  }

  @override
  Widget build(BuildContext context) {
    return SizedBox(
      width: size,
      height: size,
      child: Stack(
        children: [
          Container(
            width: size,
            height: size,
            decoration: BoxDecoration(color: _bgColor, shape: BoxShape.circle),
            child: Center(
              child: Text(
                initial,
                style: TextStyle(
                  color: Colors.white,
                  fontSize: size * 0.4,
                  fontWeight: FontWeight.w600,
                ),
              ),
            ),
          ),
          if (showOnlineStatus)
            Positioned(
              right: 0,
              bottom: 0,
              child: Container(
                width: size * 0.28,
                height: size * 0.28,
                decoration: BoxDecoration(
                  color: const Color(0xFF52B788),
                  shape: BoxShape.circle,
                  border: Border.all(color: context.surfacePrimary, width: 2),
                ),
              ),
            ),
        ],
      ),
    );
  }
}

class ConduitStyleToolbarSurface extends StatelessWidget {
  const ConduitStyleToolbarSurface({super.key, required this.child});

  final Widget child;

  @override
  Widget build(BuildContext context) {
    final isDark = Theme.of(context).brightness == Brightness.dark;

    return SizedBox(
      width: 44,
      height: 44,
      child: DecoratedBox(
        decoration: BoxDecoration(
          shape: BoxShape.circle,
          color: isDark ? const Color(0xFF141414) : const Color(0xFFF4F4F4),
          border: Border.all(
            width: 0.5,
            color: isDark ? const Color(0xFF1E1E1E) : const Color(0xFFE5E5E5),
          ),
        ),
        child: Center(child: child),
      ),
    );
  }
}

class ConduitStyleToolbarButton extends StatelessWidget {
  const ConduitStyleToolbarButton({
    super.key,
    required this.icon,
    required this.onPressed,
    this.iconSize = 22,
    this.tooltip,
  });

  final IconData icon;
  final VoidCallback onPressed;
  final double iconSize;
  final String? tooltip;

  @override
  Widget build(BuildContext context) {
    final isDark = Theme.of(context).brightness == Brightness.dark;

    final button = Material(
      color: Colors.transparent,
      shape: const CircleBorder(),
      clipBehavior: Clip.antiAlias,
      child: InkWell(
        customBorder: const CircleBorder(),
        onTap: onPressed,
        child: ConduitStyleToolbarSurface(
          child: Icon(
            icon,
            size: iconSize,
            color: isDark ? const Color(0xFFECECEC) : const Color(0xFF000000),
          ),
        ),
      ),
    );

    if (tooltip == null || tooltip!.isEmpty) {
      return button;
    }

    return Tooltip(message: tooltip!, child: button);
  }
}
