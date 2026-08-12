package overlay

import (
	"testing"
)

func TestDefaultPolicy(t *testing.T) {
	p := DefaultPolicy()

	if p.MaxActiveOverlays != 4 {
		t.Errorf("expected MaxActiveOverlays=4, got %d", p.MaxActiveOverlays)
	}
	if p.MaxTextBytes != 32*1024 {
		t.Errorf("expected MaxTextBytes=32768, got %d", p.MaxTextBytes)
	}
	if p.MaxActions != 8 {
		t.Errorf("expected MaxActions=8, got %d", p.MaxActions)
	}
	if p.MaxTTL != 24*60*60*1000 {
		t.Errorf("expected MaxTTL=86400000, got %d", p.MaxTTL)
	}
	if p.DefaultFocusable != false {
		t.Errorf("expected DefaultFocusable=false, got %v", p.DefaultFocusable)
	}
	if p.DefaultTouchable != true {
		t.Errorf("expected DefaultTouchable=true, got %v", p.DefaultTouchable)
	}
}

func TestValidateCreateRequestValid(t *testing.T) {
	p := DefaultPolicy()
	req := CreateRequest{
		Kind: OverlayKindText,
		Content: map[string]any{
			"text": "Hello World",
		},
	}

	err := p.ValidateCreateRequest(req, 0)
	if err != nil {
		t.Errorf("expected no error, got %v", err)
	}
}

func TestValidateCreateRequestInvalidKind(t *testing.T) {
	p := DefaultPolicy()
	req := CreateRequest{
		Kind: "invalid_kind",
	}

	err := p.ValidateCreateRequest(req, 0)
	if err == nil {
		t.Error("expected error for invalid kind")
	}
	var oe *overlayError
	if !errorAs(err, &oe) || oe.code != OVERLAY_INVALID_KIND {
		t.Errorf("expected OVERLAY_INVALID_KIND error, got %v", err)
	}
}

func TestValidateCreateRequestLimitReached(t *testing.T) {
	p := DefaultPolicy()
	req := CreateRequest{
		Kind: OverlayKindText,
	}

	err := p.ValidateCreateRequest(req, p.MaxActiveOverlays)
	if err == nil {
		t.Error("expected error when limit reached")
	}
	var oe *overlayError
	if !errorAs(err, &oe) || oe.code != OVERLAY_LIMIT_REACHED {
		t.Errorf("expected OVERLAY_LIMIT_REACHED error, got %v", err)
	}
}

func TestValidateCreateRequestTextTooLarge(t *testing.T) {
	p := DefaultPolicy()
	largeText := make([]byte, p.MaxTextBytes+1)
	req := CreateRequest{
		Kind: OverlayKindText,
		Content: map[string]any{
			"text": string(largeText),
		},
	}

	err := p.ValidateCreateRequest(req, 0)
	if err == nil {
		t.Error("expected error for text too large")
	}
	var oe *overlayError
	if !errorAs(err, &oe) || oe.code != OVERLAY_INVALID_INPUT {
		t.Errorf("expected OVERLAY_INVALID_INPUT error, got %v", err)
	}
}

func TestValidateCreateRequestWidthTooLarge(t *testing.T) {
	p := DefaultPolicy()
	w := p.MaxWidth + 1
	req := CreateRequest{
		Kind:    OverlayKindText,
		Width:   &w,
	}

	err := p.ValidateCreateRequest(req, 0)
	if err == nil {
		t.Error("expected error for width too large")
	}
}

func TestValidateCreateRequestHeightTooLarge(t *testing.T) {
	p := DefaultPolicy()
	h := p.MaxHeight + 1
	req := CreateRequest{
		Kind:    OverlayKindText,
		Height:  &h,
	}

	err := p.ValidateCreateRequest(req, 0)
	if err == nil {
		t.Error("expected error for height too large")
	}
}

func TestValidateCreateRequestFocusableNotAllowed(t *testing.T) {
	p := DefaultPolicy()
	focusable := true
	req := CreateRequest{
		Kind:      OverlayKindText,
		Focusable: &focusable,
	}

	err := p.ValidateCreateRequest(req, 0)
	if err == nil {
		t.Error("expected error for focusable not allowed")
	}
	var oe *overlayError
	if !errorAs(err, &oe) || oe.code != OVERLAY_INVALID_INPUT {
		t.Errorf("expected OVERLAY_INVALID_INPUT error, got %v", err)
	}
}

