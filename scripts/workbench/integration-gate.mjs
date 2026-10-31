import fs from 'node:fs';
import os from 'node:os';
import path from 'node:path';
import { fileURLToPath } from 'node:url';
import { spawnSync } from 'node:child_process';

const root = path.resolve(path.dirname(fileURLToPath(import.meta.url)), '../..');
const backend = path.join(root, 'backend');
const front = path.join(root, 'front');
const desktop = path.join(root, 'desktop');
const mobile = path.join(root, 'mobile_app');
const preflightOnly = process.argv.includes('--preflight-only');
const worktreeOnly = process.argv.includes('--worktree-only');
const liveApi = process.argv.includes('--live-api');
const developmentOnly = process.argv.includes('--development-only');
const go = process.env.AMITIA_GO_EXECUTABLE || (process.platform === 'win32' && fs.existsSync('C:\\Code\\Go\\bin\\go.exe') ? 'C:\\Code\\Go\\bin\\go.exe' : 'go');
const flutter = process.env.AMITIA_FLUTTER_EXECUTABLE || 'flutter';
const pnpm = process.env.AMITIA_PNPM_EXECUTABLE || 'pnpm';
const results = [];

function execute(name, program, args, cwd, timeout = 120000, requireGoTestEvidence = false) {
  const started = Date.now();
  const outcome = spawnSync(program, args, { cwd, encoding: 'utf8', timeout, windowsHide: true, maxBuffer: 2 * 1024 * 1024, env: (name === 'go.workbenchhttp.security-race' || name === 'go.worktree.real-git-isolated' || name === 'go.workbenchterminal.output-buffer-isolated' || name === 'go.workbenchterminal.stream-tickets-isolated' || name === 'go.workbenchterminal.manager-isolated') ? { ...process.env, GO111MODULE: 'off', GOFLAGS: '', GOENV: 'off', GOWORK: 'off', GOTOOLCHAIN: 'local', GOPROXY: 'off' } : process.env });
  let status = outcome.error?.code === 'ENOENT' ? 'blocked' : outcome.status === 0 ? 'passed' : 'failed';
  let passingTests = 0;
  let passingPackages = 0;
  let failingEvents = 0;
  if (requireGoTestEvidence && status === 'passed') {
    for (const line of (outcome.stdout || '').split('\n')) {
      try {
        const event = JSON.parse(line);
        if (typeof event.Package !== 'string' || event.Package.length === 0) continue;
        if (event.Action === 'pass' && typeof event.Test === 'string' && event.Test.length > 0) passingTests++;
        if (event.Action === 'pass' && !event.Test) passingPackages++;
        if (event.Action === 'fail') failingEvents++;
      } catch (error) {
  console.error("[original-error]", error);
}
    }
    if (passingTests === 0 || passingPackages === 0 || failingEvents > 0) status = 'failed';
  }
  const entry = {
    name,
    status,
    exitCode: outcome.status,
    durationMillis: Date.now() - started,
    ...(requireGoTestEvidence ? { passingTestEvents: passingTests, passingPackageEvents: passingPackages, failingEvents } : {}),
    detail: (outcome.error?.message || [outcome.stdout, outcome.stderr].filter(Boolean).join('\n')).slice(-6000) + (requireGoTestEvidence && status === 'failed' ? '\nGo test evidence must include test and package passes, without failures' : ''),
  };
  results.push(entry);
  return entry;
}

function blocked(name, detail) {
  results.push({ name, status: 'blocked', exitCode: null, durationMillis: 0, detail });
}

const declaredGo = fs.readFileSync(path.join(backend, 'go.mod'), 'utf8').match(/^go\s+(\d+\.\d+(?:\.\d+)?)\s*$/m)?.[1];
const currentGo = execute('go.toolchain', go, ['version'], path.dirname(root), 20000);
const correctGo = currentGo.status === 'passed' && currentGo.detail.includes(`go${declaredGo} `);
if (currentGo.status === 'passed' && !correctGo) {
  blocked('go.version.requirement', `当前工具链不符合 backend/go.mod 要求的 Go ${declaredGo}，拒绝以低版本结果冒充完整构建`);
}
const checkedPnpm = execute('pnpm.available', pnpm, ['--version'], root, 20000);
const checkedFlutter = execute('flutter.available', flutter, ['--version'], mobile, 20000);
const hasFrontDeps = fs.existsSync(path.join(front, 'node_modules', '.bin', process.platform === 'win32' ? 'vue-tsc.cmd' : 'vue-tsc'));
const hasDesktopDeps = fs.existsSync(path.join(desktop, 'node_modules', '.bin', process.platform === 'win32' ? 'tsc.cmd' : 'tsc'));
if (!hasFrontDeps) blocked('front.dependencies', '缺少本工程 front/node_modules/vue-tsc；不自动下载依赖');
if (!hasDesktopDeps) blocked('desktop.dependencies', '缺少本工程 desktop/node_modules/tsc；不自动下载依赖');

