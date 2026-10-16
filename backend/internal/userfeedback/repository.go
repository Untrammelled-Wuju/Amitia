// SPDX-FileCopyrightText: 2026 彭旭
// SPDX-License-Identifier: AGPL-3.0-only
package userfeedback

import (
	"gorm.io/gorm"
)

type Repository interface {
	Create(ticket *Ticket) error
	Update(ticket *Ticket) error
	GetByID(spaceID, id string) (*Ticket, error)
	Get(id string) (*Ticket, error)
	ListBySpace(spaceID string, limit int) ([]Ticket, error)
	ListPending(limit int) ([]Ticket, error)
}

type repository struct {
	db *gorm.DB
}

func NewRepository(db *gorm.DB) Repository {
	return &repository{db: db}
}

func (r *repository) Create(ticket *Ticket) error {
	return r.db.Create(ticket).Error
}

func (r *repository) Update(ticket *Ticket) error {
	return r.db.Model(&Ticket{}).Where("id = ?", ticket.ID).Select("*").Omit("id").Updates(ticket).Error
}

func (r *repository) GetByID(spaceID, id string) (*Ticket, error) {
	var ticket Ticket
	if err := r.db.Where("id = ? AND space_id = ?", id, spaceID).Take(&ticket).Error; err != nil {
		return nil, err
	}
	return &ticket, nil
}

func (r *repository) Get(id string) (*Ticket, error) {
	var ticket Ticket
	if err := r.db.Where("id = ?", id).Take(&ticket).Error; err != nil {
		return nil, err
	}
	return &ticket, nil
}

func (r *repository) ListBySpace(spaceID string, limit int) ([]Ticket, error) {
	items := []Ticket{}
	err := r.db.Where("space_id = ?", spaceID).Order("created_at DESC").Limit(limit).Find(&items).Error
	return items, err
}

func (r *repository) ListPending(limit int) ([]Ticket, error) {
	items := []Ticket{}
	err := r.db.Where("status IN ? AND attempts < ?", []string{StatusPending, StatusFailed}, maxUploadAttempts).
		Order("created_at ASC").Limit(limit).Find(&items).Error
	return items, err
}
