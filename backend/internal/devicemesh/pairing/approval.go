package pairing

import (
	"context"
	"database/sql"
	"errors"
	meshaudit "github.com/u-ai/backend/internal/devicemesh/audit"
	"github.com/u-ai/backend/internal/devicemesh/coordination"
	"time"

	"github.com/google/uuid"
)

const ApprovalPolicySchema = `CREATE TABLE IF NOT EXISTS kernel_device_pairing_approval_policy (offer_id TEXT PRIMARY KEY, required INTEGER NOT NULL)`
const OfferTargetSchema = `CREATE TABLE IF NOT EXISTS kernel_device_pairing_offer_targets (offer_id TEXT PRIMARY KEY, device_id TEXT NOT NULL)`
const SuccessorModeSchema = `CREATE TABLE IF NOT EXISTS kernel_device_pairing_successor_modes (offer_id TEXT PRIMARY KEY, coordinated INTEGER NOT NULL)`
const ApprovalSchema = `CREATE TABLE IF NOT EXISTS kernel_device_pairing_approvals (
	request_id TEXT PRIMARY KEY, offer_id TEXT NOT NULL UNIQUE, space_id TEXT NOT NULL,
	device_id TEXT NOT NULL, runtime_id TEXT NOT NULL, platform TEXT NOT NULL, label TEXT NOT NULL,
	public_key TEXT NOT NULL, state TEXT NOT NULL, revision INTEGER NOT NULL,
	created_at TEXT NOT NULL, decided_at TEXT)`

var ErrApprovalDenied = errors.New("服务提供设备拒绝了本次配对")
var ErrApprovalConflict = errors.New("配对审批状态已改变，请刷新后重试")

type Approval struct {
	RequestID string `json:"requestId"`
	OfferID   string `json:"offerId"`
	DeviceID  string `json:"deviceId"`
	RuntimeID string `json:"runtimeId"`
	Platform  string `json:"platform"`
	Label     string `json:"label"`
	PublicKey string `json:"publicKey"`
	State     string `json:"state"`
	Revision  int64  `json:"revision"`
	CreatedAt string `json:"createdAt"`
}

func (s *Service) approvalTx(ctx context.Context, tx *sql.Tx, offerID string, request ClaimRequest, now time.Time) (*Approval, error) {
	var required int
	err := tx.QueryRowContext(ctx, `SELECT required FROM kernel_device_pairing_approval_policy WHERE offer_id=?`, offerID).Scan(&required)
	if errors.Is(err, sql.ErrNoRows) || (err == nil && required == 0) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	if request.Proof == nil {
		return nil, errors.New("局域网配对必须验证设备身份")
	}
	approval := &Approval{}
	err = tx.QueryRowContext(ctx, `SELECT request_id,offer_id,device_id,runtime_id,platform,label,public_key,state,revision,created_at FROM kernel_device_pairing_approvals WHERE offer_id=? AND space_id=?`, offerID, s.spaceID.String()).Scan(&approval.RequestID, &approval.OfferID, &approval.DeviceID, &approval.RuntimeID, &approval.Platform, &approval.Label, &approval.PublicKey, &approval.State, &approval.Revision, &approval.CreatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		var other int
		if err := tx.QueryRowContext(ctx, `SELECT COUNT(*) FROM kernel_device_pairing_approvals a JOIN kernel_device_pairing_offers o ON o.offer_id=a.offer_id WHERE a.space_id=? AND a.device_id=? AND a.state IN ('pending','approved') AND o.status='active' AND julianday(o.expires_at)>julianday(?)`, s.spaceID.String(), request.DeviceID.String(), now.Format(time.RFC3339Nano)).Scan(&other); err != nil {
			return nil, err
		}
		if other > 0 {
			return nil, ErrPairingPending
		}
		approval = &Approval{RequestID: uuid.NewString(), OfferID: offerID, DeviceID: request.DeviceID.String(), RuntimeID: request.RuntimeID.String(), Platform: request.Platform.String(), Label: request.Label, PublicKey: request.Proof.PublicKey, State: "pending", Revision: 1, CreatedAt: now.Format(time.RFC3339Nano)}
		_, err = tx.ExecContext(ctx, `INSERT INTO kernel_device_pairing_approvals(request_id,offer_id,space_id,device_id,runtime_id,platform,label,public_key,state,revision,created_at) VALUES(?,?,?,?,?,?,?,?,?,?,?)`, approval.RequestID, offerID, s.spaceID.String(), approval.DeviceID, approval.RuntimeID, approval.Platform, approval.Label, approval.PublicKey, approval.State, approval.Revision, approval.CreatedAt)
		return approval, err
	}
	if err != nil {
		return nil, err
	}
	if approval.DeviceID != request.DeviceID.String() || approval.RuntimeID != request.RuntimeID.String() || approval.PublicKey != request.Proof.PublicKey || approval.Platform != request.Platform.String() || approval.Label != request.Label {
		return nil, ErrPairingPending
	}
	if approval.State == "rejected" {
		return nil, ErrApprovalDenied
	}
	if approval.State == "approved" {
		return nil, nil
	}
	return approval, nil
}

