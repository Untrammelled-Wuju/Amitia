package executionjournal

import (
	"context"
	"errors"
	"fmt"
	"strings"
)

var ErrLedgerInvocationMismatch = errors.New("执行账本：该幂等键已绑定到另一次调用，拒绝把它的结果交给新的调用")

type LedgerBinding struct {
	ScopeKey       string
	InvocationID   string
	IdempotencyKey string
}

func (b LedgerBinding) String() string {
	return fmt.Sprintf("scope=%q invocation=%q key=%q", b.ScopeKey, b.InvocationID, b.IdempotencyKey)
}

func (e LedgerEntry) Binding() LedgerBinding {
	return LedgerBinding{ScopeKey: e.ScopeKey, InvocationID: e.InvocationID, IdempotencyKey: e.IdempotencyKey}
}

type BindingVerdict string

const (
	BindingAbsent  BindingVerdict = "absent"
	BindingOwned   BindingVerdict = "owned"
	BindingForeign BindingVerdict = "foreign"
)

func (v BindingVerdict) Owned() bool {
	return v == BindingOwned
}

func (l *InvocationLedger) InspectBinding(ctx context.Context, scope LedgerScope, invocationID, idempotencyKey string) (LedgerEntry, BindingVerdict, error) {
	invocation := strings.TrimSpace(invocationID)
	if invocation == "" || len(invocation) > 512 {
		return LedgerEntry{}, "", fmt.Errorf("%w: invocationId 长度必须在 1..512", ErrLedgerKeyInvalid)
	}
	entry, found, err := l.Lookup(ctx, scope, idempotencyKey)
	if err != nil {
		return LedgerEntry{}, "", err
	}
	if !found {
		return LedgerEntry{}, BindingAbsent, nil
	}
	if entry.InvocationID != invocation {
		return entry, BindingForeign, nil
	}
	return entry, BindingOwned, nil
}
