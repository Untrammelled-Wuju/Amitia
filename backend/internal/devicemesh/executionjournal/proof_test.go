package executionjournal_test

import (
	"encoding/json"
	"errors"
	"path/filepath"
	"strings"
	"testing"

	executionjournal "github.com/u-ai/backend/internal/devicemesh/executionjournal"
	execution "github.com/u-ai/backend/internal/execution"
)

func emitProof(t *testing.T, line string) {
	t.Helper()
	t.Logf("m04-s11 %s", line)
}

func proofRequest(scope executionjournal.LedgerScope, key, capability string) executionjournal.LedgerRequest {
	return executionjournal.LedgerRequest{
		Scope:           scope,
		IdempotencyKey:  key,
		Trace:           execution.InvocationTrace{RootExecutionID: "root-s11", TraceID: "trace-s11", InvocationID: "inv-s11"},
		CapabilityID:    capability,
		BindingRevision: "rev-s11",
		PayloadHash:     "payload-s11",
		SideEffect:      execution.SideEffectClassWrite,
	}
}

func committedEntry(t *testing.T, ledger *executionjournal.InvocationLedger, request executionjournal.LedgerRequest) {
	t.Helper()
	entry, ran, err := ledger.Commit(t.Context(), request, func() (json.RawMessage, error) {
		return json.RawMessage(`{"delivered":true}`), nil
	})
	if err != nil || !ran || entry.State != executionjournal.EffectCommitted {
		t.Fatalf("准备已提交账本记录失败: ran=%v state=%s err=%v", ran, entry.State, err)
	}
}

func messageProof() executionjournal.EffectProof {
	return executionjournal.EffectProof{
		Kind:         executionjournal.ProofKindMessage,
		CapabilityID: "host.message.send",
		Target:       "conversation-42",
		RemoteID:     "mid-9001",
		Digest:       "payload-s11",
	}
}

func TestJournalProofMissingProofBlocksClaim(t *testing.T) {
	_, ledger := ledgerHarness(t, 2)
	scope := ledgerScope("space-s11-missing")
	request := proofRequest(scope, "key-missing", "host.message.send")
	committedEntry(t, ledger, request)

	claim, err := ledger.ExactlyOnceClaim(t.Context(), scope, "key-missing")
	if err != nil {
		t.Fatalf("查询申报资格失败: %v", err)
	}
	if claim.Claimable || claim.Reason != executionjournal.ClaimReasonMissingProof || claim.ProofFound || claim.Kind != executionjournal.ProofKindMessage {
		t.Fatalf("缺证明不得申报: %+v", claim)
	}
	emitProof(t, "claimable=false reason=missing_effect_proof state=committed proof=false kind=message")

	if _, _, err := ledger.RecordEffectProof(t.Context(), request, messageProof()); err != nil {
		t.Fatalf("补登效果证明失败: %v", err)
	}
	claim, err = ledger.ExactlyOnceClaim(t.Context(), scope, "key-missing")
	if err != nil {
		t.Fatalf("补证明后查询失败: %v", err)
	}
	if !claim.Claimable || claim.Reason != executionjournal.ClaimReasonOK || !claim.ProofFound {
		t.Fatalf("证明齐备应可申报: %+v", claim)
	}
	emitProof(t, "claimable=true reason=claimable state=committed proof=true kind=message")
}

func TestJournalProofMessageSend(t *testing.T) {
	_, ledger := ledgerHarness(t, 2)
	scope := ledgerScope("space-s11-message")
	request := proofRequest(scope, "key-message", "host.message.send")
	committedEntry(t, ledger, request)

	record, inserted, err := ledger.RecordEffectProof(t.Context(), request, messageProof())
	if err != nil {
		t.Fatalf("登记消息效果证明失败: %v", err)
	}
	if !inserted || record.Kind != executionjournal.ProofKindMessage || record.RemoteID != "mid-9001" || record.Target != "conversation-42" || record.PayloadDigest != "payload-s11" {
		t.Fatalf("消息证明落库内容不符: inserted=%v record=%+v", inserted, record)
	}
	emitProof(t, "kind=message inserted=true target=conversation-42 remote=mid-9001 digest=bound")

	again, inserted, err := ledger.RecordEffectProof(t.Context(), request, messageProof())
	if err != nil {
		t.Fatalf("同证明重复登记必须幂等: %v", err)
	}
	if inserted || again.RemoteID != "mid-9001" {
		t.Fatalf("重复登记不得追加新行: inserted=%v record=%+v", inserted, again)
	}
	emitProof(t, "repeat=idempotent inserted=false rows=1")
}

