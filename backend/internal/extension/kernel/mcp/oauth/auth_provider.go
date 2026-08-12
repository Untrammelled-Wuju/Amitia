package oauth

import (
	"context"
	"net/http"
)

type MCPServerRef struct {
	ServerID string
	Resource string
}

type MCPRequestAuthenticator interface {
	Authorize(ctx context.Context, server MCPServerRef, request *http.Request) error
}
