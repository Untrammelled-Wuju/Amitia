import 'package:flutter/material.dart';
import '../../app/theme/app_colors.dart';
import '../../app/theme/app_spacing.dart';
import '../../app/theme/app_radius.dart';
import '../../app/theme/app_typography.dart';

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

  const AmitiaButton({
    super.key,
    required this.label,
    this.onPressed,
    this.isPrimary = true,
    this.isSecondary = false,
    this.isDestructive = false,
    this.isFullWidth = false,
    this.icon,
    this.width,
    this.height = AppSpacing.buttonHeight,
  });

  @override
  Widget build(BuildContext context) {
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

    return GestureDetector(
      onTap: onPressed,
      child: Container(
        width: isFullWidth ? double.infinity : width,
        height: height,
        decoration: BoxDecoration(
          color: bgColor,
          borderRadius: AppRadius.brMedium,
        ),
        child: Row(
          mainAxisAlignment: MainAxisAlignment.center,
          children: [
            if (icon != null) ...[
              Icon(icon, size: 18, color: fgColor),
              const SizedBox(width: 8),
            ],
            Text(label, style: AppTypography.button(context).copyWith(color: fgColor)),
          ],
        ),
      ),
    );
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
    final btn = GestureDetector(
      onTap: onPressed,
      behavior: HitTestBehavior.opaque,
      child: Tooltip(
        message: tooltip ?? '',
        child: Container(
          width: 40,
          height: 40,
          decoration: BoxDecoration(
            color: backgroundColor ?? Colors.transparent,
            shape: BoxShape.circle,
          ),
          child: Icon(icon, size: size, color: color ?? context.textSecondary),
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
  });

  @override
  Widget build(BuildContext context) {
    return TextField(
      controller: controller,
      obscureText: obscureText,
      maxLines: maxLines,
      readOnly: readOnly,
      focusNode: focusNode,
      onTap: onTap,
      onChanged: onChanged,
      style: AppTypography.body(context),
      decoration: InputDecoration(
        hintText: hintText,
        hintStyle: TextStyle(color: context.textTertiary),
        prefixIcon: prefixIcon,
        suffixIcon: suffixIcon,
        isDense: true,
      ),
    );
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
    return SizedBox(
      height: 44,
      child: TextField(
        controller: controller,
        onChanged: onChanged,
        style: AppTypography.bodySmall(context),
        decoration: InputDecoration(
          hintText: hintText,
          hintStyle: TextStyle(color: context.textTertiary, fontSize: 14),
          prefixIcon: Icon(Icons.search, size: 20, color: context.textTertiary),
          isDense: true,
          contentPadding: const EdgeInsets.symmetric(vertical: 0),
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
