package builtin

import (
	"testing"

	"github.com/u-ai/backend/internal/extension/kernel/domain"
)

func TestShouldRetireBuiltinOnlyTargetsExplicitlyRetiredCoreExtension(t *testing.T) {
	active := map[domain.ExtensionID]struct{}{}
	if shouldRetireBuiltin("com.amitia/active-task", active) {
		t.Fatal("packaged builtin extension with the protected prefix must not be retired")
	}
	if !shouldRetireBuiltin(RetiredSearchExtensionID, active) {
		t.Fatal("explicitly retired builtin extension was not selected")
	}
	if shouldRetireBuiltin("com.amitia.builtin.active-task", active) {
		t.Fatal("active task migration must be handled by the bundled plugin alias lifecycle")
	}
	active[RetiredSearchExtensionID] = struct{}{}
	if shouldRetireBuiltin(RetiredSearchExtensionID, active) {
		t.Fatal("active builtin extension must not be retired")
	}
}
