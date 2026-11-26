package capability

import (
	"context"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"sort"
)

const (
	DefaultDetailTopK        = 8
	MaxDetailTopK            = 12
	DefaultSchemaTokenBudget = 2048
	detailCursorVersion      = 1
)

var (
	ErrDetailRequestInvalid  = errors.New("detail request is invalid")
	ErrDetailBudgetExhausted = errors.New("detail schema budget cannot fit a tool")
	ErrDetailCursorInvalid   = errors.New("detail cursor is invalid")
)

func estimateSchemaTokens(body []byte) int {
	return (len(body) + 3) / 4
}

func estimateViewTokens(view ModelToolView) int {
	return estimateSchemaTokens(view.InputSchema) + len(view.Name)/4 + len(view.Description)/4
}

type TrimReport struct {
	OriginalTokens int
	FinalTokens    int
	Required       []string
	Dropped        []string
}

func TrimSchemaToBudget(schema json.RawMessage, budget int) (json.RawMessage, TrimReport, error) {
	report := TrimReport{OriginalTokens: estimateSchemaTokens(schema), FinalTokens: estimateSchemaTokens(schema)}
	if budget <= 0 {
		return nil, report, fmt.Errorf("%w: schema budget must be positive", ErrDetailRequestInvalid)
	}
	if len(schema) == 0 {
		return nil, report, fmt.Errorf("%w: schema body is empty", ErrDetailRequestInvalid)
	}
	if report.OriginalTokens <= budget {
		return schema, report, nil
	}

	var object map[string]json.RawMessage
	if err := json.Unmarshal(schema, &object); err != nil {
		return nil, report, fmt.Errorf("decode schema for budget: %w", err)
	}
	propertiesRaw, ok := object["properties"]
	if !ok {
		return nil, report, fmt.Errorf("%w: schema has no optional properties budget=%d need=%d", ErrDetailBudgetExhausted, budget, report.OriginalTokens)
	}
	var properties map[string]json.RawMessage
	if err := json.Unmarshal(propertiesRaw, &properties); err != nil {
		return nil, report, fmt.Errorf("decode schema properties: %w", err)
	}
	required := []string{}
	if raw, ok := object["required"]; ok {
		if err := json.Unmarshal(raw, &required); err != nil {
			return nil, report, fmt.Errorf("decode schema required: %w", err)
		}
	}
	requiredSet := make(map[string]bool, len(required))
	for _, name := range required {
		requiredSet[name] = true
	}
	optional := make([]string, 0, len(properties))
	for name := range properties {
		if !requiredSet[name] {
			optional = append(optional, name)
		}
	}
	sort.Slice(optional, func(i, j int) bool {
		left := len(properties[optional[i]])
		right := len(properties[optional[j]])
		if left != right {
			return left > right
		}
		return optional[i] < optional[j]
	})

	for _, name := range optional {
		delete(properties, name)
		report.Dropped = append(report.Dropped, name)
		rebuiltProperties, err := json.Marshal(properties)
		if err != nil {
			return nil, report, fmt.Errorf("encode trimmed properties: %w", err)
		}
		object["properties"] = rebuiltProperties
		body, err := json.Marshal(object)
		if err != nil {
			return nil, report, fmt.Errorf("encode trimmed schema: %w", err)
		}
		if estimateSchemaTokens(body) <= budget {
			report.FinalTokens = estimateSchemaTokens(body)
			report.Required = append([]string(nil), required...)
			return body, report, nil
		}
	}
	return nil, report, fmt.Errorf("%w: required=%v need=%d budget=%d", ErrDetailBudgetExhausted, required, report.OriginalTokens, budget)
}

type detailCursor struct {
	Version int    `json:"v"`
	Offset  int    `json:"offset"`
	Digest  string `json:"digest"`
}

func detailDigest(query string, snapshot StrategySnapshot) string {
	sum := sha256.Sum256([]byte(query + "\x00" + snapshot.Version + "\x00" + snapshot.Digest))
	return "sha256:" + hex.EncodeToString(sum[:])
}

func encodeDetailCursor(offset int, digest string) (string, error) {
	body, err := json.Marshal(detailCursor{Version: detailCursorVersion, Offset: offset, Digest: digest})
	if err != nil {
		return "", fmt.Errorf("encode detail cursor: %w", err)
	}
	return base64.RawURLEncoding.EncodeToString(body), nil
}

func decodeDetailCursor(cursor string, digest string) (int, error) {
	body, err := base64.RawURLEncoding.DecodeString(cursor)
	if err != nil {
		return 0, fmt.Errorf("%w: cursor is not base64url: %w", ErrDetailCursorInvalid, err)
	}
	var decoded detailCursor
	if err := json.Unmarshal(body, &decoded); err != nil {
		return 0, fmt.Errorf("%w: cursor body is not json: %w", ErrDetailCursorInvalid, err)
	}
	if decoded.Version != detailCursorVersion {
		return 0, fmt.Errorf("%w: cursor version=%d want %d", ErrDetailCursorInvalid, decoded.Version, detailCursorVersion)
	}
	if decoded.Digest != digest {
		return 0, fmt.Errorf("%w: cursor belongs to another query or strategy", ErrDetailCursorInvalid)
	}
	if decoded.Offset < 0 {
		return 0, fmt.Errorf("%w: cursor offset=%d", ErrDetailCursorInvalid, decoded.Offset)
	}
	return decoded.Offset, nil
}