if (!preflightOnly) {
  const requestRecovery = execute('node.workbench-task-request-recovery', process.execPath, ['front/scripts/workbench-task-request-smoke.cjs'], root, 60000);
  requestRecovery.scope = 'actual TypeScript deadline helper executed with hung/aborted mock requests; Web/Flutter widget wiring assertions';
  const checkpoints = execute('node.workbench-task-checkpoint-protocol', process.execPath, ['front/scripts/workbench-task-checkpoint-smoke.cjs'], root, 60000);
  checkpoints.scope = 'actual TypeScript durable checkpoint parser, including invalid identity and contradictory counts';
  const taskProtocol = execute('node.workbench-task-protocol', process.execPath, ['front/scripts/workbench-task-protocol-smoke.cjs'], root, 60000);
  taskProtocol.scope = 'original TypeScript task-list decoder and Web/Flutter wiring assertions; not a Dart analyzer';
  const resourceLifecycle = execute('node.workbench-resource-lifecycle', process.execPath, ['front/scripts/workbench-resource-lifecycle-smoke.cjs'], root, 60000);
  resourceLifecycle.scope = 'original WorkbenchResourceView Vue setup executes with isolated API adapters; resource routing, KeepAlive reactivation and remote-device isolation';
  const historyLifecycle = execute('node.workbench-task-history-lifecycle', process.execPath, ['front/scripts/workbench-history-lifecycle-smoke.cjs'], root, 60000);
  historyLifecycle.scope = 'original WorkbenchTasksView Vue setup executes with simulated async backend, version refresh and stale-response prevention';
  const eventProtocol = execute('node.workbench-event-protocol', process.execPath, ['front/scripts/workbench-open-event-smoke.cjs'], root, 60000);
  eventProtocol.scope = 'actual TypeScript workbench.open event parser, published schemas and Flutter resource-epoch source assertions';
  const lifecycle = execute('node.workbench-multi-client-lifecycle', process.execPath, ['front/scripts/workbench-lifecycle-smoke.cjs'], root, 60000);
  lifecycle.scope = 'real TypeScript Workbench Store runtime with isolated Pinia/Vue adapters; Flutter source lifecycle assertions, not Dart analyzer';
  execute('node.workbench-auth-smoke-tests', process.execPath, ['--test', 'scripts/workbench/authenticated-smoke.test.mjs'], root, 60000);
  execute('node.workbench-terminal-stream-isolated', process.execPath, ['--test', 'scripts/workbench/terminal-stream-isolated.test.mjs'], root, 60000);
  execute('node.workbench-terminal-cursor-isolated', process.execPath, ['--test', 'scripts/workbench/terminal-cursor-isolated.test.mjs'], root, 60000);
  execute('go.workbenchterminal.owner-scope-source', process.platform === 'win32' ? 'python' : 'python3', ['scripts/workbench/terminal-scope-source-gate.py'], root, 60000);
  execute('node.workbench-dock-detach-isolated', process.execPath, ['--test', 'scripts/workbench/dock-detach-isolated.test.mjs'], root, 60000);
  execute('node.workbench-browser-lifecycle-isolated', process.execPath, ['scripts/workbench/browser-lifecycle-isolated.mjs'], root, 60000);
  execute('node.workbench-browser-lease-isolated', process.execPath, ['scripts/workbench/browser-lease-isolated.mjs'], root, 60000);
  execute('node.workbench-browser-mutation-generation', process.execPath, ['scripts/workbench/browser-mutation-generation-isolated.mjs'], root, 60000);
  execute('go.workbench-browser-mutation-routes', process.platform === 'win32' ? 'python' : 'python3', ['scripts/workbench/browser-mutation-route-source-gate.py'], root, 60000);
  execute('go.device-mesh-browser-scope-source-gate', process.platform === 'win32' ? 'python' : 'python3', ['scripts/workbench/device-mesh-browser-scope-source-gate.py'], root, 60000);
  const mobileBrowser = execute('flutter.workbench-browser-lifecycle-source-gate', process.platform === 'win32' ? 'python' : 'python3', ['scripts/workbench/flutter-browser-lifecycle-source-gate.py'], root, 60000);
  mobileBrowser.scope = 'Flutter Browser widget original-source lifecycle assertions; Dart analyzer and real devices not available in isolation';
  const allocation = execute('go.interaction.coding-allocation-isolated', process.execPath,
    ['scripts/workbench/coding-allocation-isolated.mjs'], root, 120000);
  allocation.scope = 'original Coding Worker allocator wrapper source plus contract stubs, compiled and run with Go race detector; not complete Interaction module';
  const recovery = execute('go.interaction.worker-recovery-isolated', process.execPath,
    ['scripts/workbench/recovery-isolated.mjs'], root, 120000);
  recovery.scope = 'recovery identity, dependency graph and review gate tests using linked original source files and isolated contract stubs; not a complete Interaction build';
  if (currentGo.status === 'passed') {
    const security = execute('go.workbenchhttp.security-race', go,
      ['test', '-json', '-race', '-count=1', '-timeout', '30s'],
      path.join(backend, 'internal', 'workbenchhttp'), 90000, true);
    security.scope = 'standard library HTTP cache and bounded body tests, including streaming and declared over-limit payloads; complete Gin API routing remains unverified';
    const browserIsolation = execute('go.workbenchbrowser.security-isolated', process.platform === 'win32' ? 'python' : 'python3', ['scripts/workbench/browser-security-isolated.py'], root, 90000);
    browserIsolation.scope = 'original Agent tab lease and remote read source with isolated browser contracts; no real Chromium';
    const syntax = execute('go.workbench.source-syntax', go,
      ['run', 'scripts/workbench/go-source-syntax-gate.go', backend], root, 90000);
    syntax.scope = 'standard-library parser for Workbench-related backend Go sources; no typechecking or full build';
    const check = execute('go.worktree.real-git-isolated', go,
      ['test', '-json', '-race', '-count=1', '-timeout', '180s'],
      path.join(backend, 'internal', 'gitworktree'), 210000, true);
    check.scope = 'GO111MODULE=off; real temporary Git repositories, isolated from full backend';
    const terminal = execute('go.workbenchterminal.output-buffer-isolated', go,
      ['test', '-json', '-race', '-count=1', 'buffer.go', 'buffer_test.go', 'buffer_cursor_test.go'],
      path.join(backend, 'internal', 'workbenchterminal'), 90000, true);
    terminal.scope = 'standard-library only terminal buffer tests, not PTY integration';
    const tickets = execute('go.workbenchterminal.stream-tickets-isolated', go,
      ['test', '-json', '-race', '-count=1', 'errors.go', 'stream_ticket.go', 'stream_ticket_test.go'],
      path.join(backend, 'internal', 'workbenchterminal'), 90000, true);
    tickets.scope = 'real concurrent ticket registry checks, without Gin or real WebSocket';
    const manager = execute('go.workbenchterminal.manager-isolated', go,
      ['test', '-json', '-race', '-count=1', 'manager.go', 'manager_test.go', 'manager_isolated_test.go', 'manager_open_test.go', 'manager_real_process_test.go', 'session_revocation_test.go', 'session_read_revocation_test.go', 'session.go', 'buffer.go', 'lifecycle.go', 'export.go', 'stream_notify.go', 'session_list.go', 'process.go', 'errors.go', 'task_binding.go', 'owner_scope.go', 'owner_scope_test.go'],
      path.join(backend, 'internal', 'workbenchterminal'), 90000, true);
    manager.scope = 'real concurrent Manager lifecycle and disconnected-task retention with mock processes; not PTY integration';
    const streamCursor = execute('go.workbenchterminal.stream-cursor-isolated', go,
      ['test', '-json', '-race', '-count=1', 'stream_cursor.go', 'stream_cursor_test.go', 'errors.go'],
      path.join(backend, 'internal', 'workbenchterminal'), 90000, true);
    streamCursor.scope = 'original-source websocket cursor validation and rejection prior to ticket consumption';
  } else {
    blocked('go.workbench.source-syntax', 'Go executable unavailable');
    blocked('go.worktree.real-git-isolated', 'Go executable unavailable');
    blocked('go.workbenchterminal.output-buffer-isolated', 'Go executable unavailable');
    blocked('go.workbenchterminal.stream-tickets-isolated', 'Go executable unavailable');
    blocked('go.workbenchterminal.manager-isolated', 'Go executable unavailable');
    blocked('go.workbenchterminal.stream-cursor-isolated', 'Go executable unavailable');
  }
  if (!worktreeOnly && correctGo) {
    execute('go.workbench.targeted-tests', go, ['test', '-json', './internal/workbenchtasks', './internal/workbenchterminal', './internal/workbenchbrowser', './internal/workspace', './internal/gitworktree'], backend, 180000, true);
    const temp = fs.mkdtempSync(path.join(os.tmpdir(), 'uai-workbench-go-build-'));
    try {
      const output = path.join(temp, process.platform === 'win32' ? 'AmitiaCore.exe' : 'amitia-core');
      execute('go.backend.full-compile', go, ['build', '-o', output, './cmd/server'], backend, 240000);
    } finally {
      fs.rmSync(temp, { recursive: true, force: true });
    }
  } else {
    blocked('go.workbench.targeted-tests', '需要 Go '+declaredGo);
    blocked('go.backend.full-compile', '需要 Go '+declaredGo);
  }
  if (!worktreeOnly && checkedPnpm.status === 'passed' && hasFrontDeps) {
    execute('front.vue-typecheck', pnpm, ['run', 'typecheck'], front, 180000);
    execute('front.workbench-chunk-tests', pnpm, ['exec', 'vitest', 'run', 'src/__tests__/workbenchDocumentChunks.test.ts'], front, 180000);
  } else {
    blocked('front.vue-typecheck', '需要已安装的项目 pnpm 依赖');
    blocked('front.workbench-chunk-tests', '需要已安装的项目 pnpm 依赖');
  }
  if (!worktreeOnly && checkedPnpm.status === 'passed' && hasDesktopDeps) {
    execute('electron.typecheck', pnpm, ['run', 'typecheck'], desktop, 180000);
  } else {
    blocked('electron.typecheck', '需要已安装的项目 pnpm 依赖');
  }
  if (!worktreeOnly && checkedFlutter.status === 'passed') {
    execute('flutter.workbench.analyze', flutter, ['analyze', 'lib/features/workbench', 'test/features/workbench/workbench_document_chunks_test.dart', 'test/features/workbench/workbench_task_snapshot_test.dart'], mobile, 180000);
    execute('flutter.workbench.chunk-tests', flutter, ['test', 'test/features/workbench/workbench_document_chunks_test.dart'], mobile, 180000);
    execute('flutter.workbench.task-protocol-tests', flutter, ['test', 'test/features/workbench/workbench_task_snapshot_test.dart'], mobile, 180000);
    execute('flutter.workbench.terminal-tests', flutter, ['test', 'test/features/workbench/workbench_terminal_output_test.dart', 'test/features/workbench/workbench_terminal_snapshot_test.dart'], mobile, 180000);
  } else {
    blocked('flutter.workbench.analyze', '需要 Flutter SDK');
    blocked('flutter.workbench.chunk-tests', '需要 Flutter SDK');
    blocked('flutter.workbench.task-protocol-tests', '需要 Flutter SDK');
    blocked('flutter.workbench.terminal-tests', '需要 Flutter SDK');
  }
}
if (liveApi && process.env.AMITIA_WORKBENCH_BASE_URL && (process.env.AMITIA_WORKBENCH_AUTHORIZATION || process.env.AMITIA_WORKBENCH_COOKIE) && process.env.AMITIA_WORKBENCH_CONVERSATION_ID) {
  execute('integration.authenticated-task-read', process.execPath, ['scripts/workbench/authenticated-smoke.mjs'], root, 45000);
} else {
  blocked('integration.authenticated-task-read', '需要 --live-api 及真实本机服务地址、会话认证凭据、对话 ID；未使用模拟服务代替真实运行');
}
blocked('integration.agent-execution-and-merge', '实际 Agent 调度、终端执行、审批 UI 和合并仍须在真实服务上验收');
blocked('integration.device-validation', '仍须在 Windows ConPTY、Electron、Android Release 和 iOS 真机/构建环境验收');

