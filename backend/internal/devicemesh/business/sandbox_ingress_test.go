package business

import (
	"context"
	"testing"
)

func TestSandboxConstraintFromContext(t *testing.T) {
	ctx := WithIngress(context.Background(), Ingress{SandboxRoot: " /tmp/channel-jail ", SandboxAllowNetwork: true})
	root, allowNetwork := sandboxConstraintFromContext(ctx)
	if root != "/tmp/channel-jail" || !allowNetwork {
		t.Fatalf("unexpected sandbox constraint root=%q allowNetwork=%v", root, allowNetwork)
	}
	if root, allowNetwork := sandboxConstraintFromContext(context.Background()); root != "" || allowNetwork {
		t.Fatalf("context without ingress must stay unrestricted, got root=%q allowNetwork=%v", root, allowNetwork)
	}
}
