package workbenchbrowser

import (
	"bytes"
	"context"
	"encoding/base64"
	"errors"
	"time"
	"unicode/utf8"

	"github.com/u-ai/backend/internal/browser"
)

const MaxRemoteImageBytes = 2 << 20

type RemoteReadRequest struct {
	Operation      string `json:"operation"`
	ConversationID string `json:"conversationId"`
	BrowserRef     string `json:"browserRef"`
}

type RemoteReadResult struct {
	Ready       bool                `json:"ready"`
	Observed    bool                `json:"observed"`
	URL         string              `json:"url,omitempty"`
	Title       string              `json:"title,omitempty"`
	State       string              `json:"state,omitempty"`
	Content     string              `json:"content,omitempty"`
	Truncated   bool                `json:"truncated,omitempty"`
	ImageBase64 string              `json:"imageBase64,omitempty"`
	ContentType string              `json:"contentType,omitempty"`
	ErrorCode   RemoteReadErrorCode `json:"errorCode,omitempty"`
	Error       string              `json:"error,omitempty"`
	Retryable   bool                `json:"retryable,omitempty"`
}

type RemoteReadErrorCode string

const (
	RemoteReadInvalidRequest     RemoteReadErrorCode = "invalid_request"
	RemoteReadCapabilityRevoked  RemoteReadErrorCode = "capability_revoked"
	RemoteReadRuntimeUnavailable RemoteReadErrorCode = "runtime_unavailable"
	RemoteReadUnsupported        RemoteReadErrorCode = "unsupported_operation"
	RemoteReadResourceExceeded   RemoteReadErrorCode = "resource_exceeded"
)

type RemoteReadFailure struct {
	Code      RemoteReadErrorCode
	Message   string
	Retryable bool
}

func (e *RemoteReadFailure) Error() string { return e.Message }

func RemoteReadFailureResult(err error) (RemoteReadResult, bool) {
	var failure *RemoteReadFailure
	if !errors.As(err, &failure) {
		return RemoteReadResult{}, false
	}
	return RemoteReadResult{ErrorCode: failure.Code, Error: failure.Message, Retryable: failure.Retryable}, true
}

func (r RemoteReadRequest) Valid() bool {
	if !validIdentifier.MatchString(r.ConversationID) || !validBrowserRef(r.BrowserRef) {
		return false
	}
	switch r.Operation {
	case "state", "dom", "screenshot":
		return true
	default:
		return false
	}
}

func ReadAgentTab(ctx context.Context, provider browser.BrowserProvider, spaceID string, req RemoteReadRequest) (RemoteReadResult, error) {
	if ctx == nil || provider == nil || spaceID == "" || !req.Valid() {
		return RemoteReadResult{}, &RemoteReadFailure{Code: RemoteReadInvalidRequest, Message: "浏览器远程观察参数无效"}
	}
	session, tabID, allowed := ResolveAgentTab(spaceID, req.ConversationID, req.BrowserRef)
	if !allowed {
		return RemoteReadResult{}, &RemoteReadFailure{Code: RemoteReadCapabilityRevoked, Message: "Agent 浏览器观察许可无效或过期"}
	}
	limits, cancel := context.WithTimeout(ctx, 12*time.Second)
	defer cancel()
	tab, err := provider.Tabs().GetTab(limits, session, tabID)
	if err != nil {
		return RemoteReadResult{}, classifyBrowserObservationError(err)
	}
	result := RemoteReadResult{Ready: true, Observed: true, URL: tab.URL, Title: tab.Title, State: string(tab.State)}
	switch req.Operation {
	case "state":
	case "dom":
		snapshot, browseErr := provider.Observe().GetDOMSnapshot(limits, session, tabID, 3)
		if browseErr != nil {
			return RemoteReadResult{}, classifyBrowserObservationError(browseErr)
		}
		result.URL, result.Title = snapshot.URL, snapshot.Title
		result.Content, result.Truncated = boundDOMContent(snapshot.Content)
		result.Truncated = result.Truncated || snapshot.Truncated
	case "screenshot":
		capture, ok := provider.Resources().(browser.BrowserScreenshotBytes)
		if !ok {
			return RemoteReadResult{}, &RemoteReadFailure{Code: RemoteReadUnsupported, Message: "目标设备不支持 Chromium 截图读取"}
		}
		data, browseErr := capture.ScreenshotBytes(limits, browser.BrowserScreenshotRequest{SessionID: session, TabID: tabID, Format: "jpeg", Quality: 58})
		if browseErr != nil {
			return RemoteReadResult{}, classifyBrowserObservationError(browseErr)
		}
		if len(data) == 0 || len(data) > MaxRemoteImageBytes {
			return RemoteReadResult{}, &RemoteReadFailure{Code: RemoteReadResourceExceeded, Message: "远程浏览器截图为空或超过 2 MiB 上限"}
		}
		result.ImageBase64 = base64.StdEncoding.EncodeToString(data)
		result.ContentType = "image/jpeg"
	}
	if _, _, stillAllowed := ResolveAgentTab(spaceID, req.ConversationID, req.BrowserRef); !stillAllowed {
		return RemoteReadResult{}, &RemoteReadFailure{Code: RemoteReadCapabilityRevoked, Message: "Agent 浏览器观察许可已撤销"}
	}
	return result, nil
}

