import 'package:flutter/material.dart';
import 'design_tokens.dart';

class AppColors {
  AppColors._();

  static const light = _LightColors();
  static const dark = _DarkColors();

  // Static accessors defaulting to light theme
  static Color get success => light.success;
  static Color get warning => light.warning;
  static Color get error => light.error;
  static Color get info => light.info;
}

class _LightColors {
  const _LightColors();

  final Color backgroundPrimary = const Color(0xFFFFFFFF);
  final Color backgroundSecondary = const Color(0xFFF5F5F5);
  final Color surfacePrimary = const Color(0xFFFFFFFF);
  final Color surfaceSecondary = const Color(0xFFF8F8F8);

  final Color accentPrimary = const Color(0xFF8A5728);
  final Color accentSecondary = const Color(0xFFA67547);
  final Color accentSoft = const Color(0xFFF1E4D4);
  final Color accentPressed = const Color(0xFF5E3516);

  final Color textPrimary = const Color(0xFF1F2028);
  final Color textSecondary = const Color(0xFF6F707B);
  final Color textTertiary = const Color(0xFFA2A3AC);
  final Color textDisabled = const Color(0xFFC5C6CD);

  final Color borderPrimary = const Color(0xFFE8E8E8);
  final Color borderSecondary = const Color(0xFFF0F0F0);

  final Color success = const Color(0xFF3F7653);
  final Color warning = const Color(0xFF8A5A18);
  final Color error = const Color(0xFFA83F3F);
  final Color info = const Color(0xFF4E6C82);

  final Color scrim = const Color(0x66000000);
  final Color overlay = const Color(0x1A000000);
}

class _DarkColors {
  const _DarkColors();

  final Color backgroundPrimary = const Color(0xFF111217);
  final Color backgroundSecondary = const Color(0xFF17181E);
  final Color surfacePrimary = const Color(0xFF1D1E25);
  final Color surfaceSecondary = const Color(0xFF24252D);

  final Color accentPrimary = const Color(0xFFC99557);
  final Color accentSecondary = const Color(0xFFD4AA76);
  final Color accentSoft = const Color(0x1FC99557);
  final Color accentPressed = const Color(0xFFAD773D);

  final Color textPrimary = const Color(0xFFF2F2F5);
  final Color textSecondary = const Color(0xFFB1B2BB);
  final Color textTertiary = const Color(0xFF7E7F89);
  final Color textDisabled = const Color(0xFF4A4B52);

  final Color borderPrimary = const Color(0xFF2D2E36);
  final Color borderSecondary = const Color(0xFF25262D);

  final Color success = const Color(0xFF75A184);
  final Color warning = const Color(0xFFC99A56);
  final Color error = const Color(0xFFC96E6A);
  final Color info = const Color(0xFF9A9E99);

  final Color scrim = const Color(0x99000000);
  final Color overlay = const Color(0x33000000);
}

AmitiaColorTokens defaultLightColorTokens() => AmitiaColorTokens(
  backgroundPrimary: AppColors.light.backgroundPrimary, backgroundSecondary: AppColors.light.backgroundSecondary,
  surfacePrimary: AppColors.light.surfacePrimary, surfaceSecondary: AppColors.light.surfaceSecondary,
  accentPrimary: AppColors.light.accentPrimary, accentSecondary: AppColors.light.accentSecondary, accentSoft: AppColors.light.accentSoft, accentPressed: AppColors.light.accentPressed,
  textPrimary: AppColors.light.textPrimary, textSecondary: AppColors.light.textSecondary, textTertiary: AppColors.light.textTertiary, textDisabled: AppColors.light.textDisabled,
  borderPrimary: AppColors.light.borderPrimary, borderSecondary: AppColors.light.borderSecondary,
  success: AppColors.light.success, warning: AppColors.light.warning, error: AppColors.light.error, info: AppColors.light.info, scrim: AppColors.light.scrim, overlay: AppColors.light.overlay,
);

AmitiaColorTokens defaultDarkColorTokens() => AmitiaColorTokens(
  backgroundPrimary: AppColors.dark.backgroundPrimary, backgroundSecondary: AppColors.dark.backgroundSecondary,
  surfacePrimary: AppColors.dark.surfacePrimary, surfaceSecondary: AppColors.dark.surfaceSecondary,
  accentPrimary: AppColors.dark.accentPrimary, accentSecondary: AppColors.dark.accentSecondary, accentSoft: AppColors.dark.accentSoft, accentPressed: AppColors.dark.accentPressed,
  textPrimary: AppColors.dark.textPrimary, textSecondary: AppColors.dark.textSecondary, textTertiary: AppColors.dark.textTertiary, textDisabled: AppColors.dark.textDisabled,
  borderPrimary: AppColors.dark.borderPrimary, borderSecondary: AppColors.dark.borderSecondary,
  success: AppColors.dark.success, warning: AppColors.dark.warning, error: AppColors.dark.error, info: AppColors.dark.info, scrim: AppColors.dark.scrim, overlay: AppColors.dark.overlay,
);

extension AppColorExtension on BuildContext {
  bool get isDark => Theme.of(this).brightness == Brightness.dark;
  AmitiaColorTokens get _tokens => Theme.of(this).extension<AmitiaColorTokens>() ?? (isDark ? defaultDarkColorTokens() : defaultLightColorTokens());

  Color get backgroundPrimary => _tokens.backgroundPrimary;
  Color get backgroundSecondary => _tokens.backgroundSecondary;
  Color get surfacePrimary => _tokens.surfacePrimary;
  Color get surfaceSecondary => _tokens.surfaceSecondary;
  Color get accentPrimary => _tokens.accentPrimary;
  Color get accentSecondary => _tokens.accentSecondary;
  Color get accentSoft => _tokens.accentSoft;
  Color get accentPressed => _tokens.accentPressed;
  Color get textPrimary => _tokens.textPrimary;
  Color get textSecondary => _tokens.textSecondary;
  Color get textTertiary => _tokens.textTertiary;
  Color get textDisabled => _tokens.textDisabled;
  Color get borderPrimary => _tokens.borderPrimary;
  Color get borderSecondary => _tokens.borderSecondary;
  Color get success => _tokens.success;
  Color get warning => _tokens.warning;
  Color get error => _tokens.error;
  Color get info => _tokens.info;
  Color get scrim => _tokens.scrim;
  Color get overlay => _tokens.overlay;
  Color get isSelected => _tokens.accentSoft;
}
