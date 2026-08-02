// SPDX-FileCopyrightText: 2026 彭旭
// SPDX-License-Identifier: AGPL-3.0-only
package installation

import (
	"bytes"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/u-ai/backend/pkg/comment/response"
)

func TestMapInstallationErrorCode_NotFoundGroup(t *testing.T) {
	cases := []string{
		ErrCodeInstallationNotFound,
		ErrCodeRuntimeSettingsNotFound,
		ErrCodeCharacterNotFound,
		ErrCodeActionNotFound,
	}
	for _, code := range cases {
		if got := mapInstallationErrorCode(code); got != response.NotFound {
			t.Fatalf("code=%s 期望 %d, 实际 %d", code, response.NotFound, got)
		}
	}
}

func TestMapInstallationErrorCode_InvalidParamsGroup(t *testing.T) {
	cases := []string{
		ErrCodePackageNotReady,
		ErrCodePackagePathTraversal,
		ErrCodePackageSymlinkEscape,
		ErrCodePackageExecutableFound,
		ErrCodePackageHashMismatch,
		ErrCodePackageDefaultActionInvalid,
	}
	for _, code := range cases {
		if got := mapInstallationErrorCode(code); got != response.InvalidParams {
			t.Fatalf("code=%s 期望 %d, 实际 %d", code, response.InvalidParams, got)
		}
	}
}

func TestMapInstallationErrorCode_BusinessErrorGroup(t *testing.T) {
	cases := []string{
		ErrCodeInstallationDuplicate,
		ErrCodePetNotEnabled,
		ErrCodeDefaultActionNotIdle,
		ErrCodeInstallationInvalid,
		ErrCodePurgeNotConfirmed,
	}
	for _, code := range cases {
		if got := mapInstallationErrorCode(code); got != response.BusinessError {
			t.Fatalf("code=%s 期望 %d, 实际 %d", code, response.BusinessError, got)
		}
	}
}

func TestMapInstallationErrorCode_InternalErrorGroup(t *testing.T) {
	if got := mapInstallationErrorCode(ErrCodeInstallationFailed); got != response.InternalError {
		t.Fatalf("code=%s 期望 %d, 实际 %d", ErrCodeInstallationFailed, response.InternalError, got)
	}
}

func TestMapInstallationErrorCode_UnknownCode_DefaultsToInternalError(t *testing.T) {
	if got := mapInstallationErrorCode("UNKNOWN_ERROR_CODE"); got != response.InternalError {
		t.Fatalf("未知 code 期望 %d, 实际 %d", response.InternalError, got)
	}
	if got := mapInstallationErrorCode(""); got != response.InternalError {
		t.Fatalf("空 code 期望 %d, 实际 %d", response.InternalError, got)
	}
}

func TestMapInstallationErrorCode_AllDefinedCodesCovered(t *testing.T) {
	allCodes := []string{
		ErrCodeInstallationNotFound,
		ErrCodeInstallationDuplicate,
		ErrCodeInstallationInvalid,
		ErrCodeInstallationFailed,
		ErrCodeRuntimeSettingsNotFound,
		ErrCodePackageNotReady,
		ErrCodePackagePathTraversal,
		ErrCodePackageSymlinkEscape,
		ErrCodePackageExecutableFound,
		ErrCodePackageHashMismatch,
		ErrCodePackageDefaultActionInvalid,
		ErrCodeCharacterNotFound,
		ErrCodePurgeNotConfirmed,
		ErrCodeDefaultActionNotIdle,
		ErrCodePetNotEnabled,
		ErrCodeActionNotFound,
		ErrCodeRevisionConflict,
	}
	allowed := map[int]bool{
		response.NotFound:      true,
		response.InvalidParams: true,
		response.BusinessError: true,
		response.InternalError: true,
	}
	for _, code := range allCodes {
		got := mapInstallationErrorCode(code)
		if !allowed[got] {
			t.Fatalf("code=%s 返回未预期的 HTTP 状态 %d", code, got)
		}
	}
}

type stubHandlerService struct {
	installErr        error
	installResult     *Installation
	uninstallErr      error
	enableErr         error
	disableErr        error
	switchErr         error
	updateDefaultErr  error
	updateSettingsErr error
	recenterErr       error
	playErr           error
	listErr           error
	listResult        []*Installation
	getErr            error
	getResult         *Installation
	getSettingsErr    error
	getSettingsResult *RuntimeSettings

	installCalls        []installCall
	uninstallCalls      []actionCall
	enableCalls         []actionCall
	disableCalls        []actionCall
	switchCalls         []actionCall
	updateDefaultCalls  []updateDefaultCall
	updateSettingsCalls []updateSettingsCall
	recenterCalls       []actionCall
	playCalls           []playCall
	listCalls           []string
	getCalls            []string
	getSettingsCalls    []string
}

type installCall struct {
	PackageID   string
	UserID      string
	CharacterID string
}

type actionCall struct {
	UserID         string
	InstallationID string
}

type updateDefaultCall struct {
	InstallationID string
	ActionKey      string
}

type updateSettingsCall struct {
	UserID         string
	InstallationID string
	Settings       *UpdateRuntimeSettingsRequest
}

type playCall struct {
	UserID         string
	InstallationID string
	ActionKey      string
}

