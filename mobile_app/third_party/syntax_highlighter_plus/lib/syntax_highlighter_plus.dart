import 'package:flutter/widgets.dart';

import 'src/grammar_registry.dart';
import 'src/rust/api/highlighter.dart' as rust;
import 'src/rust/frb_generated.dart';
import 'src/span_builder.dart';
import 'src/syntax_theme.dart';
import 'src/theme_registry.dart';

export 'src/rust/api/highlighter.dart' show Token;
export 'src/syntax_theme.dart' show SyntaxTheme;

/// High-level syntax-highlighting helper.
///
/// Tokenization runs in Rust (Oniguruma + TextMate grammars) on a worker
/// thread; theming and span building happen in Dart. Grammars, compiled
/// rules, themes, and scope-style lookups are all cached, so repeated calls
/// are cheap.
///
/// ```dart
/// final highlighter = SyntaxHighlighterPlus(theme: 'github-dark');
/// final span = await highlighter.highlight('dart', source);
/// // ...
/// Text.rich(span);
/// ```
class SyntaxHighlighterPlus {
  /// Every language tag accepted by [highlight] — canonical grammar ids plus
  /// fence-tag aliases like `py` and `js`.
  static List<String> get supportedLanguages => GrammarRegistry.supportedLanguages;

  /// The list of bundled theme ids.
  static List<String> get supportedThemes => ThemeRegistry.supportedThemes;

  /// The theme to apply (e.g. `'github-dark'`). Must be one of
  /// [supportedThemes].
  final String theme;

  /// Creates a highlighter that renders with [theme].
  const SyntaxHighlighterPlus({required this.theme});

  /// Highlights [source] and returns a [TextSpan] for use with `Text.rich`.
  ///
  /// * [language] — language id or alias (e.g. `'dart'`, `'py'`, `'c++'`).
  ///   Throws an [ArgumentError] for unknown tags; check
  ///   [supportedLanguages] or catch the error to fall back to plain text.
  /// * [style] — optional base style (font family, size, …) merged into the
  ///   root span; the theme controls colors on top of it.
  ///
  /// The returned span paints no background. Wrap the `Text.rich` in a
  /// container colored with the theme's background if desired (see
  /// [SyntaxTheme.background], available via [themeData]).
  Future<TextSpan> highlight(
    String language,
    String source, {
    TextStyle? style,
  }) async {
    final canonicalId = GrammarRegistry.resolve(language);
    final syntaxTheme = await ThemeRegistry.themeFor(theme);

    await _ensureInit();
    final tokens = await rust.tokenize(language: canonicalId, source: source);

    return buildTextSpan(
      source: source,
      tokens: tokens,
      theme: syntaxTheme,
      baseStyle: style,
    );
  }

  /// The parsed theme, exposing [SyntaxTheme.background],
  /// [SyntaxTheme.foreground], and [SyntaxTheme.brightness] for styling the
  /// surrounding widget.
  Future<SyntaxTheme> get themeData => ThemeRegistry.themeFor(theme);

  static Future<void>? _init;

  static Future<void> _ensureInit() => _init ??= _doInit();

  static Future<void> _doInit() async {
    try {
      await RustLib.init();
    } on StateError {
      // The host app already initialized flutter_rust_bridge itself.
    }
  }
}
