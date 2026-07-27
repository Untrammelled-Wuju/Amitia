package update

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"
)

type DownloadSource struct {
	URL            string
	SHA256         string
	ExpectedSize   int64
	PublisherID    string
	SignatureKeyID string
}

type DownloadResult struct {
	LocalPath    string
	BytesWritten int64
	SHA256       string
	Verified     bool
	Duration     time.Duration
}

type Downloader struct {
	httpClient  *http.Client
	stagingDir  string
	maxFileSize int64
	timeout     time.Duration
}

const (
	defaultDownloadTimeout = 10 * time.Minute
	defaultMaxDownloadSize = 512 * 1024 * 1024
	downloadPartSuffix     = ".part"
)

var (
	ErrDownloadEmptyURL     = errors.New("update: download source url is empty")
	ErrDownloadDirCreate    = errors.New("update: cannot create staging dir")
	ErrDownloadRequest      = errors.New("update: download request failed")
	ErrDownloadUnexpected   = errors.New("update: unexpected download status")
	ErrDownloadSizeExceeded = errors.New("update: download size exceeded limit")
	ErrDownloadSizeMismatch = errors.New("update: downloaded size mismatch")
	ErrDownloadHashMismatch = errors.New("update: downloaded sha256 mismatch")
	ErrDownloadRename       = errors.New("update: cannot rename temp file")
)

func NewDownloader(stagingDir string) *Downloader {
	return &Downloader{
		httpClient: &http.Client{
			Timeout: defaultDownloadTimeout,
			CheckRedirect: func(req *http.Request, via []*http.Request) error {
				if len(via) >= 10 {
					return fmt.Errorf("update: too many redirects")
				}
				return nil
			},
		},
		stagingDir:  stagingDir,
		maxFileSize: defaultMaxDownloadSize,
		timeout:     defaultDownloadTimeout,
	}
}

func (d *Downloader) SetMaxFileSize(size int64) {
	d.maxFileSize = size
}

func (d *Downloader) SetTimeout(timeout time.Duration) {
	d.timeout = timeout
	if timeout > 0 {
		d.httpClient.Timeout = timeout
	}
}

func (d *Downloader) Download(ctx context.Context, source DownloadSource) (*DownloadResult, error) {
	if source.URL == "" {
		return nil, ErrDownloadEmptyURL
	}

	start := time.Now()

	if err := os.MkdirAll(d.stagingDir, 0o755); err != nil {
		return nil, fmt.Errorf("%w: %v", ErrDownloadDirCreate, err)
	}

	finalName := deriveDownloadFilename(source.URL)
	finalPath := filepath.Join(d.stagingDir, finalName)
	partPath := finalPath + downloadPartSuffix

	existingSize := int64(0)
	if info, err := os.Stat(partPath); err == nil && !info.IsDir() {
		existingSize = info.Size()
	}

	if source.ExpectedSize > 0 && existingSize >= source.ExpectedSize {
		if err := os.Truncate(partPath, 0); err != nil {
			return nil, fmt.Errorf("update: reset stale part file failed: %w", err)
		}
		existingSize = 0
	}

	bytesWritten, err := d.streamDownload(ctx, source, partPath, existingSize)
	if err != nil {
		return nil, err
	}

	totalSize := existingSize + bytesWritten

	if source.ExpectedSize > 0 && totalSize != source.ExpectedSize {
		os.Remove(partPath)
		return nil, fmt.Errorf("%w: expected %d, got %d", ErrDownloadSizeMismatch, source.ExpectedSize, totalSize)
	}

	if d.maxFileSize > 0 && totalSize > d.maxFileSize {
		os.Remove(partPath)
		return nil, fmt.Errorf("%w: %d > %d", ErrDownloadSizeExceeded, totalSize, d.maxFileSize)
	}

	hash, err := hashFile(partPath)
	if err != nil {
		os.Remove(partPath)
		return nil, fmt.Errorf("update: hash downloaded file failed: %w", err)
	}

	verified := true
	if source.SHA256 != "" {
		if !strings.EqualFold(hash, source.SHA256) {
			os.Remove(partPath)
			return nil, fmt.Errorf("%w: expected %s, got %s", ErrDownloadHashMismatch, source.SHA256, hash)
		}
	} else {
		verified = false
	}

	if err := os.Rename(partPath, finalPath); err != nil {
		return nil, fmt.Errorf("%w: %v", ErrDownloadRename, err)
	}

	return &DownloadResult{
		LocalPath:    finalPath,
		BytesWritten: totalSize,
		SHA256:       hash,
		Verified:     verified,
		Duration:     time.Since(start),
	}, nil
}

func (d *Downloader) streamDownload(ctx context.Context, source DownloadSource, partPath string, existingSize int64) (int64, error) {
	reqCtx, cancel := context.WithTimeout(ctx, d.timeout)
	defer cancel()

	req, err := http.NewRequestWithContext(reqCtx, http.MethodGet, source.URL, nil)
	if err != nil {
		return 0, fmt.Errorf("%w: %v", ErrDownloadRequest, err)
	}

	if existingSize > 0 {
		req.Header.Set("Range", fmt.Sprintf("bytes=%d-", existingSize))
	}
	req.Header.Set("Accept", "application/octet-stream")
	req.Header.Set("User-Agent", "amitia-update-downloader/0.1")

	resp, err := d.httpClient.Do(req)
	if err != nil {
		return 0, fmt.Errorf("%w: %v", ErrDownloadRequest, err)
	}
	defer resp.Body.Close()

	isResume := resp.StatusCode == http.StatusPartialContent
	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusPartialContent {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 512))
		return 0, fmt.Errorf("%w: %d: %s", ErrDownloadUnexpected, resp.StatusCode, string(body))
	}

	flags := os.O_CREATE | os.O_WRONLY
	if isResume && existingSize > 0 {
		flags |= os.O_APPEND
	} else {
		flags |= os.O_TRUNC
		existingSize = 0
	}

	f, err := os.OpenFile(partPath, flags, 0o644)
	if err != nil {
		return 0, fmt.Errorf("update: open part file failed: %w", err)
	}
	defer f.Close()

	limited := io.LimitReader(resp.Body, d.maxFileSize)
	n, err := io.Copy(f, limited)
	if err != nil {
		return n, fmt.Errorf("update: write part file failed: %w", err)
	}
	return n, nil
}

func deriveDownloadFilename(url string) string {
	base := filepath.Base(url)
	if base == "" || base == "/" || base == "." {
		return fmt.Sprintf("download-%d.bin", time.Now().UnixNano())
	}
	return base
}

func hashFile(path string) (string, error) {
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
