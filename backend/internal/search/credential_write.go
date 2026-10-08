package search

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/u-ai/backend/internal/devicemesh/coordination"
)

var ErrCredentialCleanupPending = errors.New("搜索凭据变更已提交，旧密钥清理已排队重试")

type PreparedCredentialVault interface {
	ReserveReference(context.Context, string) (string, error)
	StoreReference(context.Context, string, []byte) error
}

func credentialCleanupContext(ctx context.Context) (context.Context, context.CancelFunc) {
	return context.WithTimeout(context.WithoutCancel(ctx), 15*time.Second)
}

func (s *CredentialStore) queueCredentialCleanup(ctx context.Context, engine, ref string) error {
	if !isSecretReference(ref) {
		return errors.New("invalid cleanup reference")
	}
	_, err := s.db.ExecContext(ctx, `INSERT INTO search_credential_cleanup(secret_ref,engine_id,created_at) VALUES(?,?,?) ON CONFLICT(secret_ref) DO NOTHING`, ref, engine, time.Now().UTC())
	return err
}

func (s *CredentialStore) stageCredential(ctx context.Context, engine, value string) (string, error) {
	if prepared, ok := s.vault.(PreparedCredentialVault); ok {
		ref, err := prepared.ReserveReference(ctx, "search/"+engine)
		if err != nil {
			return "", err
		}
		cleanupCtx, finish := credentialCleanupContext(ctx)
		defer finish()
		if err := s.queueCredentialCleanup(cleanupCtx, engine, ref); err != nil {
			return "", err
		}
		if err := prepared.StoreReference(ctx, ref, []byte(value)); err != nil {
			return "", errors.Join(err, s.cleanupPendingCredentials(cleanupCtx))
		}
		return ref, nil
	}
	ref, err := s.vault.Store(ctx, "search/"+engine, []byte(value))
	if err != nil {
		return "", err
	}
	cleanupCtx, finish := credentialCleanupContext(ctx)
	defer finish()
	if err := s.queueCredentialCleanup(cleanupCtx, engine, ref); err != nil {
		return "", errors.Join(err, s.vault.Delete(cleanupCtx, ref))
	}
	return ref, nil
}

func (s *CredentialStore) setGuardedCredential(ctx context.Context, definition CredentialDefinition, value string) (CredentialStatus, error) {
	return s.setGuardedCredentialIfCurrent(ctx, definition, value, nil)
}

func (s *CredentialStore) setGuardedCredentialIfCurrent(ctx context.Context, definition CredentialDefinition, value string, expected *string) (CredentialStatus, error) {
	var result CredentialStatus
	err := coordination.CommitCurrent(ctx, func() error {
		s.writeMu.Lock()
		defer s.writeMu.Unlock()
		if err := coordination.ValidateCurrent(ctx); err != nil {
			return err
		}
		old, err := s.rawValue(ctx, definition.EngineID)
		if err != nil && !errors.Is(err, sql.ErrNoRows) {
			return err
		}
		if expected != nil && old != *expected {
			return nil
		}
		stored := value
		if s.vault != nil {
			stored, err = s.stageCredential(ctx, definition.EngineID, value)
			if err != nil {
				return err
			}
		}
		now := time.Now().UTC()
		err = s.commitCredentialReference(ctx, definition.EngineID, stored, old, now, false)
		cleanupCtx, finish := credentialCleanupContext(ctx)
		defer finish()
		if err != nil {
			if s.vault != nil {
				err = errors.Join(err, s.cleanupPendingCredentials(cleanupCtx))
			}
			return err
		}
		result = CredentialStatus{EngineID: definition.EngineID, Name: definition.Name, KeyURL: definition.KeyURL, Configured: true, UpdatedAt: now.Format(time.RFC3339)}
		if s.vault != nil {
			result.CleanupPending = s.cleanupPendingCredentials(cleanupCtx) != nil
		}
		return nil
	})
	return result, err
}

