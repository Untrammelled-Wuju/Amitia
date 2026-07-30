package migration

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"path"
	"sort"
	"strings"
	"sync"
	"time"
)

var ErrMigrationExecutionInterrupted = errors.New("migration execution interrupted")
var ErrMigrationManualRecovery = errors.New("migration manual recovery required")

type ReversiblePreflightInput struct {
	ExtensionID string                `json:"extensionId"`
	FromVersion string                `json:"fromVersion"`
	ToVersion   string                `json:"toVersion"`
	Definitions []MigrationDefinition `json:"definitions"`
}

type ReversiblePlanStep struct {
	Order          int                `json:"order"`
	MigrationID    string             `json:"migrationId"`
	DefinitionHash string             `json:"definitionHash"`
	FromVersion    string             `json:"fromVersion"`
	ToVersion      string             `json:"toVersion"`
	Direction      MigrationDirection `json:"direction"`
	Reversibility  Reversibility      `json:"reversibility"`
	Idempotency    Idempotency        `json:"idempotency"`
	DataDomains    []DataDomain       `json:"dataDomains"`
}

type ReversiblePreflight struct {
	ExtensionID              string                         `json:"extensionId"`
	FromVersion              string                         `json:"fromVersion"`
	ToVersion                string                         `json:"toVersion"`
	ForwardPlan              []ReversiblePlanStep           `json:"forwardPlan"`
	ReversePlan              []ReversiblePlanStep           `json:"reversePlan"`
	Irreversible             bool                           `json:"irreversible"`
	ManualRequired           bool                           `json:"manualRequired"`
	ManualReasons            []string                       `json:"manualReasons"`
	UserDataSnapshotRequired bool                           `json:"userDataSnapshotRequired"`
	SnapshotDomains          []DataDomain                   `json:"snapshotDomains"`
	AlreadyApplied           map[string]bool                `json:"alreadyApplied"`
	PlanHash                 string                         `json:"planHash"`
	Definitions              map[string]MigrationDefinition `json:"-"`
}

type ReversibleStepResult struct {
	Evidence json.RawMessage `json:"evidence"`
}

type ReversibleStepHandler func(context.Context, ReversiblePlanStep, MigrationDefinition) (ReversibleStepResult, error)

type ReversibleExecutionRequest struct {
	OperationID     string
	Preflight       *ReversiblePreflight
	Snapshot        []byte
	SnapshotHash    string
	CurrentSnapshot func(context.Context) ([]byte, error)
	AllowManual     bool
}

type ReversibleMigrationCore struct {
	repo *MigrationRepository
	mu   sync.Mutex
}

func NewReversibleMigrationCore(repo *MigrationRepository) *ReversibleMigrationCore {
	return &ReversibleMigrationCore{repo: repo}
}

