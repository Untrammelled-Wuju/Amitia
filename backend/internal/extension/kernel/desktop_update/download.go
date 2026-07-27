package desktop_update

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

type DownloadStatus string

const (
	DownloadStatusPending   DownloadStatus = "pending"
	DownloadStatusDownloading DownloadStatus = "downloading"
	DownloadStatusPaused    DownloadStatus = "paused"
	DownloadStatusCompleted DownloadStatus = "completed"
	DownloadStatusFailed    DownloadStatus = "failed"
	DownloadStatusCancelled DownloadStatus = "cancelled"
)

type DownloadState struct {
	OperationID    string
	URL            string
	ExpectedHash   string
	ExpectedSize   int64
	DownloadedSize int64
	Status         DownloadStatus
	TempPath       string
	ETag           string
	LastModified   string
	StartedAt      time.Time
	CompletedAt    *time.Time
	Error          string
}

type DownloadManager struct {
	mu           sync.RWMutex
	downloads    map[string]*DownloadState
	maxRedirects int
	maxSize      int64
	timeout      time.Duration
	baseDir      string
	client       *http.Client
	cancelFuncs  map[string]context.CancelFunc
}

func NewDownloadManager(baseDir string) *DownloadManager {
	return &DownloadManager{
		downloads:    make(map[string]*DownloadState),
		maxRedirects: 3,
		maxSize:      500 * 1024 * 1024,
		timeout:      30 * time.Minute,
		baseDir:      filepath.Join(baseDir, "extensions", "update-downloads"),
		cancelFuncs:  make(map[string]context.CancelFunc),
		client: &http.Client{
			Timeout: 30 * time.Minute,
			CheckRedirect: func(req *http.Request, via []*http.Request) error {
				if len(via) >= 3 {
					return fmt.Errorf("%w: exceeded %d redirects", ErrDownloadRedirect, 3)
				}
				return nil
			},
		},
	}
}

func (dm *DownloadManager) validateURL(rawURL string, allowInternal bool) error {
	parsed, err := url.Parse(rawURL)
	if err != nil {
		return fmt.Errorf("%w: invalid url %s", ErrDownloadFailed, err)
	}

	scheme := strings.ToLower(parsed.Scheme)
	if scheme != "https" {
		if scheme == "file" || scheme == "ftp" {
			return fmt.Errorf("%w: %s protocol forbidden", ErrDownloadProtocolForbidden, scheme)
		}
		if scheme == "http" && !allowInternal {
			return fmt.Errorf("%w: http protocol forbidden, https required", ErrDownloadProtocolForbidden)
		}
		if scheme != "http" {
			return fmt.Errorf("%w: %s protocol forbidden", ErrDownloadProtocolForbidden, scheme)
		}
	}

	host := parsed.Hostname()
	if host == "" {
		return fmt.Errorf("%w: empty host", ErrDownloadFailed)
	}

	if !allowInternal {
		if isLocalhostOrInternal(host) {
			return fmt.Errorf("%w: localhost or internal address forbidden", ErrDownloadFailed)
		}
	}

	return nil
}

func isLocalhostOrInternal(host string) bool {
	lower := strings.ToLower(host)
	if lower == "localhost" || lower == "127.0.0.1" || lower == "::1" || lower == "0.0.0.0" {
		return true
	}
	if strings.HasPrefix(lower, "127.") {
		return true
	}
	if strings.HasPrefix(lower, "10.") {
		return true
	}
	if strings.HasPrefix(lower, "192.168.") {
		return true
	}
	if strings.HasPrefix(lower, "169.254.") {
		return true
	}
	if strings.HasPrefix(lower, "172.") {
		parts := strings.Split(lower, ".")
		if len(parts) >= 2 {
			second := 0
			fmt.Sscanf(parts[1], "%d", &second)
			if second >= 16 && second <= 31 {
				return true
			}
		}
	}
	if ip := net.ParseIP(host); ip != nil {
		if ip.IsLoopback() || ip.IsPrivate() || ip.IsLinkLocalUnicast() || ip.IsUnspecified() {
			return true
		}
	}
	return false
}

