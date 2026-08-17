// SPDX-FileCopyrightText: 2026 彭旭
// SPDX-License-Identifier: AGPL-3.0-only
package sync

import (
	"gorm.io/gorm"
)

type EntityMutationApplier interface {
	Apply(tx *gorm.DB, mutation ClientMutation) (int64, error)
	Supports(entityType EntityType) bool
}

type CompositeApplier struct {
	appliers []EntityMutationApplier
}

func NewCompositeApplier(appliers ...EntityMutationApplier) *CompositeApplier {
	return &CompositeApplier{appliers: appliers}
}

func (c *CompositeApplier) Apply(tx *gorm.DB, mutation ClientMutation) (int64, error) {
	for _, a := range c.appliers {
		if a.Supports(mutation.EntityType) {
			return a.Apply(tx, mutation)
		}
	}
	return 0, &ApplierError{
		Code:    "unsupported_entity_type",
		Message: "no applier for entity type: " + string(mutation.EntityType),
	}
}

func (c *CompositeApplier) Supports(entityType EntityType) bool {
	for _, a := range c.appliers {
		if a.Supports(entityType) {
			return true
		}
	}
	return false
}

type ApplierError struct {
	Code            string
	Message         string
	ServerRevision  int64
}

func (e *ApplierError) Error() string {
	return e.Code + ": " + e.Message
}
