import { appendFileSync } from 'node:fs';
import { dirname, resolve } from 'node:path';
import { fileURLToPath } from 'node:url';

const here = dirname(fileURLToPath(import.meta.url));
const defaultWorkspace = resolve(here, '../../../testplugins/mock-amitiax-game-plugin');
const baseUrl = (process.env.GAMEHOST_BASE_URL || 'http://127.0.0.1:18899/api').replace(/\/$/, '');
const extensionId = process.env.GAMEHOST_DEV_EXTENSION_ID || 'mock-developer/mock-amitiax-game-plugin';
const workspacePath = resolve(process.env.GAMEHOST_DEV_WORKSPACE_PATH || defaultWorkspace);
const manifestPath = resolve(process.env.GAMEHOST_DEV_MANIFEST_PATH || resolve(workspacePath, 'amitia-extension.json'));
const username = process.env.GAMEHOST_BOOTSTRAP_USERNAME || 'gamehost-e2e-admin';
const password = process.env.GAMEHOST_BOOTSTRAP_PASSWORD || 'GameHost-E2E-Admin-2026!';

function unwrap(body) {
  if (body && typeof body === 'object' && Object.prototype.hasOwnProperty.call(body, 'data')) return body.data;
  return body;
}

async function request(path, init = {}, token = '') {
  const headers = new Headers(init.headers || {});
  if (token) headers.set('Authorization', `Bearer ${token}`);
  const response = await fetch(baseUrl + path, { ...init, headers });
  const text = await response.text();
  let body = null;
  if (text.trim()) {
    try { body = JSON.parse(text); } catch { body = { raw: text }; }
  }
  if (!response.ok) {
    const message = body?.msg || body?.message || body?.error || body?.raw || `HTTP ${response.status}`;
    const error = new Error(`${path}: ${message}`);
    error.status = response.status;
    error.body = body;
    throw error;
  }
  return unwrap(body);
}

async function waitForBackend() {
  const deadline = Date.now() + 120_000;
  let lastError;
  while (Date.now() < deadline) {
    try {
      await request('/public/health', { method: 'GET' });
      return;
    } catch (error) {
      lastError = error;
      await new Promise(resolve => setTimeout(resolve, 1000));
    }
  }
  throw lastError || new Error('backend health timeout');
}

async function getAdminToken() {
  const payload = JSON.stringify({ username, password });
  const headers = { 'Content-Type': 'application/json', 'User-Agent': 'gamehost-external-e2e' };
  try {
    const setup = await request('/public/auth/setup', { method: 'POST', headers, body: payload });
    if (!setup?.accessToken) throw new Error('auth setup returned no accessToken');
    return setup.accessToken;
  } catch (error) {
    const code = error?.body?.data?.errorCode || error?.body?.errorCode || '';
    const msg = String(error?.body?.msg || error?.message || '');
    if (code !== 'auth.admin_exists' && !msg.includes('管理员已存在')) throw error;
    const login = await request('/public/auth/login', { method: 'POST', headers, body: payload });
    if (!login?.accessToken) throw new Error('auth login returned no accessToken');
    return login.accessToken;
  }
}

async function ensureWorkspace(token) {
  try {
    const created = await request('/extensions/dev-mode/workspaces', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({
        extensionId,
        path: workspacePath,
        manifestPath,
        watchEnabled: false,
        autoReload: false,
      }),
    }, token);
    return created.workspaceId;
  } catch (error) {
    if (error?.status !== 409) throw error;
    const listed = await request('/extensions/dev-mode/workspaces', { method: 'GET' }, token);
    const workspaces = Array.isArray(listed?.workspaces) ? listed.workspaces : [];
    const existing = workspaces.find(item => item?.extensionId === extensionId && item?.path === workspacePath)
      || workspaces.find(item => item?.extensionId === extensionId);
    if (!existing?.workspaceId) throw error;
    return existing.workspaceId;
  }
}

async function main() {
  await waitForBackend();
  const token = await getAdminToken();
  const workspaceId = await ensureWorkspace(token);
  await request(`/extensions/dev-mode/workspaces/${encodeURIComponent(workspaceId)}/trust`, { method: 'POST' }, token);
  const session = await request(`/extensions/dev-mode/workspaces/${encodeURIComponent(workspaceId)}/sessions`, {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({ deviceId: 'gamehost-ci', userAgent: 'gamehost-external-e2e' }),
  }, token);
  if (!session?.sessionId) throw new Error('developer session endpoint returned no sessionId');

  const values = {
    GAMEHOST_AUTH_TOKEN: token,
    GAMEHOST_DEVELOPER_SESSION_ID: session.sessionId,
    GAMEHOST_DEV_WORKSPACE_ID: workspaceId,
  };

  const githubEnv = process.env.GITHUB_ENV;
  if (githubEnv) {
    for (const [key, value] of Object.entries(values)) appendFileSync(githubEnv, `${key}=${value}\n`);
  }
  process.stdout.write(`${JSON.stringify({ ...values, GAMEHOST_AUTH_TOKEN: '<redacted>' })}\n`);
}

main().catch(error => {
  console.error(error?.stack || String(error));
  process.exit(1);
});
