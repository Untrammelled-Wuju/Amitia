package protocol

import (
	"fmt"
	"strconv"
	"strings"
	"unicode"
)

// versionMatches evaluates artifact compatibility constraints against an
// explicitly supplied integration version. Compatibility constraints are
// intentionally game-agnostic: plain strings are exact matches, while numeric
// dotted versions additionally support common semver-style ranges.
//
// Supported forms include:
//   - exact: 1.21.4, v2.0, build-2026-a
//   - wildcard: *, 1.21.x, 1.21.*
//   - comparators: >=1.20.1 <1.22, >2 <=3.4
//   - tilde/caret: ~1.20.2, ^1.20.1
//   - hyphen ranges: 1.20.1 - 1.21.4
//   - alternatives: >=1.20 <1.21 || >=1.21.4 <1.22
//
// If the artifact declares compatibility constraints, callers must provide a
// version. Treating a missing version as compatible would bypass the manifest's
// compatibility boundary and can deploy the wrong companion payload.
func CompatibilityVersionMatches(constraints []string, version string) bool {
	version = strings.TrimSpace(version)
	if len(constraints) == 0 {
		return true
	}
	if version == "" {
		return false
	}
	for _, constraint := range constraints {
		if matchVersionConstraint(strings.TrimSpace(constraint), version) {
			return true
		}
	}
	return false
}

func matchVersionConstraint(constraint, version string) bool {
	constraint = strings.TrimSpace(constraint)
	version = strings.TrimSpace(version)
	if constraint == "" || version == "" {
		return false
	}
	if constraint == "*" || strings.EqualFold(constraint, "x") {
		return true
	}

	// Preserve opaque game/build version identifiers. Exact matching never
	// requires a numeric parser and remains case-insensitive for compatibility
	// with the previous host behavior.
	if strings.EqualFold(constraint, version) {
		return true
	}
	// Alternatives may also be opaque build/game identifiers. Check exact
	// alternative matches before requiring the candidate to be numeric, so a
	// constraint such as "fabric-loader-0.16 || neoforge-21" remains useful to
	// non-semver ecosystems without weakening range parsing.
	for _, alternative := range strings.Split(constraint, "||") {
		if strings.EqualFold(strings.TrimSpace(alternative), version) {
			return true
		}
	}

	candidate, ok := parseComparableVersion(version)
	if !ok {
		return false
	}

	for _, alternative := range strings.Split(constraint, "||") {
		alternative = strings.TrimSpace(alternative)
		if alternative == "" {
			continue
		}
		matched, valid := matchVersionAlternative(candidate, alternative)
		if valid && matched {
			return true
		}
	}
	return false
}

func matchVersionAlternative(candidate comparableVersion, expr string) (bool, bool) {
	if low, high, ok := parseHyphenRange(expr); ok {
		return compareComparableVersion(candidate, low) >= 0 && compareComparableVersion(candidate, high) <= 0, true
	}

	// Commas and ASCII whitespace both mean logical AND. Operators are kept
	// attached to their version token (for example >=1.20.1).
	expr = strings.ReplaceAll(expr, ",", " ")
	tokens := strings.Fields(expr)
	if len(tokens) == 0 {
		return false, false
	}
	for _, token := range tokens {
		matched, valid := matchVersionToken(candidate, token)
		if !valid {
			return false, false
		}
		if !matched {
			return false, true
		}
	}
	return true, true
}

func parseHyphenRange(expr string) (comparableVersion, comparableVersion, bool) {
	parts := strings.Split(expr, " - ")
	if len(parts) != 2 {
		return comparableVersion{}, comparableVersion{}, false
	}
	low, okLow := parseComparableVersion(strings.TrimSpace(parts[0]))
	high, okHigh := parseComparableVersion(strings.TrimSpace(parts[1]))
	return low, high, okLow && okHigh
}

func matchVersionToken(candidate comparableVersion, token string) (bool, bool) {
	token = strings.TrimSpace(token)
	if token == "" {
		return false, false
	}
	if token == "*" || strings.EqualFold(token, "x") {
		return true, true
	}

	if strings.HasPrefix(token, "^") {
		base, ok := parseComparableVersion(strings.TrimSpace(token[1:]))
		if !ok {
			return false, false
		}
		upper := caretUpperBound(base)
		return compareComparableVersion(candidate, base) >= 0 && compareComparableVersion(candidate, upper) < 0, true
	}
	if strings.HasPrefix(token, "~") {
		baseText := strings.TrimSpace(token[1:])
		base, ok := parseComparableVersion(baseText)
		if !ok {
			return false, false
		}
		upper := tildeUpperBound(base, countNumericComponents(baseText))
		return compareComparableVersion(candidate, base) >= 0 && compareComparableVersion(candidate, upper) < 0, true
	}

	for _, op := range []string{">=", "<=", "!=", "==", ">", "<", "="} {
		if strings.HasPrefix(token, op) {
			rhs, ok := parseComparableVersion(strings.TrimSpace(token[len(op):]))
			if !ok {
				return false, false
			}
			cmp := compareComparableVersion(candidate, rhs)
			switch op {
			case ">=":
				return cmp >= 0, true
			case "<=":
				return cmp <= 0, true
			case "!=":
				return cmp != 0, true
			case "==", "=":
				return cmp == 0, true
			case ">":
				return cmp > 0, true
			case "<":
				return cmp < 0, true
			}
		}
	}

	if strings.ContainsAny(strings.ToLower(token), "x*") {
		return matchWildcardVersion(candidate, token)
	}

	rhs, ok := parseComparableVersion(token)
	if !ok {
		return false, false
	}
	return compareComparableVersion(candidate, rhs) == 0, true
}

