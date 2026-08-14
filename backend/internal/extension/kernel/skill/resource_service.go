// SPDX-FileCopyrightText: 2026 彭旭
// SPDX-License-Identifier: AGPL-3.0-only
package skill

import (
	"context"
	"fmt"
	"io"
	"strings"
)

type ArtifactResourceReader interface {
	Stat(ctx context.Context, artifactID, relativePath string) (ResourceFileInfo, error)
	Open(ctx context.Context, artifactID, relativePath string) (io.ReadCloser, error)
	ReadAll(ctx context.Context, artifactID, relativePath string) ([]byte, error)
}

type ResourceFileInfo struct {
	Size      int64
	SHA256    string
	MIMEType  string
	TextLike  bool
	Exists    bool
	IsSymlink bool
	IsValid   bool
}

type ResourceMaterializer interface {
	Materialize(ctx context.Context, source ArtifactResourceReader, artifactID, relativePath string, sizeLimit int64) (MaterializedSkillResource, error)
	Cleanup(ctx context.Context, leaseID string) error
}

type SkillResourceService interface {
	List(ctx context.Context, activation SkillActivationRef, filter SkillResourceFilter) (SkillResourcePage, error)
	ReadText(ctx context.Context, activation SkillActivationRef, ref SkillResourceRef, window TextReadWindow) (SkillTextResourceResult, error)
	Materialize(ctx context.Context, activation SkillActivationRef, ref SkillResourceRef) (MaterializedSkillResource, error)
	Lookup(ctx context.Context, activation SkillActivationRef, relativePath string) (SkillResourceDescriptor, error)
}

type ResourceIndexProvider interface {
	GetIndex(ctx context.Context, artifactID string) ([]SkillResourceDescriptor, error)
	GetEntry(ctx context.Context, artifactID, relativePath string) (SkillResourceDescriptor, bool, error)
}

type resourceService struct {
	indexProvider  ResourceIndexProvider
	artifactReader ArtifactResourceReader
	materializer   ResourceMaterializer
	limits         ResourceLimits
}

type ResourceLimits struct {
	MaxIndexEntries      int
	MaxIndexHardLimit    int
	MaxResourceReads     int
	MaxTextTokens        int
	MaxMaterializedBytes int
	SoftTextSizeLimit    int64
	HardTextSizeLimit    int64
	SoftAssetSizeLimit   int64
	HardAssetSizeLimit   int64
	DefaultListPageSize  int
	HardListPageSize     int
	DefaultReadMaxLines  int
	HardReadMaxLines     int
	DefaultTokenBudget   int
	HardTokenBudget      int
}

func DefaultResourceLimits() ResourceLimits {
	return ResourceLimits{
		MaxIndexEntries:      MaxResourceIndexEntries,
		MaxIndexHardLimit:    MaxResourceIndexHardLimit,
		MaxResourceReads:     DefaultResourceReadsLimit,
		MaxTextTokens:        DefaultTextTokensLimit,
		MaxMaterializedBytes: DefaultMaterializedBytes,
		SoftTextSizeLimit:    SoftReferenceSizeLimit,
		HardTextSizeLimit:    HardReferenceSizeLimit,
		SoftAssetSizeLimit:   SoftAssetSizeLimit,
		HardAssetSizeLimit:   HardAssetSizeLimit,
		DefaultListPageSize:  DefaultListPageSize,
		HardListPageSize:     HardListPageSize,
		DefaultReadMaxLines:  DefaultReadMaxLines,
		HardReadMaxLines:     HardReadMaxLines,
		DefaultTokenBudget:   DefaultTokenBudget,
		HardTokenBudget:      HardTokenBudget,
	}
}

func NewSkillResourceService(indexProvider ResourceIndexProvider, artifactReader ArtifactResourceReader, materializer ResourceMaterializer, limits ResourceLimits) SkillResourceService {
	if limits.MaxIndexEntries <= 0 {
		limits = DefaultResourceLimits()
	}
	return &resourceService{
		indexProvider:  indexProvider,
		artifactReader: artifactReader,
		materializer:   materializer,
		limits:         limits,
	}
}

