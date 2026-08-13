package memory

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/u-ai/backend/internal/system/dataportability"
)

const (
	ComponentIDMemoryRecords      = "memory.records"
	ComponentIDMemoryEvents       = "memory.events"
	ComponentIDMemoryTemporal     = "memory.temporal"
	ComponentIDMemoryDerivations  = "memory.derivations"
	ComponentIDMemoryCandidates   = "memory.candidates"
	ComponentIDMemoryVectorIndex  = "memory-vector-index"
	CurrentDatasetVersion         = "v1"
	RecordSizeLimit               = 1 << 20
)

type MemoryBackupContributor struct {
	svc Service
}

func NewMemoryBackupContributor(svc Service) *MemoryBackupContributor {
	return &MemoryBackupContributor{svc: svc}
}

func (c *MemoryBackupContributor) ID() string {
	return "memory"
}

func (c *MemoryBackupContributor) Name() string {
	return "Memory"
}

func (c *MemoryBackupContributor) Plan(ctx context.Context, req dataportability.BackupRequest) ([]dataportability.BackupComponentPlan, error) {
	characterID := req.CharacterID
	if req.Scope == dataportability.ScopeMemory && characterID == "" {
		return nil, nil
	}

	plans := []dataportability.BackupComponentPlan{
		{
			ID:            ComponentIDMemoryRecords,
			Kind:          dataportability.KindNDJSON,
			LogicalName:   "memory.records.v1",
			Required:      true,
			SourceOfTruth: true,
			Rebuildable:   false,
		},
		{
			ID:            ComponentIDMemoryEvents,
			Kind:          dataportability.KindNDJSON,
			LogicalName:   "memory.events.v1",
			Required:      false,
			SourceOfTruth: true,
			Rebuildable:   false,
		},
		{
			ID:            ComponentIDMemoryTemporal,
			Kind:          dataportability.KindNDJSON,
			LogicalName:   "memory.temporal.v1",
			Required:      false,
			SourceOfTruth: true,
			Rebuildable:   false,
		},
		{
			ID:            ComponentIDMemoryDerivations,
			Kind:          dataportability.KindNDJSON,
			LogicalName:   "memory.derivations.v1",
			Required:      false,
			SourceOfTruth: true,
			Rebuildable:   false,
		},
		{
			ID:            ComponentIDMemoryVectorIndex,
			Kind:          dataportability.KindMetadata,
			LogicalName:   "memory-vector-index",
			Required:      false,
			SourceOfTruth: false,
			Rebuildable:   true,
		},
	}
	_ = characterID
	return plans, nil
}

func (c *MemoryBackupContributor) Export(ctx context.Context, req dataportability.BackupRequest, out dataportability.BackupWriter) error {
	exportReq := MemoryExportRequest{
		Scope:       ExportScopeAll,
		CharacterID: req.CharacterID,
		HistoryMode: HistoryModeCurrentPlusEvents,
	}
	if req.Scope == dataportability.ScopeMemory {
		exportReq.Scope = ExportScopeCharacter
	}

	return c.doExport(ctx, exportReq, out)
}

func (c *MemoryBackupContributor) doExport(ctx context.Context, req MemoryExportRequest, out dataportability.BackupWriter) error {
	characterID := req.CharacterID

	totalRecords := int64(0)
	totalEvents := int64(0)
	totalTemporal := int64(0)
	totalDerivations := int64(0)

	recComp, err := out.CreateComponent(ComponentIDMemoryRecords, "memory.records.v1", dataportability.KindNDJSON)
	if err != nil {
		return fmt.Errorf("export: create records component: %w", err)
	}
	if err := c.streamRecords(recComp, characterID, req.SelectedIDs, &totalRecords); err != nil {
		recComp.Close()
		return fmt.Errorf("export: stream records: %w", err)
	}
	recComp.Close()

	evtComp, err := out.CreateComponent(ComponentIDMemoryEvents, "memory.events.v1", dataportability.KindNDJSON)
	if err != nil {
		return fmt.Errorf("export: create events component: %w", err)
	}
	if err := c.streamEvents(evtComp, characterID, req.SelectedIDs, &totalEvents); err != nil {
		evtComp.Close()
		return fmt.Errorf("export: stream events: %w", err)
	}
	evtComp.Close()

	tmlComp, err := out.CreateComponent(ComponentIDMemoryTemporal, "memory.temporal.v1", dataportability.KindNDJSON)
	if err != nil {
		return fmt.Errorf("export: create temporal component: %w", err)
	}
	if err := c.streamTemporal(tmlComp, characterID, req.SelectedIDs, &totalTemporal); err != nil {
		tmlComp.Close()
		return fmt.Errorf("export: stream temporal: %w", err)
	}
	tmlComp.Close()

	derComp, err := out.CreateComponent(ComponentIDMemoryDerivations, "memory.derivations.v1", dataportability.KindNDJSON)
	if err != nil {
		return fmt.Errorf("export: create derivations component: %w", err)
	}
	if err := c.streamDerivations(derComp, characterID, req.SelectedIDs, &totalDerivations); err != nil {
		derComp.Close()
		return fmt.Errorf("export: stream derivations: %w", err)
	}
	derComp.Close()

	meta := map[string]interface{}{
		"recordCount":      totalRecords,
		"eventCount":       totalEvents,
		"temporalCount":    totalTemporal,
		"derivationCount":  totalDerivations,
		"scope":            string(req.Scope),
		"historyMode":      string(req.HistoryMode),
		"generatedAt":      time.Now().UTC().Format(time.RFC3339),
		"datasetVersion":   CurrentDatasetVersion,
	}
	if err := out.WriteJSON(ComponentIDMemoryVectorIndex, meta); err != nil {
		return fmt.Errorf("export: write metadata: %w", err)
	}

	return nil
}

