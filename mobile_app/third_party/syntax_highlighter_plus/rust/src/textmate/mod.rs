//! A TextMate grammar tokenization engine on top of Oniguruma.

mod grammar;
mod raw;
mod regex;
mod tokenizer;

use std::sync::{Mutex, OnceLock};

use anyhow::Result;

pub use tokenizer::ScopeSpan;

use grammar::Registry;

static REGISTRY: OnceLock<Mutex<Registry>> = OnceLock::new();

/// Language ids (grammar file stems) of every bundled grammar.
pub fn language_ids() -> Vec<String> {
    let mut ids = Registry::language_ids();
    ids.sort();
    ids
}

/// Tokenizes `source` with the grammar registered for `language`.
///
/// Returned offsets are absolute UTF-16 code-unit indices into `source`.
pub fn tokenize(language: &str, source: &str) -> Result<Vec<ScopeSpan>> {
    let registry = REGISTRY.get_or_init(|| Mutex::new(Registry::default()));
    let mut registry = registry.lock().unwrap();
    let (scope, root) = registry.grammar_for_language(language)?;
    Ok(tokenizer::tokenize_source(&registry, root, &scope, source))
}

#[cfg(test)]
mod tests {
    use super::*;

    fn scopes_at(tokens: &[ScopeSpan], source: &str, needle: &str) -> Vec<String> {
        let pos = source.find(needle).expect("needle not in source") as u32;
        // Sources in these tests are ASCII, so byte == UTF-16 offsets.
        tokens
            .iter()
            .find(|t| t.start <= pos && pos < t.end)
            .map(|t| t.scopes.clone())
            .unwrap_or_default()
    }

    fn has_scope_prefix(scopes: &[String], prefix: &str) -> bool {
        scopes.iter().any(|s| s.starts_with(prefix))
    }

    #[test]
    fn python_basics() {
        let src = "def foo():\n    return 'hi'\n";
        let tokens = tokenize("python", src).unwrap();
        assert!(has_scope_prefix(&scopes_at(&tokens, src, "def"), "storage.type.function"));
        assert!(has_scope_prefix(&scopes_at(&tokens, src, "foo"), "entity.name.function"));
        assert!(has_scope_prefix(&scopes_at(&tokens, src, "return"), "keyword.control"));
        assert!(has_scope_prefix(&scopes_at(&tokens, src, "hi"), "string"));
    }

    #[test]
    fn dart_basics() {
        let src = "class Foo {\n  final String name = \"x\";\n}\n";
        let tokens = tokenize("dart", src).unwrap();
        assert!(has_scope_prefix(&scopes_at(&tokens, src, "class"), "keyword"));
        assert!(has_scope_prefix(&scopes_at(&tokens, src, "\"x\""), "string"));
    }

    #[test]
    fn json_nested() {
        let src = "{\"a\": [1, true, null]}";
        let tokens = tokenize("json", src).unwrap();
        assert!(has_scope_prefix(&scopes_at(&tokens, src, "true"), "constant.language"));
        assert!(has_scope_prefix(&scopes_at(&tokens, src, "1"), "constant.numeric"));
    }

    #[test]
    fn rust_multiline_string() {
        // begin/end rule spanning lines
        let src = "let s = \"line one\nline two\";\nlet x = 1;\n";
        let tokens = tokenize("rust", src).unwrap();
        assert!(has_scope_prefix(&scopes_at(&tokens, src, "line one"), "string"));
        assert!(has_scope_prefix(&scopes_at(&tokens, src, "line two"), "string"));
        assert!(!has_scope_prefix(&scopes_at(&tokens, src, "x = 1"), "string"));
    }

    #[test]
    fn markdown_embedded_grammar() {
        // Exercises cross-grammar includes and begin/while rules.
        let src = "# Title\n\n```python\nx = 'y'\n```\n\n> quoted\n";
        let tokens = tokenize("markdown", src).unwrap();
        assert!(has_scope_prefix(&scopes_at(&tokens, src, "# Title"), "heading") ||
                has_scope_prefix(&scopes_at(&tokens, src, "Title"), "markup.heading") ||
                scopes_at(&tokens, src, "Title").iter().any(|s| s.contains("heading")));
        // The embedded python string should carry python string scopes.
        assert!(has_scope_prefix(&scopes_at(&tokens, src, "'y'"), "string"));
    }

    #[test]
    fn utf16_offsets() {
        // '🎉' is 2 UTF-16 units / 4 UTF-8 bytes; offsets must be UTF-16.
        let src = "x = '🎉'\ny = 1\n";
        let tokens = tokenize("python", src).unwrap();
        let y_pos = 10; // UTF-16 offset of 'y' (line 1 is 9 units + newline)
        let tok = tokens.iter().find(|t| t.start <= y_pos && y_pos < t.end);
        assert!(tok.is_some(), "no token covers 'y': {tokens:?}");
        // The emoji itself sits inside the string.
        let emoji = tokens.iter().find(|t| t.start <= 5 && 5 < t.end).unwrap();
        assert!(has_scope_prefix(&emoji.scopes, "string"));
    }

    #[test]
    fn all_grammars_load_and_tokenize() {
        // Smoke test: every bundled grammar must load and survive a snippet.
        let src = "hello world <tag> { \"key\": [1, 2] } // comment\n";
        for id in language_ids() {
            let tokens = tokenize(&id, src);
            assert!(tokens.is_ok(), "grammar {id} failed: {:?}", tokens.err());
        }
    }
}

#[cfg(test)]
mod parse_debug {
    #[test]
    fn all_grammars_parse() {
        for (id, _, compressed) in crate::textmate::grammar::RAW_GRAMMARS {
            let json = crate::textmate::grammar::decompress_grammar(compressed)
                .unwrap_or_else(|e| panic!("{id}: {e}"));
            if let Err(e) = serde_json::from_slice::<super::raw::RawGrammar>(&json) {
                panic!("{id}: {e}");
            }
        }
    }
}

#[cfg(test)]
mod perf {
    /// Rough throughput check; run with:
    /// `cargo test --release perf_smoke -- --ignored --nocapture`
    #[test]
    #[ignore]
    fn perf_smoke() {
        let unit = r#"
/// A widget that renders highlighted code.
class Highlighted extends StatelessWidget {
  final String source;
  const Highlighted({super.key, required this.source});

  @override
  Widget build(BuildContext context) {
    final spans = <TextSpan>[];
    for (var i = 0; i < source.length; i++) {
      spans.add(TextSpan(text: source[i], style: TextStyle(color: Colors.red)));
    }
    return Text.rich(TextSpan(children: spans)); // trailing comment
  }
}
"#;
        let source = unit.repeat(40); // ~640 lines
        let lines = source.lines().count();

        let t = std::time::Instant::now();
        let tokens = super::tokenize("dart", &source).unwrap();
        let cold = t.elapsed();

        let t = std::time::Instant::now();
        for _ in 0..10 {
            super::tokenize("dart", &source).unwrap();
        }
        let warm = t.elapsed() / 10;
        println!("{lines} lines, {} tokens; cold {cold:?}, warm {warm:?}", tokens.len());
    }
}

