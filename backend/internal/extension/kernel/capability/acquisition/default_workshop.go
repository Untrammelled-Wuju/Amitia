package acquisition

import (
	"context"
	"fmt"
	"strings"
)

type defaultWorkshop struct{}

func NewDefaultWorkshop() WorkshopGeneratePort {
	return &defaultWorkshop{}
}

func (w *defaultWorkshop) GenerateInstruction(ctx context.Context, requirement string) (WorkshopInstructionDraft, error) {
	if requirement == "" {
		return WorkshopInstructionDraft{}, fmt.Errorf("workshop: empty requirement")
	}

	name := sanitizeSkillName(requirement)
	displayName := requirement
	if len(displayName) > 60 {
		displayName = displayName[:57] + "..."
	}

	body := fmt.Sprintf("# %s\n\n## Purpose\n%s\n\n## Instructions\nThis skill was generated to fulfill the requirement: %s\n\n## Usage\n- Review the skill content\n- Customize as needed\n- Test thoroughly before deployment\n",
		displayName, requirement, requirement)

	return WorkshopInstructionDraft{
		Name:        name,
		Description: requirement,
		DisplayName: displayName,
		Body:        body,
		References:  map[string]string{},
		Assets:      map[string]string{},
	}, nil
}

func sanitizeSkillName(requirement string) string {
	name := strings.ToLower(requirement)
	name = strings.ReplaceAll(name, " ", "_")
	name = strings.ReplaceAll(name, "-", "_")
	if len(name) > 40 {
		name = name[:40]
	}
	return "generated." + name
}