func TestJournalProofFileWrite(t *testing.T) {
	_, ledger := ledgerHarness(t, 2)
	scope := ledgerScope("space-s11-file")
	request := proofRequest(scope, "key-file", "host.file.write")
	committedEntry(t, ledger, request)

	record, inserted, err := ledger.RecordEffectProof(t.Context(), request, executionjournal.EffectProof{
		Kind:         executionjournal.ProofKindFileWrite,
		CapabilityID: "host.file.write",
		Target:       "/Volumes/UTD/report.md",
		Digest:       "payload-s11",
		Bytes:        2048,
	})
	if err != nil {
		t.Fatalf("登记文件写效果证明失败: %v", err)
	}
	if !inserted || record.Kind != executionjournal.ProofKindFileWrite || record.Bytes != 2048 || record.RemoteID != "" {
		t.Fatalf("文件写证明落库内容不符: inserted=%v record=%+v", inserted, record)
	}
	emitProof(t, "kind=file_write inserted=true bytes=2048 remote=empty digest=bound")

	claim, err := ledger.ExactlyOnceClaim(t.Context(), scope, "key-file")
	if err != nil {
		t.Fatalf("查询申报资格失败: %v", err)
	}
	if !claim.Claimable || claim.Kind != executionjournal.ProofKindFileWrite {
		t.Fatalf("文件写证明齐备应可申报: %+v", claim)
	}
	emitProof(t, "kind=file_write claimable=true reason=claimable")
}

func TestJournalProofDeviceControl(t *testing.T) {
	_, ledger := ledgerHarness(t, 2)
	scope := ledgerScope("space-s11-device")
	request := proofRequest(scope, "key-device", "host.device.control")
	committedEntry(t, ledger, request)

	record, inserted, err := ledger.RecordEffectProof(t.Context(), request, executionjournal.EffectProof{
		Kind:         executionjournal.ProofKindDeviceControl,
		CapabilityID: "host.device.control",
		Target:       "device-android-8864cd52",
		RemoteID:     "receipt-7f3",
		Digest:       "payload-s11",
	})
	if err != nil {
		t.Fatalf("登记设备控制效果证明失败: %v", err)
	}
	if !inserted || record.Kind != executionjournal.ProofKindDeviceControl || record.RemoteID != "receipt-7f3" {
		t.Fatalf("设备控制证明落库内容不符: inserted=%v record=%+v", inserted, record)
	}
	emitProof(t, "kind=device_control inserted=true target=device-android-8864cd52 remote=receipt-7f3")

	_, _, err = ledger.RecordEffectProof(t.Context(), request, executionjournal.EffectProof{
		Kind:         executionjournal.ProofKindDeviceControl,
		CapabilityID: "host.device.control",
		Target:       "device-android-8864cd52",
		RemoteID:     "receipt-other",
		Digest:       "payload-s11",
	})
	if !errors.Is(err, executionjournal.ErrEffectProofConflict) {
		t.Fatalf("同一效果面不同回执必须拒绝: err=%v", err)
	}
	emitProof(t, "conflict=rejected rows=1")
}

