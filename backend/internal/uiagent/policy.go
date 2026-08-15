package uiagent

import (
	"errors"
	"runtime"
	"strings"
)

var (
	ErrSourceTargetImmutable    = errors.New("ui agent: source target is immutable in production")
	ErrSourceTargetNotEditable  = errors.New("ui agent: workspace is not an editable development workspace")
	ErrPathOutOfBounds          = errors.New("ui agent: path out of allowed boundaries")
	ErrCircularDependency       = errors.New("ui agent: operation dependency cycle detected")
)

type ModeResolver interface {
	Resolve(intent UIIntent) (UITargetType, error)
}

type defaultModeResolver struct {
	productionMode bool
}

func NewModeResolver(productionMode bool) ModeResolver {
	return &defaultModeResolver{productionMode: productionMode}
}

func (r *defaultModeResolver) Resolve(intent UIIntent) (UITargetType, error) {
	if intent.Target.ExtensionID != "" || intent.Target.ContributionID != "" {
		return UITargetSchema, nil
	}

	if intent.Target.Type != "" {
		if intent.Target.Type == UITargetSource && r.productionMode {
			return "", ErrSourceTargetImmutable
		}
		return intent.Target.Type, nil
	}

	if intent.Target.WorkspaceID != "" {
		if r.productionMode {
			return "", ErrSourceTargetImmutable
		}
		return UITargetSource, nil
	}

	return UITargetSchema, nil
}

type Policy struct {
	MaxFilesPerChange      int
	SourceEditAllowlist    []string
	SourceEditBlockedPaths []string
}

func DefaultPolicy() Policy {
	return Policy{
		MaxFilesPerChange:      10,
		SourceEditAllowlist:    nil,
		SourceEditBlockedPaths: defaultBlockedPaths(),
	}
}

func defaultBlockedPaths() []string {
	paths := []string{
		"node_modules",
		".git",
		"dist",
		"build",
		"release",
	}
	if runtime.GOOS == "windows" {
		paths = append(paths, ".exe", ".dll")
	}
	return paths
}

func (p Policy) ClassifyRisk(plan UIChangePlan) UIRisk {
	if plan.Mode == UITargetSchema {
		return UIRiskLow
	}

	hasDeletion := false
	changedCount := 0
	for _, op := range plan.Operations {
		switch op.Type {
		case "delete", "remove":
			hasDeletion = true
		}
		if op.Target != "" {
			changedCount++
		}
	}

	if hasDeletion {
		return UIRiskHigh
	}

	for _, op := range plan.Operations {
		for _, blocked := range p.SourceEditBlockedPaths {
			if strings.Contains(op.Target, blocked) {
				return UIRiskHigh
			}
		}
	}

	if changedCount > 1 {
		return UIRiskMedium
	}

	return UIRiskLow
}

func (p Policy) Validate(plan UIChangePlan) error {
	if len(plan.Operations) > p.MaxFilesPerChange {
		return ErrPathOutOfBounds
	}
	return nil
}
