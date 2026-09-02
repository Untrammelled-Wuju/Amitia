package agent

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"math"
	"strings"
	"time"
)

type androidUIObservationQuality struct {
	Level               string  `json:"level"`
	WindowCount         int     `json:"windowCount"`
	NodeCount           int     `json:"nodeCount"`
	ActionableNodeCount int     `json:"actionableNodeCount"`
	VisibleTextCount    int     `json:"visibleTextCount"`
	EditableNodeCount   int     `json:"editableNodeCount"`
	MaxTreeDepth        int     `json:"maxTreeDepth"`
	StaleRatio          float64 `json:"staleRatio"`
	PackageName         string  `json:"packageName,omitempty"`
	VisualRecommended   bool    `json:"visualRecommended"`
	VisualReason        string  `json:"visualReason,omitempty"`
}

type androidUITreeEnvelope struct {
	SnapshotID string              `json:"snapshotId"`
	Generation int64               `json:"generation"`
	CapturedAt int64               `json:"capturedAt"`
	Windows    []androidUIWindow   `json:"windows"`
	Nodes      []androidUINode     `json:"nodes"`
	Truncated  bool                `json:"truncated"`
	Capability androidUICapability `json:"capability"`
}

type androidUICapability struct {
	Available bool   `json:"available"`
	Degraded  bool   `json:"degraded"`
	Source    string `json:"source"`
	Reason    string `json:"reason"`
}

type androidUIWindow struct {
	WindowID    string `json:"windowId"`
	PackageName string `json:"packageName"`
	Active      bool   `json:"active"`
	Focused     bool   `json:"focused"`
}

type androidUIBounds struct {
	Left   int `json:"left"`
	Top    int `json:"top"`
	Right  int `json:"right"`
	Bottom int `json:"bottom"`
}

type androidUINode struct {
	NodeID             string          `json:"nodeId"`
	ParentID           string          `json:"parentId"`
	WindowID           string          `json:"windowId"`
	ClassName          string          `json:"className"`
	PackageName        string          `json:"packageName"`
	Text               string          `json:"text"`
	ContentDescription string          `json:"contentDescription"`
	ResourceID         string          `json:"resourceId"`
	Role               string          `json:"role"`
	Bounds             androidUIBounds `json:"bounds"`
	VisibleToUser      bool            `json:"visibleToUser"`
	Enabled            bool            `json:"enabled"`
	Clickable          bool            `json:"clickable"`
	LongClickable      bool            `json:"longClickable"`
	Scrollable         bool            `json:"scrollable"`
	Editable           bool            `json:"editable"`
	Depth              int             `json:"depth"`
}

func analyzeAndroidUIObservation(raw json.RawMessage) (androidUIObservationQuality, androidUITreeEnvelope) {
	var tree androidUITreeEnvelope
	_ = json.Unmarshal(raw, &tree)
	q := androidUIObservationQuality{
		WindowCount: len(tree.Windows),
		NodeCount:   len(tree.Nodes),
		Level:       "GOOD",
	}
	now := time.Now().UnixMilli()
	if tree.CapturedAt > 0 && now > tree.CapturedAt+3000 {
		q.Level = "STALE"
	}
	activePackage := ""
	for _, window := range tree.Windows {
		if window.Active || window.Focused {
			activePackage = strings.TrimSpace(window.PackageName)
			if activePackage != "" {
				break
			}
		}
	}
	q.PackageName = activePackage

	suspiciousVisualNodes := 0
	for _, node := range tree.Nodes {
		if node.VisibleToUser && (strings.TrimSpace(node.Text) != "" || strings.TrimSpace(node.ContentDescription) != "") {
			q.VisibleTextCount++
		}
		if node.Enabled && node.VisibleToUser && (node.Clickable || node.LongClickable || node.Scrollable || node.Editable) {
			q.ActionableNodeCount++
		}
		if node.Editable && node.VisibleToUser {
			q.EditableNodeCount++
		}
		if node.Depth > q.MaxTreeDepth {
			q.MaxTreeDepth = node.Depth
		}
		className := strings.ToLower(node.ClassName)
		if strings.Contains(className, "webview") || strings.Contains(className, "surfaceview") || strings.Contains(className, "textureview") || strings.Contains(className, "canvas") {
			suspiciousVisualNodes++
		}
	}

	if q.NodeCount == 0 || q.WindowCount == 0 {
		q.Level = "EMPTY"
	} else if q.Level != "STALE" {
		switch {
		case q.NodeCount <= 3 && q.VisibleTextCount == 0 && q.ActionableNodeCount == 0:
			q.Level = "LOW_INFORMATION"
		case tree.Truncated || tree.Capability.Degraded || q.ActionableNodeCount == 0:
			q.Level = "PARTIAL"
		default:
			q.Level = "GOOD"
		}
	}

	if q.NodeCount > 0 {
		stale := 0
		for _, node := range tree.Nodes {
			if !node.VisibleToUser || strings.TrimSpace(node.NodeID) == "" {
				stale++
			}
		}
		q.StaleRatio = float64(stale) / float64(q.NodeCount)
	}
	q.VisualRecommended = q.Level == "EMPTY" || q.Level == "LOW_INFORMATION" || suspiciousVisualNodes > 0
	switch {
	case suspiciousVisualNodes > 0:
		q.VisualReason = "WebView/SurfaceView/custom-drawn content detected"
	case q.Level == "EMPTY":
		q.VisualReason = "structured UI tree is empty"
	case q.Level == "LOW_INFORMATION":
		q.VisualReason = "structured UI tree has too little actionable information"
	}
	return q, tree
}