func classifyBrowserObservationError(err *browser.BrowserError) *RemoteReadFailure {
	if err != nil {
		switch err.Code {
		case browser.ErrCodeTabNotFound, browser.ErrCodeSessionNotFound, browser.ErrCodeTabStale, browser.ErrCodeSessionStale, browser.ErrCodeTabCrashed:
			return &RemoteReadFailure{Code: RemoteReadCapabilityRevoked, Message: "Agent 浏览器页面已结束，需要重新取得观察引用: " + err.Error()}
		}
	}
	return &RemoteReadFailure{Code: RemoteReadRuntimeUnavailable, Message: "Agent 浏览器暂时无法读取", Retryable: true}
}

func ValidRemoteReadResult(operation string, reply RemoteReadResult) bool {
	if !reply.Ready || !reply.Observed || reply.ErrorCode != "" || reply.Error != "" || reply.Retryable ||
		len(reply.URL) > 4096 || len(reply.Title) > 1024 || len(reply.State) > 128 ||
		!utf8.ValidString(reply.URL) || !utf8.ValidString(reply.Title) || !utf8.ValidString(reply.State) {
		return false
	}
	switch operation {
	case "state":
		return reply.Content == "" && !reply.Truncated && reply.ImageBase64 == "" && reply.ContentType == ""
	case "dom":
		return len(reply.Content) <= maxBrowserDOMBytes && utf8.ValidString(reply.Content) && reply.ImageBase64 == "" && reply.ContentType == ""
	case "screenshot":
		if reply.Content != "" || reply.Truncated || reply.ContentType != "image/jpeg" || len(reply.ImageBase64) == 0 ||
			len(reply.ImageBase64) > base64.StdEncoding.EncodedLen(MaxRemoteImageBytes) {
			return false
		}
		data, err := base64.StdEncoding.DecodeString(reply.ImageBase64)
		return err == nil && len(data) >= 3 && len(data) <= MaxRemoteImageBytes && bytes.Equal(data[:3], []byte{0xff, 0xd8, 0xff})
	default:
		return false
	}
}

func NormalizeRemoteReadFailure(reply RemoteReadResult) (RemoteReadResult, bool) {
	if reply.Ready || reply.Observed || reply.URL != "" || reply.Title != "" || reply.State != "" || reply.Content != "" || reply.Truncated || reply.ImageBase64 != "" || reply.ContentType != "" || !utf8.ValidString(reply.Error) || len(reply.Error) > 512 {
		return RemoteReadResult{}, false
	}
	var message string
	var retryable bool
	switch reply.ErrorCode {
	case RemoteReadInvalidRequest:
		message = "浏览器远程观察参数无效"
	case RemoteReadCapabilityRevoked:
		message = "Agent 浏览器观察许可无效或过期"
	case RemoteReadRuntimeUnavailable:
		message = "Agent 浏览器暂时无法读取"
		retryable = true
	case RemoteReadUnsupported:
		message = "目标设备不支持请求的浏览器观察能力"
	case RemoteReadResourceExceeded:
		message = "远程浏览器观察结果超过大小限制"
	default:
		return RemoteReadResult{}, false
	}
	if reply.Error == "" || reply.Retryable != retryable {
		return RemoteReadResult{}, false
	}
	return RemoteReadResult{ErrorCode: reply.ErrorCode, Error: message, Retryable: retryable}, true
}
