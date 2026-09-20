import 'package:flutter/painting.dart';

import 'rust/api/highlighter.dart';
import 'syntax_theme.dart';

/// Converts [tokens] (from the Rust tokenizer) into a [TextSpan] tree styled
/// with [theme].
///
/// The root span carries the theme's default foreground color merged over
/// [baseStyle]; children only override what differs, and adjacent runs with
/// identical styles are merged to keep the span tree small.
TextSpan buildTextSpan({
  required String source,
  required List<Token> tokens,
  required SyntaxTheme theme,
  TextStyle? baseStyle,
}) {
  final rootStyle = (baseStyle ?? const TextStyle()).copyWith(color: theme.foreground);

  final children = <TextSpan>[];
  var runStart = 0;
  var runEnd = 0;
  TextStyle? runStyle;

  void flush() {
    if (runEnd > runStart) {
      children.add(
        TextSpan(
          text: source.substring(runStart, runEnd),
          style: runStyle,
        ),
      );
    }
    runStart = runEnd;
  }

  void append(int start, int end, TextStyle? style) {
    if (end <= start) return;
    if (start > runEnd || style != runStyle) {
      flush();
      runStart = start;
      runStyle = style;
    }
    runEnd = end;
  }

  var cursor = 0;
  for (final token in tokens) {
    final start = token.start.clamp(0, source.length);
    final end = token.end.clamp(0, source.length);
    if (end <= start) continue;
    // Gap between tokens (e.g. newlines): default style.
    if (start > cursor) append(cursor, start, null);
    final style = theme.styleFor(token.scopes);
    // An empty style means "inherit everything" — same as a gap.
    append(start, end, style == const TextStyle() ? null : style);
    cursor = end;
  }
  if (cursor < source.length) append(cursor, source.length, null);
  flush();

  return TextSpan(style: rootStyle, children: children);
}
