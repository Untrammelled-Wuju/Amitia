package business

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/base64"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"errors"
	"hash/crc32"
	"image"
	"image/png"
	"strings"
	"testing"

	"github.com/u-ai/backend/internal/devicemesh/coordination"
)

func imageAttachment(t *testing.T, size int) Attachment {
	t.Helper()
	var data bytes.Buffer
	if err := png.Encode(&data, image.NewRGBA(image.Rect(0, 0, size, size))); err != nil {
		t.Fatal(err)
	}
	hash := sha256.Sum256(data.Bytes())
	return Attachment{Kind: "image", Name: "picture.png", MIME: "image/png", Data: base64.StdEncoding.EncodeToString(data.Bytes()), Hash: hex.EncodeToString(hash[:])}
}

func TestFileAndVideoAttachmentsRequireOwnerAcknowledgementBeforeCompute(t *testing.T) {
	for _, coordinated := range []bool{false, true} {
		for _, kind := range []string{"file", "video"} {
			t.Run(map[bool]string{false: "device", true: "core"}[coordinated]+"/"+kind, func(t *testing.T) {
				engine, db, service, model := engineHarness(t)
				owner := "a"
				if coordinated {
					if _, err := service.ChangeMode(t.Context(), "core", "a", 1, true, "role"); err != nil {
						t.Fatal(err)
					}
					owner = "core"
				}
				data, mime := []byte("设备文件正文"), "text/plain"
				if kind == "video" {
					data, mime = []byte{0, 0, 0, 16, 'f', 't', 'y', 'p', 'i', 's', 'o', 'm', 0, 0, 0, 0}, "video/mp4"
				}
				digest := sha256.Sum256(data)
				item := Attachment{Kind: kind, Name: "附件", MIME: mime, Data: base64.StdEncoding.EncodeToString(data), Hash: hex.EncodeToString(digest[:])}
				model.generate = func(_ context.Context, inference Inference) (Generation, error) {
					row, err := coordination.NewOwnershipStore(db, owner).Get(t.Context(), "message", "attachment/user")
					if err != nil || row == nil || len(inference.Attachments) != 1 || inference.Attachments[0] != item {
						t.Fatal("Core computed before original binary owner acknowledgement")
					}
					return Generation{Text: "已处理"}, nil
				}
				request := Request{SpaceID: "core", DeviceID: "a", CoreID: "core", RequestID: "attachment", Message: "处理附件", Attachments: []Attachment{item}}
				response, err := engine.Run(t.Context(), request)
				if err != nil || !response.Saved || response.Scope.ResourceOwnerID != owner {
					t.Fatalf("response=%+v error=%v", response, err)
				}
				if _, err := engine.Run(t.Context(), request); err != nil || model.calls.Load() != 1 {
					t.Fatalf("attachment executed again: %v", err)
				}
				other := "core"
				if coordinated {
					other = "a"
				}
				if row, err := coordination.NewOwnershipStore(db, other).Get(t.Context(), "message", "attachment/user"); err != nil || row != nil {
					t.Fatal("binary data mirrored at another owner")
				}
			})
		}
	}
}

func TestFileAndVideoAttachmentsRejectSpoofingAndOverLimitBytes(t *testing.T) {
	for _, test := range []struct {
		kind, mime string
		data       []byte
	}{
		{"file", "text/plain", []byte{0xff}},
		{"file", "text/plain", []byte("text\x00hidden")},
		{"file", "application/pdf", []byte("not a PDF")},
		{"file", "application/vnd.openxmlformats-officedocument.wordprocessingml.document", []byte("not ZIP")},
		{"file", "application/octet-stream", []byte("unknown")},
		{"video", "video/mp4", []byte("not MP4")},
		{"video", "video/webm", []byte("not WebM")},
		{"file", "text/plain", []byte(strings.Repeat("a", (1<<20)+1))},
	} {
		digest := sha256.Sum256(test.data)
		item := Attachment{Kind: test.kind, Name: "附件", MIME: test.mime, Data: base64.StdEncoding.EncodeToString(test.data), Hash: hex.EncodeToString(digest[:])}
		if err := ValidateAttachments([]Attachment{item}); err == nil {
			t.Fatalf("invalid %s/%s accepted", test.kind, test.mime)
		}
	}
}

