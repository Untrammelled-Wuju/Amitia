package pluginmarket

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"
	"strconv"
	"strings"

	"github.com/u-ai/backend/internal/extension/kernel/capability"
	"github.com/u-ai/backend/internal/extension/kernel/domain"
	"github.com/u-ai/backend/internal/extension/kernel/host_api"
	"github.com/u-ai/backend/internal/extension/kernel/runtime_supervisor"
)

const (
	ExtensionID       = "com.amitia.builtin.plugin-market"
	RuntimeModuleID   = "plugin-market-runtime"
	HandlerName       = "plugin-market.command"
	ToolID            = "com.amitia.builtin.plugin-market/command"
	DefaultCatalogURL = "https://amitia.untrammelled.top/api/v1/market/catalog"
)

type HostCaller interface {
	Call(context.Context, host_api.CallRequest) host_api.CallResult
}

type Handler struct {
	host       HostCaller
	catalogURL string
}

type command struct {
	Action                string            `json:"action"`
	Query                 string            `json:"q,omitempty"`
	Category              string            `json:"category,omitempty"`
	CatalogURL            string            `json:"catalogUrl,omitempty"`
	ExtensionID           string            `json:"extensionId,omitempty"`
	Version               string            `json:"version,omitempty"`
	PackageURI            string            `json:"packageUri,omitempty"`
	SHA256                string            `json:"sha256,omitempty"`
	FileName              string            `json:"fileName,omitempty"`
	Publisher             string            `json:"publisher,omitempty"`
	TrustLevel            string            `json:"trustLevel,omitempty"`
	EntryID               string            `json:"entryId,omitempty"`
	SessionID             string            `json:"sessionId,omitempty"`
	RequiredConfirmations []string          `json:"requiredConfirmations,omitempty"`
	Confirmed             bool              `json:"confirmed,omitempty"`
	Confirmations         map[string]bool   `json:"confirmations,omitempty"`
	Source                map[string]string `json:"source,omitempty"`
}

type catalogResponse struct {
	Code int           `json:"code"`
	Msg  string        `json:"msg"`
	Data catalogResult `json:"data"`
}

type catalogResult struct {
	Categories []catalogCategory `json:"categories"`
	Packages   []catalogPackage  `json:"packages"`
}

type catalogCategory struct {
	ID          uint   `json:"id"`
	Slug        string `json:"slug"`
	Name        string `json:"name"`
	Description string `json:"description"`
	Icon        string `json:"icon"`
	SortOrder   int    `json:"sortOrder"`
}

type catalogPackage struct {
	ID            uint             `json:"id"`
	ExtensionID   string           `json:"extensionId"`
	CategoryID    uint             `json:"categoryId"`
	DisplayName   string           `json:"displayName"`
	Summary       string           `json:"summary"`
	Description   string           `json:"description"`
	PublisherID   string           `json:"publisherId"`
	TrustLevel    string           `json:"trustLevel"`
	SecurityScore int              `json:"securityScore"`
	Status        string           `json:"status"`
	LatestVersion string           `json:"latestVersion"`
	Icon          string           `json:"icon"`
	Tags          []string         `json:"tags"`
	Category      catalogCategory  `json:"category"`
	Versions      []catalogVersion `json:"versions"`
}

type catalogVersion struct {
	ID           uint   `json:"id"`
	Version      string `json:"version"`
	OriginalName string `json:"originalName"`
	Size         int64  `json:"size"`
	SHA256       string `json:"sha256"`
	Status       string `json:"status"`
	ReleaseNotes string `json:"releaseNotes"`
}

type listingView struct {
	ID            uint             `json:"id"`
	ExtensionID   string           `json:"extensionId"`
	DisplayName   string           `json:"displayName"`
	Summary       string           `json:"summary"`
	Description   string           `json:"description"`
	Publisher     string           `json:"publisher"`
	TrustLevel    string           `json:"trustLevel"`
	SecurityScore int              `json:"securityScore"`
	CategoryID    string           `json:"categoryId"`
	CategoryName  string           `json:"categoryName"`
	LatestVersion string           `json:"latestVersion"`
	PackageURI    string           `json:"packageUri"`
	SHA256        string           `json:"sha256"`
	FileName      string           `json:"fileName"`
	Size          int64            `json:"size"`
	Installable   bool             `json:"installable"`
	Versions      []catalogVersion `json:"versions"`
}

