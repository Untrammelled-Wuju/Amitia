package schema_ui

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
)

const SchemaUIVersion = "schema-ui/1"

type NodeType string

const (
	NodePage              NodeType = "page"
	NodeSection           NodeType = "section"
	NodeStack             NodeType = "stack"
	NodeRow               NodeType = "row"
	NodeGrid              NodeType = "grid"
	NodeTabs              NodeType = "tabs"
	NodeCard              NodeType = "card"
	NodeText              NodeType = "text"
	NodeMarkdown          NodeType = "markdown"
	NodeBadge             NodeType = "badge"
	NodeDivider           NodeType = "divider"
	NodeIcon              NodeType = "icon"
	NodeImage             NodeType = "image"
	NodeField             NodeType = "field"
	NodeSelect            NodeType = "select"
	NodeSwitch            NodeType = "switch"
	NodeSlider            NodeType = "slider"
	NodeButton            NodeType = "button"
	NodeButtonGroup       NodeType = "button_group"
	NodeList              NodeType = "list"
	NodeTable             NodeType = "table"
	NodeEmptyState        NodeType = "empty_state"
	NodeAlert             NodeType = "alert"
	NodeProgress          NodeType = "progress"
	NodeCode              NodeType = "code"
	NodeKeyValue          NodeType = "key_value"
	NodeResourceLink      NodeType = "resource_link"
	NodePermissionSummary NodeType = "permission_summary"
	NodeRuntimeStatus     NodeType = "runtime_status"
	NodeTabItem           NodeType = "tab_item"
	NodeColumn            NodeType = "column"
)

var allowedNodeTypes = map[NodeType]bool{
	NodePage: true, NodeSection: true, NodeStack: true, NodeRow: true,
	NodeGrid: true, NodeTabs: true, NodeCard: true, NodeText: true,
	NodeMarkdown: true, NodeBadge: true, NodeDivider: true, NodeIcon: true,
	NodeImage: true, NodeField: true, NodeSelect: true, NodeSwitch: true,
	NodeSlider: true, NodeButton: true, NodeButtonGroup: true, NodeList: true,
	NodeTable: true, NodeEmptyState: true, NodeAlert: true, NodeProgress: true,
	NodeCode: true, NodeKeyValue: true, NodeResourceLink: true,
	NodePermissionSummary: true, NodeRuntimeStatus: true,
	NodeTabItem: true, NodeColumn: true,
}

var forbiddenNodeTypes = map[string]bool{
	"html": true, "script": true, "style": true, "iframe": true,
	"webview": true, "canvas": true, "template": true,
}

type BindingSource string

const (
	SourceInput         BindingSource = "input"
	SourceState         BindingSource = "state"
	SourceQuery         BindingSource = "query"
	SourceRuntime       BindingSource = "runtime"
	SourceHost          BindingSource = "host"
	SourceForm          BindingSource = "form"
	SourceStatic        BindingSource = "static"
	SourceStorage       BindingSource = "storage"
	SourceRuntimeStatus BindingSource = "runtime_status"
	SourceResourceList  BindingSource = "resource_list"
)

var allowedBindingSources = map[BindingSource]bool{
	SourceInput: true, SourceState: true, SourceQuery: true,
	SourceRuntime: true, SourceHost: true, SourceForm: true,
	SourceStatic: true, SourceStorage: true,
	SourceRuntimeStatus: true, SourceResourceList: true,
}

type SchemaUIBinding struct {
	Path    string          `json:"path"`
	Source  BindingSource   `json:"source"`
	Format  string          `json:"format,omitempty"`
	Default json.RawMessage `json:"default,omitempty"`
}

type SchemaUIActionBinding struct {
	ActionID     string          `json:"action_id"`
	Target       string          `json:"target"`
	Input        json.RawMessage `json:"input,omitempty"`
	Confirmation string          `json:"confirmation,omitempty"`
}

type UICondition struct {
	Field    string `json:"field"`
	Operator string `json:"operator"`
	Value    any    `json:"value"`
}

