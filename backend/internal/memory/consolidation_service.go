package memory

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/u-ai/backend/internal/prompt/textlib"
	"github.com/u-ai/backend/log"
)

func (s *service) RunConsolidation(req *ConsolidationRequest) (*ConsolidationResult, error) {
	operationID := uuid.New().String()
	maxOutputs := req.MaxOutputs
	if maxOutputs <= 0 {
		maxOutputs = 5
	}
	if maxOutputs > 10 {
		maxOutputs = 10
	}

	candidates := s.consolidateSelectCandidates(req, maxOutputs)
	if len(candidates) == 0 {
		return &ConsolidationResult{OperationID: operationID}, nil
	}

	candidates = s.consolidateExactDedupe(candidates)
	candidates = s.consolidateIdempotencyDedupe(candidates, operationID)

	committed := 0
	skipped := 0
	requiresReview := 0
	var errors []string

	for _, c := range candidates {
		switch c.ProposedAction {
		case string(ProposedActionReinforce), string(ProposedActionMerge):
			if c.TargetMemoryID == "" {
				requiresReview++
				continue
			}
			if err := s.commitReinforceOrMerge(c, operationID); err != nil {
				errors = append(errors, err.Error())
				continue
			}
			committed++
		case string(ProposedActionKeepNew), string(ProposedActionCreate):
			if c.DerivationKey != "" {
				existing, _ := s.repo.FindByDerivationKey(c.DerivationKey)
				if existing != nil && existing.ID != "" {
					skipped++
					continue
				}
			}
			if err := s.commitDerivedMemory(c, req, operationID); err != nil {
				errors = append(errors, err.Error())
				continue
			}
			committed++
		default:
			requiresReview++
		}
	}

	return &ConsolidationResult{
		OperationID:      operationID,
		Candidates:       candidates,
		CommittedCount:   committed,
		SkippedDuplicate: skipped,
		RequiresReview:   requiresReview,
		Errors:           errors,
	}, nil
}

func (s *service) consolidateSelectCandidates(req *ConsolidationRequest, maxOutputs int) []ConsolidationCandidateProposal {

	if req.Source == "reflection" {
		return s.consolidateFromReflection(req, maxOutputs)
	}

	var mems []Memory
	if req.CharacterID != "" {
		mems, _ = s.repo.GetRankedByImportance(req.CharacterID, 50)
	}

	var factLines []string
	for _, m := range mems {
		if m.VerifiedStatus == "replaced" || m.VerifiedStatus == "tombstone" {
			continue
		}
		summary := buildEntrySummary(m.Key, m.Value, m.MemoryType, m.Importance)
		factLines = append(factLines, fmt.Sprintf("[%s] (%s) %s", m.ID, m.MemoryType, summary))
	}

	if len(factLines) < 3 {
		return nil
	}

	cfg := s.getActiveModel()
	if cfg == nil {
		return s.consolidateDeterministic(mems, maxOutputs)
	}

	userMsg := fmt.Sprintf(textlib.MemoryConsolidationUserMsgTemplate, len(factLines), strings.Join(factLines, "\n"))

	messages := []map[string]interface{}{
		{"role": "system", "content": textlib.MemoryConsolidationSystemPrompt},
		{"role": "user", "content": userMsg},
	}

	content, _, err := s.callLLM(cfg, messages)
	if err != nil {
		log.Warn("Consolidation LLM failed:", err)
		return s.consolidateDeterministic(mems, maxOutputs)
	}

	content = extractJSONObject(content)
	var llmResult struct {
		Insights []ConsolidationCandidateProposal `json:"insights"`
	}
	if err := json.Unmarshal([]byte(content), &llmResult); err != nil {
		return s.consolidateDeterministic(mems, maxOutputs)
	}

	var valid []ConsolidationCandidateProposal
	for i := range llmResult.Insights {
		c := &llmResult.Insights[i]
		if strings.TrimSpace(c.Key) == "" || strings.TrimSpace(c.Value) == "" {
			continue
		}
		if !req.IncludeConflict && c.CandidateKind == string(CandidateKindConflictResolution) {
			continue
		}
		if c.ProposedAction == "" {
			c.ProposedAction = string(ProposedActionCreate)
		}
		c.DerivationKey = s.computeDerivationKey(req, c.SourceMemoryIDs, c.SourceVersions, c.ProposedAction)
		valid = append(valid, *c)
		if len(valid) >= maxOutputs {
			break
		}
	}

	return valid
}