func New(host HostCaller, catalogURL string) *Handler {
	catalogURL = strings.TrimSpace(catalogURL)
	if catalogURL == "" {
		catalogURL = DefaultCatalogURL
	}
	return &Handler{host: host, catalogURL: catalogURL}
}

func (h *Handler) Execute(ctx context.Context, invocation capability.ToolInvocationContext, input json.RawMessage) capability.UnifiedToolResult {
	var request command
	if err := json.Unmarshal(input, &request); err != nil {
		return marketFailure(invocation, err)
	}
	var (
		output any
		err    error
	)
	switch strings.TrimSpace(request.Action) {
	case "catalog":
		output, err = h.catalog(ctx, invocation, request)
	case "prepare":
		output, err = h.prepare(ctx, invocation, request)
	case "install":
		output, err = h.install(ctx, invocation, request)
	default:
		err = fmt.Errorf("unsupported plugin market action: %s", request.Action)
	}
	if err != nil {
		return marketFailure(invocation, err)
	}
	encoded, err := json.Marshal(output)
	if err != nil {
		return marketFailure(invocation, err)
	}
	return capability.UnifiedToolResult{
		InvocationID: invocation.InvocationID,
		Status:       capability.ToolResultStatusSuccess,
		Structured:   encoded,
		Content: []capability.ToolContent{{
			Type: capability.ToolContentText,
			Text: string(encoded),
		}},
	}
}

func (h *Handler) catalog(ctx context.Context, invocation capability.ToolInvocationContext, request command) (any, error) {
	catalogURL := strings.TrimSpace(request.CatalogURL)
	if catalogURL == "" {
		catalogURL = h.catalogURL
	}
	parsed, err := url.Parse(catalogURL)
	if err != nil || parsed.Hostname() == "" || (parsed.Scheme != "https" && parsed.Scheme != "http") {
		return nil, fmt.Errorf("invalid plugin catalog URL")
	}
	query := parsed.Query()
	if value := strings.TrimSpace(request.Query); value != "" {
		query.Set("q", value)
	}
	if value := strings.TrimSpace(request.Category); value != "" {
		query.Set("category", value)
	}
	parsed.RawQuery = query.Encode()
	result, err := h.call(ctx, invocation, host_api.MethodNetworkRequest, map[string]any{
		"url": parsed.String(), "method": "GET",
	})
	if err != nil {
		return nil, err
	}
	var network struct {
		Status int             `json:"status"`
		Body   json.RawMessage `json:"body"`
	}
	if err := json.Unmarshal(result, &network); err != nil {
		return nil, err
	}
	if network.Status < 200 || network.Status >= 300 {
		return nil, fmt.Errorf("plugin catalog returned HTTP %d: %s", network.Status, string(network.Body))
	}
	var envelope catalogResponse
	if err := json.Unmarshal(network.Body, &envelope); err != nil {
		return nil, fmt.Errorf("decode plugin catalog: %w", err)
	}
	if envelope.Code != 0 && envelope.Code != 200 {
		return nil, fmt.Errorf("plugin catalog returned error %d: %s", envelope.Code, envelope.Msg)
	}
	categories := make([]map[string]any, 0, len(envelope.Data.Categories))
	for _, category := range envelope.Data.Categories {
		categories = append(categories, map[string]any{
			"id": category.Slug, "name": category.Name, "description": category.Description,
			"icon": category.Icon, "sortOrder": category.SortOrder,
		})
	}
	listings := make([]listingView, 0, len(envelope.Data.Packages))
	for _, item := range envelope.Data.Packages {
		if item.Status != "published" {
			continue
		}
		selected, ok := selectVersion(item, item.LatestVersion)
		if !ok {
			continue
		}
		listings = append(listings, listingView{
			ID: item.ID, ExtensionID: item.ExtensionID, DisplayName: item.DisplayName,
			Summary: item.Summary, Description: item.Description, Publisher: item.PublisherID,
			TrustLevel: item.TrustLevel, SecurityScore: item.SecurityScore,
			CategoryID: item.Category.Slug, CategoryName: item.Category.Name,
			LatestVersion: selected.Version, PackageURI: packageDownloadURL(parsed, item.ID, selected.Version),
			SHA256: selected.SHA256, FileName: selected.OriginalName, Size: selected.Size,
			Installable: selected.SHA256 != "", Versions: item.Versions,
		})
	}
	return map[string]any{
		"source": catalogURL, "categories": categories, "listings": listings,
		"count": len(listings),
	}, nil
}

