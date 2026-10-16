import test from 'node:test';
import assert from 'node:assert/strict';
import http from 'node:http';
import { once } from 'node:events';
import { safeLocalEndpoint, verifyAuthenticatedWorkbench } from './authenticated-smoke.mjs';

test('local-only origin validation and forbidden credential redirection', () => {
  for (const candidate of ['https://example.com', 'https://127.0.0.1.evil.test', 'http://user:pass@localhost', 'http://localhost/path', 'http://localhost?x=1', 'file:///etc/hosts']) {
    assert.throws(() => safeLocalEndpoint(candidate));
  }
  assert.equal(safeLocalEndpoint('http://127.0.0.1:18899'), 'http://127.0.0.1:18899');
});

async function withServer(responder, fn) {
  const server = http.createServer(responder);
  server.listen(0, '127.0.0.1');
  await once(server, 'listening');
  try { await fn(`http://127.0.0.1:${server.address().port}`); } finally { await new Promise(resolve => server.close(resolve)); }
}

test('authenticated smoke validates unauthorized, bad scope and valid task projection', async () => {
  let credentialCount = 0;
  await withServer((req, res) => {
    const url = new URL(req.url, 'http://localhost');
    res.setHeader('content-type', 'application/json');
    if (req.headers.authorization !== 'Bearer fixture') { res.writeHead(403).end('{}'); return; }
    credentialCount++;
    if (url.searchParams.get('conversationId') === ' ') { res.writeHead(400).end('{}'); return; }
    res.setHeader('cache-control', 'no-store');
    res.writeHead(200).end(JSON.stringify({ protocolVersion: 1, conversationId: 'conversation-1', tasks: [] }));
  }, async baseUrl => {
    const result = await verifyAuthenticatedWorkbench({ baseUrl, authorization: 'Bearer fixture', conversationId: 'conversation-1' });
    assert.equal(result.status, 'passed');
    assert.equal(result.taskCount, 0);
  });
  assert.equal(credentialCount, 2);
});

test('smoke refuses public route, wrong scope and wrong projection', async () => {
  await withServer((req, res) => { res.setHeader('content-type', 'application/json'); res.writeHead(200).end(JSON.stringify({ protocolVersion: 1, conversationId: 'conversation-1', tasks: [] })); }, async baseUrl => {
    await assert.rejects(verifyAuthenticatedWorkbench({ baseUrl, authorization: 'Bearer fixture', conversationId: 'conversation-1' }), /anonymous/);
  });
  await withServer((req, res) => {
    res.setHeader('content-type', 'application/json');
    if (!req.headers.authorization) { res.writeHead(403).end('{}'); return; }
    if (req.url.includes('%20')) { res.writeHead(400).end('{}'); return; }
    res.writeHead(200).end(JSON.stringify({ protocolVersion: 1, conversationId: 'other', tasks: [] }));
  }, async baseUrl => {
    await assert.rejects(verifyAuthenticatedWorkbench({ baseUrl, authorization: 'Bearer fixture', conversationId: 'conversation-1' }), /contract/);
  });
});
