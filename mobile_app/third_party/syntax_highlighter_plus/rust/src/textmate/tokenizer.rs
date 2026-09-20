//! Line-based TextMate tokenizer, closely following the algorithm of
//! microsoft/vscode-textmate (`grammar.ts`).
//!
//! Text is processed one line at a time with a rule stack carried between
//! lines. Each line gets a `\n` appended before matching (grammars rely on
//! matching the newline) and emitted tokens are clamped back to the real
//! line length. Token offsets are converted to absolute UTF-16 code-unit
//! offsets so they can index Dart strings directly.

use std::rc::Rc;

use super::grammar::{Registry, Rule, RuleId, ScopeName};
use super::regex::{MatchResult, RegexSrc};

/// Safety valve against pathological grammars.
const MAX_LINE_ITERATIONS: usize = 20_000;
/// Maximum depth of capture re-tokenization recursion.
const MAX_CAPTURE_DEPTH: usize = 8;

#[derive(Debug, Clone)]
pub struct ScopeSpan {
    /// Absolute UTF-16 code-unit offset into the source (inclusive).
    pub start: u32,
    /// Absolute UTF-16 code-unit offset into the source (exclusive).
    pub end: u32,
    /// Scope stack, outermost first (e.g. `["source.dart", "string.interpolated.dart"]`).
    pub scopes: Vec<String>,
}

// ---------------------------------------------------------------------------
// Scope list: immutable linked list so stack frames share tails cheaply.
// ---------------------------------------------------------------------------

#[derive(Clone)]
struct ScopeList(Option<Rc<ScopeNode>>);

struct ScopeNode {
    parent: ScopeList,
    name: String,
}

impl ScopeList {
    fn root(scope: &str) -> ScopeList {
        ScopeList(None).push_scopes(scope)
    }

    /// Pushes one or more scopes (`name` may be space-separated, e.g.
    /// `"punctuation.definition.string markup.raw"`).
    fn push_scopes(&self, name: &str) -> ScopeList {
        let mut cur = self.clone();
        for part in name.split_whitespace() {
            cur = ScopeList(Some(Rc::new(ScopeNode {
                parent: cur,
                name: part.to_string(),
            })));
        }
        cur
    }

    fn to_vec(&self) -> Vec<String> {
        let mut out = Vec::new();
        let mut node = &self.0;
        while let Some(n) = node {
            out.push(n.name.clone());
            node = &n.parent.0;
        }
        out.reverse();
        out
    }
}

// ---------------------------------------------------------------------------
// Stack frames
// ---------------------------------------------------------------------------

#[derive(Clone)]
struct Frame {
    rule: RuleId,
    /// Byte position in the current line where this frame was pushed
    /// (-1 when entered on a previous line).
    enter_pos: isize,
    /// Anchor position saved at push time, restored on pop.
    anchor_pos: isize,
    /// Whether the begin match consumed the line's trailing newline; if so,
    /// `\G` anchors at position 0 of the next line.
    begin_captured_eol: bool,
    name_scopes: ScopeList,
    content_scopes: ScopeList,
    /// End (or while) regex with begin-match backreferences substituted;
    /// `None` when the rule's pattern has no backreferences.
    resolved: Option<Rc<RegexSrc>>,
}

// ---------------------------------------------------------------------------
// Per-line token accumulator
// ---------------------------------------------------------------------------

struct LineTokens {
    last: usize,
    entries: Vec<(usize, usize, ScopeList)>,
}

impl LineTokens {
    fn new() -> Self {
        LineTokens { last: 0, entries: Vec::new() }
    }

    /// Emits a token from the previous boundary up to `end` (byte offsets).
    fn produce(&mut self, end: usize, scopes: &ScopeList) {
        if end > self.last {
            self.entries.push((self.last, end, scopes.clone()));
            self.last = end;
        }
    }
}

// ---------------------------------------------------------------------------
// Public entry point
// ---------------------------------------------------------------------------

