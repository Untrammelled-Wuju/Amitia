package main

import (
	"fmt"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func main() {
	db, err := gorm.Open(sqlite.Open("data/app.db"), &gorm.Config{})
	if err != nil {
		fmt.Println("Error:", err)
		return
	}

	tables := []string{
		"quality_evaluations",
		"quality_findings",
		"quality_scores",
		"quality_problem_frames",
		"quality_profiles",
	}

	for _, t := range tables {
		var count int64
		db.Raw("SELECT count(*) FROM sqlite_master WHERE type='table' AND name=?", t).Scan(&count)
		if count > 0 {
			var rowCount int64
			db.Raw("SELECT count(*) FROM " + t).Scan(&rowCount)
			fmt.Printf("✓ Table %-25s exists (rows: %d)\n", t, rowCount)
		} else {
			fmt.Printf("✗ Table %-25s MISSING\n", t)
		}
	}

	var migCount int64
	db.Raw("SELECT count(*) FROM schema_migrations WHERE version LIKE '20260730%'").Scan(&migCount)
	fmt.Printf("\nQuality migrations applied: %d\n", migCount)

	var allMigs []struct {
		Version  string
		Checksum string
	}
	db.Raw("SELECT version, checksum FROM schema_migrations WHERE version LIKE '%quality%' OR version LIKE '20260730%' ORDER BY version").Scan(&allMigs)
	fmt.Println("\nQuality-related migrations:")
	for _, m := range allMigs {
		fmt.Printf("  - %s (checksum: %s...)\n", m.Version, m.Checksum[:min(16, len(m.Checksum))])
	}
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
