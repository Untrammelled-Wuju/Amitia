package rpc

import (
	"fmt"
	"strings"

	"github.com/u-ai/backend/pkg/gameplugin/protocol"
)

type Namespace string

type Method string

const maxCustomNamespaceLength = 256

var reservedNamespaces = map[string]struct{}{
	"host": {}, "plugin": {}, "runtime": {}, "service": {}, "channel": {},
	"control": {}, "binary": {}, "emergency": {}, "secret": {}, "artifact": {},
	"permission": {},
}

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

// IsReservedNamespace reports whether namespace is inside a host-reserved
// namespace tree. Reserved roots cannot be reclaimed by declaring a child such
// as "host.vendor" because methods below that root are protocol-owned.
func IsReservedNamespace(namespace Namespace) bool {
	ns := strings.TrimSpace(string(namespace))
	if ns == "" {
		return false
	}
	root := ns
	if i := strings.IndexByte(ns, '.'); i >= 0 {
		root = ns[:i]
	}
	_, ok := reservedNamespaces[root]
	return ok
}

func IsReservedMethod(method Method) bool {
	return protocol.IsReservedNamespace(string(method))
}

func ValidateCustomNamespace(namespace Namespace) error {
	ns := string(namespace)
	if ns == "" {
		return fmt.Errorf("namespace must not be empty")
	}
	if len(ns) > maxCustomNamespaceLength {
		return fmt.Errorf("namespace exceeds maximum length")
	}
	if IsReservedNamespace(namespace) {
		return fmt.Errorf("namespace '%s' is reserved by host", ns)
	}
	segments := strings.Split(ns, ".")
	for _, segment := range segments {
		if segment == "" {
			return fmt.Errorf("namespace segments must not be empty")
		}
		for _, r := range segment {
			if !isValidNamespaceChar(r) {
				return fmt.Errorf("namespace contains invalid character: %q", string(r))
			}
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
	if r == '_' || r == '-' {
		return true
	}
	return false
}

// NamespaceCandidatesOfMethod returns every possible custom namespace for a
// method from most specific to least specific while always leaving at least one
// segment for the method name. For "vendor.core.inventory.read" the candidates
// are "vendor.core.inventory", "vendor.core", then "vendor".
func NamespaceCandidatesOfMethod(method Method) []Namespace {
	value := string(method)
	if err := protocol.ValidateMethod(value); err != nil {
		return nil
	}

	// Registered namespaces are capped at maxCustomNamespaceLength. Collect dot
	// offsets only within that prefix so a syntactically valid but very large RPC
	// method cannot cause quadratic prefix allocations during routing.
	limit := len(value)
	if limit > maxCustomNamespaceLength+1 {
		limit = maxCustomNamespaceLength + 1
	}
	dots := make([]int, 0, 8)
	for i := 0; i < limit; i++ {
		if value[i] == '.' && i <= maxCustomNamespaceLength {
			dots = append(dots, i)
		}
	}
	candidates := make([]Namespace, 0, len(dots))
	for i := len(dots) - 1; i >= 0; i-- {
		candidates = append(candidates, Namespace(value[:dots[i]]))
	}
	return candidates
}

// NamespaceOfMethod preserves the protocol-level root namespace classification
// used for reserved-method dispatch. Custom RPC routing must use
// NamespaceCandidatesOfMethod so hierarchical namespaces are resolved by the
// most specific registered prefix.
func NamespaceOfMethod(method Method) Namespace {
	parts := strings.SplitN(string(method), ".", 2)
	if len(parts) < 2 {
		return ""
	}
	return Namespace(parts[0])
}