func (c *MemoryBackupContributor) streamRecords(w io.Writer, characterID string, selectedIDs []string, total *int64) error {
	bw := bufio.NewWriter(w)
	batchSize := 1000
	offset := 0
	for {
		var batch []Memory
		var err error
		if len(selectedIDs) > 0 {
			batch, err = c.getRepository().StreamExportableByIDs(selectedIDs, batchSize, offset)
		} else {
			batch, err = c.getRepository().StreamExportable(characterID, batchSize, offset)
		}
		if err != nil {
			return err
		}
		if len(batch) == 0 {
			break
		}
		for _, m := range batch {
			v1 := m.toV1()
			line, err := json.Marshal(v1)
			if err != nil {
				return err
			}
			if _, err := bw.Write(line); err != nil {
				return err
			}
			if err := bw.WriteByte('\n'); err != nil {
				return err
			}
			*total++
		}
		offset += len(batch)
		if len(batch) < batchSize {
			break
		}
	}
	return bw.Flush()
}

func (c *MemoryBackupContributor) streamEvents(w io.Writer, characterID string, selectedIDs []string, total *int64) error {
	bw := bufio.NewWriter(w)
	batchSize := 1000
	offset := 0
	for {
		var mems []Memory
		var err error
		if len(selectedIDs) > 0 {
			mems, err = c.getRepository().StreamExportableByIDs(selectedIDs, batchSize, offset)
		} else {
			mems, err = c.getRepository().StreamExportable(characterID, batchSize, offset)
		}
		if err != nil {
			return err
		}
		if len(mems) == 0 {
			break
		}
		ids := make([]string, 0, len(mems))
		for _, m := range mems {
			ids = append(ids, m.ID)
		}
		events, err := c.getRepository().ListEventsByMemoryIDs(ids)
		if err != nil {
			return err
		}
		for _, e := range events {
			line, err := json.Marshal(e)
			if err != nil {
				return err
			}
			if _, err := bw.Write(line); err != nil {
				return err
			}
			if err := bw.WriteByte('\n'); err != nil {
				return err
			}
			*total++
		}
		offset += len(mems)
		if len(mems) < batchSize {
			break
		}
	}
	return bw.Flush()
}

func (c *MemoryBackupContributor) streamTemporal(w io.Writer, characterID string, selectedIDs []string, total *int64) error {
	bw := bufio.NewWriter(w)
	batchSize := 1000
	offset := 0
	for {
		var mems []Memory
		var err error
		if len(selectedIDs) > 0 {
			mems, err = c.getRepository().StreamExportableByIDs(selectedIDs, batchSize, offset)
		} else {
			mems, err = c.getRepository().StreamExportable(characterID, batchSize, offset)
		}
		if err != nil {
			return err
		}
		if len(mems) == 0 {
			break
		}
		ids := make([]string, 0, len(mems))
		for _, m := range mems {
			ids = append(ids, m.ID)
		}
		temporals, err := c.getRepository().ListTemporalByMemoryIDs(ids)
		if err != nil {
			return err
		}
		for _, t := range temporals {
			line, err := json.Marshal(t)
			if err != nil {
				return err
			}
			if _, err := bw.Write(line); err != nil {
				return err
			}
			if err := bw.WriteByte('\n'); err != nil {
				return err
			}
			*total++
		}
		offset += len(mems)
		if len(mems) < batchSize {
			break
		}
	}
	return bw.Flush()
}

