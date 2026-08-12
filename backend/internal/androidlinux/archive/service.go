//go:build linux && !android

package archive

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/u-ai/backend/internal/androidlinux/fileops"
)

type fileOpsAdapter struct {
	inner fileops.FileService
}

func (a *fileOpsAdapter) Stat(ctx context.Context, path string) (fileops.StatResult, error) {
	return a.inner.Stat(path)
}

func (a *fileOpsAdapter) Read(ctx context.Context, path string, opts fileops.ReadOptions) (fileops.ReadResult, error) {
	return a.inner.Read(path, opts)
}

func (a *fileOpsAdapter) Write(ctx context.Context, path string, data []byte, opts fileops.WriteOptions) (fileops.StatResult, error) {
	return a.inner.Write(path, data, opts)
}

func (a *fileOpsAdapter) List(ctx context.Context, path string, opts fileops.ListOptions) ([]fileops.StatResult, error) {
	return a.inner.List(path, opts)
}

func (a *fileOpsAdapter) MkdirAll(ctx context.Context, path string) error {
	_, err := a.inner.Mkdir(path, fileops.MkdirOptions{Recursive: true})
	return err
}

func (a *fileOpsAdapter) Delete(ctx context.Context, path string, opts fileops.DeleteOptions) error {
	return a.inner.Delete(path, opts)
}

type Service struct {
	files  *fileOpsAdapter
	policy Policy
}

func NewService(files fileops.FileService, policy Policy) *Service {
	return &Service{
		files:  &fileOpsAdapter{inner: files},
		policy: policy,
	}
}

func (s *Service) Detect(ctx context.Context, req DetectRequest) (DetectResult, error) {
	if req.Path == "" {
		return DetectResult{}, ErrInvalidRequest("path is required")
	}

	stat, err := s.files.Stat(ctx, req.Path)
	if err != nil {
		return DetectResult{}, err
	}

	if stat.IsDir {
		return DetectResult{}, ErrNotArchive(req.Path)
	}

	detector := NewDetector()
	return detector.Detect(stat.Path)
}

func (s *Service) List(ctx context.Context, req ListRequest) ([]Entry, int, error) {
	if req.Path == "" {
		return nil, 0, ErrInvalidRequest("path is required")
	}

	stat, err := s.files.Stat(ctx, req.Path)
	if err != nil {
		return nil, 0, err
	}

	if stat.IsDir {
		return nil, 0, ErrNotArchive(req.Path)
	}

	detector := NewDetector()
	detectResult, err := detector.Detect(stat.Path)
	if err != nil {
		return nil, 0, err
	}

	if !detectResult.Archive {
		return nil, 0, ErrNotArchive(req.Path)
	}

	reader, err := s.openArchiveReader(ctx, stat.Path, detectResult.Format)
	if err != nil {
		return nil, 0, err
	}
	defer reader.Close()

	archiveEntries, err := reader.Entries(ctx)
	if err != nil {
		return nil, 0, err
	}

	totalCount := len(archiveEntries)
	limit := s.policy.DefaultListLimit
	if req.Limit > 0 {
		limit = req.Limit
	}
	if limit > s.policy.MaxListLimit {
		limit = s.policy.MaxListLimit
	}
	if limit <= 0 {
		limit = s.policy.DefaultListLimit
	}

	offset := req.Offset
	if offset < 0 {
		offset = 0
	}
	if offset > totalCount {
		offset = totalCount
	}

	end := offset + limit
	if end > totalCount {
		end = totalCount
	}

	var entries []Entry
	for _, ae := range archiveEntries[offset:end] {
		if ae.Type == EntryTypeDirectory && !req.IncludeDirectories {
			continue
		}
		entries = append(entries, archiveEntryToEntry(ae))
	}

	return entries, totalCount, nil
}

type extractState struct {
	service         *Service
	targetRoot      string
	policy          Policy
	overwrite       bool
	stripComponents int
	allowSymlinks   bool
	maxEntries      int
	maxBytes        int64
	maxSingleEntry  int64
	entryCount      int
	totalBytes      int64
}

