package network

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"io"
	"net/http"
	"time"

	"github.com/u-ai/backend/internal/androidlinux/fileops"
)

func performDownload(ctx context.Context, req DownloadRequest, policy Policy, fileWriter FileWriter) (DownloadResult, error) {
	result := DownloadResult{
		Path: req.Target,
	}

	validator := NewEndpointValidator(policy)
	sec, err := validator.ResolveAndClassify(ctx, req.URL)
	if err != nil {
		return result, err
	}

	timeout := policy.DefaultTimeout
	if req.TimeoutMS > 0 {
		d := time.Duration(req.TimeoutMS) * time.Millisecond
		if d <= policy.MaxTimeout {
			timeout = d
		}
	}

	maxBytes := policy.MaxDownloadBytes
	if req.MaxBytes > 0 && req.MaxBytes < maxBytes {
		maxBytes = req.MaxBytes
	}

	httpReq, err := http.NewRequestWithContext(ctx, "GET", sec.URL.String(), nil)
	if err != nil {
		return result, ErrDownloadDenied(err.Error())
	}
	httpReq.Header.Set("User-Agent", policy.UserAgent)
	httpReq.Header.Set("Accept-Encoding", "identity")

	client := buildPinnedHTTPClient(sec, policy, timeout)
	resp, err := client.Do(httpReq)
	if err != nil {
		return result, ErrDownloadDenied(err.Error())
	}
	defer resp.Body.Close()

	result.StatusCode = resp.StatusCode
	result.FinalURL = resp.Request.URL.String()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return result, ErrDownloadDenied("HTTP " + resp.Status)
	}

	if resp.ContentLength > maxBytes {
		return result, ErrDownloadFileSizeLimit(maxBytes)
	}

	hash := sha256.New()
	reader := io.TeeReader(io.LimitReader(resp.Body, maxBytes+1), hash)

	var buf bytes.Buffer
	written, err := io.CopyN(&buf, reader, maxBytes+1)
	if err != nil && err != io.EOF {
		return result, ErrDownloadPartial(err.Error())
	}

	if written > maxBytes {
		return result, ErrDownloadFileSizeLimit(maxBytes)
	}

	data := buf.Bytes()
	result.BytesWritten = int64(len(data))
	result.SHA256 = hex.EncodeToString(hash.Sum(nil))
	result.MIMEType = resp.Header.Get("Content-Type")
	if idx := bytes.IndexByte([]byte(result.MIMEType), ';'); idx >= 0 {
		result.MIMEType = string(result.MIMEType[:idx])
	}

	if fileWriter == nil {
		return result, ErrDownloadDenied("file writer not available")
	}

	writeResult, err := fileWriter.Write(ctx, req.Target, data, fileops.WriteOptions{
		Overwrite:     req.Overwrite,
		CreateParents: true,
	})
	if err != nil {
		return result, ErrDownloadPartial("write failed: " + err.Error())
	}

	result.Path = writeResult.Path
	return result, nil
}