func (c *MemoryBackupContributor) streamDerivations(w io.Writer, characterID string, selectedIDs []string, total *int64) error {
	bw := bufio.NewWriter(w)
	batchSize := 1000
	offset := 0
	for {
		var mems []Memory
		var err error
		if len(selectedIDs) > 0 {
			mems, err = c.getRepository().StreamExportableByIDs(selectedIDs, batchSize, offset)
		} else {
			mems, err = c.getRepository().StreamExportable(characterID, batchSize, offset)
		}
		if err != nil {
			return err
		}
		if len(mems) == 0 {
			break
		}
		ids := make([]string, 0, len(mems))
		for _, m := range mems {
			ids = append(ids, m.ID)
		}
		derivations, err := c.getRepository().ListDerivationsByMemoryIDs(ids)
		if err != nil {
			return err
		}
		for _, d := range derivations {
			line, err := json.Marshal(d)
			if err != nil {
				return err
			}
			if _, err := bw.Write(line); err != nil {
				return err
			}
			if err := bw.WriteByte('\n'); err != nil {
				return err
			}
			*total++
		}
		offset += len(mems)
		if len(mems) < batchSize {
			break
		}
	}
	return bw.Flush()
}

func (c *MemoryBackupContributor) PreviewImport(ctx context.Context, req dataportability.ImportPreviewRequest, in dataportability.BackupReader) ([]dataportability.ImportComponentPreview, error) {
	preview := dataportability.ImportComponentPreview{
		ComponentID:  ComponentIDMemoryRecords,
		Kind:         dataportability.KindNDJSON,
		LogicalName:  "memory.records.v1",
		Collisions:   make([]dataportability.ComponentCollision, 0),
		Warnings:     make([]string, 0),
	}

	rc, err := in.ReadComponent(ComponentIDMemoryRecords + ".v1")
	if err != nil {
		return []dataportability.ImportComponentPreview{preview}, nil
	}
	defer rc.Close()

	type identityStats struct {
		seenIDs      map[string]bool
		characters   map[string]int
		types        map[string]int
		sensitivities map[string]int
	}
	stats := identityStats{
		seenIDs:       make(map[string]bool),
		characters:    make(map[string]int),
		types:         make(map[string]int),
		sensitivities: make(map[string]int),
	}

	scanner := bufio.NewScanner(rc)
	buf := make([]byte, 128*1024)
	scanner.Buffer(buf, RecordSizeLimit)
	for scanner.Scan() {
		line := scanner.Bytes()
		if len(line) == 0 {
			continue
		}
		preview.ItemCount++

		var rec MemoryRecordV1
		if err := json.Unmarshal(line, &rec); err != nil {
			preview.Warnings = append(preview.Warnings, fmt.Sprintf("record %d: decode error: %v", preview.ItemCount, err))
			continue
		}
		if !validateMemoryType(rec.MemoryType) {
			preview.Warnings = append(preview.Warnings, fmt.Sprintf("record %d: invalid memory type: %s", preview.ItemCount, rec.MemoryType))
		}
		if rec.CharacterID != "" {
			stats.characters[rec.CharacterID]++
		}
		stats.types[rec.MemoryType]++
		stats.sensitivities[rec.SensitivityLevel]++
		if rec.Version < 1 {
			preview.Warnings = append(preview.Warnings, fmt.Sprintf("record %d id=%s: invalid version %d", preview.ItemCount, rec.ID, rec.Version))
		}
	}

	preview.Warnings = append(preview.Warnings, fmt.Sprintf("typeDistribution: %v", stats.types))
	preview.Warnings = append(preview.Warnings, fmt.Sprintf("sensitivityDistribution: %v", stats.sensitivities))
	preview.Warnings = append(preview.Warnings, fmt.Sprintf("characterScopes: %d", len(stats.characters)))

	evtRC, err := in.ReadComponent(ComponentIDMemoryEvents + ".v1")
	if err == nil {
		defer evtRC.Close()
		evtScanner := bufio.NewScanner(evtRC)
		evtScanner.Buffer(buf, RecordSizeLimit)
		for evtScanner.Scan() {
			if len(evtScanner.Bytes()) == 0 {
				continue
			}
		}
	}

	derRC, err := in.ReadComponent(ComponentIDMemoryDerivations + ".v1")
	if err == nil {
		defer derRC.Close()
		derScanner := bufio.NewScanner(derRC)
		derScanner.Buffer(buf, RecordSizeLimit)
		for derScanner.Scan() {
			line := derScanner.Bytes()
			if len(line) == 0 {
				continue
			}
			var d MemoryDerivationV1
			if err := json.Unmarshal(line, &d); err == nil {
				if d.InputVersion < 1 {
					preview.Warnings = append(preview.Warnings, fmt.Sprintf("derivation %s: invalid input version %d", d.ID, d.InputVersion))
				}
			}
		}
	}

	return []dataportability.ImportComponentPreview{preview}, nil
}

