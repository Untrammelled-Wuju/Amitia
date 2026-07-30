import 'package:flutter/material.dart';

class AppColors {
  AppColors._();

  static const light = _LightColors();
  static const dark = _DarkColors();
}

class _LightColors {
  const _LightColors();

  final Color backgroundPrimary = const Color(0xFFF8F8FC);
  final Color backgroundSecondary = const Color(0xFFF3F3F9);
  final Color surfacePrimary = const Color(0xFFFFFFFF);
  final Color surfaceSecondary = const Color(0xFFF7F6FC);

  final Color accentPrimary = const Color(0xFF7668EE);
  final Color accentSecondary = const Color(0xFF9C91F5);
  final Color accentSoft = const Color(0xFFF0EEFF);
  final Color accentPressed = const Color(0xFF6456D8);

  final Color textPrimary = const Color(0xFF1F2028);
  final Color textSecondary = const Color(0xFF6F707B);
  final Color textTertiary = const Color(0xFFA2A3AC);
  final Color textDisabled = const Color(0xFFC5C6CD);

  final Color borderPrimary = const Color(0xFFE9E9F0);
  final Color borderSecondary = const Color(0xFFF0F0F5);

  final Color success = const Color(0xFF52B788);
  final Color warning = const Color(0xFFE9A23B);
  final Color error = const Color(0xFFE66767);
  final Color info = const Color(0xFF6C8FEA);

  final Color scrim = const Color(0x66000000);
  final Color overlay = const Color(0x1A000000);
}

class _DarkColors {
  const _DarkColors();

  final Color backgroundPrimary = const Color(0xFF111217);
  final Color backgroundSecondary = const Color(0xFF17181E);
  final Color surfacePrimary = const Color(0xFF1D1E25);
  final Color surfaceSecondary = const Color(0xFF24252D);

  final Color accentPrimary = const Color(0xFF9489FF);
  final Color accentSecondary = const Color(0xFFACA4FF);
  final Color accentSoft = const Color(0xFF2B2845);
  final Color accentPressed = const Color(0xFF7B6FE6);

  final Color textPrimary = const Color(0xFFF2F2F5);
  final Color textSecondary = const Color(0xFFB1B2BB);
  final Color textTertiary = const Color(0xFF7E7F89);
  final Color textDisabled = const Color(0xFF4A4B52);

  final Color borderPrimary = const Color(0xFF2D2E36);
  final Color borderSecondary = const Color(0xFF25262D);

  final Color success = const Color(0xFF52B788);
  final Color warning = const Color(0xFFE9A23B);
  final Color error = const Color(0xFFE66767);
  final Color info = const Color(0xFF6C8FEA);

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
}
