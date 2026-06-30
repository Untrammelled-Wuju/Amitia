// SPDX-FileCopyrightText: 2026 彭旭
// SPDX-License-Identifier: AGPL-3.0-only
package app

import (
	"context"

	"gorm.io/gorm"
)

type AppContext struct {
	DB      *gorm.DB
	Context context.Context
}

func NewAppContext(db *gorm.DB, _ interface{}) *AppContext {
	return &AppContext{
		DB:      db,
		Context: context.Background(),
	}
}
