package schema

import "sync"

// SchemaComponentType identifies a component type in the catalog.
type SchemaComponentType string

const (
	CompPage     SchemaComponentType = "page"
	CompSection  SchemaComponentType = "section"
	CompStack    SchemaComponentType = "stack"
	CompRow      SchemaComponentType = "row"
	CompGrid     SchemaComponentType = "grid"
	CompTabs     SchemaComponentType = "tabs"
	CompCard     SchemaComponentType = "card"
	CompText     SchemaComponentType = "text"
	CompMarkdown SchemaComponentType = "markdown"
	CompBadge    SchemaComponentType = "badge"
	CompIcon     SchemaComponentType = "icon"
	CompImage    SchemaComponentType = "image"
	CompField    SchemaComponentType = "field"
	CompSelect   SchemaComponentType = "select"
	CompSwitch   SchemaComponentType = "switch"
	CompSlider   SchemaComponentType = "slider"
	CompButton   SchemaComponentType = "button"
	CompList     SchemaComponentType = "list"
	CompTable    SchemaComponentType = "table"
	CompProgress SchemaComponentType = "progress"
)

// ComponentSchema defines a single component type's schema metadata.
type ComponentSchema struct {
	Type             SchemaComponentType   `json:"type"`
	Name             string                `json:"name"`
	Description      string                `json:"description"`
	Properties       []PropertySchema      `json:"properties"`
	RequiredProps    []string              `json:"requiredProps,omitempty"`
	AllowedChildren  []SchemaComponentType `json:"allowedChildren,omitempty"`
	Actions          []string              `json:"actions,omitempty"`
	Platforms        []string              `json:"platforms,omitempty"`
	HasBindingTarget bool                  `json:"hasBindingTarget"`
}

// PropertySchema describes a single property of a component.
type PropertySchema struct {
	Name     string   `json:"name"`
	Type     string   `json:"type"`
	required bool     `json:"-"`
	Required bool     `json:"required"`
	Enum     []string `json:"enum,omitempty"`
	Default  any      `json:"default,omitempty"`
}

// ComponentCatalog holds registered component schemas.
type ComponentCatalog struct {
	mu    sync.RWMutex
	comps map[SchemaComponentType]ComponentSchema
}

// NewComponentCatalog creates an empty catalog.
func NewComponentCatalog() *ComponentCatalog {
	return &ComponentCatalog{
		comps: make(map[SchemaComponentType]ComponentSchema),
	}
}

// Register adds or replaces a component schema.
func (c *ComponentCatalog) Register(schema ComponentSchema) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.comps[schema.Type] = schema
}

// Get retrieves a component schema by type.
func (c *ComponentCatalog) Get(compType SchemaComponentType) (ComponentSchema, bool) {
	c.mu.RLock()
	defer c.mu.RUnlock()
	s, ok := c.comps[compType]
	return s, ok
}

// List returns all registered component schemas.
func (c *ComponentCatalog) List() []ComponentSchema {
	c.mu.RLock()
	defer c.mu.RUnlock()
	result := make([]ComponentSchema, 0, len(c.comps))
	for _, s := range c.comps {
		result = append(result, s)
	}
	return result
}

// FindByAction returns all component schemas that support a given action.
func (c *ComponentCatalog) FindByAction(action string) []ComponentSchema {
	c.mu.RLock()
	defer c.mu.RUnlock()
	var result []ComponentSchema
	for _, s := range c.comps {
		for _, a := range s.Actions {
			if a == action {
				result = append(result, s)
				break
			}
		}
	}
	return result
}

// DefaultCatalog is the built-in catalog pre-populated with all standard widgets.
var DefaultCatalog *ComponentCatalog

