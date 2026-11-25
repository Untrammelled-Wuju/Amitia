//go:build rsi_pluginadapter

package pluginadapter

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"strconv"
	"strings"

	"github.com/u-ai/backend/internal/extension/kernel/capability"
	"github.com/u-ai/backend/internal/extension/kernel/host_api"
	"github.com/u-ai/backend/internal/extension/kernel/plugincontract"
	"github.com/u-ai/backend/internal/extension/kernel/runtime_supervisor"
)

var (
	ErrPluginAdapterUnavailable = errors.New("pluginadapter: plugin adapter unavailable")
	ErrPluginNotReady           = errors.New("pluginadapter: plugin provider not ready")
	ErrPluginContractMismatch   = errors.New("pluginadapter: plugin contract hash mismatch")
	ErrPluginContractVersion    = errors.New("pluginadapter: plugin contract version below requirement")
	ErrPluginPermissionDenied   = errors.New("pluginadapter: plugin permission denied")
	ErrPluginScopeViolation     = errors.New("pluginadapter: plugin scope violation")
	ErrPluginBindingMismatch    = errors.New("pluginadapter: plugin binding provider mismatch")
)

type NotReadyError struct {
	ProviderID string
	Status     capability.HealthStatus
	Cause      error
}

func (e *NotReadyError) Error() string {
	status := ""
	if e.Status != "" {
		status = " status=" + string(e.Status)
	}
	if e.Cause != nil {
		return fmt.Sprintf("pluginadapter: provider %q is not ready%s: %v", e.ProviderID, status, e.Cause)
	}
	return fmt.Sprintf("pluginadapter: provider %q is not ready%s", e.ProviderID, status)
}

func (e *NotReadyError) Unwrap() []error { return categoryWithCause(ErrPluginNotReady, e.Cause) }

type BindingMismatchError struct {
	ProviderID string
	BindingID  string
}

func (e *BindingMismatchError) Error() string {
	return fmt.Sprintf("pluginadapter: binding provider %q does not match adapter provider %q: %v", e.BindingID, e.ProviderID, ErrPluginBindingMismatch)
}

func (e *BindingMismatchError) Unwrap() error { return ErrPluginBindingMismatch }

type ContractMismatchError struct {
	ProviderID string
	Expected   string
	Actual     string
}

func (e *ContractMismatchError) Error() string {
	return fmt.Sprintf("pluginadapter: provider %q contract hash %q does not match catalog %q: %v", e.ProviderID, e.Actual, e.Expected, ErrPluginContractMismatch)
}

func (e *ContractMismatchError) Unwrap() error { return ErrPluginContractMismatch }

type ContractVersionError struct {
	ProviderID string
	Host       string
	Required   string
}

func (e *ContractVersionError) Error() string {
	return fmt.Sprintf("pluginadapter: host contract version %q is below provider %q requirement %q: %v", e.Host, e.ProviderID, e.Required, ErrPluginContractVersion)
}

func (e *ContractVersionError) Unwrap() error { return ErrPluginContractVersion }

type PermissionDeniedError struct {
	ProviderID string
	Missing    []string
	Cause      error
}

func (e *PermissionDeniedError) Error() string {
	missing := append([]string(nil), e.Missing...)
	sort.Strings(missing)
	if e.Cause != nil {
		return fmt.Sprintf("pluginadapter: provider %q permission denied (%s): %v: %v", e.ProviderID, strings.Join(missing, ","), ErrPluginPermissionDenied, e.Cause)
	}
	return fmt.Sprintf("pluginadapter: provider %q permission denied (%s): %v", e.ProviderID, strings.Join(missing, ","), ErrPluginPermissionDenied)
}

func (e *PermissionDeniedError) Unwrap() []error { return categoryWithCause(ErrPluginPermissionDenied, e.Cause) }

type ScopeViolationError struct {
	ProviderID string
	Scope      plugincontract.ServiceScope
	Declared   []plugincontract.ServiceScope
}