func (s *service) consolidateDeterministic(mems []Memory, maxOutputs int) []ConsolidationCandidateProposal {
	groups := make(map[string][]Memory)
	for _, m := range mems {
		normalizedKey := strings.ToLower(strings.TrimSpace(m.Key))
		if normalizedKey == "" {
			continue
		}
		groups[normalizedKey] = append(groups[normalizedKey], m)
	}

	var proposals []ConsolidationCandidateProposal
	for _, groupMem := range groups {
		if len(groupMem) < 2 {
			continue
		}
		sort.Slice(groupMem, func(i, j int) bool {
			return groupMem[i].Confidence > groupMem[j].Confidence
		})
		best := groupMem[0]

		sourceIDs := make([]string, 0, len(groupMem))
		sourceVersions := make([]int, 0, len(groupMem))
		for _, m := range groupMem {
			sourceIDs = append(sourceIDs, m.ID)
			sourceVersions = append(sourceVersions, m.Version)
		}

		confidence := s.computeDerivedConfidence(groupMem)
		importance := s.computeDerivedImportance(groupMem)

		proposals = append(proposals, ConsolidationCandidateProposal{
			CandidateKind:   string(CandidateKindConsolidated),
			Key:             best.Key,
			Value:           best.Value,
			MemoryType:      best.MemoryType,
			Importance:      importance,
			Confidence:      confidence,
			ProposedAction:  string(ProposedActionReinforce),
			SourceMemoryIDs: sourceIDs,
			SourceVersions:  sourceVersions,
			Reason:          fmt.Sprintf("duplicate key normalized merge (%d sources)", len(groupMem)),
		})

		if len(proposals) >= maxOutputs {
			break
		}
	}

	for i := range proposals {
		proposals[i].DerivationKey = s.computeDerivationKey(nil, proposals[i].SourceMemoryIDs, proposals[i].SourceVersions, proposals[i].ProposedAction)
	}

	return proposals
}

func (s *service) consolidateFromReflection(req *ConsolidationRequest, maxOutputs int) []ConsolidationCandidateProposal {

	var candidates []MemoryCandidateModel
	if req.CharacterID != "" {
		candidates, _ = s.repo.ListCandidates()
	}

	var proposals []ConsolidationCandidateProposal
	for _, c := range candidates {
		if c.CandidateKind != "" && c.CandidateKind != string(CandidateKindReflection) {
			continue
		}
		proposed := string(ProposedActionCreate)
		proposals = append(proposals, ConsolidationCandidateProposal{
			CandidateKind:   string(CandidateKindReflection),
			Key:             c.Key,
			Value:           c.Value,
			MemoryType:      c.MemoryType,
			Importance:      c.Importance,
			Confidence:      c.ConfidenceReal,
			ProposedAction:  proposed,
			SourceMemoryIDs: parseSourceIDs(c.SourceMemoryIDsJSON),
			DerivationKey:   c.DerivationKey,
			Reason:          c.Reason,
		})
		if len(proposals) >= maxOutputs {
			break
		}
	}

	return proposals
}

func (s *service) consolidateExactDedupe(candidates []ConsolidationCandidateProposal) []ConsolidationCandidateProposal {
	seen := make(map[string]bool)
	result := make([]ConsolidationCandidateProposal, 0, len(candidates))
	for _, c := range candidates {
		normalized := strings.ToLower(strings.TrimSpace(c.Key)) + "|" + strings.ToLower(strings.TrimSpace(c.Value))
		if seen[normalized] {
			continue
		}
		seen[normalized] = true
		result = append(result, c)
	}
	return result
}

