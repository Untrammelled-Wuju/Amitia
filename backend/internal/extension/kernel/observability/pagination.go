package observability

func paginateOps(ops []OperationRecord, limit int, cursor string) ([]OperationRecord, string, error) {
	if limit <= 0 {
		limit = 50
	}

	startIdx := 0
	if cursor != "" {
		for i, op := range ops {
			if op.OperationID > cursor {
				startIdx = i
				break
			}
		}
	}

	endIdx := startIdx + limit
	if endIdx > len(ops) {
		endIdx = len(ops)
	}

	page := ops[startIdx:endIdx]
	nextCursor := ""
	if endIdx < len(ops) {
		nextCursor = ops[endIdx-1].OperationID
	}

	return page, nextCursor, nil
}

func paginateInvs(invs []InvocationRecord, limit int, cursor string) ([]InvocationRecord, string, error) {
	if limit <= 0 {
		limit = 50
	}

	startIdx := 0
	if cursor != "" {
		for i, inv := range invs {
			if inv.InvocationID > cursor {
				startIdx = i
				break
			}
		}
	}

	endIdx := startIdx + limit
	if endIdx > len(invs) {
		endIdx = len(invs)
	}

	page := invs[startIdx:endIdx]
	nextCursor := ""
	if endIdx < len(invs) {
		nextCursor = invs[endIdx-1].InvocationID
	}

	return page, nextCursor, nil
}

func paginateEvents(events []RuntimeEventRecord, limit int, cursor string) ([]RuntimeEventRecord, string, error) {
	if limit <= 0 {
		limit = 50
	}

	startIdx := 0
	if cursor != "" {
		for i, evt := range events {
			if evt.EventID > cursor {
				startIdx = i
				break
			}
		}
	}

	endIdx := startIdx + limit
	if endIdx > len(events) {
		endIdx = len(events)
	}

	page := events[startIdx:endIdx]
	nextCursor := ""
	if endIdx < len(events) {
		nextCursor = events[endIdx-1].EventID
	}

	return page, nextCursor, nil
}

func paginateAudits(audits []AuditEvent, limit int, cursor string) ([]AuditEvent, string, error) {
	if limit <= 0 {
		limit = 50
	}

	startIdx := 0
	if cursor != "" {
		for i, a := range audits {
			if a.AuditID > cursor {
				startIdx = i
				break
			}
		}
	}

	endIdx := startIdx + limit
	if endIdx > len(audits) {
		endIdx = len(audits)
	}

	page := audits[startIdx:endIdx]
	nextCursor := ""
	if endIdx < len(audits) {
		nextCursor = audits[endIdx-1].AuditID
	}

	return page, nextCursor, nil
}