type SchemaUINode struct {
	ID         string                  `json:"id"`
	Type       NodeType                `json:"type"`
	Props      json.RawMessage         `json:"props,omitempty"`
	Bindings   []SchemaUIBinding       `json:"bindings,omitempty"`
	Actions    []SchemaUIActionBinding `json:"actions,omitempty"`
	Visibility []UICondition           `json:"visibility,omitempty"`
	Children   []SchemaUINode          `json:"children,omitempty"`
}

type SchemaUIDocument struct {
	SchemaVersion string                   `json:"schemaVersion"`
	Type          string                   `json:"type"`
	Title         string                   `json:"title,omitempty"`
	Layout        map[string]any           `json:"layout,omitempty"`
	Children      []SchemaUINode           `json:"children"`
	DataSources   []SchemaUIDataSource     `json:"dataSources,omitempty"`
	Actions       []SchemaUIDeclaredAction `json:"actions,omitempty"`
	Theme         *ThemeConfig             `json:"theme,omitempty"`
	Locale        *LocaleConfig            `json:"locale,omitempty"`
	Accessibility *AccessibilityConfig     `json:"accessibility,omitempty"`
}

type UITheme string

const (
	UIThemeLight UITheme = "light"
	UIThemeDark  UITheme = "dark"
	UIThemeAuto  UITheme = "auto"
)

type ThemeConfig struct {
	Mode      UITheme           `json:"mode"`
	Overrides map[string]string `json:"overrides,omitempty"`
}

type LocaleConfig struct {
	Current   string   `json:"current"`
	Available []string `json:"available,omitempty"`
}

type AccessibilityConfig struct {
	Enabled       bool `json:"enabled"`
	HighContrast  bool `json:"highContrast,omitempty"`
	ReducedMotion bool `json:"reducedMotion,omitempty"`
	ScreenReader  bool `json:"screenReader,omitempty"`
	KeyboardNav   bool `json:"keyboardNav,omitempty"`
}

type PerformanceBudget struct {
	MaxRenderTimeMS   int64 `json:"maxRenderTimeMs"`
	MaxLayoutCount    int   `json:"maxLayoutCount"`
	MaxNodeCount      int   `json:"maxNodeCount"`
	MaxDataFetchCount int   `json:"maxDataFetchCount"`
	MaxActionCount    int   `json:"maxActionCount"`
}

type SchemaUIDataSource struct {
	ID            string          `json:"id"`
	Type          BindingSource   `json:"type"`
	InputSchema   json.RawMessage `json:"inputSchema,omitempty"`
	OutputSchema  json.RawMessage `json:"outputSchema,omitempty"`
	RefreshPolicy string          `json:"refreshPolicy,omitempty"`
	RuntimeEntry  string          `json:"runtimeEntry,omitempty"`
}

type SchemaUIDeclaredAction struct {
	ActionID    string          `json:"actionId"`
	Target      string          `json:"target"`
	InputSchema json.RawMessage `json:"inputSchema,omitempty"`
}

type SchemaLimits struct {
	MaxFileBytes      int64
	MaxNodes          int
	MaxDepth          int
	MaxExpressionLen  int
	MaxActions        int
	MaxDataSources    int
	MaxGridColumns    int
	MaxTableRows      int
	MaxMarkdownKB     int
	MaxImageKB        int
	MaxChildren       int
	PerformanceBudget *PerformanceBudget
}

func DefaultLimits() SchemaLimits {
	return SchemaLimits{
		MaxFileBytes:     1024 * 1024,
		MaxNodes:         500,
		MaxDepth:         12,
		MaxExpressionLen: 256,
		MaxActions:       64,
		MaxDataSources:   64,
		MaxGridColumns:   12,
		MaxTableRows:     200,
		MaxMarkdownKB:    64,
		MaxImageKB:       256,
		MaxChildren:      64,
	}
}

var (
	ErrInvalidSchemaVersion = errors.New("schema_ui: invalid schemaVersion")
	ErrInvalidNodeType      = errors.New("schema_ui: invalid node type")
	ErrForbiddenNodeType    = errors.New("schema_ui: forbidden node type")
	ErrTooManyNodes         = errors.New("schema_ui: too many nodes")
	ErrDepthExceeded        = errors.New("schema_ui: max depth exceeded")
	ErrTooManyChildren      = errors.New("schema_ui: too many children")
	ErrGridColumnsExceeded  = errors.New("schema_ui: grid columns exceeded")
	ErrInvalidBindingSource = errors.New("schema_ui: invalid binding source")
	ErrInvalidExpression    = errors.New("schema_ui: invalid expression")
	ErrActionNotDeclared    = errors.New("schema_ui: action not declared")
	ErrMissingNodeID        = errors.New("schema_ui: missing node id")
	ErrDuplicateNodeID      = errors.New("schema_ui: duplicate node id")
	ErrEmptyDocument        = errors.New("schema_ui: empty document")
)