func (e *ScopeViolationError) Error() string {
	declared := make([]string, 0, len(e.Declared))
	for _, scope := range e.Declared {
		declared = append(declared, string(scope))
	}
	return fmt.Sprintf("pluginadapter: provider %q scope %q is outside declared scopes %s: %v", e.ProviderID, e.Scope, strings.Join(declared, ","), ErrPluginScopeViolation)
}

func (e *ScopeViolationError) Unwrap() error { return ErrPluginScopeViolation }

func categoryWithCause(category error, cause error) []error {
	if cause == nil {
		return []error{category}
	}
	return []error{category, cause}
}

type ProviderProfile struct {
	ProviderID          string
	PluginID            string
	ServiceID           string
	ContractHash        string
	Scopes              []plugincontract.ServiceScope
	RequiredPermissions []string
	MinContractVersion  string
}

func ProfileFromBinding(binding plugincontract.ProviderBinding) ProviderProfile {
	return ProviderProfile{
		ProviderID:          strings.TrimSpace(binding.ProviderID),
		PluginID:            strings.TrimSpace(binding.PluginID),
		ServiceID:           strings.TrimSpace(binding.ServiceID),
		ContractHash:        strings.TrimSpace(binding.ContractHash),
		Scopes:              append([]plugincontract.ServiceScope(nil), binding.Scopes...),
		RequiredPermissions: append([]string(nil), binding.RequiredPermissions...),
		MinContractVersion:  strings.TrimSpace(binding.MinContractVersion),
	}
}

type Config struct {
	Profile          ProviderProfile
	ExpectedContract string
	HostVersion      string
	Permissions      host_api.PermissionChecker
	Identity         runtime_supervisor.RuntimeIdentity
	Caller           capability.PluginCallFunc
	Health           capability.PluginHealthFunc
}

type PluginAdapter struct {
	profile          ProviderProfile
	expectedContract string
	hostVersion      string
	permissions      host_api.PermissionChecker
	identity         runtime_supervisor.RuntimeIdentity
	caller           capability.PluginCallFunc
	health           capability.PluginHealthFunc
}

func NewPluginAdapter(config Config) *PluginAdapter {
	return &PluginAdapter{
		profile:          config.Profile,
		expectedContract: strings.TrimSpace(config.ExpectedContract),
		hostVersion:      strings.TrimSpace(config.HostVersion),
		permissions:      config.Permissions,
		identity:         config.Identity,
		caller:           config.Caller,
		health:           config.Health,
	}
}

func (a *PluginAdapter) Profile() ProviderProfile {
	return a.profile
}

func (a *PluginAdapter) Supports(binding capability.RuntimeBinding) bool {
	if a == nil {
		return false
	}
	switch binding.RuntimeType {
	case capability.RuntimeTypePluginJS, capability.RuntimeTypePluginService:
	default:
		return false
	}
	if a.profile.ProviderID == "" {
		return false
	}
	bound := strings.TrimSpace(binding.ProviderID)
	return bound == "" || bound == a.profile.ProviderID
}

func (a *PluginAdapter) Execute(
	ctx context.Context,
	binding capability.RuntimeBinding,
	invocation capability.ToolInvocationContext,
	input json.RawMessage,
) capability.UnifiedToolResult {
	if a == nil {
		return failedResult(invocation.InvocationID, capability.ErrorCodeProviderUnavailable, ErrPluginAdapterUnavailable, nil)
	}
	if err := a.admit(ctx, binding, invocation); err != nil {
		return failedResult(invocation.InvocationID, errorCodeFor(err), err, errorDetails(err))
	}
	output, err := a.caller(ctx, a.profile.PluginID, binding.HandlerName, input)
	if err != nil {
		return failedResult(invocation.InvocationID, capability.ErrorCodeUpstream, err, errorDetails(err))
	}
	result := capability.UnifiedToolResult{
		InvocationID: invocation.InvocationID,
		Status:       capability.ToolResultStatusSuccess,
		Content:      []capability.ToolContent{{Type: capability.ToolContentText, Text: string(output)}},
	}
	if json.Valid(output) {
		result.Structured = append(json.RawMessage(nil), output...)
	}
	return result
}

