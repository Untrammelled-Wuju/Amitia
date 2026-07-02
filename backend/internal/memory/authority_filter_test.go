package memory

import (
	"testing"
	"time"
)

func TestMemoryAllowedBySQLiteAuthorityFiltersScopeExpiryStatusAndProactiveMention(t *testing.T) {
	now := time.Date(2026, 7, 2, 12, 0, 0, 0, time.Local)
	expired := "2026-07-02 11:59:59"
	future := "2026-07-02 12:01:00"

	cases := []struct {
		name   string
		memory Memory
		policy retrievalAuthorityPolicy
		want   bool
	}{
		{
			name: "character scope allowed",
			memory: Memory{
				CharacterID:           "char-a",
				Scope:                 "character",
				VerifiedStatus:        "unverified",
				AllowProactiveMention: true,
				ExpiresAt:             &future,
			},
			policy: retrievalAuthorityPolicy{CharacterID: "char-a", ProactiveMention: true, Now: now},
			want:   true,
		},
		{
			name: "other character blocked",
			memory: Memory{
				CharacterID:           "char-b",
				Scope:                 "character",
				VerifiedStatus:        "unverified",
				AllowProactiveMention: true,
			},
			policy: retrievalAuthorityPolicy{CharacterID: "char-a", Now: now},
			want:   false,
		},
		{
			name: "user scope blocked without matching user",
			memory: Memory{
				CharacterID:           "user-1",
				Scope:                 "user",
				VerifiedStatus:        "user_verified",
				AllowProactiveMention: true,
			},
			policy: retrievalAuthorityPolicy{CharacterID: "char-a", ProactiveMention: true, Now: now},
			want:   false,
		},
		{
			name: "user scope allowed by user id",
			memory: Memory{
				CharacterID:           "user-1",
				Scope:                 "user",
				VerifiedStatus:        "user_verified",
				AllowProactiveMention: true,
			},
			policy: retrievalAuthorityPolicy{CharacterID: "char-a", UserID: "user-1", ProactiveMention: true, Now: now},
			want:   true,
		},
		{
			name: "expired blocked",
			memory: Memory{
				CharacterID:           "char-a",
				Scope:                 "character",
				VerifiedStatus:        "unverified",
				AllowProactiveMention: true,
				ExpiresAt:             &expired,
			},
			policy: retrievalAuthorityPolicy{CharacterID: "char-a", ProactiveMention: true, Now: now},
			want:   false,
		},
		{
			name: "tombstone status blocked",
			memory: Memory{
				CharacterID:           "char-a",
				Scope:                 "character",
				VerifiedStatus:        "tombstone",
				AllowProactiveMention: true,
			},
			policy: retrievalAuthorityPolicy{CharacterID: "char-a", ProactiveMention: true, Now: now},
			want:   false,
		},
		{
			name: "proactive mention permission blocked only in proactive mode",
			memory: Memory{
				CharacterID:           "char-a",
				Scope:                 "character",
				VerifiedStatus:        "unverified",
				AllowProactiveMention: false,
			},
			policy: retrievalAuthorityPolicy{CharacterID: "char-a", ProactiveMention: true, Now: now},
			want:   false,
		},
		{
			name: "manual retrieval ignores proactive mention permission",
			memory: Memory{
				CharacterID:           "char-a",
				Scope:                 "character",
				VerifiedStatus:        "unverified",
				AllowProactiveMention: false,
			},
			policy: retrievalAuthorityPolicy{CharacterID: "char-a", ProactiveMention: false, Now: now},
			want:   true,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := memoryAllowedBySQLiteAuthority(tc.memory, tc.policy)
			if got != tc.want {
				t.Fatalf("allowed = %v, want %v", got, tc.want)
			}
		})
	}
}
