import 'package:flutter/material.dart';
import '../../core/settings/appearance_preferences.dart';
import '../../core/settings/custom_palette.dart';
import 'design_tokens.dart';

AmitiaColorTokens resolveAppearanceColors(
  AmitiaColorTokens base,
  Brightness brightness,
  AppearancePreferences appearance,
) {
  const light = [
    Color(0xFF8A5728),
    Color(0xFF6C8FEA),
    Color(0xFF52B788),
    Color(0xFFE9A23B),
  ];
  const dark = [
    Color(0xFF9C8068),
    Color(0xFF8CA8F0),
    Color(0xFF78C99A),
    Color(0xFFF0B65D),
  ];
  final palette = appearance.customPalette;
  final accent = palette.enabled
      ? palette.primary
      : (brightness == Brightness.dark
            ? dark
            : light)[appearance.accentColorIndex];
  final text = palette.enabled
      ? paletteForeground(palette, base.surfacePrimary)
      : base.textPrimary;
  Color supportingText(double amount) {
    final candidate = Color.lerp(text, base.surfacePrimary, amount)!;
    return colorContrast(candidate, base.surfacePrimary) >= 4.5 ? candidate : text;
  }
  return base.copyWith(
    accentPrimary: accent,
    accentSecondary: palette.enabled ? palette.secondary : accent,
    accentPressed: Color.lerp(
      accent,
      brightness == Brightness.dark ? Colors.white : Colors.black,
      0.16,
    ),
    accentSoft: Color.alphaBlend(
      accent.withValues(alpha: 0.14),
      base.surfacePrimary,
    ),
    textPrimary: text,
    textSecondary: palette.enabled ? supportingText(0.2) : base.textSecondary,
    textTertiary: palette.enabled ? supportingText(0.35) : base.textTertiary,
    textDisabled: palette.enabled
        ? Color.lerp(text, base.surfacePrimary, 0.5)
        : base.textDisabled,
  );
}