var (
	exprAllowedPattern = regexp.MustCompile(`^[a-zA-Z_$][a-zA-Z0-9_$.]*$`)
	htmlPattern        = regexp.MustCompile(`(?i)<(script|style|iframe|webview|canvas|template|html)[\s>]`)
)

type Validator struct {
	limits SchemaLimits
}

func NewValidator() *Validator {
	return &Validator{limits: DefaultLimits()}
}

func (v *Validator) WithLimits(limits SchemaLimits) *Validator {
	v.limits = limits
	return v
}

type ValidationResult struct {
	Valid         bool
	NodeCount     int
	MaxDepth      int
	Errors        []string
	Warnings      []string
	ActionIDs     map[string]bool
	DataSourceIDs map[string]bool
}

func (v *Validator) Validate(doc *SchemaUIDocument) *ValidationResult {
	result := &ValidationResult{
		ActionIDs:     make(map[string]bool),
		DataSourceIDs: make(map[string]bool),
	}
	if doc == nil {
		result.Errors = append(result.Errors, "nil document")
		return result
	}
	if doc.SchemaVersion != SchemaUIVersion {
		result.Errors = append(result.Errors, fmt.Sprintf("%v: expected %s got %s", ErrInvalidSchemaVersion, SchemaUIVersion, doc.SchemaVersion))
		return result
	}
	if doc.Type == "" {
		result.Errors = append(result.Errors, "missing document type")
		return result
	}
	if len(doc.Children) == 0 {
		result.Errors = append(result.Errors, ErrEmptyDocument.Error())
		return result
	}
	if len(doc.Actions) > v.limits.MaxActions {
		result.Errors = append(result.Errors, fmt.Sprintf("too many actions: %d > %d", len(doc.Actions), v.limits.MaxActions))
	}
	if len(doc.DataSources) > v.limits.MaxDataSources {
		result.Errors = append(result.Errors, fmt.Sprintf("too many data sources: %d > %d", len(doc.DataSources), v.limits.MaxDataSources))
	}
	for _, a := range doc.Actions {
		if a.ActionID == "" || a.Target == "" {
			result.Errors = append(result.Errors, "declared action requires actionId and target")
			continue
		}
		if result.ActionIDs[a.ActionID] {
			result.Errors = append(result.Errors, "duplicate action: "+a.ActionID)
		}
		result.ActionIDs[a.ActionID] = true
	}
	for _, ds := range doc.DataSources {
		if ds.ID == "" {
			result.Errors = append(result.Errors, "data source id required")
			continue
		}
		if result.DataSourceIDs[ds.ID] {
			result.Errors = append(result.Errors, "duplicate data source: "+ds.ID)
		}
		result.DataSourceIDs[ds.ID] = true
		if !allowedBindingSources[ds.Type] {
			result.Errors = append(result.Errors, fmt.Sprintf("data source %s invalid type %s", ds.ID, ds.Type))
		}
	}
	seenIDs := make(map[string]bool)
	for i := range doc.Children {
		v.validateNode(&doc.Children[i], 1, result, seenIDs)
	}
	if result.NodeCount > v.limits.MaxNodes {
		result.Errors = append(result.Errors, fmt.Sprintf("%v: %d > %d", ErrTooManyNodes, result.NodeCount, v.limits.MaxNodes))
	}
	if result.MaxDepth > v.limits.MaxDepth {
		result.Errors = append(result.Errors, fmt.Sprintf("%v: %d > %d", ErrDepthExceeded, result.MaxDepth, v.limits.MaxDepth))
	}
	result.Valid = len(result.Errors) == 0
	return result
}

