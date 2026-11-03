import assert from "node:assert/strict";
import { pathToFileURL } from "node:url";
import { test } from "node:test";

async function loadExtension(relativePath) {
  let extension = null;
  globalThis.defineExtension = (value) => {
    extension = value;
  };
  const url = pathToFileURL(new URL(relativePath, import.meta.url).pathname);
  url.search = `?test=${Date.now()}-${Math.random()}`;
  await import(url.href);
  delete globalThis.defineExtension;
  if (!extension) throw new Error("extension did not register");
  return extension;
}

function createHarness() {
  const state = new Map();
  const versions = new Map();
  let command = null;
  const host = {
    async call(method, params) {
      if (method === "host.state.get") {
        if (!state.has(params.key)) return { found: false, version: versions.get(params.key) || 0 };
        return { found: true, value: state.get(params.key), version: versions.get(params.key) || 1 };
      }
      if (method === "host.state.cas") {
        const current = versions.get(params.key) || 0;
        if (current !== params.expectedVersion) return { swapped: false, version: current };
        state.set(params.key, params.newValue);
        versions.set(params.key, current + 1);
        return { swapped: true, version: current + 1 };
      }
      if (method === "host.secret.get") {
        return { ref: params.ref, value: `token-for:${params.ref}` };
      }
      throw new Error(`unexpected host method ${method}`);
    },
  };
  return {
    host,
    get command() {
      return command;
    },
    context: {
      host,
      log: { info() {} },
      handlers: {
        bindTool(name, handler) {
          assert.equal(name, "command");
          command = handler;
        },
      },
    },
  };
}

function responseJSON(value) {
  return {
    ok: true,
    status: 200,
    async text() {
      return JSON.stringify(value);
    },
  };
}

test("email plugin configures identity, resolves SecretRef and applies signature", async () => {
  const calls = [];
  globalThis.fetch = async (url, init) => {
    calls.push({ url: String(url), init });
    return responseJSON({ id: "message-1" });
  };
  const extension = await loadExtension("../email/src/index.mjs");
  const harness = createHarness();
  await extension.activate(harness.context);
  const configured = await harness.command({
    action: "configure",
    config: {
      agentId: "agent-1",
      provider: { type: "gmail", address: "agent@example.com", signature: "Amitia Agent" },
      credentialRef: "secret:mail/token",
    },
  });
  assert.equal(configured.configured, true);
  const sent = await harness.command({
    action: "execute",
    input: { operation: "message.send", to: "user@example.com", subject: "Hello", body: "Body" },
  });
  assert.equal(sent.id, "message-1");
  const body = JSON.parse(calls[0].init.body);
  const raw = Buffer.from(body.raw.replace(/-/g, "+").replace(/_/g, "/"), "base64").toString("utf8");
  assert.match(raw, /Amitia Agent/);
});

test("calendar plugin lists Google events", async () => {
  globalThis.fetch = async () => responseJSON({ items: [{ id: "event-1" }] });
  const extension = await loadExtension("../calendar/src/index.mjs");
  const harness = createHarness();
  await extension.activate(harness.context);
  await harness.command({
    action: "configure",
    config: {
      agentId: "agent-2",
      provider: { type: "google", calendarId: "primary" },
      credentialRef: "secret:calendar/token",
    },
  });
  const result = await harness.command({
    action: "execute",
    input: { operation: "events.list", limit: 5 },
  });
  assert.equal(result.items[0].id, "event-1");
});

test("phone plugin sends Twilio SMS through configured number", async () => {
  const calls = [];
  globalThis.fetch = async (url, init) => {
    calls.push({ url: String(url), init });
    return responseJSON({ sid: "SMS-1" });
  };
  const extension = await loadExtension("../phone/src/index.mjs");
  const harness = createHarness();
  await extension.activate(harness.context);
  await harness.command({
    action: "configure",
    config: {
      agentId: "agent-3",
      provider: { type: "twilio", accountSid: "AC123", phoneNumber: "+15550000000" },
      credentialRef: "secret:phone/token",
    },
  });
  const result = await harness.command({
    action: "execute",
    input: { operation: "sms.send", to: "+15551111111", body: "hello" },
  });
  assert.equal(result.sid, "SMS-1");
  assert.match(calls[0].url, /Accounts\/AC123\/Messages\.json/);
  assert.match(calls[0].init.authorization || calls[0].init.headers.Authorization, /^Basic /);
});

test("wallet plugin blocks transfers until explicitly enabled and approved", async () => {
  globalThis.fetch = async () => responseJSON({ ok: true });
  const extension = await loadExtension("../wallet/src/index.mjs");
  const harness = createHarness();
  await extension.activate(harness.context);
  await harness.command({
    action: "configure",
    config: {
      agentId: "agent-4",
      provider: { type: "generic", baseUrl: "https://wallet.example.test", accountId: "account-1" },
      credentialRef: "secret:wallet/token",
    },
  });
  await harness.command({ action: "enable" });
  await assert.rejects(
    harness.command({
      action: "execute",
      input: { operation: "payment.execute", amount: 10, payee: "merchant" },
    }),
    /默认关闭|审批令牌/,
  );
});

test("browser profile plugin lists CDP targets", async () => {
  globalThis.fetch = async () => responseJSON([{ id: "page-1", type: "page" }]);
  const extension = await loadExtension("../browser-profile/src/index.mjs");
  const harness = createHarness();
  await extension.activate(harness.context);
  await harness.command({
    action: "configure",
    config: {
      agentId: "agent-5",
      provider: { type: "cdp", baseUrl: "http://127.0.0.1:9222", profileId: "agent-profile" },
    },
  });
  const pages = await harness.command({
    action: "execute",
    input: { operation: "pages.list" },
  });
  assert.equal(pages[0].id, "page-1");
});