func (a *PluginAdapter) Health(ctx context.Context, binding capability.RuntimeBinding) capability.HealthStatus {
	if a == nil || a.health == nil {
		return capability.HealthUnknown
	}
	return a.health(ctx, binding.RuntimeID)
}

func (a *PluginAdapter) admit(ctx context.Context, binding capability.RuntimeBinding, invocation capability.ToolInvocationContext) error {
	if a.caller == nil {
		return &NotReadyError{ProviderID: a.profile.ProviderID, Cause: ErrPluginAdapterUnavailable}
	}
	bound := strings.TrimSpace(binding.ProviderID)
	if bound != "" && bound != a.profile.ProviderID {
		return &BindingMismatchError{ProviderID: a.profile.ProviderID, BindingID: bound}
	}
	if status := a.Health(ctx, binding); !pluginReady(status) {
		return &NotReadyError{ProviderID: a.profile.ProviderID, Status: status}
	}
	if err := a.checkContract(binding); err != nil {
		return err
	}
	if err := a.checkVersion(); err != nil {
		return err
	}
	if err := a.checkPermissions(ctx); err != nil {
		return err
	}
	return a.checkScope(invocation)
}

func pluginReady(status capability.HealthStatus) bool {
	switch status {
	case "", capability.HealthReady, capability.HealthDegraded, capability.HealthUnknown:
		return true
	}
	return false
}

func (a *PluginAdapter) checkContract(binding capability.RuntimeBinding) error {
	if a.expectedContract == "" {
		return nil
	}
	actual := metadataString(binding.Metadata, "contractHash")
	if actual == a.expectedContract {
		return nil
	}
	return &ContractMismatchError{ProviderID: a.profile.ProviderID, Expected: a.expectedContract, Actual: actual}
}

func (a *PluginAdapter) checkVersion() error {
	required := strings.TrimSpace(a.profile.MinContractVersion)
	if required == "" {
		return nil
	}
	if a.hostVersion == "" || compareDotted(a.hostVersion, required) < 0 {
		return &ContractVersionError{ProviderID: a.profile.ProviderID, Host: a.hostVersion, Required: required}
	}
	return nil
}

func (a *PluginAdapter) checkPermissions(ctx context.Context) error {
	names := make([]string, 0, len(a.profile.RequiredPermissions))
	for _, name := range a.profile.RequiredPermissions {
		name = strings.TrimSpace(name)
		if name == "" {
			continue
		}
		names = append(names, name)
	}
	if len(names) == 0 {
		return nil
	}
	sort.Strings(names)
	if a.permissions == nil {
		return &PermissionDeniedError{ProviderID: a.profile.ProviderID, Missing: names, Cause: host_api.ErrPermissionDenied}
	}
	requirements := make([]host_api.PermissionRequirement, 0, len(names))
	for _, name := range names {
		requirements = append(requirements, host_api.PermissionRequirement{Name: name})
	}
	if err := a.permissions.Check(ctx, a.identity, requirements); err != nil {
		return &PermissionDeniedError{ProviderID: a.profile.ProviderID, Missing: names, Cause: err}
	}
	return nil
}

func (a *PluginAdapter) checkScope(invocation capability.ToolInvocationContext) error {
	if len(a.profile.Scopes) == 0 {
		return nil
	}
	scope := ScopeFromInvocation(invocation)
	for _, declared := range a.profile.Scopes {
		if declared == scope {
			return nil
		}
	}
	return &ScopeViolationError{ProviderID: a.profile.ProviderID, Scope: scope, Declared: append([]plugincontract.ServiceScope(nil), a.profile.Scopes...)}
}

