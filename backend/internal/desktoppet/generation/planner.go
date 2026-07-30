package generation

import (
	"encoding/json"
	"fmt"

	"github.com/u-ai/backend/internal/desktoppet/generationlayout"
	"github.com/u-ai/backend/internal/desktoppet/generationprompt"
	"github.com/u-ai/backend/internal/imageprovider"
)

const (
	FallbackModeNone    = ""
	FallbackModeShrink  = "shrink"
	FallbackModeSplit   = "split"
	FallbackModeDegrade = "degrade"
)

const (
	SeedPolicyFixed         = "fixed"
	SeedPolicyRandom        = "random"
	SeedPolicyDeterministic = "deterministic"
)

const (
	PlanSchemaVersion     = 1
	TaskPlanSchemaVersion = 1
	MaxRetriesPerSegment  = 1
)

var modeFallbackChain = []string{
	string(imageprovider.ModeSpriteSheet),
	string(imageprovider.ModeKeyframe),
	string(imageprovider.ModeSingleFrame),
	string(imageprovider.ModeLegacyFrame),
}

type ActionGenerationPlanner struct {
	actionKey        string
	frameCount       int
	mode             string
	provider         string
	model            string
	capabilities     imageprovider.ProviderCapabilities
	capabilityHash   string
	layoutResult     *generationlayout.LayoutResult
	promptSnapshot   *generationprompt.PromptSnapshot
	seedPolicy       string
	outputCount      int
	budget           *Budget
	configID         int
	configRevision   string
	referenceAssetID string
	seedValue        *int64
}

func NewActionGenerationPlanner(actionKey string, frameCount int) *ActionGenerationPlanner {
	return &ActionGenerationPlanner{
		actionKey:   actionKey,
		frameCount:  frameCount,
		mode:        string(imageprovider.ModeSpriteSheet),
		outputCount: 1,
		seedPolicy:  SeedPolicyRandom,
	}
}

func (p *ActionGenerationPlanner) WithMode(mode string) *ActionGenerationPlanner {
	if mode != "" {
		p.mode = mode
	}
	return p
}

func (p *ActionGenerationPlanner) WithProvider(provider string) *ActionGenerationPlanner {
	p.provider = provider
	return p
}

func (p *ActionGenerationPlanner) WithModel(model string) *ActionGenerationPlanner {
	p.model = model
	return p
}

func (p *ActionGenerationPlanner) WithCapabilities(caps imageprovider.ProviderCapabilities) *ActionGenerationPlanner {
	p.capabilities = caps
	return p
}

func (p *ActionGenerationPlanner) WithCapabilityHash(hash string) *ActionGenerationPlanner {
	p.capabilityHash = hash
	return p
}

func (p *ActionGenerationPlanner) WithLayoutResult(result *generationlayout.LayoutResult) *ActionGenerationPlanner {
	p.layoutResult = result
	return p
}

func (p *ActionGenerationPlanner) WithPromptSnapshot(snapshot *generationprompt.PromptSnapshot) *ActionGenerationPlanner {
	p.promptSnapshot = snapshot
	return p
}

func (p *ActionGenerationPlanner) WithSeedPolicy(policy string) *ActionGenerationPlanner {
	if policy != "" {
		p.seedPolicy = policy
	}
	return p
}

func (p *ActionGenerationPlanner) WithSeedValue(seed *int64) *ActionGenerationPlanner {
	p.seedValue = seed
	return p
}

func (p *ActionGenerationPlanner) WithOutputCount(count int) *ActionGenerationPlanner {
	if count > 0 {
		p.outputCount = count
	}
	return p
}

func (p *ActionGenerationPlanner) WithBudget(budget *Budget) *ActionGenerationPlanner {
	p.budget = budget
	return p
}

func (p *ActionGenerationPlanner) WithConfig(id int, revision string) *ActionGenerationPlanner {
	p.configID = id
	p.configRevision = revision
	return p
}

func (p *ActionGenerationPlanner) WithReferenceAsset(id string) *ActionGenerationPlanner {
	p.referenceAssetID = id
	return p
}

