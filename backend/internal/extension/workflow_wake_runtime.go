package extension

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/u-ai/backend/internal/asr"
	"github.com/u-ai/backend/internal/extension/kernel/workflow"
	"github.com/u-ai/backend/internal/realtime"
)

const (
	workflowWakeEventType       = "voice.wake.detected"
	workflowWakeRefreshInterval = 2 * time.Second
)

type workflowWakeRuntimeState struct {
	mu              sync.Mutex
	lastRefresh     time.Time
	detectors       map[string]*workflowWakeDetectorState
	bindingCount    int
	lastErr         error
	deviceState     string
	deviceReason    string
	deviceUpdatedAt time.Time
}

type workflowWakeDetectorState struct {
	configID   string
	configHash string
	detector   realtime.WakeDetector
}

type workflowWakeRuntimeStatus struct {
	Required        bool      `json:"required"`
	Ready           bool      `json:"ready"`
	BindingCount    int       `json:"bindingCount"`
	ConfigCount     int       `json:"configCount"`
	Reason          string    `json:"reason,omitempty"`
	DeviceState     string    `json:"deviceState,omitempty"`
	DeviceReason    string    `json:"deviceReason,omitempty"`
	DeviceUpdatedAt time.Time `json:"deviceUpdatedAt,omitempty"`
}

type workflowWakeConfigCatalogItem struct {
	ID      string `json:"id"`
	Name    string `json:"name"`
	Backend string `json:"backend"`
}

type workflowWakeTriggerConfig struct {
	Mode         string `json:"mode"`
	WakeConfigID string `json:"wakeConfigId"`
}

type workflowWakeConfigCreateRequest struct {
	Name             string   `json:"name"`
	Phrases          []string `json:"phrases"`
	Locale           string   `json:"locale"`
	Threshold        float64  `json:"threshold"`
	CooldownMS       int64    `json:"cooldownMs"`
	Backend          string   `json:"backend"`
	ModelResourceURI string   `json:"modelResourceUri"`
}

type workflowWakeConfigRecord struct {
	ID               string  `gorm:"column:id"`
	Name             string  `gorm:"column:name"`
	Enabled          bool    `gorm:"column:enabled"`
	Backend          string  `gorm:"column:backend"`
	ModelResourceURI string  `gorm:"column:model_resource_uri"`
	Phrases          string  `gorm:"column:phrases"`
	Threshold        float64 `gorm:"column:threshold"`
	CooldownMS       int64   `gorm:"column:cooldown_ms"`
	CreatedAt        string  `gorm:"column:created_at"`
	UpdatedAt        string  `gorm:"column:updated_at"`
}

func (workflowWakeConfigRecord) TableName() string { return "wake_configs" }

