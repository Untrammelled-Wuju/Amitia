package identity

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/u-ai/backend/internal/desktoppet/release"
)

type Service struct {
	repo release.ReleaseRepository
}

func NewService(repo release.ReleaseRepository) *Service {
	return &Service{repo: repo}
}

type PetIdentityError struct {
	Code string
	Msg  string
	Err  error
}

func (e *PetIdentityError) Error() string {
	if e.Err != nil {
		return fmt.Sprintf("%s: %s: %v", e.Code, e.Msg, e.Err)
	}
	return fmt.Sprintf("%s: %s", e.Code, e.Msg)
}

func (e *PetIdentityError) Unwrap() error {
	return e.Err
}

var (
	ErrPetIdentityNotFound = errors.New("pet identity not found")
	ErrPetIdentityConflict = errors.New("pet identity conflict")
)

func (s *Service) ResolveOrCreateForCharacter(
	ctx context.Context,
	userID string,
	characterID string,
	preferredName string,
) (*release.PetIdentityData, error) {
	if userID == "" {
		return nil, &PetIdentityError{Code: "INVALID_USER", Msg: "用户 ID 不能为空"}
	}
	if characterID == "" {
		return nil, &PetIdentityError{Code: "INVALID_CHARACTER", Msg: "角色 ID 不能为空"}
	}

	existing, err := s.repo.GetPetIdentityByCharacter(userID, characterID)
	if err == nil {
		return existing, nil
	}
	if !errors.Is(err, ErrPetIdentityNotFound) && !errors.Is(err, release.ErrPetIdentityNotFound) {
		return nil, &PetIdentityError{Code: "QUERY_FAILED", Msg: "查询 pet identity 失败", Err: err}
	}

	now := NewTimestamp()
	name := preferredName
	if name == "" {
		name = characterID
	}
	identity := &release.PetIdentityData{
		ID:                  uuid.NewString(),
		OwnerUserID:         userID,
		SourceCharacterID:   characterID,
		Name:                name,
		Slug:                makeSlug(name),
		BindingPolicy:       "character_locked",
		NextReleaseSequence: 1,
		CreatedAt:           now,
		UpdatedAt:           now,
	}

	if createErr := s.repo.CreatePetIdentity(identity); createErr != nil {
		existingPet, conflictErr := s.repo.GetPetIdentityByCharacter(userID, characterID)
		if conflictErr == nil {
			return existingPet, nil
		}
		return nil, &PetIdentityError{Code: "CREATE_FAILED", Msg: "创建 pet identity 失败", Err: createErr}
	}

	return identity, nil
}

func (s *Service) GetIdentity(ctx context.Context, userID, petID string) (*release.PetIdentityData, error) {
	identity, err := s.repo.GetPetIdentity(petID)
	if err != nil {
		if errors.Is(err, ErrPetIdentityNotFound) {
			return nil, &PetIdentityError{Code: "NOT_FOUND", Msg: "pet identity 不存在"}
		}
		return nil, &PetIdentityError{Code: "QUERY_FAILED", Msg: "查询失败", Err: err}
	}
	if identity.OwnerUserID != userID {
		return nil, &PetIdentityError{Code: "OWNERSHIP_DENIED", Msg: "不属于当前用户"}
	}
	return identity, nil
}

func makeSlug(name string) string {
	s := strings.ToLower(name)
	s = strings.ReplaceAll(s, " ", "-")
	s = strings.ReplaceAll(s, "_", "-")
	var b strings.Builder
	for _, r := range s {
		if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') || r == '-' {
			b.WriteRune(r)
		}
	}
	result := strings.Trim(b.String(), "-")
	if result == "" {
		result = "pet"
	}
	return result
}

func NewTimestamp() string {
	return time.Now().Format("2006-01-02 15:04:05")
}

var _ = context.Background
