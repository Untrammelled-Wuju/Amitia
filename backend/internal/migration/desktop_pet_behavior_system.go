package migration

import (
	"github.com/u-ai/backend/internal/desktoppet/behavior/persistence"
)

func DesktopPetBehaviorMigration() Migration {
	return Migration{
		Version: "202607310008",
		Name:    "add_desktop_pet_behavior_system",
		Up: func(s *Step) error {
			for _, sql := range persistence.DesktopPetBehaviorTableSQL {
				s.CreateTable(sql)
			}
			for _, idx := range persistence.DesktopPetBehaviorIndexDefs {
				if err := s.CreateIndex(idx.Name, idx.Table, idx.Columns, idx.Unique); err != nil {
					return err
				}
			}
			return nil
		},
	}
}