type DetailRequest struct {
	Query  string
	Limit  int
	Budget int
	Cursor string
}

type DetailPage struct {
	Views       []ModelToolView
	Selected    []string
	UsedTokens  int
	Budget      int
	Trimmed     int
	Dropped     []string
	NextCursor  string
	HasMore     bool
	Strategy    StrategySnapshot
	SchemaReads int64
}

func (p DetailPage) Names() []string {
	names := make([]string, 0, len(p.Views))
	for _, view := range p.Views {
		names = append(names, view.Name)
	}
	return names
}

func (m *ToolExposureManager) WithSchemaSource(index *DescriptorIndex, store *SchemaStore) *ToolExposureManager {
	m.schemaIndex = index
	m.schemaStore = store
	return m
}

func (m *ToolExposureManager) GetModelToolDetails(ctx context.Context, sessionCtx ToolInvocationContext, request DetailRequest) (DetailPage, error) {
	page := DetailPage{Views: []ModelToolView{}, Selected: []string{}, Dropped: []string{}}
	if err := ctx.Err(); err != nil {
		return page, err
	}
	if m.taskSelector == nil {
		return page, fmt.Errorf("%w: detail loading requires a task selector", ErrDetailRequestInvalid)
	}
	limit := request.Limit
	if limit < 0 {
		return page, fmt.Errorf("%w: limit=%d", ErrDetailRequestInvalid, request.Limit)
	}
	if limit == 0 {
		limit = DefaultDetailTopK
	}
	if limit > MaxDetailTopK {
		limit = MaxDetailTopK
	}
	budget := request.Budget
	if budget < 0 {
		return page, fmt.Errorf("%w: budget=%d", ErrDetailRequestInvalid, request.Budget)
	}
	if budget == 0 {
		budget = DefaultSchemaTokenBudget
	}
	page.Budget = budget

	report, err := m.taskSelector.Select(ctx, sessionCtx, request.Query, 0)
	if err != nil {
		return page, fmt.Errorf("select tools for detail: %w", err)
	}
	page.Strategy = report.Strategy
	digest := detailDigest(request.Query, report.Strategy)

	offset := 0
	if request.Cursor != "" {
		offset, err = decodeDetailCursor(request.Cursor, digest)
		if err != nil {
			return page, err
		}
	}
	if offset >= len(report.Selected) {
		if len(report.Selected) == 0 && request.Cursor == "" {
			return page, nil
		}
		return page, fmt.Errorf("%w: offset=%d selected=%d", ErrDetailCursorInvalid, offset, len(report.Selected))
	}

	end := offset + limit
	if end > len(report.Selected) {
		end = len(report.Selected)
	}

	var readsBefore int64
	if m.schemaStore != nil {
		readsBefore = m.schemaStore.Reads()
	}
	used := 0
	for _, selected := range report.Selected[offset:end] {
		if err := ctx.Err(); err != nil {
			return DetailPage{Views: []ModelToolView{}, Selected: []string{}, Dropped: []string{}, Budget: budget, Strategy: report.Strategy}, err
		}
		header := len(selected.Tool.ModelName)/4 + len(selected.Tool.Description)/4
		remaining := budget - used - header
		if remaining <= 0 {
			break
		}
		schema, err := m.detailSchema(selected.Tool)
		if err != nil {
			return page, err
		}
		if estimateSchemaTokens(schema) > remaining {
			trimmed, trimReport, trimErr := TrimSchemaToBudget(schema, remaining)
			if trimErr != nil {
				if errors.Is(trimErr, ErrDetailBudgetExhausted) {
					break
				}
				return page, trimErr
			}
			schema = trimmed
			page.Trimmed++
			page.Dropped = append(page.Dropped, trimReport.Dropped...)
		}
		view := selected.Tool.ModelToolView()
		view.InputSchema = schema
		page.Views = append(page.Views, view)
		page.Selected = append(page.Selected, selected.Tool.ID)
		used += estimateViewTokens(view)
	}
	page.UsedTokens = used
	if m.schemaStore != nil {
		page.SchemaReads = m.schemaStore.Reads() - readsBefore
	}
	if len(page.Views) == 0 {
		return page, fmt.Errorf("%w: budget=%d first_tool=%s", ErrDetailBudgetExhausted, budget, report.Selected[offset].Tool.ID)
	}
	next := offset + len(page.Views)
	if next < len(report.Selected) {
		cursor, err := encodeDetailCursor(next, digest)
		if err != nil {
			return page, err
		}
		page.NextCursor = cursor
		page.HasMore = true
	}
	return page, nil
}

func (m *ToolExposureManager) detailSchema(tool ToolDefinition) (json.RawMessage, error) {
	if m.schemaIndex == nil || m.schemaStore == nil {
		return tool.InputSchema, nil
	}
	entry, ok := m.schemaIndex.Entry(tool.ID)
	if !ok {
		return nil, fmt.Errorf("%w: %s", ErrDescriptorUnknown, tool.ID)
	}
	schema, err := m.schemaStore.Load(entry.SchemaRef, false)
	if err != nil {
		return nil, fmt.Errorf("tool %s: %w", entry.ToolID, err)
	}
	return schema, nil
}
