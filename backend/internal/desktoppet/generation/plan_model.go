package generation

type GenerationPlanSnapshot struct {
	SchemaVersion               int
	ActionKey                   string
	Mode                        string
	Provider                    string
	Model                       string
	ConfigID                    int
	ConfigRevision              string
	CapabilityHash              string
	ReferenceAssetID            string
	LayoutJSON                  string
	LayoutHash                  string
	PromptTemplateVersion       string
	PromptDocumentJSON          string
	PromptSnapshot              string
	PromptHash                  string
	NegativePromptSnapshot      string
	NegativePromptHash          string
	SeedPolicy                  string
	SeedValue                   *int64
	OutputCount                 int
	TargetFrameCount            int
	PlannedSegmentCount         int
	PlannedPrimaryRequestCount  int
	PlannedMaxProviderCallCount int
	SheetWidth                  int
	SheetHeight                 int
	CellWidth                   int
	CellHeight                  int
	FallbackMode                string
	Hash                        string
}

type TaskPlanSnapshot struct {
	SchemaVersion               int
	TaskID                      string
	Provider                    string
	Model                       string
	ConfigID                    int
	ConfigRevision              string
	CapabilitySnapshotJSON      string
	CapabilitySnapshotHash      string
	ReferenceAssetID            string
	CostEstimateJSON            string
	PlannedPrimaryRequestCount  int
	PlannedMaxProviderCallCount int
	ActionPlans                 map[string]GenerationPlanSnapshot
	Hash                        string
}

type Budget struct {
	MaxPrimaryRequests int
	MaxProviderCalls   int
	MaxOutputImages    int
	MaxTotalPixels     int64
	MaxEstimatedAmount *float64
}