func (h *Handler) prepare(ctx context.Context, invocation capability.ToolInvocationContext, request command) (any, error) {
	if strings.TrimSpace(request.ExtensionID) == "" || strings.TrimSpace(request.Version) == "" || strings.TrimSpace(request.PackageURI) == "" || strings.TrimSpace(request.SHA256) == "" {
		return nil, fmt.Errorf("extensionId, version, packageUri and sha256 are required")
	}
	source := map[string]string{
		"uri": request.PackageURI, "catalogId": "plugin-market", "sourceType": "market",
		"publisher": request.Publisher, "trustLevel": request.TrustLevel, "entryId": request.EntryID,
	}
	for key, value := range request.Source {
		if strings.TrimSpace(value) != "" {
			source[key] = value
		}
	}
	result, err := h.call(ctx, invocation, host_api.MethodPluginPreview, map[string]any{
		"extensionId": request.ExtensionID, "version": request.Version,
		"packageUri": request.PackageURI, "sha256": request.SHA256,
		"fileName": request.FileName, "source": source,
	})
	if err != nil {
		return nil, err
	}
	var preview map[string]any
	if err := json.Unmarshal(result, &preview); err != nil {
		return nil, fmt.Errorf("decode plugin preview: %w", err)
	}
	preview["summary"] = []map[string]any{
		{"label": "插件", "value": firstNonEmpty(stringValue(preview["extensionId"]), request.ExtensionID)},
		{"label": "版本", "value": firstNonEmpty(stringValue(preview["version"]), request.Version)},
		{"label": "发布者", "value": stringValue(preview["publisher"])},
		{"label": "签名", "value": stringValue(preview["signatureStatus"])},
		{"label": "包信任", "value": stringValue(preview["trustDecision"])},
		{"label": "安全审查", "value": boolText(preview["securityPassed"])},
	}
	confirmationText := listText(preview["requiredConfirmations"])
	riskText := listText(preview["riskFlags"])
	if confirmationText != "" && riskText != "" {
		confirmationText += "；风险标记：" + riskText
	} else if riskText != "" {
		confirmationText = "风险标记：" + riskText
	}
	preview["confirmationsText"] = confirmationText
	preview["issuesText"] = issueText(preview["issues"])
	return preview, nil
}

func (h *Handler) install(ctx context.Context, invocation capability.ToolInvocationContext, request command) (any, error) {
	if !request.Confirmed {
		return nil, fmt.Errorf("explicit user confirmation is required")
	}
	if strings.TrimSpace(request.SessionID) == "" || strings.TrimSpace(request.ExtensionID) == "" || strings.TrimSpace(request.Version) == "" {
		return nil, fmt.Errorf("sessionId, extensionId and version are required")
	}
	confirmations := make(map[string]bool, len(request.RequiredConfirmations))
	for key, value := range request.Confirmations {
		confirmations[key] = value
	}
	for _, item := range request.RequiredConfirmations {
		item = strings.TrimSpace(item)
		if item != "" {
			confirmations[item] = true
		}
	}
	result, err := h.call(ctx, invocation, host_api.MethodPluginInstall, map[string]any{
		"extensionId": request.ExtensionID, "version": request.Version,
		"previewSessionId": request.SessionID, "confirmations": confirmations,
		"idempotencyKey": "official-market:" + request.ExtensionID + ":" + request.Version + ":" + invocation.SpaceID,
	})
	if err != nil {
		return nil, err
	}
	return json.RawMessage(result), nil
}

