// SPDX-FileCopyrightText: 2026 彭旭
// SPDX-License-Identifier: AGPL-3.0-only
package adminrelease

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"

	"github.com/gin-gonic/gin"
)

type uploadHTTPResponse[T any] struct {
	Code int    `json:"code"`
	Msg  string `json:"msg"`
	Data T      `json:"data"`
}

func TestConcurrentArtifactUploadHTTPRunsTenCycles(t *testing.T) {
	gin.SetMode(gin.TestMode)
	server := newUploadTestServer(t)
	release := UpdateRelease{
		Product: ProductAndroid,
		Channel: ChannelStable,
		Version: "26.2.0-beta.97",
		Status:  StatusDraft,
	}
	if err := server.db.Create(&release).Error; err != nil {
		t.Fatal(err)
	}
	router := gin.New()
	router.Use(func(c *gin.Context) {
		c.Set(actorContextKey, adminActor{UserID: 1, Username: "upload-http", Role: RoleAdmin})
		c.Next()
	})
	router.POST("/releases/:id/upload-sessions", server.handleCreateArtifactUpload)
	router.GET("/releases/:id/upload-sessions/:uploadId", server.handleGetArtifactUpload)
	router.PUT("/releases/:id/upload-sessions/:uploadId/parts/:index", server.handleUploadArtifactPart)
	router.POST("/releases/:id/upload-sessions/:uploadId/complete", server.handleCompleteArtifactUpload)
	httpServer := httptest.NewServer(router)
	defer httpServer.Close()
	client := httpServer.Client()
	content := make([]byte, int(artifactUploadChunkSize*2+3457))
	for index := range content {
		content[index] = byte((index*29 + 11) % 251)
	}
	fileName := "amitia-upload-test-arm64-v8a.apk"

	for round := 1; round <= 10; round++ {
		createBody, err := json.Marshal(createArtifactUploadInput{
			Kind:     "apk",
			FileName: fileName,
			Size:     int64(len(content)),
		})
		if err != nil {
			t.Fatal(err)
		}
		createRequest, err := http.NewRequest(
			http.MethodPost,
			fmt.Sprintf("%s/releases/%d/upload-sessions", httpServer.URL, release.ID),
			bytes.NewReader(createBody),
		)
		if err != nil {
			t.Fatal(err)
		}
		createRequest.Header.Set("Content-Type", "application/json")
		createResponse, err := client.Do(createRequest)
		if err != nil {
			t.Fatalf("round %d create request: %v", round, err)
		}
		var created uploadHTTPResponse[artifactUploadSessionResponse]
		decodeErr := json.NewDecoder(createResponse.Body).Decode(&created)
		createResponse.Body.Close()
		if decodeErr != nil || createResponse.StatusCode != http.StatusOK || created.Code != 200 {
			t.Fatalf("round %d create response: status=%d body=%+v error=%v", round, createResponse.StatusCode, created, decodeErr)
		}
		chunkCount := created.Data.ChunkCount
		var wait sync.WaitGroup
		errs := make(chan error, chunkCount)
		for index := 0; index < chunkCount; index++ {
			wait.Add(1)
			go func(partIndex int) {
				defer wait.Done()
				start := int64(partIndex) * created.Data.ChunkSize
				end := start + created.Data.ChunkSize
				if end > int64(len(content)) {
					end = int64(len(content))
				}
				part := content[start:end]
				digest := sha256.Sum256(part)
				request, requestErr := http.NewRequest(
					http.MethodPut,
					fmt.Sprintf(
						"%s/releases/%d/upload-sessions/%s/parts/%d",
						httpServer.URL,
						release.ID,
						created.Data.UploadID,
						partIndex,
					),
					bytes.NewReader(part),
				)
				if requestErr != nil {
					errs <- requestErr
					return
				}
				request.Header.Set("Content-Type", "application/octet-stream")
				request.Header.Set("X-Chunk-SHA256", hex.EncodeToString(digest[:]))
				response, requestErr := client.Do(request)
				if requestErr != nil {
					errs <- requestErr
					return
				}
				_, _ = io.Copy(io.Discard, response.Body)
				response.Body.Close()
				if response.StatusCode != http.StatusOK {
					errs <- fmt.Errorf("part %d returned HTTP %d", partIndex, response.StatusCode)
					return
				}
				errs <- nil
			}(index)
		}
		wait.Wait()
		close(errs)
		for partErr := range errs {
			if partErr != nil {
				t.Fatalf("round %d upload part: %v", round, partErr)
			}
		}
		completeRequest, err := http.NewRequest(
			http.MethodPost,
			fmt.Sprintf(
				"%s/releases/%d/upload-sessions/%s/complete",
				httpServer.URL,
				release.ID,
				created.Data.UploadID,
			),
			nil,
		)
		if err != nil {
			t.Fatal(err)
		}
		completeResponse, err := client.Do(completeRequest)
		if err != nil {
			t.Fatalf("round %d complete request: %v", round, err)
		}
		var completed uploadHTTPResponse[UpdateReleaseArtifact]
		decodeErr = json.NewDecoder(completeResponse.Body).Decode(&completed)
		completeResponse.Body.Close()
		expected := sha256.Sum256(content)
		if decodeErr != nil || completeResponse.StatusCode != http.StatusOK ||
			completed.Data.SHA256 != hex.EncodeToString(expected[:]) {
			t.Fatalf(
				"round %d complete response: status=%d body=%+v error=%v",
				round,
				completeResponse.StatusCode,
				completed,
				decodeErr,
			)
		}
	}
}
