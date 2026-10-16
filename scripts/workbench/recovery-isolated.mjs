import fs from 'node:fs';
import os from 'node:os';
import path from 'node:path';
import { spawnSync } from 'node:child_process';
import { fileURLToPath } from 'node:url';

const root = path.resolve(path.dirname(fileURLToPath(import.meta.url)), '../..');
const source = path.join(root, 'backend/internal/interaction');
const temp = fs.mkdtempSync(path.join(os.tmpdir(), 'amitia-worker-recovery-'));
const pkg = path.join(temp, 'src/github.com/u-ai/backend/internal/interaction');
const decision = path.join(temp, 'src/github.com/u-ai/backend/internal/decision');
const go = process.env.AMITIA_GO_EXECUTABLE || 'go';
const files = [
  'multi_agent_recovery_validation.go', 'multi_agent_recovery_validation_test.go',
  'multi_agent_child_reconciliation.go', 'multi_agent_child_reconciliation_test.go',
  'multi_agent_worktree_evidence.go', 'coding_worker_evidence_gate.go',
  'coding_worker_evidence_gate_test.go', 'coding_worker_evidence_validation.go',
  'multi_agent_terminal_validation.go', 'multi_agent_terminal_validation_test.go',
  'multi_agent_result_validation.go', 'multi_agent_result_validation_test.go',
  'coding_worker_evidence_summary.go', 'coding_worker_evidence_summary_test.go',
];
try {
  fs.mkdirSync(pkg, { recursive: true });
  fs.mkdirSync(decision, { recursive: true });
  fs.writeFileSync(path.join(decision, 'goal.go'), 'package decision\ntype Goal struct { ID, SpaceID, ConversationID, CharacterID string; Revision int64 }\n');
  for (const name of files) {
    const original = path.join(source, name);
    const link = path.join(pkg, name);
    try { fs.symlinkSync(original, link); } catch { fs.copyFileSync(original, link); }
  }
  fs.writeFileSync(path.join(pkg, 'smoke_types_test.go'), `package interaction

type InteractionScope struct { SpaceID,ConversationID,CharacterID,RequestID string }
func (s InteractionScope) Normalize() InteractionScope { return s }
type InteractionStatus string
const(InteractionStatusCompleted InteractionStatus = "completed"; InteractionStatusFailed InteractionStatus = "failed"; InteractionStatusCancelled InteractionStatus = "cancelled"; InteractionStatusSuperseded InteractionStatus = "superseded"; InteractionStatusProcessing InteractionStatus = "processing"; InteractionStatusInterrupted InteractionStatus = "interrupted"; InteractionStatusArchived InteractionStatus = "archived")
type InteractionRecord struct { ID string; Scope InteractionScope; Status InteractionStatus }
func (r *InteractionRecord) IsTerminal() bool { return r != nil && (r.Status == InteractionStatusCompleted || r.Status == InteractionStatusFailed || r.Status == InteractionStatusCancelled || r.Status == InteractionStatusSuperseded || r.Status == InteractionStatusInterrupted || r.Status == InteractionStatusArchived) }
type MultiAgentRecoveryRef struct { ParentGoalID,ParentGoalSpaceID,ParentGoalConversationID,ParentGoalCharacterID string; ParentGoalRevision int64; AssignmentRefs []AssignmentRecoveryRef; Depth int }
type AssignmentRecoveryRef struct { AssignmentID,WorkerID,Objective,Status string; Dependencies []string }
type WorkerRunRequest struct { AssignmentID string }
type AgentWorkerRef struct { CharacterID string }
type AgentAssignment struct { ID,ChildInteractionID string; WorkerRef AgentWorkerRef; Status AgentAssignmentStatus }
type AgentAssignmentResult struct { AssignmentID,ChildInteractionID string; Status AgentAssignmentStatus }
type CoordinationAssignmentSnapshot struct { AssignmentID,ChildInteractionID,WorkerRole string; Status AgentAssignmentStatus; WorktreeReview *CodingWorktreeReview }
func (s AgentAssignmentStatus) IsTerminal() bool { return s == AssignmentFailed || s == AssignmentCancelled || s == AssignmentSucceeded }
func workerConversationID(r WorkerRunRequest) string { return "multi-agent-worker-"+r.AssignmentID }
type AgentAssignmentStatus string
const(AssignmentRunning AgentAssignmentStatus = "running"; AssignmentFailed AgentAssignmentStatus = "failed"; AssignmentCancelled AgentAssignmentStatus = "cancelled"; AssignmentSucceeded AgentAssignmentStatus = "succeeded")
`);
  const run = spawnSync(go, ['test', '-json', '-race', '-count=1', '-timeout=90s', '.'], {
    cwd: pkg, encoding: 'utf8', timeout: 115000, maxBuffer: 4 * 1024 * 1024,
    env: { ...process.env, GOPATH: temp, GO111MODULE: 'off', GOFLAGS: '', GOENV: 'off', GOWORK: 'off', GOTOOLCHAIN: 'local', GOPROXY: 'off' },
  });
  let tests = 0;
  let packages = 0;
  let failures = 0;
  for (const line of (run.stdout || '').split('\n')) {
    try {
      const event = JSON.parse(line);
      if (!event.Package) continue;
      if (event.Action === 'pass' && event.Test) tests++;
      if (event.Action === 'pass' && !event.Test) packages++;
      if (event.Action === 'fail') failures++;
    } catch {}
  }
  const passed = run.status === 0 && tests > 0 && packages > 0 && failures === 0;
  const failLines = (run.stdout || '').split('\n').filter(line => { try { const e = JSON.parse(line); return e.Action === 'fail' || (e.Action === 'output' && /FAIL|Error|unexpected|invalid|mismatch|gate:/i.test(e.Output || '')); } catch { return false; } });
  console.log(JSON.stringify({ name: 'go.interaction.worker-recovery-isolated', passed, tests, packages, failures, error: run.error?.message || '', detail: passed ? '' : (run.stderr + '\n' + failLines.join('\n')).slice(-12000) }));
  if (!passed) process.exitCode = 1;
} finally {
  fs.rmSync(temp, { recursive: true, force: true });
}
