import 'dart:ui' show Brightness, Color;

import 'package:flutter/painting.dart';

/// A parsed VS Code color theme (the `tokenColors` part plus editor colors).
///
/// Resolves a token's TextMate scope stack to a [TextStyle] using scope
/// selector matching: a selector like `meta.function string` matches a scope
/// stack where an outer scope starts with `meta.function` and a deeper scope
/// starts with `string`. Among matching rules the deepest / most specific
/// match wins. Resolution results are memoized per scope stack.
class SyntaxTheme {
  /// Theme id (e.g. `github-dark`).
  final String name;

  /// Whether this is a `dark` or `light` theme.
  final Brightness brightness;

  /// Default text color (`editor.foreground`).
  final Color? foreground;

  /// Editor background color (`editor.background`). Apply it to the widget
  /// that contains your `Text.rich` — spans themselves don't paint it.
  final Color? background;

  final List<_ThemeRule> _rules;

  final Map<String, TextStyle> _cache = {};

  SyntaxTheme._(
    this._rules, {
    required this.name,
    required this.brightness,
    required this.foreground,
    required this.background,
  });

  /// Parses a VS Code theme JSON document.
  factory SyntaxTheme.fromJson(String name, Map<String, Object?> json) {
    final type = json['type'] as String?;
    final colors = (json['colors'] as Map?)?.cast<String, Object?>() ?? const {};
    final isDark = type == 'dark';

    final foreground = _parseColor(colors['editor.foreground'] as String?);
    final background = _parseColor(colors['editor.background'] as String?);

    final rules = <_ThemeRule>[];
    final tokenColors = json['tokenColors'] as List? ?? const [];
    for (final entry in tokenColors) {
      if (entry is! Map) continue;
      final settings = (entry['settings'] as Map?)?.cast<String, Object?>();
      if (settings == null) continue;

      final scope = entry['scope'];
      final selectors = <_Selector>[];
      if (scope is String) {
        // A single string may hold comma-separated selectors.
        selectors.addAll(scope.split(',').map(_Selector.parse));
      } else if (scope is List) {
        selectors.addAll(scope.whereType<String>().map(_Selector.parse));
      } else if (scope == null) {
        // Global defaults entry: applies to everything.
        selectors.add(const _Selector([]));
      }

      final style = _parseSettings(settings);
      if (style == null) continue;
      for (final selector in selectors) {
        rules.add(_ThemeRule(selector, rules.length, style));
      }
    }

    return SyntaxTheme._(
      rules,
      name: name,
      brightness: isDark ? Brightness.dark : Brightness.light,
      foreground: foreground,
      background: background,
    );
  }

  /// Resolves the style for a token's scope stack (outermost first).
  ///
  /// Returns a [TextStyle] with only the properties the theme sets for these
  /// scopes; unset properties inherit from the surrounding text.
  TextStyle styleFor(List<String> scopes) {
    final key = scopes.join(' ');
    final cached = _cache[key];
    if (cached != null) return cached;

    _RuleMatch? bestColor;
    _RuleMatch? bestFont;
    for (final rule in _rules) {
      final match = rule.selector.match(scopes);
      if (match == null) continue;
      final candidate = _RuleMatch(rule, match);
      if (rule.style.foreground != null && (bestColor == null || candidate.beats(bestColor))) {
        bestColor = candidate;
      }
      if (rule.style.fontStyle != null && (bestFont == null || candidate.beats(bestFont))) {
        bestFont = candidate;
      }
    }

    var style = const TextStyle();
    if (bestColor != null) {
      style = style.copyWith(color: bestColor.rule.style.foreground);
    }
    if (bestFont != null) {
      style = bestFont.rule.style.fontStyle!.apply(style);
    }
    _cache[key] = style;
    return style;
  }
}

// ---------------------------------------------------------------------------
// Rules and selectors
// ---------------------------------------------------------------------------

class _ThemeRule {
  final _Selector selector;
  final int index;
  final _RuleStyle style;
  const _ThemeRule(this.selector, this.index, this.style);
}

class _RuleStyle {
  final Color? foreground;
  final _FontStyle? fontStyle;
  const _RuleStyle(this.foreground, this.fontStyle);
}

class _FontStyle {
  final bool italic;
  final bool bold;
  final bool underline;
  final bool strikethrough;
  const _FontStyle(this.italic, this.bold, this.underline, this.strikethrough);