func (h *Handler) call(ctx context.Context, invocation capability.ToolInvocationContext, method host_api.Method, input any) (json.RawMessage, error) {
	encoded, err := json.Marshal(input)
	if err != nil {
		return nil, err
	}
	result := h.host.Call(ctx, host_api.CallRequest{
		CallID: "plugin-market-" + invocation.InvocationID + "-" + string(method),
		RuntimeIdentity: runtime_supervisor.RuntimeIdentity{
			InstanceID: invocation.SessionID, ExtensionID: domain.ExtensionID(firstNonEmpty(invocation.ExtensionID, ExtensionID)),
			ModuleID: domain.ModuleID(firstNonEmpty(invocation.ModuleID, RuntimeModuleID)), Generation: invocation.Generation,
		},
		Method: method, Version: 1, Input: encoded,
		SpaceID: invocation.SpaceID, CharacterID: invocation.CharacterID, ConversationID: invocation.ConversationID,
		Channel: invocation.Channel, TraceID: invocation.TraceID, InvocationID: invocation.InvocationID,
		ScopeSnapshotID: invocation.ScopeSnapshotID, PermissionSnapshotID: invocation.PermissionSnapshotID,
	})
	if result.Error != nil {
		return nil, fmt.Errorf("%s: %s", result.Error.Code, result.Error.Message)
	}
	return result.Output, nil
}

func selectVersion(item catalogPackage, version string) (catalogVersion, bool) {
	for _, candidate := range item.Versions {
		if candidate.Version == version && (candidate.Status == "published" || candidate.Status == "superseded") {
			return candidate, true
		}
	}
	return catalogVersion{}, false
}

func packageDownloadURL(catalog *url.URL, packageID uint, version string) string {
	if catalog == nil || packageID == 0 || version == "" {
		return ""
	}
	base := *catalog
	base.Path = strings.TrimSuffix(base.Path, "/catalog") + "/packages/" + strconv.FormatUint(uint64(packageID), 10) + "/download"
	base.RawQuery = url.Values{"version": []string{version}}.Encode()
	base.Fragment = ""
	return base.String()
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return strings.TrimSpace(value)
		}
	}
	return ""
}

func stringValue(value any) string {
	if value == nil {
		return ""
	}
	return strings.TrimSpace(fmt.Sprint(value))
}

func boolText(value any) string {
	if result, ok := value.(bool); ok && result {
		return "通过"
	}
	return "未通过"
}

func listText(value any) string {
	items, ok := value.([]any)
	if !ok {
		return ""
	}
	parts := make([]string, 0, len(items))
	for _, item := range items {
		if text := strings.TrimSpace(fmt.Sprint(item)); text != "" {
			parts = append(parts, text)
		}
	}
	return strings.Join(parts, "、")
}

func issueText(value any) string {
	items, ok := value.([]any)
	if !ok {
		return ""
	}
	parts := make([]string, 0, len(items))
	for _, item := range items {
		row, ok := item.(map[string]any)
		if !ok {
			continue
		}
		if text := stringValue(row["message"]); text != "" {
			parts = append(parts, text)
		}
	}
	return strings.Join(parts, "；")
}

func marketFailure(invocation capability.ToolInvocationContext, err error) capability.UnifiedToolResult {
	return capability.UnifiedToolResult{
		InvocationID: invocation.InvocationID,
		Status:       capability.ToolResultStatusFailed,
		Error: &capability.ToolError{
			Code: "PLUGIN_MARKET_FAILED", Message: err.Error(), UserVisible: true,
		},
	}
}
