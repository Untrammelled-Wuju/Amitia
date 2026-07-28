package repair_baseline

import (
	"reflect"
	"testing"

	"github.com/u-ai/backend/internal/chat"
)

func TestBaseline_Chat_HasKernelToolRuntimeSetter(t *testing.T) {
	serviceType := reflect.TypeOf((*chat.Service)(nil)).Elem()
	setter, ok := serviceType.MethodByName("SetToolRuntime")
	if !ok {
		t.Fatalf("chat.Service must expose SetToolRuntime to receive the kernel tool facade; Phase 5 must replace SetSkillRuntime(*extension.Runtime) with a kernel-only ModelToolRuntime interface")
	}
	if setter.Type.NumIn() != 1 {
		t.Fatalf("SetToolRuntime must accept a single ModelToolRuntime argument, got %d params", setter.Type.NumIn())
	}
}
