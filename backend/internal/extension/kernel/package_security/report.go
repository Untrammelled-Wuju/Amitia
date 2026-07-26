package package_security

import "time"

type Severity string

const (
	SeverityInfo     Severity = "info"
	SeverityWarning  Severity = "warning"
	SeverityHigh     Severity = "high"
	SeverityCritical Severity = "critical"
)

type SecurityIssue struct {
	Severity    Severity `json:"severity"`
	Category    string   `json:"category"`
	Path        string   `json:"path,omitempty"`
	Description string   `json:"description"`
	Blocking    bool     `json:"blocking"`
}

type PackageSecurityReport struct {
	ReportID          string                       `json:"report_id"`
	SourceType        string                       `json:"source_type"`
	ArchiveHash       string                       `json:"archive_hash"`
	ContentTreeHash   string                       `json:"content_tree_hash"`
	EntryCount        int                          `json:"entry_count"`
	TotalCompressed   int64                        `json:"total_compressed"`
	TotalUncompressed int64                        `json:"total_uncompressed"`
	CompressionRatio  float64                      `json:"compression_ratio"`
	SignatureResult   SignatureVerificationResult  `json:"signature_result"`
	PublisherTrust    PublisherTrustResult         `json:"publisher_trust"`
	PathIssues        []SecurityIssue              `json:"path_issues"`
	TypeIssues        []SecurityIssue              `json:"type_issues"`
	SizeIssues        []SecurityIssue              `json:"size_issues"`
	PlatformIssues    []SecurityIssue              `json:"platform_issues"`
	Warnings          []SecurityIssue              `json:"warnings"`
	BlockingIssues    []SecurityIssue              `json:"blocking_issues"`
	Passed            bool                         `json:"passed"`
	CreatedAt         time.Time                    `json:"created_at"`
}

func (r *PackageSecurityReport) IsBlocked() bool {
	return len(r.BlockingIssues) > 0
}

func (r *PackageSecurityReport) HasHighRisk() bool {
	for _, issue := range r.BlockingIssues {
		if issue.Severity == SeverityCritical || issue.Severity == SeverityHigh {
			return true
		}
	}
	for _, issue := range r.PathIssues {
		if issue.Severity == SeverityCritical {
			return true
		}
	}
	return false
}

func (r *PackageSecurityReport) AddPathIssue(path string, description string, severity Severity, blocking bool) {
	issue := SecurityIssue{
		Severity:    severity,
		Category:    "path",
		Path:        path,
		Description: description,
		Blocking:    blocking,
	}
	r.PathIssues = append(r.PathIssues, issue)
	if blocking {
		r.BlockingIssues = append(r.BlockingIssues, issue)
		r.Passed = false
	}
}

func (r *PackageSecurityReport) AddTypeIssue(path string, description string, severity Severity, blocking bool) {
	issue := SecurityIssue{
		Severity:    severity,
		Category:    "type",
		Path:        path,
		Description: description,
		Blocking:    blocking,
	}
	r.TypeIssues = append(r.TypeIssues, issue)
	if blocking {
		r.BlockingIssues = append(r.BlockingIssues, issue)
		r.Passed = false
	}
}

func (r *PackageSecurityReport) AddSizeIssue(path string, description string, severity Severity, blocking bool) {
	issue := SecurityIssue{
		Severity:    severity,
		Category:    "size",
		Path:        path,
		Description: description,
		Blocking:    blocking,
	}
	r.SizeIssues = append(r.SizeIssues, issue)
	if blocking {
		r.BlockingIssues = append(r.BlockingIssues, issue)
		r.Passed = false
	}
}

func (r *PackageSecurityReport) AddWarning(description string) {
	r.Warnings = append(r.Warnings, SecurityIssue{
		Severity:    SeverityWarning,
		Description: description,
	})
}