pub fn tokenize_source(reg: &Registry, root: RuleId, base_scope: &str, source: &str) -> Vec<ScopeSpan> {
    let root_scopes = ScopeList::root(base_scope);
    let mut stack = vec![Frame {
        rule: root,
        enter_pos: -1,
        anchor_pos: -1,
        begin_captured_eol: false,
        name_scopes: root_scopes.clone(),
        content_scopes: root_scopes,
        resolved: None,
    }];

    let mut tokens = Vec::new();
    let mut base_u16: usize = 0;
    for (i, line) in source.split('\n').enumerate() {
        // Positions refer to the current line; frames entered on previous
        // lines get -1 (mirrors vscode-textmate's StateStack.reset()).
        for frame in stack.iter_mut() {
            frame.enter_pos = -1;
            frame.anchor_pos = -1;
        }

        let mut buf = String::with_capacity(line.len() + 1);
        buf.push_str(line);
        buf.push('\n');

        let mut out = LineTokens::new();
        let (pos, anchor) = check_while_conditions(reg, &buf, i == 0, &mut stack, &mut out);
        tokenize_string(reg, &buf, i == 0, pos, anchor, &mut stack, &mut out, 0);

        let conv = Utf16Conv::new(line);
        for (start, end, scopes) in out.entries {
            // Clamp away the artificial trailing newline.
            let end = end.min(line.len());
            if end <= start {
                continue;
            }
            tokens.push(ScopeSpan {
                start: (base_u16 + conv.to_utf16(start)) as u32,
                end: (base_u16 + conv.to_utf16(end)) as u32,
                scopes: scopes.to_vec(),
            });
        }
        base_u16 += utf16_len(line) + 1; // +1 for the '\n' separator
    }
    tokens
}

// ---------------------------------------------------------------------------
// While conditions (checked at the start of every line)
// ---------------------------------------------------------------------------

fn check_while_conditions(
    reg: &Registry,
    line: &str,
    is_first_line: bool,
    stack: &mut Vec<Frame>,
    out: &mut LineTokens,
) -> (usize, isize) {
    let mut pos = 0usize;
    let mut anchor: isize = if stack.last().unwrap().begin_captured_eol { 0 } else { -1 };

    let while_frames: Vec<usize> = stack
        .iter()
        .enumerate()
        .filter(|(_, f)| matches!(reg.rule(f.rule), Rule::BeginWhile(_)))
        .map(|(i, _)| i)
        .collect();

    for idx in while_frames {
        if idx >= stack.len() {
            break; // already popped by an earlier failed while
        }
        let frame = stack[idx].clone();
        let Rule::BeginWhile(rule) = reg.rule(frame.rule) else { continue };
        let regex: &RegexSrc = match (&frame.resolved, &rule.while_) {
            (Some(r), _) => r,
            (None, Some(r)) => r,
            (None, None) => continue,
        };
        match regex.search(line, pos, is_first_line, pos as isize == anchor) {
            Some(m) => {
                handle_captures(
                    reg, line, is_first_line, stack, &frame.content_scopes,
                    &rule.while_captures, &m, out, 0,
                );
                out.produce(m.end(), &frame.content_scopes);
                anchor = m.end() as isize;
                if m.end() > pos {
                    pos = m.end();
                }
            }
            None => {
                stack.truncate(idx);
                break;
            }
        }
    }
    (pos, anchor)
}

// ---------------------------------------------------------------------------
// Main matching loop for (part of) one line
// ---------------------------------------------------------------------------