func (p *ActionGenerationPlanner) Plan() (*GenerationPlanSnapshot, error) {
	if p.layoutResult == nil {
		return nil, NewGenerationError(ErrCodePlanLayoutMissing, "layout result is nil", nil)
	}
	if p.promptSnapshot == nil {
		return nil, NewGenerationError(ErrCodePlanPromptMissing, "prompt snapshot is nil", nil)
	}

	mode := p.mode
	if mode == "" {
		mode = string(imageprovider.ModeSpriteSheet)
	}

	fallbackMode := FallbackModeNone

	resolvedMode, fbMode, err := p.resolveMode(mode)
	if err != nil {
		return nil, err
	}
	if resolvedMode != mode {
		fallbackMode = fbMode
		mode = resolvedMode
	}

	if p.referenceAssetID != "" && !p.capabilities.SupportsReferenceImage {
		return nil, NewGenerationError(ErrCodePlanReferenceNotSupported,
			fmt.Sprintf("provider %s does not support reference images", p.provider), nil)
	}

	sheetWidth := p.layoutResult.SheetWidth
	sheetHeight := p.layoutResult.SheetHeight
	cellWidth := p.layoutResult.CellWidth
	cellHeight := p.layoutResult.CellHeight

	if !p.capabilities.SupportsSize(sheetWidth, sheetHeight) {
		if cellWidth > 0 && cellHeight > 0 && p.capabilities.SupportsSize(cellWidth, cellHeight) {
			fallbackMode = FallbackModeSplit
			sheetWidth = cellWidth
			sheetHeight = cellHeight
		} else {
			return nil, NewGenerationError(ErrCodePlanDimensionNotSupported,
				fmt.Sprintf("provider does not support sheet %dx%d or cell %dx%d",
					p.layoutResult.SheetWidth, p.layoutResult.SheetHeight, cellWidth, cellHeight), nil)
		}
	}

	outputCount := p.capabilities.ClampOutputCount(p.outputCount)

	segmentCount := p.layoutResult.TotalSegments
	if segmentCount <= 0 {
		segmentCount = 1
	}

	primaryRequests := p.calculatePrimaryRequests(mode, segmentCount)
	maxProviderCalls := p.calculateMaxProviderCalls(primaryRequests)

	if p.budget != nil {
		if err := p.checkBudget(primaryRequests, maxProviderCalls, outputCount, sheetWidth, sheetHeight); err != nil {
			return nil, err
		}
	}

	layoutJSON, err := json.Marshal(p.layoutResult)
	if err != nil {
		return nil, NewGenerationError(ErrCodePlanInvalid, "failed to marshal layout", err)
	}

	plan := &GenerationPlanSnapshot{
		SchemaVersion:               PlanSchemaVersion,
		ActionKey:                   p.actionKey,
		Mode:                        mode,
		Provider:                    p.provider,
		Model:                       p.model,
		ConfigID:                    p.configID,
		ConfigRevision:              p.configRevision,
		CapabilityHash:              p.capabilityHash,
		ReferenceAssetID:            p.referenceAssetID,
		LayoutJSON:                  string(layoutJSON),
		LayoutHash:                  computeSHA256Hex(string(layoutJSON)),
		PromptTemplateVersion:       p.promptSnapshot.TemplateVersion,
		PromptDocumentJSON:          p.promptSnapshot.DocumentJSON,
		PromptSnapshot:              p.promptSnapshot.FinalPrompt,
		PromptHash:                  p.promptSnapshot.PromptHash,
		NegativePromptSnapshot:      p.promptSnapshot.NegativePrompt,
		NegativePromptHash:          p.promptSnapshot.NegativePromptHash,
		SeedPolicy:                  p.seedPolicy,
		OutputCount:                 outputCount,
		TargetFrameCount:            p.frameCount,
		PlannedSegmentCount:         segmentCount,
		PlannedPrimaryRequestCount:  primaryRequests,
		PlannedMaxProviderCallCount: maxProviderCalls,
		SheetWidth:                  sheetWidth,
		SheetHeight:                 sheetHeight,
		CellWidth:                   cellWidth,
		CellHeight:                  cellHeight,
		FallbackMode:                fallbackMode,
	}

	plan.SeedValue = p.resolveSeedValue(plan)

	plan.Hash = computePlanHash(plan)

	return plan, nil
}

