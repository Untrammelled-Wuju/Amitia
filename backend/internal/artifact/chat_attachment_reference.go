package artifact

import (
	"context"
	"database/sql"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"
)

const (
	ChatAttachmentPendingReferenceType = "message_attachment_pending"
	ChatAttachmentMessageReferenceType = "message_attachment"
	ChatAttachmentBindingReferenceType = "chat_attachment_binding"
)

type ChatAttachmentReferenceState string

const (
	ChatAttachmentReferencePending   ChatAttachmentReferenceState = "pending"
	ChatAttachmentReferenceConfirmed ChatAttachmentReferenceState = "confirmed"
)

type ChatAttachmentReferenceBindingRecord struct {
	Artifact  Artifact
	Token     string
	CreatedAt time.Time
	State     ChatAttachmentReferenceState
}

func (s *Service) PrepareChatAttachmentReference(ctx context.Context, id ID, token, messageID string) error {
	if err := validateChatAttachmentReferenceInput(id, token, messageID); err != nil {
		return err
	}
	art, err := s.repo.GetByID(id)
	if err != nil {
		return fmt.Errorf("artifact: read chat attachment before prepare: %w", err)
	}
	if art.Status != StatusReady || art.Source != SourceChatUpload {
		return fmt.Errorf("artifact: chat attachment is not ready: %w", ErrInvalidReference("artifact is not a ready chat upload"))
	}
	sqlDB, err := s.repo.SqlDB()
	if err != nil {
		return ErrMetadataWriteFailed(err)
	}
	tx, err := sqlDB.BeginTx(ctx, nil)
	if err != nil {
		return ErrMetadataWriteFailed(err)
	}
	defer tx.Rollback()
	if err := s.repo.InsertReferenceSqlTx(tx, &ArtifactReference{
		ArtifactID:    id,
		ReferenceType: ChatAttachmentPendingReferenceType,
		ReferenceID:   token,
		CreatedAt:     time.Now().UTC(),
	}); err != nil {
		return ErrMetadataWriteFailed(err)
	}
	if err := tx.Commit(); err != nil {
		return ErrMetadataWriteFailed(err)
	}
	return nil
}

func (s *Service) CommitChatAttachmentReference(ctx context.Context, id ID, token, messageID string) error {
	if err := validateChatAttachmentReferenceInput(id, token, messageID); err != nil {
		return err
	}
	sqlDB, err := s.repo.SqlDB()
	if err != nil {
		return ErrMetadataWriteFailed(err)
	}
	tx, err := sqlDB.BeginTx(ctx, nil)
	if err != nil {
		return ErrMetadataWriteFailed(err)
	}
	defer tx.Rollback()
	var committed int
	if err := tx.QueryRowContext(ctx, `SELECT COUNT(*) FROM artifact_references WHERE artifact_id=? AND reference_type=? AND reference_id=?`, id, ChatAttachmentMessageReferenceType, messageID).Scan(&committed); err != nil {
		return ErrMetadataWriteFailed(err)
	}
	if committed == 0 {
		var pending int
		if err := tx.QueryRowContext(ctx, `SELECT COUNT(*) FROM artifact_references WHERE artifact_id=? AND reference_type=? AND reference_id=?`, id, ChatAttachmentPendingReferenceType, token).Scan(&pending); err != nil {
			return ErrMetadataWriteFailed(err)
		}
		if pending != 1 {
			return fmt.Errorf("artifact: pending chat attachment reference is missing: %w", ErrInvalidReference("pending chat attachment reference is missing"))
		}
	}
	if err := s.repo.RemoveReferenceSqlTx(tx, id, ChatAttachmentPendingReferenceType, token); err != nil {
		return ErrMetadataWriteFailed(err)
	}
	if committed == 0 {
		if _, err := tx.ExecContext(ctx, `INSERT INTO artifact_references(artifact_id,reference_type,reference_id,created_at) VALUES(?,?,?,?)`, id, ChatAttachmentMessageReferenceType, messageID, time.Now().UTC()); err != nil {
			return ErrMetadataWriteFailed(err)
		}
	}
	if _, err := tx.ExecContext(ctx, `INSERT OR IGNORE INTO artifact_references(artifact_id,reference_type,reference_id,created_at) VALUES(?,?,?,?)`, id, ChatAttachmentBindingReferenceType, token, time.Now().UTC()); err != nil {
		return ErrMetadataWriteFailed(err)
	}
	if err := tx.Commit(); err != nil {
		return ErrMetadataWriteFailed(err)
	}
	return nil
}

func (s *Service) AbortChatAttachmentReference(ctx context.Context, id ID, token string) error {
	if id == "" || strings.TrimSpace(token) == "" {
		return ErrInvalidReference("chat attachment abort is incomplete")
	}
	sqlDB, err := s.repo.SqlDB()
	if err != nil {
		return ErrMetadataWriteFailed(err)
	}
	tx, err := sqlDB.BeginTx(ctx, nil)
	if err != nil {
		return ErrMetadataWriteFailed(err)
	}
	defer tx.Rollback()
	if err := s.repo.RemoveReferenceSqlTx(tx, id, ChatAttachmentPendingReferenceType, token); err != nil {
		return ErrMetadataWriteFailed(err)
	}
	if err := tx.Commit(); err != nil {
		return ErrMetadataWriteFailed(err)
	}
	return nil
}

