package business

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"time"

	"github.com/u-ai/backend/internal/continuity"
	"github.com/u-ai/backend/internal/devicemesh/coordination"
)

func (e *Engine) SignalContinuity(ctx context.Context, request Request, signal continuity.Signal) ([]string, error) {
	if !continuity.ValidWaitType(signal.WaitType) || signal.WaitType == continuity.WaitTypeUser || signal.OccurredAt.IsZero() || len(body(signal)) > 64<<10 {
		return nil, errors.New("持续事项事件参数无效")
	}
	ctx, scope, finish, err := e.continuityAuthority(ctx, request)
	if err != nil {
		return nil, err
	}
	defer finish()
	signal.SpaceID = scope.SpaceID
	digest := sha256.Sum256(body(signal))
	fingerprint := hex.EncodeToString(digest[:])
	query := coordination.DataQuery{ResourceKind: "continuity", Limit: 128}
	resolved := []string{}
	for {
		snapshot, err := e.data.Snapshot(ctx, scope, query)
		if err != nil {
			return resolved, err
		}
		if err := coordination.ValidateSnapshot(scope, snapshot); err != nil {
			return resolved, err
		}
		for _, resource := range snapshot.Resources {
			if resource.Kind != "continuity" {
				continue
			}
			var document OwnedContinuity
			if json.Unmarshal(resource.Body, &document) != nil {
				return resolved, coordination.ErrWrongOwner
			}
			present := PresentContinuity(document, scope, time.Now())
			if present.Thread.Status == continuity.ThreadStatusPaused || present.Thread.Status.IsTerminal() || document.Lease != nil && document.Lease.State == "running" {
				continue
			}
			for _, wait := range document.Waits {
				if wait.Status != continuity.WaitStatusWaiting || !continuity.WaitMatchesSignal(wait, signal) {
					continue
				}
				eventID := sha256.Sum256([]byte(request.RequestID + "\x00" + document.Thread.ID + "\x00" + wait.ID))
				mutationRequest := request
				mutationRequest.RequestID = "signal/" + hex.EncodeToString(eventID[:])
				updated, _, err := e.Continuity(ctx, mutationRequest, ContinuityMutation{ID: document.Thread.ID, ExpectedRevision: document.Thread.Revision, Action: "resolve_wait", WaitID: wait.ID, Resume: wait.AutoResume, SignalHash: fingerprint})
				if err != nil {
					return resolved, err
				}
				document = updated
				resolved = append(resolved, wait.ID)
			}
		}
		next := snapshot.NextCursors["continuity"]
		if next == "" {
			break
		}
		if next == query.Cursor {
			return resolved, coordination.ErrRequestConflict
		}
		query.Cursor = next
	}
	return resolved, e.coordination.Validate(ctx, scope)
}