#[allow(clippy::too_many_arguments)]
fn tokenize_string(
    reg: &Registry,
    line: &str,
    is_first_line: bool,
    start_pos: usize,
    start_anchor: isize,
    stack: &mut Vec<Frame>,
    out: &mut LineTokens,
    depth: usize,
) {
    let mut pos = start_pos;
    let mut anchor = start_anchor;
    let mut iterations = 0usize;

    loop {
        iterations += 1;
        if iterations > MAX_LINE_ITERATIONS {
            out.produce(line.len(), &stack.last().unwrap().content_scopes);
            return;
        }

        let top = stack.last().unwrap().clone();
        let allow_g = pos as isize == anchor;

        // Find the best (leftmost; ties by candidate order) match among the
        // active rule's patterns, plus its end pattern for begin/end rules.
        let mut best: Option<(MatchResult, Option<RuleId>)> = None;
        let mut best_start = usize::MAX;
        {
            // (candidate id, regex); `None` id marks the end pattern.
            let end_candidate: Option<(Option<RuleId>, &RegexSrc)> = match reg.rule(top.rule) {
                Rule::BeginEnd(r) => {
                    let regex: &RegexSrc = match (&top.resolved, &r.end) {
                        (Some(re), _) => re,
                        (None, Some(re)) => re,
                        (None, None) => unreachable!("begin/end rule without end regex"),
                    };
                    Some((None, regex))
                }
                _ => None,
            };
            let apply_end_last = matches!(
                reg.rule(top.rule),
                Rule::BeginEnd(r) if r.apply_end_pattern_last
            );

            let candidates = reg.candidates(top.rule);
            let pattern_candidates = candidates.iter().map(|&id| {
                let regex: &RegexSrc = match reg.rule(id) {
                    Rule::Match(r) => &r.regex,
                    Rule::BeginEnd(r) => &r.begin,
                    Rule::BeginWhile(r) => &r.begin,
                    _ => unreachable!("containers are flattened"),
                };
                (Some(id), regex)
            });

            let ordered: Vec<(Option<RuleId>, &RegexSrc)> = if apply_end_last {
                pattern_candidates.chain(end_candidate).collect()
            } else {
                end_candidate.into_iter().chain(pattern_candidates).collect()
            };

            for (id, regex) in ordered {
                if let Some(m) = regex.search(line, pos, is_first_line, allow_g) {
                    let s = m.start();
                    if s < best_start {
                        best_start = s;
                        let at_pos = s == pos;
                        best = Some((m, id));
                        if at_pos {
                            break; // can't do better than matching right here
                        }
                    }
                }
            }
        }

        let Some((m, matched)) = best else {
            // Nothing matched: the rest of the line belongs to the current scope.
            out.produce(line.len(), &top.content_scopes);
            return;
        };
        let match_end = m.end();
        let has_advanced = match_end > pos;

        out.produce(m.start(), &top.content_scopes);

        match matched {
            // The end pattern of the current begin/end rule.
            None => {
                let Rule::BeginEnd(rule) = reg.rule(top.rule) else { unreachable!() };
                // The contentName scope does not apply to the end match itself.
                handle_captures(
                    reg, line, is_first_line, stack, &top.name_scopes,
                    &rule.end_captures, &m, out, depth,
                );
                out.produce(match_end, &top.name_scopes);
                let popped = stack.pop().unwrap();
                anchor = popped.anchor_pos;
                if !has_advanced && popped.enter_pos == pos as isize {
                    // Grammar pushed & popped a rule without advancing:
                    // restore and bail out to avoid an endless loop.
                    stack.push(popped);
                    out.produce(line.len(), &stack.last().unwrap().content_scopes);
                    return;
                }
            }
            Some(id) => match reg.rule(id) {
                Rule::Match(rule) => {
                    let name_scopes = push_name(&top.content_scopes, rule.name.as_ref(), line, &m);
                    handle_captures(
                        reg, line, is_first_line, stack, &name_scopes,
                        &rule.captures, &m, out, depth,
                    );
                    out.produce(match_end, &name_scopes);
                    if !has_advanced {
                        // A match rule that consumed nothing would loop forever.
                        out.produce(line.len(), &top.content_scopes);
                        return;
                    }
                }
                Rule::BeginEnd(rule) => {
                    let name_scopes = push_name(&top.content_scopes, rule.name.as_ref(), line, &m);
                    let resolved = rule.end_has_back_references.then(|| {
                        Rc::new(RegexSrc::new(&RegexSrc::resolve_back_references(
                            &rule.end_source, line, &m,
                        )))
                    });
                    stack.push(Frame {
                        rule: id,
                        enter_pos: pos as isize,
                        anchor_pos: anchor,
                        begin_captured_eol: match_end == line.len(),
                        name_scopes: name_scopes.clone(),
                        content_scopes: name_scopes.clone(),
                        resolved,
                    });
                    handle_captures(
                        reg, line, is_first_line, stack, &name_scopes,
                        &rule.begin_captures, &m, out, depth,
                    );
                    out.produce(match_end, &name_scopes);
                    anchor = match_end as isize;
                    let content_scopes = push_name(&name_scopes, rule.content_name.as_ref(), line, &m);
                    stack.last_mut().unwrap().content_scopes = content_scopes;
                    if !has_advanced && stack.len() >= 2 && stack[stack.len() - 2].rule == id {
                        // Rule pushed itself without advancing.
                        stack.pop();
                        out.produce(line.len(), &stack.last().unwrap().content_scopes);
                        return;
                    }
                }
                Rule::BeginWhile(rule) => {
                    let name_scopes = push_name(&top.content_scopes, rule.name.as_ref(), line, &m);
                    let resolved = rule.while_has_back_references.then(|| {
                        Rc::new(RegexSrc::new(&RegexSrc::resolve_back_references(
                            &rule.while_source, line, &m,
                        )))
                    });
                    stack.push(Frame {
                        rule: id,
                        enter_pos: pos as isize,
                        anchor_pos: anchor,
                        begin_captured_eol: match_end == line.len(),
                        name_scopes: name_scopes.clone(),
                        content_scopes: name_scopes.clone(),
                        resolved,
                    });
                    handle_captures(
                        reg, line, is_first_line, stack, &name_scopes,
                        &rule.begin_captures, &m, out, depth,
                    );
                    out.produce(match_end, &name_scopes);
                    anchor = match_end as isize;
                    let content_scopes = push_name(&name_scopes, rule.content_name.as_ref(), line, &m);
                    stack.last_mut().unwrap().content_scopes = content_scopes;
                    if !has_advanced && stack.len() >= 2 && stack[stack.len() - 2].rule == id {
                        stack.pop();
                        out.produce(line.len(), &stack.last().unwrap().content_scopes);
                        return;
                    }
                }
                Rule::Include { .. } | Rule::None => unreachable!("containers are flattened"),
            },
        }

        if match_end > pos {
            pos = match_end;
        }
    }
}

