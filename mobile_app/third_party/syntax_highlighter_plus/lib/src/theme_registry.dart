import 'dart:convert';

import 'package:flutter/services.dart';

import 'syntax_theme.dart';

/// Registry for supported TextMate color themes.
class ThemeRegistry {
  const ThemeRegistry._();

  static const Set<String> _themes = {
    'github-dark',
    'github-light',
  };

  /// Returns the sorted list of supported theme ids.
  static List<String> get supportedThemes => _themes.toList()..sort();

  static final Map<String, SyntaxTheme> _themeCache = {};

  /// Returns the [SyntaxTheme] for [themeName], loading and parsing it from the
  /// bundled asset the first time it is requested.
  ///
  /// Throws an [ArgumentError] if [themeName] does not match any bundled theme.
  static Future<SyntaxTheme> themeFor(String themeName) async {
    final normalized = themeName.toLowerCase().trim();

    if (!_themes.contains(normalized)) {
      throw ArgumentError(
        'No bundled theme for "$themeName". '
        'Supported themes: ${supportedThemes.join(', ')}.',
      );
    }

    if (_themeCache.containsKey(normalized)) {
      return _themeCache[normalized]!;
    }

    final jsonString = await rootBundle.loadString(
      'packages/syntax_highlighter_plus/assets/themes/$normalized.json',
    );
    final theme = SyntaxTheme.fromJson(
      normalized,
      jsonDecode(jsonString) as Map<String, Object?>,
    );

    _themeCache[normalized] = theme;
    return theme;
  }
}