  TextStyle apply(TextStyle style) {
    final decorations = <TextDecoration>[
      if (underline) TextDecoration.underline,
      if (strikethrough) TextDecoration.lineThrough,
    ];
    return style.copyWith(
      fontStyle: italic ? FontStyle.italic : null,
      fontWeight: bold ? FontWeight.bold : null,
      decoration: decorations.isEmpty ? null : TextDecoration.combine(decorations),
    );
  }
}

/// A space-separated scope selector path, e.g. `meta.function string.quoted`
/// parses to `[["meta","function"], ["string","quoted"]]`.
class _Selector {
  final List<String> segments;
  const _Selector(this.segments);

  factory _Selector.parse(String text) =>
      _Selector(text.trim().split(RegExp(r'\s+')).where((s) => s.isNotEmpty).toList());

  /// Matches against a scope stack (outermost first). Returns match strength,
  /// or null when the selector doesn't apply.
  _MatchStrength? match(List<String> scopes) {
    if (segments.isEmpty) {
      // Empty selector (global defaults): weakest possible match.
      return const _MatchStrength(-1, 0);
    }
    // The last segment must match some scope; earlier segments must match
    // strictly shallower scopes in order. Prefer the deepest anchor.
    final last = segments.last;
    for (var depth = scopes.length - 1; depth >= 0; depth--) {
      if (!_segmentMatches(last, scopes[depth])) continue;
      // Check parent segments against scopes[0..depth).
      var scopeIdx = 0;
      var matched = true;
      for (var i = 0; i < segments.length - 1; i++) {
        var found = false;
        while (scopeIdx < depth) {
          if (_segmentMatches(segments[i], scopes[scopeIdx])) {
            found = true;
            scopeIdx++;
            break;
          }
          scopeIdx++;
        }
        if (!found) {
          matched = false;
          break;
        }
      }
      if (matched) return _MatchStrength(depth, last.length);
    }
    return null;
  }

  /// `string` matches `string` and `string.quoted` but not `stringx`.
  static bool _segmentMatches(String segment, String scope) =>
      scope == segment ||
      (scope.length > segment.length && scope.startsWith(segment) && scope.codeUnitAt(segment.length) == 0x2E /* . */ );
}

class _MatchStrength {
  /// Index in the scope stack matched by the selector's last segment.
  final int depth;

  /// Length of the last segment (longer prefix = more specific).
  final int prefixLength;
  const _MatchStrength(this.depth, this.prefixLength);
}

class _RuleMatch {
  final _ThemeRule rule;
  final _MatchStrength strength;
  const _RuleMatch(this.rule, this.strength);

  /// TextMate precedence: deeper scope match, then longer prefix, then more
  /// selector segments, then later rule in the theme file.
  bool beats(_RuleMatch other) {
    if (strength.depth != other.strength.depth) {
      return strength.depth > other.strength.depth;
    }
    if (strength.prefixLength != other.strength.prefixLength) {
      return strength.prefixLength > other.strength.prefixLength;
    }
    final segments = rule.selector.segments.length;
    final otherSegments = other.rule.selector.segments.length;
    if (segments != otherSegments) return segments > otherSegments;
    return rule.index >= other.rule.index;
  }
}

// ---------------------------------------------------------------------------
// Parsing helpers
// ---------------------------------------------------------------------------

_RuleStyle? _parseSettings(Map<String, Object?> settings) {
  final foreground = _parseColor(settings['foreground'] as String?);
  final fontStyleText = settings['fontStyle'] as String?;

  _FontStyle? fontStyle;
  if (fontStyleText != null) {
    final parts = fontStyleText.split(RegExp(r'\s+'));
    fontStyle = _FontStyle(
      parts.contains('italic'),
      parts.contains('bold'),
      parts.contains('underline'),
      parts.contains('strikethrough'),
    );
  }
  if (foreground == null && fontStyle == null) return null;
  return _RuleStyle(foreground, fontStyle);
}

Color? _parseColor(String? hex) {
  if (hex == null || !hex.startsWith('#')) return null;
  var value = hex.substring(1);
  // Expand #RGB / #RGBA shorthand.
  if (value.length == 3 || value.length == 4) {
    value = value.split('').map((c) => '$c$c').join();
  }
  if (value.length == 6) value = 'FF$value'; // opaque
  if (value.length == 8) {
    // Input is RRGGBBAA; Color wants AARRGGBB.
    if (hex.length == 9 || hex.length == 5) {
      value = value.substring(6) + value.substring(0, 6);
    }
  } else {
    return null;
  }
  final parsed = int.tryParse(value, radix: 16);
  return parsed == null ? null : Color(parsed);
}
