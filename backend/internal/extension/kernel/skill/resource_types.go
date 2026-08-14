// SPDX-FileCopyrightText: 2026 彭旭
// SPDX-License-Identifier: AGPL-3.0-only
package skill

import (
	"errors"
	"time"
)

const (
	ResourceMimeTextPlain         = "text/plain"
	ResourceMimeTextMarkdown      = "text/markdown"
	ResourceMimeTextCSV           = "text/csv"
	ResourceMimeTextHTML          = "text/html"
	ResourceMimeApplicationJSON   = "application/json"
	ResourceMimeApplicationYAML   = "application/yaml"
	ResourceMimeApplicationXML    = "application/xml"
	ResourceMimeApplicationTOML   = "application/toml"
	ResourceMimeImagePNG          = "image/png"
	ResourceMimeImageJPEG         = "image/jpeg"
	ResourceMimeImageGIF          = "image/gif"
	ResourceMimeImageWebP         = "image/webp"
	ResourceMimeApplicationPDF    = "application/pdf"
	ResourceMimeApplicationOffice = "application/vnd.openxmlformats-officedocument"
)

const (
	MaxResourceIndexEntries   = 1000
	MaxResourceIndexHardLimit = 5000
	MaxReferenceChainDepth    = 4
	DefaultResourceReadsLimit = 20
	HardResourceReadsLimit    = 100
	DefaultTextTokensLimit    = 20000
	HardTextTokensLimit       = 50000
	DefaultMaterializedBytes  = 250 << 20
	HardMaterializedBytes     = 1 << 30
	SoftReferenceSizeLimit    = 128 << 10
	HardReferenceSizeLimit    = 1 << 20
	SoftAssetSizeLimit        = 50 << 20
	HardAssetSizeLimit        = 250 << 20
	DefaultListPageSize       = 50
	HardListPageSize          = 200
	DefaultReadMaxLines       = 200
	HardReadMaxLines          = 1000
	DefaultTokenBudget        = 4000
	HardTokenBudget           = 8000
)

var (
	ErrResourceSkillNotActive         = errors.New("SKILL_RESOURCE_SKILL_NOT_ACTIVE")
	ErrResourceNotAvailable           = errors.New("SKILL_RESOURCE_NOT_AVAILABLE")
	ErrResourceNotFound               = errors.New("SKILL_RESOURCE_NOT_FOUND")
	ErrResourceKindUnsupported        = errors.New("SKILL_RESOURCE_KIND_UNSUPPORTED")
	ErrResourcePathInvalid            = errors.New("SKILL_RESOURCE_PATH_INVALID")
	ErrResourcePathTraversal          = errors.New("SKILL_RESOURCE_PATH_TRAVERSAL")
	ErrResourceSymlinkRejected        = errors.New("SKILL_RESOURCE_SYMLINK_REJECTED")
	ErrResourceIndexIncomplete        = errors.New("SKILL_RESOURCE_INDEX_INCOMPLETE")
	ErrResourceIndexLimitExceeded     = errors.New("SKILL_RESOURCE_INDEX_LIMIT_EXCEEDED")
	ErrResourceArtifactMissing        = errors.New("SKILL_RESOURCE_ARTIFACT_MISSING")
	ErrResourceArtifactHashMismatch   = errors.New("SKILL_RESOURCE_ARTIFACT_HASH_MISMATCH")
	ErrResourceHashMismatch           = errors.New("SKILL_RESOURCE_HASH_MISMATCH")
	ErrResourceContentChanged         = errors.New("SKILL_RESOURCE_CONTENT_CHANGED")
	ErrResourceMimeUnsupported        = errors.New("SKILL_RESOURCE_MIME_UNSUPPORTED")
	ErrResourceEncodingUnsupported    = errors.New("SKILL_RESOURCE_ENCODING_UNSUPPORTED")
	ErrResourceTooLarge               = errors.New("SKILL_RESOURCE_TOO_LARGE")
	ErrResourceReadLimitExceeded      = errors.New("SKILL_RESOURCE_READ_LIMIT_EXCEEDED")
	ErrResourceTokenLimitExceeded     = errors.New("SKILL_RESOURCE_TOKEN_LIMIT_EXCEEDED")
	ErrResourceContextBudgetExceeded  = errors.New("SKILL_RESOURCE_CONTEXT_BUDGET_EXCEEDED")
	ErrResourceReferenceDepthExceeded = errors.New("SKILL_RESOURCE_REFERENCE_DEPTH_EXCEEDED")
	ErrResourceMaterializeUnavailable = errors.New("SKILL_RESOURCE_MATERIALIZE_UNAVAILABLE")
	ErrResourceMaterializeFailed      = errors.New("SKILL_RESOURCE_MATERIALIZE_FAILED")
	ErrResourceDiskSpaceInsufficient  = errors.New("SKILL_RESOURCE_DISK_SPACE_INSUFFICIENT")
	ErrResourcePermissionDenied       = errors.New("SKILL_RESOURCE_PERMISSION_DENIED")
	ErrResourceCancelled              = errors.New("SKILL_RESOURCE_CANCELLED")
	ErrResourceTimeout                = errors.New("SKILL_RESOURCE_TIMEOUT")
)