func (r *Runtime) createWorkflowWakeConfig(ctx context.Context, request workflowWakeConfigCreateRequest) (workflowWakeConfigCatalogItem, error) {
	if r == nil || r.Repository == nil || r.Repository.DB() == nil {
		return workflowWakeConfigCatalogItem{}, errors.New("wake config database unavailable")
	}
	name := strings.TrimSpace(request.Name)
	if name == "" {
		return workflowWakeConfigCatalogItem{}, errors.New("wake config name is required")
	}
	if len([]rune(name)) > 80 {
		return workflowWakeConfigCatalogItem{}, errors.New("wake config name is too long")
	}
	if len(request.Phrases) == 0 || len(request.Phrases) > 16 {
		return workflowWakeConfigCatalogItem{}, errors.New("wake config requires 1 to 16 phrases")
	}
	locale := strings.TrimSpace(request.Locale)
	if locale == "" {
		locale = "zh-CN"
	}
	phrases := make([]map[string]string, 0, len(request.Phrases))
	for index, raw := range request.Phrases {
		phrase := strings.TrimSpace(raw)
		if phrase == "" {
			continue
		}
		if len([]rune(phrase)) > 80 {
			return workflowWakeConfigCatalogItem{}, fmt.Errorf("wake phrase %d is too long", index+1)
		}
		phrases = append(phrases, map[string]string{
			"id":          fmt.Sprintf("wake-%d", len(phrases)+1),
			"displayText": phrase,
			"locale":      locale,
		})
	}
	if len(phrases) == 0 {
		return workflowWakeConfigCatalogItem{}, errors.New("wake config phrases are empty")
	}
	backend := normalizeWorkflowWakeBackend(request.Backend)
	modelResourceURI := strings.TrimSpace(request.ModelResourceURI)
	switch backend {
	case workflowLocalKWSWakeBackend:
		if modelResourceURI == "" {
			modelResourceURI = "builtin://amitia-kws/default"
		}
	case workflowASRWakeBackend:
		activeASR, err := asr.ActiveRuntimeConfig()
		if err != nil {
			return workflowWakeConfigCatalogItem{}, fmt.Errorf("active ASR is required for cloud ASR wake recognition: %w", err)
		}
		if !asr.SupportsSegmentPCM(activeASR) {
			return workflowWakeConfigCatalogItem{}, fmt.Errorf("active ASR provider %q cannot recognize private PCM segments; select an OpenAI-compatible or Azure ASR config first", activeASR.ApiType)
		}
		if strings.TrimSpace(activeASR.ApiKey) == "" {
			return workflowWakeConfigCatalogItem{}, errors.New("active ASR credential is empty")
		}
	default:
		return workflowWakeConfigCatalogItem{}, fmt.Errorf("unsupported wake backend %q", backend)
	}
	threshold := request.Threshold
	if threshold == 0 {
		threshold = 0.85
	}
	if threshold < 0 || threshold > 1 {
		return workflowWakeConfigCatalogItem{}, errors.New("wake threshold must be between 0 and 1")
	}
	cooldownMS := request.CooldownMS
	if cooldownMS == 0 {
		cooldownMS = 2000
	}
	if cooldownMS < 0 || cooldownMS > 600000 {
		return workflowWakeConfigCatalogItem{}, errors.New("wake cooldownMs must be between 0 and 600000")
	}
	encodedPhrases, err := json.Marshal(phrases)
	if err != nil {
		return workflowWakeConfigCatalogItem{}, err
	}
	now := time.Now().UTC().Format(time.RFC3339Nano)
	record := workflowWakeConfigRecord{
		ID:               "workflow-wake-" + uuid.NewString(),
		Name:             name,
		Enabled:          true,
		Backend:          backend,
		ModelResourceURI: modelResourceURI,
		Phrases:          string(encodedPhrases),
		Threshold:        threshold,
		CooldownMS:       cooldownMS,
		CreatedAt:        now,
		UpdatedAt:        now,
	}
	if err := r.Repository.DB().WithContext(ctx).Table(record.TableName()).Create(&record).Error; err != nil {
		return workflowWakeConfigCatalogItem{}, fmt.Errorf("create wake config: %w", err)
	}
	if state := r.workflowWakeState(); state != nil {
		state.mu.Lock()
		state.lastRefresh = time.Time{}
		state.mu.Unlock()
	}
	return workflowWakeConfigCatalogItem{ID: record.ID, Name: record.Name, Backend: record.Backend}, nil
}

func (r *Runtime) workflowWakeState() *workflowWakeRuntimeState {
	if r == nil {
		return nil
	}
	r.workflowWakeRuntimeMu.Lock()
	defer r.workflowWakeRuntimeMu.Unlock()
	if r.workflowWakeRuntime == nil {
		r.workflowWakeRuntime = &workflowWakeRuntimeState{detectors: make(map[string]*workflowWakeDetectorState)}
	}
	return r.workflowWakeRuntime
}

func normalizeWorkflowWakeBackend(value string) string {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "", "auto", "local", "local_kws", workflowLocalKWSWakeBackend:
		return workflowLocalKWSWakeBackend
	case "cloud", "asr", "asr_phrase", workflowASRWakeBackend:
		return workflowASRWakeBackend
	default:
		return strings.TrimSpace(value)
	}
}

func workflowWakeBackendUsableForAutomation(backend string) bool {
	backend = strings.TrimSpace(backend)
	if backend == "" || backend == "software" {
		// The built-in software detector is an energy-rise detector, not a
		// wake-word recognizer. Treating it as a Workflow wake source would turn
		// arbitrary loud speech into automation execution.
		return false
	}
	_, ok := realtime.GetWakeBackend(backend)
	return ok
}