func (s *service) consolidateIdempotencyDedupe(candidates []ConsolidationCandidateProposal, operationID string) []ConsolidationCandidateProposal {
	result := make([]ConsolidationCandidateProposal, 0, len(candidates))
	for _, c := range candidates {
		if c.DerivationKey == "" {
			result = append(result, c)
			continue
		}
		existing, _ := s.repo.FindByDerivationKey(c.DerivationKey)
		if existing != nil && existing.ID != "" {
			continue
		}
		result = append(result, c)
	}
	return result
}

func (s *service) commitDerivedMemory(c ConsolidationCandidateProposal, req *ConsolidationRequest, operationID string) error {
	characterID := req.CharacterID
	scope := req.Scope
	if scope == "" {
		scope = "character"
	}
	source := req.Source
	if source == "" {
		source = "auto"
	}

	sensitivity := computeConservativeSensitivity(c.SourceMemoryIDs, s.repo)
	proactive := sensitivity.allowProactive
	requiresConf := sensitivity.requiresConfirmation

	importance := c.Importance
	if importance < 1 {
		importance = 5
	}
	if importance > 10 {
		importance = 10
	}

	confidence := int(c.Confidence*100 + 0.5)
	if confidence < 1 {
		confidence = 1
	}
	if confidence > 100 {
		confidence = 100
	}
	if confidence == 0 {
		confidence = 50
	}

	memoryType, ok := NormalizeMemoryType(c.MemoryType)
	if !ok {
		memoryType = MemoryTypeFact
	}

	var derivations []MemoryDerivationInput
	for i, srcID := range c.SourceMemoryIDs {
		srcVersion := 1
		if i < len(c.SourceVersions) {
			srcVersion = c.SourceVersions[i]
		}
		inputSnapshotHash := ""
		if srcMem, err := s.repo.FindByID(srcID); err == nil {
			inputSnapshotHash = computeMemorySnapshotHashCanonical(srcMem)
		}
		derivations = append(derivations, MemoryDerivationInput{
			InputMemoryID:     srcID,
			InputVersion:      srcVersion,
			InputSnapshotHash: inputSnapshotHash,
			DerivationKind:    string(DerivationKindMerge),
			Ordinal:           i,
		})
	}

	m, err := s.createCanonicalMemory(canonicalCreateRequest{
		CharacterID:           characterID,
		MemoryType:            memoryType,
		Source:                source,
		Scope:                 scope,
		Key:                   c.Key,
		Value:                 c.Value,
		Importance:            importance,
		Confidence:            confidence,
		DerivationKey:         c.DerivationKey,
		SensitivityLevel:      sensitivity.level,
		AllowProactiveMention: proactive,
		RequiresConfirmation:  requiresConf,
		SourceConvID:          req.Scope,
		OperationID:           operationID,
		EventType:             "memory_created",
		EventReason:           "consolidation_create",
		Derivations:           derivations,
	})
	if err != nil {
		return err
	}

	go s.SyncEmbedding(m.ID, m.Key, m.Value, m.CharacterID, m.MemoryType)
	s.syncGraph(m)
	return nil
}

func (s *service) commitReinforceOrMerge(c ConsolidationCandidateProposal, operationID string) error {
	existing, err := s.repo.FindByID(c.TargetMemoryID)
	if err != nil {
		return err
	}

	updates := map[string]interface{}{
		"importance":       maxInt(existing.Importance, c.Importance),
		"confidence":       maxInt(existing.Confidence, int(c.Confidence*100+0.5)),
		"last_verified_at": time.Now().Format("2006-01-02 15:04:05"),
	}

	if c.ProposedAction == string(ProposedActionMerge) && c.Value != "" {
		updates["value"] = existing.Value + "; " + c.Value
	}

	memoryType, ok := NormalizeMemoryType(c.MemoryType)
	if ok {
		updates["memory_type"] = string(memoryType)
	}

	m, err := s.updateCanonicalMemory(existing.ID, canonicalUpdateRequest{
		Updates:     updates,
		OperationID: operationID,
		EventType:   "memory_reinforced",
		EventReason: "consolidation_merge",
	})
	if err != nil {
		return err
	}

	go s.SyncEmbedding(m.ID, m.Key, m.Value, m.CharacterID, m.MemoryType)
	s.syncGraph(m)
	return nil
}

