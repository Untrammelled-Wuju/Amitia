import { closeSync, existsSync, mkdirSync, openSync, readFileSync, rmSync, writeFileSync } from 'node:fs';
import { dirname, resolve } from 'node:path';
import { spawn, spawnSync } from 'node:child_process';

const required = name => {
  const value = (process.env[name] || '').trim();
  if (!value) throw new Error(`${name} is required`);
  return value;
};

const backendBin = resolve(required('GAMEHOST_BACKEND_BIN'));
const pidFile = resolve(required('GAMEHOST_BACKEND_PID_FILE'));
const logFile = resolve(required('GAMEHOST_BACKEND_LOG'));
const backendCwd = resolve(process.env.GAMEHOST_BACKEND_CWD || process.cwd());
const serverRoot = (process.env.GAMEHOST_SERVER_ROOT || 'http://127.0.0.1:18899').replace(/\/$/, '');
const healthUrl = `${serverRoot}/api/public/health`;
const action = process.argv[2] || 'restart';

const sleep = ms => new Promise(resolvePromise => setTimeout(resolvePromise, ms));

function readPid() {
  if (!existsSync(pidFile)) return null;
  const text = readFileSync(pidFile, 'utf8').trim();
  if (!/^\d+$/.test(text)) return null;
  const pid = Number(text);
  return Number.isSafeInteger(pid) && pid > 0 ? pid : null;
}

function isRunning(pid = readPid()) {
  if (!pid) return false;
  try {
    process.kill(pid, 0);
    return true;
  } catch {
    return false;
  }
}

function isOwnedBackendProcess(pid) {
  if (!pid || !isRunning(pid)) return false;
  if (process.platform === 'win32') {
    const systemRoot = (process.env.SystemRoot || '').trim();
    if (!systemRoot) return false;
    const powershell = resolve(systemRoot, 'System32', 'WindowsPowerShell', 'v1.0', 'powershell.exe');
    const escapedPid = String(pid);
    const command = `$p = Get-CimInstance Win32_Process -Filter \"ProcessId = ${escapedPid}\" -ErrorAction SilentlyContinue; if ($p) { [Console]::Out.Write($p.ExecutablePath) }`;
    const result = spawnSync(powershell, ['-NoLogo', '-NoProfile', '-NonInteractive', '-Command', command], {
      encoding: 'utf8',
      windowsHide: true,
      stdio: ['ignore', 'pipe', 'ignore'],
    });
    if (result.error || result.status !== 0) return false;
    const executable = resolve(String(result.stdout || '').trim()).toLowerCase();
    return executable === backendBin.toLowerCase();
  }

  const ps = ['/bin/ps', '/usr/bin/ps'].find(existsSync);
  if (!ps) return false;
  const result = spawnSync(ps, ['-p', String(pid), '-o', 'command='], {
    encoding: 'utf8',
    stdio: ['ignore', 'pipe', 'ignore'],
  });
  if (result.error || result.status !== 0) return false;
  const command = String(result.stdout || '').trim();
  return command === backendBin || command.startsWith(`${backendBin} `);
}

function assertOwnedBackendProcess(pid) {
  if (!isOwnedBackendProcess(pid)) {
    throw new Error(`refusing to manage PID ${pid}: it is not the configured GameHost E2E backend ${backendBin}`);
  }
}

async function stopBackend() {
  const pid = readPid();
  if (!pid || !isRunning(pid)) {
    rmSync(pidFile, { force: true });
    return;
  }
  assertOwnedBackendProcess(pid);

  // Deliberately terminate only the backend process. Do not recursively kill
  // children: the lifecycle matrix must prove GameHost recovery eliminates or
  // reclaims orphaned plugin runtimes rather than letting the test runner hide
  // residue by killing the entire process tree.
  try {
    if (process.platform === 'win32') {
      const systemRoot = (process.env.SystemRoot || '').trim();
      if (!systemRoot) throw new Error('SystemRoot is required to terminate the Windows E2E backend');
      const taskkill = resolve(systemRoot, 'System32', 'taskkill.exe');
      const result = spawnSync(taskkill, ['/PID', String(pid), '/F'], { windowsHide: true, stdio: 'ignore' });
      if (result.error) throw result.error;
      if (result.status !== 0 && isRunning(pid)) {
        throw new Error(`taskkill failed for backend process ${pid} with exit code ${result.status}`);
      }
    } else {
      process.kill(pid, 'SIGKILL');
    }
  } catch (error) {
    if (isRunning(pid)) throw error;
  }

  for (let i = 0; i < 100; i++) {
    if (!isRunning(pid)) break;
    await sleep(100);
  }
  if (isRunning(pid)) throw new Error(`backend process ${pid} did not terminate`);
  rmSync(pidFile, { force: true });
}

async function waitHealthy(pid) {
  let lastError;
  for (let i = 0; i < 180; i++) {
    if (!isRunning(pid)) {
      const tail = existsSync(logFile)
        ? readFileSync(logFile, 'utf8').split(/\r?\n/).slice(-200).join('\n')
        : '<no log>';
      throw new Error(`GameHost backend exited before becoming healthy\n${tail}`);
    }
    try {
      const response = await fetch(healthUrl, { signal: AbortSignal.timeout(2000) });
      if (response.ok) return;
      lastError = new Error(`health returned HTTP ${response.status}`);
    } catch (error) {
      lastError = error;
    }
    await sleep(1000);
  }
  const tail = existsSync(logFile)
    ? readFileSync(logFile, 'utf8').split(/\r?\n/).slice(-200).join('\n')
    : '<no log>';
  throw new Error(`GameHost backend did not become healthy at ${healthUrl}: ${lastError || 'timeout'}\n${tail}`);
}

async function startBackend() {
  if (!existsSync(backendBin)) throw new Error(`backend binary does not exist: ${backendBin}`);
  mkdirSync(dirname(pidFile), { recursive: true });
  mkdirSync(dirname(logFile), { recursive: true });

  const logFd = openSync(logFile, 'a');
  let child;
  try {
    child = spawn(backendBin, ['--runtime-profile=local'], {
      cwd: backendCwd,
      env: process.env,
      detached: true,
      windowsHide: true,
      stdio: ['ignore', logFd, logFd],
    });
    child.unref();
  } finally {
    closeSync(logFd);
  }
  if (!child?.pid) throw new Error('failed to start GameHost backend');
  writeFileSync(pidFile, `${child.pid}\n`, 'utf8');
  await waitHealthy(child.pid);
}

async function main() {
  switch (action) {
    case 'start': {
      const pid = readPid();
      if (pid && isRunning(pid)) {
        assertOwnedBackendProcess(pid);
        await waitHealthy(pid);
      } else {
        rmSync(pidFile, { force: true });
        await startBackend();
      }
      break;
    }
    case 'restart':
      await stopBackend();
      await startBackend();
      break;
    case 'stop':
      await stopBackend();
      break;
    case 'status': {
      const pid = readPid();
      if (!pid || !isRunning(pid) || !isOwnedBackendProcess(pid)) process.exitCode = 1;
      else process.stdout.write(`running ${pid}\n`);
      break;
    }
    default:
      throw new Error('usage: restart-backend.mjs {start|restart|stop|status}');
  }
}

main().catch(error => {
  console.error(error?.stack || String(error));
  process.exit(1);
});
