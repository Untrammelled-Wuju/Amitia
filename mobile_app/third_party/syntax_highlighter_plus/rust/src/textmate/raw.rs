//! Serde model for TextMate grammar JSON files (the `tmLanguage.json` format
//! used by VS Code / shiki).

use std::collections::HashMap;

use serde::de::{self, Deserializer};
use serde::Deserialize;

#[derive(Debug, Deserialize)]
#[serde(rename_all = "camelCase")]
pub struct RawGrammar {
    /// Kept for completeness; the registry resolves scope names from the
    /// build-time embedded table instead.
    #[allow(dead_code)]
    pub scope_name: Option<String>,
    #[serde(default)]
    pub patterns: Vec<RawRule>,
    pub repository: Option<HashMap<String, RawRule>>,
}

/// A rule inside a grammar. Capture entries reuse this type as well (they may
/// carry a `name` and/or nested `patterns`).
#[derive(Debug, Default, Deserialize)]
#[serde(rename_all = "camelCase")]
pub struct RawRule {
    pub include: Option<String>,
    pub name: Option<String>,
    pub content_name: Option<String>,
    #[serde(rename = "match")]
    pub match_: Option<String>,
    pub begin: Option<String>,
    pub end: Option<String>,
    #[serde(rename = "while")]
    pub while_: Option<String>,
    pub patterns: Option<Vec<RawRule>>,
    pub repository: Option<HashMap<String, RawRule>>,
    #[serde(default, deserialize_with = "de_captures")]
    pub captures: Option<HashMap<String, RawRule>>,
    #[serde(default, deserialize_with = "de_captures")]
    pub begin_captures: Option<HashMap<String, RawRule>>,
    #[serde(default, deserialize_with = "de_captures")]
    pub end_captures: Option<HashMap<String, RawRule>>,
    #[serde(default, deserialize_with = "de_captures")]
    pub while_captures: Option<HashMap<String, RawRule>>,
    /// Appears as `true`/`false` or `1`/`0` in the wild.
    #[serde(default, deserialize_with = "de_flag")]
    pub apply_end_pattern_last: bool,
    /// Some grammars disable rules with `"disabled": 1`.
    #[serde(default, deserialize_with = "de_flag")]
    pub disabled: bool,
}

/// Captures usually come as a map keyed by group index, but some grammars
/// (e.g. jinja) use the JSON array form, where element `i` is capture `i`.
fn de_captures<'de, D: Deserializer<'de>>(
    deserializer: D,
) -> Result<Option<HashMap<String, RawRule>>, D::Error> {
    struct Visitor;
    impl<'de> de::Visitor<'de> for Visitor {
        type Value = Option<HashMap<String, RawRule>>;
        fn expecting(&self, f: &mut std::fmt::Formatter) -> std::fmt::Result {
            f.write_str("a map or an array of capture rules")
        }
        fn visit_none<E: de::Error>(self) -> Result<Self::Value, E> {
            Ok(None)
        }
        fn visit_unit<E: de::Error>(self) -> Result<Self::Value, E> {
            Ok(None)
        }
        // Malformed entries (e.g. xml.json nests string-valued keys inside
        // `captures`) are skipped rather than failing the whole grammar,
        // matching vscode-textmate's tolerance.
        fn visit_map<A: de::MapAccess<'de>>(self, mut map: A) -> Result<Self::Value, A::Error> {
            let mut out = HashMap::new();
            while let Some((key, value)) = map.next_entry::<String, serde_json::Value>()? {
                if let Ok(rule) = serde_json::from_value::<RawRule>(value) {
                    out.insert(key, rule);
                }
            }
            Ok(Some(out))
        }
        fn visit_seq<A: de::SeqAccess<'de>>(self, mut seq: A) -> Result<Self::Value, A::Error> {
            let mut out = HashMap::new();
            let mut i = 0usize;
            while let Some(value) = seq.next_element::<serde_json::Value>()? {
                if let Ok(rule) = serde_json::from_value::<RawRule>(value) {
                    out.insert(i.to_string(), rule);
                }
                i += 1;
            }
            Ok(Some(out))
        }
    }
    deserializer.deserialize_any(Visitor)
}

fn de_flag<'de, D: Deserializer<'de>>(deserializer: D) -> Result<bool, D::Error> {
    struct Visitor;
    impl de::Visitor<'_> for Visitor {
        type Value = bool;
        fn expecting(&self, f: &mut std::fmt::Formatter) -> std::fmt::Result {
            f.write_str("a boolean or an integer")
        }
        fn visit_bool<E: de::Error>(self, v: bool) -> Result<bool, E> {
            Ok(v)
        }
        fn visit_i64<E: de::Error>(self, v: i64) -> Result<bool, E> {
            Ok(v != 0)
        }
        fn visit_u64<E: de::Error>(self, v: u64) -> Result<bool, E> {
            Ok(v != 0)
        }
    }
    deserializer.deserialize_any(Visitor)
}

impl RawRule {
    /// Captures for a `match` rule, or the begin-captures of a
    /// `begin`/`end` rule (TextMate: `captures` is shorthand for specifying
    /// both `beginCaptures` and `endCaptures` with the same values).
    pub fn effective_begin_captures(&self) -> Option<&HashMap<String, RawRule>> {
        self.begin_captures.as_ref().or(self.captures.as_ref())
    }

    pub fn effective_end_captures(&self) -> Option<&HashMap<String, RawRule>> {
        self.end_captures.as_ref().or(self.captures.as_ref())
    }

    pub fn effective_while_captures(&self) -> Option<&HashMap<String, RawRule>> {
        self.while_captures.as_ref().or(self.captures.as_ref())
    }
}
