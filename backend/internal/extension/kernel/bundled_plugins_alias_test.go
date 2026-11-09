package kernel

import "testing"

func TestActiveTaskBundledPluginLegacyAlias(t *testing.T) {
	legacy := bundledPluginLegacyIDs["com.amitia/active-task"]
	if len(legacy) != 1 || legacy[0] != "com.amitia.builtin.active-task" {
		t.Fatalf("unexpected active task legacy aliases: %v", legacy)
	}
}