func TestJournalProofIncompleteRejected(t *testing.T) {
	_, ledger := ledgerHarness(t, 2)
	scope := ledgerScope("space-s11-incomplete")
	messageRequest := proofRequest(scope, "key-incomplete-message", "host.message.send")
	committedEntry(t, ledger, messageRequest)

	cases := []struct {
		name    string
		request executionjournal.LedgerRequest
		proof   executionjournal.EffectProof
		missing string
	}{
		{
			name:    "message-without-remote-id",
			request: messageRequest,
			proof: executionjournal.EffectProof{
				Kind:         executionjournal.ProofKindMessage,
				CapabilityID: "host.message.send",
				Target:       "conversation-42",
				Digest:       "payload-s11",
			},
			missing: "remoteId",
		},
		{
			name:    "message-without-digest",
			request: messageRequest,
			proof: executionjournal.EffectProof{
				Kind:         executionjournal.ProofKindMessage,
				CapabilityID: "host.message.send",
				Target:       "conversation-42",
				RemoteID:     "mid-9001",
			},
			missing: "digest",
		},
	}
	for _, testCase := range cases {
		_, _, err := ledger.RecordEffectProof(t.Context(), testCase.request, testCase.proof)
		if !errors.Is(err, executionjournal.ErrEffectProofIncomplete) || !strings.Contains(err.Error(), testCase.missing) {
			t.Fatalf("%s: 必须按缺失字段拒绝: err=%v", testCase.name, err)
		}
	}
	emitProof(t, "incomplete_message=rejected missing=remoteId,digest")

	fileRequest := proofRequest(scope, "key-incomplete-file", "host.file.write")
	committedEntry(t, ledger, fileRequest)
	_, _, err := ledger.RecordEffectProof(t.Context(), fileRequest, executionjournal.EffectProof{
		Kind:         executionjournal.ProofKindFileWrite,
		CapabilityID: "host.file.write",
		Target:       "/Volumes/UTD/report.md",
		Digest:       "payload-s11",
	})
	if !errors.Is(err, executionjournal.ErrEffectProofIncomplete) || !strings.Contains(err.Error(), "bytes") {
		t.Fatalf("文件写缺字节数必须拒绝: err=%v", err)
	}
	emitProof(t, "incomplete_file_write=rejected missing=bytes")
}

func TestJournalProofSurfaceGuards(t *testing.T) {
	_, ledger := ledgerHarness(t, 2)
	scope := ledgerScope("space-s11-surface")
	messageRequest := proofRequest(scope, "key-surface-message", "host.message.send")
	committedEntry(t, ledger, messageRequest)

	_, _, err := ledger.RecordEffectProof(t.Context(), messageRequest, executionjournal.EffectProof{
		Kind:         executionjournal.ProofKindFileWrite,
		CapabilityID: "host.message.send",
		Target:       "/Volumes/UTD/report.md",
		Digest:       "payload-s11",
		Bytes:        10,
	})
	if !errors.Is(err, executionjournal.ErrEffectProofKindMismatch) {
		t.Fatalf("跨效果面证明必须拒绝: err=%v", err)
	}
	emitProof(t, "kind_mismatch=rejected capability=host.message.send proof=file_write")

	_, _, err = ledger.RecordEffectProof(t.Context(), messageRequest, executionjournal.EffectProof{
		Kind:         executionjournal.ProofKindMessage,
		CapabilityID: "host.file.write",
		Target:       "conversation-42",
		RemoteID:     "mid-9001",
		Digest:       "payload-s11",
	})
	if !errors.Is(err, executionjournal.ErrEffectProofForeign) {
		t.Fatalf("异源提供方证明必须拒绝: err=%v", err)
	}
	emitProof(t, "foreign_capability=rejected")

	_, _, err = ledger.RecordEffectProof(t.Context(), messageRequest, executionjournal.EffectProof{
		Kind:         executionjournal.EffectProofKind("webhook"),
		CapabilityID: "host.message.send",
		Target:       "conversation-42",
		RemoteID:     "mid-9001",
		Digest:       "payload-s11",
	})
	if !errors.Is(err, executionjournal.ErrEffectProofKindInvalid) {
		t.Fatalf("未知证明类型必须拒绝: err=%v", err)
	}
	emitProof(t, "unknown_kind=rejected")

	plainRequest := proofRequest(scope, "key-surface-plain", "host.tool.execute")
	committedEntry(t, ledger, plainRequest)
	_, _, err = ledger.RecordEffectProof(t.Context(), plainRequest, executionjournal.EffectProof{
		Kind:         executionjournal.ProofKindMessage,
		CapabilityID: "host.tool.execute",
		Target:       "conversation-42",
		RemoteID:     "mid-9001",
		Digest:       "payload-s11",
	})
	if !errors.Is(err, executionjournal.ErrEffectProofSurface) {
		t.Fatalf("未定义效果面的能力必须拒绝: err=%v", err)
	}
	claim, err := ledger.ExactlyOnceClaim(t.Context(), scope, "key-surface-plain")
	if err != nil {
		t.Fatalf("查询申报资格失败: %v", err)
	}
	if claim.Claimable || claim.Reason != executionjournal.ClaimReasonSurfaceUnknown {
		t.Fatalf("未定义效果面不得申报: %+v", claim)
	}
	emitProof(t, "surface_undefined=rejected claimable=false reason=unknown_effect_surface")
}

