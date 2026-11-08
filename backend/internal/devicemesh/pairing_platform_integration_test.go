package devicemesh_test

import (
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/u-ai/backend/internal/devicemesh/agent"
	"github.com/u-ai/backend/internal/devicemesh/pairing"
	"github.com/u-ai/backend/internal/devicemesh/proof"
	"github.com/u-ai/backend/internal/extension/kernel/host_registry"
	"github.com/u-ai/backend/internal/runtimeidentity"
)

func TestSignedPairingApprovalAndDuplicateBlockingAcrossPhoneAndComputerPlatforms(t *testing.T) {
	for _, issuerPlatform := range []runtimeidentity.Platform{runtimeidentity.PlatformWindows, runtimeidentity.PlatformAndroid} {
		for _, joiningPlatform := range []runtimeidentity.Platform{runtimeidentity.PlatformWindows, runtimeidentity.PlatformAndroid} {
			t.Run(issuerPlatform.String()+"-to-"+joiningPlatform.String(), func(t *testing.T) {
				db := meshAuthorityDB(t)
				registry := host_registry.NewRegistry(db)
				if _, err := registry.EnsureDevice(t.Context(), host_registry.DeviceRecord{SpaceID: "core", DeviceID: "issuer", Platform: issuerPlatform, TrustState: host_registry.DeviceTrustTrusted}); err != nil {
					t.Fatal(err)
				}
				rt := newCloudRuntimeForTest(t, db, registry)
				service, err := pairing.NewService(db, t.TempDir(), "core", registry, rt.BootstrapSvc)
				if err != nil {
					t.Fatal(err)
				}
				identityStore := agent.NewIdentityStore(t.TempDir())
				identity, err := identityStore.Load()
				if err != nil {
					t.Fatal(err)
				}
				claim := func(token string, device runtimeidentity.DeviceID) (*pairing.ClaimResult, error) {
					body := proof.ClaimBody{DeviceID: device.String(), RuntimeID: identity.RuntimeID.String(), Platform: joiningPlatform.String(), OfferToken: token}
					signed := proof.New(identity.PublicKey, "core", uuid.NewString(), body, time.Now())
					signed.Signature, err = identityStore.Sign(signed.SigningBytes())
					if err != nil {
						return nil, err
					}
					return service.Claim(t.Context(), pairing.ClaimRequest{OfferToken: token, DeviceID: device, RuntimeID: identity.RuntimeID, Platform: joiningPlatform, Proof: &signed})
				}
				_, token, err := service.CreateOffer(t.Context(), "issuer", time.Minute, true)
				if err != nil {
					t.Fatal(err)
				}
				pending, err := claim(token, identity.DeviceID)
				if err != nil || pending.Pending == nil || pending.RawTicket != "" {
					t.Fatalf("new platform bypassed pairing approval: %v", err)
				}
				if err := service.DecideApproval(t.Context(), pending.Pending.RequestID, pending.Pending.Revision, true); err != nil {
					t.Fatal(err)
				}
				accepted, err := claim(token, identity.DeviceID)
				if err != nil || accepted.RawTicket == "" {
					t.Fatalf("approved platform did not receive credential ticket: %v", err)
				}
				_, rawCredential, _, err := rt.BootstrapSvc.Exchange(t.Context(), accepted.RawTicket, identity.DeviceID, identity.RuntimeID)
				if err != nil {
					t.Fatal(err)
				}
				if _, err := rt.CredentialSvc.Validate(t.Context(), rawCredential); err != nil {
					t.Fatal(err)
				}
				record, err := registry.GetDevice(t.Context(), identity.DeviceID)
				if err != nil || record.Platform != joiningPlatform || record.TrustState != host_registry.DeviceTrustTrusted {
					t.Fatalf("platform identity lost after credential exchange: %+v %v", record, err)
				}
				_, duplicateToken, err := service.CreateOffer(t.Context(), "issuer", time.Minute, true)
				if err != nil {
					t.Fatal(err)
				}
				replacement, err := claim(duplicateToken, identity.DeviceID)
				if err != nil || replacement.Pending == nil {
					t.Fatalf("已信任设备无法重新配对: %v", err)
				}
				if err := service.DecideApproval(t.Context(), replacement.Pending.RequestID, replacement.Pending.Revision, true); err != nil {
					t.Fatal(err)
				}
				replacement, err = claim(duplicateToken, identity.DeviceID)
				if err != nil || replacement.RawTicket == "" {
					t.Fatalf("已信任设备重新配对未获得新票据: %v", err)
				}
				_, replacementCredential, _, err := rt.BootstrapSvc.Exchange(t.Context(), replacement.RawTicket, identity.DeviceID, identity.RuntimeID)
				if err != nil {
					t.Fatal(err)
				}
				if _, err := rt.CredentialSvc.Validate(t.Context(), replacementCredential); err != nil {
					t.Fatal(err)
				}
				if replacementCredential == rawCredential {
					t.Fatal("重新配对未轮换 Device Credential")
				}
				_, copiedToken, err := service.CreateOffer(t.Context(), "issuer", time.Minute, true)
				if err != nil {
					t.Fatal(err)
				}
				if _, err := claim(copiedToken, "copied-device"); !errors.Is(err, proof.ErrIdentityCopy) {
					t.Fatalf("same identity key admitted under another device: %v", err)
				}
			})
		}
	}
}