func (r *Runtime) workflowWakeConfigCatalog(ctx context.Context) ([]workflowWakeConfigCatalogItem, error) {
	if r == nil || r.Repository == nil || r.Repository.DB() == nil {
		return nil, errors.New("wake config database unavailable")
	}
	var records []workflowWakeConfigRecord
	if err := r.Repository.DB().WithContext(ctx).Table("wake_configs").Where("enabled = 1").Order("name ASC, id ASC").Find(&records).Error; err != nil {
		return nil, fmt.Errorf("list wake configs: %w", err)
	}
	items := make([]workflowWakeConfigCatalogItem, 0, len(records))
	for _, record := range records {
		id := strings.TrimSpace(record.ID)
		if id == "" {
			continue
		}
		name := strings.TrimSpace(record.Name)
		if name == "" {
			name = id
		}
		backend := strings.TrimSpace(record.Backend)
		if backend == "" {
			backend = "software"
		}
		if !workflowWakeBackendUsableForAutomation(backend) {
			continue
		}
		items = append(items, workflowWakeConfigCatalogItem{ID: id, Name: name, Backend: backend})
	}
	return items, nil
}

func (r *Runtime) workflowWakeStatus(ctx context.Context, force bool) workflowWakeRuntimeStatus {
	state := r.workflowWakeState()
	if state == nil {
		return workflowWakeRuntimeStatus{Reason: "workflow wake runtime unavailable"}
	}
	state.mu.Lock()
	defer state.mu.Unlock()
	state.refreshLocked(ctx, r, force)
	status := workflowWakeRuntimeStatus{
		Required:        state.bindingCount > 0,
		Ready:           state.bindingCount > 0 && len(state.detectors) > 0,
		BindingCount:    state.bindingCount,
		ConfigCount:     len(state.detectors),
		DeviceState:     state.deviceState,
		DeviceReason:    state.deviceReason,
		DeviceUpdatedAt: state.deviceUpdatedAt,
	}
	if state.lastErr != nil {
		status.Reason = state.lastErr.Error()
	} else if status.Required && len(state.detectors) == 0 {
		status.Reason = "no usable wake config"
	}
	return status
}

func (r *Runtime) updateWorkflowWakeDeviceStatus(stateValue, reason string) error {
	stateValue = strings.ToLower(strings.TrimSpace(stateValue))
	reason = strings.TrimSpace(reason)
	switch stateValue {
	case "idle", "wake_required", "wake_active", "wake_suspended", "wake_blocked_by_android", "wake_permission_missing":
	default:
		return fmt.Errorf("invalid workflow wake device state %q", stateValue)
	}
	if len(reason) > 512 {
		reason = reason[:512]
	}
	state := r.workflowWakeState()
	if state == nil {
		return errors.New("workflow wake runtime unavailable")
	}
	state.mu.Lock()
	state.deviceState = stateValue
	state.deviceReason = reason
	state.deviceUpdatedAt = time.Now().UTC()
	state.mu.Unlock()
	return nil
}

