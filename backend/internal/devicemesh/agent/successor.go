package agent

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/u-ai/backend/internal/devicemesh/coordination"
	"github.com/u-ai/backend/internal/devicemesh/lan"
	"github.com/u-ai/backend/internal/devicemesh/proof"
	"github.com/u-ai/backend/internal/secretstore"
)

type Successor struct {
	Endpoint     lan.Endpoint `json:"endpoint"`
	ProviderPath []string     `json:"providerPath"`
	OfferToken   string       `json:"offerToken"`
	ExpiresAt    time.Time    `json:"expiresAt"`
	DeviceID     string       `json:"deviceId"`
	Coordinated  bool         `json:"coordinated"`
}

func (h *LocalHandler) providerRequest(ctx context.Context, credential *StoredCredential, method, path string, input any) (int, []byte, error) {
	if credential == nil || credential.Fingerprint == "" || !time.Now().Before(credential.ExpiresAt) {
		return 0, nil, errors.New("服务切换要求有效的局域网绑定身份")
	}
	client, err := NewPinnedBootstrapClient(lan.Endpoint{URL: credential.CloudBaseUrl, CoreID: credential.SpaceID.String(), Fingerprint: credential.Fingerprint})
	if err != nil {
		return 0, nil, err
	}
	var encoded []byte
	if input != nil {
		encoded, err = json.Marshal(input)
		if err != nil {
			return 0, nil, err
		}
	}
	request, err := http.NewRequestWithContext(ctx, method, strings.TrimRight(credential.CloudBaseUrl, "/")+path, bytes.NewReader(encoded))
	if err != nil {
		return 0, nil, err
	}
	request.Header.Set("Authorization", "AmitiaDevice "+credential.Credential)
	request.Header.Set("Content-Type", "application/json")
	if err := h.identity.SignRequest(request, credential.SpaceID.String()); err != nil {
		return 0, nil, err
	}
	response, err := client.httpClient.Do(request)
	if err != nil {
		return 0, nil, err
	}
	defer response.Body.Close()
	if response.StatusCode == http.StatusNoContent {
		return response.StatusCode, nil, nil
	}
	body, err := io.ReadAll(io.LimitReader(response.Body, (64<<10)+1))
	if err != nil || len(body) > 64<<10 || !json.Valid(body) {
		return 0, nil, errors.New("服务切换响应无效")
	}
	return response.StatusCode, body, nil
}

func (h *LocalHandler) PrepareSuccessor(ctx context.Context, target string, coordinated bool) (*Successor, error) {
	h.successorMu.Lock()
	defer h.successorMu.Unlock()
	if target == "" || len(target) > 512 {
		return nil, errors.New("设备身份无效")
	}
	credential, err := h.credStore.LoadCredential()
	if err != nil || credential == nil {
		return nil, err
	}
	digest := sha256.Sum256([]byte(target))
	path := filepath.Join(h.dataDir, "device-mesh", "successors", hex.EncodeToString(digest[:])+".json")
	if encoded, err := secretstore.Read(path); err == nil {
		var saved Successor
		if json.Unmarshal(encoded, &saved) == nil && saved.Coordinated == coordinated && saved.DeviceID == target && saved.Endpoint.CoreID == credential.SpaceID.String() && saved.Endpoint.Fingerprint == credential.Fingerprint && saved.Endpoint.URL == credential.CloudBaseUrl && time.Now().Add(15*time.Second).Before(saved.ExpiresAt) {
			return &saved, nil
		}
	} else if !os.IsNotExist(err) {
		return nil, err
	}
	status, body, err := h.providerRequest(ctx, credential, http.MethodPost, "/api/device-mesh/v1/pairing/successor-offers", map[string]any{"deviceId": target, "coordinated": coordinated})
	if err != nil {
		return nil, err
	}
	if status != http.StatusOK {
		return nil, errors.New("新服务提供者暂不能受理下级设备配对")
	}
	var response struct {
		OfferToken       string    `json:"offerToken"`
		ExpiresAt        time.Time `json:"expiresAt"`
		DeviceID         string    `json:"deviceId"`
		ApprovalRequired bool      `json:"approvalRequired"`
	}
	if json.Unmarshal(body, &response) != nil || response.OfferToken == "" || response.DeviceID != target || !response.ApprovalRequired || !time.Now().Before(response.ExpiresAt) {
		return nil, errors.New("新服务提供者未返回有效的独立审批入口")
	}
	providerPath := credential.ProviderPath
	if len(providerPath) == 0 {
		providerPath = []string{credential.SpaceID.String()}
	}
	result := &Successor{Endpoint: lan.Endpoint{URL: credential.CloudBaseUrl, CoreID: credential.SpaceID.String(), Fingerprint: credential.Fingerprint}, ProviderPath: providerPath, OfferToken: response.OfferToken, ExpiresAt: response.ExpiresAt, DeviceID: target}
	result.Coordinated = coordinated
	encoded, err := json.Marshal(result)
	if err != nil {
		return nil, err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		return nil, err
	}
	if err := secretstore.Write(path, encoded); err != nil {
		return nil, err
	}
	return result, nil
}