func (s *Service) Extract(ctx context.Context, req ExtractRequest) (int, int64, error) {
	if req.Path == "" {
		return 0, 0, ErrInvalidRequest("path is required")
	}
	if req.Target == "" {
		return 0, 0, ErrInvalidRequest("target is required")
	}

	srcStat, err := s.files.Stat(ctx, req.Path)
	if err != nil {
		return 0, 0, err
	}

	if srcStat.IsDir {
		return 0, 0, ErrNotArchive(req.Path)
	}

	detector := NewDetector()
	detectResult, err := detector.Detect(srcStat.Path)
	if err != nil {
		return 0, 0, err
	}

	if !detectResult.Archive {
		return 0, 0, ErrNotArchive(req.Path)
	}

	if err := s.files.MkdirAll(ctx, req.Target); err != nil {
		return 0, 0, ErrWriteFailed(err.Error())
	}

	reader, err := s.openArchiveReader(ctx, srcStat.Path, detectResult.Format)
	if err != nil {
		return 0, 0, err
	}
	defer reader.Close()

	sa, ok := reader.(StreamingArchiveReader)
	if !ok {
		return 0, 0, ErrFormatUnsupported("streaming not supported for format: " + string(detectResult.Format))
	}

	maxEntries := s.policy.MaxEntries
	if req.MaxEntries != nil && *req.MaxEntries < maxEntries {
		maxEntries = *req.MaxEntries
	}

	maxBytes := s.policy.MaxTotalUncompressedBytes
	if req.MaxBytes != nil && *req.MaxBytes < maxBytes {
		maxBytes = *req.MaxBytes
	}

	es := &extractState{
		service:        s,
		targetRoot:     req.Target,
		policy:         s.policy,
		overwrite:      req.Overwrite,
		stripComponents: req.StripOptions,
		allowSymlinks:  req.AllowSymlinks,
		maxEntries:     maxEntries,
		maxBytes:       maxBytes,
		maxSingleEntry: s.policy.MaxSingleEntryBytes,
	}

	err = sa.ForEachEntry(ctx, func(ctx context.Context, entry ArchiveEntry, content io.Reader) error {
		return es.processEntry(ctx, entry, content)
	})

	return es.entryCount, es.totalBytes, err
}

type createState struct {
	service        *Service
	writer         ArchiveWriter
	sources        []string
	basePath       string
	includeHidden  bool
	followSymlinks bool
	maxEntries     int
	entryCount     int
	totalBytes     int64
}

func (s *Service) Create(ctx context.Context, req CreateRequest) (int, int64, error) {
	if len(req.Sources) == 0 {
		return 0, 0, ErrInvalidRequest("sources is required")
	}
	if req.Target == "" {
		return 0, 0, ErrInvalidRequest("target is required")
	}
	if len(req.Sources) > s.policy.MaxSources {
		return 0, 0, ErrInvalidRequest("too many sources")
	}

	format := req.Format
	if format == "" {
		format = detectByExtension(req.Target)
	}
	if format == "" {
		return 0, 0, ErrFormatRequired()
	}

	if !isArchiveFormat(format) {
		return 0, 0, ErrFormatUnsupported(string(format))
	}

	writer, err := s.openArchiveWriter(ctx, req.Target, format, req.Overwrite)
	if err != nil {
		return 0, 0, err
	}

	cs := &createState{
		service:       s,
		writer:        writer,
		sources:       req.Sources,
		basePath:      req.BasePath,
		includeHidden: req.IncludeHidden,
		maxEntries:    s.policy.MaxEntries,
	}

	err = cs.run(ctx)
	if err != nil {
		writer.Close()
		return cs.entryCount, cs.totalBytes, err
	}

	if err := writer.Close(); err != nil {
		return cs.entryCount, cs.totalBytes, ErrWriteFailed(err.Error())
	}

	return cs.entryCount, cs.totalBytes, nil
}

func (s *Service) Verify(ctx context.Context, req DetectRequest) (*VerifyResult, error) {
	if req.Path == "" {
		return nil, ErrInvalidRequest("path is required")
	}

	stat, err := s.files.Stat(ctx, req.Path)
	if err != nil {
		return nil, err
	}

	if stat.IsDir {
		return nil, ErrNotArchive(req.Path)
	}

	detector := NewDetector()
	detectResult, err := detector.Detect(stat.Path)
	if err != nil {
		return nil, err
	}

	if !detectResult.Archive {
		return nil, ErrNotArchive(req.Path)
	}

	reader, err := s.openArchiveReader(ctx, stat.Path, detectResult.Format)
	if err != nil {
		return nil, err
	}
	defer reader.Close()

	switch v := reader.(type) {
	case *ZipReader:
		return v.Verify(ctx)
	case *TarReader:
		return v.Verify(ctx)
	default:
		return nil, ErrFormatUnsupported("verify not supported for format: " + string(detectResult.Format))
	}
}

