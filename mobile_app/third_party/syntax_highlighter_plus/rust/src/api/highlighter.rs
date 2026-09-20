use flutter_rust_bridge::frb;

use crate::textmate;

/// A run of source text with the TextMate scopes that apply to it.
///
/// Offsets are UTF-16 code units into the source string passed to
/// [`tokenize`], so they can be used directly as Dart string indices.
pub struct Token {
    pub start: u32,
    pub end: u32,
    /// Scope stack, outermost first
    /// (e.g. `["source.dart", "string.interpolated.dart"]`).
    pub scopes: Vec<String>,
}

/// Tokenizes `source` using the bundled TextMate grammar for `language`.
///
/// `language` must be a canonical grammar id (a file stem from
/// `assets/grammars`, e.g. `dart`, `cpp`); alias resolution (`py` -> `python`)
/// happens on the Dart side. Runs on a worker thread; grammars are compiled
/// lazily and cached for the process lifetime.
pub fn tokenize(language: String, source: String) -> anyhow::Result<Vec<Token>> {
    let tokens = textmate::tokenize(&language, &source)?;
    Ok(tokens
        .into_iter()
        .map(|t| Token { start: t.start, end: t.end, scopes: t.scopes })
        .collect())
}

/// Canonical ids of all bundled grammars.
#[frb(sync)]
pub fn supported_languages() -> Vec<String> {
    textmate::language_ids()
}
