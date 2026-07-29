package scope

import "fmt"

type ScopeOperator string

const (
	OpAND ScopeOperator = "AND"
	OpOR  ScopeOperator = "OR"
)

type ScopePlaceholder string

const (
	PHCurrentCharacter    ScopePlaceholder = "CURRENT_CHARACTER"
	PHCurrentConversation ScopePlaceholder = "CURRENT_CONVERSATION"
	PHOwnerExtension      ScopePlaceholder = "OWNER_EXTENSION"
	PHOwnerModule         ScopePlaceholder = "OWNER_MODULE"
)

type ScopeExpression struct {
	Operator     ScopeOperator      `json:"operator"`
	Scopes       []ScopeRef         `json:"scopes,omitempty"`
	Placeholders []ScopePlaceholder `json:"placeholders,omitempty"`
	Children     []ScopeExpression  `json:"children,omitempty"`
}

func SingleScope(scope ScopeRef) ScopeExpression {
	return ScopeExpression{
		Operator: OpAND,
		Scopes:   []ScopeRef{scope},
	}
}

func AllOf(scopes ...ScopeRef) ScopeExpression {
	return ScopeExpression{
		Operator: OpAND,
		Scopes:   scopes,
	}
}

func AnyOf(scopes ...ScopeRef) ScopeExpression {
	return ScopeExpression{
		Operator: OpOR,
		Scopes:   scopes,
	}
}

func (e ScopeExpression) Validate() error {
	if e.Operator != OpAND && e.Operator != OpOR {
		return fmt.Errorf("invalid operator: %s", e.Operator)
	}
	for _, s := range e.Scopes {
		if err := s.Validate(); err != nil {
			return err
		}
	}
	for _, c := range e.Children {
		if err := c.Validate(); err != nil {
			return err
		}
	}
	return nil
}

func (e ScopeExpression) HasPlaceholder() bool {
	if len(e.Placeholders) > 0 {
		return true
	}
	for _, c := range e.Children {
		if c.HasPlaceholder() {
			return true
		}
	}
	return false
}

func WithPlaceholder(expr ScopeExpression, placeholders ...ScopePlaceholder) ScopeExpression {
	expr.Placeholders = placeholders
	return expr
}
