package mcp_manifest

import (
	"fmt"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"time"
)

const (
	MaxMCPServersPerExtension = 32
	MaxArgs                   = 128
	MaxEnvEntries             = 64
	MaxHeaders                = 64
	MaxMetadataEntries        = 64
	MaxSpecBytes              = 256 * 1024
	MaxServerIDLength         = 128
	StartupTimeoutMax         = 2 * time.Minute
	ShutdownTimeoutMax        = 30 * time.Second
)

var shellMetacharPattern = regexp.MustCompile(`[\s|&;(){}<>$\\\"'` + "`" + `]`)
var restrictedHeaderNames = map[string]bool{
	"host":            true,
	"content-length":  true,
	"origin":          true,
}
var secretLikeHeaderNames = map[string]bool{
	"authorization": true,
	"x-api-key":     true,
	"x-apikey":      true,
	"api-key":       true,
	"apikey":        true,
	"token":         true,
	"x-token":       true,
}
var secretLikeEnvNames = map[string]bool{
	"api_key":       true,
	"apikey":        true,
	"secret":        true,
	"password":      true,
	"token":         true,
	"access_token":  true,
	"refresh_token": true,
	"client_secret": true,
	"private_key":   true,
}

type ValidationError struct {
	Path string
	Code string
}

func (e ValidationError) Error() string {
	return fmt.Sprintf("mcp_manifest: validation error at %s: %s", e.Path, e.Code)
}

func Validate(spec MCPServerSpec, path string) []ValidationError {
	var errors []ValidationError
	add := func(field, code string) {
		errors = append(errors, ValidationError{
			Path: path + "." + field,
			Code: code,
		})
	}

	if spec.SchemaVersion != MCPServerSpecVersion {
		add("schemaVersion", "mcp_spec_unsupported_version")
	}

	validateTransport(&spec, path, &errors, add)
	validateCapabilities(&spec.Capabilities, path, &errors, add)
	validateLifecycle(&spec.Lifecycle, path, &errors, add)
	validateMetadata(spec.Metadata, path, &errors, add)

	return errors
}

func validateTransport(spec *MCPServerSpec, path string, errors *[]ValidationError, add func(string, string)) {
	switch spec.Transport.Type {
	case MCPTransportTypeStdio:
		if spec.Transport.Stdio == nil {
			add("transport.stdio", "mcp_transport_missing")
		} else {
			validateStdio(spec.Transport.Stdio, path, errors, add)
		}
		if spec.Transport.Remote != nil {
			add("transport.remote", "mcp_transport_remote_conflict")
		}
	case MCPTransportTypeStreamableHTTP:
		if spec.Transport.Remote == nil {
			add("transport.remote", "mcp_transport_missing")
		} else {
			validateRemote(spec.Transport.Remote, path, errors, add)
		}
		if spec.Transport.Stdio != nil {
			add("transport.stdio", "mcp_transport_stdio_conflict")
		}
	case MCPTransportTypeSSE:
		add("transport.type", "mcp_transport_sse_reserved")
	case "":
		add("transport.type", "mcp_transport_missing")
	default:
		add("transport.type", "mcp_transport_invalid")
	}
}