const report = {
  protocolVersion: 1,
  goal: 'U-Ai Workbench coding worker isolation, checkpoint evidence and development/acceptance gate',
  toolchainRequired: { go: declaredGo },
  mode: preflightOnly ? 'preflight' : developmentOnly ? 'development-validation' : worktreeOnly ? 'isolated-worktree' : 'targeted-integration',
  liveApiRequested: liveApi,
  results,
  verified: results.filter(row => row.status === 'passed').length,
  failed: results.filter(row => row.status === 'failed').length,
  blocked: results.filter(row => row.status === 'blocked').length,
  developmentValidationPassed: !preflightOnly && results.filter(row => row.status === 'failed').length === 0 &&
    ['go.interaction.coding-allocation-isolated', 'go.interaction.worker-recovery-isolated', 'go.worktree.real-git-isolated', 'go.workbenchterminal.manager-isolated', 'go.workbenchhttp.security-race'].every(name => results.some(row => row.name === name && row.status === 'passed')),
  productionAcceptancePassed: !preflightOnly && !worktreeOnly && !developmentOnly && results.every(row => row.status === 'passed'),
};
console.log(JSON.stringify(report, null, 2));
if (process.env.AMITIA_WORKBENCH_GATE_REPORT) fs.writeFileSync(process.env.AMITIA_WORKBENCH_GATE_REPORT, JSON.stringify(report, null, 2) + '\n');
if (report.failed > 0 || (liveApi && results.some(row => row.name === 'integration.authenticated-task-read' && row.status !== 'passed')) || (!preflightOnly && !worktreeOnly && !developmentOnly && report.blocked > 0)) process.exitCode = 1;
