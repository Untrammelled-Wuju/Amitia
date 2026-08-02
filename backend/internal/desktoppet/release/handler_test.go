package release

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	desktoppetAuth "github.com/u-ai/backend/internal/auth"
	"github.com/u-ai/backend/internal/desktoppet/security"
)

type stubOwnershipGuard struct{}

func (s *stubOwnershipGuard) RequireCharacter(ctx context.Context, actor *desktoppetAuth.ActorContext, characterID string) (*security.CharacterScope, error) {
	return &security.CharacterScope{UserID: actor.UserID, CharacterID: characterID}, nil
}
func (s *stubOwnershipGuard) RequireGenerationTask(ctx context.Context, actor *desktoppetAuth.ActorContext, taskID string) (*security.GenerationTaskScope, error) {
	return &security.GenerationTaskScope{UserID: actor.UserID, TaskID: taskID}, nil
}
func (s *stubOwnershipGuard) RequireProcessingTask(ctx context.Context, actor *desktoppetAuth.ActorContext, taskID string) (*security.ProcessingTaskScope, error) {
	return &security.ProcessingTaskScope{UserID: actor.UserID, TaskID: taskID}, nil
}
func (s *stubOwnershipGuard) RequireActionRevision(ctx context.Context, actor *desktoppetAuth.ActorContext, revisionID string) (*security.ActionRevisionScope, error) {
	return &security.ActionRevisionScope{UserID: actor.UserID, RevisionID: revisionID}, nil
}
func (s *stubOwnershipGuard) RequireQualityEvaluation(ctx context.Context, actor *desktoppetAuth.ActorContext, evaluationID string) (*security.QualityScope, error) {
	return &security.QualityScope{UserID: actor.UserID, EvaluationID: evaluationID}, nil
}
func (s *stubOwnershipGuard) RequireRelease(ctx context.Context, actor *desktoppetAuth.ActorContext, releaseID string) (*security.ReleaseScope, error) {
	return &security.ReleaseScope{UserID: actor.UserID, ReleaseID: releaseID}, nil
}
func (s *stubOwnershipGuard) RequireInstallation(ctx context.Context, actor *desktoppetAuth.ActorContext, deviceID, installationID string) (*security.InstallationScope, error) {
	return &security.InstallationScope{UserID: actor.UserID, InstallationID: installationID}, nil
}
func (s *stubOwnershipGuard) RequireEditSession(ctx context.Context, actor *desktoppetAuth.ActorContext, sessionID string) (*security.EditSessionScope, error) {
	return &security.EditSessionScope{UserID: actor.UserID, SessionID: sessionID}, nil
}
func (s *stubOwnershipGuard) RequireRegenerationJob(ctx context.Context, actor *desktoppetAuth.ActorContext, jobID string) (*security.RegenerationJobScope, error) {
	return &security.RegenerationJobScope{UserID: actor.UserID, JobID: jobID}, nil
}
func (s *stubOwnershipGuard) RequireCandidate(ctx context.Context, actor *desktoppetAuth.ActorContext, candidateID string) (*security.CandidateScope, error) {
	return &security.CandidateScope{UserID: actor.UserID, CandidateID: candidateID}, nil
}
func (s *stubOwnershipGuard) RequireRuntimeCommand(ctx context.Context, actor *desktoppetAuth.ActorContext, commandID string) (*security.RuntimeCommandScope, error) {
	return &security.RuntimeCommandScope{UserID: actor.UserID, CommandID: commandID}, nil
}
func (s *stubOwnershipGuard) RequireBehaviorBinding(ctx context.Context, actor *desktoppetAuth.ActorContext, bindingID string) (*security.BehaviorBindingScope, error) {
	return &security.BehaviorBindingScope{UserID: actor.UserID, BindingID: bindingID}, nil
}

type stubReleaseService struct {
	buildReleaseResult *BuildReleaseResult
	buildReleaseErr    error

	getBuildOperationResult *ReleaseBuildOperation
	getBuildOperationErr    error

	cancelBuildOperationErr error

	listReleasesResult []*ReleaseData
	listReleasesErr    error

	listReleasesForPetResult []*ReleaseData
	listReleasesForPetErr    error

	getReleaseResult *ReleaseData
	getReleaseErr    error

	getReleaseFilesResult []ReleaseFileData
	getReleaseFilesErr    error

	archiveReleaseErr error
	revokeReleaseErr  error

	getPetIdentityResult *PetIdentityData
	getPetIdentityErr    error
}

func (s *stubReleaseService) BuildRelease(ctx context.Context, req *BuildReleaseRequest) (*BuildReleaseResult, error) {
	return s.buildReleaseResult, s.buildReleaseErr
}

func (s *stubReleaseService) GetBuildOperation(ctx context.Context, operationID, userID string) (*ReleaseBuildOperation, error) {
	return s.getBuildOperationResult, s.getBuildOperationErr
}