func (s *CredentialStore) commitCredentialReference(ctx context.Context, engine, stored, old string, now time.Time, remove bool) error {
	if err := coordination.ValidateCurrent(ctx); err != nil {
		return err
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if s.vault != nil && isSecretReference(old) && old != stored {
		if _, err := tx.ExecContext(ctx, `INSERT INTO search_credential_cleanup(secret_ref,engine_id,created_at) VALUES(?,?,?) ON CONFLICT(secret_ref) DO NOTHING`, old, engine, now); err != nil {
			return err
		}
	}
	if remove {
		_, err = tx.ExecContext(ctx, "DELETE FROM search_api_keys WHERE engine_id=?", engine)
	} else {
		_, err = tx.ExecContext(ctx, `INSERT INTO search_api_keys(engine_id,api_key,updated_at) VALUES(?,?,?) ON CONFLICT(engine_id) DO UPDATE SET api_key=excluded.api_key,updated_at=excluded.updated_at`, engine, stored, now)
	}
	if err != nil {
		return err
	}
	if s.vault != nil && !remove {
		if _, err := tx.ExecContext(ctx, "DELETE FROM search_credential_cleanup WHERE secret_ref=?", stored); err != nil {
			return err
		}
	}
	return tx.Commit()
}

func (s *CredentialStore) deleteGuardedCredential(ctx context.Context, engine string) error {
	return coordination.CommitCurrent(ctx, func() error {
		s.writeMu.Lock()
		defer s.writeMu.Unlock()
		if err := coordination.ValidateCurrent(ctx); err != nil {
			return err
		}
		old, err := s.rawValue(ctx, engine)
		if err != nil && !errors.Is(err, sql.ErrNoRows) {
			return err
		}
		if err := s.commitCredentialReference(ctx, engine, "", old, time.Now().UTC(), true); err != nil {
			return err
		}
		if s.vault != nil {
			cleanupCtx, finish := credentialCleanupContext(ctx)
			defer finish()
			if err := s.cleanupPendingCredentials(cleanupCtx); err != nil {
				return fmt.Errorf("%w: %v", ErrCredentialCleanupPending, err)
			}
		}
		return nil
	})
}

func (s *CredentialStore) CleanupPendingCredentials(ctx context.Context) error {
	if s == nil || s.db == nil || s.vault == nil {
		return nil
	}
	s.writeMu.Lock()
	defer s.writeMu.Unlock()
	return s.cleanupPendingCredentials(ctx)
}

func (s *CredentialStore) cleanupPendingCredentials(ctx context.Context) error {
	rows, err := s.db.QueryContext(ctx, "SELECT secret_ref FROM search_credential_cleanup ORDER BY created_at,secret_ref LIMIT 128")
	if err != nil {
		return err
	}
	var refs []string
	for rows.Next() {
		var ref string
		if err := rows.Scan(&ref); err != nil {
			rows.Close()
			return err
		}
		refs = append(refs, ref)
	}
	if err := rows.Err(); err != nil {
		rows.Close()
		return err
	}
	rows.Close()
	var failures error
	for _, ref := range refs {
		var active int
		canonical := strings.Replace(strings.TrimSpace(ref), "mcp-secret://", "secret://", 1)
		legacy := strings.Replace(canonical, "secret://", "mcp-secret://", 1)
		if err := s.db.QueryRowContext(ctx, "SELECT count(*) FROM search_api_keys WHERE TRIM(api_key) IN (?,?)", canonical, legacy).Scan(&active); err != nil {
			return errors.Join(failures, err)
		}
		if active == 0 {
			if !isSecretReference(ref) {
				failures = errors.Join(failures, errors.New("invalid queued secret reference"))
				continue
			}
			if err := s.vault.Delete(ctx, ref); err != nil {
				_, updateErr := s.db.ExecContext(ctx, "UPDATE search_credential_cleanup SET attempts=attempts+1,last_error='secret cleanup failed' WHERE secret_ref=?", ref)
				failures = errors.Join(failures, err, updateErr)
				continue
			}
		}
		if _, err := s.db.ExecContext(ctx, "DELETE FROM search_credential_cleanup WHERE secret_ref=?", ref); err != nil {
			failures = errors.Join(failures, err)
		}
	}
	return failures
}
