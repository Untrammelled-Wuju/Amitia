package behaviorcatalog

import (
	"encoding/json"
	"errors"
	"fmt"
	"regexp"
	"sort"
	"strings"
)

const MetadataKey = "behaviors"

var behaviorIDPattern = regexp.MustCompile(`^[a-z0-9][a-z0-9._:/-]{0,255}$`)

var (
	ErrMissing = errors.New("behavior catalog is missing")
	ErrInvalid = errors.New("behavior catalog is invalid")
)

type Behavior struct {
	ID                   string          `json:"id"`
	Name                 string          `json:"name"`
	Description          string          `json:"description,omitempty"`
	ServiceID            string          `json:"serviceId,omitempty"`
	Method               string          `json:"method,omitempty"`
	ToolID               string          `json:"toolId,omitempty"`
	InputSchema          json.RawMessage `json:"inputSchema"`
	Risk                 string          `json:"risk,omitempty"`
	RequiresConfirmation bool            `json:"requiresConfirmation,omitempty"`
	Available            *bool           `json:"available,omitempty"`
}

type Catalog struct {
	Version   int        `json:"version"`
	Revision  string     `json:"revision"`
	Behaviors []Behavior `json:"behaviors"`
}

func Parse(metadata map[string]string) (Catalog, error) {
	if metadata == nil {
		return Catalog{}, ErrMissing
	}
	raw := strings.TrimSpace(metadata[MetadataKey])
	if raw == "" {
		return Catalog{}, ErrMissing
	}
	var catalog Catalog
	decoder := json.NewDecoder(strings.NewReader(raw))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&catalog); err != nil {
		return Catalog{}, fmt.Errorf("%w: %v", ErrInvalid, err)
	}
	if err := catalog.Validate(); err != nil {
		return Catalog{}, err
	}
	return catalog, nil
}

func ParseDefinition(metadata map[string]any) (Catalog, error) {
	if metadata == nil {
		return Catalog{}, ErrMissing
	}
	value, ok := metadata[MetadataKey]
	if !ok || value == nil {
		return Catalog{}, ErrMissing
	}
	if text, ok := value.(string); ok {
		return Parse(map[string]string{MetadataKey: text})
	}
	raw, err := json.Marshal(value)
	if err != nil {
		return Catalog{}, fmt.Errorf("%w: %v", ErrInvalid, err)
	}
	return Parse(map[string]string{MetadataKey: string(raw)})
}

func (c Catalog) Validate() error {
	if c.Version != 1 {
		return fmt.Errorf("%w: unsupported version %d", ErrInvalid, c.Version)
	}
	if strings.TrimSpace(c.Revision) == "" {
		return fmt.Errorf("%w: revision is required", ErrInvalid)
	}
	seen := make(map[string]struct{}, len(c.Behaviors))
	for index := range c.Behaviors {
		behavior := &c.Behaviors[index]
		behavior.ID = strings.TrimSpace(behavior.ID)
		behavior.Name = strings.TrimSpace(behavior.Name)
		behavior.Description = strings.TrimSpace(behavior.Description)
		behavior.ServiceID = strings.TrimSpace(behavior.ServiceID)
		behavior.Method = strings.TrimSpace(behavior.Method)
		behavior.ToolID = strings.TrimSpace(behavior.ToolID)
		behavior.Risk = strings.ToLower(strings.TrimSpace(behavior.Risk))
		if !behaviorIDPattern.MatchString(behavior.ID) {
			return fmt.Errorf("%w: behavior[%d] id is invalid", ErrInvalid, index)
		}
		if _, exists := seen[behavior.ID]; exists {
			return fmt.Errorf("%w: duplicate behavior %s", ErrInvalid, behavior.ID)
		}
		seen[behavior.ID] = struct{}{}
		if behavior.Name == "" {
			return fmt.Errorf("%w: behavior %s name is required", ErrInvalid, behavior.ID)
		}
		if len(behavior.InputSchema) == 0 || !json.Valid(behavior.InputSchema) {
			return fmt.Errorf("%w: behavior %s inputSchema is invalid", ErrInvalid, behavior.ID)
		}
		var schema map[string]any
		if err := json.Unmarshal(behavior.InputSchema, &schema); err != nil || schema == nil {
			return fmt.Errorf("%w: behavior %s inputSchema must be an object", ErrInvalid, behavior.ID)
		}
		switch behavior.Risk {
		case "", "low", "medium", "high":
		default:
			return fmt.Errorf("%w: behavior %s risk is invalid", ErrInvalid, behavior.ID)
		}
	}
	sort.Slice(c.Behaviors, func(i, j int) bool { return c.Behaviors[i].ID < c.Behaviors[j].ID })
	return nil
}

func (c Catalog) Find(id string) (Behavior, bool) {
	id = strings.TrimSpace(id)
	for _, behavior := range c.Behaviors {
		if behavior.ID == id {
			return behavior, true
		}
	}
	return Behavior{}, false
}