func (v *Validator) validateNode(node *SchemaUINode, depth int, result *ValidationResult, seenIDs map[string]bool) {
	if node == nil {
		result.Errors = append(result.Errors, "nil node at depth "+fmt.Sprint(depth))
		return
	}
	result.NodeCount++
	if depth > result.MaxDepth {
		result.MaxDepth = depth
	}
	if node.ID == "" {
		result.Errors = append(result.Errors, fmt.Sprintf("%v at depth %d", ErrMissingNodeID, depth))
	} else {
		if seenIDs[node.ID] {
			result.Errors = append(result.Errors, fmt.Sprintf("%v: %s", ErrDuplicateNodeID, node.ID))
		}
		seenIDs[node.ID] = true
	}
	if forbiddenNodeTypes[string(node.Type)] {
		result.Errors = append(result.Errors, fmt.Sprintf("%v: %s", ErrForbiddenNodeType, node.Type))
		return
	}
	if !allowedNodeTypes[node.Type] {
		result.Errors = append(result.Errors, fmt.Sprintf("%v: %s", ErrInvalidNodeType, node.Type))
		return
	}
	for _, b := range node.Bindings {
		if !allowedBindingSources[b.Source] {
			result.Errors = append(result.Errors, fmt.Sprintf("%v: %s", ErrInvalidBindingSource, b.Source))
		}
		if b.Path != "" && (len(b.Path) > v.limits.MaxExpressionLen || !exprAllowedPattern.MatchString(b.Path)) {
			result.Errors = append(result.Errors, fmt.Sprintf("%v: %s", ErrInvalidExpression, b.Path))
		}
	}
	for _, a := range node.Actions {
		if a.ActionID == "" {
			result.Errors = append(result.Errors, "node "+node.ID+": action_id empty")
		} else if !result.ActionIDs[a.ActionID] {
			result.Errors = append(result.Errors, fmt.Sprintf("%v: %s", ErrActionNotDeclared, a.ActionID))
		}
	}
	for _, condition := range node.Visibility {
		if condition.Field == "" || len(condition.Field) > v.limits.MaxExpressionLen || !exprAllowedPattern.MatchString(condition.Field) {
			result.Errors = append(result.Errors, fmt.Sprintf("%v: %s", ErrInvalidExpression, condition.Field))
		}
	}
	if node.Type == NodeGrid {
		var props struct {
			Columns int `json:"columns"`
		}
		_ = json.Unmarshal(node.Props, &props)
		if props.Columns > v.limits.MaxGridColumns {
			result.Errors = append(result.Errors, fmt.Sprintf("%v: %d > %d", ErrGridColumnsExceeded, props.Columns, v.limits.MaxGridColumns))
		}
	}
	if node.Type == NodeTable {
		var props struct {
			Rows int `json:"rows"`
		}
		_ = json.Unmarshal(node.Props, &props)
		if props.Rows > v.limits.MaxTableRows {
			result.Errors = append(result.Errors, fmt.Sprintf("table %s rows %d exceeds %d", node.ID, props.Rows, v.limits.MaxTableRows))
		}
	}
	if node.Type == NodeMarkdown {
		var props struct {
			Content string `json:"content"`
		}
		_ = json.Unmarshal(node.Props, &props)
		if htmlPattern.MatchString(props.Content) {
			result.Errors = append(result.Errors, "markdown "+node.ID+": contains forbidden HTML")
		}
		if len(props.Content) > v.limits.MaxMarkdownKB*1024 {
			result.Errors = append(result.Errors, "markdown "+node.ID+": content too large")
		}
	}
	if node.Type == NodeImage {
		var props struct {
			Src string `json:"src"`
		}
		_ = json.Unmarshal(node.Props, &props)
		if strings.HasPrefix(props.Src, "data:") {
			result.Warnings = append(result.Warnings, "image "+node.ID+": data URL not recommended")
		}
		if strings.HasPrefix(props.Src, "http://") || strings.HasPrefix(props.Src, "https://") {
			result.Warnings = append(result.Warnings, "image "+node.ID+": remote URL must be controlled")
		}
	}
	if len(node.Children) > v.limits.MaxChildren {
		result.Errors = append(result.Errors, fmt.Sprintf("%v: node %s has %d children", ErrTooManyChildren, node.ID, len(node.Children)))
	}
	for i := range node.Children {
		v.validateNode(&node.Children[i], depth+1, result, seenIDs)
	}
}