func init() {
	DefaultCatalog = NewComponentCatalog()

	// --- Layout components ---
	DefaultCatalog.Register(ComponentSchema{
		Type:             CompPage,
		Name:             "Page",
		Description:      "Top-level page container with a root stack.",
		Properties:       []PropertySchema{{Name: "title", Type: "string", Required: true}, {Name: "theme", Type: "string", Enum: []string{"light", "dark", "auto"}, Default: "auto"}},
		RequiredProps:    []string{"title"},
		AllowedChildren:  []SchemaComponentType{CompSection, CompStack, CompTabs, CompList, CompTable},
		Platforms:        []string{"web", "ios", "android"},
		HasBindingTarget: false,
	})

	DefaultCatalog.Register(ComponentSchema{
		Type:             CompSection,
		Name:             "Section",
		Description:      "A titled section grouping child components.",
		Properties:       []PropertySchema{{Name: "title", Type: "string", Required: true}, {Name: "collapsible", Type: "boolean", Default: false}},
		RequiredProps:    []string{"title"},
		AllowedChildren:  []SchemaComponentType{CompStack, CompRow, CompGrid, CompCard, CompText, CompMarkdown, CompImage, CompList, CompTable, CompField, CompSelect, CompSwitch, CompSlider, CompButton, CompProgress, CompBadge, CompTabs},
		Platforms:        []string{"web", "ios", "android"},
		HasBindingTarget: false,
	})

	DefaultCatalog.Register(ComponentSchema{
		Type:             CompStack,
		Name:             "Stack",
		Description:      "Vertical layout stack.",
		Properties:       []PropertySchema{{Name: "spacing", Type: "number", Default: 8}, {Name: "align", Type: "string", Enum: []string{"start", "center", "end", "stretch"}, Default: "start"}},
		AllowedChildren:  []SchemaComponentType{CompText, CompMarkdown, CompBadge, CompIcon, CompImage, CompField, CompSelect, CompSwitch, CompSlider, CompButton, CompCard, CompProgress, CompRow, CompGrid, CompList, CompTabs},
		Platforms:        []string{"web", "ios", "android"},
		HasBindingTarget: false,
	})

	DefaultCatalog.Register(ComponentSchema{
		Type:             CompRow,
		Name:             "Row",
		Description:      "Horizontal layout row.",
		Properties:       []PropertySchema{{Name: "spacing", Type: "number", Default: 8}, {Name: "align", Type: "string", Enum: []string{"start", "center", "end"}, Default: "center"}},
		AllowedChildren:  []SchemaComponentType{CompText, CompBadge, CompIcon, CompImage, CompButton, CompField, CompSelect, CompSwitch},
		Platforms:        []string{"web", "ios", "android"},
		HasBindingTarget: false,
	})

	DefaultCatalog.Register(ComponentSchema{
		Type:             CompGrid,
		Name:             "Grid",
		Description:      "Grid layout with configurable columns.",
		Properties:       []PropertySchema{{Name: "columns", Type: "number", Required: true, Default: 2}, {Name: "spacing", Type: "number", Default: 8}},
		RequiredProps:    []string{"columns"},
		AllowedChildren:  []SchemaComponentType{CompCard, CompText, CompBadge, CompImage, CompButton, CompProgress},
		Platforms:        []string{"web", "ios", "android"},
		HasBindingTarget: false,
	})

	DefaultCatalog.Register(ComponentSchema{
		Type:             CompTabs,
		Name:             "Tabs",
		Description:      "Tabbed content panels.",
		Properties:       []PropertySchema{{Name: "tabs", Type: "array", Required: true}, {Name: "defaultTab", Type: "string"}},
		RequiredProps:    []string{"tabs"},
		AllowedChildren:  []SchemaComponentType{CompStack, CompSection, CompList, CompTable},
		Platforms:        []string{"web", "ios", "android"},
		HasBindingTarget: false,
	})

	DefaultCatalog.Register(ComponentSchema{
		Type:             CompCard,
		Name:             "Card",
		Description:      "Visual card container.",
		Properties:       []PropertySchema{{Name: "elevation", Type: "number", Default: 1}, {Name: "padding", Type: "number", Default: 16}},
		AllowedChildren:  []SchemaComponentType{CompStack, CompRow, CompText, CompMarkdown, CompBadge, CompImage, CompButton, CompIcon, CompField, CompProgress},
		Platforms:        []string{"web", "ios", "android"},
		HasBindingTarget: false,
	})

	// --- Display components ---
	DefaultCatalog.Register(ComponentSchema{
		Type:             CompText,
		Name:             "Text",
		Description:      "Text display component.",
		Properties:       []PropertySchema{{Name: "content", Type: "string", Required: true}, {Name: "size", Type: "string", Enum: []string{"xs", "sm", "md", "lg", "xl"}, Default: "md"}, {Name: "weight", Type: "string", Enum: []string{"normal", "medium", "bold"}, Default: "normal"}, {Name: "color", Type: "string"}},
		RequiredProps:    []string{"content"},
		Platforms:        []string{"web", "ios", "android"},
		HasBindingTarget: true,
	})

	DefaultCatalog.Register(ComponentSchema{
		Type:             CompMarkdown,
		Name:             "Markdown",
		Description:      "Markdown-rendered text.",
		Properties:       []PropertySchema{{Name: "content", Type: "string", Required: true}},
		RequiredProps:    []string{"content"},
		Platforms:        []string{"web", "ios", "android"},
		HasBindingTarget: true,
	})

	DefaultCatalog.Register(ComponentSchema{
		Type:             CompBadge,
		Name:             "Badge",
		Description:      "Status badge or tag.",
		Properties:       []PropertySchema{{Name: "label", Type: "string", Required: true}, {Name: "variant", Type: "string", Enum: []string{"default", "success", "warning", "error", "info"}, Default: "default"}},
		RequiredProps:    []string{"label"},
		Platforms:        []string{"web", "ios", "android"},
		HasBindingTarget: true,
	})

	DefaultCatalog.Register(ComponentSchema{
		Type:             CompIcon,
		Name:             "Icon",
		Description:      "Icon component.",
		Properties:       []PropertySchema{{Name: "name", Type: "string", Required: true}, {Name: "size", Type: "number", Default: 24}, {Name: "color", Type: "string"}},
		RequiredProps:    []string{"name"},
		Platforms:        []string{"web", "ios", "android"},
		HasBindingTarget: false,
	})

	DefaultCatalog.Register(ComponentSchema{
		Type:             CompImage,
		Name:             "Image",
		Description:      "Image display component.",
		Properties:       []PropertySchema{{Name: "src", Type: "string", Required: true}, {Name: "alt", Type: "string"}, {Name: "aspectRatio", Type: "string"}},
		RequiredProps:    []string{"src"},
		Platforms:        []string{"web", "ios", "android"},
		HasBindingTarget: true,
	})

	// --- Input / interactive components ---
	DefaultCatalog.Register(ComponentSchema{
		Type:             CompField,
		Name:             "Field",
		Description:      "Text input field.",
		Properties:       []PropertySchema{{Name: "label", Type: "string", Required: true}, {Name: "placeholder", Type: "string"}, {Name: "type", Type: "string", Enum: []string{"text", "number", "email", "password", "tel"}, Default: "text"}, {Name: "required", Type: "boolean", Default: false}},
		RequiredProps:    []string{"label"},
		Actions:          []string{"set_state"},
		Platforms:        []string{"web", "ios", "android"},
		HasBindingTarget: true,
	})

	DefaultCatalog.Register(ComponentSchema{
		Type:             CompSelect,
		Name:             "Select",
		Description:      "Dropdown select component.",
		Properties:       []PropertySchema{{Name: "label", Type: "string", Required: true}, {Name: "options", Type: "array", Required: true}, {Name: "placeholder", Type: "string"}},
		RequiredProps:    []string{"label", "options"},
		Actions:          []string{"set_state"},
		Platforms:        []string{"web", "ios", "android"},
		HasBindingTarget: true,
	})

	DefaultCatalog.Register(ComponentSchema{
		Type:             CompSwitch,
		Name:             "Switch",
		Description:      "Toggle switch component.",
		Properties:       []PropertySchema{{Name: "label", Type: "string", Required: true}, {Name: "defaultValue", Type: "boolean", Default: false}},
		RequiredProps:    []string{"label"},
		Actions:          []string{"set_state"},
		Platforms:        []string{"web", "ios", "android"},
		HasBindingTarget: true,
	})

	DefaultCatalog.Register(ComponentSchema{
		Type:             CompSlider,
		Name:             "Slider",
		Description:      "Range slider component.",
		Properties:       []PropertySchema{{Name: "label", Type: "string", Required: true}, {Name: "min", Type: "number", Default: 0}, {Name: "max", Type: "number", Default: 100}, {Name: "step", Type: "number", Default: 1}},
		RequiredProps:    []string{"label"},
		Actions:          []string{"set_state"},
		Platforms:        []string{"web", "ios", "android"},
		HasBindingTarget: true,
	})

	DefaultCatalog.Register(ComponentSchema{
		Type:             CompButton,
		Name:             "Button",
		Description:      "Action button.",
		Properties:       []PropertySchema{{Name: "label", Type: "string", Required: true}, {Name: "variant", Type: "string", Enum: []string{"primary", "secondary", "danger", "ghost"}, Default: "primary"}, {Name: "size", Type: "string", Enum: []string{"sm", "md", "lg"}, Default: "md"}},
		RequiredProps:    []string{"label"},
		Actions:          []string{"navigate", "invoke_tool", "invoke_capability", "open_resource", "set_state", "submit_form"},
		Platforms:        []string{"web", "ios", "android"},
		HasBindingTarget: false,
	})

	// --- Collection components ---
	DefaultCatalog.Register(ComponentSchema{
		Type:             CompList,
		Name:             "List",
		Description:      "Dynamic list bound to a data source.",
		Properties:       []PropertySchema{{Name: "dataSource", Type: "string", Required: true}, {Name: "itemTemplate", Type: "object", Required: true}, {Name: "pageSize", Type: "number", Default: 20}},
		RequiredProps:    []string{"dataSource", "itemTemplate"},
		Platforms:        []string{"web", "ios", "android"},
		HasBindingTarget: true,
	})

	DefaultCatalog.Register(ComponentSchema{
		Type:             CompTable,
		Name:             "Table",
		Description:      "Data table with columns.",
		Properties:       []PropertySchema{{Name: "dataSource", Type: "string", Required: true}, {Name: "columns", Type: "array", Required: true}, {Name: "pageSize", Type: "number", Default: 20}},
		RequiredProps:    []string{"dataSource", "columns"},
		Platforms:        []string{"web", "ios", "android"},
		HasBindingTarget: true,
	})

	// --- Feedback components ---
	DefaultCatalog.Register(ComponentSchema{
		Type:             CompProgress,
		Name:             "Progress",
		Description:      "Progress indicator.",
		Properties:       []PropertySchema{{Name: "value", Type: "number", Required: true}, {Name: "max", Type: "number", Default: 100}, {Name: "label", Type: "string"}},
		RequiredProps:    []string{"value"},
		Platforms:        []string{"web", "ios", "android"},
		HasBindingTarget: true,
	})
}
