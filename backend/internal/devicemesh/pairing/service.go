package pairing

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"database/sql"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/u-ai/backend/internal/devicemesh/bootstrap"
	"github.com/u-ai/backend/internal/extension/kernel/host_registry"
	"github.com/u-ai/backend/internal/runtimeidentity"
)

const (
	DefaultOfferTTL = 5 * time.Minute
	bootstrapFile   = "device-pairing-bootstrap"
)

type Service struct {
	db           *sql.DB
	dataDir      string
	spaceID      runtimeidentity.SpaceID
	devices      *host_registry.Registry
	bootstrapSvc *bootstrap.Service
}

type Offer struct {
	OfferID           string
	SpaceID           runtimeidentity.SpaceID
	CreatedByDeviceID runtimeidentity.DeviceID
	Status            string
	ExpiresAt         time.Time
	CreatedAt         time.Time
	ConsumedAt        *time.Time
}

type ClaimRequest struct {
	OfferToken string
	SetupCode  string
	DeviceID   runtimeidentity.DeviceID
	RuntimeID  runtimeidentity.RuntimeID
	Platform   runtimeidentity.Platform
	Label      string
}

type ClaimResult struct {
	Ticket    *bootstrap.BootstrapTicket
	RawTicket string
}

func NewService(db *sql.DB, dataDir string, spaceID runtimeidentity.SpaceID, devices *host_registry.Registry, bootstrapSvc *bootstrap.Service) (*Service, error) {
	if db == nil || devices == nil || bootstrapSvc == nil || strings.TrimSpace(spaceID.String()) == "" {
		return nil, errors.New("pairing: incomplete service dependencies")
	}
	s := &Service{db: db, dataDir: dataDir, spaceID: spaceID, devices: devices, bootstrapSvc: bootstrapSvc}
	if err := s.ensureSchema(context.Background()); err != nil {
		return nil, err
	}
	if _, err := s.ensureBootstrapCode(); err != nil {
		return nil, err
	}
	return s, nil
}

func (s *Service) SpaceID() runtimeidentity.SpaceID { return s.spaceID }

func (s *Service) CreateOffer(ctx context.Context, creator runtimeidentity.DeviceID, ttl time.Duration) (*Offer, string, error) {
	if strings.TrimSpace(creator.String()) == "" {
		return nil, "", errors.New("pairing: creator device is required")
	}
	if err := s.devices.RequireTrustedDevice(ctx, s.spaceID, creator); err != nil {
		return nil, "", fmt.Errorf("pairing: creator device: %w", err)
	}
	if ttl <= 0 || ttl > 30*time.Minute {
		ttl = DefaultOfferTTL
	}
	raw, err := randomSecret("amt_pair_")
	if err != nil {
		return nil, "", err
	}
	now := time.Now().UTC()
	offer := &Offer{
		OfferID: uuid.NewString(), SpaceID: s.spaceID, CreatedByDeviceID: creator,
		Status: "active", ExpiresAt: now.Add(ttl), CreatedAt: now,
	}
	_, err = s.db.ExecContext(ctx, `INSERT INTO kernel_device_pairing_offers
		(offer_id, offer_hash, space_id, created_by_device_id, status, expires_at, consumed_at, created_at)
		VALUES (?, ?, ?, ?, ?, ?, NULL, ?)`,
		offer.OfferID, hash(raw), s.spaceID.String(), creator.String(), offer.Status,
		offer.ExpiresAt.Format(time.RFC3339Nano), offer.CreatedAt.Format(time.RFC3339Nano))
	if err != nil {
		return nil, "", fmt.Errorf("pairing: create offer: %w", err)
	}
	return offer, raw, nil
}

func (s *Service) Claim(ctx context.Context, req ClaimRequest) (*ClaimResult, error) {
	if req.DeviceID == "" || req.RuntimeID == "" || req.Platform == runtimeidentity.PlatformUnknown {
		return nil, errors.New("pairing: deviceId, runtimeId and platform are required")
	}
	if strings.TrimSpace(req.OfferToken) == "" && strings.TrimSpace(req.SetupCode) == "" {
		return nil, errors.New("pairing: offerToken or setupCode is required")
	}

	if strings.TrimSpace(req.OfferToken) != "" {
		if err := s.consumeOffer(ctx, strings.TrimSpace(req.OfferToken)); err != nil {
			return nil, err
		}
	} else {
		if err := s.authorizeFirstDevice(ctx, strings.TrimSpace(req.SetupCode)); err != nil {
			return nil, err
		}
	}

	existing, err := s.devices.GetDevice(ctx, req.DeviceID)
	if err != nil {
		return nil, err
	}
	if existing != nil && existing.SpaceID != s.spaceID {
		return nil, host_registry.ErrDeviceOwnedByOther
	}
	if _, err := s.devices.EnsureDevice(ctx, host_registry.DeviceRecord{
		SpaceID: s.spaceID, DeviceID: req.DeviceID, Platform: req.Platform,
		Label: strings.TrimSpace(req.Label), TrustState: host_registry.DeviceTrustPending,
	}); err != nil {
		return nil, err
	}
	ticket, rawTicket, err := s.bootstrapSvc.Issue(ctx, s.spaceID, req.DeviceID, req.RuntimeID, req.Platform)
	if err != nil {
		return nil, err
	}
	return &ClaimResult{Ticket: ticket, RawTicket: rawTicket}, nil
}

