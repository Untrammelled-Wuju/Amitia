package migration

import "gorm.io/gorm"

func DesktopPetCatalogRepairMigration() Migration {
	return Migration{
		Version: "20260910001",
		Name:    "repair_desktop_pet_action_catalog",
		Up: func(s *Step) error {
			populateCatalogProjections(s)
			return nil
		},
	}
}

func applyDesktopPetCatalogBaseline(db *gorm.DB) error {
	return db.Transaction(func(tx *gorm.DB) error {
		step := &Step{db: tx}
		populateCatalogProjections(step)
		if step.err != nil {
			return step.err
		}
		for _, command := range step.commands {
			if err := tx.Exec(command).Error; err != nil {
				return err
			}
		}
		return nil
	})
}
