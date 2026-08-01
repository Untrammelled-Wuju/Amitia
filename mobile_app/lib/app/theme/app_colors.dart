import 'package:flutter/material.dart';

class AppColors {
  AppColors._();

  static const light = _LightColors();
  static const dark = _DarkColors();
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

extension AppColorExtension on BuildContext {
  bool get isDark => Theme.of(this).brightness == Brightness.dark;

  Color get backgroundPrimary => isDark ? AppColors.dark.backgroundPrimary : AppColors.light.backgroundPrimary;
  Color get backgroundSecondary => isDark ? AppColors.dark.backgroundSecondary : AppColors.light.backgroundSecondary;
  Color get surfacePrimary => isDark ? AppColors.dark.surfacePrimary : AppColors.light.surfacePrimary;
  Color get surfaceSecondary => isDark ? AppColors.dark.surfaceSecondary : AppColors.light.surfaceSecondary;

  Color get accentPrimary => isDark ? AppColors.dark.accentPrimary : AppColors.light.accentPrimary;
  Color get accentSecondary => isDark ? AppColors.dark.accentSecondary : AppColors.light.accentSecondary;
  Color get accentSoft => isDark ? AppColors.dark.accentSoft : AppColors.light.accentSoft;
  Color get accentPressed => isDark ? AppColors.dark.accentPressed : AppColors.light.accentPressed;

  Color get textPrimary => isDark ? AppColors.dark.textPrimary : AppColors.light.textPrimary;
  Color get textSecondary => isDark ? AppColors.dark.textSecondary : AppColors.light.textSecondary;
  Color get textTertiary => isDark ? AppColors.dark.textTertiary : AppColors.light.textTertiary;
  Color get textDisabled => isDark ? AppColors.dark.textDisabled : AppColors.light.textDisabled;

  Color get borderPrimary => isDark ? AppColors.dark.borderPrimary : AppColors.light.borderPrimary;
  Color get borderSecondary => isDark ? AppColors.dark.borderSecondary : AppColors.light.borderSecondary;

  Color get success => isDark ? AppColors.dark.success : AppColors.light.success;
  Color get warning => isDark ? AppColors.dark.warning : AppColors.light.warning;
  Color get error => isDark ? AppColors.dark.error : AppColors.light.error;
  Color get info => isDark ? AppColors.dark.info : AppColors.light.info;

  Color get scrim => isDark ? AppColors.dark.scrim : AppColors.light.scrim;
  Color get overlay => isDark ? AppColors.dark.overlay : AppColors.light.overlay;

  Color get isSelected => isDark ? AppColors.dark.accentSoft : AppColors.light.accentSoft;
}
