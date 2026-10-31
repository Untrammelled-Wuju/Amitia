import fs from 'node:fs';
import os from 'node:os';
import path from 'node:path';
import { fileURLToPath } from 'node:url';
import { spawnSync } from 'node:child_process';

const sourceDir = path.resolve(path.dirname(fileURLToPath(import.meta.url)), '../../backend/internal/interaction');
const temporary = fs.mkdtempSync(path.join(os.tmpdir(), 'amitia-coding-allocation-'));
const go = process.env.AMITIA_GO_EXECUTABLE || 'go';
try {
  for (const filename of ['multi_agent_coding_worktree.go', 'multi_agent_coding_worktree_test.go']) {
    fs.copyFileSync(path.join(sourceDir, filename), path.join(temporary, filename));
  }
  fs.writeFileSync(path.join(temporary, 'contract_stub_test.go'), `package interaction
import "context"
type WorkerRunRequest struct {
  Source, WorkerRole, AssignmentID, ConversationID, WorkspaceID string
  WorkspaceRootURI, WorkspaceName, WorkspaceKind, SpaceID, WorkspaceDeviceID string
}
type AgentWorkerRunner interface {
  StartWorker(context.Context, WorkerRunRequest) (string, error)
}
`);
  const result = spawnSync(go, ['test', '-race', '-json', '-count=1', '-timeout=90s'], {
    cwd: temporary, env: { ...process.env, GO111MODULE: 'off', GOFLAGS: '', GOENV: 'off', GOWORK: 'off', GOTOOLCHAIN: 'local', GOPROXY: 'off' },
    encoding: 'utf8', timeout: 115000, maxBuffer: 4 * 1024 * 1024,
  });
  let tests = 0;
  let packages = 0;
  let fails = 0;
  for (const line of (result.stdout || '').split('\n')) {
    try {
      const event = JSON.parse(line);
      if (!event.Package) continue;
      if (event.Action === 'pass' && event.Test) tests++;
      if (event.Action === 'pass' && !event.Test) packages++;
      if (event.Action === 'fail') fails++;
    } catch (error) {
  console.error("[original-error]", error);
}
  }
  const passed = result.status === 0 && !result.error && tests > 0 && packages > 0 && fails === 0;
  console.log(JSON.stringify({ name: 'go.interaction.coding-allocation-isolated', passed, tests, packages, fails, exitCode: result.status, error: result.error?.message || '', detail: passed ? '' : (result.stdout + '\n' + result.stderr).slice(-4500) }, null, 2));
  if (!passed) process.exitCode = 1;
} finally {
  fs.rmSync(temporary, { recursive: true, force: true });
}
