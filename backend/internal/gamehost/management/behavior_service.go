package management

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/u-ai/backend/internal/devicemesh/capabilities/behaviorcatalog"
	"github.com/u-ai/backend/internal/gamehost/domain"
)

var (
	ErrBehaviorNotFound = errors.New("game behavior not found")
	ErrBehaviorRevision = errors.New("game behavior catalog revision changed")
)

type GameBehaviorService struct {
	plugins    PluginRegistryReader
	runtimes   RuntimeManagerReader
	rpc        RPCInvoker
	authorizer BehaviorInvocationAuthorizer
}

type BehaviorInvocationAuthorizer func(ctx context.Context, spaceID, targetDeviceID, pluginID, behaviorID string) error

type GameBehaviorList struct {
	PluginID        string                     `json:"pluginId"`
	ExtensionID     string                     `json:"extensionId"`
	CatalogRevision string                     `json:"catalogRevision"`
	Behaviors       []behaviorcatalog.Behavior `json:"behaviors"`
}

type GameBehaviorInvokeResult struct {
	PluginID        string          `json:"pluginId"`
	BehaviorID      string          `json:"behaviorId"`
	CatalogRevision string          `json:"catalogRevision"`
	RuntimeID       string          `json:"runtimeId"`
	ServiceID       string          `json:"serviceId"`
	Method          string          `json:"method"`
	Result          json.RawMessage `json:"result"`
}

func NewGameBehaviorService(plugins PluginRegistryReader, runtimes RuntimeManagerReader, rpc RPCInvoker, authorizers ...BehaviorInvocationAuthorizer) *GameBehaviorService {
	service := &GameBehaviorService{plugins: plugins, runtimes: runtimes, rpc: rpc}
	if len(authorizers) > 0 {
		service.authorizer = authorizers[0]
	}
	return service
}

func (s *GameBehaviorService) Authorize(ctx context.Context, spaceID, targetDeviceID, pluginID, behaviorID string) error {
	if s == nil || s.authorizer == nil {
		return nil
	}
	return s.authorizer(ctx, spaceID, targetDeviceID, pluginID, behaviorID)
}

func (s *GameBehaviorService) List(ctx context.Context, pluginID string) (GameBehaviorList, error) {
	plugin, catalog, err := s.catalog(ctx, pluginID)
	if err != nil {
		return GameBehaviorList{}, err
	}
	return GameBehaviorList{PluginID: string(plugin.ID), ExtensionID: plugin.ExtensionID, CatalogRevision: catalog.Revision, Behaviors: catalog.Behaviors}, nil
}

func (s *GameBehaviorService) Invoke(ctx context.Context, pluginID, behaviorID, runtimeID, expectedRevision string, arguments json.RawMessage) (GameBehaviorInvokeResult, error) {
	plugin, catalog, err := s.catalog(ctx, pluginID)
	if err != nil {
		return GameBehaviorInvokeResult{}, err
	}
	if strings.TrimSpace(expectedRevision) != catalog.Revision {
		return GameBehaviorInvokeResult{}, fmt.Errorf("%w: expected=%s current=%s", ErrBehaviorRevision, expectedRevision, catalog.Revision)
	}
	behavior, ok := catalog.Find(behaviorID)
	if !ok {
		return GameBehaviorInvokeResult{}, fmt.Errorf("%w: %s", ErrBehaviorNotFound, behaviorID)
	}
	if behavior.Available != nil && !*behavior.Available {
		return GameBehaviorInvokeResult{}, fmt.Errorf("%w: %s is unavailable", ErrBehaviorNotFound, behavior.ID)
	}
	if behavior.ServiceID == "" || behavior.Method == "" {
		return GameBehaviorInvokeResult{}, fmt.Errorf("%w: behavior %s has no executable service method", ErrBehaviorNotFound, behavior.ID)
	}
	if len(arguments) == 0 {
		arguments = json.RawMessage(`{}`)
	}
	runtimeID = strings.TrimSpace(runtimeID)
	if runtimeID == "" {
		runtimeID, err = s.singleRuntime(plugin.ID)
		if err != nil {
			return GameBehaviorInvokeResult{}, err
		}
	}
	response, err := s.rpc.SendCustomRPC(ctx, runtimeID, behavior.ServiceID, behavior.Method, arguments, 30*time.Second)
	if err != nil {
		return GameBehaviorInvokeResult{}, fmt.Errorf("invoke game behavior %s: %w", behavior.ID, err)
	}
	if response == nil {
		return GameBehaviorInvokeResult{}, fmt.Errorf("invoke game behavior %s returned an empty response", behavior.ID)
	}
	if response.Error != nil {
		return GameBehaviorInvokeResult{}, fmt.Errorf("invoke game behavior %s: %s: %s", behavior.ID, response.Error.Code, response.Error.Message)
	}
	return GameBehaviorInvokeResult{
		PluginID: string(plugin.ID), BehaviorID: behavior.ID, CatalogRevision: catalog.Revision,
		RuntimeID: runtimeID, ServiceID: behavior.ServiceID, Method: behavior.Method,
		Result: append(json.RawMessage(nil), response.Payload...),
	}, nil
}

func (s *GameBehaviorService) catalog(ctx context.Context, pluginID string) (domain.PluginDescriptor, behaviorcatalog.Catalog, error) {
	if s == nil || s.plugins == nil {
		return domain.PluginDescriptor{}, behaviorcatalog.Catalog{}, errors.New("game behavior plugin registry is unavailable")
	}
	plugin, err := s.plugins.Get(ctx, strings.TrimSpace(pluginID))
	if err != nil {
		return domain.PluginDescriptor{}, behaviorcatalog.Catalog{}, fmt.Errorf("read game plugin: %w", err)
	}
	catalog, err := behaviorcatalog.Parse(plugin.Metadata)
	if errors.Is(err, behaviorcatalog.ErrMissing) {
		return plugin, behaviorcatalog.Catalog{Version: 1, Revision: "empty", Behaviors: []behaviorcatalog.Behavior{}}, nil
	}
	if err != nil {
		return domain.PluginDescriptor{}, behaviorcatalog.Catalog{}, err
	}
	return plugin, catalog, nil
}

func (s *GameBehaviorService) singleRuntime(pluginID domain.PluginID) (string, error) {
	if s.runtimes == nil {
		return "", errors.New("game runtime manager is unavailable")
	}
	matches := make([]string, 0, 2)
	for _, runtime := range s.runtimes.ListRuntimes() {
		if runtime != nil && runtime.PluginID == pluginID {
			matches = append(matches, string(runtime.ID))
		}
	}
	if len(matches) == 1 {
		return matches[0], nil
	}
	if len(matches) == 0 {
		return "", fmt.Errorf("game plugin %s has no runtime", pluginID)
	}
	return "", fmt.Errorf("game plugin %s has multiple runtimes; runtimeId is required", pluginID)
}