func (s *Service) PendingApprovals(ctx context.Context) ([]Approval, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT a.request_id,a.offer_id,a.device_id,a.runtime_id,a.platform,a.label,a.public_key,a.state,a.revision,a.created_at FROM kernel_device_pairing_approvals a JOIN kernel_device_pairing_offers o ON o.offer_id=a.offer_id WHERE a.space_id=? AND a.state='pending' AND o.status='active' AND julianday(o.expires_at)>julianday(?) ORDER BY a.created_at ASC LIMIT 128`, s.spaceID.String(), time.Now().UTC().Format(time.RFC3339Nano))
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	result := []Approval{}
	for rows.Next() {
		var value Approval
		if err := rows.Scan(&value.RequestID, &value.OfferID, &value.DeviceID, &value.RuntimeID, &value.Platform, &value.Label, &value.PublicKey, &value.State, &value.Revision, &value.CreatedAt); err != nil {
			return nil, err
		}
		result = append(result, value)
	}
	return result, rows.Err()
}

func (s *Service) DecideApproval(ctx context.Context, requestID string, expectedRevision int64, allow bool) error {
	return coordination.CommitRequestCurrent(ctx, func() error { return s.decideApproval(ctx, requestID, expectedRevision, allow) })
}

func (s *Service) decideApproval(ctx context.Context, requestID string, expectedRevision int64, allow bool) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if err := coordination.ValidateRequestAuthorityTx(ctx, tx); err != nil {
		return err
	}
	state := "rejected"
	if allow {
		state = "approved"
	}
	result, err := tx.ExecContext(ctx, `UPDATE kernel_device_pairing_approvals SET state=?,revision=revision+1,decided_at=? WHERE request_id=? AND space_id=? AND state='pending' AND revision=? AND offer_id IN (SELECT offer_id FROM kernel_device_pairing_offers WHERE status='active' AND julianday(expires_at)>julianday(?))`, state, time.Now().UTC().Format(time.RFC3339Nano), requestID, s.spaceID.String(), expectedRevision, time.Now().UTC().Format(time.RFC3339Nano))
	if err != nil {
		return err
	}
	count, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if count != 1 {
		return ErrApprovalConflict
	}
	var device string
	if err := tx.QueryRowContext(ctx, `SELECT device_id FROM kernel_device_pairing_approvals WHERE request_id=? AND space_id=?`, requestID, s.spaceID.String()).Scan(&device); err != nil {
		return err
	}
	eventType := "device_mesh.pairing_approved"
	if !allow {
		eventType = "device_mesh.pairing_rejected"
	}
	if err := meshaudit.QueueTx(ctx, tx, s.spaceID.String(), device, eventType, meshaudit.Details{ApprovalID: requestID}); err != nil {
		return err
	}
	return tx.Commit()
}
