package pairing_test

import (
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"encoding/base64"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/u-ai/backend/internal/devicemesh/pairing"
	"github.com/u-ai/backend/internal/devicemesh/proof"
	"github.com/u-ai/backend/internal/extension/kernel/host_registry"
	"github.com/u-ai/backend/internal/runtimeidentity"
)

func signedApprovalRequest(t *testing.T, token string, key ed25519.PrivateKey) pairing.ClaimRequest {
	t.Helper()
	return signedApprovalRequestWithLabel(t, token, key, "")
}

func signedApprovalRequestWithLabel(t *testing.T, token string, key ed25519.PrivateKey, label string) pairing.ClaimRequest {
	t.Helper()
	return signedApprovalRequestFor(t, token, key, "phone-a", label)
}

func signedApprovalRequestFor(t *testing.T, token string, key ed25519.PrivateKey, deviceID, label string) pairing.ClaimRequest {
	t.Helper()
	request := pairingTestRequest(token, runtimeidentity.ParseDeviceID(deviceID))
	request.IssuerDeviceID = "issuer"
	request.Coordinated = true
	request.Label = label
	body := proof.ClaimBody{OfferToken: token, DeviceID: request.DeviceID.String(), RuntimeID: request.RuntimeID.String(), Platform: request.Platform.String(), Label: label}
	signed := proof.New(base64.RawURLEncoding.EncodeToString(key.Public().(ed25519.PublicKey)), "space-a", uuid.NewString(), body, time.Now())
	signed.Signature = base64.RawURLEncoding.EncodeToString(ed25519.Sign(key, signed.SigningBytes()))
	request.Proof = &signed
	return request
}

func TestDirectAddressPairingUsesTheSameApprovalFlow(t *testing.T) {
	service, db := pairingTestService(t)
	_, key, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	request := signedApprovalRequestFor(t, "", key, "phone-direct", "")
	pending, err := service.Claim(t.Context(), request)
	if err != nil || pending.Pending == nil || pending.PairingToken == "" || pending.Ticket != nil {
		t.Fatalf("地址配对未创建审批请求: result=%+v err=%v", pending, err)
	}
	request.OfferToken = pending.PairingToken
	request.Proof = signedApprovalRequestFor(t, pending.PairingToken, key, "phone-direct", "").Proof
	repeated, err := service.Claim(t.Context(), request)
	if err != nil || repeated.Pending == nil || repeated.Pending.RequestID != pending.Pending.RequestID {
		t.Fatalf("地址配对重复请求未复用审批: result=%+v err=%v", repeated, err)
	}
	if err := service.DecideApproval(t.Context(), pending.Pending.RequestID, 1, true); err != nil {
		t.Fatal(err)
	}
	request.Proof = signedApprovalRequestFor(t, pending.PairingToken, key, "phone-direct", "").Proof
	accepted, err := service.Claim(t.Context(), request)
	if err != nil || accepted.Ticket == nil || accepted.RawTicket == "" {
		t.Fatalf("批准后地址配对未签发票据: result=%+v err=%v", accepted, err)
	}
	var target string
	if err := db.QueryRow(`SELECT device_id FROM kernel_device_pairing_offer_targets WHERE offer_id=?`, pending.Pending.OfferID).Scan(&target); err != nil || target != "phone-direct" {
		t.Fatalf("地址配对凭据未限定请求设备: target=%s err=%v", target, err)
	}
}

