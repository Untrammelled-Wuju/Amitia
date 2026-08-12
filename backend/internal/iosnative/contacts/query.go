package contacts

type ContactPredicate struct {
	Type string `json:"type"`

	Field string `json:"field,omitempty"`

	Op    string `json:"op,omitempty"`
	Value any    `json:"value,omitempty"`
}

type SortOrder struct {
	Field     string `json:"field"`
	Ascending bool   `json:"ascending"`
}

const (
	SortFieldDisplayName  = "displayName"
	SortFieldFamilyName   = "familyName"
	SortFieldGivenName    = "givenName"
	SortFieldOrganization = "organization"
	SortFieldDateAdded    = "dateAdded"
	SortFieldDateModified = "dateModified"
)

const (
	PredicateTypeEqual     = "equal"
	PredicateTypeNotEqual  = "notEqual"
	PredicateTypeContains  = "contains"
	PredicateTypePrefix    = "prefix"
	PredicateTypeAnd       = "and"
	PredicateTypeOr        = "or"
	PredicateTypeNot       = "not"
)

var DefaultSortOrder = []SortOrder{
	{Field: SortFieldDisplayName, Ascending: true},
}

func ValidatePredicate(p ContactPredicate) error {
	switch p.Type {
	case PredicateTypeEqual, PredicateTypeNotEqual, PredicateTypeContains, PredicateTypePrefix:
		if p.Field == "" {
			return NewQueryError("predicate field is required for type: " + p.Type)
		}
	case PredicateTypeAnd, PredicateTypeOr:
		children, ok := p.Value.([]any)
		if !ok || len(children) == 0 {
			return NewQueryError("and/or predicate requires children array")
		}
		for _, child := range children {
			cp, ok := child.(ContactPredicate)
			if !ok {
				if m, isMap := child.(map[string]any); isMap {
					cp = mapToPredicate(m)
				} else {
					return NewQueryError("invalid child predicate")
				}
			}
			if err := ValidatePredicate(cp); err != nil {
				return err
			}
		}
	case PredicateTypeNot:
		child, ok := p.Value.(ContactPredicate)
		if !ok {
			if m, isMap := p.Value.(map[string]any); isMap {
				child = mapToPredicate(m)
			} else {
				return NewQueryError("not predicate requires a child predicate")
			}
		}
		if err := ValidatePredicate(child); err != nil {
			return err
		}
	default:
		return NewQueryError("unsupported predicate type: " + p.Type)
	}
	return nil
}

func mapToPredicate(m map[string]any) ContactPredicate {
	p := ContactPredicate{}
	if t, ok := m["type"].(string); ok {
		p.Type = t
	}
	if f, ok := m["field"].(string); ok {
		p.Field = f
	}
	if o, ok := m["op"].(string); ok {
		p.Op = o
	}
	if v, ok := m["value"]; ok {
		p.Value = v
	}
	return p
}

type QueryError struct {
	Message string
}

func NewQueryError(message string) *QueryError {
	return &QueryError{Message: message}
}

func (e *QueryError) Error() string {
	return e.Message
}
