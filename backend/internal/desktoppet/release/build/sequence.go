package build

import (
	"context"
	"fmt"
	"time"

	"github.com/u-ai/backend/internal/desktoppet/release"
	"gorm.io/gorm"
)

type SequenceAllocator struct {
	repo release.ReleaseRepository
}

func NewSequenceAllocator(repo release.ReleaseRepository) *SequenceAllocator {
	return &SequenceAllocator{repo: repo}
}

func (a *SequenceAllocator) AllocateSequence(ctx context.Context, petID string) (int, error) {
	var seq int
	err := a.repo.Transaction(func(tx *gorm.DB) error {
		var nextSeq int
		row := tx.Table("desktop_pet_identities").
			Where("id = ?", petID).
			Select("next_release_sequence").Row()
		if err := row.Scan(&nextSeq); err != nil {
			if err == gorm.ErrRecordNotFound {
				return NewBuildError(ErrCodeReleaseOwnershipDenied, "pet identity not found", err)
			}
			return err
		}

		seq = nextSeq
		allocated := nextSeq + 1

		result := tx.Table("desktop_pet_identities").
			Where("id = ? AND next_release_sequence = ?", petID, nextSeq).
			Update("next_release_sequence", allocated)
		if result.Error != nil {
			return result.Error
		}
		if result.RowsAffected == 0 {
			return NewBuildError(ErrCodeReleaseOperationConflict, "sequence allocation conflict, retry", nil)
		}
		return nil
	})
	if err != nil {
		return 0, err
	}
	return seq, nil
}

func formatTimestamp(t time.Time) string {
	return t.Format("2006-01-02 15:04:05")
}

func computeInputHash(userID, petID, activeRevisionSetHash, qualityGateID, buildConfigHash string) string {
	return fmt.Sprintf("%s|%s|%s|%s|%s", userID, petID, activeRevisionSetHash, qualityGateID, buildConfigHash)
}
