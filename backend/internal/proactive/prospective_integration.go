package proactive

import (
	"log"
	"time"

	"github.com/u-ai/backend/internal/memory"
	"gorm.io/gorm"
)

type ProspectiveIntegrator struct {
	db             *gorm.DB
	prospectiveSvc *memory.ProspectiveMemoryService
	checkInterval  time.Duration
	stopCh         chan struct{}
}

func NewProspectiveIntegrator(db *gorm.DB, checkInterval time.Duration) *ProspectiveIntegrator {
	if checkInterval <= 0 {
		checkInterval = 30 * time.Second
	}
	return &ProspectiveIntegrator{
		db:             db,
		prospectiveSvc: memory.NewProspectiveMemoryService(db),
		checkInterval:  checkInterval,
		stopCh:         make(chan struct{}),
	}
}

func (pi *ProspectiveIntegrator) Start() {
	go pi.loop()
	log.Printf("[ProspectiveIntegrator] started with interval=%v", pi.checkInterval)
}

func (pi *ProspectiveIntegrator) Stop() {
	close(pi.stopCh)
	log.Println("[ProspectiveIntegrator] stopped")
}

func (pi *ProspectiveIntegrator) loop() {
	ticker := time.NewTicker(pi.checkInterval)
	defer ticker.Stop()
	for {
		select {
		case <-ticker.C:
			pi.checkAndTrigger()
		case <-pi.stopCh:
			return
		}
	}
}

func (pi *ProspectiveIntegrator) checkAndTrigger() {
	var chars []string
	if err := pi.db.Table("characters").Select("id").Where("is_active = 1 OR is_default = 1").Pluck("id", &chars).Error; err != nil {
		log.Printf("[ProspectiveIntegrator] failed to list characters: %v", err)
		return
	}

	now := time.Now()
	for _, charID := range chars {
		due, err := pi.prospectiveSvc.CheckDue(charID)
		if err != nil {
			log.Printf("[ProspectiveIntegrator] check error char=%s: %v", charID, err)
			continue
		}
		for _, m := range due {
			pi.triggerDueMemory(charID, m)
		}
		_ = now
	}
}

func (pi *ProspectiveIntegrator) triggerDueMemory(characterID string, pm memory.ProspectiveMemory) {
	correlationID := GenerateCorrelationID(characterID, "prospective:"+pm.ID, pm.Title)
	if GlobalDedupManager.HasSentAnyChannel(correlationID) {
		return
	}

	lease := GlobalLeaseManager.AcquireLease(
		PriorityHigh,
		characterID,
		"",
		"all",
		correlationID,
		2*time.Minute,
	)

	go func() {
		defer GlobalLeaseManager.ReleaseLease(lease.ID)

		if !lease.IsValid(time.Now()) {
			return
		}

		convID := pi.resolveConversation(characterID)
		if convID == "" {
			log.Printf("[ProspectiveIntegrator] no conversation for char=%s", characterID)
			return
		}

		content := pm.Content
		if content == "" {
			content = pm.Title
		}

		channels := []string{"web"}
		for _, ch := range channels {
			if GlobalDedupManager.IsDuplicate(correlationID, ch) {
				continue
			}
			record := GlobalDedupManager.RecordDelivery(correlationID, characterID, convID, ch, content)
			_ = record
		}

		_, err := pi.prospectiveSvc.MarkTriggered(pm.ID)
		if err != nil {
			log.Printf("[ProspectiveIntegrator] mark triggered error id=%s: %v", pm.ID, err)
			return
		}
		log.Printf("[ProspectiveIntegrator] triggered memory id=%s title=%s char=%s", pm.ID, pm.Title, characterID)
	}()
}

func (pi *ProspectiveIntegrator) resolveConversation(characterID string) string {
	var convID string
	err := pi.db.Table("conversations").
		Select("id").
		Where("character_id = ?", characterID).
		Order("updated_at DESC").
		Limit(1).Row().Scan(&convID)
	if err != nil {
		return ""
	}
	return convID
}

func (pi *ProspectiveIntegrator) CountDue(characterID string) int {
	due, err := pi.prospectiveSvc.CheckDue(characterID)
	if err != nil {
		return 0
	}
	count := 0
	now := time.Now()
	for _, m := range due {
		correlationID := GenerateCorrelationID(characterID, "prospective:"+m.ID, m.Title)
		if !GlobalDedupManager.HasSentAnyChannel(correlationID) {
			count++
		}
	}
	_ = now
	return count
}

func (pi *ProspectiveIntegrator) ProduceActiveItems(characterID string, maxItems int) []memory.ProspectiveMemory {
	due, err := pi.prospectiveSvc.ListDue(characterID, maxItems)
	if err != nil {
		return nil
	}
	var pending []memory.ProspectiveMemory
	for _, m := range due {
		correlationID := GenerateCorrelationID(characterID, "prospective:"+m.ID, m.Title)
		if !GlobalDedupManager.HasSentAnyChannel(correlationID) {
			pending = append(pending, m)
		}
	}
	if pending == nil {
		pending = []memory.ProspectiveMemory{}
	}
	return pending
}
