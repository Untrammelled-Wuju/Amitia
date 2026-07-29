package resource

import "time"

type OrphanKind string

const (
	OrphanKindDatabaseRecord OrphanKind = "database_record"
	OrphanKindFile           OrphanKind = "file"
	OrphanKindSecret         OrphanKind = "secret"
	OrphanKindProcess        OrphanKind = "process"
	OrphanKindConnection     OrphanKind = "connection"
	OrphanKindReference      OrphanKind = "reference"
	OrphanKindCache          OrphanKind = "cache"
	OrphanKindSchedule       OrphanKind = "schedule"
)

type OrphanEntry struct {
	Kind         OrphanKind     `json:"kind"`
	ResourceID   string         `json:"resource_id,omitempty"`
	ResourceType ResourceType   `json:"resource_type,omitempty"`
	Identifier   string         `json:"identifier"`
	Description  string         `json:"description"`
	Risk         string         `json:"risk"`
	CanAutoClean bool           `json:"can_auto_clean"`
	DetectedAt   time.Time      `json:"detected_at"`
	Metadata     map[string]any `json:"metadata,omitempty"`
}

type OrphanReport struct {
	ReportID   string        `json:"report_id"`
	Entries    []OrphanEntry `json:"entries"`
	TotalCount int           `json:"total_count"`
	HighRisk   int           `json:"high_risk"`
	AutoClean  int           `json:"auto_clean"`
	CreatedAt  time.Time     `json:"created_at"`
}

func (r *OrphanReport) HasHighRisk() bool {
	return r.HighRisk > 0
}

func (r *OrphanReport) HasOrphans() bool {
	return r.TotalCount > 0
}