func TestAttachmentsRejectInvalidContentBeforeSavingOrComputing(t *testing.T) {
	valid := imageAttachment(t, 1)
	engine, db, _, model := engineHarness(t)
	for _, name := range []string{"hash", "mime", "kind", "url", "empty", "name", "dimensions", "count"} {
		t.Run(name, func(t *testing.T) {
			item := valid
			switch name {
			case "hash":
				item.Hash = "wrong"
			case "mime":
				item.MIME = "image/jpeg"
			case "kind":
				item.Kind = "file"
			case "url":
				item.Data = "https://internal.example/private"
			case "empty":
				item.Data = ""
			case "name":
				item.Name = "picture\n.png"
			case "dimensions":
				data, err := base64.StdEncoding.DecodeString(item.Data)
				if err != nil {
					t.Fatal(err)
				}
				binary.BigEndian.PutUint32(data[16:20], 8193)
				binary.BigEndian.PutUint32(data[29:33], crc32.ChecksumIEEE(data[12:29]))
				hash := sha256.Sum256(data)
				item.Data, item.Hash = base64.StdEncoding.EncodeToString(data), hex.EncodeToString(hash[:])
			}
			items := []Attachment{item}
			if name == "count" {
				items = []Attachment{item, item, item}
			}
			_, err := engine.Run(t.Context(), Request{SpaceID: "core", DeviceID: "a", CoreID: "core", RequestID: "image", Message: "describe", Attachments: items})
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

func TestImageSavedAtSelectedOwnerBeforeComputeAndBoundToRequestHash(t *testing.T) {
	for _, coordinated := range []bool{false, true} {
		t.Run(map[bool]string{false: "device", true: "core"}[coordinated], func(t *testing.T) {
			engine, db, service, model := engineHarness(t)
			owner := "a"
			if coordinated {
				if _, err := service.ChangeMode(t.Context(), "core", "a", 1, true, "role"); err != nil {
					t.Fatal(err)
				}
				owner = "core"
			}
			item := imageAttachment(t, 1)
			model.generate = func(_ context.Context, inference Inference) (Generation, error) {
				if len(inference.Attachments) != 1 || inference.Attachments[0] != item {
					t.Fatal("image lost before compute")
				}
				row, err := coordination.NewOwnershipStore(db, owner).Get(t.Context(), "message", "image/user")
				if err != nil || row == nil {
					t.Fatalf("compute ran before owner ACK: %v", err)
				}
				return Generation{Text: "image description"}, nil
			}
			request := Request{SpaceID: "core", DeviceID: "a", CoreID: "core", RequestID: "image", Message: "describe", Attachments: []Attachment{item}}
			response, err := engine.Run(t.Context(), request)
			if err != nil || !response.Saved || response.Scope.ResourceOwnerID != owner {
				t.Fatalf("response=%+v err=%v", response, err)
			}
			row, err := coordination.NewOwnershipStore(db, owner).Get(t.Context(), "message", "image/user")
			if err != nil {
				t.Fatal(err)
			}
			var document struct {
				Attachments []Attachment `json:"attachments"`
			}
			if err := json.Unmarshal(row.Body, &document); err != nil || len(document.Attachments) != 1 || document.Attachments[0] != item {
				t.Fatalf("image not persisted: %v", err)
			}
			other := "core"
			if coordinated {
				other = "a"
			}
			mirror, err := coordination.NewOwnershipStore(db, other).Get(t.Context(), "message", "image/user")
			if err != nil || mirror != nil {
				t.Fatal("attachment mirrored at other owner")
			}
			if _, err := engine.Run(t.Context(), request); err != nil || model.calls.Load() != 1 {
				t.Fatalf("request replayed: %v", err)
			}
			request.Attachments = []Attachment{imageAttachment(t, 2)}
			if _, err := engine.Run(t.Context(), request); !errors.Is(err, coordination.ErrRequestConflict) {
				t.Fatalf("changed image accepted under same request: %v", err)
			}
		})
	}
}
