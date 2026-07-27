package equivalence

import (
	"context"
	"testing"
	"time"
)

func TestEquivalenceSuite(t *testing.T) {
	suite := NewSuite()
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	report, err := suite.Run(ctx)
	if err != nil {
		t.Fatalf("equivalence run failed: %v", err)
	}
	if report.Outcome != "passed" {
		t.Fatalf("equivalence outcome not passed: %s, failed=%d", report.Outcome, report.Summary.Failed)
	}
	if report.Summary.Failed > 0 {
		t.Fatalf("equivalence has %d failed checks", report.Summary.Failed)
	}

	uiCheckIDs := map[string]bool{
		"ui.slots":                 false,
		"ui.schema":                false,
		"ui.sandbox":               false,
		"ui.page_host":             false,
		"ui.ordering":              false,
		"desktop.extension_points": false,
	}
	for _, c := range report.Checks {
		if _, want := uiCheckIDs[c.CheckID]; want {
			uiCheckIDs[c.CheckID] = true
			if c.Status != CheckStatusPassed {
				t.Errorf("ui check %s not passed: status=%s err=%s", c.CheckID, c.Status, c.Error)
			}
			if c.Result != ResultImproved {
				t.Errorf("ui check %s result not improved: %s", c.CheckID, c.Result)
			}
			if len(c.Evidence) == 0 {
				t.Errorf("ui check %s has no evidence", c.CheckID)
			}
		}
	}
	for id, found := range uiCheckIDs {
		if !found {
			t.Errorf("ui check %s not registered", id)
		}
	}
}