// ---------------------------------------------------------------------------
// Capture handling
// ---------------------------------------------------------------------------

#[allow(clippy::too_many_arguments)]
fn handle_captures(
    reg: &Registry,
    line: &str,
    is_first_line: bool,
    stack: &Vec<Frame>,
    scopes: &ScopeList,
    captures: &[Option<super::grammar::CaptureDef>],
    m: &MatchResult,
    out: &mut LineTokens,
    depth: usize,
) {
    if captures.is_empty() {
        return;
    }
    let len = captures.len().min(m.groups.len());
    let Some((_, max_end)) = m.group(0) else { return };

    // Stack of (end position, scopes) for nested named captures.
    let mut local: Vec<(usize, ScopeList)> = Vec::new();

    for i in 0..len {
        let Some(cap) = &captures[i] else { continue };
        let Some((start, end)) = m.group(i) else { continue };
        if start == end {
            continue;
        }
        if start > max_end {
            break;
        }

        // Close named captures that ended before this one starts.
        while let Some((top_end, _)) = local.last() {
            if *top_end <= start {
                let (e, s) = local.pop().unwrap();
                out.produce(e, &s);
            } else {
                break;
            }
        }
        let base = local
            .last()
            .map(|(_, s)| s.clone())
            .unwrap_or_else(|| scopes.clone());
        out.produce(start, &base);

        let cap_scopes = push_name(&base, cap.name.as_ref(), line, m);

        if let Some(rule) = cap.rule {
            // Capture has nested patterns: re-tokenize the captured text.
            if depth < MAX_CAPTURE_DEPTH {
                let mut stack2 = stack.clone();
                stack2.push(Frame {
                    rule,
                    enter_pos: start as isize,
                    anchor_pos: -1,
                    begin_captured_eol: false,
                    name_scopes: cap_scopes.clone(),
                    content_scopes: cap_scopes.clone(),
                    resolved: None,
                });
                tokenize_string(
                    reg,
                    &line[..end],
                    is_first_line && start == 0,
                    start,
                    -1,
                    &mut stack2,
                    out,
                    depth + 1,
                );
                continue;
            }
            // Depth limit reached: fall through and treat as a plain capture.
        }
        if cap.name.is_some() {
            local.push((end, cap_scopes));
        }
    }

    while let Some((e, s)) = local.pop() {
        out.produce(e, &s);
    }
}