func (c *ReversibleMigrationCore) Preflight(ctx context.Context, input ReversiblePreflightInput) (*ReversiblePreflight, error) {
	if strings.TrimSpace(input.ExtensionID) == "" || strings.TrimSpace(input.FromVersion) == "" || strings.TrimSpace(input.ToVersion) == "" {
		return nil, fmt.Errorf("migration: extension and versions are required")
	}
	if input.FromVersion == input.ToVersion {
		return nil, fmt.Errorf("migration: source and target versions are identical")
	}
	checker := NewVersionCompatibilityChecker()
	if _, err := checker.ParseSemver(input.FromVersion); err != nil {
		return nil, err
	}
	if _, err := checker.ParseSemver(input.ToVersion); err != nil {
		return nil, err
	}
	definitions, err := validateAndIndexReversibleDefinitions(input)
	if err != nil {
		return nil, err
	}
	forward, err := resolveDeterministicForwardPlan(input.FromVersion, input.ToVersion, definitions)
	if err != nil {
		return nil, err
	}
	result := &ReversiblePreflight{ExtensionID: input.ExtensionID, FromVersion: input.FromVersion, ToVersion: input.ToVersion, ForwardPlan: forward, AlreadyApplied: map[string]bool{}, Definitions: definitions}
	domainKeys := map[string]bool{}
	for index := len(forward) - 1; index >= 0; index-- {
		step := forward[index]
		definition := definitions[step.MigrationID]
		for _, domain := range definition.DataDomains {
			key := domain.Domain + "\x00" + domain.Storage + "\x00" + domain.Namespace
			if !domainKeys[key] {
				domainKeys[key] = true
				result.SnapshotDomains = append(result.SnapshotDomains, domain)
			}
		}
		if len(definition.DataDomains) > 0 {
			result.UserDataSnapshotRequired = true
		}
		if definition.Reversibility == ReversibilityIrreversible {
			result.Irreversible = true
			result.ManualRequired = true
			result.ManualReasons = append(result.ManualReasons, "irreversible:"+definition.MigrationID)
			continue
		}
		if definition.ReverseMigrationID == nil || strings.TrimSpace(*definition.ReverseMigrationID) == "" {
			result.ManualRequired = true
			result.ManualReasons = append(result.ManualReasons, "missing_reverse:"+definition.MigrationID)
			continue
		}
		reverse, ok := definitions[*definition.ReverseMigrationID]
		if !ok || reverse.Direction != DirectionReverse {
			return nil, fmt.Errorf("migration: invalid reverse definition for %s", definition.MigrationID)
		}
		if reverse.ForwardMigrationID == nil || *reverse.ForwardMigrationID != definition.MigrationID {
			return nil, fmt.Errorf("migration: reverse linkage mismatch for %s", definition.MigrationID)
		}
		result.ReversePlan = append(result.ReversePlan, reversibleStep(len(result.ReversePlan)+1, reverse, step.ToVersion, step.FromVersion))
	}
	sort.Slice(result.SnapshotDomains, func(i, j int) bool {
		left := result.SnapshotDomains[i].Domain + "\x00" + result.SnapshotDomains[i].Storage + "\x00" + result.SnapshotDomains[i].Namespace
		right := result.SnapshotDomains[j].Domain + "\x00" + result.SnapshotDomains[j].Storage + "\x00" + result.SnapshotDomains[j].Namespace
		return left < right
	})
	sort.Strings(result.ManualReasons)
	if c != nil && c.repo != nil {
		if err := c.loadAppliedState(ctx, result); err != nil {
			return nil, err
		}
	}
	result.PlanHash, err = hashReversiblePreflight(result)
	if err != nil {
		return nil, err
	}
	return result, nil
}

func validateAndIndexReversibleDefinitions(input ReversiblePreflightInput) (map[string]MigrationDefinition, error) {
	definitions := make(map[string]MigrationDefinition, len(input.Definitions))
	edges := map[string]string{}
	checker := NewVersionCompatibilityChecker()
	for _, definition := range input.Definitions {
		if strings.TrimSpace(definition.MigrationID) == "" || strings.TrimSpace(definition.DefinitionHash) == "" {
			return nil, fmt.Errorf("migration: migration id and definition hash are required")
		}
		if _, exists := definitions[definition.MigrationID]; exists {
			return nil, fmt.Errorf("migration: duplicate migration id %s", definition.MigrationID)
		}
		if definition.ExtensionID != input.ExtensionID {
			return nil, fmt.Errorf("migration: extension mismatch for %s", definition.MigrationID)
		}
		if err := validateMigrationEntry(definition.Entry); err != nil {
			return nil, fmt.Errorf("migration: %s: %w", definition.MigrationID, err)
		}
		if _, err := checker.ParseSemver(definition.ToVersion); err != nil {
			return nil, err
		}
		if !strings.ContainsAny(definition.FromVersionRange, "<>=^~*,| ()[]") {
			if _, err := checker.ParseSemver(definition.FromVersionRange); err != nil {
				return nil, err
			}
		}
		if definition.Direction != DirectionForward && definition.Direction != DirectionReverse {
			return nil, fmt.Errorf("migration: invalid direction for %s", definition.MigrationID)
		}
		switch definition.Idempotency {
		case IdempotencyIdempotent, IdempotencyCheckpointIdempotent, IdempotencyNonIdempotent:
		default:
			return nil, fmt.Errorf("migration: invalid idempotency for %s", definition.MigrationID)
		}
		switch definition.Reversibility {
		case ReversibilityFullyReversible, ReversibilitySnapshotReversible, ReversibilityReverseScriptRequired, ReversibilityIrreversible:
		default:
			return nil, fmt.Errorf("migration: invalid reversibility for %s", definition.MigrationID)
		}
		edge := string(definition.Direction) + "\x00" + definition.FromVersionRange + "\x00" + definition.ToVersion
		if previous, exists := edges[edge]; exists {
			return nil, fmt.Errorf("migration: duplicate version edge %s and %s", previous, definition.MigrationID)
		}
		edges[edge] = definition.MigrationID
		definitions[definition.MigrationID] = definition
	}
	if err := validateReversibleVersionGraph(definitions); err != nil {
		return nil, err
	}
	return definitions, nil
}