func (s *stubReleaseService) CancelBuildOperation(ctx context.Context, operationID, userID string) error {
	return s.cancelBuildOperationErr
}

func (s *stubReleaseService) GetRelease(ctx context.Context, releaseID, userID string) (*ReleaseData, error) {
	return s.getReleaseResult, s.getReleaseErr
}

func (s *stubReleaseService) ListReleases(ctx context.Context, userID string) ([]*ReleaseData, error) {
	return s.listReleasesResult, s.listReleasesErr
}

func (s *stubReleaseService) ListReleasesForPet(ctx context.Context, userID, petID string) ([]*ReleaseData, error) {
	return s.listReleasesForPetResult, s.listReleasesForPetErr
}

func (s *stubReleaseService) GetReleaseFiles(ctx context.Context, releaseID, userID string) ([]ReleaseFileData, error) {
	return s.getReleaseFilesResult, s.getReleaseFilesErr
}

func (s *stubReleaseService) ArchiveRelease(ctx context.Context, releaseID, userID string) error {
	return s.archiveReleaseErr
}

func (s *stubReleaseService) RevokeRelease(ctx context.Context, releaseID, userID, reason string) error {
	return s.revokeReleaseErr
}

func (s *stubReleaseService) GetPetIdentity(ctx context.Context, userID, petID string) (*PetIdentityData, error) {
	return s.getPetIdentityResult, s.getPetIdentityErr
}

func newTestRouter(svc ReleaseService) *gin.Engine {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(func(c *gin.Context) {
		c.Set("actorContext", &desktoppetAuth.ActorContext{
			ActorType:   desktoppetAuth.ActorTypeUser,
			UserID:      "test-user-1",
			Roles:       []string{"user"},
			Permissions: desktoppetAuth.DefaultUserPermissions(),
		})
		c.Next()
	})
	RegisterRoutes(r.Group("/api/v2"), svc, &stubOwnershipGuard{})
	return r
}

func doRequest(t *testing.T, r *gin.Engine, method, path string, body interface{}) *httptest.ResponseRecorder {
	t.Helper()
	var req *http.Request
	if body == nil {
		req = httptest.NewRequest(method, path, nil)
	} else {
		data, err := json.Marshal(body)
		if err != nil {
			t.Fatalf("marshal body: %v", err)
		}
		req = httptest.NewRequest(method, path, bytes.NewReader(data))
		req.Header.Set("Content-Type", "application/json")
	}
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	return w
}

type apiResponse struct {
	Code int         `json:"code"`
	Msg  string      `json:"msg"`
	Data interface{} `json:"data"`
}

func parseResponse(t *testing.T, body []byte) apiResponse {
	t.Helper()
	var r apiResponse
	if err := json.Unmarshal(body, &r); err != nil {
		t.Fatalf("parse response: %v", err)
	}
	return r
}

func TestBuildRelease_Returns400_WhenProcessingTaskIDEmpty(t *testing.T) {
	svc := &stubReleaseService{}
	r := newTestRouter(svc)

	w := doRequest(t, r, http.MethodPost, "/api/v2/releases/build", map[string]interface{}{
		"processingTaskId": "",
	})

	resp := parseResponse(t, w.Body.Bytes())
	if resp.Code != http.StatusBadRequest {
		t.Fatalf("expected body code 400, got %d: %s", resp.Code, w.Body.String())
	}
}

func TestBuildRelease_Returns200_OnSuccess(t *testing.T) {
	now := time.Now().Format("2006-01-02 15:04:05")
	svc := &stubReleaseService{
		buildReleaseResult: &BuildReleaseResult{
			Operation: &ReleaseBuildOperation{
				ID:        uuid.NewString(),
				State:     BuildOpStateCompleted,
				StartedAt: now,
			},
			Release: &ReleaseData{
				ID:      uuid.NewString(),
				Version: "1.0.1",
			},
			Snapshot: &ReleaseBuildSequenceInfo{
				Sequence: 1,
				Version:  "1.0.1",
			},
		},
	}
	r := newTestRouter(svc)

	w := doRequest(t, r, http.MethodPost, "/api/v2/releases/build", map[string]interface{}{
		"processingTaskId": "task-1",
	})

	resp := parseResponse(t, w.Body.Bytes())
	if resp.Code != http.StatusOK {
		t.Fatalf("expected body code 200, got %d: %s", resp.Code, w.Body.String())
	}
}

func TestBuildRelease_ReturnsError_OnServiceError(t *testing.T) {
	svc := &stubReleaseService{
		buildReleaseErr: NewReleaseError("INVALID_REQUEST", "missing fields", nil),
	}
	r := newTestRouter(svc)

	w := doRequest(t, r, http.MethodPost, "/api/v2/releases/build", map[string]interface{}{
		"processingTaskId": "",
	})

	resp := parseResponse(t, w.Body.Bytes())
	if resp.Code == http.StatusOK {
		t.Fatalf("expected non-200 body code, got 200")
	}
}