func (s *Service) Status(ctx context.Context) (trustedDevices int64, firstDeviceSetupRequired bool, err error) {
	err = s.db.QueryRowContext(ctx, `SELECT COUNT(1) FROM kernel_devices WHERE space_id = ? AND trust_state = 'trusted'`, s.spaceID.String()).Scan(&trustedDevices)
	if err != nil {
		return 0, false, err
	}
	return trustedDevices, trustedDevices == 0, nil
}

// BootstrapCode returns the one-time-owner setup secret. Callers must enforce a
// loopback-only transport boundary. Once one trusted device exists the code can
// no longer be used for pairing.
func (s *Service) BootstrapCode() (string, error) { return s.ensureBootstrapCode() }

func (s *Service) consumeOffer(ctx context.Context, raw string) error {
	now := time.Now().UTC()
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	var offerID, status, expires string
	err = tx.QueryRowContext(ctx, `SELECT offer_id, status, expires_at FROM kernel_device_pairing_offers WHERE offer_hash = ? AND space_id = ?`, hash(raw), s.spaceID.String()).Scan(&offerID, &status, &expires)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return errors.New("pairing: offer not found")
		}
		return err
	}
	expiresAt, err := time.Parse(time.RFC3339Nano, expires)
	if err != nil {
		return err
	}
	if status != "active" || !now.Before(expiresAt) {
		return errors.New("pairing: offer expired or consumed")
	}
	res, err := tx.ExecContext(ctx, `UPDATE kernel_device_pairing_offers SET status='consumed', consumed_at=? WHERE offer_id=? AND status='active'`, now.Format(time.RFC3339Nano), offerID)
	if err != nil {
		return err
	}
	n, _ := res.RowsAffected()
	if n != 1 {
		return errors.New("pairing: offer already consumed")
	}
	return tx.Commit()
}

func (s *Service) authorizeFirstDevice(ctx context.Context, setupCode string) error {
	trusted, first, err := s.Status(ctx)
	if err != nil {
		return err
	}
	if !first || trusted != 0 {
		return errors.New("pairing: bootstrap setup is closed; create a pairing offer from a trusted device")
	}
	expected, err := s.ensureBootstrapCode()
	if err != nil {
		return err
	}
	a := sha256.Sum256([]byte(setupCode))
	b := sha256.Sum256([]byte(expected))
	if subtle.ConstantTimeCompare(a[:], b[:]) != 1 {
		return errors.New("pairing: invalid setup code")
	}
	return nil
}

func (s *Service) ensureSchema(ctx context.Context) error {
	stmts := []string{
		`CREATE TABLE IF NOT EXISTS kernel_device_pairing_offers (
			offer_id TEXT PRIMARY KEY,
			offer_hash TEXT NOT NULL UNIQUE,
			space_id TEXT NOT NULL,
			created_by_device_id TEXT NOT NULL,
			status TEXT NOT NULL DEFAULT 'active',
			expires_at TEXT NOT NULL,
			consumed_at TEXT,
			created_at TEXT NOT NULL
		)`,
		`CREATE INDEX IF NOT EXISTS idx_kernel_device_pairing_offers_space_status ON kernel_device_pairing_offers(space_id, status, expires_at)`,
	}
	for _, stmt := range stmts {
		if _, err := s.db.ExecContext(ctx, stmt); err != nil {
			return fmt.Errorf("pairing: ensure schema: %w", err)
		}
	}
	return nil
}

func (s *Service) ensureBootstrapCode() (string, error) {
	path := filepath.Join(s.dataDir, "security", bootstrapFile)
	if data, err := os.ReadFile(path); err == nil {
		v := strings.TrimSpace(string(data))
		if strings.HasPrefix(v, "amt_pair_setup_") && len(v) > 32 {
			return v, nil
		}
		return "", errors.New("pairing: invalid bootstrap code file")
	} else if !errors.Is(err, os.ErrNotExist) {
		return "", err
	}
	raw, err := randomSecret("amt_pair_setup_")
	if err != nil {
		return "", err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return "", err
	}
	if err := os.WriteFile(path, []byte(raw+"\n"), 0o600); err != nil {
		return "", err
	}
	return raw, nil
}

func randomSecret(prefix string) (string, error) {
	buf := make([]byte, 32)
	if _, err := rand.Read(buf); err != nil {
		return "", err
	}
	return prefix + base64.RawURLEncoding.EncodeToString(buf), nil
}

func hash(raw string) string {
	sum := sha256.Sum256([]byte(raw))
	return hex.EncodeToString(sum[:])
}