func (dm *DownloadManager) StartDownload(ctx context.Context, operationID, rawURL, expectedHash string, expectedSize int64, allowInternal bool) error {
	dm.mu.Lock()
	if existing, ok := dm.downloads[operationID]; ok {
		if existing.Status == DownloadStatusDownloading {
			dm.mu.Unlock()
			return fmt.Errorf("%w: download already in progress for %s", ErrUpdateConflict, operationID)
		}
	}
	dm.mu.Unlock()

	if err := dm.validateURL(rawURL, allowInternal); err != nil {
		return err
	}

	if expectedSize > 0 && expectedSize > dm.maxSize {
		return fmt.Errorf("%w: expected size %d exceeds max %d", ErrDownloadSizeExceeded, expectedSize, dm.maxSize)
	}

	downloadCtx, cancel := context.WithTimeout(ctx, dm.timeout)

	dm.mu.Lock()
	dm.cancelFuncs[operationID] = cancel
	state := &DownloadState{
		OperationID:  operationID,
		URL:          rawURL,
		ExpectedHash: expectedHash,
		ExpectedSize: expectedSize,
		Status:       DownloadStatusDownloading,
		StartedAt:    time.Now().UTC(),
	}
	opDir := filepath.Join(dm.baseDir, operationID)
	state.TempPath = filepath.Join(opDir, "package.tmp")
	dm.downloads[operationID] = state
	dm.mu.Unlock()

	go dm.executeDownload(downloadCtx, operationID, rawURL, state)

	return nil
}

func (dm *DownloadManager) executeDownload(ctx context.Context, operationID, rawURL string, state *DownloadState) {
	dm.failDownload(operationID, dm.doDownload(ctx, operationID, rawURL, state))
}

