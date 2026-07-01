package memory

import (
	"fmt"
	"time"

	"gorm.io/gorm"
)

type ProspectiveMemory struct {
	ID       string    `json:"id"`
	Title    string    `json:"title"`
	Content  string    `json:"content"`
	RemindAt time.Time `json:"remindAt"`
	Status   string    `json:"status"`
}

type ProspectiveMemoryService struct {
	db *gorm.DB
}

func NewProspectiveMemoryService(db *gorm.DB) *ProspectiveMemoryService {
	return &ProspectiveMemoryService{db: db}
}

func (s *ProspectiveMemoryService) CheckDue(characterID string) ([]ProspectiveMemory, error) {
	var results []ProspectiveMemory
	if err := s.db.Table("prospective_memories").
		Where("character_id = ? AND status = ? AND remind_at <= ?", characterID, "pending", time.Now()).
		Find(&results).Error; err != nil {
		return nil, fmt.Errorf("check due: %w", err)
	}
	if results == nil {
		results = []ProspectiveMemory{}
	}
	return results, nil
}

func (s *ProspectiveMemoryService) MarkTriggered(id string) (bool, error) {
	result := s.db.Table("prospective_memories").
		Where("id = ? AND status = ?", id, "pending").
		Update("status", "triggered")
	if result.Error != nil {
		return false, fmt.Errorf("mark triggered: %w", result.Error)
	}
	return result.RowsAffected > 0, nil
}

func (s *ProspectiveMemoryService) ListDue(characterID string, maxItems int) ([]ProspectiveMemory, error) {
	var results []ProspectiveMemory
	if err := s.db.Table("prospective_memories").
		Where("character_id = ? AND status = ?", characterID, "pending").
		Order("remind_at ASC").Limit(maxItems).Find(&results).Error; err != nil {
		return nil, fmt.Errorf("list due: %w", err)
	}
	if results == nil {
		results = []ProspectiveMemory{}
	}
	return results, nil
}