type CompiledDocument struct {
	Document        *SchemaUIDocument
	Hash            string
	NodeIndex       map[string]*SchemaUINode
	ActionIndex     map[string]*SchemaUIDeclaredAction
	DataSourceIndex map[string]*SchemaUIDataSource
}

func Compile(doc *SchemaUIDocument) (*CompiledDocument, error) {
	v := NewValidator()
	result := v.Validate(doc)
	if !result.Valid {
		return nil, fmt.Errorf("schema_ui: validation failed: %s", strings.Join(result.Errors, "; "))
	}
	compiled := &CompiledDocument{
		Document:        doc,
		NodeIndex:       make(map[string]*SchemaUINode),
		ActionIndex:     make(map[string]*SchemaUIDeclaredAction),
		DataSourceIndex: make(map[string]*SchemaUIDataSource),
	}
	for i := range doc.Actions {
		compiled.ActionIndex[doc.Actions[i].ActionID] = &doc.Actions[i]
	}
	for i := range doc.DataSources {
		compiled.DataSourceIndex[doc.DataSources[i].ID] = &doc.DataSources[i]
	}
	for i := range doc.Children {
		indexNodes(&doc.Children[i], compiled.NodeIndex)
	}
	compiled.Hash = computeHash(doc)
	return compiled, nil
}

func indexNodes(node *SchemaUINode, idx map[string]*SchemaUINode) {
	if node == nil {
		return
	}
	if node.ID != "" {
		idx[node.ID] = node
	}
	for i := range node.Children {
		indexNodes(&node.Children[i], idx)
	}
}

func computeHash(doc *SchemaUIDocument) string {
	data, err := json.Marshal(doc)
	if err != nil {
		return ""
	}
	hash := sha256.Sum256(data)
	return "sha256:" + hex.EncodeToString(hash[:])
}

type PageState string

const (
	PageStateIdle               PageState = "idle"
	PageStateLoading            PageState = "loading"
	PageStateReady              PageState = "ready"
	PageStateSubmitting         PageState = "submitting"
	PageStateSuccess            PageState = "success"
	PageStateError              PageState = "error"
	PageStateEmpty              PageState = "empty"
	PageStateOffline            PageState = "offline"
	PageStateRuntimeUnavailable PageState = "runtime_unavailable"
)

type ThemeTokens struct {
	Surface         string `json:"surface"`
	SurfaceElevated string `json:"surfaceElevated"`
	TextPrimary     string `json:"textPrimary"`
	TextSecondary   string `json:"textSecondary"`
	Border          string `json:"border"`
	Accent          string `json:"accent"`
	Danger          string `json:"danger"`
	Success         string `json:"success"`
	Warning         string `json:"warning"`
}

type Renderer struct {
	compiled *CompiledDocument
	theme    ThemeTokens
	locale   string
	state    map[string]any
	mu       struct {
		sync.Mutex
	}
}

func NewRenderer(compiled *CompiledDocument, theme ThemeTokens, locale string) *Renderer {
	return &Renderer{
		compiled: compiled,
		theme:    theme,
		locale:   locale,
		state:    make(map[string]any),
	}
}

func (r *Renderer) SetState(key string, value any) {
	r.mu.Lock()
	r.state[key] = value
	r.mu.Unlock()
}

func (r *Renderer) GetState(key string) any {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.state[key]
}

type RenderedNode struct {
	ID       string         `json:"id"`
	Type     NodeType       `json:"type"`
	Props    map[string]any `json:"props,omitempty"`
	Visible  bool           `json:"visible"`
	Children []RenderedNode `json:"children,omitempty"`
}

func (r *Renderer) Render(data map[string]any) []RenderedNode {
	out := make([]RenderedNode, 0, len(r.compiled.Document.Children))
	for i := range r.compiled.Document.Children {
		out = append(out, r.renderNode(&r.compiled.Document.Children[i], data))
	}
	return out
}