func TestApprovalOfferCanPairMultipleDevicesUntilExpiry(t *testing.T) {
	service, db := pairingTestService(t)
	offer, token, err := service.CreateOffer(context.Background(), "issuer", time.Minute, true)
	if err != nil {
		t.Fatal(err)
	}
	_, key, _ := ed25519.GenerateKey(rand.Reader)
	result, err := service.Claim(context.Background(), signedApprovalRequest(t, token, key))
	if err != nil || result.Pending == nil || result.Ticket != nil || result.RawTicket != "" {
		t.Fatalf("审批前签发凭证: %v", err)
	}
	var tickets int
	if err := db.QueryRow(`SELECT COUNT(*) FROM kernel_device_mesh_bootstrap_tickets`).Scan(&tickets); err != nil || tickets != 0 {
		t.Fatal("审批前存在绑定票据")
	}
	repeated, err := service.Claim(context.Background(), signedApprovalRequest(t, token, key))
	if err != nil || repeated.Pending == nil || repeated.Pending.RequestID != result.Pending.RequestID {
		t.Fatal("重复等待创建了新请求")
	}
	if err := service.DecideApproval(context.Background(), result.Pending.RequestID, 2, true); !errors.Is(err, pairing.ErrApprovalConflict) {
		t.Fatal("错误审批版本未被拒绝")
	}
	if err := service.DecideApproval(context.Background(), result.Pending.RequestID, 1, true); err != nil {
		t.Fatal(err)
	}
	accepted, err := service.Claim(context.Background(), signedApprovalRequest(t, token, key))
	if err != nil || accepted.Pending != nil || accepted.Ticket == nil || accepted.RawTicket == "" {
		t.Fatalf("批准后未签发绑定票据: %v", err)
	}
	_, secondKey, _ := ed25519.GenerateKey(rand.Reader)
	second, err := service.Claim(context.Background(), signedApprovalRequestFor(t, token, secondKey, "phone-b", ""))
	if err != nil || second.Pending == nil {
		t.Fatalf("同一二维码无法供第二台设备发起配对: %v", err)
	}
	if err := service.DecideApproval(context.Background(), second.Pending.RequestID, 1, true); err != nil {
		t.Fatal(err)
	}
	secondAccepted, err := service.Claim(context.Background(), signedApprovalRequestFor(t, token, secondKey, "phone-b", ""))
	if err != nil || secondAccepted.Ticket == nil {
		t.Fatalf("同一二维码未签发第二台设备凭证: %v", err)
	}
	var status string
	if err := db.QueryRow(`SELECT status FROM kernel_device_pairing_offers WHERE offer_id=?`, offer.OfferID).Scan(&status); err != nil || status != "active" {
		t.Fatalf("有效期内二维码未保持可重复使用: status=%s err=%v", status, err)
	}
}

func TestApprovalRejectsUnsignedAndDeniedClaims(t *testing.T) {
	service, _ := pairingTestService(t)
	_, token, err := service.CreateOffer(context.Background(), "issuer", time.Minute, true)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := service.Claim(context.Background(), pairingTestRequest(token, "phone-a")); err == nil {
		t.Fatal("安全配对接受了未签名请求")
	}
	_, key, _ := ed25519.GenerateKey(rand.Reader)
	result, err := service.Claim(context.Background(), signedApprovalRequest(t, token, key))
	if err != nil {
		t.Fatal(err)
	}
	if err := service.DecideApproval(context.Background(), result.Pending.RequestID, 1, false); err != nil {
		t.Fatal(err)
	}
	if _, err := service.Claim(context.Background(), signedApprovalRequest(t, token, key)); !errors.Is(err, pairing.ErrApprovalDenied) {
		t.Fatalf("拒绝的设备仍能领取凭证: %v", err)
	}
}