func wrapAndroidUIObservation(raw json.RawMessage, quality androidUIObservationQuality) json.RawMessage {
	var tree any
	if err := json.Unmarshal(raw, &tree); err != nil {
		return raw
	}
	wrapped := map[string]any{
		"tree":    tree,
		"quality": quality,
		"visualEscalation": map[string]any{
			"recommended": quality.VisualRecommended,
			"reason":      quality.VisualReason,
		},
	}
	encoded, err := json.Marshal(wrapped)
	if err != nil {
		return raw
	}
	return encoded
}

func androidUIObservationHash(raw json.RawMessage) string {
	sum := sha256.Sum256(raw)
	return hex.EncodeToString(sum[:12])
}

func androidUIActionFingerprint(action plannedAndroidUIAction) string {
	normalized := map[string]any{
		"action":      strings.ToLower(strings.TrimSpace(action.Action)),
		"target":      sanitizeUITarget(action.Target),
		"text":        strings.TrimSpace(action.Text),
		"direction":   strings.ToLower(strings.TrimSpace(action.Direction)),
		"amount":      strings.ToLower(strings.TrimSpace(action.Amount)),
		"packageName": strings.TrimSpace(action.PackageName),
		"description": strings.TrimSpace(action.Description),
		"role":        strings.TrimSpace(action.Role),
		"startX":      action.StartX,
		"startY":      action.StartY,
		"endX":        action.EndX,
		"endY":        action.EndY,
	}
	encoded, _ := json.Marshal(normalized)
	sum := sha256.Sum256(encoded)
	return hex.EncodeToString(sum[:12])
}

func repeatedAndroidUIAction(history []androiduiagentStepView, fingerprint, observationHash string) int {
	count := 0
	for i := len(history) - 1; i >= 0; i-- {
		step := history[i]
		if step.ActionFingerprint != fingerprint {
			break
		}
		if step.AfterHash != "" && observationHash != "" && step.AfterHash != observationHash {
			break
		}
		count++
	}
	return count
}

type androiduiagentStepView struct {
	ActionFingerprint string
	AfterHash         string
}

func semanticRematchTarget(tree androidUITreeEnvelope, original map[string]any) (map[string]any, float64, bool) {
	if len(tree.Nodes) == 0 || len(original) == 0 {
		return nil, 0, false
	}
	oldNodeID, _ := original["nodeId"].(string)
	text := stringValue(original["text"])
	resourceID := stringValue(original["resourceId"])
	role := stringValue(original["role"])
	description := stringValue(original["description"])
	if oldNodeID != "" {
		for _, node := range tree.Nodes {
			if node.NodeID == oldNodeID {
				return targetFromNode(tree.SnapshotID, node), 1, true
			}
		}
	}

	bestScore := 0.0
	var best androidUINode
	for _, node := range tree.Nodes {
		if !node.VisibleToUser || strings.TrimSpace(node.NodeID) == "" {
			continue
		}
		score := 0.0
		if resourceID != "" && strings.EqualFold(resourceID, node.ResourceID) {
			score += 0.55
		}
		if text != "" && normalizedUIString(text) == normalizedUIString(node.Text) {
			score += 0.35
		}
		if description != "" && normalizedUIString(description) == normalizedUIString(node.ContentDescription) {
			score += 0.35
		}
		if role != "" && strings.EqualFold(role, node.Role) {
			score += 0.15
		}
		if node.Enabled {
			score += 0.05
		}
		if score > bestScore {
			bestScore = score
			best = node
		}
	}
	if bestScore < 0.55 {
		return nil, bestScore, false
	}
	return targetFromNode(tree.SnapshotID, best), math.Min(bestScore, 1), true
}

