package workbenchterminal

type TaskReceiptSummary struct {
	Total            int    `json:"total"`
	Running          int    `json:"running"`
	Exited           int    `json:"exited"`
	Interrupted      int    `json:"interrupted"`
	Revoked          int    `json:"revoked"`
	Closed           int    `json:"closed"`
	LatestState      string `json:"latestState"`
	LatestSessionID  string `json:"latestSessionId,omitempty"`
	LastExitCode     *int   `json:"lastExitCode,omitempty"`
	VerificationKind string `json:"verificationKind"`
}

func SummarizeTaskReceipts(receipts []TaskReceipt) TaskReceiptSummary {
	summary := TaskReceiptSummary{Total: len(receipts), LatestState: "none", VerificationKind: "lifecycle-only-not-test-proof"}
	for i, receipt := range receipts {
		if i == 0 {
			summary.LatestState = receipt.State
			summary.LatestSessionID = receipt.SessionID
			if receipt.ExitCode != nil {
				value := *receipt.ExitCode
				summary.LastExitCode = &value
			}
		}
		switch receipt.State {
		case "running":
			summary.Running++
		case "exited":
			summary.Exited++
		case "interrupted":
			summary.Interrupted++
		case "revoked":
			summary.Revoked++
		case "closed":
			summary.Closed++
		}
	}
	return summary
}
