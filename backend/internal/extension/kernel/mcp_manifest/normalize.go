package mcp_manifest

import (
	"strings"
)

func NormalizeTransportType(s string) MCPTransportType {
	switch strings.TrimSpace(strings.ToLower(s)) {
	case "stdio":
		return MCPTransportTypeStdio
	case "streamable_http", "streamable-http", "streamablehttp":
		return MCPTransportTypeStreamableHTTP
	case "sse":
		return MCPTransportTypeSSE
	default:
		return MCPTransportType(s)
	}
}

func NormalizeRestartPolicy(s string) MCPRestartPolicy {
	switch strings.TrimSpace(strings.ToLower(s)) {
	case "never":
		return MCPRestartPolicyNever
	case "on_failure", "on-failure", "onfailure":
		return MCPRestartPolicyOnFailure
	case "always":
		return MCPRestartPolicyAlways
	default:
		return MCPRestartPolicyOnFailure
	}
}

func NormalizeAuthType(s string) MCPAuthType {
	switch strings.TrimSpace(strings.ToLower(s)) {
	case "none":
		return MCPAuthTypeNone
	case "bearer_token", "bearer-token", "bearertoken", "bearer":
		return MCPAuthTypeBearerToken
	case "custom_headers", "custom-headers", "customheaders":
		return MCPAuthTypeCustomHeaders
	case "stdio_env", "stdio-env", "stdioenv":
		return MCPAuthTypeStdioEnv
	case "oauth":
		return MCPAuthTypeOAuth
	default:
		return MCPAuthType(s)
	}
}

func NormalizeCapabilityState(s string) MCPCapabilityState {
	switch strings.TrimSpace(strings.ToLower(s)) {
	case "disabled":
		return MCPCapabilityStateDisabled
	case "optional":
		return MCPCapabilityStateOptional
	case "required":
		return MCPCapabilityStateRequired
	default:
		return MCPCapabilityStateOptional
	}
}