func validateReversibleVersionGraph(definitions map[string]MigrationDefinition) error {
	graph := map[string][]string{}
	for _, definition := range definitions {
		if definition.Direction != DirectionForward || strings.ContainsAny(definition.FromVersionRange, "<>=^~*,| ") {
			continue
		}
		graph[definition.FromVersionRange] = append(graph[definition.FromVersionRange], definition.ToVersion)
	}
	state := map[string]uint8{}
	var visit func(string) error
	visit = func(version string) error {
		if state[version] == 1 {
			return fmt.Errorf("migration: forward version graph contains cycle at %s", version)
		}
		if state[version] == 2 {
			return nil
		}
		state[version] = 1
		for _, next := range graph[version] {
			if err := visit(next); err != nil {
				return err
			}
		}
		state[version] = 2
		return nil
	}
	for version := range graph {
		if err := visit(version); err != nil {
			return err
		}
	}
	return nil
}

func validateMigrationEntry(entry string) error {
	if entry == "" || strings.Contains(entry, "\\") || strings.Contains(entry, ":") || strings.ContainsRune(entry, 0) || strings.HasPrefix(entry, "/") {
		return fmt.Errorf("unsafe migration entry")
	}
	cleaned := path.Clean(entry)
	if cleaned == "." || cleaned == ".." || strings.HasPrefix(cleaned, "../") || cleaned != entry {
		return fmt.Errorf("unsafe migration entry")
	}
	decoded, err := url.PathUnescape(entry)
	if err != nil || decoded != entry {
		if err != nil || decoded == "" || strings.Contains(decoded, "\\") || strings.Contains(decoded, ":") || strings.ContainsRune(decoded, 0) || strings.HasPrefix(decoded, "/") {
			return fmt.Errorf("unsafe migration entry")
		}
		decodedClean := path.Clean(decoded)
		if decodedClean == "." || decodedClean == ".." || strings.HasPrefix(decodedClean, "../") || decodedClean != decoded {
			return fmt.Errorf("unsafe migration entry")
		}
	}
	return nil
}