func (dm *DownloadManager) doDownload(ctx context.Context, operationID, rawURL string, state *DownloadState) error {
	opDir := filepath.Join(dm.baseDir, operationID)
	if err := os.MkdirAll(opDir, 0o755); err != nil {
		return fmt.Errorf("%w: cannot create dir: %v", ErrDownloadFailed, err)
	}

	var existingSize int64
	if info, err := os.Stat(state.TempPath); err == nil {
		existingSize = info.Size()
	}

	req, err := http.NewRequestWithContext(ctx, "GET", rawURL, nil)
	if err != nil {
		return fmt.Errorf("%w: %v", ErrDownloadFailed, err)
	}

	if existingSize > 0 {
		req.Header.Set("Range", fmt.Sprintf("bytes=%d-", existingSize))
		if state.ETag != "" {
			req.Header.Set("If-Range", state.ETag)
		} else if state.LastModified != "" {
			req.Header.Set("If-Range", state.LastModified)
		}
	}

	resp, err := dm.client.Do(req)
	if err != nil {
		if ctx.Err() != nil {
			return fmt.Errorf("%w: %v", ErrDownloadCancelled, ctx.Err())
		}
		return fmt.Errorf("%w: %v", ErrDownloadFailed, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusPartialContent {
		if resp.StatusCode == http.StatusRequestedRangeNotSatisfiable {
			if err := os.Remove(state.TempPath); err != nil && !os.IsNotExist(err) {
				return fmt.Errorf("%w: cannot remove stale temp file: %v", ErrDownloadFailed, err)
			}
			existingSize = 0
			return dm.redownloadFromStart(ctx, operationID, rawURL, state)
		}
		return fmt.Errorf("%w: http status %d", ErrDownloadFailed, resp.StatusCode)
	}

	if resp.Header.Get("ETag") != "" {
		state.ETag = resp.Header.Get("ETag")
	}
	if resp.Header.Get("Last-Modified") != "" {
		state.LastModified = resp.Header.Get("Last-Modified")
	}

	flags := os.O_CREATE | os.O_WRONLY
	if existingSize > 0 && resp.StatusCode == http.StatusPartialContent {
		flags |= os.O_APPEND
	} else {
		flags |= os.O_TRUNC
		existingSize = 0
	}

	f, err := os.OpenFile(state.TempPath, flags, 0o644)
	if err != nil {
		return fmt.Errorf("%w: cannot open temp file: %v", ErrDownloadFailed, err)
	}
	defer f.Close()

	written := existingSize
	buf := make([]byte, 32*1024)
	for {
		select {
		case <-ctx.Done():
			return fmt.Errorf("%w: %v", ErrDownloadCancelled, ctx.Err())
		default:
		}

		n, readErr := resp.Body.Read(buf)
		if n > 0 {
			if _, wErr := f.Write(buf[:n]); wErr != nil {
				return fmt.Errorf("%w: write error: %v", ErrDownloadFailed, wErr)
			}
			written += int64(n)

			dm.mu.Lock()
			if s, ok := dm.downloads[operationID]; ok {
				s.DownloadedSize = written
			}
			dm.mu.Unlock()

			if written > dm.maxSize {
				return fmt.Errorf("%w: downloaded %d exceeds max %d", ErrDownloadSizeExceeded, written, dm.maxSize)
			}
		}

		if readErr == io.EOF {
			break
		}
		if readErr != nil {
			return fmt.Errorf("%w: read error: %v", ErrDownloadFailed, readErr)
		}
	}

	if err := f.Sync(); err != nil {
		return fmt.Errorf("%w: sync error: %v", ErrDownloadFailed, err)
	}

	if state.ExpectedSize > 0 && written != state.ExpectedSize {
		return fmt.Errorf("%w: size mismatch, expected %d got %d", ErrDownloadFailed, state.ExpectedSize, written)
	}

	actualHash, err := computeFileSHA256(state.TempPath)
	if err != nil {
		return fmt.Errorf("%w: hash computation failed: %v", ErrDownloadFailed, err)
	}

	if state.ExpectedHash != "" {
		expectedLower := strings.ToLower(state.ExpectedHash)
		actualLower := strings.ToLower(actualHash)
		if expectedLower != actualLower {
			return fmt.Errorf("%w: expected %s got %s", ErrHashMismatch, state.ExpectedHash, actualHash)
		}
	}

	dm.mu.Lock()
	if s, ok := dm.downloads[operationID]; ok {
		s.Status = DownloadStatusCompleted
		s.DownloadedSize = written
		now := time.Now().UTC()
		s.CompletedAt = &now
	}
	dm.mu.Unlock()

	return nil
}

func (dm *DownloadManager) redownloadFromStart(ctx context.Context, operationID, rawURL string, state *DownloadState) error {
	req, err := http.NewRequestWithContext(ctx, "GET", rawURL, nil)
	if err != nil {
		return fmt.Errorf("%w: %v", ErrDownloadFailed, err)
	}

	resp, err := dm.client.Do(req)
	if err != nil {
		if ctx.Err() != nil {
			return fmt.Errorf("%w: %v", ErrDownloadCancelled, ctx.Err())
		}
		return fmt.Errorf("%w: %v", ErrDownloadFailed, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("%w: http status %d", ErrDownloadFailed, resp.StatusCode)
	}

	f, err := os.Create(state.TempPath)
	if err != nil {
		return fmt.Errorf("%w: cannot create temp file: %v", ErrDownloadFailed, err)
	}
	defer f.Close()

	written, err := io.Copy(f, resp.Body)
	if err != nil {
		if ctx.Err() != nil {
			return fmt.Errorf("%w: %v", ErrDownloadCancelled, ctx.Err())
		}
		return fmt.Errorf("%w: copy error: %v", ErrDownloadFailed, err)
	}

	if state.ExpectedSize > 0 && written != state.ExpectedSize {
		return fmt.Errorf("%w: size mismatch", ErrDownloadFailed)
	}

	actualHash, err := computeFileSHA256(state.TempPath)
	if err != nil {
		return fmt.Errorf("%w: hash computation failed: %v", ErrDownloadFailed, err)
	}

	if state.ExpectedHash != "" {
		if strings.ToLower(state.ExpectedHash) != strings.ToLower(actualHash) {
			return fmt.Errorf("%w: expected %s got %s", ErrHashMismatch, state.ExpectedHash, actualHash)
		}
	}

	dm.mu.Lock()
	if s, ok := dm.downloads[operationID]; ok {
		s.Status = DownloadStatusCompleted
		s.DownloadedSize = written
		now := time.Now().UTC()
		s.CompletedAt = &now
	}
	dm.mu.Unlock()

	return nil
}

func (dm *DownloadManager) failDownload(operationID string, err error) {
	if err == nil {
		return
	}
	dm.mu.Lock()
	defer dm.mu.Unlock()
	if s, ok := dm.downloads[operationID]; ok {
		if s.Status == DownloadStatusCompleted {
			return
		}
		s.Status = DownloadStatusFailed
		s.Error = err.Error()
	}
}

func (dm *DownloadManager) PauseDownload(operationID string) error {
	dm.mu.Lock()
	defer dm.mu.Unlock()
	if cancel, ok := dm.cancelFuncs[operationID]; ok {
		cancel()
	}
	if s, ok := dm.downloads[operationID]; ok {
		if s.Status != DownloadStatusDownloading && s.Status != DownloadStatusPending {
			return fmt.Errorf("%w: cannot pause download in status %s", ErrUpdateConflict, s.Status)
		}
		s.Status = DownloadStatusPaused
		return nil
	}
	return fmt.Errorf("%w: download %s not found", ErrUpdateOperationNotFound, operationID)
}

func (dm *DownloadManager) ResumeDownload(ctx context.Context, operationID string, allowInternal bool) error {
	dm.mu.Lock()
	s, ok := dm.downloads[operationID]
	if !ok {
		dm.mu.Unlock()
		return fmt.Errorf("%w: download %s not found", ErrUpdateOperationNotFound, operationID)
	}
	if s.Status != DownloadStatusPaused && s.Status != DownloadStatusFailed {
		dm.mu.Unlock()
		return fmt.Errorf("%w: cannot resume download in status %s", ErrUpdateConflict, s.Status)
	}
	s.Status = DownloadStatusDownloading
	dm.mu.Unlock()

	downloadCtx, cancel := context.WithTimeout(ctx, dm.timeout)
	dm.mu.Lock()
	dm.cancelFuncs[operationID] = cancel
	dm.mu.Unlock()

	go dm.executeDownload(downloadCtx, operationID, s.URL, s)
	return nil
}

func (dm *DownloadManager) CancelDownload(operationID string) error {
	dm.mu.Lock()
	if cancel, ok := dm.cancelFuncs[operationID]; ok {
		cancel()
		delete(dm.cancelFuncs, operationID)
	}
	s, ok := dm.downloads[operationID]
	if !ok {
		dm.mu.Unlock()
		return fmt.Errorf("%w: download %s not found", ErrUpdateOperationNotFound, operationID)
	}
	s.Status = DownloadStatusCancelled
	tempPath := s.TempPath
	dm.mu.Unlock()

	if tempPath != "" {
		os.Remove(tempPath)
	}
	return nil
}

func (dm *DownloadManager) GetDownloadState(operationID string) (*DownloadState, bool) {
	dm.mu.RLock()
	defer dm.mu.RUnlock()
	s, ok := dm.downloads[operationID]
	if !ok {
		return nil, false
	}
	cp := *s
	return &cp, true
}

func (dm *DownloadManager) ListDownloads() []DownloadState {
	dm.mu.RLock()
	defer dm.mu.RUnlock()
	out := make([]DownloadState, 0, len(dm.downloads))
	for _, s := range dm.downloads {
		out = append(out, *s)
	}
	return out
}

func (dm *DownloadManager) CleanupDownload(operationID string) {
	dm.mu.Lock()
	s, ok := dm.downloads[operationID]
	if ok {
		if s.TempPath != "" {
			os.Remove(s.TempPath)
		}
		opDir := filepath.Join(dm.baseDir, operationID)
		os.RemoveAll(opDir)
		delete(dm.downloads, operationID)
	}
	if cancel, ok := dm.cancelFuncs[operationID]; ok {
		cancel()
		delete(dm.cancelFuncs, operationID)
	}
	dm.mu.Unlock()
}

func (dm *DownloadManager) GetDownloadPath(operationID string) (string, error) {
	dm.mu.RLock()
	defer dm.mu.RUnlock()
	s, ok := dm.downloads[operationID]
	if !ok {
		return "", fmt.Errorf("%w: download %s not found", ErrUpdateOperationNotFound, operationID)
	}
	if s.Status != DownloadStatusCompleted {
		return "", fmt.Errorf("%w: download not completed, status %s", ErrDownloadFailed, s.Status)
	}
	return s.TempPath, nil
}

func computeFileSHA256(path string) (string, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer f.Close()

	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		return "", err
	}
	return hex.EncodeToString(h.Sum(nil)), nil
}
