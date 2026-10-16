package agent

import "testing"

func TestBrowserAgentMutationBoundaryTable(t *testing.T) {
	for _, name := range []string{"browser_navigate", "browser_interact_click", "browser_interact_input", "browser_interact_scroll", "browser_interact_select", "browser_navigate_back", "browser_navigate_forward"} {
		if !browserAgentMutatesTab(name) {
			t.Errorf("tool %s bypasses user control", name)
		}
	}
	for _, name := range []string{"browser_dom_snapshot", "browser_screenshot", "browser_tab_close", "browser_session_close", "browser_session_create"} {
		if browserAgentMutatesTab(name) {
			t.Errorf("observation/cleanup %s was blocked", name)
		}
	}
}