func (c *MemoryBackupContributor) Import(ctx context.Context, req dataportability.ImportRequest, in dataportability.BackupReader) error {
	idMap := dataportability.NewImportIdentityMap()
	charPolicy := string(req.CharacterPolicy)

	recRC, err := in.ReadComponent(ComponentIDMemoryRecords + ".v1")
	if err != nil {
		return fmt.Errorf("import: records component missing: %w", err)
	}
	defer recRC.Close()

	type importStats struct {
		imported      int
		deduped       int
		remapped      int
		collisions    int
	}
	stats := importStats{}

	charIDMap := make(map[string]string)
	for oldCID := range c.extractTargetCharacters(req) {
		newCID := oldCID
		if charPolicy == string(dataportability.CollisionReplace) {
			newCID = oldCID
		}
		charIDMap[oldCID] = newCID
		idMap.AddCharacter(oldCID, newCID)
	}

	scanner := bufio.NewScanner(recRC)
	buf := make([]byte, 128*1024)
	scanner.Buffer(buf, RecordSizeLimit)
	for scanner.Scan() {
		line := scanner.Bytes()
		if len(line) == 0 {
			continue
		}

		var rec MemoryRecordV1
		if err := json.Unmarshal(line, &rec); err != nil {
			return fmt.Errorf("import: decode memory record: %w", err)
		}
		if err := normalizeMemoryRecord(&rec); err != nil {
			return fmt.Errorf("import: normalize record %s: %w", rec.ID, err)
		}

		targetChar := rec.CharacterID
		if newChar, ok := charIDMap[rec.CharacterID]; ok {
			targetChar = newChar
		} else if newChar, ok := idMap.GetCharacter(rec.CharacterID); ok {
			targetChar = newChar
		}

		isNew, err := c.getRepository().IsNewID(rec.ID)
		if err != nil {
			return fmt.Errorf("import: check id %s: %w", rec.ID, err)
		}

		newID := rec.ID
		if !isNew {
			existing, err := c.getRepository().FindByID(rec.ID)
			if err != nil {
				return fmt.Errorf("import: fetch existing %s: %w", rec.ID, err)
			}
			if existing != nil && sameMemorySnapshot(existing, &rec) {
				stats.deduped++
				stats.imported++
				continue
			}
			newID = uuid.New().String()
			stats.remapped++
			idMap.AddMemory(rec.ID, newID)
		} else {
			stats.imported++
		}

		memory := rec.toMemory()
		memory.CharacterID = targetChar
		memory.ID = newID
		memory.SourceMsgID = idMap.RemapMessageRef(rec.SourceMessageID)
		memory.SourceConvID = idMap.RemapConversationRef(rec.SourceConversationID)

		if err := c.getRepository().Create(&memory); err != nil {
			return fmt.Errorf("import: create memory %s: %w", newID, err)
		}

		evtRC, err := in.ReadComponent(ComponentIDMemoryEvents + ".v1")
		if err == nil {
			c.importEvents(evtRC, rec.ID, newID, idMap)
			evtRC.Close()
		}
	}
	_ = stats
	return nil
}

func (c *MemoryBackupContributor) importEvents(r io.Reader, oldID, newID string, idMap *dataportability.ImportIdentityMap) {
	buf := make([]byte, 128*1024)
	scanner := bufio.NewScanner(r)
	scanner.Buffer(buf, RecordSizeLimit)
	var events []MemoryEventV1
	for scanner.Scan() {
		line := scanner.Bytes()
		if len(line) == 0 {
			continue
		}
		var e MemoryEventV1
		if err := json.Unmarshal(line, &e); err != nil {
			continue
		}
		if e.MemoryID != oldID {
			continue
		}
		e.ID = uuid.New().String()
		e.MemoryID = newID
		events = append(events, e)
	}
	if len(events) > 0 {
		_ = c.getRepository().BatchUpsert(events)
	}
}

func (c *MemoryBackupContributor) extractTargetCharacters(req dataportability.ImportRequest) map[string]bool {
	result := make(map[string]bool)
	if req.DefaultCharacterID != "" {
		result[req.DefaultCharacterID] = true
	}
	return result
}

