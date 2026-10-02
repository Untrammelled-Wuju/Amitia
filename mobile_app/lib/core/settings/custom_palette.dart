import 'dart:convert';
import 'package:flutter/material.dart';

enum ThemeTextMode { auto, dark, light, custom }

@immutable
class CustomPalette {
  final bool enabled;
  final Color primary;
  final Color secondary;
  final Color text;
  final ThemeTextMode textMode;
  const CustomPalette({
    this.enabled = false,
    this.primary = const Color(0xFF6C8FEA),
    this.secondary = const Color(0xFF52B788),
    this.text = const Color(0xFF24221F),
    this.textMode = ThemeTextMode.auto,
  });
  CustomPalette copyWith({
    bool? enabled,
    Color? primary,
    Color? secondary,
    Color? text,
    ThemeTextMode? textMode,
  }) => CustomPalette(
    enabled: enabled ?? this.enabled,
    primary: primary ?? this.primary,
    secondary: secondary ?? this.secondary,
    text: text ?? this.text,
    textMode: textMode ?? this.textMode,
  );
  String encode() => jsonEncode({
    'enabled': enabled,
    'primary': primary.toARGB32(),
    'secondary': secondary.toARGB32(),
    'text': text.toARGB32(),
    'textMode': textMode.name,
  });
  static CustomPalette decode(String? value) {
    try {
      final raw = jsonDecode(value ?? '{}');
      if (raw is! Map<String, dynamic>) return const CustomPalette();
      Color color(String key, Color fallback) {
        final value = raw[key];
        return value is int && value >= 0 && value <= 0xFFFFFFFF
            ? Color(value | 0xFF000000)
            : fallback;
      }

      const defaults = CustomPalette();
      return CustomPalette(
        enabled: raw['enabled'] == true,
        primary: color('primary', defaults.primary),
        secondary: color('secondary', defaults.secondary),
        text: color('text', defaults.text),
        textMode: ThemeTextMode.values.firstWhere(
          (mode) => mode.name == raw['textMode'],
          orElse: () => ThemeTextMode.auto,
        ),
      );
    } catch (_) {
      return const CustomPalette();
    }
  }
}

double colorContrast(Color text, Color background) {
  final a = text.computeLuminance(), b = background.computeLuminance();
  return ((a > b ? a : b) + 0.05) / ((a > b ? b : a) + 0.05);
}

Color readableForeground(Color background) =>
    colorContrast(Colors.black, background) >=
        colorContrast(Colors.white, background)
    ? Colors.black
    : Colors.white;
Color paletteForeground(CustomPalette palette, Color background) =>
    switch (palette.textMode) {
      ThemeTextMode.auto => readableForeground(background),
      ThemeTextMode.dark => const Color(0xFF24221F),
      ThemeTextMode.light => const Color(0xFFF5F5F5),
      ThemeTextMode.custom => palette.text,
    };
