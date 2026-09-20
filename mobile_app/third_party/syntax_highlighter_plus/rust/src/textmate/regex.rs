//! Lazy-compiled Oniguruma regex with `\A` / `\G` anchor variants.
//!
//! TextMate grammars use `\A` (start of document) and `\G` (position where the
//! enclosing begin match ended). Because we feed Oniguruma one line at a time
//! and search from arbitrary positions, those anchors are only *sometimes*
//! valid. Like vscode-textmate we pre-substitute the anchors with a
//! never-matching character (U+FFFF) in the variants where they must not
//! match, and cache up to four compiled variants per pattern.

use std::sync::Mutex;

use onig::{Regex, RegexOptions, Region, SearchOptions, Syntax};

/// Group ranges (byte offsets into the searched line) of a successful match.
#[derive(Debug, Clone)]
pub struct MatchResult {
    /// `groups[0]` is the overall match; entries are `None` for groups that
    /// did not participate.
    pub groups: Vec<Option<(usize, usize)>>,
}

impl MatchResult {
    pub fn start(&self) -> usize {
        self.groups[0].unwrap().0
    }
    pub fn end(&self) -> usize {
        self.groups[0].unwrap().1
    }
    pub fn group(&self, i: usize) -> Option<(usize, usize)> {
        self.groups.get(i).copied().flatten()
    }
}

#[derive(Debug)]
pub struct RegexSrc {
    source: String,
    has_a: bool,
    has_g: bool,
    /// Compiled variants indexed by `allow_a as usize | (allow_g as usize) << 1`.
    /// `Some(Err(()))` marks a pattern that failed to compile (never matches).
    cache: Mutex<[Option<Result<Regex, ()>>; 4]>,
}

impl RegexSrc {
    pub fn new(source: &str) -> Self {
        let (has_a, has_g) = scan_anchors(source);
        RegexSrc {
            source: source.to_string(),
            has_a,
            has_g,
            cache: Mutex::new([None, None, None, None]),
        }
    }

    /// Whether the pattern references begin-match backreferences (`\1`…),
    /// which means it must be re-resolved per begin match.
    pub fn has_back_references(source: &str) -> bool {
        let bytes = source.as_bytes();
        let mut i = 0;
        while i + 1 < bytes.len() {
            if bytes[i] == b'\\' {
                if bytes[i + 1].is_ascii_digit() {
                    return true;
                }
                i += 2;
            } else {
                i += 1;
            }
        }
        false
    }

    /// Replaces `\1`…`\9` with the (regex-escaped) text captured by the begin
    /// match, producing a concrete end/while pattern for a stack frame.
    pub fn resolve_back_references(source: &str, line: &str, m: &MatchResult) -> String {
        let bytes = source.as_bytes();
        let mut out = String::with_capacity(source.len());
        let mut i = 0;
        while i < bytes.len() {
            if bytes[i] == b'\\' && i + 1 < bytes.len() {
                let next = bytes[i + 1];
                if next.is_ascii_digit() {
                    let group = (next - b'0') as usize;
                    if let Some((s, e)) = m.group(group) {
                        escape_regex_into(&line[s..e], &mut out);
                    }
                    i += 2;
                    continue;
                }
                out.push('\\');
                out.push(next as char);
                i += 2;
                continue;
            }
            // Source is valid UTF-8; copy the full char.
            let ch = source[i..].chars().next().unwrap();
            out.push(ch);
            i += ch.len_utf8();
        }
        out
    }

    /// Searches `line` starting at byte offset `from`.
    ///
    /// * `allow_a` — whether `\A` may match (first line of the document).
    /// * `allow_g` — whether `\G` may match (i.e. `from` equals the anchor
    ///   position). When allowed, `\G` anchors to `from` because Oniguruma
    ///   binds `\G` to the search start position.
    pub fn search(
        &self,
        line: &str,
        from: usize,
        allow_a: bool,
        allow_g: bool,
    ) -> Option<MatchResult> {
        // Collapse to a single variant when the pattern has no such anchor.
        let allow_a = allow_a || !self.has_a;
        let allow_g = allow_g || !self.has_g;
        let idx = (allow_a as usize) | ((allow_g as usize) << 1);

        let mut cache = self.cache.lock().unwrap();
        if cache[idx].is_none() {
            let pattern = self.variant_source(allow_a, allow_g);
            cache[idx] = Some(compile(&pattern));
        }
        let regex = match cache[idx].as_ref().unwrap() {
            Ok(r) => r,
            Err(()) => return None,
        };

        let mut region = Region::new();
        regex.search_with_options(
            line,
            from,
            line.len(),
            SearchOptions::SEARCH_OPTION_NONE,
            Some(&mut region),
        )?;
        let groups = (0..region.len()).map(|i| region.pos(i)).collect();
        Some(MatchResult { groups })
    }

    fn variant_source(&self, allow_a: bool, allow_g: bool) -> String {
        if allow_a && allow_g {
            return self.source.clone();
        }
        let bytes = self.source.as_bytes();
        let mut out = String::with_capacity(self.source.len());
        let mut i = 0;
        while i < bytes.len() {
            if bytes[i] == b'\\' && i + 1 < bytes.len() {
                let next = bytes[i + 1];
                if (next == b'A' && !allow_a) || (next == b'G' && !allow_g) {
                    out.push('\u{FFFF}'); // never present in valid text
                } else {
                    out.push('\\');
                    out.push(next as char);
                }
                i += 2;
                continue;
            }
            let ch = self.source[i..].chars().next().unwrap();
            out.push(ch);
            i += ch.len_utf8();
        }
        out
    }
}

fn compile(pattern: &str) -> Result<Regex, ()> {
    Regex::with_options(
        pattern,
        RegexOptions::REGEX_OPTION_CAPTURE_GROUP,
        Syntax::default(),
    )
    .map_err(|_| ())
}

fn scan_anchors(source: &str) -> (bool, bool) {
    let bytes = source.as_bytes();
    let (mut has_a, mut has_g) = (false, false);
    let mut i = 0;
    while i + 1 < bytes.len() {
        if bytes[i] == b'\\' {
            match bytes[i + 1] {
                b'A' => has_a = true,
                b'G' => has_g = true,
                _ => {}
            }
            i += 2;
        } else {
            i += 1;
        }
    }
    (has_a, has_g)
}

fn escape_regex_into(text: &str, out: &mut String) {
    for ch in text.chars() {
        if "\\^$*+?()[]{}|.-".contains(ch) {
            out.push('\\');
        }
        out.push(ch);
    }
}