func TestGetRelease_Returns404_WhenNotFound(t *testing.T) {
	svc := &stubReleaseService{
		getReleaseErr: NewReleaseError("RELEASE_NOT_FOUND", "不存在", errors.New("not found")),
	}
	r := newTestRouter(svc)

	w := doRequest(t, r, http.MethodGet, "/api/v2/releases/"+uuid.NewString(), nil)

	resp := parseResponse(t, w.Body.Bytes())
	if resp.Code != http.StatusNotFound {
		t.Fatalf("expected body code 404, got %d: %s", resp.Code, w.Body.String())
	}
}

func TestListReleases_Returns200(t *testing.T) {
	now := time.Now().Format("2006-01-02 15:04:05")
	svc := &stubReleaseService{
		listReleasesResult: []*ReleaseData{
			{ID: uuid.NewString(), Version: "1.0.1", CreatedAt: now},
		},
	}
	r := newTestRouter(svc)

	w := doRequest(t, r, http.MethodGet, "/api/v2/releases/list", nil)

	resp := parseResponse(t, w.Body.Bytes())
	if resp.Code != http.StatusOK {
		t.Fatalf("expected body code 200, got %d: %s", resp.Code, w.Body.String())
	}
}

func TestRevokeRelease_Success(t *testing.T) {
	releaseID := uuid.NewString()
	svc := &stubReleaseService{}
	r := newTestRouter(svc)

	w := doRequest(t, r, http.MethodPost, "/api/v2/releases/"+releaseID+"/revoke?reason=test", nil)

	resp := parseResponse(t, w.Body.Bytes())
	if resp.Code != http.StatusOK {
		t.Fatalf("expected body code 200, got %d: %s", resp.Code, w.Body.String())
	}
}

func TestGetPetIdentity_MissingPetID(t *testing.T) {
	svc := &stubReleaseService{}
	r := newTestRouter(svc)

	w := doRequest(t, r, http.MethodGet, "/api/v2/pets/identity", nil)

	resp := parseResponse(t, w.Body.Bytes())
	if resp.Code != http.StatusBadRequest {
		t.Fatalf("expected body code 400, got %d: %s", resp.Code, w.Body.String())
	}
}

func TestGetBuildOperation_Success(t *testing.T) {
	operationID := uuid.NewString()
	now := time.Now().Format("2006-01-02 15:04:05")
	svc := &stubReleaseService{
		getBuildOperationResult: &ReleaseBuildOperation{
			ID:        operationID,
			State:     BuildOpStateBuilding,
			StartedAt: now,
		},
	}
	r := newTestRouter(svc)

	w := doRequest(t, r, http.MethodGet, "/api/v2/releases/operations/"+operationID, nil)

	resp := parseResponse(t, w.Body.Bytes())
	if resp.Code != http.StatusOK {
		t.Fatalf("expected body code 200, got %d: %s", resp.Code, w.Body.String())
	}
}

func TestCancelBuildOperation_Success(t *testing.T) {
	operationID := uuid.NewString()
	svc := &stubReleaseService{}
	r := newTestRouter(svc)

	w := doRequest(t, r, http.MethodPost, "/api/v2/releases/operations/"+operationID+"/cancel", nil)

	resp := parseResponse(t, w.Body.Bytes())
	if resp.Code != http.StatusOK {
		t.Fatalf("expected body code 200, got %d: %s", resp.Code, w.Body.String())
	}
}

func TestGetReleaseFiles_Success(t *testing.T) {
	releaseID := uuid.NewString()
	svc := &stubReleaseService{
		getReleaseFilesResult: []ReleaseFileData{
			{ActionKey: "idle", Path: "idle.gif"},
		},
	}
	r := newTestRouter(svc)

	w := doRequest(t, r, http.MethodGet, "/api/v2/releases/"+releaseID+"/files", nil)

	resp := parseResponse(t, w.Body.Bytes())
	if resp.Code != http.StatusOK {
		t.Fatalf("expected body code 200, got %d: %s", resp.Code, w.Body.String())
	}
}

func TestListReleasesForPet_Success(t *testing.T) {
	now := time.Now().Format("2006-01-02 15:04:05")
	svc := &stubReleaseService{
		listReleasesForPetResult: []*ReleaseData{
			{ID: uuid.NewString(), Version: "1.0.1", CreatedAt: now},
		},
	}
	r := newTestRouter(svc)

	w := doRequest(t, r, http.MethodGet, "/api/v2/releases/pet/pet-123", nil)

	resp := parseResponse(t, w.Body.Bytes())
	if resp.Code != http.StatusOK {
		t.Fatalf("expected body code 200, got %d: %s", resp.Code, w.Body.String())
	}
}
