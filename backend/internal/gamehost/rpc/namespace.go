package rpc

import (
	"fmt"
	"strings"

	"github.com/u-ai/backend/pkg/gameplugin/protocol"
)

type Namespace string

type Method string

func ParseMethod(method string) (Namespace, []string, error) {
	if err := protocol.ValidateMethod(method); err != nil {
		return "", nil, fmt.Errorf("invalid method: %w", err)
	}

	segments := strings.Split(method, ".")
	if len(segments) < 2 {
		return "", nil, fmt.Errorf("method must contain at least two segments")
	}

	namespace := Namespace(segments[0])
	return namespace, segments, nil
}

func IsReservedNamespace(namespace Namespace) bool {
	reserved := []string{"host", "plugin", "runtime", "service", "channel", "control", "emergency", "secret"}
	ns := string(namespace)
	for _, r := range reserved {
		if ns == r {
			return true
		}
	}
	return false
}

func IsReservedMethod(method Method) bool {
	return protocol.IsReservedNamespace(string(method))
}

func ValidateCustomNamespace(namespace Namespace) error {
	ns := string(namespace)
	if ns == "" {
		return fmt.Errorf("namespace must not be empty")
	}
	if len(ns) > 256 {
		return fmt.Errorf("namespace exceeds maximum length")
	}
	if IsReservedNamespace(namespace) {
		return fmt.Errorf("namespace '%s' is reserved by host", ns)
	}
	for _, r := range ns {
		if !isValidNamespaceChar(r) {
			return fmt.Errorf("namespace contains invalid character: %q", string(r))
		}
	}
	return nil
}

func isValidNamespaceChar(r rune) bool {
	if r >= 'a' && r <= 'z' {
		return true
	}
	if r >= '0' && r <= '9' {
		return true
	}
	if r == '_' || r == '-' || r == '.' {
		return true
	}
	return false
}

func NamespaceOfMethod(method Method) Namespace {
	parts := strings.SplitN(string(method), ".", 2)
	if len(parts) < 2 {
		return ""
	}
	return Namespace(parts[0])
}
