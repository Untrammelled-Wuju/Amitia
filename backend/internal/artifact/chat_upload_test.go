package artifact

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/u-ai/backend/internal/auth"
	"github.com/u-ai/backend/internal/runtimeidentity"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

type failingUploadReader struct{}

func (failingUploadReader) Read([]byte) (int, error) {
	return 0, errors.New("upload transport failed: test source disconnected")
}

func TestChatUploadStreamsBinaryAndEnforcesOwner(t *testing.T) {
	gin.SetMode(gin.TestMode)
	db, err := gorm.Open(sqlite.Open(filepath.Join(t.TempDir(), "artifacts.db")), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	if err := db.AutoMigrate(&Artifact{}, &ArtifactReference{}); err != nil {
		t.Fatal(err)
	}
	store := NewFilesystemBlobStore(t.TempDir())
	service := NewService(store, NewRepository(db), UploadLimits{FileMaxBytes: 1024})
	if err := service.SetStorageDeviceID("device-a"); err != nil {
		t.Fatal(err)
	}
	router := gin.New()
	group := router.Group("/api", func(c *gin.Context) {
		if owner := c.GetHeader("X-Test-Owner"); owner != "" {
			c.Set("actorContext", &auth.ActorContext{SpaceID: runtimeidentity.SpaceID(owner), DeviceID: runtimeidentity.DeviceID(c.GetHeader("X-Test-Device"))})
		}
	})
	NewHandler(service).Register(group)
	data := bytes.Repeat([]byte{0, 255, 1, 2, 3}, 300000)
	request := func(method, path, owner, device string, body []byte) *httptest.ResponseRecorder {
		req := httptest.NewRequest(method, path, bytes.NewReader(body))
		req.Header.Set("X-Test-Owner", owner)
		req.Header.Set("X-Test-Device", device)
		response := httptest.NewRecorder()
		router.ServeHTTP(response, req)
		return response
	}
	path := "/api/artifacts/v1/chat-uploads?filename=data.bin"
	failing := httptest.NewRequest(http.MethodPost, path, nil)
	failing.Header.Set("X-Test-Owner", "space-a")
	failing.Header.Set("X-Test-Device", "device-a")
	failing.Body = io.NopCloser(failingUploadReader{})
	failure := httptest.NewRecorder()
	router.ServeHTTP(failure, failing)
	if failure.Code == http.StatusOK || !strings.Contains(failure.Body.String(), "upload transport failed: test source disconnected") {
		t.Fatalf("upload error lost: %d %s", failure.Code, failure.Body.String())
	}
	if response := request(http.MethodPost, path, "", "", data); response.Code != http.StatusUnauthorized {
		t.Fatalf("unauthorized upload: %d %s", response.Code, response.Body.String())
	}
	if response := request(http.MethodPost, path, "space-a", "", data); response.Code == http.StatusOK {
		t.Fatal("upload without a device owner accepted")
	}
	response := request(http.MethodPost, path, "space-a", "device-a", data)
	if response.Code != http.StatusOK {
		t.Fatalf("upload: %d %s", response.Code, response.Body.String())
	}
	var result struct {
		Artifact Artifact `json:"artifact"`
	}
	if err := json.Unmarshal(response.Body.Bytes(), &result); err != nil {
		t.Fatal(err)
	}
	if result.Artifact.SizeBytes != int64(len(data)) || result.Artifact.Source != SourceChatUpload || result.Artifact.OwnerSpaceID != "space-a" {
		t.Fatalf("incorrect metadata: %+v", result.Artifact)
	}
	if _, err := service.ReadChatUploadChunk(t.Context(), "space-a", "device-b", "device-a", result.Artifact.ID, 0, ChatUploadChunkMaxBytes); err == nil {
		t.Fatal("different device read an upload chunk")
	}
	var reconstructed []byte
	for offset := int64(0); ; {
		chunk, err := service.ReadChatUploadChunk(t.Context(), "space-a", "device-a", "device-a", result.Artifact.ID, offset, ChatUploadChunkMaxBytes)
		if err != nil {
			t.Fatal(err)
		}
		if chunk.Offset != offset || chunk.Digest != result.Artifact.BlobDigest || chunk.Size != int64(len(data)) {
			t.Fatalf("unexpected chunk metadata: %+v", chunk)
		}
		reconstructed = append(reconstructed, chunk.Data...)
		offset += int64(len(chunk.Data))
		if chunk.EOF {
			break
		}
	}
	if !bytes.Equal(reconstructed, data) {
		t.Fatal("chunked transfer changed file contents")
	}
	if _, err := service.ReadChatUploadChunk(t.Context(), "space-a", "device-a", "device-a", result.Artifact.ID, int64(len(data))+1, 1); err == nil {
		t.Fatal("out of bounds range accepted")
	}
	contentPath := "/api/artifacts/v1/" + string(result.Artifact.ID) + "/content"
	if response := request(http.MethodGet, contentPath, "space-b", "device-a", nil); response.Code == http.StatusOK {
		t.Fatal("different owner read uploaded file")
	}
	basePath := "/api/artifacts/v1/" + string(result.Artifact.ID)
	for _, operation := range []struct{ method, path string }{
		{http.MethodGet, basePath},
		{http.MethodGet, contentPath},
		{http.MethodGet, basePath + "/media-ticket"},
		{http.MethodDelete, basePath},
	} {
		if response := request(operation.method, operation.path, "space-a", "device-b", nil); response.Code == http.StatusOK {
			t.Fatalf("different device could %s %s", operation.method, operation.path)
		}
	}
	response = request(http.MethodGet, contentPath, "space-a", "device-a", nil)
	if response.Code != http.StatusOK || !bytes.Equal(response.Body.Bytes(), data) {
		t.Fatalf("uploaded bytes not recovered: status %d", response.Code)
	}
	if response := request(http.MethodDelete, "/api/artifacts/v1/"+string(result.Artifact.ID), "space-a", "device-a", nil); response.Code != http.StatusOK {
		t.Fatalf("owner could not delete upload: %d %s", response.Code, response.Body.String())
	}
}