func enrichTargetFromObservation(tree androidUITreeEnvelope, target map[string]any) map[string]any {
	if len(target) == 0 {
		return target
	}
	nodeID := stringValue(target["nodeId"])
	if nodeID == "" {
		return target
	}
	for _, node := range tree.Nodes {
		if node.NodeID != nodeID {
			continue
		}
		out := make(map[string]any, len(target)+5)
		for k, v := range target {
			out[k] = v
		}
		if strings.TrimSpace(node.Text) != "" {
			out["text"] = node.Text
		}
		if strings.TrimSpace(node.ResourceID) != "" {
			out["resourceId"] = node.ResourceID
		}
		if strings.TrimSpace(node.Role) != "" {
			out["role"] = node.Role
		}
		if strings.TrimSpace(node.ContentDescription) != "" {
			out["description"] = node.ContentDescription
		}
		return out
	}
	return target
}

func targetFromNode(snapshotID string, node androidUINode) map[string]any {
	out := map[string]any{"snapshotId": snapshotID, "nodeId": node.NodeID}
	if node.Text != "" {
		out["text"] = node.Text
	}
	if node.ResourceID != "" {
		out["resourceId"] = node.ResourceID
	}
	if node.Role != "" {
		out["role"] = node.Role
	}
	if node.ContentDescription != "" {
		out["description"] = node.ContentDescription
	}
	return out
}

func visualFallbackAction(action plannedAndroidUIAction, enrichedTarget map[string]any) (plannedAndroidUIAction, bool) {
	if action.Action != "click" && action.Action != "long_click" {
		return plannedAndroidUIAction{}, false
	}
	description := firstNonEmpty(
		stringValue(enrichedTarget["description"]),
		stringValue(enrichedTarget["text"]),
		strings.TrimSpace(action.Description),
		strings.TrimSpace(action.Text),
		stringValue(enrichedTarget["resourceId"]),
	)
	if description == "" {
		return plannedAndroidUIAction{}, false
	}
	return plannedAndroidUIAction{
		Action:      "visual_click",
		Reason:      "automatic visual fallback after structured target failure",
		Description: description,
		Text:        firstNonEmpty(stringValue(enrichedTarget["text"]), strings.TrimSpace(action.Text)),
		Role:        firstNonEmpty(stringValue(enrichedTarget["role"]), strings.TrimSpace(action.Role)),
	}, true
}

func stringValue(value any) string {
	if value == nil {
		return ""
	}
	if s, ok := value.(string); ok {
		return strings.TrimSpace(s)
	}
	return strings.TrimSpace(fmt.Sprint(value))
}

func normalizedUIString(value string) string {
	return strings.ToLower(strings.Join(strings.Fields(strings.TrimSpace(value)), " "))
}

func androidUISemanticHash(tree androidUITreeEnvelope) string {
	type semanticNode struct {
		ClassName   string          `json:"className,omitempty"`
		Text        string          `json:"text,omitempty"`
		Description string          `json:"description,omitempty"`
		ResourceID  string          `json:"resourceId,omitempty"`
		Role        string          `json:"role,omitempty"`
		Bounds      androidUIBounds `json:"bounds"`
		Visible     bool            `json:"visible"`
		Enabled     bool            `json:"enabled"`
		Clickable   bool            `json:"clickable"`
		Editable    bool            `json:"editable"`
		Scrollable  bool            `json:"scrollable"`
	}
	nodes := make([]semanticNode, 0, len(tree.Nodes))
	for _, node := range tree.Nodes {
		nodes = append(nodes, semanticNode{
			ClassName: node.ClassName, Text: node.Text, Description: node.ContentDescription,
			ResourceID: node.ResourceID, Role: node.Role, Bounds: node.Bounds,
			Visible: node.VisibleToUser, Enabled: node.Enabled, Clickable: node.Clickable,
			Editable: node.Editable, Scrollable: node.Scrollable,
		})
	}
	packages := make([]string, 0, len(tree.Windows))
	for _, window := range tree.Windows {
		packages = append(packages, window.PackageName)
	}
	encoded, _ := json.Marshal(map[string]any{"packages": packages, "nodes": nodes})
	sum := sha256.Sum256(encoded)
	return hex.EncodeToString(sum[:12])
}
