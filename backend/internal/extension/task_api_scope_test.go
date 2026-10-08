package extension

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"reflect"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/u-ai/backend/internal/auth"
	"github.com/u-ai/backend/internal/devicemesh/coordination"
	"github.com/u-ai/backend/internal/extension/kernel/task_runtime"
)

func TestTaskArtifactHTTPReadRejectsReplacedOriginalScope(t *testing.T) {
	actual := coordination.ExecutionScope{CoreID: "core", ResourceOwnerID: "source", RoleID: "role", RoleRevision: 2, ProviderEpoch: 3}
	for _, scenario := range []string{"valid", "historical", "owner", "role", "epoch", "missing"} {
		t.Run(scenario, func(t *testing.T) {
			expected := actual
			if scenario == "owner" {
				expected.ResourceOwnerID = "other"
			}
			if scenario == "role" {
				expected.RoleRevision--
			}
			if scenario == "epoch" {
				expected.ProviderEpoch--
			}
			if scenario == "historical" {
				expected.RequestID = "old-read"
			}
			encoded, _ := json.Marshal(expected)
			path := "/tasks/run/result/artifact"
			if scenario != "missing" {
				path += "?expectedExecutionScope=" + url.QueryEscape(string(encoded))
			}
			router := gin.New()
			router.GET("/tasks/run/result/artifact", func(c *gin.Context) {
				if validateTaskReadIntent(c, coordination.WithScope(t.Context(), actual)) {
					c.Data(200, "application/octet-stream", []byte("owned artifact"))
				}
			})
			recorder := httptest.NewRecorder()
			router.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, path, nil))
			status := 409
			if scenario == "valid" || scenario == "historical" || scenario == "missing" {
				status = 200
			}
			if recorder.Code != status {
				t.Fatalf("status %d, expected %d", recorder.Code, status)
			}
		})
	}
}

func TestTaskControlHTTPRequiresOriginalAuthorityAndPreservesHandlerBody(t *testing.T) {
	actual := coordination.ExecutionScope{SpaceID: "core", AuthorizationRealm: "core", CoreID: "core", InitiatorDeviceID: "phone", TargetDeviceID: "source", ResourceOwnerID: "phone", RoleOwnerID: "source", RoleID: "role", RoleRevision: 2, ProviderEpoch: 3, TargetProviderEpoch: 4, ModeRevision: 2, PermissionRevision: 5, TargetPermissionRevision: 6}
	for _, scenario := range []string{"missing", "foreign_owner", "same_core_aba", "realm", "target_permission", "valid", "operation_ids", "local_compatible", "malformed", "oversize"} {
		t.Run(scenario, func(t *testing.T) {
			ctx := coordination.WithScope(t.Context(), actual)
			actor := &auth.ActorContext{PrincipalType: auth.PrincipalTrustedDevice}
			if scenario == "local_compatible" {
				actor.PrincipalType = auth.PrincipalLocalUI
			}
			ctx = auth.WithActor(ctx, actor)
			expected := actual
			switch scenario {
			case "foreign_owner":
				expected.ResourceOwnerID = "other"
			case "same_core_aba":
				expected.ProviderEpoch--
			case "realm":
				expected.AuthorizationRealm = "other"
			case "target_permission":
				expected.TargetPermissionRevision--
			case "operation_ids":
				expected.RequestID, expected.TurnID, expected.ExecutionID = "request", "turn", "execution"
			}
			body := map[string]any{"generation": 7}
			if scenario != "missing" && scenario != "local_compatible" {
				body["expectedExecutionScope"] = expected
			}
			encoded, err := json.Marshal(body)
			if err != nil {
				t.Fatal(err)
			}
			if scenario == "malformed" {
				encoded = []byte("{")
			}
			if scenario == "oversize" {
				encoded = bytes.Repeat([]byte(" "), 16<<10+1)
			}
			router := gin.New()
			router.POST("/tasks/run/pause", func(c *gin.Context) {
				if !validateTaskControlIntent(c, ctx) {
					return
				}
				var request map[string]any
				if c.ShouldBindJSON(&request) != nil || request["generation"] != float64(7) {
					c.Status(http.StatusBadRequest)
					return
				}
				c.Status(http.StatusOK)
			})
			recorder := httptest.NewRecorder()
			request := httptest.NewRequest(http.MethodPost, "/tasks/run/pause", bytes.NewReader(encoded))
			request.Header.Set("Content-Type", "application/json")
			router.ServeHTTP(recorder, request)
			status := http.StatusConflict
			if scenario == "valid" || scenario == "operation_ids" || scenario == "local_compatible" {
				status = http.StatusOK
			}
			if scenario == "malformed" || scenario == "oversize" {
				status = http.StatusBadRequest
			}
			if recorder.Code != status {
				t.Fatalf("status %d, expected %d: %s", recorder.Code, status, recorder.Body.String())
			}
		})
	}
}