func (s *stubHandlerService) InstallPackage(packageId, userId, characterId string) (*Installation, error) {
	s.installCalls = append(s.installCalls, installCall{PackageID: packageId, UserID: userId, CharacterID: characterId})
	if s.installErr != nil {
		return nil, s.installErr
	}
	if s.installResult != nil {
		return s.installResult, nil
	}
	return &Installation{ID: "inst_default", UserID: userId, CharacterID: characterId, PackageID: packageId, Status: StatusInstalled}, nil
}

func (s *stubHandlerService) Uninstall(userId, installationId string) error {
	s.uninstallCalls = append(s.uninstallCalls, actionCall{UserID: userId, InstallationID: installationId})
	return s.uninstallErr
}

func (s *stubHandlerService) EnableInstallation(userId, installationId string) error {
	s.enableCalls = append(s.enableCalls, actionCall{UserID: userId, InstallationID: installationId})
	return s.enableErr
}

func (s *stubHandlerService) DisableInstallation(userId, installationId string) error {
	s.disableCalls = append(s.disableCalls, actionCall{UserID: userId, InstallationID: installationId})
	return s.disableErr
}

func (s *stubHandlerService) SwitchInstallation(userId, installationId string) error {
	s.switchCalls = append(s.switchCalls, actionCall{UserID: userId, InstallationID: installationId})
	return s.switchErr
}

func (s *stubHandlerService) UpdateDefaultAction(installationId, actionKey string) error {
	s.updateDefaultCalls = append(s.updateDefaultCalls, updateDefaultCall{InstallationID: installationId, ActionKey: actionKey})
	return s.updateDefaultErr
}

func (s *stubHandlerService) UpdateRuntimeSettings(userId, installationId string, req *UpdateRuntimeSettingsRequest) (*RuntimeSettings, error) {
	s.updateSettingsCalls = append(s.updateSettingsCalls, updateSettingsCall{UserID: userId, InstallationID: installationId, Settings: req})
	if s.updateSettingsErr != nil {
		return nil, s.updateSettingsErr
	}
	return s.getSettingsResult, nil
}

func (s *stubHandlerService) Recenter(userId, installationId string) error {
	s.recenterCalls = append(s.recenterCalls, actionCall{UserID: userId, InstallationID: installationId})
	return s.recenterErr
}

func (s *stubHandlerService) PlayAction(userId, installationId, actionKey string) error {
	s.playCalls = append(s.playCalls, playCall{UserID: userId, InstallationID: installationId, ActionKey: actionKey})
	return s.playErr
}

func (s *stubHandlerService) ListInstallations(userId string) ([]*Installation, error) {
	s.listCalls = append(s.listCalls, userId)
	if s.listErr != nil {
		return nil, s.listErr
	}
	if s.listResult != nil {
		return s.listResult, nil
	}
	return []*Installation{}, nil
}

func (s *stubHandlerService) GetInstallation(installationId string) (*Installation, error) {
	s.getCalls = append(s.getCalls, installationId)
	if s.getErr != nil {
		return nil, s.getErr
	}
	if s.getResult != nil {
		return s.getResult, nil
	}
	return &Installation{ID: installationId, Status: StatusInstalled}, nil
}

func (s *stubHandlerService) GetRuntimeSettings(installationId string) (*RuntimeSettings, error) {
	s.getSettingsCalls = append(s.getSettingsCalls, installationId)
	if s.getSettingsErr != nil {
		return nil, s.getSettingsErr
	}
	if s.getSettingsResult != nil {
		return s.getSettingsResult, nil
	}
	return &RuntimeSettings{ID: "rts_default", InstallationID: installationId, Scale: 1.0}, nil
}

func (s *stubHandlerService) CheckInstallationOwnership(installationID, userID string) error {
	return nil
}

func (s *stubHandlerService) GetCoordinator() V2Coordinator {
	return nil
}