func (s *service) computeDerivationKey(req *ConsolidationRequest, sourceIDs []string, sourceVersions []int, proposedAction string) string {
	sortedIDs := make([]string, len(sourceIDs))
	copy(sortedIDs, sourceIDs)
	sort.Strings(sortedIDs)

	var versionParts []string
	for _, v := range sourceVersions {
		versionParts = append(versionParts, fmt.Sprintf("%d", v))
	}
	sort.Strings(versionParts)

	var sb strings.Builder
	if req != nil {
		sb.WriteString(req.CharacterID)
		sb.WriteString("|")
		sb.WriteString(req.Source)
		sb.WriteString("|")
	}
	sb.WriteString(proposedAction)
	sb.WriteString("|")
	sb.WriteString(strings.Join(sortedIDs, ","))
	sb.WriteString("|v")
	sb.WriteString(strings.Join(versionParts, ","))

	if req != nil {
		if req.PolicyVersion != "" {
			sb.WriteString("|p:")
			sb.WriteString(req.PolicyVersion)
		}
		if req.PromptVersion != "" {
			sb.WriteString("|i:")
			sb.WriteString(req.PromptVersion)
		}
	}

	hash := sha256.Sum256([]byte(sb.String()))
	return hex.EncodeToString(hash[:])
}

func (s *service) computeDerivedConfidence(mems []Memory) float64 {
	if len(mems) == 0 {
		return 0.5
	}
	var total float64
	for _, m := range mems {
		total += float64(m.Confidence) / 100.0
	}
	avg := total / float64(len(mems))
	return avg
}

func (s *service) computeDerivedImportance(mems []Memory) int {
	if len(mems) == 0 {
		return 5
	}
	maxImp := 0
	for _, m := range mems {
		if m.Importance > maxImp {
			maxImp = m.Importance
		}
	}
	return maxImp
}

func (s *service) ListConsolidationCandidates(kind string) ([]MemoryCandidate, error) {
	all, err := s.repo.ListCandidates()
	if err != nil {
		return nil, err
	}
	result := make([]MemoryCandidate, 0)
	for _, m := range all {
		if kind == "" || m.CandidateKind == kind {
			result = append(result, MemoryCandidate{
				ID: m.ID, Key: m.Key, Value: m.Value,
				MemoryType: m.MemoryType, Importance: m.Importance,
				SourceText: m.SourceText, ConversationID: m.ConversationID,
				CharacterID: m.CharacterID, CreatedAt: m.CreatedAt,
			})
		}
	}
	return result, nil
}

func (s *service) AcceptConsolidationCandidate(id string) (*Memory, error) {
	return s.AcceptCandidate(id)
}

func (s *service) RejectConsolidationCandidate(id string) error {
	return s.RejectCandidate(id)
}

func computeConservativeSensitivity(sourceIDs []string, repo Repository) sensitivityAggregate {
	level := "internal"
	proactive := true
	requiresConf := false

	for _, id := range sourceIDs {
		if m, err := repo.FindByID(id); err == nil {
			sensLevel := strings.ToLower(m.SensitivityLevel)
			if rankSensitivity(sensLevel) > rankSensitivity(level) {
				level = sensLevel
			}
			if !m.AllowProactiveMention {
				proactive = false
			}
			if m.RequiresConfirmation {
				requiresConf = true
			}
		}
	}

	return sensitivityAggregate{level: level, allowProactive: proactive, requiresConfirmation: requiresConf}
}

func rankSensitivity(level string) int {
	switch strings.TrimSpace(level) {
	case "public":
		return 0
	case "internal":
		return 1
	case "private":
		return 2
	case "secret":
		return 3
	default:
		return 1
	}
}

func parseSourceIDs(raw string) []string {
	if strings.TrimSpace(raw) == "" {
		return nil
	}
	var ids []string
	raw = strings.TrimSpace(raw)
	if strings.HasPrefix(raw, "[") {
		_ = json.Unmarshal([]byte(raw), &ids)
		return ids
	}
	return strings.Split(raw, ",")
}

type sensitivityAggregate struct {
	level                string
	allowProactive       bool
	requiresConfirmation bool
}
