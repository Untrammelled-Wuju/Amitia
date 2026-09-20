# syntax_highlighter_plus

A robust syntax highlighting Flutter package powered by TextMate grammars and themes.

> [!TIP]
> This is the exact highlighter powering [**Reins: Local & Private AI
> Assistant**](https://getreins.app) which runs LLMs on your iPhone, iPad, and
> Mac. Grab Reins on the [App Store](https://apps.apple.com/app/id6739738501) to see it in production.

## Features

- Full TextMate grammar support (begin/end/while rules, captures, embedded
  languages), tokenized by a Rust engine built on Oniguruma — the same regex
  engine VS Code's highlighter uses.
- 70+ bundled grammars and fence-tag aliases (`py`, `js`, `c++`, …).
- VS Code color themes (`github-dark`, `github-light` bundled).
- Tokenization runs off the UI thread via `flutter_rust_bridge`; theming and
  `TextSpan` building happen in Dart, so switching themes is cheap.
- Simple and direct: `highlight()` just returns a `TextSpan`. Just drop it into
  `Text.rich`, no custom widgets or controllers to learn.

## Usage

`highlight()` just returns a `TextSpan`. There's no special widget to wrap
your code in, just pass the span straight to `Text.rich` (or
`SelectableText.rich`) like any other rich text.

Here is a basic example of how to use the `SyntaxHighlighterPlus` class to highlight Dart code:

```dart
import 'package:flutter/material.dart';
import 'package:syntax_highlighter_plus/syntax_highlighter_plus.dart';

class HighlightedText extends StatelessWidget {
  const HighlightedText({super.key});

  @override
  Widget build(BuildContext context) {
    final syntaxHighlighter = SyntaxHighlighterPlus(theme: 'github-dark');
    final highlightFuture = syntaxHighlighter.highlight('dart', 'print("Hello, World!");');

    return FutureBuilder<TextSpan>(
      future: highlightFuture,
      builder: (context, snapshot) {
        if (snapshot.hasError) {
          return Center(child: Text('Error: ${snapshot.error}'));
        }

        final span = snapshot.data ?? const TextSpan(text: 'print("Hello, World!");');

        return SingleChildScrollView(
          padding: const EdgeInsets.all(16),
          child: SelectableText.rich(
            span,
            style: const TextStyle(fontFamily: 'monospace', height: 1.6),
          ),
        );
      },
    );
  }
}
```

`highlight` accepts any supported language id or alias
(`SyntaxHighlighterPlus.supportedLanguages`) and throws an `ArgumentError`
for unknown tags — catch it to fall back to plain text for unrecognized
markdown fence tags.

To style the surrounding widget (e.g. the code block's background), use the
parsed theme:

```dart
final theme = await syntaxHighlighter.themeData;
// theme.background, theme.foreground, theme.brightness
```

## Architecture

- `assets/grammars/*.json.zst` (zstd-compressed TextMate grammars) are
  embedded into the native library at build time and interpreted by a Rust
  tokenizer (`rust/src/textmate/`) using the
  [onig](https://crates.io/crates/onig) crate. Grammars are decompressed on
  first use; parsed grammars and compiled regexes are cached lazily per
  process.
- Tokens come back as `(start, end, scopes)` with UTF-16 offsets, ready to
  index Dart strings.
- `assets/themes/*.json` (VS Code themes) are parsed in Dart; scope-selector
  matching maps each token's scope stack to a `TextStyle`, and adjacent runs
  merge into a compact `TextSpan` tree.