func (s *Service) RemoveChatAttachmentReference(ctx context.Context, id ID, token, messageID string) error {
	if id == "" || strings.TrimSpace(token) == "" || strings.TrimSpace(messageID) == "" {
		return ErrInvalidReference("chat attachment reference identity is incomplete")
	}
	sqlDB, err := s.repo.SqlDB()
	if err != nil {
		return ErrMetadataWriteFailed(err)
	}
	tx, err := sqlDB.BeginTx(ctx, nil)
	if err != nil {
		return ErrMetadataWriteFailed(err)
	}
	defer tx.Rollback()
	if err := s.repo.RemoveReferenceSqlTx(tx, id, ChatAttachmentMessageReferenceType, messageID); err != nil {
		return ErrMetadataWriteFailed(err)
	}
	rows, err := tx.QueryContext(ctx, `SELECT reference_type,reference_id FROM artifact_references WHERE artifact_id=? AND reference_type IN (?, ?)`, id, ChatAttachmentPendingReferenceType, ChatAttachmentBindingReferenceType)
	if err != nil {
		return ErrMetadataWriteFailed(err)
	}
	tokens := make([]struct {
		referenceType string
		token         string
	}, 0)
	for rows.Next() {
		var referenceType, candidate string
		if err := rows.Scan(&referenceType, &candidate); err != nil {
			rows.Close()
			return ErrMetadataWriteFailed(err)
		}
		if chatAttachmentBindingMessageID(candidate) == messageID {
			tokens = append(tokens, struct {
				referenceType string
				token         string
			}{referenceType: referenceType, token: candidate})
		}
	}
	if err := rows.Err(); err != nil {
		rows.Close()
		return ErrMetadataWriteFailed(err)
	}
	if err := rows.Close(); err != nil {
		return ErrMetadataWriteFailed(err)
	}
	for _, row := range tokens {
		if err := s.repo.RemoveReferenceSqlTx(tx, id, row.referenceType, row.token); err != nil {
			return ErrMetadataWriteFailed(err)
		}
	}
	if err := tx.Commit(); err != nil {
		return ErrMetadataWriteFailed(err)
	}
	return nil
}

func chatAttachmentBindingMessageID(token string) string {
	payload, err := base64.RawURLEncoding.DecodeString(strings.TrimSpace(token))
	if err != nil {
		return ""
	}
	var binding struct {
		MessageID string `json:"messageId"`
	}
	if json.Unmarshal(payload, &binding) != nil {
		return ""
	}
	return binding.MessageID
}

func (s *Service) ListChatAttachmentReferenceBindings(ctx context.Context, limit int) ([]ChatAttachmentReferenceBindingRecord, error) {
	if limit < 1 {
		return nil, ErrInvalidReference("chat attachment binding limit must be positive")
	}
	if limit > 4096 {
		limit = 4096
	}
	rows, err := s.repo.DB().WithContext(ctx).Raw(
		`SELECT artifact_id,reference_type,reference_id,created_at FROM artifact_references WHERE reference_type IN (?, ?) ORDER BY created_at ASC, artifact_id ASC LIMIT ?`,
		ChatAttachmentPendingReferenceType,
		ChatAttachmentBindingReferenceType,
		limit,
	).Rows()
	if err != nil {
		return nil, fmt.Errorf("artifact: list chat attachment references: %w", err)
	}
	defer rows.Close()
	result := make([]ChatAttachmentReferenceBindingRecord, 0)
	for rows.Next() {
		var artifactID ID
		var referenceType string
		var token string
		var createdAt time.Time
		if err := rows.Scan(&artifactID, &referenceType, &token, &createdAt); err != nil {
			return nil, fmt.Errorf("artifact: scan chat attachment reference: %w", err)
		}
		art, err := s.repo.GetByID(artifactID)
		if err != nil {
			if errors.Is(err, sql.ErrNoRows) {
				return nil, fmt.Errorf("artifact: chat attachment %s metadata is missing: %w", artifactID, err)
			}
			return nil, fmt.Errorf("artifact: read chat attachment %s: %w", artifactID, err)
		}
		if art.Source == SourceChatUpload {
			owner, storage, err := s.chatUploadLocations(ctx, artifactID)
			if err != nil {
				return nil, err
			}
			art.OwnerDeviceID, art.StorageDeviceID = owner, storage
		}
		state := ChatAttachmentReferenceConfirmed
		if referenceType == ChatAttachmentPendingReferenceType {
			state = ChatAttachmentReferencePending
		}
		result = append(result, ChatAttachmentReferenceBindingRecord{Artifact: *art, Token: token, CreatedAt: createdAt, State: state})
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("artifact: iterate chat attachment references: %w", err)
	}
	return result, nil
}

func validateChatAttachmentReferenceInput(id ID, token, messageID string) error {
	if id == "" || strings.TrimSpace(token) == "" || len(token) > 16<<10 || strings.TrimSpace(messageID) == "" || len(messageID) > 512 || strings.ContainsAny(messageID, "\x00\r\n") {
		return ErrInvalidReference("chat attachment reference identity is invalid")
	}
	return nil
}