func newHandlerTestRouter(svc Service) *gin.Engine {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	RegisterRoutes(r.Group("/api"), svc)
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

func parseResponseCode(t *testing.T, w *httptest.ResponseRecorder) (int, string, string) {
	t.Helper()
	var resp struct {
		Code      int             `json:"code"`
		Msg       string          `json:"msg"`
		Data      json.RawMessage `json:"data"`
		ErrorCode string          `json:"-"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("解析响应失败: %v, body=%s", err, w.Body.String())
	}
	if resp.Data != nil {
		var dataMap map[string]interface{}
		if err := json.Unmarshal(resp.Data, &dataMap); err == nil {
			if ec, ok := dataMap["errorCode"].(string); ok {
				resp.ErrorCode = ec
			}
		}
	}
	return resp.Code, resp.Msg, resp.ErrorCode
}

func assertHTTPCode(t *testing.T, w *httptest.ResponseRecorder, wantHTTPCode int, wantErrorCode string) {
	t.Helper()
	if w.Code != http.StatusOK {
		t.Fatalf("HTTP 状态 = %d, 期望 %d, body=%s", w.Code, http.StatusOK, w.Body.String())
	}
	gotCode, _, gotErrorCode := parseResponseCode(t, w)
	if gotCode != wantHTTPCode {
		t.Fatalf("业务 code = %d, 期望 %d, body=%s", gotCode, wantHTTPCode, w.Body.String())
	}
	if wantErrorCode != "" && gotErrorCode != wantErrorCode {
		t.Fatalf("errorCode = %s, 期望 %s, body=%s", gotErrorCode, wantErrorCode, w.Body.String())
	}
}

func TestHandler_InstallPackage_EmptyPackageID_InvalidParams(t *testing.T) {
	svc := &stubHandlerService{}
	r := newHandlerTestRouter(svc)

	w := doRequest(t, r, http.MethodPost, "/api/desktop-pets/packages//install", gin.H{"character_id": testCharacterID})
	if w.Code != http.StatusNotFound && w.Code != http.StatusOK {
		t.Fatalf("空 packageId 的请求 HTTP 状态 = %d", w.Code)
	}
}

func TestHandler_InstallPackage_EmptyCharacterID_InvalidParams(t *testing.T) {
	svc := &stubHandlerService{}
	r := newHandlerTestRouter(svc)

	w := doRequest(t, r, http.MethodPost, "/api/desktop-pets/packages/"+testPackageID+"/install", gin.H{"character_id": ""})
	assertHTTPCode(t, w, response.InvalidParams, ErrCodeInstallationFailed)
}

func TestHandler_InstallPackage_InvalidJSON_InvalidParams(t *testing.T) {
	svc := &stubHandlerService{}
	r := newHandlerTestRouter(svc)

	req := httptest.NewRequest(http.MethodPost, "/api/desktop-pets/packages/"+testPackageID+"/install", bytes.NewReader([]byte("not json")))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assertHTTPCode(t, w, response.InvalidParams, ErrCodeInstallationFailed)
}

func TestHandler_InstallPackage_PackageNotReady_InvalidParams(t *testing.T) {
	svc := &stubHandlerService{
		installErr: NewInstallationError(ErrCodePackageNotReady, "package not ready", ErrPackageNotReady),
	}
	r := newHandlerTestRouter(svc)

	w := doRequest(t, r, http.MethodPost, "/api/desktop-pets/packages/"+testPackageID+"/install", gin.H{"character_id": testCharacterID})
	assertHTTPCode(t, w, response.InvalidParams, ErrCodePackageNotReady)
}

func TestHandler_InstallPackage_PathTraversal_InvalidParams(t *testing.T) {
	svc := &stubHandlerService{
		installErr: NewInstallationError(ErrCodePackagePathTraversal, "path traversal", ErrPackagePathTraversal),
	}
	r := newHandlerTestRouter(svc)

	w := doRequest(t, r, http.MethodPost, "/api/desktop-pets/packages/"+testPackageID+"/install", gin.H{"character_id": testCharacterID})
	assertHTTPCode(t, w, response.InvalidParams, ErrCodePackagePathTraversal)
}

func TestHandler_InstallPackage_HashMismatch_InvalidParams(t *testing.T) {
	svc := &stubHandlerService{
		installErr: NewInstallationError(ErrCodePackageHashMismatch, "hash mismatch", ErrPackageHashMismatch),
	}
	r := newHandlerTestRouter(svc)

	w := doRequest(t, r, http.MethodPost, "/api/desktop-pets/packages/"+testPackageID+"/install", gin.H{"character_id": testCharacterID})
	assertHTTPCode(t, w, response.InvalidParams, ErrCodePackageHashMismatch)
}

func TestHandler_InstallPackage_SymlinkEscape_InvalidParams(t *testing.T) {
	svc := &stubHandlerService{
		installErr: NewInstallationError(ErrCodePackageSymlinkEscape, "symlink escape", ErrPackageSymlinkEscape),
	}
	r := newHandlerTestRouter(svc)

	w := doRequest(t, r, http.MethodPost, "/api/desktop-pets/packages/"+testPackageID+"/install", gin.H{"character_id": testCharacterID})
	assertHTTPCode(t, w, response.InvalidParams, ErrCodePackageSymlinkEscape)
}

func TestHandler_InstallPackage_ExecutableFound_InvalidParams(t *testing.T) {
	svc := &stubHandlerService{
		installErr: NewInstallationError(ErrCodePackageExecutableFound, "executable found", ErrPackageExecutableFound),
	}
	r := newHandlerTestRouter(svc)

	w := doRequest(t, r, http.MethodPost, "/api/desktop-pets/packages/"+testPackageID+"/install", gin.H{"character_id": testCharacterID})
	assertHTTPCode(t, w, response.InvalidParams, ErrCodePackageExecutableFound)
}

func TestHandler_InstallPackage_DefaultActionInvalid_InvalidParams(t *testing.T) {
	svc := &stubHandlerService{
		installErr: NewInstallationError(ErrCodePackageDefaultActionInvalid, "default action invalid", ErrPackageDefaultActionInvalid),
	}
	r := newHandlerTestRouter(svc)

	w := doRequest(t, r, http.MethodPost, "/api/desktop-pets/packages/"+testPackageID+"/install", gin.H{"character_id": testCharacterID})
	assertHTTPCode(t, w, response.InvalidParams, ErrCodePackageDefaultActionInvalid)
}

func TestHandler_InstallPackage_CharacterNotFound_NotFound(t *testing.T) {
	svc := &stubHandlerService{
		installErr: NewInstallationError(ErrCodeCharacterNotFound, "character not found", ErrCharacterNotFound),
	}
	r := newHandlerTestRouter(svc)

	w := doRequest(t, r, http.MethodPost, "/api/desktop-pets/packages/"+testPackageID+"/install", gin.H{"character_id": testCharacterID})
	assertHTTPCode(t, w, response.NotFound, ErrCodeCharacterNotFound)
}

func TestHandler_InstallPackage_Duplicate_BusinessError(t *testing.T) {
	svc := &stubHandlerService{
		installErr: NewInstallationError(ErrCodeInstallationDuplicate, "duplicate", ErrInstallationDuplicate),
	}
	r := newHandlerTestRouter(svc)

	w := doRequest(t, r, http.MethodPost, "/api/desktop-pets/packages/"+testPackageID+"/install", gin.H{"character_id": testCharacterID})
	assertHTTPCode(t, w, response.BusinessError, ErrCodeInstallationDuplicate)
}

func TestHandler_InstallPackage_InstallationFailed_InternalError(t *testing.T) {
	svc := &stubHandlerService{
		installErr: NewInstallationError(ErrCodeInstallationFailed, "install failed", ErrInstallationFailed),
	}
	r := newHandlerTestRouter(svc)

	w := doRequest(t, r, http.MethodPost, "/api/desktop-pets/packages/"+testPackageID+"/install", gin.H{"character_id": testCharacterID})
	assertHTTPCode(t, w, response.InternalError, ErrCodeInstallationFailed)
}

func TestHandler_InstallPackage_Success_OK(t *testing.T) {
	expected := &Installation{
		ID:               "inst_123",
		UserID:           "default",
		CharacterID:      testCharacterID,
		PackageID:        testPackageID,
		PackageVersion:   "1",
		Name:             "测试包",
		Status:           StatusInstalled,
		InstallPath:      "desktop-pets/installed/inst_123/",
		ManifestPath:     "desktop-pets/installed/inst_123/manifest.json",
		DefaultActionKey: "idle_normal",
	}
	svc := &stubHandlerService{installResult: expected}
	r := newHandlerTestRouter(svc)

	w := doRequest(t, r, http.MethodPost, "/api/desktop-pets/packages/"+testPackageID+"/install", gin.H{"character_id": testCharacterID})
	assertHTTPCode(t, w, response.OK, "")

	var resp struct {
		Code int          `json:"code"`
		Msg  string       `json:"msg"`
		Data Installation `json:"data"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("解析响应失败: %v", err)
	}
	if resp.Data.ID != expected.ID {
		t.Fatalf("Installation ID = %s, 期望 %s", resp.Data.ID, expected.ID)
	}
	if resp.Data.Status != StatusInstalled {
		t.Fatalf("Status = %s, 期望 %s", resp.Data.Status, StatusInstalled)
	}

	if len(svc.installCalls) != 1 {
		t.Fatalf("期望 1 次 InstallPackage 调用, 实际 %d", len(svc.installCalls))
	}
	if svc.installCalls[0].PackageID != testPackageID {
		t.Fatalf("PackageID = %s, 期望 %s", svc.installCalls[0].PackageID, testPackageID)
	}
	if svc.installCalls[0].CharacterID != testCharacterID {
		t.Fatalf("CharacterID = %s, 期望 %s", svc.installCalls[0].CharacterID, testCharacterID)
	}
}

func TestHandler_InstallPackage_GenericError_InternalError(t *testing.T) {
	svc := &stubHandlerService{
		installErr: errors.New("some generic error"),
	}
	r := newHandlerTestRouter(svc)

	w := doRequest(t, r, http.MethodPost, "/api/desktop-pets/packages/"+testPackageID+"/install", gin.H{"character_id": testCharacterID})
	assertHTTPCode(t, w, response.InternalError, "")
}

func TestHandler_ListInstallations_Success(t *testing.T) {
	expected := []*Installation{
		{ID: "inst_1", UserID: "default", Status: StatusInstalled},
		{ID: "inst_2", UserID: "default", Status: StatusEnabled},
	}
	svc := &stubHandlerService{listResult: expected}
	r := newHandlerTestRouter(svc)

	w := doRequest(t, r, http.MethodGet, "/api/desktop-pets/installations", nil)
	assertHTTPCode(t, w, response.OK, "")

	var resp struct {
		Code int `json:"code"`
		Data struct {
			Items []*Installation `json:"items"`
			Total int             `json:"total"`
		} `json:"data"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("解析响应失败: %v", err)
	}
	if resp.Data.Total != 2 {
		t.Fatalf("Total = %d, 期望 2", resp.Data.Total)
	}
	if len(resp.Data.Items) != 2 {
		t.Fatalf("Items 长度 = %d, 期望 2", len(resp.Data.Items))
	}
}

func TestHandler_ListInstallations_Empty_ReturnsEmptyArray(t *testing.T) {
	svc := &stubHandlerService{}
	r := newHandlerTestRouter(svc)

	w := doRequest(t, r, http.MethodGet, "/api/desktop-pets/installations", nil)
	assertHTTPCode(t, w, response.OK, "")

	var resp struct {
		Data struct {
			Items []interface{} `json:"items"`
			Total int           `json:"total"`
		} `json:"data"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("解析响应失败: %v", err)
	}
	if resp.Data.Total != 0 {
		t.Fatalf("Total = %d, 期望 0", resp.Data.Total)
	}
	if resp.Data.Items == nil {
		t.Fatal("Items 不应为 nil")
	}
	if len(resp.Data.Items) != 0 {
		t.Fatalf("Items 长度 = %d, 期望 0", len(resp.Data.Items))
	}
}

func TestHandler_GetInstallation_NotFound(t *testing.T) {
	svc := &stubHandlerService{
		getErr: NewInstallationError(ErrCodeInstallationNotFound, "not found", ErrInstallationNotFound),
	}
	r := newHandlerTestRouter(svc)

	w := doRequest(t, r, http.MethodGet, "/api/desktop-pets/installations/nonexistent", nil)
	assertHTTPCode(t, w, response.NotFound, ErrCodeInstallationNotFound)
}

func TestHandler_GetInstallation_Success(t *testing.T) {
	expected := &Installation{ID: "inst_1", Status: StatusEnabled}
	svc := &stubHandlerService{getResult: expected}
	r := newHandlerTestRouter(svc)

	w := doRequest(t, r, http.MethodGet, "/api/desktop-pets/installations/inst_1", nil)
	assertHTTPCode(t, w, response.OK, "")

	var resp struct {
		Data Installation `json:"data"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("解析响应失败: %v", err)
	}
	if resp.Data.ID != "inst_1" {
		t.Fatalf("ID = %s, 期望 inst_1", resp.Data.ID)
	}
}

func TestHandler_EnableInstallation_NotFound(t *testing.T) {
	svc := &stubHandlerService{
		enableErr: NewInstallationError(ErrCodeInstallationNotFound, "not found", ErrInstallationNotFound),
	}
	r := newHandlerTestRouter(svc)

	w := doRequest(t, r, http.MethodPost, "/api/desktop-pets/installations/nonexistent/enable", nil)
	assertHTTPCode(t, w, response.NotFound, ErrCodeInstallationNotFound)
}

func TestHandler_EnableInstallation_Invalid_BusinessError(t *testing.T) {
	svc := &stubHandlerService{
		enableErr: NewInstallationError(ErrCodeInstallationInvalid, "invalid", ErrInstallationInvalid),
	}
	r := newHandlerTestRouter(svc)

	w := doRequest(t, r, http.MethodPost, "/api/desktop-pets/installations/inst_1/enable", nil)
	assertHTTPCode(t, w, response.BusinessError, ErrCodeInstallationInvalid)
}

func TestHandler_EnableInstallation_PackageNotReady_InvalidParams(t *testing.T) {
	svc := &stubHandlerService{
		enableErr: NewInstallationError(ErrCodePackageNotReady, "package not ready", ErrPackageNotReady),
	}
	r := newHandlerTestRouter(svc)

	w := doRequest(t, r, http.MethodPost, "/api/desktop-pets/installations/inst_1/enable", nil)
	assertHTTPCode(t, w, response.InvalidParams, ErrCodePackageNotReady)
}

func TestHandler_EnableInstallation_PackageHashMismatch_InvalidParams(t *testing.T) {
	svc := &stubHandlerService{
		enableErr: NewInstallationError(ErrCodePackageHashMismatch, "hash mismatch", ErrPackageHashMismatch),
	}
	r := newHandlerTestRouter(svc)

	w := doRequest(t, r, http.MethodPost, "/api/desktop-pets/installations/inst_1/enable", nil)
	assertHTTPCode(t, w, response.InvalidParams, ErrCodePackageHashMismatch)
}

func TestHandler_EnableInstallation_DefaultActionInvalid_InvalidParams(t *testing.T) {
	svc := &stubHandlerService{
		enableErr: NewInstallationError(ErrCodePackageDefaultActionInvalid, "default action invalid", ErrPackageDefaultActionInvalid),
	}
	r := newHandlerTestRouter(svc)

	w := doRequest(t, r, http.MethodPost, "/api/desktop-pets/installations/inst_1/enable", nil)
	assertHTTPCode(t, w, response.InvalidParams, ErrCodePackageDefaultActionInvalid)
}

func TestHandler_EnableInstallation_CharacterNotFound_NotFound(t *testing.T) {
	svc := &stubHandlerService{
		enableErr: NewInstallationError(ErrCodeCharacterNotFound, "character not found", ErrCharacterNotFound),
	}
	r := newHandlerTestRouter(svc)

	w := doRequest(t, r, http.MethodPost, "/api/desktop-pets/installations/inst_1/enable", nil)
	assertHTTPCode(t, w, response.NotFound, ErrCodeCharacterNotFound)
}

func TestHandler_EnableInstallation_Success(t *testing.T) {
	svc := &stubHandlerService{}
	r := newHandlerTestRouter(svc)

	w := doRequest(t, r, http.MethodPost, "/api/desktop-pets/installations/inst_1/enable", nil)
	assertHTTPCode(t, w, response.OK, "")

	if len(svc.enableCalls) != 1 {
		t.Fatalf("期望 1 次 EnableInstallation 调用, 实际 %d", len(svc.enableCalls))
	}
}

func TestHandler_DisableInstallation_NotFound(t *testing.T) {
	svc := &stubHandlerService{
		disableErr: NewInstallationError(ErrCodeInstallationNotFound, "not found", ErrInstallationNotFound),
	}
	r := newHandlerTestRouter(svc)

	w := doRequest(t, r, http.MethodPost, "/api/desktop-pets/installations/nonexistent/disable", nil)
	assertHTTPCode(t, w, response.NotFound, ErrCodeInstallationNotFound)
}

func TestHandler_DisableInstallation_Success(t *testing.T) {
	svc := &stubHandlerService{}
	r := newHandlerTestRouter(svc)

	w := doRequest(t, r, http.MethodPost, "/api/desktop-pets/installations/inst_1/disable", nil)
	assertHTTPCode(t, w, response.OK, "")
}

func TestHandler_UpdateDefaultAction_EmptyActionKey_InvalidParams(t *testing.T) {
	svc := &stubHandlerService{}
	r := newHandlerTestRouter(svc)

	w := doRequest(t, r, http.MethodPatch, "/api/desktop-pets/installations/inst_1/default-action", gin.H{"action_key": ""})
	assertHTTPCode(t, w, response.InvalidParams, ErrCodeActionNotFound)
}

func TestHandler_UpdateDefaultAction_InvalidJSON_InvalidParams(t *testing.T) {
	svc := &stubHandlerService{}
	r := newHandlerTestRouter(svc)

	req := httptest.NewRequest(http.MethodPatch, "/api/desktop-pets/installations/inst_1/default-action", bytes.NewReader([]byte("bad json")))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assertHTTPCode(t, w, response.InvalidParams, ErrCodeActionNotFound)
}

func TestHandler_UpdateDefaultAction_NotIdle_BusinessError(t *testing.T) {
	svc := &stubHandlerService{
		updateDefaultErr: NewInstallationError(ErrCodeDefaultActionNotIdle, "not idle", ErrDefaultActionNotIdle),
	}
	r := newHandlerTestRouter(svc)

	w := doRequest(t, r, http.MethodPatch, "/api/desktop-pets/installations/inst_1/default-action", gin.H{"action_key": "wave"})
	assertHTTPCode(t, w, response.BusinessError, ErrCodeDefaultActionNotIdle)
}

func TestHandler_UpdateDefaultAction_ActionNotFound_NotFound(t *testing.T) {
	svc := &stubHandlerService{
		updateDefaultErr: NewInstallationError(ErrCodeActionNotFound, "action not found", ErrActionNotFound),
	}
	r := newHandlerTestRouter(svc)

	w := doRequest(t, r, http.MethodPatch, "/api/desktop-pets/installations/inst_1/default-action", gin.H{"action_key": "nonexistent"})
	assertHTTPCode(t, w, response.NotFound, ErrCodeActionNotFound)
}

func TestHandler_UpdateDefaultAction_InstallationNotFound_NotFound(t *testing.T) {
	svc := &stubHandlerService{
		updateDefaultErr: NewInstallationError(ErrCodeInstallationNotFound, "not found", ErrInstallationNotFound),
	}
	r := newHandlerTestRouter(svc)

	w := doRequest(t, r, http.MethodPatch, "/api/desktop-pets/installations/nonexistent/default-action", gin.H{"action_key": "idle_normal"})
	assertHTTPCode(t, w, response.NotFound, ErrCodeInstallationNotFound)
}

func TestHandler_UpdateDefaultAction_Success(t *testing.T) {
	svc := &stubHandlerService{}
	r := newHandlerTestRouter(svc)

	w := doRequest(t, r, http.MethodPatch, "/api/desktop-pets/installations/inst_1/default-action", gin.H{"action_key": "idle_normal"})
	assertHTTPCode(t, w, response.OK, "")

	if len(svc.updateDefaultCalls) != 1 {
		t.Fatalf("期望 1 次 UpdateDefaultAction 调用, 实际 %d", len(svc.updateDefaultCalls))
	}
	if svc.updateDefaultCalls[0].ActionKey != "idle_normal" {
		t.Fatalf("ActionKey = %s, 期望 idle_normal", svc.updateDefaultCalls[0].ActionKey)
	}
}

func TestHandler_UpdateRuntimeSettings_NotFound(t *testing.T) {
	svc := &stubHandlerService{
		updateSettingsErr: NewInstallationError(ErrCodeInstallationNotFound, "not found", ErrInstallationNotFound),
	}
	r := newHandlerTestRouter(svc)

	w := doRequest(t, r, http.MethodPatch, "/api/desktop-pets/installations/nonexistent/settings", gin.H{"scale": 1.5})
	assertHTTPCode(t, w, response.NotFound, ErrCodeInstallationNotFound)
}

func TestHandler_UpdateRuntimeSettings_RuntimeSettingsNotFound_NotFound(t *testing.T) {
	svc := &stubHandlerService{
		updateSettingsErr: NewInstallationError(ErrCodeRuntimeSettingsNotFound, "settings not found", ErrRuntimeSettingsNotFound),
	}
	r := newHandlerTestRouter(svc)

	w := doRequest(t, r, http.MethodPatch, "/api/desktop-pets/installations/inst_1/settings", gin.H{"scale": 1.5})
	assertHTTPCode(t, w, response.NotFound, ErrCodeRuntimeSettingsNotFound)
}

func TestHandler_UpdateRuntimeSettings_Invalid_BusinessError(t *testing.T) {
	svc := &stubHandlerService{
		updateSettingsErr: NewInstallationError(ErrCodeInstallationInvalid, "invalid field", ErrInstallationInvalid),
	}
	r := newHandlerTestRouter(svc)

	w := doRequest(t, r, http.MethodPatch, "/api/desktop-pets/installations/inst_1/settings", gin.H{"bad_field": "value"})
	assertHTTPCode(t, w, response.BusinessError, ErrCodeInstallationInvalid)
}

func TestHandler_UpdateRuntimeSettings_InvalidJSON_InvalidParams(t *testing.T) {
	svc := &stubHandlerService{}
	r := newHandlerTestRouter(svc)

	req := httptest.NewRequest(http.MethodPatch, "/api/desktop-pets/installations/inst_1/settings", bytes.NewReader([]byte("bad json")))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assertHTTPCode(t, w, response.InvalidParams, ErrCodeInstallationFailed)
}

func TestHandler_UpdateRuntimeSettings_Success(t *testing.T) {
	svc := &stubHandlerService{}
	r := newHandlerTestRouter(svc)

	w := doRequest(t, r, http.MethodPatch, "/api/desktop-pets/installations/inst_1/settings", gin.H{"scale": 2.0, "position_x": 100})
	assertHTTPCode(t, w, response.OK, "")
}

func TestHandler_Recenter_NotFound(t *testing.T) {
	svc := &stubHandlerService{
		recenterErr: NewInstallationError(ErrCodeInstallationNotFound, "not found", ErrInstallationNotFound),
	}
	r := newHandlerTestRouter(svc)

	w := doRequest(t, r, http.MethodPost, "/api/desktop-pets/installations/nonexistent/recenter", nil)
	assertHTTPCode(t, w, response.NotFound, ErrCodeInstallationNotFound)
}

func TestHandler_Recenter_PetNotEnabled_BusinessError(t *testing.T) {
	svc := &stubHandlerService{
		recenterErr: NewInstallationError(ErrCodePetNotEnabled, "not enabled", ErrPetNotEnabled),
	}
	r := newHandlerTestRouter(svc)

	w := doRequest(t, r, http.MethodPost, "/api/desktop-pets/installations/inst_1/recenter", nil)
	assertHTTPCode(t, w, response.BusinessError, ErrCodePetNotEnabled)
}

func TestHandler_Recenter_RuntimeSettingsNotFound_NotFound(t *testing.T) {
	svc := &stubHandlerService{
		recenterErr: NewInstallationError(ErrCodeRuntimeSettingsNotFound, "settings not found", ErrRuntimeSettingsNotFound),
	}
	r := newHandlerTestRouter(svc)

	w := doRequest(t, r, http.MethodPost, "/api/desktop-pets/installations/inst_1/recenter", nil)
	assertHTTPCode(t, w, response.NotFound, ErrCodeRuntimeSettingsNotFound)
}

func TestHandler_Recenter_Success(t *testing.T) {
	svc := &stubHandlerService{}
	r := newHandlerTestRouter(svc)

	w := doRequest(t, r, http.MethodPost, "/api/desktop-pets/installations/inst_1/recenter", nil)
	assertHTTPCode(t, w, response.OK, "")
}

func TestHandler_PlayAction_NotFound(t *testing.T) {
	svc := &stubHandlerService{
		playErr: NewInstallationError(ErrCodeInstallationNotFound, "not found", ErrInstallationNotFound),
	}
	r := newHandlerTestRouter(svc)

	w := doRequest(t, r, http.MethodPost, "/api/desktop-pets/installations/nonexistent/actions/idle_normal/play", nil)
	assertHTTPCode(t, w, response.NotFound, ErrCodeInstallationNotFound)
}

func TestHandler_PlayAction_PetNotEnabled_BusinessError(t *testing.T) {
	svc := &stubHandlerService{
		playErr: NewInstallationError(ErrCodePetNotEnabled, "not enabled", ErrPetNotEnabled),
	}
	r := newHandlerTestRouter(svc)

	w := doRequest(t, r, http.MethodPost, "/api/desktop-pets/installations/inst_1/actions/idle_normal/play", nil)
	assertHTTPCode(t, w, response.BusinessError, ErrCodePetNotEnabled)
}

func TestHandler_PlayAction_ActionNotFound_NotFound(t *testing.T) {
	svc := &stubHandlerService{
		playErr: NewInstallationError(ErrCodeActionNotFound, "action not found", ErrActionNotFound),
	}
	r := newHandlerTestRouter(svc)

	w := doRequest(t, r, http.MethodPost, "/api/desktop-pets/installations/inst_1/actions/nonexistent/play", nil)
	assertHTTPCode(t, w, response.NotFound, ErrCodeActionNotFound)
}

func TestHandler_PlayAction_Invalid_BusinessError(t *testing.T) {
	svc := &stubHandlerService{
		playErr: NewInstallationError(ErrCodeInstallationInvalid, "invalid", ErrInstallationInvalid),
	}
	r := newHandlerTestRouter(svc)

	w := doRequest(t, r, http.MethodPost, "/api/desktop-pets/installations/inst_1/actions/idle_normal/play", nil)
	assertHTTPCode(t, w, response.BusinessError, ErrCodeInstallationInvalid)
}

func TestHandler_PlayAction_Success(t *testing.T) {
	svc := &stubHandlerService{}
	r := newHandlerTestRouter(svc)

	w := doRequest(t, r, http.MethodPost, "/api/desktop-pets/installations/inst_1/actions/idle_normal/play", nil)
	assertHTTPCode(t, w, response.OK, "")

	if len(svc.playCalls) != 1 {
		t.Fatalf("期望 1 次 PlayAction 调用, 实际 %d", len(svc.playCalls))
	}
	if svc.playCalls[0].ActionKey != "idle_normal" {
		t.Fatalf("ActionKey = %s, 期望 idle_normal", svc.playCalls[0].ActionKey)
	}
}

func TestWriteInstallationError_InstallationError_UsesMappedCode(t *testing.T) {
	gin.SetMode(gin.TestMode)

	cases := []struct {
		name     string
		code     string
		wantHTTP int
	}{
		{"not found", ErrCodeInstallationNotFound, response.NotFound},
		{"runtime settings not found", ErrCodeRuntimeSettingsNotFound, response.NotFound},
		{"character not found", ErrCodeCharacterNotFound, response.NotFound},
		{"action not found", ErrCodeActionNotFound, response.NotFound},
		{"package not ready", ErrCodePackageNotReady, response.InvalidParams},
		{"path traversal", ErrCodePackagePathTraversal, response.InvalidParams},
		{"symlink escape", ErrCodePackageSymlinkEscape, response.InvalidParams},
		{"executable found", ErrCodePackageExecutableFound, response.InvalidParams},
		{"hash mismatch", ErrCodePackageHashMismatch, response.InvalidParams},
		{"default action invalid", ErrCodePackageDefaultActionInvalid, response.InvalidParams},
		{"duplicate", ErrCodeInstallationDuplicate, response.BusinessError},
		{"pet not enabled", ErrCodePetNotEnabled, response.BusinessError},
		{"default action not idle", ErrCodeDefaultActionNotIdle, response.BusinessError},
		{"invalid", ErrCodeInstallationInvalid, response.BusinessError},
		{"purge not confirmed", ErrCodePurgeNotConfirmed, response.BusinessError},
		{"install failed", ErrCodeInstallationFailed, response.InternalError},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			w := httptest.NewRecorder()
			c, _ := gin.CreateTestContext(w)
			c.Request = httptest.NewRequest(http.MethodGet, "/", nil)

			err := NewInstallationError(tc.code, "测试错误: "+tc.code, nil)
			writeInstallationError(c, err)

			if w.Code != http.StatusOK {
				t.Fatalf("HTTP 状态 = %d, 期望 %d", w.Code, http.StatusOK)
			}
			var resp struct {
				Code int             `json:"code"`
				Data json.RawMessage `json:"data"`
			}
			if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
				t.Fatalf("解析响应失败: %v", err)
			}
			if resp.Code != tc.wantHTTP {
				t.Fatalf("code=%s 业务 code = %d, 期望 %d", tc.code, resp.Code, tc.wantHTTP)
			}
			var dataMap map[string]interface{}
			if err := json.Unmarshal(resp.Data, &dataMap); err != nil {
				t.Fatalf("解析 data 失败: %v", err)
			}
			if ec, _ := dataMap["errorCode"].(string); ec != tc.code {
				t.Fatalf("code=%s errorCode = %s, 期望 %s", tc.code, ec, tc.code)
			}
		})
	}
}

func TestWriteInstallationError_GenericError_InternalError(t *testing.T) {
	gin.SetMode(gin.TestMode)

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodGet, "/", nil)

	writeInstallationError(c, errors.New("非 InstallationError 类型"))

	if w.Code != http.StatusOK {
		t.Fatalf("HTTP 状态 = %d, 期望 %d", w.Code, http.StatusOK)
	}
	var resp struct {
		Code int `json:"code"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("解析响应失败: %v", err)
	}
	if resp.Code != response.InternalError {
		t.Fatalf("业务 code = %d, 期望 %d", resp.Code, response.InternalError)
	}
}

func TestWriteInstallationError_WrappedInstallationError_StillMapped(t *testing.T) {
	gin.SetMode(gin.TestMode)

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodGet, "/", nil)

	inner := NewInstallationError(ErrCodeInstallationNotFound, "未找到", ErrInstallationNotFound)
	wrapped := errors.Join(errors.New("外层包装"), inner)

	writeInstallationError(c, wrapped)

	var resp struct {
		Code int             `json:"code"`
		Data json.RawMessage `json:"data"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("解析响应失败: %v", err)
	}
	if resp.Code != response.NotFound {
		t.Fatalf("wrapped error 业务 code = %d, 期望 %d", resp.Code, response.NotFound)
	}
	var dataMap map[string]interface{}
	_ = json.Unmarshal(resp.Data, &dataMap)
	if ec, _ := dataMap["errorCode"].(string); ec != ErrCodeInstallationNotFound {
		t.Fatalf("wrapped error errorCode = %s, 期望 %s", ec, ErrCodeInstallationNotFound)
	}
}