type SkillResourceRef struct {
	ExtensionID      string
	ArtifactID       string
	SkillContentHash string
	RelativePath     string
	ResourceHash     string
}

type SkillResourceDescriptor struct {
	RelativePath string `json:"relativePath"`
	Kind         string `json:"kind"`
	MIMEType     string `json:"mimeType,omitempty"`
	SizeBytes    int64  `json:"sizeBytes"`
	SHA256       string `json:"sha256"`
	TextLike     bool   `json:"textLike"`
	Executable   bool   `json:"executable"`
	Extension    string `json:"extension,omitempty"`
	Depth        int    `json:"depth"`
	Available    bool   `json:"available"`
}

type SkillResourceFilter struct {
	Kind   string
	Prefix string
	Cursor string
	Limit  int
}

type SkillResourcePage struct {
	Items         []SkillResourceDescriptor `json:"items"`
	NextCursor    string                    `json:"nextCursor,omitempty"`
	IndexComplete bool                      `json:"indexComplete"`
	TotalCount    int                       `json:"totalCount"`
}

type TextReadWindow struct {
	StartLine int `json:"startLine,omitempty"`
	MaxLines  int `json:"maxLines,omitempty"`
}

type SkillTextResourceResult struct {
	Skill         string `json:"skill"`
	Path          string `json:"path"`
	MIMEType      string `json:"mimeType"`
	StartLine     int    `json:"startLine"`
	EndLine       int    `json:"endLine"`
	TotalLines    int    `json:"totalLines,omitempty"`
	Content       string `json:"content"`
	Truncated     bool   `json:"truncated"`
	NextStartLine int    `json:"nextStartLine,omitempty"`
	SHA256        string `json:"sha256"`
}

type MaterializedSkillResource struct {
	Skill       string `json:"skill"`
	Path        string `json:"path"`
	ResourceURI string `json:"resourceUri"`
	MIMEType    string `json:"mimeType"`
	SizeBytes   int64  `json:"sizeBytes"`
	SHA256      string `json:"sha256"`
	LeaseID     string `json:"leaseId,omitempty"`
}

type SkillResourceEvidenceRef struct {
	Skill      string
	ArtifactID string
	Path       string
	SHA256     string
	StartLine  int
	EndLine    int
}

type SkillActivationRef struct {
	ActivationID        string
	ExtensionID         string
	ArtifactID          string
	SkillContentHash    string
	ScopeCharacterID    string
	ScopeConversationID string
	ScopeUserID         string
}

type ResourceReadMetrics struct {
	Skill        string
	ArtifactID   string
	Path         string
	ResourceHash string
	Kind         string
	MIMEType     string
	Operation    string
	Bytes        int64
	Tokens       int
	Result       string
	Duration     time.Duration
}

func (r SkillResourceRef) IsValid() bool {
	return r.ExtensionID != "" && r.RelativePath != "" && r.ResourceHash != ""
}

func (d SkillResourceDescriptor) IsTextLike() bool {
	if !d.TextLike {
		return false
	}
	switch d.MIMEType {
	case ResourceMimeTextPlain, ResourceMimeTextMarkdown, ResourceMimeTextCSV,
		ResourceMimeTextHTML, ResourceMimeApplicationJSON, ResourceMimeApplicationYAML,
		ResourceMimeApplicationXML, ResourceMimeApplicationTOML:
		return true
	}
	return false
}

func (d SkillResourceDescriptor) IsBinary() bool {
	return !d.TextLike
}
