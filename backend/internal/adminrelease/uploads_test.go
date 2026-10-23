// SPDX-FileCopyrightText: 2026 彭旭
// SPDX-License-Identifier: AGPL-3.0-only
package adminrelease

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sync"
	"testing"

	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
)

func newUploadTestServer(t *testing.T) *Server {
	t.Helper()
	db, err := gorm.Open(sqlite.Open("file::memory:?cache=shared"), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	if err := db.AutoMigrate(&UpdateRelease{}, &UpdateReleaseArtifact{}, &AdminAuditLog{}); err != nil {
		t.Fatal(err)
	}
	return &Server{
		cfg: Config{
			DataDir:        t.TempDir(),
			MaxUploadBytes: 256 * 1024 * 1024,
		},
		db: db,
	}
}

func TestConcurrentArtifactUploadRunsTenCompleteCycles(t *testing.T) {
	server := newUploadTestServer(t)
	release := UpdateRelease{
		Product: ProductDesktop,
		Channel: ChannelStable,
		Version: "26.2.0-beta.99",
		Status:  StatusDraft,
	}
	if err := server.db.Create(&release).Error; err != nil {
		t.Fatal(err)
	}
	fileName := "AmitiaSetup-26.2.0-beta.99-x64.exe"
	content := make([]byte, int(artifactUploadChunkSize*2+12345))
	for index := range content {
		content[index] = byte((index*31 + 17) % 251)
	}
	actor := adminActor{UserID: 1, Username: "upload-test", Role: RoleAdmin}

	for round := 1; round <= 10; round++ {
		if round > 1 {
			content[0] = byte(round)
		}
		session, err := server.createArtifactUpload(release.ID, createArtifactUploadInput{
			Kind:     "installer",
			FileName: fileName,
			Size:     int64(len(content)),
		})
		if err != nil {
			t.Fatalf("round %d create session: %v", round, err)
		}
		var wait sync.WaitGroup
		errs := make(chan error, session.ChunkCount)
		for index := 0; index < session.ChunkCount; index++ {
			wait.Add(1)
			go func(partIndex int) {
				defer wait.Done()
				start := int64(partIndex) * session.ChunkSize
				end := start + session.ChunkSize
				if end > int64(len(content)) {
					end = int64(len(content))
				}
				part := content[start:end]
				digest := sha256.Sum256(part)
				errs <- server.uploadArtifactPart(
					release.ID,
					session.UploadID,
					partIndex,
					bytes.NewReader(part),
					hex.EncodeToString(digest[:]),
				)
			}(index)
		}
		wait.Wait()
		close(errs)
		for partErr := range errs {
			if partErr != nil {
				t.Fatalf("round %d upload part: %v", round, partErr)
			}
		}
		status, err := server.artifactUploadStatus(release.ID, session.UploadID)
		if err != nil {
			t.Fatalf("round %d status: %v", round, err)
		}
		if !status.Completed || len(status.UploadedParts) != session.ChunkCount {
			t.Fatalf("round %d incomplete status: %+v", round, status)
		}
		artifact, err := server.completeArtifactUpload(actor, release.ID, session.UploadID)
		if err != nil {
			t.Fatalf("round %d complete: %v", round, err)
		}
		expected := sha256.Sum256(content)
		if artifact.Size != int64(len(content)) ||
			artifact.SHA256 != hex.EncodeToString(expected[:]) ||
			artifact.OriginalName != fileName {
			t.Fatalf("round %d artifact mismatch: %+v", round, artifact)
		}
		if round%2 == 0 {
			second, err := server.createArtifactUpload(release.ID, createArtifactUploadInput{
				Kind:     "installer",
				FileName: fileName,
				Size:     int64(len(content)),
			})
			if err != nil {
				t.Fatal(err)
			}
			if err := server.abortArtifactUpload(release.ID, second.UploadID); err != nil {
				t.Fatal(err)
			}
		}
	}
}

func TestArtifactUploadRejectsCorruptedPart(t *testing.T) {
	server := newUploadTestServer(t)
	release := UpdateRelease{
		Product: ProductAndroid,
		Channel: ChannelStable,
		Version: "26.2.0-beta.98",
		Status:  StatusDraft,
	}
	if err := server.db.Create(&release).Error; err != nil {
		t.Fatal(err)
	}
	session, err := server.createArtifactUpload(release.ID, createArtifactUploadInput{
		Kind:     "apk",
		FileName: "amitia-release-arm64-v8a.apk",
		Size:     64,
	})
	if err != nil {
		t.Fatal(err)
	}
	content := bytes.Repeat([]byte{0x5a}, 64)
	err = server.uploadArtifactPart(
		release.ID,
		session.UploadID,
		0,
		bytes.NewReader(content),
		fmt.Sprintf("%064x", 1),
	)
	if err == nil {
		t.Fatal("corrupted part should be rejected")
	}
}

func TestProductionArtifactUploadOneCycle(t *testing.T) {
	artifactPath := os.Getenv("AMITIA_UPLOAD_TEST_ARTIFACT")
	expectedSHA256 := os.Getenv("AMITIA_UPLOAD_TEST_SHA256")
	if artifactPath == "" || expectedSHA256 == "" {
		t.Skip("production artifact upload test is not configured")
	}
	file, err := os.Open(artifactPath)
	if err != nil {
		t.Fatal(err)
	}
	defer file.Close()
	info, err := file.Stat()
	if err != nil {
		t.Fatal(err)
	}
	server := newUploadTestServer(t)
	release := UpdateRelease{
		Product: ProductAndroid,
		Channel: ChannelStable,
		Version: "26.2.0-beta.96",
		Status:  StatusDraft,
	}
	if err := server.db.Create(&release).Error; err != nil {
		t.Fatal(err)
	}
	fileName := filepath.Base(artifactPath)
	session, err := server.createArtifactUpload(release.ID, createArtifactUploadInput{
		Kind:     "apk",
		FileName: fileName,
		Size:     info.Size(),
	})
	if err != nil {
		t.Fatal(err)
	}
	var wait sync.WaitGroup
	errs := make(chan error, session.ChunkCount)
	for index := 0; index < session.ChunkCount; index++ {
		wait.Add(1)
		go func(partIndex int) {
			defer wait.Done()
			start := int64(partIndex) * session.ChunkSize
			size := expectedArtifactPartSize(artifactUploadManifest{
				Size:      info.Size(),
				ChunkSize: session.ChunkSize,
			}, partIndex)
			section := io.NewSectionReader(file, start, size)
			hasher := sha256.New()
			if _, err := io.Copy(hasher, io.NewSectionReader(file, start, size)); err != nil {
				errs <- err
				return
			}
			errs <- server.uploadArtifactPart(
				release.ID,
				session.UploadID,
				partIndex,
				section,
				hex.EncodeToString(hasher.Sum(nil)),
			)
		}(index)
	}
	wait.Wait()
	close(errs)
	for uploadErr := range errs {
		if uploadErr != nil {
			t.Fatal(uploadErr)
		}
	}
	artifact, err := server.completeArtifactUpload(
		adminActor{UserID: 1, Username: "production-upload", Role: RoleAdmin},
		release.ID,
		session.UploadID,
	)
	if err != nil {
		t.Fatal(err)
	}
	if artifact.Size != info.Size() || artifact.SHA256 != expectedSHA256 {
		t.Fatalf("uploaded artifact mismatch: %+v", artifact)
	}
}
