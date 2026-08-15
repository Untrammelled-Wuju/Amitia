package schema

// RenderSchemaRequest 请求 Flutter Renderer 渲染一个 Schema UI
type RenderSchemaRequest struct {
	Document     *SchemaUIDocument `json:"document"`
	Mode         string            `json:"mode,omitempty"`
	Width        int               `json:"width,omitempty"`
	Height       int               `json:"height,omitempty"`
	Locale       string            `json:"locale,omitempty"`
	Theme        string            `json:"theme,omitempty"`
	Platform     string            `json:"platform"`
	PreviewToken string            `json:"previewToken,omitempty"`
	Resolution   string            `json:"resolution,omitempty"`
	Incremental  bool              `json:"incremental,omitempty"`
	ChangedPaths []string          `json:"changedPaths,omitempty"`
}

type RenderSchemaResponse struct {
	Rendered     bool        `json:"rendered"`
	PreviewToken string      `json:"previewToken,omitempty"`
	ErrorMessage string      `json:"errorMessage,omitempty"`
	FrameworkSeg interface{} `json:"frameworkSeg,omitempty"`
}