func TestValidateCreateRequestTTLTooLarge(t *testing.T) {
	p := DefaultPolicy()
	ttl := p.MaxTTL + 1
	req := CreateRequest{
		Kind:  OverlayKindText,
		TTLms: &ttl,
	}

	err := p.ValidateCreateRequest(req, 0)
	if err == nil {
		t.Error("expected error for TTL too large")
	}
}

func TestValidateCreateRequestInvalidResourceURI(t *testing.T) {
	p := DefaultPolicy()
	req := CreateRequest{
		Kind: OverlayKindImage,
		Content: map[string]any{
			"imageUri": "file:///sdcard/test.png",
		},
	}

	err := p.ValidateCreateRequest(req, 0)
	if err == nil {
		t.Error("expected error for invalid resource URI")
	}
	var oe *overlayError
	if !errorAs(err, &oe) || oe.code != OVERLAY_INVALID_RESOURCE {
		t.Errorf("expected OVERLAY_INVALID_RESOURCE error, got %v", err)
	}
}

func TestValidateCreateRequestTooManyActions(t *testing.T) {
	p := DefaultPolicy()
	actions := make([]interface{}, p.MaxActions+1)
	for i := range actions {
		actions[i] = map[string]any{
			"id":     "action_" + string(rune(i)),
			"label":  "Action",
			"action": ActionDismiss,
		}
	}
	req := CreateRequest{
		Kind: OverlayKindCard,
		Content: map[string]any{
			"actions": actions,
		},
	}

	err := p.ValidateCreateRequest(req, 0)
	if err == nil {
		t.Error("expected error for too many actions")
	}
}

func TestValidateCreateRequestCustomKindNotSupported(t *testing.T) {
	p := DefaultPolicy()
	req := CreateRequest{
		Kind: OverlayKindCustom,
	}

	err := p.ValidateCreateRequest(req, 0)
	if err == nil {
		t.Error("expected error for custom kind")
	}
	var oe *overlayError
	if !errorAs(err, &oe) || oe.code != OVERLAY_INVALID_KIND {
		t.Errorf("expected OVERLAY_INVALID_KIND error, got %v", err)
	}
}

func TestValidateUpdateRequestValid(t *testing.T) {
	p := DefaultPolicy()
	req := UpdateRequest{
		OverlayID: "ovl_test123",
		Content: map[string]any{
			"text": "Updated text",
		},
	}

	err := p.ValidateUpdateRequest(req)
	if err != nil {
		t.Errorf("expected no error, got %v", err)
	}
}

func TestValidateUpdateRequestTextTooLarge(t *testing.T) {
	p := DefaultPolicy()
	largeText := make([]byte, p.MaxTextBytes+1)
	req := UpdateRequest{
		OverlayID: "ovl_test123",
		Content: map[string]any{
			"text": string(largeText),
		},
	}

	err := p.ValidateUpdateRequest(req)
	if err == nil {
		t.Error("expected error for text too large")
	}
}

func TestValidateUpdateRequestWidthTooLarge(t *testing.T) {
	p := DefaultPolicy()
	w := p.MaxWidth + 1
	req := UpdateRequest{
		OverlayID: "ovl_test123",
		Width:     &w,
	}

	err := p.ValidateUpdateRequest(req)
	if err == nil {
		t.Error("expected error for width too large")
	}
}

func TestValidateUpdateRequestFocusableNotAllowed(t *testing.T) {
	p := DefaultPolicy()
	focusable := true
	req := UpdateRequest{
		OverlayID: "ovl_test123",
		Focusable: &focusable,
	}

	err := p.ValidateUpdateRequest(req)
	if err == nil {
		t.Error("expected error for focusable not allowed")
	}
}

func TestResolveFocusable(t *testing.T) {
	p := DefaultPolicy()

	if p.ResolveFocusable(nil) != p.DefaultFocusable {
		t.Errorf("expected default focusable")
	}

	f := true
	if p.ResolveFocusable(&f) != false {
		t.Error("expected false when focusable not allowed")
	}

	touch := false
	if p.ResolveTouchable(&touch) != false {
		t.Error("expected false for touchable=false")
	}
}