// ---------------------------------------------------------------------------
// Scope name resolution ($1 / ${1:/downcase} substitution)
// ---------------------------------------------------------------------------

fn push_name(
    base: &ScopeList,
    name: Option<&ScopeName>,
    line: &str,
    m: &MatchResult,
) -> ScopeList {
    match name {
        None => base.clone(),
        Some(n) if !n.needs_substitution => base.push_scopes(&n.raw),
        Some(n) => base.push_scopes(&substitute_captures(&n.raw, line, m)),
    }
}

fn substitute_captures(name: &str, line: &str, m: &MatchResult) -> String {
    let bytes = name.as_bytes();
    let mut out = String::with_capacity(name.len());
    let mut i = 0;
    while i < bytes.len() {
        if bytes[i] == b'$' && i + 1 < bytes.len() {
            // $1, $22, ...
            if bytes[i + 1].is_ascii_digit() {
                let mut j = i + 1;
                while j < bytes.len() && bytes[j].is_ascii_digit() {
                    j += 1;
                }
                let group: usize = name[i + 1..j].parse().unwrap_or(0);
                if let Some((s, e)) = m.group(group) {
                    out.push_str(line[s..e].trim());
                }
                i = j;
                continue;
            }
            // ${1:/downcase} or ${1:/upcase}
            if bytes[i + 1] == b'{' {
                if let Some(close) = name[i..].find('}') {
                    let inner = &name[i + 2..i + close];
                    let (num, cmd) = match inner.split_once(":/") {
                        Some((n, c)) => (n, Some(c)),
                        None => (inner, None),
                    };
                    if let Ok(group) = num.parse::<usize>() {
                        if let Some((s, e)) = m.group(group) {
                            let text = line[s..e].trim();
                            match cmd {
                                Some("downcase") => out.push_str(&text.to_lowercase()),
                                Some("upcase") => out.push_str(&text.to_uppercase()),
                                _ => out.push_str(text),
                            }
                        }
                        i += close + 1;
                        continue;
                    }
                }
            }
        }
        let ch = name[i..].chars().next().unwrap();
        out.push(ch);
        i += ch.len_utf8();
    }
    out
}

// ---------------------------------------------------------------------------
// UTF-8 byte offset -> UTF-16 code unit offset conversion
// ---------------------------------------------------------------------------

fn utf16_len(s: &str) -> usize {
    if s.is_ascii() {
        s.len()
    } else {
        s.chars().map(char::len_utf16).sum()
    }
}

struct Utf16Conv {
    /// `None` for pure-ASCII lines (offsets are identical).
    map: Option<Vec<(usize, usize)>>,
}

impl Utf16Conv {
    fn new(line: &str) -> Self {
        if line.is_ascii() {
            return Utf16Conv { map: None };
        }
        let mut map = Vec::with_capacity(line.len() + 1);
        let mut u16_idx = 0;
        for (byte_idx, ch) in line.char_indices() {
            map.push((byte_idx, u16_idx));
            u16_idx += ch.len_utf16();
        }
        map.push((line.len(), u16_idx));
        Utf16Conv { map: Some(map) }
    }

    fn to_utf16(&self, byte: usize) -> usize {
        match &self.map {
            None => byte,
            Some(map) => {
                // Token boundaries always fall on char boundaries.
                match map.binary_search_by_key(&byte, |&(b, _)| b) {
                    Ok(i) => map[i].1,
                    Err(i) => map[i.saturating_sub(1)].1,
                }
            }
        }
    }
}