func validateStdio(stdio *MCPStdioTransportSpec, path string, errors *[]ValidationError, add func(string, string)) {
	if stdio.Command == "" {
		add("transport.stdio.command", "mcp_stdio_command_missing")
	} else {
		if shellMetacharPattern.MatchString(stdio.Command) {
			add("transport.stdio.command", "mcp_stdio_command_invalid")
		}
		if strings.Contains(stdio.Command, " ") ||
			strings.Contains(stdio.Command, "\t") ||
			strings.Contains(stdio.Command, "\n") ||
			strings.Contains(stdio.Command, "\x00") {
			add("transport.stdio.command", "mcp_stdio_command_invalid")
		}
	}

	if len(stdio.Args) > MaxArgs {
		add("transport.stdio.args", "mcp_stdio_args_exceeded")
	}
	for i, arg := range stdio.Args {
		if strings.Contains(arg, "\x00") {
			add(fmt.Sprintf("transport.stdio.args[%d]", i), "mcp_stdio_arg_invalid")
		}
	}

	if stdio.WorkDir != "" {
		if strings.HasPrefix(stdio.WorkDir, "/") || strings.HasPrefix(stdio.WorkDir, `C:\`) ||
			strings.HasPrefix(stdio.WorkDir, `c:\`) || strings.HasPrefix(stdio.WorkDir, `D:\`) ||
			strings.Contains(stdio.WorkDir, ":\\") {
			add("transport.stdio.workDir", "mcp_stdio_workdir_invalid")
		}
		if !strings.HasPrefix(stdio.WorkDir, "amitia://") {
			add("transport.stdio.workDir", "mcp_stdio_workdir_invalid")
		}
	}

	validateEnv(&stdio.Env, path, errors, add)
}

func validateEnv(env *MCPEnvSpec, path string, errors *[]ValidationError, add func(string, string)) {
	if env == nil {
		return
	}
	total := len(env.Values) + len(env.Secrets)
	if total > MaxEnvEntries {
		add("env", "mcp_env_entries_exceeded")
	}
	for k, v := range env.Values {
		if strings.Contains(k, "\x00") || strings.Contains(v, "\x00") {
			add(fmt.Sprintf("env.values.%s", k), "mcp_env_value_invalid")
		}
		if isSecretLikeName(k) {
			add(fmt.Sprintf("env.values.%s", k), "mcp_secret_inline_forbidden")
		}
	}
	for k := range env.Secrets {
		if strings.Contains(k, "\x00") {
			add(fmt.Sprintf("env.secrets.%s", k), "mcp_env_secret_invalid")
		}
	}
}

func validateRemote(remote *MCPRemoteTransportSpec, path string, errors *[]ValidationError, add func(string, string)) {
	if remote.URL == "" {
		add("transport.remote.url", "mcp_remote_url_missing")
	} else {
		u, err := url.Parse(remote.URL)
		if err != nil || u.Scheme == "" || u.Host == "" {
			add("transport.remote.url", "mcp_remote_url_invalid")
		} else {
			if u.Scheme != "https" && u.Scheme != "http" {
				add("transport.remote.url", "mcp_remote_url_invalid")
			}
			if u.Scheme == "http" {
				host := strings.ToLower(u.Hostname())
				if host != "localhost" && host != "127.0.0.1" {
					add("transport.remote.url", "mcp_remote_url_invalid")
				}
			}
			if u.User != nil {
				add("transport.remote.url", "mcp_remote_url_invalid")
			}
			if u.Fragment != "" {
				add("transport.remote.url", "mcp_remote_url_invalid")
			}
		}
	}

	if len(remote.Headers) > MaxHeaders {
		add("transport.remote.headers", "mcp_remote_headers_exceeded")
	}
	for k, v := range remote.Headers {
		lower := strings.ToLower(k)
		if restrictedHeaderNames[lower] {
			add(fmt.Sprintf("transport.remote.headers.%s", k), "mcp_remote_header_restricted")
		}
		if strings.Contains(k, "\r") || strings.Contains(k, "\n") ||
			strings.Contains(k, "\x00") {
			add(fmt.Sprintf("transport.remote.headers.%s", k), "mcp_remote_header_invalid")
		}
		if secretLikeHeaderNames[lower] {
			if v.Value != "" {
				add(fmt.Sprintf("transport.remote.headers.%s", k), "mcp_secret_inline_forbidden")
			}
		}
		if strings.Contains(v.Value, "\r") || strings.Contains(v.Value, "\n") ||
			strings.Contains(v.Value, "\x00") {
			add(fmt.Sprintf("transport.remote.headers.%s", k), "mcp_remote_header_value_invalid")
		}
	}

	if remote.Auth != nil {
		validateAuth(remote.Auth, path, errors, add)
	}
}

func validateAuth(auth *MCPAuthSpec, path string, errors *[]ValidationError, add func(string, string)) {
	switch auth.Type {
	case MCPAuthTypeNone, "":
		if auth.SecretRef != "" {
			add("auth.secretRef", "mcp_auth_invalid")
		}
	case MCPAuthTypeBearerToken:
		if auth.SecretRef == "" {
			add("auth.secretRef", "mcp_auth_invalid")
		}
	case MCPAuthTypeOAuth:
		if auth.SecretRef != "" {
			add("auth.secretRef", "mcp_auth_invalid")
		}
		if auth.OAuth != nil {
			if auth.OAuth.ProviderRef == "" && auth.OAuth.ClientRef == "" {
				add("auth.oauth", "mcp_auth_invalid")
			}
		}
	case MCPAuthTypeCustomHeaders, MCPAuthTypeStdioEnv:
		if auth.SecretRef == "" {
			add("auth.secretRef", "mcp_auth_invalid")
		}
	default:
		add("auth.type", "mcp_auth_invalid")
	}
}

func validateCapabilities(cap *MCPCapabilityPolicy, path string, errors *[]ValidationError, add func(string, string)) {
	validateServerFeature := func(name string, state MCPCapabilityState) {
		switch state {
		case "", MCPCapabilityStateDisabled, MCPCapabilityStateOptional, MCPCapabilityStateRequired:
		default:
			add("capabilities.server."+name, "mcp_capability_invalid")
		}
	}
	validateClientFeature := func(name string, state MCPCapabilityState) {
		switch state {
		case "", MCPCapabilityStateDisabled, MCPCapabilityStateOptional, MCPCapabilityStateRequired:
		default:
			add("capabilities.client."+name, "mcp_capability_invalid")
		}
	}
	if cap != nil {
		validateServerFeature("tools", cap.Server.Tools)
		validateServerFeature("resources", cap.Server.Resources)
		validateServerFeature("prompts", cap.Server.Prompts)
		validateServerFeature("tasks", cap.Server.Tasks)
		validateServerFeature("completion", cap.Server.Completion)
		validateServerFeature("logging", cap.Server.Logging)
		validateClientFeature("roots", cap.Client.Roots)
		validateClientFeature("sampling", cap.Client.Sampling)
		validateClientFeature("elicitation", cap.Client.Elicitation)
		validateClientFeature("tasks", cap.Client.Tasks)
	}
}

func validateLifecycle(lc *MCPLifecyclePolicy, path string, errors *[]ValidationError, add func(string, string)) {
	if lc == nil {
		return
	}
	switch lc.RestartPolicy {
	case "", MCPRestartPolicyNever, MCPRestartPolicyOnFailure, MCPRestartPolicyAlways:
	default:
		add("lifecycle.restartPolicy", "mcp_lifecycle_invalid")
	}
	if lc.MaxRestartAttempts < 0 {
		add("lifecycle.maxRestartAttempts", "mcp_lifecycle_invalid")
	}
	if lc.ReconnectPolicy.Enabled {
		if lc.ReconnectPolicy.MaxAttempts < 0 {
			add("lifecycle.reconnect.maxAttempts", "mcp_lifecycle_invalid")
		}
		switch lc.ReconnectPolicy.Backoff {
		case "", "fixed", "exponential":
		default:
			add("lifecycle.reconnect.backoff", "mcp_lifecycle_invalid")
		}
	}
	if lc.StartupTimeout != "" {
		d, err := time.ParseDuration(lc.StartupTimeout)
		if err != nil || d <= 0 || d > StartupTimeoutMax {
			add("lifecycle.startupTimeout", "mcp_lifecycle_invalid")
		}
	}
	if lc.ShutdownTimeout != "" {
		d, err := time.ParseDuration(lc.ShutdownTimeout)
		if err != nil || d <= 0 || d > ShutdownTimeoutMax {
			add("lifecycle.shutdownTimeout", "mcp_lifecycle_invalid")
		}
	}
}

func validateMetadata(meta map[string]string, path string, errors *[]ValidationError, add func(string, string)) {
	if meta == nil {
		return
	}
	if len(meta) > MaxMetadataEntries {
		add("metadata", "mcp_metadata_exceeded")
	}
	for k, v := range meta {
		if strings.Contains(k, "\x00") || strings.Contains(v, "\x00") {
			add(fmt.Sprintf("metadata.%s", k), "mcp_metadata_invalid")
		}
	}
}

func isSecretLikeName(name string) bool {
	lower := strings.ToLower(name)
	lower = strings.ReplaceAll(lower, "-", "_")
	lower = strings.ReplaceAll(lower, ".", "_")
	for secretName := range secretLikeEnvNames {
		if strings.Contains(lower, secretName) {
			return true
		}
	}
	return false
}

func validateSpecSize(spec []byte) error {
	if len(spec) > MaxSpecBytes {
		return fmt.Errorf("mcp_manifest: spec size %d exceeds maximum %d", len(spec), MaxSpecBytes)
	}
	return nil
}

func _() {
	_ = strconv.Itoa
}