func (s *resourceService) List(ctx context.Context, activation SkillActivationRef, filter SkillResourceFilter) (SkillResourcePage, error) {
	if activation.ExtensionID == "" {
		return SkillResourcePage{}, ErrResourceSkillNotActive
	}
	index, err := s.indexProvider.GetIndex(ctx, activation.ArtifactID)
	if err != nil {
		return SkillResourcePage{}, ErrResourceArtifactMissing
	}
	if len(index) > s.limits.MaxIndexHardLimit {
		return SkillResourcePage{IndexComplete: false, TotalCount: len(index)}, ErrResourceIndexLimitExceeded
	}
	filtered := filterIndex(index, filter)
	pageSize := s.limits.DefaultListPageSize
	if filter.Limit > 0 {
		pageSize = filter.Limit
	}
	if pageSize > s.limits.HardListPageSize {
		pageSize = s.limits.HardListPageSize
	}
	start := 0
	if filter.Cursor != "" {
		start = decodeCursor(filter.Cursor)
	}
	if start < 0 {
		start = 0
	}
	end := start + pageSize
	if end > len(filtered) {
		end = len(filtered)
	}
	items := filtered[start:end]
	nextCursor := ""
	if end < len(filtered) {
		nextCursor = encodeCursor(end)
	}
	indexComplete := len(index) <= s.limits.MaxIndexEntries
	return SkillResourcePage{
		Items:         items,
		NextCursor:    nextCursor,
		IndexComplete: indexComplete,
		TotalCount:    len(index),
	}, nil
}

func (s *resourceService) ReadText(ctx context.Context, activation SkillActivationRef, ref SkillResourceRef, window TextReadWindow) (SkillTextResourceResult, error) {
	if activation.ExtensionID == "" {
		return SkillTextResourceResult{}, ErrResourceSkillNotActive
	}
	if err := ValidateResourceRelativePath(ref.RelativePath); err != nil {
		return SkillTextResourceResult{}, err
	}
	desc, found, err := s.indexProvider.GetEntry(ctx, activation.ArtifactID, ref.RelativePath)
	if err != nil {
		return SkillTextResourceResult{}, ErrResourceArtifactMissing
	}
	if !found {
		return SkillTextResourceResult{}, ErrResourceNotFound
	}
	if !desc.Available {
		return SkillTextResourceResult{}, ErrResourceNotAvailable
	}
	if !desc.IsTextLike() {
		return SkillTextResourceResult{}, ErrResourceMimeUnsupported
	}
	if desc.SizeBytes > s.limits.HardTextSizeLimit {
		return SkillTextResourceResult{}, ErrResourceTooLarge
	}
	if ref.ResourceHash != "" && ref.ResourceHash != desc.SHA256 {
		return SkillTextResourceResult{}, ErrResourceHashMismatch
	}
	data, err := s.artifactReader.ReadAll(ctx, activation.ArtifactID, ref.RelativePath)
	if err != nil {
		return SkillTextResourceResult{}, ErrResourceReadLimitExceeded
	}
	if !IsValidUTF8Text(data) {
		return SkillTextResourceResult{}, ErrResourceEncodingUnsupported
	}
	data = StripBOM(data)
	content := string(data)
	lines := splitLines(content)
	totalLines := len(lines)
	startLine := window.StartLine
	if startLine < 0 {
		startLine = 0
	}
	maxLines := s.limits.DefaultReadMaxLines
	if window.MaxLines > 0 {
		maxLines = window.MaxLines
	}
	if maxLines > s.limits.HardReadMaxLines {
		maxLines = s.limits.HardReadMaxLines
	}
	endLine := startLine + maxLines
	truncated := false
	if endLine < totalLines {
		truncated = true
	} else {
		endLine = totalLines
	}
	if startLine > totalLines {
		startLine = totalLines
	}
	selectedLines := lines[startLine:endLine]
	result := strings.Join(selectedLines, "\n")
	tokens := estimateTokenCount(result)
	if tokens > s.limits.HardTokenBudget {
		result = truncateToTokens(result, s.limits.DefaultTokenBudget)
		truncated = true
	}
	nextStartLine := 0
	if truncated && endLine < totalLines {
		nextStartLine = endLine
	}
	return SkillTextResourceResult{
		Skill:         activation.ExtensionID,
		Path:          ref.RelativePath,
		MIMEType:      desc.MIMEType,
		StartLine:     startLine + 1,
		EndLine:       endLine,
		TotalLines:    totalLines,
		Content:       result,
		Truncated:     truncated,
		NextStartLine: nextStartLine,
		SHA256:        desc.SHA256,
	}, nil
}