type comparableVersion struct {
	major int64
	minor int64
	patch int64
	pre   []versionIdentifier
}

type versionIdentifier struct {
	numeric bool
	number  int64
	text    string
}

func parseComparableVersion(raw string) (comparableVersion, bool) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return comparableVersion{}, false
	}
	if raw[0] == 'v' || raw[0] == 'V' {
		raw = raw[1:]
	}
	if plus := strings.IndexByte(raw, '+'); plus >= 0 {
		raw = raw[:plus]
	}
	base := raw
	pre := ""
	if dash := strings.IndexByte(raw, '-'); dash >= 0 {
		base = raw[:dash]
		pre = raw[dash+1:]
		if pre == "" {
			return comparableVersion{}, false
		}
	}
	parts := strings.Split(base, ".")
	if len(parts) == 0 || len(parts) > 3 {
		return comparableVersion{}, false
	}
	nums := []int64{0, 0, 0}
	for i, part := range parts {
		if part == "" {
			return comparableVersion{}, false
		}
		for _, r := range part {
			if !unicode.IsDigit(r) {
				return comparableVersion{}, false
			}
		}
		n, err := strconv.ParseInt(part, 10, 64)
		if err != nil || n < 0 {
			return comparableVersion{}, false
		}
		nums[i] = n
	}
	out := comparableVersion{major: nums[0], minor: nums[1], patch: nums[2]}
	if pre != "" {
		for _, id := range strings.Split(pre, ".") {
			if id == "" {
				return comparableVersion{}, false
			}
			vi := versionIdentifier{text: strings.ToLower(id)}
			if allDigits(id) {
				n, err := strconv.ParseInt(id, 10, 64)
				if err != nil {
					return comparableVersion{}, false
				}
				vi.numeric = true
				vi.number = n
			}
			out.pre = append(out.pre, vi)
		}
	}
	return out, true
}

func compareComparableVersion(a, b comparableVersion) int {
	if a.major != b.major {
		return compareInt64(a.major, b.major)
	}
	if a.minor != b.minor {
		return compareInt64(a.minor, b.minor)
	}
	if a.patch != b.patch {
		return compareInt64(a.patch, b.patch)
	}
	if len(a.pre) == 0 && len(b.pre) == 0 {
		return 0
	}
	if len(a.pre) == 0 {
		return 1
	}
	if len(b.pre) == 0 {
		return -1
	}
	limit := len(a.pre)
	if len(b.pre) < limit {
		limit = len(b.pre)
	}
	for i := 0; i < limit; i++ {
		left, right := a.pre[i], b.pre[i]
		if left.numeric && right.numeric {
			if left.number != right.number {
				return compareInt64(left.number, right.number)
			}
			continue
		}
		if left.numeric != right.numeric {
			if left.numeric {
				return -1
			}
			return 1
		}
		if left.text < right.text {
			return -1
		}
		if left.text > right.text {
			return 1
		}
	}
	if len(a.pre) < len(b.pre) {
		return -1
	}
	if len(a.pre) > len(b.pre) {
		return 1
	}
	return 0
}

func matchWildcardVersion(candidate comparableVersion, raw string) (bool, bool) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return false, false
	}
	if raw[0] == 'v' || raw[0] == 'V' {
		raw = raw[1:]
	}
	parts := strings.Split(raw, ".")
	if len(parts) == 0 || len(parts) > 3 {
		return false, false
	}
	candidateParts := []int64{candidate.major, candidate.minor, candidate.patch}
	wildcardSeen := false
	for i, part := range parts {
		part = strings.TrimSpace(part)
		if part == "*" || strings.EqualFold(part, "x") {
			wildcardSeen = true
			continue
		}
		if wildcardSeen {
			return false, false // e.g. 1.x.3 is ambiguous and rejected.
		}
		n, err := strconv.ParseInt(part, 10, 64)
		if err != nil || n < 0 || candidateParts[i] != n {
			return false, err == nil && n >= 0
		}
	}
	return wildcardSeen, wildcardSeen
}