func TestTaskControlScopeRejectsEveryChangedAuthorityField(t *testing.T) {
	original := coordination.ExecutionScope{SpaceID: "core", AuthorizationRealm: "core", CoreID: "core", InitiatorDeviceID: "a", TargetDeviceID: "a", ProviderEpoch: 1, TargetProviderEpoch: 1, ModeRevision: 1, PermissionRevision: 1, TargetPermissionRevision: 1, RoleID: "role", RoleRevision: 1, RoleOwnerID: "a", ResourceOwnerID: "a", RequestID: "read", TurnID: "turn", ExecutionID: "execution"}
	for index := 0; index < reflect.TypeOf(original).NumField(); index++ {
		field := reflect.TypeOf(original).Field(index)
		changed := original
		value := reflect.ValueOf(&changed).Elem().Field(index)
		switch value.Kind() {
		case reflect.String:
			value.SetString(value.String() + "-changed")
		case reflect.Int64:
			value.SetInt(value.Int() + 1)
		case reflect.Bool:
			value.SetBool(!value.Bool())
		default:
			t.Fatalf("unsupported scope field %s", field.Name)
		}
		expected := field.Name == "RequestID" || field.Name == "TurnID" || field.Name == "ExecutionID"
		if taskScopeEqual(original, changed) != expected {
			t.Errorf("scope comparison accepted changed %s", field.Name)
		}
	}
}

func TestTaskAuthorityPresentationPreservesOwnerAndOriginalScope(t *testing.T) {
	scope := coordination.ExecutionScope{CoreID: "core", ResourceOwnerID: "source", RoleID: "role", ProviderEpoch: 2}
	ctx := coordination.WithScope(context.Background(), scope)
	payload, err := taskAuthorityPayload(ctx, map[string]any{"taskRunId": "same", "resultArtifactId": "artifact"}, true)
	if err != nil {
		t.Fatal(err)
	}
	var decoded coordination.ExecutionScope
	if json.Unmarshal(payload["executionScope"], &decoded) != nil || decoded != scope || string(payload["ownerId"]) != `"source"` || string(payload["readOnly"]) != "true" || string(payload["resultArtifactId"]) != `"artifact"` || !bytes.Equal(payload["executionScope"], payload["managementExecutionScope"]) {
		t.Fatalf("invalid owned presentation: %#v", payload)
	}
}

func TestTaskAuthorityHTTPResultPreservesOriginalJSONAndHash(t *testing.T) {
	original := json.RawMessage(`{"z":"last","first":{"b":2,"a":1},"binary":"YWJj","large":9007199254740993}`)
	digest := sha256.Sum256(original)
	result := task_runtime.TaskRunResult{TaskRunID: "run", ResultJSON: original, ResultHash: hex.EncodeToString(digest[:])}
	ctx := coordination.WithScope(t.Context(), coordination.ExecutionScope{CoreID: "core", ResourceOwnerID: "source"})
	router := gin.New()
	router.GET("/result", func(c *gin.Context) {
		payload, err := taskAuthorityPayload(ctx, result, true)
		if err != nil {
			t.Fatal(err)
		}
		c.JSON(200, payload)
	})
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/result", nil))
	var decoded task_runtime.TaskRunResult
	if json.Unmarshal(recorder.Body.Bytes(), &decoded) != nil || !bytes.Equal(decoded.ResultJSON, original) || decoded.ResultHash != result.ResultHash {
		t.Fatalf("original result changed: %s", recorder.Body.String())
	}
	actual := sha256.Sum256(decoded.ResultJSON)
	if hex.EncodeToString(actual[:]) != decoded.ResultHash {
		t.Fatal("HTTP result no longer matches artifact hash")
	}
}