func (c *MemoryBackupContributor) getRepository() Repository {
	svc := c.svc
	if repo, ok := svc.(*service); ok {
		return repo.repo
	}
	return nil
}

func validateMemoryType(mt string) bool {
	switch strings.ToLower(mt) {
	case "fact", "profile", "episodic", "working", "worldbook", "graph", "custom", "preference", "habit", "relationship", "nickname", "personal_info", "event", "scene":
		return true
	default:
		return false
	}
}

func normalizeMemoryRecord(rec *MemoryRecordV1) error {
	if rec.Key == "" {
		return fmt.Errorf("key is required")
	}
	if rec.Value == "" {
		return fmt.Errorf("value is required")
	}
	if rec.MemoryType == "" {
		rec.MemoryType = "custom"
	}
	if rec.Version < 1 {
		rec.Version = 1
	}
	if rec.SensitivityLevel == "" {
		rec.SensitivityLevel = "internal"
	}
	if rec.VerifiedStatus == "" {
		rec.VerifiedStatus = "unverified"
	}
	if rec.Importance < 0 {
		rec.Importance = 0
	}
	if rec.Confidence < 0 || rec.Confidence > 100 {
		rec.Confidence = 50
	}
	return nil
}

func sameMemorySnapshot(m *Memory, rec *MemoryRecordV1) bool {
	if m == nil || rec == nil {
		return false
	}
	return m.Key == rec.Key && m.Value == rec.Value && m.MemoryType == rec.MemoryType && m.Importance == rec.Importance && m.SensitivityLevel == rec.SensitivityLevel
}

func (m *Memory) toV1() MemoryRecordV1 {
	layer := "fact"
	switch strings.ToLower(m.MemoryType) {
	case "profile", "user_profile", "personal_info", "hobby", "preference", "habit", "relationship", "nickname":
		layer = "profile"
	case "episodic", "episode", "event", "moment", "scene":
		layer = "episodic"
	case "working_memory", "working", "summary", "current_summary":
		layer = "working"
	case "worldbook", "world", "world_info":
		layer = "worldbook"
	case "graph", "node", "edge":
		layer = "graph"
	}
	return MemoryRecordV1{
		ID:                    m.ID,
		Key:                   m.Key,
		Value:                 m.Value,
		MemoryLayer:           layer,
		MemoryType:            m.MemoryType,
		Importance:            m.Importance,
		Confidence:            m.Confidence,
		Source:                m.Source,
		Scope:                 m.Scope,
		CharacterID:           m.CharacterID,
		EntityID:              m.EntityID,
		EntityType:            m.EntityType,
		SourceMessageID:       m.SourceMsgID,
		SourceConversationID:  m.SourceConvID,
		VerifiedStatus:        m.VerifiedStatus,
		LastVerifiedAt:        m.LastVerifiedAt,
		ExpiresAt:             m.ExpiresAt,
		SensitivityLevel:      m.SensitivityLevel,
		AllowProactiveMention: m.AllowProactiveMention,
		RequiresConfirmation:  m.RequiresConfirmation,
		Version:               m.Version,
		CreatedAt:             m.CreatedAt,
		UpdatedAt:             m.UpdatedAt,
		UseCount:              m.UseCount,
		LastUsedAt:            m.LastUsedAt,
		DerivationKey:         m.DerivationKey,
	}
}

func (r MemoryRecordV1) toMemory() Memory {
	return Memory{
		ID:                    r.ID,
		CharacterID:           r.CharacterID,
		MemoryType:            r.MemoryType,
		Source:                r.Source,
		Scope:                 r.Scope,
		Key:                   r.Key,
		Value:                 r.Value,
		Importance:            r.Importance,
		Confidence:            r.Confidence,
		ExpiresAt:             r.ExpiresAt,
		EntityID:              r.EntityID,
		EntityType:            r.EntityType,
		SourceMsgID:           r.SourceMessageID,
		SourceConvID:          r.SourceConversationID,
		VerifiedStatus:        r.VerifiedStatus,
		LastVerifiedAt:        r.LastVerifiedAt,
		UseCount:              r.UseCount,
		LastUsedAt:            r.LastUsedAt,
		SensitivityLevel:      r.SensitivityLevel,
		AllowProactiveMention: r.AllowProactiveMention,
		RequiresConfirmation:  r.RequiresConfirmation,
		Version:               r.Version,
		DerivationKey:         r.DerivationKey,
		CreatedAt:             r.CreatedAt,
		UpdatedAt:             r.UpdatedAt,
	}
}
