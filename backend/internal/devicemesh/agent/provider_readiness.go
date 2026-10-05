package agent

import (
	"context"
	"crypto/tls"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/u-ai/backend/internal/devicemesh/coordination"
)

func (h *LocalHandler) checkProviderBusiness(ctx context.Context, credential *StoredCredential, configuration *tls.Config) error {
	transport := http.DefaultTransport.(*http.Transport).Clone()
	transport.TLSClientConfig = configuration
	defer transport.CloseIdleConnections()
	client := &http.Client{Transport: transport, Timeout: 15 * time.Second, CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}
	read := func(path string, target any) error {
		request, err := http.NewRequestWithContext(ctx, http.MethodGet, strings.TrimRight(credential.CloudBaseUrl, "/")+path, nil)
		if err != nil {
			return err
		}
		request.Header.Set("Authorization", "AmitiaDevice "+credential.Credential)
		if err := h.identity.SignRequest(request, credential.SpaceID.String()); err != nil {
			return err
		}
		response, err := client.Do(request)
		if err != nil {
			return err
		}
		defer response.Body.Close()
		if response.StatusCode != http.StatusOK {
			return errors.New("新 Core 的业务接口尚未就绪")
		}
		encoded, err := io.ReadAll(io.LimitReader(response.Body, (256<<10)+1))
		if err != nil || len(encoded) > 256<<10 || json.Unmarshal(encoded, target) != nil {
			return errors.New("新 Core 的业务状态响应无效")
		}
		return nil
	}
	type providerPolicy struct {
		CoreID    string              `json:"coreId"`
		Available bool                `json:"coordinationAvailable"`
		Provider  string              `json:"aiProvider"`
		Policy    coordination.Policy `json:"policy"`
	}
	var initial providerPolicy
	if err := read("/api/device-mesh/v1/coordination/me", &initial); err != nil {
		return err
	}
	if initial.CoreID != credential.SpaceID.String() || !initial.Available || initial.Provider != "core" || initial.Policy.ProviderEpoch < 1 || initial.Policy.ModeRevision < 1 || initial.Policy.PermissionRevision < 1 {
		return errors.New("新 Core 尚未提供完整的设备业务服务，切换保持暂停")
	}
	var roles struct {
		Owner string `json:"roleOwnerId"`
		Epoch int64  `json:"providerEpoch"`
		Mode  int64  `json:"modeRevision"`
		Roles []struct {
			ID       string `json:"id"`
			Revision int64  `json:"revision"`
		} `json:"roles"`
	}
	if err := read("/api/device-mesh/v1/business/roles", &roles); err != nil {
		return err
	}
	owner := credential.DeviceID.String()
	if initial.Policy.Coordinated {
		owner = credential.SpaceID.String()
	}
	if roles.Owner != owner || roles.Epoch != initial.Policy.ProviderEpoch || roles.Mode != initial.Policy.ModeRevision || len(roles.Roles) > 1024 {
		return errors.New("新 Core 的角色来源或数据归属无效")
	}
	seen := map[string]bool{}
	selectedValid := initial.Policy.SelectedRole == ""
	for _, role := range roles.Roles {
		if role.ID == "" || role.Revision < 0 || seen[role.ID] {
			return errors.New("新 Core 返回了无效的调用角色")
		}
		seen[role.ID] = true
		selectedValid = selectedValid || role.ID == initial.Policy.SelectedRole
	}
	if !selectedValid {
		return errors.New("新 Core 所选角色已失效，请在 Core 修正后重试切换")
	}
	var final providerPolicy
	if err := read("/api/device-mesh/v1/coordination/me", &final); err != nil {
		return err
	}
	if initial != final {
		return errors.New("新 Core 的服务配置在切换过程中变化，请重试")
	}
	return nil
}
