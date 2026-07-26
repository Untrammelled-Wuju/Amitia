package dependency

import (
	"fmt"
	"strings"

	"github.com/u-ai/backend/internal/extension/kernel/domain"
)

type RangeOperator string

const (
	OpExact      RangeOperator = "="
	OpGreaterEq  RangeOperator = ">="
	OpLessEq     RangeOperator = "<="
	OpGreater    RangeOperator = ">"
	OpLess       RangeOperator = "<"
	OpCaret      RangeOperator = "^"
	OpTilde      RangeOperator = "~"
	OpWildcard   RangeOperator = "x"
)

type VersionConstraint struct {
	Operator RangeOperator
	Version  domain.SemanticVersion
	Raw      string
}

type VersionRange struct {
	Constraints []VersionConstraint
	Raw         string
}

func ParseRange(raw string) (*VersionRange, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" || raw == "*" {
		return &VersionRange{Raw: raw}, nil
	}
	parts := strings.Split(raw, ",")
	range_ := &VersionRange{Raw: raw}
	for _, part := range parts {
		part = strings.TrimSpace(part)
		c, err := parseSingleConstraint(part)
		if err != nil {
			return nil, err
		}
		range_.Constraints = append(range_.Constraints, c)
	}
	return range_, nil
}

func parseSingleConstraint(s string) (VersionConstraint, error) {
	s = strings.TrimSpace(s)
	if s == "" {
		return VersionConstraint{}, fmt.Errorf("dependency: empty constraint")
	}
	op := OpExact
	rest := s
	switch {
	case strings.HasPrefix(s, ">="):
		op = OpGreaterEq
		rest = strings.TrimPrefix(s, ">=")
	case strings.HasPrefix(s, "<="):
		op = OpLessEq
		rest = strings.TrimPrefix(s, "<=")
	case strings.HasPrefix(s, ">"):
		op = OpGreater
		rest = strings.TrimPrefix(s, ">")
	case strings.HasPrefix(s, "<"):
		op = OpLess
		rest = strings.TrimPrefix(s, "<")
	case strings.HasPrefix(s, "^"):
		op = OpCaret
		rest = strings.TrimPrefix(s, "^")
	case strings.HasPrefix(s, "~"):
		op = OpTilde
		rest = strings.TrimPrefix(s, "~")
	case strings.HasPrefix(s, "="):
		op = OpExact
		rest = strings.TrimPrefix(s, "=")
	}
	rest = strings.TrimSpace(rest)
	if strings.HasSuffix(rest, ".x") || rest == "x" {
		op = OpWildcard
		rest = strings.TrimSuffix(rest, ".x")
		if rest == "x" {
			rest = ""
		}
	}
	if rest == "" {
		return VersionConstraint{Operator: op, Raw: s}, nil
	}
	v, err := domain.ParseVersion(rest)
	if err != nil {
		return VersionConstraint{}, fmt.Errorf("dependency: invalid version %q: %w", rest, err)
	}
	return VersionConstraint{Operator: op, Version: v, Raw: s}, nil
}

func (r *VersionRange) Satisfies(v domain.SemanticVersion) bool {
	if r == nil || len(r.Constraints) == 0 {
		return true
	}
	for _, c := range r.Constraints {
		if !c.Satisfies(v) {
			return false
		}
	}
	return true
}

func (c VersionConstraint) Satisfies(v domain.SemanticVersion) bool {
	switch c.Operator {
	case OpExact, OpWildcard:
		if c.Operator == OpWildcard {
			if c.Version.Major == 0 {
				return true
			}
			if c.Version.Minor == 0 {
				return v.Major == c.Version.Major
			}
			if c.Version.Patch == 0 {
				return v.Major == c.Version.Major && v.Minor == c.Version.Minor
			}
		}
		return v.Compare(c.Version) == 0
	case OpGreaterEq:
		return v.Compare(c.Version) >= 0
	case OpLessEq:
		return v.Compare(c.Version) <= 0
	case OpGreater:
		return v.Compare(c.Version) > 0
	case OpLess:
		return v.Compare(c.Version) < 0
	case OpCaret:
		if v.Compare(c.Version) < 0 {
			return false
		}
		if c.Version.Major > 0 {
			return v.Major == c.Version.Major
		}
		if c.Version.Minor > 0 {
			return v.Major == 0 && v.Minor == c.Version.Minor
		}
		return v.Major == 0 && v.Minor == 0 && v.Patch == c.Version.Patch
	case OpTilde:
		if v.Compare(c.Version) < 0 {
			return false
		}
		return v.Major == c.Version.Major && v.Minor == c.Version.Minor
	}
	return false
}

func Intersect(r1, r2 *VersionRange) *VersionRange {
	if r1 == nil {
		return r2
	}
	if r2 == nil {
		return r1
	}
	combined := &VersionRange{
		Constraints: append(append([]VersionConstraint{}, r1.Constraints...), r2.Constraints...),
	}
	combined.Raw = r1.Raw + "," + r2.Raw
	return combined
}