func TestJournalProofPayloadMismatch(t *testing.T) {
	db, ledger := ledgerHarness(t, 2)
	scope := ledgerScope("space-s11-payload")
	request := proofRequest(scope, "key-payload", "host.message.send")
	committedEntry(t, ledger, request)

	_, _, err := ledger.RecordEffectProof(t.Context(), request, executionjournal.EffectProof{
		Kind:         executionjournal.ProofKindMessage,
		CapabilityID: "host.message.send",
		Target:       "conversation-42",
		RemoteID:     "mid-9001",
		Digest:       "payload-other",
	})
	if !errors.Is(err, executionjournal.ErrEffectProofMismatch) {
		t.Fatalf("载荷不一致的证明必须拒绝: err=%v", err)
	}
	emitProof(t, "payload_mismatch=rejected")

	scopeKey := identityScopeKey(t, scope)
	if _, err := db.Exec(`INSERT INTO kernel_invocation_effect_proof (scope_key,idempotency_key,proof_kind,capability_id,target,remote_id,payload_digest,bytes,observed_at,recorded_at)
VALUES (?,?,?,?,?,?,?,?,?,?)`, scopeKey, "key-payload", "message", "host.message.send", "conversation-42", "mid-9001", "payload-injected", 0, "2026-01-01T00:00:00Z", "2026-01-01T00:00:00Z"); err != nil {
		t.Fatalf("注入不一致证明失败: %v", err)
	}
	claim, err := ledger.ExactlyOnceClaim(t.Context(), scope, "key-payload")
	if err != nil {
		t.Fatalf("查询申报资格失败: %v", err)
	}
	if claim.Claimable || claim.Reason != executionjournal.ClaimReasonPayloadDiffers {
		t.Fatalf("证明载荷与账本不一致时不得申报: %+v", claim)
	}
	emitProof(t, "claimable=false reason=proof_payload_mismatch proof=true")
}

func TestJournalProofOrphanRejected(t *testing.T) {
	_, ledger := ledgerHarness(t, 2)
	scope := ledgerScope("space-s11-orphan")

	_, _, err := ledger.RecordEffectProof(t.Context(), proofRequest(scope, "key-orphan", "host.message.send"), messageProof())
	if !errors.Is(err, executionjournal.ErrEffectProofOrphan) {
		t.Fatalf("无账本记录必须拒绝登记: err=%v", err)
	}
	proofs, err := ledger.InspectProofs(t.Context(), scope, "key-orphan")
	if err != nil {
		t.Fatalf("读取效果证明失败: %v", err)
	}
	if len(proofs) != 0 {
		t.Fatalf("拒绝登记后不得留痕: %+v", proofs)
	}
	emitProof(t, "orphan=rejected rows=0")
}

func TestJournalProofClaimRequiresSettledLedger(t *testing.T) {
	db, ledger := ledgerHarness(t, 2)
	scope := ledgerScope("space-s11-settled")
	request := proofRequest(scope, "key-settled", "host.message.send")
	committedEntry(t, ledger, request)
	if _, _, err := ledger.RecordEffectProof(t.Context(), request, messageProof()); err != nil {
		t.Fatalf("登记效果证明失败: %v", err)
	}
	scopeKey := identityScopeKey(t, scope)
	if _, err := db.Exec(`UPDATE kernel_invocation_ledger SET effect_state='unknown' WHERE scope_key=? AND idempotency_key=?`, scopeKey, "key-settled"); err != nil {
		t.Fatalf("模拟未定状态失败: %v", err)
	}
	claim, err := ledger.ExactlyOnceClaim(t.Context(), scope, "key-settled")
	if err != nil {
		t.Fatalf("查询申报资格失败: %v", err)
	}
	if claim.Claimable || claim.Reason != executionjournal.ClaimReasonNotSettled || !claim.ProofFound {
		t.Fatalf("账本未定不得申报: %+v", claim)
	}
	emitProof(t, "state=unknown claimable=false reason=not_settled proof=true")
	if _, err := db.Exec(`UPDATE kernel_invocation_ledger SET effect_state='committed' WHERE scope_key=? AND idempotency_key=?`, scopeKey, "key-settled"); err != nil {
		t.Fatalf("恢复已提交状态失败: %v", err)
	}
	claim, err = ledger.ExactlyOnceClaim(t.Context(), scope, "key-settled")
	if err != nil {
		t.Fatalf("恢复后查询失败: %v", err)
	}
	if !claim.Claimable {
		t.Fatalf("恢复后应可申报: %+v", claim)
	}
	emitProof(t, "state=committed claimable=true reason=claimable")
}

