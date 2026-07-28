package repair_baseline

import (
	"context"
	"strings"
	"testing"

	"github.com/u-ai/backend/internal/extension/kernel/final_acceptance"
)

func TestBaseline_FinalAcceptance_NoEvidenceMustNotPass(t *testing.T) {
	suite := final_acceptance.DefaultSuite()
	report, err := suite.Run(context.Background())
	if err != nil {
		t.Fatalf("run final acceptance suite: %v", err)
	}

	for _, item := range report.Items {
		if item.Status == final_acceptance.StatusPassed {
			if !hasRealEvidence(item.Evidence) {
				t.Fatalf("item %s passed without real evidence; evidence=%v; Phase 8 must delete default Passed and require real runner evidence", item.ItemID, item.Evidence)
			}
		}
		if item.Required && item.Status == final_acceptance.StatusSkipped {
			t.Fatalf("required item %s must NOT be skipped; Phase 8 must use StatusBlocked instead of StatusSkipped for required items without a runner", item.ItemID)
		}
		if item.Required && item.Status == final_acceptance.StatusBlocked {
			t.Fatalf("required item %s must NOT be blocked; all required items must have real runners (Phase 10)", item.ItemID)
		}
	}

	if !report.ReleaseReady {
		t.Fatalf("ReleaseReady must be true when all required items pass with real evidence; failed=%d blocked=%d", report.Summary.Failed, report.Summary.Blocked)
	}
}

func hasRealEvidence(evidence []string) bool {
	for _, e := range evidence {
		trimmed := strings.TrimSpace(e)
		if trimmed == "" || trimmed == "verified" || trimmed == "passed" || trimmed == "success" {
			continue
		}
		return true
	}
	return false
}
