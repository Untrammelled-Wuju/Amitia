import { fileURLToPath } from 'node:url';

export function safeLocalEndpoint(raw) {
  const parsed = new URL(raw);
  if (!['http:', 'https:'].includes(parsed.protocol) || !['127.0.0.1', 'localhost', '[::1]'].includes(parsed.hostname) || parsed.username || parsed.password || parsed.search || parsed.hash || (parsed.pathname !== '/' && parsed.pathname !== '')) {
    throw new Error('only a local HTTP(S) service origin is allowed');
  }
  return parsed.origin;
}

async function request(base, suffix, headers = {}) {
  const response = await fetch(`${base}${suffix}`, { method: 'GET', headers: { accept: 'application/json', ...headers }, redirect: 'manual', signal: AbortSignal.timeout(8000) });
  const raw = await response.text();
  let payload;
  try { payload = JSON.parse(raw); } catch { payload = null; }
  return { status: response.status, payload, cacheControl: response.headers.get('cache-control') || '' };
}

export async function verifyAuthenticatedWorkbench({ baseUrl, authorization, cookie, conversationId }) {
  const base = safeLocalEndpoint(baseUrl);
  if ((!authorization && !cookie) || !/^[A-Za-z0-9_:.-]{1,160}$/.test(conversationId || '')) throw new Error('an authenticated session and valid conversationId are required');
  if (authorization?.includes('\n') || cookie?.includes('\n') || authorization?.includes('\r') || cookie?.includes('\r')) throw new Error('invalid credential header');
  const suffix = `/api/workbench/tasks?conversationId=${encodeURIComponent(conversationId)}`;
  const anonymous = await request(base, suffix);
  if (![401, 403].includes(anonymous.status)) throw new Error(`anonymous task endpoint returned ${anonymous.status}, expected 401 or 403`);
  const headers = { ...(authorization ? { authorization } : {}), ...(cookie ? { cookie } : {}) };
  const invalid = await request(base, '/api/workbench/tasks?conversationId=%20', headers);
  if (invalid.status !== 400) throw new Error(`authenticated bad conversation returned ${invalid.status}, expected 400`);
  const valid = await request(base, suffix, headers);
  if (valid.status !== 200 || !valid.payload || valid.payload.protocolVersion !== 1 || valid.payload.conversationId !== conversationId || !Array.isArray(valid.payload.tasks) || !/no-store/i.test(valid.cacheControl)) {
    throw new Error(`authenticated tasks contract failed, HTTP ${valid.status}`);
  }
  return { status: 'passed', anonymousStatus: anonymous.status, invalidScopeStatus: invalid.status, authenticatedStatus: valid.status, protocolVersion: valid.payload.protocolVersion, conversationId, taskCount: valid.payload.tasks.length };
}

if (process.argv[1] && fileURLToPath(import.meta.url) === process.argv[1]) {
  try {
    const result = await verifyAuthenticatedWorkbench({ baseUrl: process.env.AMITIA_WORKBENCH_BASE_URL || '', authorization: process.env.AMITIA_WORKBENCH_AUTHORIZATION || '', cookie: process.env.AMITIA_WORKBENCH_COOKIE || '', conversationId: process.env.AMITIA_WORKBENCH_CONVERSATION_ID || '' });
    process.stdout.write(JSON.stringify(result) + '\n');
  } catch (err) {
    process.stderr.write(`workbench authenticated smoke blocked/failed: ${err.message}\n`);
    process.exitCode = 1;
  }
}