func ScopeFromInvocation(invocation capability.ToolInvocationContext) plugincontract.ServiceScope {
	if invocation.ExecContext != nil {
		if explicit := metadataString(invocation.ExecContext.Metadata, "scope"); explicit != "" {
			return plugincontract.ServiceScope(explicit)
		}
	}
	switch {
	case strings.TrimSpace(invocation.ConversationID) != "":
		return plugincontract.ServiceScopeConversation
	case strings.TrimSpace(invocation.CharacterID) != "":
		return plugincontract.ServiceScopeCharacter
	case strings.TrimSpace(invocation.SpaceID) != "":
		return plugincontract.ServiceScopeDevice
	}
	return plugincontract.ServiceScopeGlobal
}

func metadataString(metadata map[string]any, key string) string {
	if metadata == nil {
		return ""
	}
	value, ok := metadata[key]
	if !ok {
		return ""
	}
	text, ok := value.(string)
	if !ok {
		return ""
	}
	return strings.TrimSpace(text)
}

func compareDotted(left, right string) int {
	leftParts, leftErr := parseDottedVersion(left)
	rightParts, rightErr := parseDottedVersion(right)
	if leftErr != nil || rightErr != nil {
		return strings.Compare(strings.TrimSpace(left), strings.TrimSpace(right))
	}
	for index := 0; index < len(leftParts) || index < len(rightParts); index++ {
		leftValue := 0
		if index < len(leftParts) {
			leftValue = leftParts[index]
		}
		rightValue := 0
		if index < len(rightParts) {
			rightValue = rightParts[index]
		}
		if leftValue != rightValue {
			if leftValue < rightValue {
				return -1
			}
			return 1
		}
	}
	return 0
}

func parseDottedVersion(value string) ([]int, error) {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return nil, errors.New("empty version")
	}
	parts := strings.Split(trimmed, ".")
	numbers := make([]int, 0, len(parts))
	for _, part := range parts {
		if part == "" {
			return nil, fmt.Errorf("empty version segment in %q", trimmed)
		}
		number, err := strconv.Atoi(part)
		if err != nil {
			return nil, fmt.Errorf("non numeric version segment %q in %q", part, trimmed)
		}
		if number < 0 {
			return nil, fmt.Errorf("negative version segment %q in %q", part, trimmed)
		}
		numbers = append(numbers, number)
	}
	return numbers, nil
}

func errorCodeFor(err error) string {
	switch {
	case errors.Is(err, ErrPluginPermissionDenied):
		return capability.ErrorCodePermissionDenied
	case errors.Is(err, ErrPluginScopeViolation):
		return capability.ErrorCodeScopeDenied
	case errors.Is(err, ErrPluginContractMismatch), errors.Is(err, ErrPluginContractVersion):
		return capability.ErrorCodeNotAvailable
	case errors.Is(err, ErrPluginNotReady), errors.Is(err, ErrPluginBindingMismatch), errors.Is(err, ErrPluginAdapterUnavailable):
		return capability.ErrorCodeProviderUnavailable
	default:
		return capability.ErrorCodeExecutionFailed
	}
}

func errorDetails(err error) map[string]any {
	var mismatch *ContractMismatchError
	if errors.As(err, &mismatch) {
		return map[string]any{"expectedContract": mismatch.Expected, "actualContract": mismatch.Actual}
	}
	var version *ContractVersionError
	if errors.As(err, &version) {
		return map[string]any{"hostVersion": version.Host, "requiredVersion": version.Required}
	}
	var denied *PermissionDeniedError
	if errors.As(err, &denied) {
		return map[string]any{"missingPermissions": append([]string(nil), denied.Missing...)}
	}
	var scope *ScopeViolationError
	if errors.As(err, &scope) {
		declared := make([]string, 0, len(scope.Declared))
		for _, item := range scope.Declared {
			declared = append(declared, string(item))
		}
		return map[string]any{"scope": string(scope.Scope), "declaredScopes": declared}
	}
	return nil
}

func failedResult(invocationID string, code string, err error, details map[string]any) capability.UnifiedToolResult {
	return capability.UnifiedToolResult{
		InvocationID: invocationID,
		Status:       capability.ToolResultStatusFailed,
		Error: &capability.ToolError{
			Code:        code,
			Message:     err.Error(),
			UserVisible: true,
			Cause:       err,
			Details:     details,
		},
	}
}