func TestTrustedDeviceWithOriginalProofCanRotateCredentialAndPreserveAdministrator(t *testing.T) {
	service, db := pairingTestService(t)
	public, key, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := service.devices.EnsureDevice(t.Context(), host_registry.DeviceRecord{
		SpaceID: "space-a", DeviceID: "phone-a", Platform: runtimeidentity.PlatformAndroid, Label: "old-phone",
		TrustState: host_registry.DeviceTrustTrusted,
	}); err != nil {
		t.Fatal(err)
	}
	encodedPublic := base64.RawURLEncoding.EncodeToString(public)
	if _, err := db.ExecContext(t.Context(), `INSERT INTO kernel_device_identity_keys(device_id,public_key,created_at) VALUES(?,?,?)`, "phone-a", encodedPublic, time.Now().Unix()); err != nil {
		t.Fatal(err)
	}
	if _, err := db.ExecContext(t.Context(), `INSERT INTO kernel_device_coordination(space_id,device_id,coordinated,administrator,mode_revision,permission_revision,selected_role) VALUES('space-a','phone-a',1,1,6,9,'role-a')`); err != nil {
		t.Fatal(err)
	}
	_, token, err := service.CreateOffer(t.Context(), "issuer", time.Minute, true)
	if err != nil {
		t.Fatal(err)
	}
	first, err := service.Claim(t.Context(), signedApprovalRequestWithLabel(t, token, key, "new-phone"))
	if err != nil || first.Pending == nil {
		t.Fatalf("已信任设备无法进入重新配对审批: %v", err)
	}
	if err := service.DecideApproval(t.Context(), first.Pending.RequestID, 1, true); err != nil {
		t.Fatal(err)
	}
	rotated, err := service.Claim(t.Context(), signedApprovalRequestWithLabel(t, token, key, "new-phone"))
	if err != nil || rotated.Ticket == nil || rotated.RawTicket == "" {
		t.Fatalf("已信任设备无法轮换凭据: %v", err)
	}
	device, err := service.devices.GetDevice(t.Context(), "phone-a")
	if err != nil || device == nil || device.TrustState != host_registry.DeviceTrustTrusted || device.Label != "new-phone" {
		t.Fatalf("重新配对破坏了设备信任: device=%+v err=%v", device, err)
	}
	var administrator, modeRevision, permissionRevision int64
	var role string
	if err := db.QueryRowContext(t.Context(), `SELECT administrator,mode_revision,permission_revision,selected_role FROM kernel_device_coordination WHERE space_id='space-a' AND device_id='phone-a'`).Scan(&administrator, &modeRevision, &permissionRevision, &role); err != nil {
		t.Fatal(err)
	}
	if administrator != 1 || modeRevision != 6 || permissionRevision != 9 || role != "role-a" {
		t.Fatalf("重新配对重置了中心授权: administrator=%d mode=%d permission=%d role=%s", administrator, modeRevision, permissionRevision, role)
	}
}

func TestTrustedDeviceRotationStillRequiresMatchingProof(t *testing.T) {
	service, db := pairingTestService(t)
	public, _, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := service.devices.EnsureDevice(t.Context(), host_registry.DeviceRecord{
		SpaceID: "space-a", DeviceID: "phone-a", Platform: runtimeidentity.PlatformAndroid,
		TrustState: host_registry.DeviceTrustTrusted,
	}); err != nil {
		t.Fatal(err)
	}
	if _, err := db.ExecContext(t.Context(), `INSERT INTO kernel_device_identity_keys(device_id,public_key,created_at) VALUES(?,?,?)`, "phone-a", base64.RawURLEncoding.EncodeToString(public), time.Now().Unix()); err != nil {
		t.Fatal(err)
	}
	offer, token, err := service.CreateOffer(t.Context(), "issuer", time.Minute, true)
	if err != nil {
		t.Fatal(err)
	}
	unsigned := pairingTestRequest(token, "phone-a")
	if _, err := service.Claim(t.Context(), unsigned); !errors.Is(err, proof.ErrProof) {
		t.Fatalf("未签名重新配对未被拒绝: %v", err)
	}
	_, wrongKey, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := service.Claim(t.Context(), signedApprovalRequest(t, token, wrongKey)); !errors.Is(err, proof.ErrIdentityCopy) {
		t.Fatalf("错误设备身份未被拒绝: %v", err)
	}
	var status string
	if err := db.QueryRowContext(t.Context(), `SELECT status FROM kernel_device_pairing_offers WHERE offer_id=?`, offer.OfferID).Scan(&status); err != nil || status != "active" {
		t.Fatalf("失败的身份校验消费了 Offer: status=%s err=%v", status, err)
	}
}
