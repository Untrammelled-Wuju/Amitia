package runtime

import (
	"context"
	"crypto/rand"
	"encoding/hex"

	"github.com/u-ai/backend/internal/gamehost/contracts"
	"github.com/u-ai/backend/internal/gamehost/domain"
)

type executionGenerationContextKey struct{}

func WithExecutionGeneration(ctx context.Context, generation int64) context.Context {
	return context.WithValue(ctx, executionGenerationContextKey{}, generation)
}

func executionGeneration(ctx context.Context) int64 {
	generation, _ := ctx.Value(executionGenerationContextKey{}).(int64)
	return generation
}

func newExecutionSessionToken() (string, error) {
	buffer := make([]byte, 32)
	if _, err := rand.Read(buffer); err != nil {
		return "", err
	}
	return hex.EncodeToString(buffer), nil
}

type RuntimePaths struct {
	Root string
	Data string
	Temp string
	Logs string
}

type ServicePaths struct {
	Data string
	Temp string
	Logs string
}

type ServiceExecutionContext struct {
	RuntimeID    domain.RuntimeInstanceID
	PluginID     domain.PluginID
	ServiceID    domain.ServiceID
	DefinitionID string

	ServiceKind domain.ServiceKind

	Required    bool
	ServiceName string

	BasePath   string
	PluginDir  string
	RuntimeDir string

	RuntimePaths RuntimePaths
	ServicePaths ServicePaths

	ConfigSnapshot map[string]string

	Env map[string]string

	Generation         int64
	SessionToken       string
	SecretLeaseSession *contracts.RuntimeSecretLeaseSession
}

func (c ServiceExecutionContext) DirectoryIdentifier() string {
	return BuildProcessInstanceID(c.RuntimeID, c.ServiceID)
}

func BuildProcessInstanceID(runtimeID domain.RuntimeInstanceID, serviceID domain.ServiceID) string {
	return string(runtimeID) + "/" + string(serviceID)
}

func ParseProcessInstanceID(id string) (domain.RuntimeInstanceID, domain.ServiceID, error) {
	for i := 0; i < len(id); i++ {
		if id[i] == '/' {
			return domain.RuntimeInstanceID(id[:i]), domain.ServiceID(id[i+1:]), nil
		}
	}
	return "", "", &TopologyError{Code: ErrInvalidArgument, Message: "invalid process instance id: " + id}
}
