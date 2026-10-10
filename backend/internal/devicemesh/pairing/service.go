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
	"github.com/u-ai/backend/internal/auth"
	meshaudit "github.com/u-ai/backend/internal/devicemesh/audit"
	"github.com/u-ai/backend/internal/devicemesh/bootstrap"
	"github.com/u-ai/backend/internal/devicemesh/proof"
	"github.com/u-ai/backend/internal/extension/kernel/host_registry"
	"github.com/u-ai/backend/internal/runtimeidentity"
)

const (
	DefaultOfferTTL = 5 * time.Minute
	bootstrapFile   = "device-pairing-bootstrap"
)

var (
	ErrSelfPairing    = errors.New("不能添加当前设备自身")
	ErrAlreadyPaired  = errors.New("该设备已添加，无需重复扫码")
	ErrPairingPending = errors.New("该设备正在配对，请勿重复添加")
	ErrOfferNotFound  = errors.New("配对码无效")
	ErrOfferExpired   = errors.New("配对码已过期，请重新生成")
	ErrOfferConsumed  = errors.New("配对码已使用，请重新生成")
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
	Proof      *proof.Proof
}

type ClaimResult struct {
	Ticket    *bootstrap.BootstrapTicket
	RawTicket string
	Pending   *Approval
}

func NewService(db *sql.DB, dataDir string, spaceID runtimeidentity.SpaceID, devices *host_registry.Registry, bootstrapSvc *bootstrap.Service) (*Service, error) {
	if db == nil || devices == nil || bootstrapSvc == nil || strings.TrimSpace(spaceID.String()) == "" {
		return nil, errors.New("pairing: incomplete service dependencies")
	}
	if devices.Database() != db {
		return nil, errors.New("pairing: device registry and pairing state must share the authoritative database")
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

func (s *Service) RecoverLocalDevice(ctx context.Context, coreDeviceID runtimeidentity.DeviceID) error {
	actor, ok := auth.FromContext(ctx)
	if !ok || actor == nil || !actor.IsLocalTrusted || actor.PrincipalType != auth.PrincipalLocalUI || !actor.HasPermission(auth.PermSystemAdmin) || (actor.AuthMethod != "desktop_session" && actor.AuthMethod != "local_token") || coreDeviceID == "" || actor.DeviceID != coreDeviceID || actor.SpaceID != s.spaceID {
		return errors.New("只有本机 Core 所有者可以恢复本机配对权限")
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	device, err := s.devices.GetDeviceTx(ctx, tx, coreDeviceID)
	if err != nil {
		return err
	}
	if device == nil || device.SpaceID != s.spaceID {
		return host_registry.ErrDeviceOwnedByOther
	}
	if device.TrustState == host_registry.DeviceTrustTrusted {
		return nil
	}
	if err := s.devices.MarkDeviceTrustedTx(ctx, tx, coreDeviceID); err != nil {
		return err
	}
	if err := meshaudit.QueueTx(ctx, tx, s.spaceID.String(), coreDeviceID.String(), "device_mesh.local_core_trust_recovered", meshaudit.Details{}); err != nil {
		return err
	}
	return tx.Commit()
}

func (s *Service) CreateOffer(ctx context.Context, creator runtimeidentity.DeviceID, ttl time.Duration, requireApproval ...bool) (*Offer, string, error) {
	required := len(requireApproval) > 0 && requireApproval[0]
	return s.createOffer(ctx, creator, ttl, required, "")
}

func (s *Service) CreateSuccessorOffer(ctx context.Context, creator, target runtimeidentity.DeviceID, coordinated ...bool) (*Offer, string, error) {
	if target == "" || target == creator || len(target.String()) > 512 {
		return nil, "", ErrSelfPairing
	}
	offer, token, err := s.createOffer(ctx, creator, DefaultOfferTTL, true, target.String())
	if err != nil {
		return nil, "", err
	}
	enabled := len(coordinated) > 0 && coordinated[0]
	if _, err := s.db.ExecContext(ctx, `INSERT INTO kernel_device_pairing_successor_modes(offer_id,coordinated) VALUES(?,?)`, offer.OfferID, enabled); err != nil {
		return nil, "", err
	}
	return offer, token, nil
}

func (s *Service) createOffer(ctx context.Context, creator runtimeidentity.DeviceID, ttl time.Duration, required bool, target string) (*Offer, string, error) {
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
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, "", err
	}
	defer tx.Rollback()
	var active int
	if err := tx.QueryRowContext(ctx, `SELECT COUNT(*) FROM kernel_device_pairing_offers WHERE space_id=? AND created_by_device_id=? AND status='active' AND julianday(expires_at)>julianday(?)`, s.spaceID.String(), creator.String(), now.Format(time.RFC3339Nano)).Scan(&active); err != nil {
		return nil, "", err
	}
	if active >= 128 {
		return nil, "", errors.New("待处理配对入口已达到上限，请处理或等待过期后重试")
	}
	_, err = tx.ExecContext(ctx, `INSERT INTO kernel_device_pairing_offers
		(offer_id, offer_hash, space_id, created_by_device_id, status, expires_at, consumed_at, created_at)
		VALUES (?, ?, ?, ?, ?, ?, NULL, ?)`,
		offer.OfferID, hash(raw), s.spaceID.String(), creator.String(), offer.Status,
		offer.ExpiresAt.Format(time.RFC3339Nano), offer.CreatedAt.Format(time.RFC3339Nano))
	if err != nil {
		return nil, "", fmt.Errorf("pairing: create offer: %w", err)
	}
	if required {
		if _, err := tx.ExecContext(ctx, `INSERT INTO kernel_device_pairing_approval_policy(offer_id,required) VALUES(?,1)`, offer.OfferID); err != nil {
			return nil, "", err
		}
	}
	if target != "" {
		if _, err := tx.ExecContext(ctx, `INSERT INTO kernel_device_pairing_offer_targets(offer_id,device_id) VALUES(?,?)`, offer.OfferID, target); err != nil {
			return nil, "", err
		}
	}
	if err := meshaudit.QueueTx(ctx, tx, s.spaceID.String(), creator.String(), "device_mesh.pairing_offer_created", meshaudit.Details{}); err != nil {
		return nil, "", err
	}
	if err := tx.Commit(); err != nil {
		return nil, "", err
	}
	return offer, raw, nil
}

func (s *Service) Claim(ctx context.Context, req ClaimRequest) (*ClaimResult, error) {
	req.DeviceID = runtimeidentity.ParseDeviceID(req.DeviceID.String())
	req.RuntimeID = runtimeidentity.ParseRuntimeID(req.RuntimeID.String())
	if req.DeviceID == "" || req.RuntimeID == "" || req.Platform == runtimeidentity.PlatformUnknown || !req.Platform.IsKnown() {
		return nil, errors.New("pairing: deviceId, runtimeId and platform are required")
	}
	if strings.TrimSpace(req.OfferToken) == "" && strings.TrimSpace(req.SetupCode) == "" {
		return nil, errors.New("pairing: offerToken or setupCode is required")
	}

	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()
	if _, err := tx.ExecContext(ctx, `UPDATE kernel_devices SET revision=revision WHERE device_id=?`, req.DeviceID.String()); err != nil {
		return nil, err
	}
	now := time.Now().UTC()
	offerID := ""
	if strings.TrimSpace(req.OfferToken) != "" {
		offerID, err = s.validateOfferTx(ctx, tx, strings.TrimSpace(req.OfferToken), req.DeviceID, now)
	} else {
		err = s.authorizeFirstDeviceTx(ctx, tx, strings.TrimSpace(req.SetupCode))
	}
	if err != nil {
		return nil, err
	}

	existing, err := s.devices.GetDeviceTx(ctx, tx, req.DeviceID)
	if err != nil {
		return nil, err
	}
	if existing != nil && existing.SpaceID != s.spaceID {
		return nil, host_registry.ErrDeviceOwnedByOther
	}
	if existing != nil && existing.TrustState == host_registry.DeviceTrustTrusted {
		return nil, ErrAlreadyPaired
	}
	var activeTickets int
	if err := tx.QueryRowContext(ctx, `SELECT COUNT(1) FROM kernel_device_mesh_bootstrap_tickets
		WHERE space_id=? AND device_id=? AND status='active' AND julianday(expires_at)>julianday(?)`,
		s.spaceID.String(), req.DeviceID.String(), now.Format(time.RFC3339Nano)).Scan(&activeTickets); err != nil {
		return nil, err
	}
	if existing != nil && existing.TrustState == host_registry.DeviceTrustPending && activeTickets > 0 {
		return nil, ErrPairingPending
	}
	if req.Proof != nil {
		body := proof.ClaimBody{DeviceID: req.DeviceID.String(), RuntimeID: req.RuntimeID.String(), Platform: req.Platform.String(), Label: strings.TrimSpace(req.Label), OfferToken: strings.TrimSpace(req.OfferToken), SetupCode: strings.TrimSpace(req.SetupCode)}
		if err := proof.BindTx(ctx, tx, *req.Proof, s.spaceID.String(), body, now); err != nil {
			return nil, err
		}
	} else {
		var bound int
		if err := tx.QueryRowContext(ctx, `SELECT COUNT(*) FROM kernel_device_identity_keys WHERE device_id=?`, req.DeviceID.String()).Scan(&bound); err != nil {
			return nil, err
		}
		if bound > 0 {
			return nil, proof.ErrProof
		}
	}
	if offerID != "" {
		pending, err := s.approvalTx(ctx, tx, offerID, req, now)
		if err != nil {
			return nil, err
		}
		if pending != nil {
			if err := meshaudit.QueueTx(pairingAuditContext(ctx, s.spaceID.String(), req), tx, s.spaceID.String(), req.DeviceID.String(), "device_mesh.pairing_approval_requested", meshaudit.Details{ApprovalID: pending.RequestID}); err != nil {
				return nil, err
			}
			if err := tx.Commit(); err != nil {
				return nil, err
			}
			return &ClaimResult{Pending: pending}, nil
		}
		var coordinated bool
		if err := tx.QueryRowContext(ctx, `SELECT coordinated FROM kernel_device_pairing_successor_modes WHERE offer_id=?`, offerID).Scan(&coordinated); err == nil {
			if _, err := tx.ExecContext(ctx, `INSERT INTO kernel_device_coordination(space_id,device_id,coordinated) VALUES(?,?,?) ON CONFLICT(space_id,device_id) DO UPDATE SET coordinated=excluded.coordinated,administrator=0,mode_revision=mode_revision+1,permission_revision=permission_revision+1,selected_role=''`, s.spaceID.String(), req.DeviceID.String(), coordinated); err != nil {
				return nil, err
			}
		} else if !errors.Is(err, sql.ErrNoRows) {
			return nil, err
		}
	}
	if _, err := tx.ExecContext(ctx, `UPDATE kernel_device_mesh_bootstrap_tickets SET status='revoked', updated_at=?
		WHERE space_id=? AND device_id=? AND status='active'`, now.Format(time.RFC3339Nano), s.spaceID.String(), req.DeviceID.String()); err != nil {
		return nil, err
	}
	record := host_registry.DeviceRecord{
		SpaceID: s.spaceID, DeviceID: req.DeviceID, Platform: req.Platform,
		Label: strings.TrimSpace(req.Label), TrustState: host_registry.DeviceTrustPending,
		CreatedAt: now, LastSeenAt: now, Revision: 1,
	}
	if existing != nil {
		record.CreatedAt = existing.CreatedAt
		record.Revision = existing.Revision + 1
	}
	if err := s.devices.SaveDeviceTx(ctx, tx, &record); err != nil {
		return nil, err
	}
	ticket, rawTicket, err := s.bootstrapSvc.IssueTx(ctx, tx, s.spaceID, req.DeviceID, req.RuntimeID, req.Platform)
	if err != nil {
		return nil, err
	}
	if offerID != "" {
		res, err := tx.ExecContext(ctx, `UPDATE kernel_device_pairing_offers SET status='consumed', consumed_at=? WHERE offer_id=? AND status='active'`, now.Format(time.RFC3339Nano), offerID)
		if err != nil {
			return nil, err
		}
		count, err := res.RowsAffected()
		if err != nil {
			return nil, err
		}
		if count != 1 {
			return nil, ErrOfferConsumed
		}
	}
	if err := meshaudit.QueueTx(pairingAuditContext(ctx, s.spaceID.String(), req), tx, s.spaceID.String(), req.DeviceID.String(), "device_mesh.pairing_claimed", meshaudit.Details{}); err != nil {
		return nil, err
	}
	if err := tx.Commit(); err != nil {
		return nil, err
	}
	return &ClaimResult{Ticket: ticket, RawTicket: rawTicket}, nil
}

func pairingAuditContext(ctx context.Context, space string, request ClaimRequest) context.Context {
	method := "pairing_offer"
	if request.Proof != nil {
		method = "pairing_proof"
	} else if strings.TrimSpace(request.OfferToken) == "" {
		method = "bootstrap_setup"
	}
	return meshaudit.WithActor(ctx, meshaudit.Actor{SpaceID: space, DeviceID: request.DeviceID.String(), PrincipalType: "device_bootstrap", AuthMethod: method, Realm: "mesh"})
}

func (s *Service) Status(ctx context.Context) (trustedDevices int64, firstDeviceSetupRequired bool, err error) {
	err = s.db.QueryRowContext(ctx, `SELECT COUNT(1) FROM kernel_devices WHERE space_id = ? AND trust_state = 'trusted'`, s.spaceID.String()).Scan(&trustedDevices)
	if err != nil {
		return 0, false, err
	}
	return trustedDevices, trustedDevices == 0, nil
}

func (s *Service) RevokeDevicePairingTx(ctx context.Context, tx *sql.Tx, deviceID runtimeidentity.DeviceID) error {
	now := time.Now().UTC().Format(time.RFC3339Nano)
	if _, err := tx.ExecContext(ctx, `UPDATE kernel_device_mesh_bootstrap_tickets SET status='revoked', updated_at=?
		WHERE space_id=? AND device_id=? AND status='active'`, now, s.spaceID.String(), deviceID.String()); err != nil {
		return err
	}
	_, err := tx.ExecContext(ctx, `UPDATE kernel_device_pairing_offers SET status='revoked'
		WHERE space_id=? AND created_by_device_id=? AND status='active'`, s.spaceID.String(), deviceID.String())
	return err
}

// BootstrapCode returns the one-time-owner setup secret. Callers must enforce a
// loopback-only transport boundary. Once one trusted device exists the code can
// no longer be used for pairing.
func (s *Service) BootstrapCode() (string, error) { return s.ensureBootstrapCode() }

func (s *Service) validateOfferTx(ctx context.Context, tx *sql.Tx, raw string, deviceID runtimeidentity.DeviceID, now time.Time) (string, error) {
	var offerID, creator, status, expires string
	err := tx.QueryRowContext(ctx, `SELECT offer_id, created_by_device_id, status, expires_at FROM kernel_device_pairing_offers WHERE offer_hash = ? AND space_id = ?`, hash(raw), s.spaceID.String()).Scan(&offerID, &creator, &status, &expires)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return "", ErrOfferNotFound
		}
		return "", err
	}
	if runtimeidentity.ParseDeviceID(creator) == deviceID {
		return "", ErrSelfPairing
	}
	var target string
	err = tx.QueryRowContext(ctx, `SELECT device_id FROM kernel_device_pairing_offer_targets WHERE offer_id=?`, offerID).Scan(&target)
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		return "", err
	}
	if err == nil && target != deviceID.String() {
		return "", errors.New("该配对入口仅供指定设备连接新服务提供者")
	}
	existing, err := s.devices.GetDeviceTx(ctx, tx, deviceID)
	if err != nil {
		return "", err
	}
	if existing != nil && existing.SpaceID == s.spaceID && existing.TrustState == host_registry.DeviceTrustTrusted {
		return "", ErrAlreadyPaired
	}
	expiresAt, err := time.Parse(time.RFC3339Nano, expires)
	if err != nil {
		return "", err
	}
	if status != "active" {
		return "", ErrOfferConsumed
	}
	if !now.Before(expiresAt) {
		return "", ErrOfferExpired
	}
	issuer, err := s.devices.GetDeviceTx(ctx, tx, runtimeidentity.ParseDeviceID(creator))
	if err != nil {
		return "", err
	}
	if issuer == nil || issuer.SpaceID != s.spaceID || issuer.TrustState != host_registry.DeviceTrustTrusted {
		return "", host_registry.ErrDeviceNotTrusted
	}
	return offerID, nil
}

func (s *Service) authorizeFirstDeviceTx(ctx context.Context, tx *sql.Tx, setupCode string) error {
	var trusted int64
	if err := tx.QueryRowContext(ctx, `SELECT COUNT(1) FROM kernel_devices WHERE space_id=? AND trust_state='trusted'`, s.spaceID.String()).Scan(&trusted); err != nil {
		return err
	}
	if trusted != 0 {
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
		ApprovalPolicySchema,
		ApprovalSchema,
		OfferTargetSchema,
		SuccessorModeSchema,
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
