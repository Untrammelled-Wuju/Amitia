package coordination

import "context"

func (s *Service) ValidateCommitAuthorities(ctx context.Context, commit Commit) error {
	if len(commit.AdditionalAuthorities) > 8 {
		return ErrPendingLimit
	}
	if err := s.Validate(ctx, commit.Scope); err != nil {
		return err
	}
	for _, authority := range commit.AdditionalAuthorities {
		current := commit.Scope
		if authority.SpaceID != current.SpaceID || authority.CoreID != current.CoreID || authority.AuthorizationRealm != current.AuthorizationRealm || authority.TargetDeviceID != current.TargetDeviceID || authority.ResourceOwnerID != current.ResourceOwnerID || authority.RoleOwnerID != current.RoleOwnerID || authority.RoleID != current.RoleID || authority.RoleRevision != current.RoleRevision || authority.Coordinated != current.Coordinated || authority.ModeRevision != current.ModeRevision || authority.TargetProviderEpoch != current.TargetProviderEpoch || authority.TargetPermissionRevision != current.TargetPermissionRevision {
			return ErrWrongOwner
		}
		if err := s.Validate(ctx, authority); err != nil {
			return err
		}
	}
	return nil
}