func (r *Renderer) renderNode(node *SchemaUINode, data map[string]any) RenderedNode {
	rendered := RenderedNode{
		ID:      node.ID,
		Type:    node.Type,
		Visible: r.evaluateVisibility(node.Visibility, data),
	}
	if len(node.Props) > 0 {
		_ = json.Unmarshal(node.Props, &rendered.Props)
	}
	for _, binding := range node.Bindings {
		val := r.resolveBinding(binding, data)
		if val != nil {
			if rendered.Props == nil {
				rendered.Props = make(map[string]any)
			}
			rendered.Props[binding.Path] = val
		}
	}
	for i := range node.Children {
		rendered.Children = append(rendered.Children, r.renderNode(&node.Children[i], data))
	}
	return rendered
}

func (r *Renderer) evaluateVisibility(conditions []UICondition, data map[string]any) bool {
	if len(conditions) == 0 {
		return true
	}
	for _, c := range conditions {
		val, ok := lookupPath(data, c.Field)
		if !ok {
			return false
		}
		if !evaluateCondition(val, c.Operator, c.Value) {
			return false
		}
	}
	return true
}

func (r *Renderer) resolveBinding(binding SchemaUIBinding, data map[string]any) any {
	switch binding.Source {
	case SourceInput, SourceForm, SourceState, SourceQuery:
		if val, ok := lookupPath(data, binding.Path); ok {
			return val
		}
		if len(binding.Default) > 0 {
			var v any
			_ = json.Unmarshal(binding.Default, &v)
			return v
		}
	}
	return nil
}

func lookupPath(data map[string]any, path string) (any, bool) {
	if path == "" {
		return nil, false
	}
	parts := strings.Split(path, ".")
	var current any = data
	for _, p := range parts {
		m, ok := current.(map[string]any)
		if !ok {
			return nil, false
		}
		current, ok = m[p]
		if !ok {
			return nil, false
		}
	}
	return current, true
}

func evaluateCondition(value any, op string, expected any) bool {
	switch op {
	case "==", "eq":
		return value == expected
	case "!=", "ne":
		return value != expected
	case "in":
		if arr, ok := expected.([]any); ok {
			for _, item := range arr {
				if item == value {
					return true
				}
			}
		}
		return false
	case "not_null":
		return value != nil
	case "is_null":
		return value == nil
	}
	return false
}

type ActionDispatcher struct {
	compiled *CompiledDocument
	handler  func(actionID string, input map[string]any) (map[string]any, error)
}

func NewActionDispatcher(compiled *CompiledDocument, handler func(actionID string, input map[string]any) (map[string]any, error)) *ActionDispatcher {
	return &ActionDispatcher{compiled: compiled, handler: handler}
}

func (d *ActionDispatcher) Dispatch(actionID string, input map[string]any) (map[string]any, error) {
	if _, ok := d.compiled.ActionIndex[actionID]; !ok {
		return nil, fmt.Errorf("%w: %s", ErrActionNotDeclared, actionID)
	}
	if d.handler == nil {
		return nil, errors.New("schema_ui: action dispatcher unavailable")
	}
	return d.handler(actionID, input)
}

type SecretReference struct {
	RefID   string `json:"ref_id"`
	Field   string `json:"field"`
	LeaseID string `json:"lease_id,omitempty"`
}

type SecretInputResolver struct {
	broker func(refID string) (string, error)
}

func NewSecretInputResolver(broker func(refID string) (string, error)) *SecretInputResolver {
	return &SecretInputResolver{broker: broker}
}

func (r *SecretInputResolver) Resolve(ref SecretReference) (string, error) {
	if r.broker == nil {
		return "", errors.New("schema_ui: secret broker not configured")
	}
	return r.broker(ref.RefID)
}

func (r *SecretInputResolver) ResolveFields(props map[string]any) (map[string]any, error) {
	out := make(map[string]any)
	for k, v := range props {
		if ref, ok := v.(SecretReference); ok && ref.RefID != "" {
			val, err := r.Resolve(ref)
			if err != nil {
				return nil, fmt.Errorf("schema_ui: resolve secret %s: %w", k, err)
			}
			out[k] = val
		} else {
			out[k] = v
		}
	}
	return out, nil
}

type CompilerCache struct {
	mu      sync.RWMutex
	entries map[string]*CompiledDocument
}

