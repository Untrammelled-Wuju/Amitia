package behavior

import "testing"

func TestDefaultFallbackGraphCompilesWithoutCycle(t *testing.T) {
	graph := DefaultFallbackGraph()
	if err := graph.Compile(); err != nil {
		t.Fatal(err)
	}
	for _, semantic := range []string{"emotion_happy", "emotion_excited", "emotion_proud"} {
		action, _, err := graph.Resolve(semantic, map[string]bool{"idle_normal": true})
		if err != nil {
			t.Fatalf("resolve %s: %v", semantic, err)
		}
		if action != "idle_normal" {
			t.Fatalf("resolve %s=%q, want idle_normal", semantic, action)
		}
	}
}
