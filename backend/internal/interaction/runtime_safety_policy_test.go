package interaction

import (
	"context"
	"testing"

	"github.com/u-ai/backend/internal/devicemesh/coordination"
	"github.com/u-ai/backend/internal/safety"
)

type recordingSafetyIncidents struct {
	items []safety.Incident
	err   error
}

type staticSafetyPolicyReader struct {
	policy safety.RuntimePolicy
}

func (r *recordingSafetyIncidents) RecordIncident(_ context.Context, incident safety.Incident) error {
	r.items = append(r.items, incident)
	return r.err
}

func (r staticSafetyPolicyReader) RuntimePolicy(context.Context, string, string) (safety.RuntimePolicy, error) {
	return r.policy, nil
}

func TestRuntimePipelineUsesCallerRequestIDForSafetyIncidents(t *testing.T) {
	policy := safety.DefaultPolicy()
	policy.Revision = 7
	policy.Config.RestrictAdultContent = true
	recorder := &recordingSafetyIncidents{}
	pipeline := NewRuntimePipeline(nil, nil, nil)
	pipeline.SetSafetyPolicyReader(staticSafetyPolicyReader{policy: policy})
	pipeline.SetSafetyIncidentRecorder(recorder)
	scope := InteractionScope{SpaceID: "space", CharacterID: "role", ConversationID: "conversation", RequestID: "owned-parent/internal-key", Channel: "web"}
	ctx := coordination.WithScope(t.Context(), coordination.ExecutionScope{SpaceID: "space", CoreID: "space", ResourceOwnerID: "space", RequestID: "coordinated-request"})
	runtime := pipeline.Assemble(ctx, scope, &ProcessRequest{Message: "请生成色情内容", RequestID: "request-object"})
	if !runtime.Safety.Blocked || runtime.Safety.PolicyRevision != 7 || len(recorder.items) != 1 {
		t.Fatalf("runtime safety mismatch: safety=%+v incidents=%+v", runtime.Safety, recorder.items)
	}
	if recorder.items[0].RequestID != "coordinated-request" || runtime.safetyIncident.RequestID != "coordinated-request" {
		t.Fatalf("safety incident lost caller request id: event=%+v assembly=%+v", recorder.items[0], runtime.safetyIncident)
	}
}

func TestRuntimeAssemblyAppliesPolicyAndRecordsIncident(t *testing.T) {
	policy := safety.DefaultPolicy()
	policy.Config.PreventExclusiveDependency = true
	policy.Config.ViolationAction = "block"
	recorder := &recordingSafetyIncidents{}
	runtime := &RuntimeAssembly{
		SafetyPolicy:   policy,
		safetyGovernor: safety.NewPolicyGovernor(policy),
		safetyRecorder: recorder,
		safetyIncident: safety.Incident{
			SpaceID:        "space",
			OwnerID:        "owner",
			CoreID:         "core",
			RoleID:         "role",
			ConversationID: "conversation",
			RequestID:      "request",
			EventType:      "policy.violation",
		},
	}
	result, err := runtime.EvaluateGeneratedText(t.Context(), "你只需要我，不要相信别人。")
	if err != nil || !result.Audited || result.Allowed || !result.Replacement || len(recorder.items) != 1 {
		t.Fatalf("policy was not enforced: result=%+v incidents=%+v err=%v", result, recorder.items, err)
	}
	incident := recorder.items[0]
	if incident.SpaceID != "space" || incident.OwnerID != "owner" || incident.CoreID != "core" || incident.RoleID != "role" || incident.ConversationID != "conversation" || incident.RequestID != "request" || incident.PolicyRevision != policy.Revision {
		t.Fatalf("incident lost authority: %+v", incident)
	}
}

func TestRuntimeAssemblyAuditOnlyKeepsText(t *testing.T) {
	policy := safety.DefaultPolicy()
	policy.Config.PreventPretendingHuman = true
	policy.Config.ViolationAction = "audit_only"
	recorder := &recordingSafetyIncidents{}
	runtime := &RuntimeAssembly{SafetyPolicy: policy, safetyGovernor: safety.NewPolicyGovernor(policy), safetyRecorder: recorder, safetyIncident: safety.Incident{SpaceID: "space", OwnerID: "owner", CoreID: "core", EventType: "policy.violation"}}
	result, err := runtime.EvaluateGeneratedText(t.Context(), "我是真人。")
	if err != nil || !result.Audited || !result.Allowed || result.Cleaned != "我是真人。" || len(recorder.items) != 1 {
		t.Fatalf("audit-only policy mismatch: result=%+v incidents=%+v err=%v", result, recorder.items, err)
	}
}