func (r *Runtime) processWorkflowWakeAudio(ctx context.Context, pcm []byte, deviceID string, sequence uint64, capturedAt time.Time) error {
	if len(pcm) == 0 {
		return nil
	}
	state := r.workflowWakeState()
	if state == nil {
		return errors.New("workflow wake runtime unavailable")
	}
	if capturedAt.IsZero() {
		capturedAt = time.Now().UTC()
	}

	type detection struct {
		configID string
		result   realtime.WakeDetectionResult
	}
	detections := make([]detection, 0, 1)

	state.mu.Lock()
	state.refreshLocked(ctx, r, false)
	if state.bindingCount == 0 {
		state.mu.Unlock()
		return nil
	}
	if len(state.detectors) == 0 {
		err := state.lastErr
		state.mu.Unlock()
		if err != nil {
			return err
		}
		return errors.New("workflow wake detector unavailable")
	}
	var detectorErrors []error
	for configID, entry := range state.detectors {
		frame := &realtime.VoiceAudioFrame{
			SessionID:   "workflow-wake",
			Sequence:    sequence,
			TimestampNS: capturedAt.UnixNano(),
			SampleRate:  realtime.CanonicalSampleRate,
			Channels:    realtime.CanonicalChannels,
			Encoding:    realtime.CanonicalEncoding,
			PCM:         pcm,
		}
		result, err := entry.detector.Process(frame)
		if err != nil {
			// One broken backend must not suppress valid detectors bound to other
			// workflows. Remove it so subsequent frames do not hot-loop failures;
			// the normal refresh path will recreate it after configuration/backend
			// recovery.
			_ = entry.detector.Unload()
			delete(state.detectors, configID)
			processErr := fmt.Errorf("wake detector %s process: %w", configID, err)
			detectorErrors = append(detectorErrors, processErr)
			state.lastErr = errors.Join(state.lastErr, processErr)
			continue
		}
		if result.Detected {
			detections = append(detections, detection{configID: configID, result: result})
		}
	}
	state.mu.Unlock()

	if len(detections) == 0 {
		return errors.Join(detectorErrors...)
	}
	if r.Kernel == nil || r.Kernel.Container() == nil || r.Kernel.Container().WorkflowTriggerManager == nil {
		return errors.New("workflow trigger manager unavailable")
	}
	triggerErrors := append([]error(nil), detectorErrors...)
	for _, item := range detections {
		detectedAtNS := item.result.DetectedAtNS
		if detectedAtNS <= 0 {
			detectedAtNS = capturedAt.UnixNano()
		}
		payload, err := json.Marshal(map[string]any{
			"wakeConfigId": item.configID,
			"phraseId":     item.result.PhraseID,
			"confidence":   item.result.Confidence,
			"detectedAtNs": detectedAtNS,
		})
		if err != nil {
			triggerErrors = append(triggerErrors, err)
			continue
		}
		detectedAt := time.Unix(0, detectedAtNS).UTC()
		eventID := fmt.Sprintf("voice-wake:%s:%d", sanitizeWakeEventPart(item.configID), detectedAtNS)
		event := workflow.WorkflowTriggerEvent{
			EventID:    eventID,
			EventType:  workflowWakeEventType,
			Source:     "device.android.wake_audio",
			DeviceID:   strings.TrimSpace(deviceID),
			OccurredAt: detectedAt,
			Payload:    payload,
		}
		if err := r.Kernel.Container().WorkflowTriggerManager.HandleStructuredEvent(ctx, event, workflow.ExecutionContext{DeviceID: event.DeviceID}); err != nil {
			triggerErrors = append(triggerErrors, err)
		}
	}
	return errors.Join(triggerErrors...)
}

func (s *workflowWakeRuntimeState) refreshLocked(ctx context.Context, runtime *Runtime, force bool) {
	now := time.Now().UTC()
	if !force && !s.lastRefresh.IsZero() && now.Sub(s.lastRefresh) < workflowWakeRefreshInterval {
		return
	}
	s.lastRefresh = now
	s.lastErr = nil

	if runtime == nil || runtime.Kernel == nil || runtime.Kernel.Container() == nil || runtime.Kernel.Container().WorkflowDefRepo == nil {
		s.bindingCount = 0
		s.replaceDetectors(nil)
		s.lastErr = errors.New("workflow trigger store unavailable")
		return
	}
	bindings, err := runtime.Kernel.Container().WorkflowDefRepo.ListTriggers(ctx, workflow.TriggerTypeEvent, "", "")
	if err != nil {
		// Device wake automation must fail closed. Keeping a stale detector alive
		// after the trigger store becomes unreadable could continue capturing audio
		// and firing workflows from a definition that was already disabled/changed.
		s.bindingCount = 0
		s.replaceDetectors(nil)
		s.lastErr = fmt.Errorf("list wake trigger bindings: %w", err)
		return
	}
	requested := make(map[string]struct{})
	bindingCount := 0
	for _, binding := range bindings {
		if !binding.Enabled || canonicalBindingEventType(binding.EventType) != workflowWakeEventType {
			continue
		}
		bindingCount++
		var cfg workflowWakeTriggerConfig
		if err := json.Unmarshal(binding.Config, &cfg); err != nil {
			s.lastErr = errors.Join(s.lastErr, fmt.Errorf("decode wake trigger %s: %w", binding.BindingID, err))
			continue
		}
		wakeConfigID := strings.TrimSpace(cfg.WakeConfigID)
		if wakeConfigID == "" {
			s.lastErr = errors.Join(s.lastErr, fmt.Errorf("wake trigger %s has empty wakeConfigId", binding.BindingID))
			continue
		}
		requested[wakeConfigID] = struct{}{}
	}
	s.bindingCount = bindingCount
	if bindingCount == 0 {
		s.replaceDetectors(nil)
		return
	}
	if runtime.Repository == nil || runtime.Repository.DB() == nil {
		s.replaceDetectors(nil)
		s.lastErr = errors.New("wake config database unavailable")
		return
	}

	desired := make(map[string]workflowWakeConfigRecord)
	for requestedID := range requested {
		var record workflowWakeConfigRecord
		query := runtime.Repository.DB().WithContext(ctx).Table("wake_configs").Where("enabled = 1").Where("id = ?", requestedID)
		if err := query.First(&record).Error; err != nil {
			s.lastErr = errors.Join(s.lastErr, fmt.Errorf("load wake config %q: %w", requestedID, err))
			continue
		}
		record.ID = strings.TrimSpace(record.ID)
		if record.ID == "" {
			s.lastErr = errors.Join(s.lastErr, errors.New("wake config id is empty"))
			continue
		}
		backend := strings.TrimSpace(record.Backend)
		if backend == "" {
			backend = "software"
		}
		if !workflowWakeBackendUsableForAutomation(backend) {
			s.lastErr = errors.Join(s.lastErr, fmt.Errorf("wake config %q backend %q is not a wake-word recognizer", requestedID, backend))
			continue
		}
		desired[record.ID] = record
	}
	s.reconcileDetectors(ctx, desired)
}

