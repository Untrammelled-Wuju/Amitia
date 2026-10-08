package main

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/u-ai/backend/internal/character"
	"github.com/u-ai/backend/internal/devicemesh/business"
	"github.com/u-ai/backend/internal/devicemesh/coordination"
)

type threeCoreSpeechModel struct {
	*threeCoreMemoryModel
	started   chan struct{}
	cancelled chan struct{}
	roles     chan business.SpeechInference
}

func (m *threeCoreSpeechModel) GenerateOwnedSpeech(ctx context.Context, input business.SpeechInference) ([]byte, error) {
	m.roles <- input
	if input.Text == "等待语音服务切换" {
		close(m.started)
		<-ctx.Done()
		close(m.cancelled)
		return []byte("ID3应丢弃迟到音频"), nil
	}
	return []byte("ID3语音来自" + m.core), nil
}

func threeCoreSpeechScope(t *testing.T, provider, source *threeCoreFixture) coordination.ExecutionScope {
	t.Helper()
	return decodeThreeCoreMemoryResponse[business.QueryResult](t, provider.request(t, source, http.MethodGet, "/api/device-mesh/v1/business/data?kind=memory&characterId=one", nil, http.StatusOK)).Scope
}

func TestThreeRealCoreSpeechUsesCurrentRoleAndOwnerAndCancelsOnProviderSwitch(t *testing.T) {
	schemas := newThreeCoreSchemas(t)
	a, b, c := newThreeCoreFixture(t, "speech-a", schemas), newThreeCoreFixture(t, "speech-b", schemas), newThreeCoreFixture(t, "speech-c", schemas)
	models := make(map[*threeCoreFixture]*threeCoreSpeechModel)
	for _, fixture := range []*threeCoreFixture{a, b, c} {
		if err := fixture.services.DB.Model(&character.Character{}).Where("id=?", "one").Updates(map[string]any{"voice_type": fixture.core + "-preset", "revision": 2}).Error; err != nil {
			t.Fatal(err)
		}
		model := &threeCoreSpeechModel{threeCoreMemoryModel: &threeCoreMemoryModel{core: fixture.core}, started: make(chan struct{}), cancelled: make(chan struct{}), roles: make(chan business.SpeechInference, 8)}
		models[fixture] = model
		fixture.services.OwnedBusiness = business.NewEngine(fixture.services.DeviceMesh.Coordination, fixture.services.DeviceMesh, model)
	}
	pairThreeCoreFixtures(t, a, b)
	deviceScope := threeCoreSpeechScope(t, b, a)
	deviceResult := decodeThreeCoreMemoryResponse[business.SpeechResponse](t, b.request(t, a, http.MethodPost, "/api/device-mesh/v1/business/speech", map[string]any{"requestId": "device-speech", "characterId": "one", "text": "设备角色朗读", "expectedExecutionScope": deviceScope}, http.StatusOK))
	if !deviceResult.Saved || deviceResult.Scope.ResourceOwnerID != a.device.DeviceID.String() || deviceResult.Acknowledgement.OwnerID != a.device.DeviceID.String() {
		t.Fatal("OFF语音没有经Source确认保存")
	}
	input := <-models[b].roles
	var deviceProfile struct {
		Voice struct {
			VoiceType string `json:"voiceType"`
		} `json:"voice"`
	}
	if json.Unmarshal(input.Role.Profile, &deviceProfile) != nil || input.Role.Name != a.core+" 的角色" || deviceProfile.Voice.VoiceType != a.core+"-preset" {
		t.Fatal("OFF语音未携带Source独立角色和声学快照")
	}
	assertThreeCorePrivateCopies(t, b, a.device.DeviceID.String())
	policy, err := b.services.DeviceMesh.Coordination.Get(t.Context(), b.core, a.device.DeviceID.String())
	if err != nil {
		t.Fatal(err)
	}
	b.request(t, a, http.MethodPut, "/api/device-mesh/v1/coordination/me", map[string]any{"coordinated": true, "expectedRevision": policy.ModeRevision, "selectedRole": "one"}, http.StatusOK)
	coreScope := threeCoreSpeechScope(t, b, a)
	coreResult := decodeThreeCoreMemoryResponse[business.SpeechResponse](t, b.request(t, a, http.MethodPost, "/api/device-mesh/v1/business/speech", map[string]any{"requestId": "core-speech", "characterId": "one", "text": "云端角色朗读", "expectedExecutionScope": coreScope}, http.StatusOK))
	if !coreResult.Saved || coreResult.Scope.ResourceOwnerID != b.core || coreResult.Acknowledgement.OwnerID != b.core {
		t.Fatal("ON语音没有保存在当前Core")
	}
	input = <-models[b].roles
	if json.Unmarshal(input.Role.Profile, &deviceProfile) != nil || input.Role.Name != b.core+" 的角色" || deviceProfile.Voice.VoiceType != b.core+"-preset" {
		t.Fatal("ON语音使用了设备旧声学快照")
	}
	done := make(chan struct{})
	recorder := httptest.NewRecorder()
	payload, _ := json.Marshal(map[string]any{"requestId": "cancelled-speech", "characterId": "one", "text": "等待语音服务切换", "expectedExecutionScope": coreScope})
	go func() {
		defer close(done)
		request := httptest.NewRequest(http.MethodPost, "/internal/device-mesh/provider/api/device-mesh/v1/business/speech", bytes.NewReader(payload))
		request.RemoteAddr = "127.0.0.1:40000"
		request.Header.Set("Content-Type", "application/json")
		a.router.ServeHTTP(recorder, request)
	}()
	select {
	case <-models[b].started:
	case <-time.After(10 * time.Second):
		t.Fatal("旧Core语音未开始")
	}
	pairThreeCoreFixtures(t, b, c)
	select {
	case <-models[b].cancelled:
	case <-time.After(5 * time.Second):
		t.Fatal("Core切换没有取消进行中的语音")
	}
	select {
	case <-done:
	case <-time.After(5 * time.Second):
		t.Fatal("旧语音请求没有结束")
	}
	if recorder.Code == http.StatusOK {
		t.Fatal("迟到语音被返回给设备播放")
	}
	if err := a.local.FollowSuccessor(t.Context()); err == nil {
		t.Fatal("A未经新Core独立审批完成切换")
	}
	approvals, err := c.pairing.PendingApprovals(t.Context())
	if err != nil || len(approvals) != 1 {
		t.Fatal("C没有独立审批A")
	}
	if err := c.pairing.DecideApproval(t.Context(), approvals[0].RequestID, approvals[0].Revision, true); err != nil {
		t.Fatal(err)
	}
	followApprovedThreeCoreSuccessor(t, a, c)
	c.request(t, a, http.MethodPost, "/api/device-mesh/v1/business/speech", map[string]any{"requestId": "stale-core-speech", "characterId": "one", "text": "旧页面音频", "expectedExecutionScope": coreScope}, http.StatusConflict)
	currentScope := threeCoreSpeechScope(t, c, a)
	current := decodeThreeCoreMemoryResponse[business.SpeechResponse](t, c.request(t, a, http.MethodPost, "/api/device-mesh/v1/business/speech", map[string]any{"requestId": "new-core-speech", "characterId": "one", "text": "新Core角色朗读", "expectedExecutionScope": currentScope}, http.StatusOK))
	if !current.Saved || current.Scope.CoreID != c.core || current.Scope.ResourceOwnerID != c.core || current.Acknowledgement.OwnerID != c.core {
		t.Fatal("新Core音频没有使用新Core归属")
	}
	input = <-models[c].roles
	if input.Role.Name != c.core+" 的角色" {
		t.Fatal("新Core语音借用了旧Core角色")
	}
	for _, fixture := range []*threeCoreFixture{a, b, c} {
		owner := fixture.core
		if fixture == a {
			owner = a.device.DeviceID.String()
		}
		var count int
		if err := fixture.services.KernelContainer.DeviceRegistry.Database().QueryRowContext(t.Context(), `SELECT COUNT(*) FROM kernel_device_owned_resources WHERE owner_id=? AND kind='tool-result' AND resource_id LIKE 'speech/%' AND deleted=0`, owner).Scan(&count); err != nil || count != 1 {
			t.Fatalf("音频所有者丢失/迁移或迟到写入: count=%d error=%v", count, err)
		}
	}
	assertThreeCorePrivateCopies(t, c, a.device.DeviceID.String())
	assertThreeCorePrivateCopies(t, c, b.core)
}