func (p *ActionGenerationPlanner) resolveMode(requestedMode string) (string, string, error) {
	if p.capabilities.SupportsMode(imageprovider.GenerationMode(requestedMode)) {
		return requestedMode, FallbackModeNone, nil
	}

	startIdx := -1
	for i, m := range modeFallbackChain {
		if m == requestedMode {
			startIdx = i
			break
		}
	}
	if startIdx == -1 {
		startIdx = 0
	}

	for i := startIdx + 1; i < len(modeFallbackChain); i++ {
		candidate := modeFallbackChain[i]
		if p.capabilities.SupportsMode(imageprovider.GenerationMode(candidate)) {
			return candidate, FallbackModeDegrade, nil
		}
	}

	return "", FallbackModeNone, NewGenerationError(ErrCodePlanModeNotSupported,
		fmt.Sprintf("provider %s does not support mode %s or any fallback", p.provider, requestedMode), nil)
}

func (p *ActionGenerationPlanner) calculatePrimaryRequests(mode string, segmentCount int) int {
	switch imageprovider.GenerationMode(mode) {
	case imageprovider.ModeSingleFrame, imageprovider.ModeLegacyFrame:
		if p.frameCount > 0 {
			return p.frameCount
		}
		return 1
	default:
		return segmentCount
	}
}

func (p *ActionGenerationPlanner) calculateMaxProviderCalls(primaryRequests int) int {
	return primaryRequests * (1 + MaxRetriesPerSegment)
}

func (p *ActionGenerationPlanner) checkBudget(primaryRequests, maxProviderCalls, outputCount, width, height int) error {
	if p.budget.MaxPrimaryRequests > 0 && primaryRequests > p.budget.MaxPrimaryRequests {
		return NewGenerationError(ErrCodeBudgetPrimaryRequestsExceed,
			fmt.Sprintf("primary requests %d exceeds budget %d", primaryRequests, p.budget.MaxPrimaryRequests), nil)
	}
	if p.budget.MaxProviderCalls > 0 && maxProviderCalls > p.budget.MaxProviderCalls {
		return NewGenerationError(ErrCodeBudgetProviderCallsExceed,
			fmt.Sprintf("max provider calls %d exceeds budget %d", maxProviderCalls, p.budget.MaxProviderCalls), nil)
	}
	totalImages := maxProviderCalls * outputCount
	if p.budget.MaxOutputImages > 0 && totalImages > p.budget.MaxOutputImages {
		return NewGenerationError(ErrCodeBudgetOutputImagesExceed,
			fmt.Sprintf("total output images %d exceeds budget %d", totalImages, p.budget.MaxOutputImages), nil)
	}
	if p.budget.MaxTotalPixels > 0 {
		totalPixels := int64(maxProviderCalls) * int64(width) * int64(height)
		if totalPixels > p.budget.MaxTotalPixels {
			return NewGenerationError(ErrCodeBudgetTotalPixelsExceed,
				fmt.Sprintf("total pixels %d exceeds budget %d", totalPixels, p.budget.MaxTotalPixels), nil)
		}
	}
	return nil
}

func (p *ActionGenerationPlanner) resolveSeedValue(plan *GenerationPlanSnapshot) *int64 {
	if p.seedValue != nil {
		return p.seedValue
	}
	if p.seedPolicy == SeedPolicyFixed || p.seedPolicy == SeedPolicyDeterministic {
		seed := p.computeDeterministicSeed(plan)
		return &seed
	}
	return nil
}

func (p *ActionGenerationPlanner) computeDeterministicSeed(plan *GenerationPlanSnapshot) int64 {
	data := fmt.Sprintf("%s|%s|%s|%s|%s", plan.ActionKey, plan.Mode, plan.PromptHash, plan.LayoutHash, plan.CapabilityHash)
	h := computeSHA256Hex(data)
	var seed int64
	for i := 0; i < 8 && i < len(h); i++ {
		seed = seed*16 + int64(h[i]%16)
	}
	if seed < 0 {
		seed = -seed
	}
	return seed
}

func computePlanHash(plan *GenerationPlanSnapshot) string {
	data := fmt.Sprintf("%d|%s|%s|%s|%s|%d|%s|%s|%s|%s|%s|%d|%d|%d|%d|%dx%d",
		plan.SchemaVersion,
		plan.ActionKey,
		plan.Mode,
		plan.Provider,
		plan.Model,
		plan.ConfigID,
		plan.CapabilityHash,
		plan.LayoutHash,
		plan.PromptHash,
		plan.NegativePromptHash,
		plan.SeedPolicy,
		plan.OutputCount,
		plan.PlannedSegmentCount,
		plan.PlannedPrimaryRequestCount,
		plan.PlannedMaxProviderCallCount,
		plan.SheetWidth,
		plan.SheetHeight,
	)
	return computeSHA256Hex(data)
}