func (s *workflowWakeRuntimeState) reconcileDetectors(ctx context.Context, desired map[string]workflowWakeConfigRecord) {
	if s.detectors == nil {
		s.detectors = make(map[string]*workflowWakeDetectorState)
	}
	for configID, existing := range s.detectors {
		record, ok := desired[configID]
		if !ok {
			_ = existing.detector.Unload()
			delete(s.detectors, configID)
			continue
		}
		desiredHash, err := workflowWakeDetectorConfigHash(record)
		if err != nil {
			_ = existing.detector.Unload()
			delete(s.detectors, configID)
			delete(desired, configID)
			s.lastErr = errors.Join(s.lastErr, fmt.Errorf("resolve wake detector config %s: %w", configID, err))
			continue
		}
		if existing.configHash == desiredHash {
			delete(desired, configID)
			continue
		}
		_ = existing.detector.Unload()
		delete(s.detectors, configID)
	}
	for configID, record := range desired {
		desiredHash, err := workflowWakeDetectorConfigHash(record)
		if err != nil {
			s.lastErr = errors.Join(s.lastErr, fmt.Errorf("resolve wake detector config %s: %w", configID, err))
			continue
		}
		backend := strings.TrimSpace(record.Backend)
		if backend == "" {
			backend = "software"
		}
		factory, ok := realtime.GetWakeBackend(backend)
		if !ok {
			s.lastErr = errors.Join(s.lastErr, fmt.Errorf("wake backend unavailable: %s", backend))
			continue
		}
		detector, err := factory("{}")
		if err != nil {
			s.lastErr = errors.Join(s.lastErr, fmt.Errorf("create wake detector %s: %w", configID, err))
			continue
		}
		phrases, err := decodeWorkflowWakePhrases(record.Phrases)
		if err != nil {
			_ = detector.Unload()
			s.lastErr = errors.Join(s.lastErr, fmt.Errorf("wake config %s: %w", configID, err))
			continue
		}
		if err := detector.Load(ctx, realtime.WakeDetectorConfig{
			Enabled:          record.Enabled,
			Backend:          backend,
			ModelResourceURI: record.ModelResourceURI,
			Phrases:          phrases,
			Threshold:        record.Threshold,
			CooldownMS:       record.CooldownMS,
		}); err != nil {
			_ = detector.Unload()
			s.lastErr = errors.Join(s.lastErr, fmt.Errorf("load wake detector %s: %w", configID, err))
			continue
		}
		s.detectors[configID] = &workflowWakeDetectorState{
			configID:   configID,
			configHash: desiredHash,
			detector:   detector,
		}
	}
}