func TestResolveGravity(t *testing.T) {
	p := DefaultPolicy()

	if p.ResolveGravity("") != GravityBottomRight {
		t.Errorf("expected default gravity bottom_right")
	}
	if p.ResolveGravity(GravityTopLeft) != GravityTopLeft {
		t.Errorf("expected top_left gravity")
	}
	if p.ResolveGravity("invalid") != GravityBottomRight {
		t.Errorf("expected default gravity for invalid value")
	}
}

func TestNormalizeKind(t *testing.T) {
	if NormalizeKind("text") != OverlayKindText {
		t.Errorf("expected text")
	}
	if NormalizeKind("invalid") != OverlayKindText {
		t.Errorf("expected text for invalid kind")
	}
	if NormalizeKind("") != OverlayKindText {
		t.Errorf("expected text for empty kind")
	}
}

func TestNormalizeAction(t *testing.T) {
	if NormalizeAction(ActionDismiss) != ActionDismiss {
		t.Errorf("expected dismiss")
	}
	if NormalizeAction(ActionInvokeTool) != ActionInvokeTool {
		t.Errorf("expected invoke_tool")
	}
	if NormalizeAction("invalid") != ActionDismiss {
		t.Errorf("expected dismiss for invalid action")
	}
}

func TestValidateCreateRequestValidImageResourceURI(t *testing.T) {
	p := DefaultPolicy()
	req := CreateRequest{
		Kind: OverlayKindImage,
		Content: map[string]any{
			"imageUri": "amitia://workspace/images/test.png",
		},
	}

	err := p.ValidateCreateRequest(req, 0)
	if err != nil {
		t.Errorf("expected no error for valid workspace URI, got %v", err)
	}
}

func TestValidateCreateRequestInvalidUTF8(t *testing.T) {
	p := DefaultPolicy()
	req := CreateRequest{
		Kind: OverlayKindText,
		Content: map[string]any{
			"text": "Hello \xff\xfe World",
		},
	}

	err := p.ValidateCreateRequest(req, 0)
	if err == nil {
		t.Error("expected error for invalid UTF-8")
	}
}

func TestValidateUpdateRequestTTLTooLarge(t *testing.T) {
	p := DefaultPolicy()
	ttl := p.MaxTTL + 1
	req := UpdateRequest{
		OverlayID: "ovl_test123",
		TTLms:     &ttl,
	}

	err := p.ValidateUpdateRequest(req)
	if err == nil {
		t.Error("expected error for TTL too large")
	}
}

func TestResolveTouchable(t *testing.T) {
	p := DefaultPolicy()

	if p.ResolveTouchable(nil) != p.DefaultTouchable {
		t.Errorf("expected default touchable")
	}

	tt := true
	if p.ResolveTouchable(&tt) != true {
		t.Error("expected true for touchable=true")
	}

	ff := false
	if p.ResolveTouchable(&ff) != false {
		t.Error("expected false for touchable=false")
	}
}

func TestValidateUpdateRequestHeightTooLarge(t *testing.T) {
	p := DefaultPolicy()
	h := p.MaxHeight + 1
	req := UpdateRequest{
		OverlayID: "ovl_test123",
		Height:    &h,
	}

	err := p.ValidateUpdateRequest(req)
	if err == nil {
		t.Error("expected error for height too large")
	}
}

func TestValidateUpdateRequestTooManyActions(t *testing.T) {
	p := DefaultPolicy()
	actions := make([]interface{}, p.MaxActions+1)
	for i := range actions {
		actions[i] = map[string]any{
			"id":     "action_" + string(rune(i)),
			"label":  "Action",
			"action": ActionDismiss,
		}
	}
	req := UpdateRequest{
		OverlayID: "ovl_test123",
		Content: map[string]any{
			"actions": actions,
		},
	}

	err := p.ValidateUpdateRequest(req)
	if err == nil {
		t.Error("expected error for too many actions")
	}
}

func errorAs(err error, target **overlayError) bool {
	if oe, ok := err.(*overlayError); ok {
		*target = oe
		return true
	}
	return false
}