func caretUpperBound(base comparableVersion) comparableVersion {
	upper := base
	upper.pre = nil
	switch {
	case base.major > 0:
		upper.major++
		upper.minor = 0
		upper.patch = 0
	case base.minor > 0:
		upper.minor++
		upper.patch = 0
	default:
		upper.patch++
	}
	return upper
}

func tildeUpperBound(base comparableVersion, components int) comparableVersion {
	upper := base
	upper.pre = nil
	if components <= 1 {
		upper.major++
		upper.minor = 0
		upper.patch = 0
		return upper
	}
	upper.minor++
	upper.patch = 0
	return upper
}

func countNumericComponents(raw string) int {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return 0
	}
	if raw[0] == 'v' || raw[0] == 'V' {
		raw = raw[1:]
	}
	if idx := strings.IndexAny(raw, "-+"); idx >= 0 {
		raw = raw[:idx]
	}
	return len(strings.Split(raw, "."))
}

func compareInt64(a, b int64) int {
	if a < b {
		return -1
	}
	if a > b {
		return 1
	}
	return 0
}

func allDigits(s string) bool {
	if s == "" {
		return false
	}
	for _, r := range s {
		if r < '0' || r > '9' {
			return false
		}
	}
	return true
}

func ValidateCompatibilityConstraint(constraint string) error {
	constraint = strings.TrimSpace(constraint)
	if constraint == "" {
		return fmt.Errorf("compatibility constraint must not be empty")
	}
	if constraint == "*" || strings.EqualFold(constraint, "x") {
		return nil
	}
	// A single token with no range/wildcard syntax is an opaque exact version.
	// Everything else is validated alternative-by-alternative so wildcard OR
	// expressions such as "1.20.x || 1.21.x" are accepted while malformed
	// wildcards such as "1.x.3" still fail closed.
	if !strings.ContainsAny(constraint, "<>^~*=|, ") && !containsWildcardSegment(constraint) {
		return nil
	}
	for _, alternative := range strings.Split(constraint, "||") {
		alternative = strings.TrimSpace(alternative)
		if alternative == "" {
			return fmt.Errorf("invalid empty compatibility alternative in %q", constraint)
		}
		if _, _, ok := parseHyphenRange(alternative); ok {
			continue
		}
		if strings.Contains(alternative, " - ") {
			return fmt.Errorf("invalid compatibility hyphen range %q", alternative)
		}
		tokens := strings.Fields(strings.ReplaceAll(alternative, ",", " "))
		if len(tokens) == 0 {
			return fmt.Errorf("invalid compatibility constraint %q", constraint)
		}
		if len(tokens) == 1 && !strings.ContainsAny(tokens[0], "<>^~*=,") && !containsWildcardSegment(tokens[0]) {
			// Opaque exact alternative (for example a loader/build identifier).
			continue
		}
		for _, token := range tokens {
			if !validateConstraintToken(token) {
				return fmt.Errorf("invalid compatibility token %q", token)
			}
		}
	}
	return nil
}

func validateConstraintToken(token string) bool {
	if token == "*" || strings.EqualFold(token, "x") {
		return true
	}
	for _, prefix := range []string{">=", "<=", "!=", "==", ">", "<", "=", "^", "~"} {
		if strings.HasPrefix(token, prefix) {
			_, ok := parseComparableVersion(strings.TrimSpace(token[len(prefix):]))
			return ok
		}
	}
	if strings.ContainsAny(strings.ToLower(token), "x*") {
		return validWildcardConstraint(token)
	}
	_, ok := parseComparableVersion(token)
	return ok
}

func containsWildcardSegment(raw string) bool {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return false
	}
	if raw[0] == 'v' || raw[0] == 'V' {
		raw = raw[1:]
	}
	for _, part := range strings.Split(raw, ".") {
		part = strings.TrimSpace(part)
		if part == "*" || strings.EqualFold(part, "x") {
			return true
		}
	}
	return false
}

func validWildcardConstraint(raw string) bool {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return false
	}
	if raw[0] == 'v' || raw[0] == 'V' {
		raw = raw[1:]
	}
	parts := strings.Split(raw, ".")
	if len(parts) == 0 || len(parts) > 3 {
		return false
	}
	wildcardSeen := false
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part == "*" || strings.EqualFold(part, "x") {
			wildcardSeen = true
			continue
		}
		if wildcardSeen || part == "" {
			return false
		}
		for _, r := range part {
			if !unicode.IsDigit(r) {
				return false
			}
		}
		if _, err := strconv.ParseInt(part, 10, 64); err != nil {
			return false
		}
	}
	return wildcardSeen
}
