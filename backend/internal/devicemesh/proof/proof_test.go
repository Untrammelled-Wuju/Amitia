package proof_test

import (
	"crypto/ed25519"
	"crypto/rand"
	"database/sql"
	"encoding/base64"
	"errors"
	"path/filepath"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/u-ai/backend/internal/devicemesh/proof"
	kernelsqlite "github.com/u-ai/backend/internal/extension/kernel/persistence/sqlite"
)

func signedClaim(t *testing.T, private ed25519.PrivateKey, body proof.ClaimBody) proof.Proof {
	t.Helper()
	public := base64.RawURLEncoding.EncodeToString(private.Public().(ed25519.PublicKey))
	p := proof.New(public, "core", uuid.NewString(), body, time.Now())
	p.Signature = base64.RawURLEncoding.EncodeToString(ed25519.Sign(private, p.SigningBytes()))
	return p
}

func TestIdentityProofBindsCoreRuntimeAndExactPairingBody(t *testing.T) {
	_, private, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	body := proof.ClaimBody{DeviceID: "a", RuntimeID: "runtime-a", Platform: "windows", OfferToken: "offer", Label: "device"}
	p := signedClaim(t, private, body)
	if err := proof.Verify(p, "core", body, time.Now()); err != nil {
		t.Fatal(err)
	}
	for _, changed := range []proof.ClaimBody{{DeviceID: "b", RuntimeID: body.RuntimeID, Platform: body.Platform, OfferToken: body.OfferToken, Label: body.Label}, {DeviceID: body.DeviceID, RuntimeID: body.RuntimeID, Platform: body.Platform, OfferToken: "stolen-offer", Label: body.Label}} {
		if err := proof.Verify(p, "core", changed, time.Now()); !errors.Is(err, proof.ErrProof) {
			t.Fatalf("changed body accepted: %v", err)
		}
	}
	if err := proof.Verify(p, "another-core", body, time.Now()); !errors.Is(err, proof.ErrProof) {
		t.Fatal("proof crossed Core authority")
	}
	if err := proof.Verify(p, "core", body, time.Now().Add(3*time.Minute)); !errors.Is(err, proof.ErrProof) {
		t.Fatal("expired proof accepted")
	}
}

func TestIdentityKeyBindingRejectsReplayCopyAndSubstitution(t *testing.T) {
	db, err := sql.Open("sqlite", filepath.Join(t.TempDir(), "proof.db"))
	if err != nil {
		t.Fatal(err)
	}
	db.SetMaxOpenConns(1)
	t.Cleanup(func() { _ = db.Close() })
	if err := kernelsqlite.Migrate(t.Context(), db); err != nil {
		t.Fatal(err)
	}
	bind := func(p proof.Proof, body proof.ClaimBody) error {
		tx, err := db.BeginTx(t.Context(), nil)
		if err != nil {
			return err
		}
		defer tx.Rollback()
		if err := proof.BindTx(t.Context(), tx, p, "core", body, time.Now()); err != nil {
			return err
		}
		return tx.Commit()
	}
	_, private, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	body := proof.ClaimBody{DeviceID: "a", RuntimeID: "runtime-a", Platform: "windows", OfferToken: "offer"}
	p := signedClaim(t, private, body)
	if err := bind(p, body); err != nil {
		t.Fatal(err)
	}
	if err := bind(p, body); !errors.Is(err, proof.ErrProof) {
		t.Fatalf("proof replay accepted: %v", err)
	}
	body.DeviceID = "copied-device"
	if err := bind(signedClaim(t, private, body), body); !errors.Is(err, proof.ErrIdentityCopy) {
		t.Fatalf("one key claimed two identities: %v", err)
	}
	body.DeviceID = "a"
	_, otherPrivate, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	if err := bind(signedClaim(t, otherPrivate, body), body); !errors.Is(err, proof.ErrIdentityCopy) {
		t.Fatalf("existing device key replaced: %v", err)
	}
}
