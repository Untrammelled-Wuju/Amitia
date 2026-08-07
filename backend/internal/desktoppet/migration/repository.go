// SPDX-FileCopyrightText: 2026 彭旭
// SPDX-License-Identifier: AGPL-3.0-only
package migration

import (
	"fmt"
	"gorm.io/gorm"
)

type DBRepository struct {
	db *gorm.DB
}

func NewDBRepository(db *gorm.DB) *DBRepository {
	return &DBRepository{db: db}
}

func (r *DBRepository) DB() *gorm.DB { return r.db }

func (r *DBRepository) GetOperation(id string) (*MigrationOperation, error) {
	var op MigrationOperation
	return &op, fmt.Errorf("not implemented")
}

func (r *DBRepository) LegacyInstallationWriteDisabled() bool {
	return false
}

func (r *DBRepository) LegacyEditingWriteDisabled() bool {
	return false
}
