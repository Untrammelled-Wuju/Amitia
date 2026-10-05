package agent

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strings"

	"github.com/u-ai/backend/internal/devicemesh/coordination"
)

func (c *BootstrapClient) ProviderPath(ctx context.Context, endpoint, expectedCore, localCore string) ([]string, error) {
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, strings.TrimRight(endpoint, "/")+"/api/public/device-mesh/v1/pairing/status", nil)
	if err != nil {
		return nil, err
	}
	response, err := c.httpClient.Do(request)
	if err != nil {
		return nil, err
	}
	defer response.Body.Close()
	var status struct {
		SpaceID string   `json:"spaceId"`
		Path    []string `json:"providerPath"`
	}
	if response.StatusCode != http.StatusOK || json.NewDecoder(io.LimitReader(response.Body, 32<<10)).Decode(&status) != nil || status.SpaceID == "" || expectedCore != "" && status.SpaceID != expectedCore {
		return nil, errors.New("服务提供者身份或拓扑无效")
	}
	if len(status.Path) == 0 {
		status.Path = []string{status.SpaceID}
	}
	if status.Path[0] != status.SpaceID {
		return nil, errors.New("服务提供者拓扑与配对身份不一致")
	}
	if err := coordination.ValidateProviderPath(localCore, status.Path); err != nil {
		return nil, err
	}
	return status.Path, nil
}