func (s *Service) openArchiveReader(ctx context.Context, path string, format Format) (ArchiveReader, error) {
	switch format {
	case FormatZIP:
		return OpenZipReader(ctx, path, s.policy.MaxArchiveBytes)
	case FormatTAR:
		file, err := os.Open(path)
		if err != nil {
			return nil, ErrReadFailed(err.Error())
		}
		return OpenTarReader(ctx, file, path)
	case FormatTARGZ:
		file, err := os.Open(path)
		if err != nil {
			return nil, ErrReadFailed(err.Error())
		}
		decomp, err := openDecompressor(ctx, file, format)
		if err != nil {
			file.Close()
			return nil, err
		}
		return OpenTarReader(ctx, &wrappedReadCloser{Reader: decomp, close: func() error {
			decomp.Close()
			return file.Close()
		}}, path)
	case FormatTARBZ2:
		file, err := os.Open(path)
		if err != nil {
			return nil, ErrReadFailed(err.Error())
		}
		decomp, err := openDecompressor(ctx, file, format)
		if err != nil {
			file.Close()
			return nil, err
		}
		return OpenTarReader(ctx, &wrappedReadCloser{Reader: decomp, close: func() error {
			decomp.Close()
			return file.Close()
		}}, path)
	default:
		return nil, ErrFormatUnsupported(string(format))
	}
}

func (s *Service) openArchiveWriter(ctx context.Context, path string, format Format, overwrite bool) (ArchiveWriter, error) {
	switch format {
	case FormatZIP:
		return OpenZipWriter(path, overwrite)
	case FormatTAR:
		tmpPath := path + ".amitia.tmp"
		file, err := os.OpenFile(tmpPath, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0644)
		if err != nil {
			return nil, ErrWriteFailed(err.Error())
		}
		return OpenTarWriter(&tempWriteCloser{File: file}), nil
	case FormatTARGZ:
		tmpPath := path + ".amitia.tmp"
		file, err := os.OpenFile(tmpPath, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0644)
		if err != nil {
			return nil, ErrWriteFailed(err.Error())
		}
		gzWriter, _, err := openCompressor(ctx, file, format)
		if err != nil {
			file.Close()
			return nil, err
		}
		return OpenTarWriter(&tempWriteCloser{File: gzWriter}), nil
	default:
		return nil, ErrFormatUnsupported(string(format))
	}
}

type tempWriteCloser struct {
	File io.WriteCloser
}

func (t *tempWriteCloser) Write(p []byte) (int, error) {
	return t.File.Write(p)
}

func (t *tempWriteCloser) Close() error {
	return t.File.Close()
}

func (e *extractState) processEntry(ctx context.Context, entry ArchiveEntry, content io.Reader) error {
	if e.entryCount >= e.maxEntries {
		return ErrTooManyEntries(e.maxEntries)
	}

	if entry.SizeBytes > e.maxSingleEntry {
		return ErrEntryTooLarge(entry.Path, e.maxSingleEntry)
	}

	if e.totalBytes+entry.SizeBytes > e.maxBytes {
		return ErrTooLarge(e.maxBytes)
	}

	if !isSafeEntryPath(entry.Path) {
		return ErrPathEscape(entry.Path)
	}

	if entry.Type == EntryTypeSymlink && !e.allowSymlinks {
		return ErrSymlinkForbidden(entry.Path)
	}

	if entry.Type == EntryTypeHardlink {
		return ErrHardlinkForbidden(entry.Path)
	}

	if entry.Type == EntryTypeOther {
		return ErrSpecialFileForbidden(entry.Path)
	}

	entryPath := entry.Path
	if e.stripComponents > 0 {
		parts := strings.Split(entryPath, "/")
		if len(parts) <= e.stripComponents {
			return nil
		}
		entryPath = strings.Join(parts[e.stripComponents:], "/")
	}

	if entryPath == "" {
		return nil
	}

	fullPath := filepath.Join(e.targetRoot, entryPath)

	if entry.Type == EntryTypeDirectory {
		return e.service.files.MkdirAll(ctx, fullPath)
	}

	if entry.Type == EntryTypeFile {
		data, err := io.ReadAll(io.LimitReader(content, entry.SizeBytes+1))
		if err != nil {
			return ErrReadFailed(err.Error())
		}

		if int64(len(data)) != entry.SizeBytes && entry.SizeBytes > 0 {
			return ErrCorrupt("size mismatch for " + entry.Path)
		}

		_, err = e.service.files.Write(ctx, fullPath, data, fileops.WriteOptions{
			Overwrite:     e.overwrite,
			CreateParents: true,
		})
		if err != nil {
			return ErrWriteFailed(err.Error())
		}

		e.totalBytes += entry.SizeBytes
	}

	e.entryCount++
	return nil
}

