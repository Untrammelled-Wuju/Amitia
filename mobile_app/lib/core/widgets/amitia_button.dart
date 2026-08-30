import 'package:flutter/cupertino.dart';
import 'package:flutter/material.dart';
import '../../app/theme/app_colors.dart';
import '../../app/theme/app_spacing.dart';
import '../../app/theme/app_radius.dart';
import '../../app/theme/app_typography.dart';
import '../../app/theme/design_tokens.dart';

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
  }) : height = height ?? AppSpacing.buttonHeight;

  @override
  Widget build(BuildContext context) {
    final variant = context.uiComponentVariant('button');
    double number(String key, double fallback) =>
        variant[key] is num ? (variant[key] as num).toDouble() : fallback;
    final effectiveHeight = number('height', number('minHeight', height));
    final radius = number('radius', 14);
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
      bgColor = onPressed == null ? context.error.withValues(alpha: 0.3) : context.error;
      fgColor = Colors.white;
    } else if (isSecondary) {
      bgColor = context.accentSoft;
      fgColor = context.accentPrimary;
    } else {
      bgColor = onPressed == null ? context.accentPrimary.withValues(alpha: 0.4) : context.accentPrimary;
      fgColor = Colors.white;
    }

    final button = GestureDetector(
      onTap: onPressed,
      child: Container(
        width: isFullWidth ? double.infinity : width,
        height: effectiveHeight,
        padding: EdgeInsets.symmetric(
          horizontal: number('paddingX', 0),
          vertical: number('paddingY', 0),
        ),
        decoration: BoxDecoration(
          color: bgColor,
          borderRadius: BorderRadius.circular(radius),
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
                fontSize: fontSize,
                fontWeight: fontWeight,
              ),
            ),
          ],
        ),
      ),
    );
    final opacity = variant['opacity'];
    return opacity is num
        ? Opacity(opacity: opacity.toDouble().clamp(0.0, 1.0).toDouble(), child: button)
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

  const AmitiaIconButton({
    super.key,
    required this.icon,
    this.onPressed,
    this.size = 22,
    this.color,
    this.backgroundColor,
    this.tooltip,
  });

  @override
  Widget build(BuildContext context) {
    final variant = context.uiComponentVariant('iconButton');
    double number(String key, double fallback) =>
        variant[key] is num ? (variant[key] as num).toDouble() : fallback;
    final buttonSize = number('height', number('minHeight', 40));
    final btn = GestureDetector(
      onTap: onPressed,
      behavior: HitTestBehavior.opaque,
      child: Tooltip(
        message: tooltip ?? '',
        child: Container(
          width: buttonSize,
          height: buttonSize,
          decoration: BoxDecoration(
            color: backgroundColor ?? context.surfacePrimary,
            borderRadius: BorderRadius.circular(number('radius', 13)),
            border: Border.all(
              color: context.borderPrimary,
              width: number('borderWidth', .8),
            ),
          ),
          child: Icon(
            icon,
            size: number('iconSize', size),
            color: color ?? context.textSecondary,
          ),
        ),
      ),
    );
    return btn;
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
  });

  @override
  Widget build(BuildContext context) {
    final variant = context.uiComponentVariant('input');
    double number(String key, double fallback) =>
        variant[key] is num ? (variant[key] as num).toDouble() : fallback;
    final radius = number('radius', 14);
    final normalBorder = OutlineInputBorder(
      borderRadius: BorderRadius.circular(radius),
      borderSide: BorderSide.none,
    );
    final focusedBorder = OutlineInputBorder(
      borderRadius: BorderRadius.circular(radius),
      borderSide: BorderSide(
        color: context.accentPrimary,
        width: number('borderWidth', 1.5),
      ),
    );
    final field = TextField(
      controller: controller,
      obscureText: obscureText,
      maxLines: maxLines,
      readOnly: readOnly,
      focusNode: focusNode,
      onTap: onTap,
      onChanged: onChanged,
      keyboardType: keyboardType,
      style: AppTypography.body(context).copyWith(
        fontSize: number('fontSize', context.uiTypography.bodySize),
      ),
      decoration: InputDecoration(
        hintText: hintText,
        hintStyle: TextStyle(color: context.textTertiary),
        prefixIcon: prefixIcon,
        suffixIcon: suffixIcon,
        isDense: true,
        contentPadding: EdgeInsets.symmetric(
          horizontal: number('paddingX', AppSpacing.lg),
          vertical: number('paddingY', AppSpacing.md),
        ),
        border: normalBorder,
        enabledBorder: normalBorder,
        focusedBorder: focusedBorder,
      ),
    );
    final minHeight = variant['minHeight'];
    return minHeight is num
        ? ConstrainedBox(
            constraints: BoxConstraints(minHeight: minHeight.toDouble()),
            child: field,
          )
        : field;
  }
}

class AmitiaSearchField extends StatelessWidget {
  final String hintText;
  final TextEditingController? controller;
  final ValueChanged<String>? onChanged;

  const AmitiaSearchField({
    super.key,
    this.hintText = '搜索',
    this.controller,
    this.onChanged,
  });

  @override
  Widget build(BuildContext context) {
    final variant = context.uiComponentVariant('input');
    double number(String key, double fallback) =>
        variant[key] is num ? (variant[key] as num).toDouble() : fallback;
    return SizedBox(
      height: number('height', number('minHeight', 44)),
      child: TextField(
        controller: controller,
        onChanged: onChanged,
        style: AppTypography.bodySmall(context).copyWith(
          fontSize: number('fontSize', context.uiTypography.bodySmallSize),
        ),
        decoration: InputDecoration(
          hintText: hintText,
          hintStyle: TextStyle(color: context.textTertiary, fontSize: 14),
          prefixIcon: Icon(
            Icons.search,
            size: number('iconSize', context.uiIcons.medium),
            color: context.textTertiary,
          ),
          isDense: true,
          contentPadding: EdgeInsets.symmetric(
            horizontal: number('paddingX', 0),
            vertical: number('paddingY', 0),
          ),
          border: OutlineInputBorder(
            borderRadius: BorderRadius.circular(number('radius', AppRadius.medium)),
            borderSide: BorderSide.none,
          ),
        ),
      ),
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
            decoration: BoxDecoration(
              color: _bgColor,
              shape: BoxShape.circle,
            ),
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
  const ConduitStyleToolbarSurface({
    super.key,
    required this.child,
  });

  final Widget child;

  @override
  Widget build(BuildContext context) {
    final isDark =
        Theme.of(context).brightness == Brightness.dark;

    return SizedBox(
      width: 44,
      height: 44,
      child: DecoratedBox(
        decoration: BoxDecoration(
          shape: BoxShape.circle,
          color: isDark
              ? const Color(0xFF141414)
              : const Color(0xFFF4F4F4),
          border: Border.all(
            width: 0.5,
            color: isDark
                ? const Color(0xFF1E1E1E)
                : const Color(0xFFE5E5E5),
          ),
        ),
        child: Center(
          child: child,
        ),
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
    final isDark =
        Theme.of(context).brightness == Brightness.dark;

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
            color: isDark
                ? const Color(0xFFECECEC)
                : const Color(0xFF000000),
          ),
        ),
      ),
    );

    if (tooltip == null || tooltip!.isEmpty) {
      return button;
    }

    return Tooltip(
      message: tooltip!,
      child: button,
    );
  }
}
