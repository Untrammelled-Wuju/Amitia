package chat

import (
	"context"
	"sync/atomic"
	"testing"
	"time"

	"github.com/u-ai/backend/internal/interaction"
	"gorm.io/gorm"
)

type agentToolLeaseProbe struct {
	db            *gorm.DB
	recorder      *assistantTurnRecorder
	call          agentToolCall
	steal         bool
	expire        bool
	executions    atomic.Int32
	captured      bool
	receivedClaim bool
	last          agentToolExecution
}

func (p *agentToolLeaseProbe) ProcessMessageCtx(ctx context.Context, req *interaction.ProcessRequest) (*interaction.ProcessResponse, error) {
	claim, present := interaction.ClaimedExecutionLease(ctx)
	p.receivedClaim = present && claim.InteractionID == req.InteractionID && claim.OwnerInstanceID != ""
	if p.steal {
		if err := p.db.Exec("UPDATE interaction_records SET owner_instance_id = 'successor-owner' WHERE id = ?", req.InteractionID).Error; err != nil {
			return nil, err
		}
	}
	if p.expire {
		if err := p.db.Exec("UPDATE interaction_records SET heartbeat_at = ? WHERE id = ?",
			time.Now().UTC().Add(-3*time.Minute), req.InteractionID).Error; err != nil {
			return nil, err
		}
	}
	p.last = newAgentToolLedger(p.recorder, p.call).execution(ctx, func() agentToolExecution {
		p.executions.Add(1)
		return agentToolExecution{Outcome: toolExecOutcome{Status: "SUCCEEDED", Found: true}}
	})
	p.captured = true
	if p.last.Outcome.ErrorCode == "TOOL_REQUIRES_RECONCILIATION" {
		return nil, interaction.ErrToolReconciliationRequired
	}
	return &interaction.ProcessResponse{
		RequestID: req.RequestID, ConversationID: req.ConversationID,
		CharacterID: req.CharacterID, Reply: "confirmed",
	}, nil
}

func TestAgentToolLedgerFencesExpiredOrStolenParentLease(t *testing.T) {
	for _, scenario := range []string{"valid", "stolen", "expired"} {
		t.Run(scenario, func(t *testing.T) {
			db, recorder, call := setupAgentToolLedger(t)
			if err := db.AutoMigrate(&interaction.InteractionRecordModel{}); err != nil {
				t.Fatal(err)
			}
			probe := &agentToolLeaseProbe{
				db: db, recorder: recorder, call: call,
				steal: scenario == "stolen", expire: scenario == "expired",
			}
			tracker := interaction.NewSQLiteInteractionTracker(db)
			orch := interaction.NewOrchestratorWithStores(interaction.DefaultOrchestratorConfig(), probe, tracker, nil)
			orch.SetReady(true)
			_, err := orch.Process(context.Background(), &interaction.ProcessRequest{
				SpaceID: "space", CharacterID: "character",
				ConversationID: call.Scope.ConversationID, Channel: "web",
				Source: "web", RequestID: call.Scope.RequestID,
				Message: "verify lease", TurnID: recorder.TurnID,
			})
			if !probe.captured || !probe.receivedClaim {
				t.Fatalf("real orchestrator did not propagate execution lease: captured=%t claim=%t error=%v",
					probe.captured, probe.receivedClaim, err)
			}
			if scenario == "valid" {
				if probe.executions.Load() != 1 || probe.last.Outcome.HasError {
					t.Fatalf("current owner could not execute authorized operation: %+v err=%v", probe.last, err)
				}
			} else {
				if probe.executions.Load() != 0 || probe.last.Outcome.ErrorCode != "TOOL_REQUIRES_RECONCILIATION" {
					t.Fatalf("%s original owner dispatched external tool after lease loss: %+v error=%v", scenario, probe.last, err)
				}
			}
		})
	}
}