func (c *createState) run(ctx context.Context) error {
	for _, source := range c.sources {
		err := c.processSource(ctx, source)
		if err != nil {
			return err
		}
	}
	return nil
}

func (c *createState) processSource(ctx context.Context, source string) error {
	stat, err := c.service.files.Stat(ctx, source)
	if err != nil {
		return ErrSourceNotFound(source)
	}

	if stat.IsDir {
		return c.walkDir(ctx, source, stat.Path)
	}

	return c.addFile(ctx, stat.Path, "")
}

func (c *createState) walkDir(ctx context.Context, source, dirPath string) error {
	entries, err := c.service.files.List(ctx, dirPath, fileops.ListOptions{})
	if err != nil {
		return ErrReadFailed(err.Error())
	}

	for _, entry := range entries {
		fullPath := entry.Path
		relPath := strings.TrimPrefix(fullPath, c.basePath)
		relPath = strings.TrimPrefix(relPath, "/")

		if entry.IsDir {
			if err := c.addDirectory(ctx, relPath); err != nil {
				return err
			}
			if err := c.walkDir(ctx, source, fullPath); err != nil {
				return err
			}
			continue
		}

		if err := c.addFile(ctx, fullPath, relPath); err != nil {
			return err
		}
	}
	return nil
}

func (c *createState) addFile(ctx context.Context, fullPath, relPath string) error {
	if c.entryCount >= c.maxEntries {
		return ErrTooManyEntries(c.maxEntries)
	}

	readResult, err := c.service.files.Read(ctx, fullPath, fileops.ReadOptions{})
	if err != nil {
		return ErrReadFailed(err.Error())
	}

	if relPath == "" {
		relPath = filepath.Base(fullPath)
	}

	archiveEntry := ArchiveEntry{
		Name:       filepath.Base(relPath),
		Path:       relPath,
		Type:       EntryTypeFile,
		SizeBytes:  int64(readResult.BytesRead),
		Mode:       0644,
		ModifiedAt: time.Now().Unix(),
	}

	content := string(readResult.Content)
	if err := c.writer.CreateEntry(ctx, archiveEntry, strings.NewReader(content)); err != nil {
		return ErrWriteFailed(err.Error())
	}

	c.entryCount++
	c.totalBytes += int64(readResult.BytesRead)
	return nil
}

func (c *createState) addDirectory(ctx context.Context, relPath string) error {
	if relPath == "" {
		return nil
	}

	archiveEntry := ArchiveEntry{
		Name:       filepath.Base(relPath),
		Path:       relPath,
		Type:       EntryTypeDirectory,
		Mode:       0755,
		ModifiedAt: time.Now().Unix(),
	}

	return c.writer.CreateEmptyDirectory(ctx, archiveEntry)
}

func archiveEntryToEntry(ae ArchiveEntry) Entry {
	var modTime *time.Time
	if ae.ModifiedAt > 0 {
		t := time.Unix(ae.ModifiedAt, 0)
		modTime = &t
	}

	return Entry{
		Name:       ae.Name,
		Path:       ae.Path,
		Type:       string(ae.Type),
		SizeBytes:  ae.SizeBytes,
		Mode:       formatMode(ae.Mode),
		ModifiedAt: modTime,
		LinkTarget: ae.LinkTarget,
		Encrypted:  ae.Encrypted,
	}
}

func formatMode(mode uint32) string {
	if mode == 0 {
		return ""
	}
	return fmt.Sprintf("%04o", mode&0777)
}