func resolveDeterministicForwardPlan(fromVersion, toVersion string, definitions map[string]MigrationDefinition) ([]ReversiblePlanStep, error) {
	forwards := make([]MigrationDefinition, 0)
	for _, definition := range definitions {
		if definition.Direction == DirectionForward {
			forwards = append(forwards, definition)
		}
	}
	sort.Slice(forwards, func(i, j int) bool {
		if forwards[i].ToVersion != forwards[j].ToVersion {
			return forwards[i].ToVersion < forwards[j].ToVersion
		}
		return forwards[i].MigrationID < forwards[j].MigrationID
	})
	type route struct {
		version string
		steps   []ReversiblePlanStep
	}
	queue := []route{{version: fromVersion}}
	visited := map[string]bool{fromVersion: true}
	checker := NewVersionCompatibilityChecker()
	for len(queue) > 0 {
		current := queue[0]
		queue = queue[1:]
		for _, definition := range forwards {
			if !checker.MatchVersionRange(definition.FromVersionRange, current.version) || definition.ToVersion == current.version {
				continue
			}
			nextSteps := append(append([]ReversiblePlanStep(nil), current.steps...), reversibleStep(len(current.steps)+1, definition, current.version, definition.ToVersion))
			if definition.ToVersion == toVersion {
				return nextSteps, nil
			}
			if !visited[definition.ToVersion] {
				visited[definition.ToVersion] = true
				queue = append(queue, route{version: definition.ToVersion, steps: nextSteps})
			}
		}
	}
	return nil, fmt.Errorf("migration: no deterministic path from %s to %s", fromVersion, toVersion)
}

func reversibleStep(order int, definition MigrationDefinition, fromVersion, toVersion string) ReversiblePlanStep {
	return ReversiblePlanStep{Order: order, MigrationID: definition.MigrationID, DefinitionHash: definition.DefinitionHash, FromVersion: fromVersion, ToVersion: toVersion, Direction: definition.Direction, Reversibility: definition.Reversibility, Idempotency: definition.Idempotency, DataDomains: append([]DataDomain(nil), definition.DataDomains...)}
}

func hashReversiblePreflight(preflight *ReversiblePreflight) (string, error) {
	payload := struct {
		ExtensionID    string
		FromVersion    string
		ToVersion      string
		ForwardPlan    []ReversiblePlanStep
		ReversePlan    []ReversiblePlanStep
		Irreversible   bool
		ManualRequired bool
		ManualReasons  []string
		Snapshot       []DataDomain
	}{preflight.ExtensionID, preflight.FromVersion, preflight.ToVersion, preflight.ForwardPlan, preflight.ReversePlan, preflight.Irreversible, preflight.ManualRequired, preflight.ManualReasons, preflight.SnapshotDomains}
	raw, err := json.Marshal(payload)
	if err != nil {
		return "", err
	}
	return hashMigrationBytes(raw), nil
}

func (c *ReversibleMigrationCore) loadAppliedState(ctx context.Context, preflight *ReversiblePreflight) error {
	operations, err := c.repo.ListMigrationOperations(ctx, preflight.ExtensionID)
	if err != nil {
		return err
	}
	for _, operation := range operations {
		steps, err := c.repo.ListMigrationSteps(ctx, operation.OperationID)
		if err != nil {
			return err
		}
		reversedForward := map[string]bool{}
		for _, step := range steps {
			definition, exists := preflight.Definitions[step.MigrationID]
			if step.Status == "reversed" && exists && definition.ForwardMigrationID != nil {
				reversedForward[*definition.ForwardMigrationID] = true
			}
		}
		for _, step := range steps {
			if step.Status != "succeeded" || reversedForward[step.MigrationID] {
				continue
			}
			definition, exists := preflight.Definitions[step.MigrationID]
			if !exists {
				continue
			}
			if step.InputHash != "" && step.InputHash != definition.DefinitionHash {
				return fmt.Errorf("migration: applied definition drift for %s", step.MigrationID)
			}
			preflight.AlreadyApplied[step.MigrationID] = true
		}
	}
	return nil
}

