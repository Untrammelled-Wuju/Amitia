package overlay

import (
	"strings"
	"unicode/utf8"

	"github.com/u-ai/backend/pkg/resourceuri"
)

type Policy struct {
	MaxActiveOverlays int

	MaxWidth  int
	MaxHeight int

	MaxTextBytes int

	MaxActions int

	MaxTTL int64

	DefaultTouchable bool
	DefaultFocusable bool

	AllowFocusable bool
	AllowDraggable bool
}

func DefaultPolicy() Policy {
	return Policy{
		MaxActiveOverlays: 4,

		MaxWidth:  1080,
		MaxHeight: 1920,

		MaxTextBytes: 32 * 1024,

		MaxActions: 8,

		MaxTTL: 24 * 60 * 60 * 1000,

		DefaultTouchable: true,
		DefaultFocusable: false,

		AllowFocusable: false,
		AllowDraggable: true,
	}
}

func (p Policy) ValidateCreateRequest(req CreateRequest, activeCount int) error {
	if activeCount >= p.MaxActiveOverlays {
		return newOverlayError(OVERLAY_LIMIT_REACHED, "maximum active overlays reached")
	}

	switch req.Kind {
	case OverlayKindText, OverlayKindImage, OverlayKindCard, OverlayKindStatus:
	case OverlayKindCustom:
		return newOverlayError(OVERLAY_INVALID_KIND, "custom kind not supported in this step")
	default:
		return newOverlayError(OVERLAY_INVALID_KIND, "invalid overlay kind: " + req.Kind)
	}

	if req.Content != nil {
		if text, ok := req.Content["text"].(string); ok {
			if !utf8.ValidString(text) {
				return newOverlayError(OVERLAY_INVALID_INPUT, "invalid utf-8 text content")
			}
			if len(text) > p.MaxTextBytes {
				return newOverlayError(OVERLAY_INVALID_INPUT, "text content exceeds maximum size")
			}
		}

		if imageURI, ok := req.Content["imageUri"].(string); ok && imageURI != "" {
			if !p.isValidResourceURI(imageURI) {
				return newOverlayError(OVERLAY_INVALID_RESOURCE, "invalid image resource URI")
			}
		}

		if actions, ok := req.Content["actions"].([]interface{}); ok {
			if len(actions) > p.MaxActions {
				return newOverlayError(OVERLAY_INVALID_INPUT, "too many overlay actions")
			}
		}
	}

	if req.Width != nil && *req.Width > p.MaxWidth {
		return newOverlayError(OVERLAY_INVALID_INPUT, "overlay width exceeds maximum")
	}

	if req.Height != nil && *req.Height > p.MaxHeight {
		return newOverlayError(OVERLAY_INVALID_INPUT, "overlay height exceeds maximum")
	}

	if req.Focusable != nil && *req.Focusable && !p.AllowFocusable {
		return newOverlayError(OVERLAY_INVALID_INPUT, "focusable overlay not allowed")
	}

	if req.TTLms != nil && *req.TTLms > p.MaxTTL {
		return newOverlayError(OVERLAY_INVALID_INPUT, "TTL exceeds maximum allowed")
	}

	return nil
}

func (p Policy) ValidateUpdateRequest(req UpdateRequest) error {
	if req.Content != nil {
		if text, ok := req.Content["text"].(string); ok {
			if !utf8.ValidString(text) {
				return newOverlayError(OVERLAY_INVALID_INPUT, "invalid utf-8 text content")
			}
			if len(text) > p.MaxTextBytes {
				return newOverlayError(OVERLAY_INVALID_INPUT, "text content exceeds maximum size")
			}
		}

		if imageURI, ok := req.Content["imageUri"].(string); ok && imageURI != "" {
			if !p.isValidResourceURI(imageURI) {
				return newOverlayError(OVERLAY_INVALID_RESOURCE, "invalid image resource URI")
			}
		}

		if actions, ok := req.Content["actions"].([]interface{}); ok {
			if len(actions) > p.MaxActions {
				return newOverlayError(OVERLAY_INVALID_INPUT, "too many overlay actions")
			}
		}
	}

	if req.Width != nil && *req.Width > p.MaxWidth {
		return newOverlayError(OVERLAY_INVALID_INPUT, "overlay width exceeds maximum")
	}

	if req.Height != nil && *req.Height > p.MaxHeight {
		return newOverlayError(OVERLAY_INVALID_INPUT, "overlay height exceeds maximum")
	}

	if req.Focusable != nil && *req.Focusable && !p.AllowFocusable {
		return newOverlayError(OVERLAY_INVALID_INPUT, "focusable overlay not allowed")
	}

	if req.TTLms != nil && *req.TTLms > p.MaxTTL {
		return newOverlayError(OVERLAY_INVALID_INPUT, "TTL exceeds maximum allowed")
	}

	return nil
}

func (p Policy) ResolveFocusable(requested *bool) bool {
	if requested == nil {
		return p.DefaultFocusable
	}
	if *requested && !p.AllowFocusable {
		return p.DefaultFocusable
	}
	return *requested
}

func (p Policy) ResolveTouchable(requested *bool) bool {
	if requested == nil {
		return p.DefaultTouchable
	}
	return *requested
}

func (p Policy) ResolveGravity(requested string) string {
	if requested == "" {
		return GravityBottomRight
	}
	switch requested {
	case GravityTopLeft, GravityTopRight, GravityBottomLeft, GravityBottomRight,
		GravityCenter, GravityTopCenter, GravityBottomCenter:
		return requested
	default:
		return GravityBottomRight
	}
}

func (p Policy) isValidResourceURI(uri string) bool {
	parsed, err := resourceuri.Parse(uri)
	if err != nil {
		return false
	}

	switch parsed.Root() {
	case resourceuri.ResourceRootWorkspace,
		resourceuri.ResourceRootAttachments,
		resourceuri.ResourceRootTemp,
		resourceuri.ResourceRootCache:
		return true
	default:
		return false
	}
}

func NormalizeKind(kind string) string {
	switch strings.ToLower(strings.TrimSpace(kind)) {
	case OverlayKindText, OverlayKindImage, OverlayKindCard, OverlayKindStatus:
		return kind
	default:
		return OverlayKindText
	}
}

func NormalizeAction(action string) string {
	switch action {
	case ActionDismiss, ActionEmitEvent, ActionInvokeTool, ActionOpenApp:
		return action
	default:
		return ActionDismiss
	}
}
