package migration

import (
	_ "embed"
	"fmt"
	"time"

	"gorm.io/gorm"
)

//go:embed baseline.sql
var baselineSQL string

func ApplyBaseline(db *gorm.DB) error {
	if err := ApplyInitialSQL(db, baselineSQL); err != nil {
		return err
	}
	if err := ensureOptionalBaselineIndexes(db); err != nil {
		return err
	}
	return applyDesktopPetCatalogBaseline(db)
}

func ensureOptionalBaselineIndexes(db *gorm.DB) error {
	type columnRow struct {
		Name string `gorm:"column:name"`
	}
	hasColumn := func(table, column string) (bool, error) {
		var rows []columnRow
		if err := db.Raw("PRAGMA table_info(" + table + ")").Scan(&rows).Error; err != nil {
			return false, err
		}
		for _, row := range rows {
			if row.Name == column {
				return true, nil
			}
		}
		return false, nil
	}
	hasColumns := func(table string, columns ...string) (bool, error) {
		for _, column := range columns {
			exists, err := hasColumn(table, column)
			if err != nil {
				return false, err
			}
			if !exists {
				return false, nil
			}
		}
		return true, nil
	}
	projectColumn, err := hasColumn("conversations", "project_id")
	if err != nil {
		return err
	}
	if projectColumn {
		if err := db.Exec("CREATE INDEX IF NOT EXISTS idx_conversations_project_updated ON conversations(project_id, updated_at)").Error; err != nil {
			return err
		}
	}
	messageCharacterColumn, err := hasColumn("messages", "character_id")
	if err != nil {
		return err
	}
	if messageCharacterColumn {
		if err := db.Exec("CREATE INDEX IF NOT EXISTS idx_messages_character_id ON messages(character_id)").Error; err != nil {
			return err
		}
	}
	conversationPinnedColumn, err := hasColumn("conversations", "pinned_at")
	if err != nil {
		return err
	}
	conversationArchivedColumn, err := hasColumn("conversations", "archived_at")
	if err != nil {
		return err
	}
	if conversationPinnedColumn && conversationArchivedColumn {
		if err := db.Exec("CREATE INDEX IF NOT EXISTS idx_conversations_sidebar_pinned ON conversations(space_id, pinned_at, updated_at)").Error; err != nil {
			return err
		}
		if err := db.Exec("CREATE INDEX IF NOT EXISTS idx_conversations_sidebar_archived ON conversations(space_id, archived_at, updated_at)").Error; err != nil {
			return err
		}
	}
	projectPinnedColumn, err := hasColumn("projects", "pinned_at")
	if err != nil {
		return err
	}
	if projectPinnedColumn {
		if err := db.Exec("CREATE INDEX IF NOT EXISTS idx_projects_sidebar_pinned ON projects(space_id, pinned_at, updated_at)").Error; err != nil {
			return err
		}
	}
	conversationWorkspaceColumns, err := hasColumns("conversations", "workspace_id")
	if err != nil {
		return err
	}
	if conversationWorkspaceColumns {
		if err := db.Exec("CREATE INDEX IF NOT EXISTS idx_conversations_workspace ON conversations(workspace_id)").Error; err != nil {
			return err
		}
	}
	assistantTurnRuntimeColumns, err := hasColumns("assistant_turns", "execution_id", "parent_turn_id", "agent_id")
	if err != nil {
		return err
	}
	if assistantTurnRuntimeColumns {
		if err := db.Exec("CREATE INDEX IF NOT EXISTS idx_assistant_turns_execution ON assistant_turns(execution_id)").Error; err != nil {
			return err
		}
		if err := db.Exec("CREATE INDEX IF NOT EXISTS idx_assistant_turns_parent ON assistant_turns(parent_turn_id)").Error; err != nil {
			return err
		}
		if err := db.Exec("CREATE INDEX IF NOT EXISTS idx_assistant_turns_agent ON assistant_turns(agent_id)").Error; err != nil {
			return err
		}
	}
	return nil
}

func MarkAllMigrationsApplied(db *gorm.DB, migrations []Migration) error {
	runner := Runner{DB: db}
	if err := runner.EnsureTable(); err != nil {
		return err
	}
	now := time.Now().Format(time.RFC3339)
	for _, migration := range migrations {
		checksum, err := runner.computeMigrationChecksum(migration)
		if err != nil {
			return fmt.Errorf("compute checksum for %s: %w", migration.Version, err)
		}
		record := Record{
			Version:    migration.Version,
			Name:       migration.Name,
			Checksum:   checksum,
			Status:     "applied",
			StartedAt:  now,
			FinishedAt: now,
		}
		if err := db.Where("version = ?", migration.Version).FirstOrCreate(&record).Error; err != nil {
			return fmt.Errorf("mark migration %s applied: %w", migration.Version, err)
		}
	}
	return nil
}

func IsNewDatabase(db *gorm.DB) (bool, error) {
	var count int64
	if err := db.Raw("SELECT COUNT(*) FROM sqlite_master WHERE type = 'table' AND name NOT LIKE 'sqlite_%'").Scan(&count).Error; err != nil {
		return false, err
	}
	return count == 0, nil
}

func HasCoreSchema(db *gorm.DB) (bool, error) {
	for _, table := range []string{"characters", "conversations", "messages"} {
		var count int64
		if err := db.Raw("SELECT COUNT(*) FROM sqlite_master WHERE type = 'table' AND name = ?", table).Scan(&count).Error; err != nil {
			return false, err
		}
		if count > 0 {
			return true, nil
		}
	}
	return false, nil
}