func (c *ReversibleMigrationCore) ExecuteForward(ctx context.Context, request ReversibleExecutionRequest, handler ReversibleStepHandler) (*MigrationOperation, error) {
	if c == nil || c.repo == nil || request.Preflight == nil || handler == nil || strings.TrimSpace(request.OperationID) == "" {
		return nil, fmt.Errorf("migration: reversible execution dependencies incomplete")
	}
	if request.Preflight.ManualRequired && !request.AllowManual {
		return nil, ErrMigrationManualRecovery
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	if err := verifyMigrationSnapshot(request.Snapshot, request.SnapshotHash, request.Preflight.UserDataSnapshotRequired); err != nil {
		return nil, err
	}
	op, err := c.loadOrCreateOperation(ctx, request)
	if err != nil {
		return nil, err
	}
	if op.Status == OperationStatusCompleted {
		return op, nil
	}
	if op.Status == OperationStatusFailed || op.Status == OperationStatusRecoveryRequired || op.Status == OperationStatusManualIntervention {
		return c.markManual(ctx, op, "migration_terminal_resume", ErrMigrationManualRecovery)
	}
	existing, err := c.repo.ListMigrationSteps(ctx, request.OperationID)
	if err != nil {
		return op, err
	}
	stepRecords := make(map[int]MigrationStepRecord, len(existing))
	for _, record := range existing {
		stepRecords[record.StepID] = record
	}
	op.Status = OperationStatusMigrating
	if err := c.repo.SaveMigrationOperation(ctx, op); err != nil {
		return op, err
	}
	for _, step := range request.Preflight.ForwardPlan {
		definition := request.Preflight.Definitions[step.MigrationID]
		if request.Preflight.AlreadyApplied[step.MigrationID] {
			continue
		}
		if record, exists := stepRecords[step.Order]; exists {
			if record.MigrationID != step.MigrationID || record.InputHash != definition.DefinitionHash {
				return c.markManual(ctx, op, "migration_step_cas", fmt.Errorf("migration: step journal drift at %d", step.Order))
			}
			if record.Status == "succeeded" {
				continue
			}
			if record.Status == "running" && definition.Idempotency == IdempotencyNonIdempotent {
				return c.markManual(ctx, op, "migration_non_idempotent_resume", ErrMigrationManualRecovery)
			}
		}
		now := time.Now().UTC()
		record := MigrationStepRecord{StepID: step.Order, OperationID: request.OperationID, MigrationID: step.MigrationID, Status: "running", InputHash: definition.DefinitionHash, StartedAt: now}
		if err := c.repo.SaveMigrationStep(ctx, &record); err != nil {
			return op, err
		}
		op.CurrentStep = step.Order
		if err := c.repo.SaveMigrationOperation(ctx, op); err != nil {
			return op, err
		}
		result, runErr := handler(ctx, step, definition)
		if errors.Is(runErr, ErrMigrationExecutionInterrupted) {
			return op, runErr
		}
		finished := time.Now().UTC()
		record.FinishedAt = &finished
		if runErr != nil {
			record.Status = "failed"
			record.ErrorCode = "forward_failed"
			record.ErrorMessage = runErr.Error()
			record.OutputHash = hashMigrationBytes(result.Evidence)
			_ = c.repo.SaveMigrationStep(context.Background(), &record)
			if request.Preflight.Irreversible || definition.Reversibility == ReversibilityIrreversible {
				return c.markManual(ctx, op, "irreversible_forward_failed", runErr)
			}
			compensateErr := c.compensateReverse(ctx, request, handler, false)
			if compensateErr != nil {
				return c.markRecovery(ctx, op, "reverse_failed", errors.Join(runErr, compensateErr))
			}
			op.Status = OperationStatusFailed
			op.ErrorCode = "forward_compensated"
			op.ErrorMessage = runErr.Error()
			_ = c.repo.SaveMigrationOperation(context.Background(), op)
			return op, runErr
		}
		record.Status = "succeeded"
		record.OutputHash = hashMigrationBytes(result.Evidence)
		if err := c.repo.SaveMigrationStep(ctx, &record); err != nil {
			return op, err
		}
	}
	finished := time.Now().UTC()
	op.Status = OperationStatusCompleted
	op.FinishedAt = &finished
	op.CurrentStep = len(request.Preflight.ForwardPlan)
	if err := c.repo.SaveMigrationOperation(ctx, op); err != nil {
		return op, err
	}
	return op, nil
}

func (c *ReversibleMigrationCore) CompensateReverse(ctx context.Context, request ReversibleExecutionRequest, handler ReversibleStepHandler) error {
	return c.compensateReverse(ctx, request, handler, true)
}

func (c *ReversibleMigrationCore) compensateReverse(ctx context.Context, request ReversibleExecutionRequest, handler ReversibleStepHandler, lock bool) error {
	if c == nil || c.repo == nil || request.Preflight == nil || handler == nil {
		return fmt.Errorf("migration: reverse dependencies incomplete")
	}
	if lock {
		c.mu.Lock()
		defer c.mu.Unlock()
	}
	if request.Preflight.Irreversible {
		return ErrMigrationManualRecovery
	}
	if request.Preflight.ManualRequired {
		return ErrMigrationManualRecovery
	}
	if err := verifyMigrationSnapshot(request.Snapshot, request.SnapshotHash, request.Preflight.UserDataSnapshotRequired); err != nil {
		return err
	}
	steps, err := c.repo.ListMigrationSteps(ctx, request.OperationID)
	if err != nil {
		return err
	}
	applied := map[string]bool{}
	reversed := map[string]bool{}
	records := map[int]MigrationStepRecord{}
	for _, record := range steps {
		records[record.StepID] = record
		if record.Status == "succeeded" {
			applied[record.MigrationID] = true
		}
		if record.Status == "reversed" {
			reversed[record.MigrationID] = true
		}
	}
	for index, reverse := range request.Preflight.ReversePlan {
		definition := request.Preflight.Definitions[reverse.MigrationID]
		forwardID := ""
		if definition.ForwardMigrationID != nil {
			forwardID = *definition.ForwardMigrationID
		}
		if forwardID == "" || !applied[forwardID] || reversed[reverse.MigrationID] {
			continue
		}
		stepID := len(request.Preflight.ForwardPlan) + index + 1
		if existing, ok := records[stepID]; ok {
			if existing.MigrationID != reverse.MigrationID || existing.InputHash != definition.DefinitionHash {
				return fmt.Errorf("migration: reverse step journal drift at %d", stepID)
			}
			if existing.Status == "reversed" {
				continue
			}
			if existing.Status == "reversing" && definition.Idempotency == IdempotencyNonIdempotent {
				return ErrMigrationManualRecovery
			}
		}
		now := time.Now().UTC()
		record := MigrationStepRecord{StepID: stepID, OperationID: request.OperationID, MigrationID: reverse.MigrationID, Status: "reversing", InputHash: definition.DefinitionHash, StartedAt: now}
		if err := c.repo.SaveMigrationStep(ctx, &record); err != nil {
			return err
		}
		result, runErr := handler(ctx, reverse, definition)
		if errors.Is(runErr, ErrMigrationExecutionInterrupted) {
			return runErr
		}
		finished := time.Now().UTC()
		record.FinishedAt = &finished
		if runErr != nil {
			record.Status = "reverse_failed"
			record.ErrorCode = "reverse_failed"
			record.ErrorMessage = runErr.Error()
			record.OutputHash = hashMigrationBytes(result.Evidence)
			_ = c.repo.SaveMigrationStep(context.Background(), &record)
			return runErr
		}
		record.Status = "reversed"
		record.OutputHash = hashMigrationBytes(result.Evidence)
		if err := c.repo.SaveMigrationStep(ctx, &record); err != nil {
			return err
		}
	}
	if request.CurrentSnapshot != nil {
		current, err := request.CurrentSnapshot(ctx)
		if err != nil {
			return err
		}
		if hashMigrationBytes(current) != normalizeMigrationHash(request.SnapshotHash) {
			return fmt.Errorf("migration: compensated snapshot hash mismatch")
		}
	}
	return nil
}

func (c *ReversibleMigrationCore) loadOrCreateOperation(ctx context.Context, request ReversibleExecutionRequest) (*MigrationOperation, error) {
	op, err := c.repo.GetMigrationOperation(ctx, request.OperationID)
	if err == nil {
		if op.ExtensionID != request.Preflight.ExtensionID || op.FromVersion != request.Preflight.FromVersion || op.ToVersion != request.Preflight.ToVersion || op.ToDefinitionHash != request.Preflight.PlanHash || op.FromDefinitionHash != normalizeMigrationHash(request.SnapshotHash) {
			return nil, fmt.Errorf("migration: operation compare-and-swap mismatch")
		}
		return op, nil
	}
	if !strings.Contains(err.Error(), "operation not found") {
		return nil, err
	}
	path := MigrationPath{FromVersion: request.Preflight.FromVersion, ToVersion: request.Preflight.ToVersion, IsDirect: len(request.Preflight.ForwardPlan) == 1}
	for _, step := range request.Preflight.ForwardPlan {
		path.Steps = append(path.Steps, MigrationPathStep{StepID: step.Order, NodeID: MigrationNodeID(step.MigrationID), MigrationID: step.MigrationID, FromVersion: step.FromVersion, ToVersion: step.ToVersion, Direction: step.Direction})
	}
	op = &MigrationOperation{OperationID: request.OperationID, ExtensionID: request.Preflight.ExtensionID, FromVersion: request.Preflight.FromVersion, ToVersion: request.Preflight.ToVersion, FromDefinitionHash: normalizeMigrationHash(request.SnapshotHash), ToDefinitionHash: request.Preflight.PlanHash, MigrationPath: path, Status: OperationStatusCreated, StartedAt: time.Now().UTC(), Reversibility: overallPlanReversibility(request.Preflight), RequiresUserConfirm: request.Preflight.ManualRequired, UserConfirmed: request.AllowManual}
	if err := c.repo.SaveMigrationOperation(ctx, op); err != nil {
		return nil, err
	}
	return op, nil
}

func overallPlanReversibility(preflight *ReversiblePreflight) Reversibility {
	if preflight.Irreversible {
		return ReversibilityIrreversible
	}
	if preflight.UserDataSnapshotRequired {
		return ReversibilitySnapshotReversible
	}
	return ReversibilityFullyReversible
}

func (c *ReversibleMigrationCore) markManual(ctx context.Context, op *MigrationOperation, code string, cause error) (*MigrationOperation, error) {
	op.Status = OperationStatusManualIntervention
	op.ErrorCode = code
	op.ErrorMessage = cause.Error()
	_ = c.repo.SaveMigrationOperation(context.Background(), op)
	return op, errors.Join(ErrMigrationManualRecovery, cause)
}

func (c *ReversibleMigrationCore) markRecovery(ctx context.Context, op *MigrationOperation, code string, cause error) (*MigrationOperation, error) {
	op.Status = OperationStatusRecoveryRequired
	op.ErrorCode = code
	op.ErrorMessage = cause.Error()
	_ = c.repo.SaveMigrationOperation(context.Background(), op)
	return op, cause
}

func verifyMigrationSnapshot(snapshot []byte, expected string, required bool) error {
	if !required && len(snapshot) == 0 && expected == "" {
		return nil
	}
	if len(snapshot) == 0 || strings.TrimSpace(expected) == "" {
		return fmt.Errorf("migration: user data snapshot evidence required")
	}
	if hashMigrationBytes(snapshot) != normalizeMigrationHash(expected) {
		return fmt.Errorf("migration: user data snapshot hash mismatch")
	}
	return nil
}

func hashMigrationBytes(value []byte) string {
	sum := sha256.Sum256(value)
	return "sha256:" + hex.EncodeToString(sum[:])
}

func normalizeMigrationHash(value string) string {
	value = strings.TrimSpace(value)
	if value == "" || strings.HasPrefix(value, "sha256:") {
		return value
	}
	return "sha256:" + value
}
