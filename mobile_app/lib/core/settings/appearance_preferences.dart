import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:shared_preferences/shared_preferences.dart';

@immutable
class AppearancePreferences {
  final ThemeMode themeMode;
  final double fontScale;
  final int accentColorIndex;
  final int cornerStyleIndex;
  final bool dynamicEffect;
  final bool reduceAnimation;

  const AppearancePreferences({
    this.themeMode = ThemeMode.light,
    this.fontScale = 1.0,
    this.accentColorIndex = 0,
    this.cornerStyleIndex = 1,
    this.dynamicEffect = true,
    this.reduceAnimation = false,
  });

  AppearancePreferences copyWith({
    ThemeMode? themeMode,
    double? fontScale,
    int? accentColorIndex,
    int? cornerStyleIndex,
    bool? dynamicEffect,
    bool? reduceAnimation,
  }) {
    return AppearancePreferences(
      themeMode: themeMode ?? this.themeMode,
      fontScale: fontScale ?? this.fontScale,
      accentColorIndex: accentColorIndex ?? this.accentColorIndex,
      cornerStyleIndex: cornerStyleIndex ?? this.cornerStyleIndex,
      dynamicEffect: dynamicEffect ?? this.dynamicEffect,
      reduceAnimation: reduceAnimation ?? this.reduceAnimation,
    );
  }
}

final appearancePreferencesProvider =
    StateNotifierProvider<AppearancePreferencesNotifier, AppearancePreferences>(
  (ref) => AppearancePreferencesNotifier(),
);

class AppearancePreferencesNotifier extends StateNotifier<AppearancePreferences> {
  AppearancePreferencesNotifier() : super(const AppearancePreferences());

  static const _themeKey = 'appearance.themeMode';
  static const _fontScaleKey = 'appearance.fontScale';
  static const _accentKey = 'appearance.accentColorIndex';
  static const _cornerKey = 'appearance.cornerStyleIndex';
  static const _dynamicKey = 'appearance.dynamicEffect';
  static const _reduceAnimationKey = 'appearance.reduceAnimation';

  bool _initialized = false;

  Future<void> init() async {
    if (_initialized) return;
    _initialized = true;
    final prefs = await SharedPreferences.getInstance();
    final rawTheme = prefs.getString(_themeKey);
    final theme = switch (rawTheme) {
      'dark' => ThemeMode.dark,
      'system' => ThemeMode.system,
      _ => ThemeMode.light,
    };
    state = AppearancePreferences(
      themeMode: theme,
      fontScale: (prefs.getDouble(_fontScaleKey) ?? 1.0).clamp(0.8, 1.4).toDouble(),
      accentColorIndex: (prefs.getInt(_accentKey) ?? 0).clamp(0, 3).toInt(),
      cornerStyleIndex: (prefs.getInt(_cornerKey) ?? 1).clamp(0, 2).toInt(),
      dynamicEffect: prefs.getBool(_dynamicKey) ?? true,
      reduceAnimation: prefs.getBool(_reduceAnimationKey) ?? false,
    );
  }

  Future<void> setThemeMode(ThemeMode value) async {
    state = state.copyWith(themeMode: value);
    final prefs = await SharedPreferences.getInstance();
    await prefs.setString(_themeKey, value.name);
  }

  Future<void> setFontScale(double value) async {
    final next = value.clamp(0.8, 1.4).toDouble();
    state = state.copyWith(fontScale: next);
    final prefs = await SharedPreferences.getInstance();
    await prefs.setDouble(_fontScaleKey, next);
  }

  Future<void> setAccentColorIndex(int value) async {
    final next = value.clamp(0, 3).toInt();
    state = state.copyWith(accentColorIndex: next);
    final prefs = await SharedPreferences.getInstance();
    await prefs.setInt(_accentKey, next);
  }

  Future<void> setCornerStyleIndex(int value) async {
    final next = value.clamp(0, 2).toInt();
    state = state.copyWith(cornerStyleIndex: next);
    final prefs = await SharedPreferences.getInstance();
    await prefs.setInt(_cornerKey, next);
  }

  Future<void> setDynamicEffect(bool value) async {
    state = state.copyWith(dynamicEffect: value);
    final prefs = await SharedPreferences.getInstance();
    await prefs.setBool(_dynamicKey, value);
  }

  Future<void> setReduceAnimation(bool value) async {
    state = state.copyWith(reduceAnimation: value);
    final prefs = await SharedPreferences.getInstance();
    await prefs.setBool(_reduceAnimationKey, value);
  }
}