type CompilerCacheKey struct {
	DefinitionHash string
	Generation     int64
	Locale         string
	Theme          string
}

func (k CompilerCacheKey) String() string {
	return fmt.Sprintf("%s|%d|%s|%s", k.DefinitionHash, k.Generation, k.Locale, k.Theme)
}

func NewCompilerCache() *CompilerCache {
	return &CompilerCache{entries: make(map[string]*CompiledDocument)}
}

func (c *CompilerCache) Get(hash string) (*CompiledDocument, bool) {
	c.mu.RLock()
	defer c.mu.RUnlock()
	v, ok := c.entries[hash]
	if !ok {
		v, ok = c.entries[CompilerCacheKey{DefinitionHash: hash}.String()]
	}
	return v, ok
}

func (c *CompilerCache) Put(doc *CompiledDocument) {
	c.PutWithKey(CompilerCacheKey{DefinitionHash: doc.Hash}, doc)
}

func (c *CompilerCache) PutWithKey(key CompilerCacheKey, doc *CompiledDocument) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.entries[key.String()] = doc
}

func (c *CompilerCache) Delete(key CompilerCacheKey) {
	c.mu.Lock()
	delete(c.entries, key.String())
	c.mu.Unlock()
}

func (c *CompilerCache) Size() int {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return len(c.entries)
}

type SchemaRegistry struct {
	mu        sync.RWMutex
	validator *Validator
	cache     *CompilerCache
	schemas   map[string]*CompiledDocument
	cacheKeys map[string]CompilerCacheKey
}

func NewSchemaRegistry(validator *Validator, cache *CompilerCache) *SchemaRegistry {
	if validator == nil {
		validator = NewValidator()
	}
	if cache == nil {
		cache = NewCompilerCache()
	}
	return &SchemaRegistry{
		validator: validator,
		cache:     cache,
		schemas:   make(map[string]*CompiledDocument),
		cacheKeys: make(map[string]CompilerCacheKey),
	}
}

func schemaKey(extensionID, pageID string) string {
	return extensionID + "/" + pageID
}

func (r *SchemaRegistry) RegisterSchema(extensionID, pageID string, doc *SchemaUIDocument) error {
	return r.RegisterSchemaWithContext(extensionID, pageID, 0, "", "", doc)
}

func (r *SchemaRegistry) RegisterSchemaWithContext(extensionID, pageID string, generation int64, locale, theme string, doc *SchemaUIDocument) error {
	if extensionID == "" || pageID == "" {
		return fmt.Errorf("schema_ui: extension id and page id required")
	}
	if doc == nil {
		return fmt.Errorf("schema_ui: document is nil")
	}
	result := r.validator.Validate(doc)
	if !result.Valid {
		return fmt.Errorf("schema_ui: validation failed for %s: %s", schemaKey(extensionID, pageID), strings.Join(result.Errors, "; "))
	}
	compiled, err := Compile(doc)
	if err != nil {
		return fmt.Errorf("schema_ui: compile failed for %s: %w", schemaKey(extensionID, pageID), err)
	}
	key := schemaKey(extensionID, pageID)
	cacheKey := CompilerCacheKey{DefinitionHash: compiled.Hash, Generation: generation, Locale: locale, Theme: theme}
	r.mu.Lock()
	if old, ok := r.cacheKeys[key]; ok {
		r.cache.Delete(old)
	}
	r.schemas[key] = compiled
	r.cacheKeys[key] = cacheKey
	r.cache.PutWithKey(cacheKey, compiled)
	r.mu.Unlock()
	return nil
}

func (r *SchemaRegistry) LoadFromBytes(extensionID, pageID string, data []byte) error {
	return r.LoadFromBytesWithContext(extensionID, pageID, 0, "", "", data)
}

func (r *SchemaRegistry) LoadFromBytesWithContext(extensionID, pageID string, generation int64, locale, theme string, data []byte) error {
	if int64(len(data)) > r.validator.limits.MaxFileBytes {
		return fmt.Errorf("schema_ui: schema file too large: %d > %d", len(data), r.validator.limits.MaxFileBytes)
	}
	var doc SchemaUIDocument
	if err := json.Unmarshal(data, &doc); err != nil {
		return fmt.Errorf("schema_ui: unmarshal failed for %s: %w", schemaKey(extensionID, pageID), err)
	}
	return r.RegisterSchemaWithContext(extensionID, pageID, generation, locale, theme, &doc)
}