func (s *workflowWakeRuntimeState) close() {
	if s == nil {
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	s.bindingCount = 0
	s.lastErr = nil
	s.replaceDetectors(nil)
}

func (s *workflowWakeRuntimeState) replaceDetectors(next map[string]*workflowWakeDetectorState) {
	for _, entry := range s.detectors {
		_ = entry.detector.Unload()
	}
	if next == nil {
		next = make(map[string]*workflowWakeDetectorState)
	}
	s.detectors = next
}

func canonicalBindingEventType(value string) string {
	value = strings.TrimSpace(value)
	if value == workflowWakeEventType {
		return value
	}
	suffix := ":" + workflowWakeEventType
	if strings.HasPrefix(value, "user:") && strings.HasSuffix(value, suffix) {
		return workflowWakeEventType
	}
	return value
}

func workflowWakeDetectorConfigHash(record workflowWakeConfigRecord) (string, error) {
	base := wakeConfigHash(record)
	if strings.TrimSpace(record.Backend) != workflowASRWakeBackend {
		return base, nil
	}
	active, err := asr.ActiveRuntimeConfig()
	if err != nil {
		return "", fmt.Errorf("active ASR unavailable: %w", err)
	}
	if !asr.SupportsSegmentPCM(active) {
		return "", fmt.Errorf("active ASR provider %q does not support private PCM segments", active.ApiType)
	}
	// Include credential material only inside the SHA-256 input. The raw key is
	// never persisted to workflow state or logs, but rotating it must invalidate
	// an already-loaded detector immediately.
	raw, _ := json.Marshal(map[string]any{
		"id": active.ID, "type": active.ApiType, "baseUrl": active.BaseURL,
		"resourceId": active.ResourceId, "updatedAt": active.UpdatedAt, "apiKey": active.ApiKey,
	})
	digest := sha256.Sum256(raw)
	return base + ":" + hex.EncodeToString(digest[:]), nil
}

func wakeConfigHash(record workflowWakeConfigRecord) string {
	raw, _ := json.Marshal(record)
	digest := sha256.Sum256(raw)
	return hex.EncodeToString(digest[:])
}

func decodeWorkflowWakePhrases(raw string) ([]realtime.WakePhrase, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return nil, errors.New("wake phrases are empty")
	}
	var objects []struct {
		ID          string `json:"id"`
		DisplayText string `json:"displayText"`
		Locale      string `json:"locale"`
	}
	if err := json.Unmarshal([]byte(raw), &objects); err == nil && len(objects) > 0 {
		result := make([]realtime.WakePhrase, 0, len(objects))
		for index, item := range objects {
			text := strings.TrimSpace(item.DisplayText)
			if text == "" {
				continue
			}
			id := strings.TrimSpace(item.ID)
			if id == "" {
				id = fmt.Sprintf("wake-%d", index+1)
			}
			result = append(result, realtime.WakePhrase{ID: id, DisplayText: text, Locale: strings.TrimSpace(item.Locale)})
		}
		if len(result) > 0 {
			return result, nil
		}
	}
	var stringsOnly []string
	if err := json.Unmarshal([]byte(raw), &stringsOnly); err != nil {
		return nil, fmt.Errorf("decode wake phrases: %w", err)
	}
	result := make([]realtime.WakePhrase, 0, len(stringsOnly))
	for index, value := range stringsOnly {
		value = strings.TrimSpace(value)
		if value != "" {
			result = append(result, realtime.WakePhrase{ID: fmt.Sprintf("wake-%d", index+1), DisplayText: value})
		}
	}
	if len(result) == 0 {
		return nil, errors.New("wake phrases are empty")
	}
	return result, nil
}

func sanitizeWakeEventPart(value string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return "default"
	}
	var builder strings.Builder
	for _, r := range value {
		if r >= 'a' && r <= 'z' || r >= 'A' && r <= 'Z' || r >= '0' && r <= '9' || r == '.' || r == '_' || r == ':' || r == '-' {
			builder.WriteRune(r)
		} else {
			builder.WriteByte('_')
		}
		if builder.Len() >= 80 {
			break
		}
	}
	if builder.Len() == 0 {
		return "config"
	}
	return builder.String()
}
