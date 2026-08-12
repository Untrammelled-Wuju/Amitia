package mcp_manifest

import (
	"fmt"
)

func FromLegacyTransport(transport string) MCPTransportType {
	switch transport {
	case "stdio":
		return MCPTransportTypeStdio
	case "streamable_http":
		return MCPTransportTypeStreamableHTTP
	case "sse":
		return MCPTransportTypeSSE
	default:
		return MCPTransportType(transport)
	}
}

func ValidateLegacyConversion(transport, command, url string, env map[string]string) error {
	switch transport {
	case "stdio":
		if command == "" {
			return fmt.Errorf("mcp_manifest: legacy conversion: stdio command missing")
		}
	case "streamable_http":
		if url == "" {
			return fmt.Errorf("mcp_manifest: legacy conversion: remote url missing")
		}
	default:
		return fmt.Errorf("mcp_manifest: legacy conversion: unsupported transport: %s", transport)
	}
	if hasPlaintextEnv(env) {
		return fmt.Errorf("mcp_manifest: legacy conversion: plaintext env requires secret migration")
	}
	return nil
}

func hasPlaintextEnv(env map[string]string) bool {
	for k := range env {
		if isSecretLikeName(k) {
			return true
		}
	}
	return false
}
