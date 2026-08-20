package mcp

import (
	"time"

	"github.com/u-ai/backend/internal/mcp/protocol"
)

type MCPStdioSpec struct {
	ServerID     string
	Command      string
	Args         []string
	WorkDir      string
	Env          map[string]string
	Capabilities protocol.ClientCapabilities

	StartTimeout    time.Duration
	ShutdownTimeout time.Duration
	MaxMessageBytes int64
}

type MCPStdioResolvedSpec struct {
	ServerID     string
	Executable   string
	Args         []string
	WorkDir      string
	Env          map[string]string
	Capabilities protocol.ClientCapabilities
}

type MCPStdioServerState string

const (
	MCPStdioStateStopped      MCPStdioServerState = "stopped"
	MCPStdioStateStarting     MCPStdioServerState = "starting"
	MCPStdioStateInitializing MCPStdioServerState = "initializing"
	MCPStdioStateReady        MCPStdioServerState = "ready"
	MCPStdioStateClosing      MCPStdioServerState = "closing"
	MCPStdioStateFailed       MCPStdioServerState = "failed"
)