func TestJournalProofImmutableRows(t *testing.T) {
	db, ledger := ledgerHarness(t, 2)
	scope := ledgerScope("space-s11-immutable")
	request := proofRequest(scope, "key-immutable", "host.message.send")
	committedEntry(t, ledger, request)
	if _, _, err := ledger.RecordEffectProof(t.Context(), request, messageProof()); err != nil {
		t.Fatalf("登记效果证明失败: %v", err)
	}
	if _, err := db.Exec(`UPDATE kernel_invocation_effect_proof SET remote_id='mid-tampered' WHERE idempotency_key='key-immutable'`); !executionjournal.IsEffectProofImmutableError(err) {
		t.Fatalf("效果证明必须不可改写: err=%v", err)
	}
	if _, err := db.Exec(`DELETE FROM kernel_invocation_effect_proof WHERE idempotency_key='key-immutable'`); !executionjournal.IsEffectProofImmutableError(err) {
		t.Fatalf("效果证明必须不可删除: err=%v", err)
	}
	proofs, err := ledger.InspectProofs(t.Context(), scope, "key-immutable")
	if err != nil {
		t.Fatalf("读取效果证明失败: %v", err)
	}
	if len(proofs) != 1 || proofs[0].RemoteID != "mid-9001" {
		t.Fatalf("篡改尝试后证明必须原样保留: %+v", proofs)
	}
	emitProof(t, "update=blocked delete=blocked rows=1 remote=mid-9001")
}

func TestJournalProofSurvivesRestart(t *testing.T) {
	path := filepath.Join(t.TempDir(), "proof.db")
	scope := ledgerScope("space-s11-restart")
	request := proofRequest(scope, "key-restart", "host.device.control")

	db, ledger := identityLedger(t, path)
	committedEntry(t, ledger, request)
	if _, inserted, err := ledger.RecordEffectProof(t.Context(), request, executionjournal.EffectProof{
		Kind:         executionjournal.ProofKindDeviceControl,
		CapabilityID: "host.device.control",
		Target:       "device-android-8864cd52",
		RemoteID:     "receipt-restart",
		Digest:       "payload-s11",
	}); err != nil || !inserted {
		t.Fatalf("登记设备控制证明失败: inserted=%v err=%v", inserted, err)
	}
	if err := db.Close(); err != nil {
		t.Fatalf("关闭数据库失败: %v", err)
	}

	restarted, restartedLedger := identityLedger(t, path)
	defer restarted.Close()
	claim, err := restartedLedger.ExactlyOnceClaim(t.Context(), scope, "key-restart")
	if err != nil {
		t.Fatalf("重启后查询申报资格失败: %v", err)
	}
	if !claim.Claimable || claim.Kind != executionjournal.ProofKindDeviceControl {
		t.Fatalf("重启后证明必须仍成立: %+v", claim)
	}
	proofs, err := restartedLedger.InspectProofs(t.Context(), scope, "key-restart")
	if err != nil {
		t.Fatalf("重启后读取效果证明失败: %v", err)
	}
	if len(proofs) != 1 || proofs[0].RemoteID != "receipt-restart" {
		t.Fatalf("重启后证明必须原样保留: %+v", proofs)
	}
	emitProof(t, "restart=preserved rows=1 claimable=true kind=device_control")

	_, inserted, err := restartedLedger.RecordEffectProof(t.Context(), request, executionjournal.EffectProof{
		Kind:         executionjournal.ProofKindDeviceControl,
		CapabilityID: "host.device.control",
		Target:       "device-android-8864cd52",
		RemoteID:     "receipt-restart",
		Digest:       "payload-s11",
	})
	if err != nil || inserted {
		t.Fatalf("重启后重复登记必须幂等: inserted=%v err=%v", inserted, err)
	}
	if _, err := restarted.Exec(`DELETE FROM kernel_invocation_effect_proof`); !executionjournal.IsEffectProofImmutableError(err) {
		t.Fatalf("重启后不可删除约束必须生效: err=%v", err)
	}
	emitProof(t, "restart_repeat=idempotent delete=blocked")
}
