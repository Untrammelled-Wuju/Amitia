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
)

func signedApprovalRequest(t *testing.T, token string, key ed25519.PrivateKey) pairing.ClaimRequest {
	t.Helper()
	request := pairingTestRequest(token, "phone-a")
	body := proof.ClaimBody{OfferToken: token, DeviceID: request.DeviceID.String(), RuntimeID: request.RuntimeID.String(), Platform: request.Platform.String()}
	signed := proof.New(base64.RawURLEncoding.EncodeToString(key.Public().(ed25519.PublicKey)), "space-a", uuid.NewString(), body, time.Now())
	signed.Signature = base64.RawURLEncoding.EncodeToString(ed25519.Sign(key, signed.SigningBytes()))
	request.Proof = &signed
	return request
}

func TestApprovalDoesNotIssueCredentialsBeforeProviderDecision(t *testing.T) {
	service, db := pairingTestService(t)
	_, token, err := service.CreateOffer(context.Background(), "issuer", time.Minute, true)
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
	if _, err := service.Claim(context.Background(), signedApprovalRequest(t, token, key)); !errors.Is(err, pairing.ErrOfferConsumed) {
		t.Fatalf("二维码未保证单次使用: %v", err)
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
