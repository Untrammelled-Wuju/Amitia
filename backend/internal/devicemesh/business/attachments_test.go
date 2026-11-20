package business

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"strings"
	"testing"

	"github.com/u-ai/backend/internal/devicemesh/coordination"
)

func imageAttachment(t *testing.T, variant int) Attachment {
	t.Helper()
	data := []byte(strings.Repeat("image-"+string(rune('a'+variant)), 16))
	digest := sha256.Sum256(data)
	return Attachment{
		ArtifactID:      "art-image-" + string(rune('a'+variant)),
		OwnerDeviceID:   "a",
		StorageDeviceID: "a",
		Kind:            "image",
		Name:            "picture.png",
		MIMEType:        "image/png",
		SHA256:          hex.EncodeToString(digest[:]),
		SizeBytes:       int64(len(data)),
	}
}

func TestAttachmentsUseReferencesWithoutEmbeddedBytes(t *testing.T) {
	for _, kind := range []string{"image", "audio", "video", "file"} {
		item := imageAttachment(t, 0)
		item.Kind = kind
		item.MIMEType = map[string]string{"image": "image/png", "audio": "audio/wav", "video": "video/mp4", "file": "application/octet-stream"}[kind]
		if err := ValidateAttachments([]Attachment{item}); err != nil {
			t.Fatalf("%s reference rejected: %v", kind, err)
		}
	}
}

func TestAttachmentReferencesRejectInvalidOrSpoofedIdentity(t *testing.T) {
	valid := imageAttachment(t, 0)
	tests := map[string]func(*Attachment){
		"artifactId":      func(item *Attachment) { item.ArtifactID = "../artifact" },
		"ownerDeviceId":   func(item *Attachment) { item.OwnerDeviceID = "" },
		"storageDeviceId": func(item *Attachment) { item.StorageDeviceID = "" },
		"kind":            func(item *Attachment) { item.Kind = "executable" },
		"name":            func(item *Attachment) { item.Name = "bad\nname" },
		"mimeType":        func(item *Attachment) { item.MIMEType = "" },
		"sha256":          func(item *Attachment) { item.SHA256 = "not-a-digest" },
		"sizeBytes":       func(item *Attachment) { item.SizeBytes = -1 },
	}
	for name, mutate := range tests {
		t.Run(name, func(t *testing.T) {
			item := valid
			mutate(&item)
			if err := ValidateAttachments([]Attachment{item}); err == nil {
				t.Fatal("invalid attachment reference accepted")
			}
		})
	}
}

func TestMessageRejectsSpoofedAttachmentsBeforeSavingOrComputing(t *testing.T) {
	valid := imageAttachment(t, 0)
	engine, db, _, model := engineHarness(t)
	tests := map[string][]Attachment{
		"spoofed-owner": {{ArtifactID: valid.ArtifactID, OwnerDeviceID: "other", StorageDeviceID: "a", Kind: valid.Kind, Name: valid.Name, MIMEType: valid.MIMEType, SHA256: valid.SHA256, SizeBytes: valid.SizeBytes}},
		"too-many":      {valid, {ArtifactID: "art-2", OwnerDeviceID: "a", StorageDeviceID: "a", Kind: "file", Name: "b.bin", MIMEType: "application/octet-stream", SHA256: valid.SHA256, SizeBytes: 1}, {ArtifactID: "art-3", OwnerDeviceID: "a", StorageDeviceID: "a", Kind: "file", Name: "c.bin", MIMEType: "application/octet-stream", SHA256: valid.SHA256, SizeBytes: 1}},
	}
	for name, attachments := range tests {
		t.Run(name, func(t *testing.T) {
			_, err := engine.Run(t.Context(), Request{SpaceID: "core", DeviceID: "a", CoreID: "core", RequestID: name, Message: "describe", Attachments: attachments})
			if err == nil || model.calls.Load() != 0 {
				t.Fatalf("invalid attachment reached compute: %v", err)
			}
			var count int
			if err := db.QueryRow(`SELECT count(*) FROM kernel_device_owned_resources`).Scan(&count); err != nil || count != 0 {
				t.Fatalf("invalid attachment saved: %d %v", count, err)
			}
		})
	}
}

func TestAttachmentReferenceIsPersistedAtSelectedOwnerBeforeCompute(t *testing.T) {
	for _, coordinated := range []bool{false, true} {
		t.Run(map[bool]string{false: "device", true: "core"}[coordinated], func(t *testing.T) {
			engine, db, service, model := engineHarness(t)
			owner := "a"
			if coordinated {
				if _, err := service.InitializeCoordinated(t.Context(), "core", "a", 1, "role"); err != nil {
					t.Fatal(err)
				}
				owner = "core"
			}
			item := imageAttachment(t, 0)
			model.generate = func(_ context.Context, inference Inference) (Generation, error) {
				if len(inference.Attachments) != 1 || inference.Attachments[0] != item {
					t.Fatal("attachment reference lost before compute")
				}
				row, err := coordination.NewOwnershipStore(db, owner).Get(t.Context(), "message", "attachment/user")
				if err != nil || row == nil {
					t.Fatalf("compute ran before owner acknowledgement: %v", err)
				}
				return Generation{Text: "已处理"}, nil
			}
			response, err := engine.Run(t.Context(), Request{SpaceID: "core", DeviceID: "a", CoreID: "core", RequestID: "attachment", Message: "处理附件", Attachments: []Attachment{item}})
			if err != nil || !response.Saved || response.Scope.ResourceOwnerID != owner {
				t.Fatalf("response=%+v error=%v", response, err)
			}
			row, err := coordination.NewOwnershipStore(db, owner).Get(t.Context(), "message", "attachment/user")
			if err != nil || row == nil {
				t.Fatal(err)
			}
			var document struct {
				Attachments []Attachment `json:"attachments"`
				Data        any          `json:"data"`
			}
			if err := json.Unmarshal(row.Body, &document); err != nil || len(document.Attachments) != 1 || document.Attachments[0] != item || document.Data != nil {
				t.Fatalf("attachment reference not persisted exactly: %+v %v", document, err)
			}
		})
	}
}