func (s *resourceService) Materialize(ctx context.Context, activation SkillActivationRef, ref SkillResourceRef) (MaterializedSkillResource, error) {
	if activation.ExtensionID == "" {
		return MaterializedSkillResource{}, ErrResourceSkillNotActive
	}
	if err := ValidateResourceRelativePath(ref.RelativePath); err != nil {
		return MaterializedSkillResource{}, err
	}
	desc, found, err := s.indexProvider.GetEntry(ctx, activation.ArtifactID, ref.RelativePath)
	if err != nil {
		return MaterializedSkillResource{}, ErrResourceArtifactMissing
	}
	if !found {
		return MaterializedSkillResource{}, ErrResourceNotFound
	}
	if !desc.Available {
		return MaterializedSkillResource{}, ErrResourceNotAvailable
	}
	if desc.Executable {
		return MaterializedSkillResource{}, ErrResourceMaterializeUnavailable
	}
	sizeLimit := s.limits.HardAssetSizeLimit
	if desc.Kind == ResourceKindReference {
		sizeLimit = s.limits.HardTextSizeLimit * 2
	}
	if desc.SizeBytes > sizeLimit {
		return MaterializedSkillResource{}, ErrResourceTooLarge
	}
	if s.materializer == nil {
		return MaterializedSkillResource{}, ErrResourceMaterializeUnavailable
	}
	if ref.ResourceHash != "" && ref.ResourceHash != desc.SHA256 {
		return MaterializedSkillResource{}, ErrResourceHashMismatch
	}
	return s.materializer.Materialize(ctx, s.artifactReader, activation.ArtifactID, ref.RelativePath, sizeLimit)
}

func (s *resourceService) Lookup(ctx context.Context, activation SkillActivationRef, relativePath string) (SkillResourceDescriptor, error) {
	if activation.ExtensionID == "" {
		return SkillResourceDescriptor{}, ErrResourceSkillNotActive
	}
	if err := ValidateResourceRelativePath(relativePath); err != nil {
		return SkillResourceDescriptor{}, err
	}
	desc, found, err := s.indexProvider.GetEntry(ctx, activation.ArtifactID, relativePath)
	if err != nil {
		return SkillResourceDescriptor{}, ErrResourceArtifactMissing
	}
	if !found {
		return SkillResourceDescriptor{}, ErrResourceNotFound
	}
	return desc, nil
}

func filterIndex(index []SkillResourceDescriptor, filter SkillResourceFilter) []SkillResourceDescriptor {
	var result []SkillResourceDescriptor
	kindFilter := filter.Kind
	prefixFilter := filter.Prefix
	for _, item := range index {
		if kindFilter != "" && item.Kind != kindFilter {
			continue
		}
		if kindFilter == "" && item.Kind == ResourceKindOther {
			continue
		}
		if prefixFilter != "" && !strings.HasPrefix(item.RelativePath, prefixFilter) {
			continue
		}
		result = append(result, item)
	}
	return result
}

func decodeCursor(cursor string) int {
	var n int
	_, err := fmt.Sscanf(cursor, "%d", &n)
	if err != nil {
		return 0
	}
	return n
}

func encodeCursor(offset int) string {
	return fmt.Sprintf("%d", offset)
}

func splitLines(s string) []string {
	if s == "" {
		return []string{}
	}
	lines := strings.Split(s, "\n")
	if len(lines) > 0 && lines[len(lines)-1] == "" {
		lines = lines[:len(lines)-1]
	}
	return lines
}

func estimateTokenCount(text string) int {
	if text == "" {
		return 0
	}
	return len(text) / 4
}

func truncateToTokens(text string, maxTokens int) string {
	maxBytes := maxTokens * 4
	if len(text) <= maxBytes {
		return text
	}
	truncated := text[:maxBytes]
	lastNewline := strings.LastIndex(truncated, "\n")
	if lastNewline > maxBytes/2 {
		truncated = truncated[:lastNewline]
	}
	return truncated
}
