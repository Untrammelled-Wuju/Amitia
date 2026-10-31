package main

import (
	"context"
	"testing"

	"github.com/u-ai/backend/internal/devicemesh/business"
)

func TestOwnedMessageContextSurvivesClientDisconnect(t *testing.T) {
	requestContext, cancel := context.WithCancel(context.Background())
	cancel()
	detached := ownedMessageContext(requestContext, "thread-1")
	if detached.Done() != nil {
		t.Fatal("detached execution context remained cancellable by the client request")
	}
	ingress, ok := business.IngressFromContext(detached)
	if !ok || ingress.Source != "owned" || ingress.Channel != "web" || ingress.ThreadID != "thread-1" {
		t.Fatalf("detached ingress = %+v, %v", ingress, ok)
	}
}