func (r *SchemaRegistry) LoadFromPath(extensionID, pageID, basePath, schemaPath string) error {
	return r.LoadFromPathWithContext(extensionID, pageID, 0, "", "", basePath, schemaPath)
}

func (r *SchemaRegistry) LoadFromPathWithContext(extensionID, pageID string, generation int64, locale, theme, basePath, schemaPath string) error {
	cleanPath, err := safeSchemaPath(basePath, schemaPath)
	if err != nil {
		return err
	}
	data, err := os.ReadFile(cleanPath)
	if err != nil {
		return fmt.Errorf("schema_ui: read schema file %s: %w", cleanPath, err)
	}
	if int64(len(data)) > r.validator.limits.MaxFileBytes {
		return fmt.Errorf("schema_ui: schema file too large: %s", cleanPath)
	}
	return r.LoadFromBytesWithContext(extensionID, pageID, generation, locale, theme, data)
}

func (r *SchemaRegistry) Get(extensionID, pageID string) (*CompiledDocument, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	compiled, ok := r.schemas[schemaKey(extensionID, pageID)]
	return compiled, ok
}

func (r *SchemaRegistry) Unregister(extensionID string) int {
	r.mu.Lock()
	defer r.mu.Unlock()
	count := 0
	for key := range r.schemas {
		if strings.HasPrefix(key, extensionID+"/") {
			if cacheKey, ok := r.cacheKeys[key]; ok {
				r.cache.Delete(cacheKey)
				delete(r.cacheKeys, key)
			}
			delete(r.schemas, key)
			count++
		}
	}
	return count
}

func (r *SchemaRegistry) UnregisterSchema(extensionID, pageID string) bool {
	key := schemaKey(extensionID, pageID)
	r.mu.Lock()
	defer r.mu.Unlock()
	if _, ok := r.schemas[key]; !ok {
		return false
	}
	if cacheKey, ok := r.cacheKeys[key]; ok {
		r.cache.Delete(cacheKey)
		delete(r.cacheKeys, key)
	}
	delete(r.schemas, key)
	return true
}

func (r *SchemaRegistry) Validator() *Validator {
	return r.validator
}

func (r *SchemaRegistry) Cache() *CompilerCache {
	return r.cache
}

func (r *SchemaRegistry) Size() int {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return len(r.schemas)
}

func safeSchemaPath(basePath, schemaPath string) (string, error) {
	if schemaPath == "" {
		return "", fmt.Errorf("schema_ui: schema path is empty")
	}
	if len(schemaPath) > 1024 {
		return "", fmt.Errorf("schema_ui: schema path too long")
	}
	if strings.ContainsRune(schemaPath, 0) {
		return "", fmt.Errorf("schema_ui: schema path contains null byte")
	}
	lower := strings.ToLower(schemaPath)
	if strings.Contains(lower, "%2e") || strings.Contains(lower, "%5c") || strings.Contains(lower, "%2f") {
		return "", fmt.Errorf("schema_ui: path traversal detected")
	}
	cleaned := filepath.Clean(schemaPath)
	if strings.HasPrefix(cleaned, "..") || strings.Contains(cleaned, "..\\") || strings.Contains(cleaned, "../") {
		return "", fmt.Errorf("schema_ui: path traversal detected")
	}
	if filepath.IsAbs(cleaned) {
		if basePath == "" {
			return "", fmt.Errorf("schema_ui: absolute path not allowed without base")
		}
		absBase, err := filepath.Abs(basePath)
		if err != nil {
			return "", fmt.Errorf("schema_ui: invalid base path")
		}
		absCleaned, err := filepath.Abs(cleaned)
		if err != nil {
			return "", fmt.Errorf("schema_ui: invalid schema path")
		}
		rel, err := filepath.Rel(absBase, absCleaned)
		if err != nil || strings.HasPrefix(rel, "..") {
			return "", fmt.Errorf("schema_ui: path outside base directory")
		}
		return absCleaned, nil
	}
	full := filepath.Join(basePath, cleaned)
	return full, nil
}