func (h *LocalHandler) FollowSuccessor(ctx context.Context) error {
	if resumed, err := h.ResumeProviderBinding(ctx); resumed || err != nil {
		return err
	}
	h.mu.RLock()
	version := h.bindingVersion
	previous, err := h.credStore.LoadCredential()
	h.mu.RUnlock()
	if err != nil || previous == nil {
		return err
	}
	var successor Successor
	pending, err := h.credStore.LoadTransition()
	if err != nil {
		return err
	}
	if pending != nil && (pending.PreviousCoreID != previous.SpaceID.String() || pending.PreviousCredentialID != previous.CredentialID || pending.DeviceID != previous.DeviceID.String() || pending.RuntimeID != previous.RuntimeID.String()) {
		return errors.New("服务切换记录与当前绑定不一致，请撤销配对后重新连接")
	}
	if pending != nil && time.Now().Before(pending.Successor.ExpiresAt) {
		successor = pending.Successor
	} else {
		status, encoded, err := h.providerRequest(ctx, previous, http.MethodGet, "/api/device-mesh/v1/provider/successor", nil)
		if err != nil {
			return err
		}
		if status == http.StatusNoContent {
			return nil
		}
		if status != http.StatusOK {
			return errors.New("原服务提供者暂未提供有效的切换入口")
		}
		if json.Unmarshal(encoded, &successor) != nil {
			return errors.New("服务切换入口格式无效")
		}
	}
	if successor.DeviceID != previous.DeviceID.String() || successor.Endpoint.CoreID == previous.SpaceID.String() || successor.OfferToken == "" || !time.Now().Before(successor.ExpiresAt) {
		return errors.New("服务切换入口身份无效")
	}
	h.mu.RLock()
	localCore := h.localCoreID
	h.mu.RUnlock()
	if err := coordination.ValidateProviderPath(localCore, successor.ProviderPath); err != nil {
		return err
	}
	client, err := NewPinnedBootstrapClient(successor.Endpoint)
	if err != nil {
		return err
	}
	identity, err := h.identity.Load()
	if err != nil {
		return err
	}
	if _, err := client.ProviderPath(ctx, successor.Endpoint.URL, successor.Endpoint.CoreID, localCore); err != nil {
		return err
	}
	h.mu.Lock()
	if version != h.bindingVersion {
		h.mu.Unlock()
		return errors.New("设备绑定已变化，旧服务切换请求已拦截")
	}
	h.pendingCore = successor.Endpoint.CoreID
	h.pauseProvider()
	if err := h.credStore.SaveTransition(PendingProviderTransition{PreviousCoreID: previous.SpaceID.String(), PreviousCredentialID: previous.CredentialID, DeviceID: previous.DeviceID.String(), RuntimeID: previous.RuntimeID.String(), Successor: successor}); err != nil {
		h.mu.Unlock()
		return err
	}
	h.mu.Unlock()
	claim := proof.ClaimBody{DeviceID: identity.DeviceID.String(), RuntimeID: identity.RuntimeID.String(), Platform: h.platform.String(), Label: "来自 " + previous.SpaceID.String() + " 的设备", OfferToken: successor.OfferToken}
	signed := proof.New(identity.PublicKey, successor.Endpoint.CoreID, uuid.NewString(), claim, time.Now())
	signed.Signature, err = h.identity.Sign(signed.SigningBytes())
	if err != nil {
		return err
	}
	payload, err := json.Marshal(struct {
		proof.ClaimBody
		Proof *proof.Proof `json:"proof"`
	}{claim, &signed})
	if err != nil {
		return err
	}
	request, err := http.NewRequestWithContext(ctx, http.MethodPost, strings.TrimRight(successor.Endpoint.URL, "/")+"/api/public/device-mesh/v1/pairing/claim", bytes.NewReader(payload))
	if err != nil {
		return err
	}
	request.Header.Set("Content-Type", "application/json")
	response, err := client.httpClient.Do(request)
	if err != nil {
		return err
	}
	defer response.Body.Close()
	if response.StatusCode == http.StatusAccepted {
		return errors.New("等待新服务提供者批准，当前云端对话已暂停")
	}
	var accepted struct {
		Ticket   string `json:"ticket"`
		SpaceID  string `json:"spaceId"`
		DeviceID string `json:"deviceId"`
	}
	if response.StatusCode != http.StatusOK || json.NewDecoder(io.LimitReader(response.Body, 32<<10)).Decode(&accepted) != nil || accepted.Ticket == "" || accepted.SpaceID != successor.Endpoint.CoreID || accepted.DeviceID != identity.DeviceID.String() {
		return errors.New("新服务提供者拒绝或尚未完成独立配对")
	}
	_, err = h.BindProvider(ctx, BindingRequest{CloudBaseURL: successor.Endpoint.URL, BootstrapTicket: accepted.Ticket, Fingerprint: successor.Endpoint.Fingerprint, CoreID: successor.Endpoint.CoreID, expectedBindingVersion: &version})
	return err
}
