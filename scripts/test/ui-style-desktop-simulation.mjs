const CDP_ENDPOINT = "http://127.0.0.1:9222/json/list";
const WAIT_AFTER_SEND_MS = 24000;

const cases = [
  {
    name: "desktop-chat-input-background",
    prompt: 'Call uiagent.appearance apply with target="chat.composer.input" and properties={"backgroundColor":"#2563EB"} in page chat. Do not change other targets.',
    target: "chat.composer.input",
    property: "backgroundColor",
    expected: "rgb(37, 99, 235)",
  },
  {
    name: "desktop-chat-input-color",
    prompt: 'Call uiagent.appearance apply with target="chat.composer.input" and properties={"color":"#EA580C"} in page chat. Do not change other targets.',
    target: "chat.composer.input",
    property: "color",
    expected: "rgb(234, 88, 12)",
  },
  {
    name: "desktop-composer-radius",
    prompt: 'Use uiagent.appearance. action="apply". changes={"targets":[{"scope":{"page":"chat"},"target":"chat.composer.surface","states":["default"],"properties":{"borderRadius":24}}]}. Call it now exactly once.',
    target: "chat.composer.surface",
    property: "borderRadius",
    expected: "24px",
  },
  {
    name: "desktop-composer-add-background",
    prompt: 'Call uiagent.appearance apply with target="chat.composer.add" and properties={"backgroundColor":"#EF4444"} in page chat. Do not change other targets.',
    target: "chat.composer.add",
    property: "backgroundColor",
    expected: "rgb(239, 68, 68)",
  },
  {
    name: "desktop-shell-background",
    prompt: 'Use uiagent.appearance. action="apply". changes={"targets":[{"scope":{"page":"chat"},"target":"app.shell","states":["default"],"properties":{"backgroundColor":"#F1F5F9"}}]}. Call it now exactly once.',
    target: "app.shell",
    property: "backgroundColor",
    expected: "rgb(241, 245, 249)",
  },
  {
    name: "desktop-header-background",
    prompt: 'Call uiagent.appearance apply with target="app.header" and properties={"backgroundColor":"#1E40AF"} in page chat. Do not change other targets.',
    target: "app.header",
    property: "backgroundColor",
    expected: "rgb(30, 64, 175)",
  },
  {
    name: "desktop-sidebar-background",
    prompt: 'Call uiagent.appearance apply with target="app.sidebar" and properties={"backgroundColor":"#F5F3FF"} in page chat. Do not change other targets.',
    target: "app.sidebar",
    property: "backgroundColor",
    expected: "rgb(245, 243, 255)",
  },
  {
    name: "desktop-sidebar-item-color",
    prompt: 'Use uiagent.appearance. action="apply". changes={"targets":[{"scope":{"page":"chat"},"target":"app.sidebar.item","states":["default"],"properties":{"color":"#16A34A"}}]}. Call it now exactly once.',
    target: "app.sidebar.item",
    property: "color",
    expected: "rgb(22, 163, 74)",
  },
  {
    name: "desktop-content-background",
    prompt: 'Call uiagent.appearance apply with target="app.content" and properties={"backgroundColor":"#FEFCE8"} in page chat. Do not change other targets.',
    target: "app.content",
    property: "backgroundColor",
    expected: "rgb(254, 252, 232)",
  },
  {
    name: "desktop-page-background",
    prompt: 'Call uiagent.appearance apply with target="chat.page" and properties={"backgroundColor":"#ECFEFF"} in page chat. Do not change other targets.',
    target: "chat.page",
    property: "backgroundColor",
    expected: "rgb(236, 254, 255)",
  },
];

function sleep(milliseconds) {
  return new Promise((resolve) => setTimeout(resolve, milliseconds));
}

async function getPageSocket() {
  const pages = await fetch(CDP_ENDPOINT).then((response) => response.json());
  const page = pages.find((item) => String(item.url || "").includes("127.0.0.1:15178"));
  if (!page?.webSocketDebuggerUrl) throw new Error("desktop simulation page not found");
  return page.webSocketDebuggerUrl;
}

function connect(webSocketUrl) {
  return new Promise((resolve, reject) => {
    const socket = new WebSocket(webSocketUrl);
    const pending = new Map();
    let nextId = 0;
    socket.onopen = () => {
      resolve({
        socket,
        send(method, params = {}) {
          return new Promise((resolveCommand, rejectCommand) => {
            const id = ++nextId;
            pending.set(id, { resolve: resolveCommand, reject: rejectCommand });
            socket.send(JSON.stringify({ id, method, params }));
          });
        },
        close() {
          socket.close();
        },
      });
    };
    socket.onerror = reject;
    socket.onmessage = (event) => {
      const message = JSON.parse(event.data);
      const item = pending.get(message.id);
      if (!item) return;
      pending.delete(message.id);
      if (message.error) item.reject(new Error(message.error.message || "CDP command failed"));
      else item.resolve(message.result);
    };
  });
}

async function evaluate(client, expression) {
  const result = await client.send("Runtime.evaluate", {
    expression,
    returnByValue: true,
    awaitPromise: true,
  });
  if (result.exceptionDetails) {
    throw new Error(result.exceptionDetails.text || "Runtime evaluation failed");
  }
  return result.result?.value;
}

async function submitPrompt(client, prompt) {
  const value = JSON.stringify(prompt);
  const expression = `(() => {
    const input = document.querySelector('[data-amitia-ui="chat.composer.input"]');
    if (!input) return 'missing-input';
    input.focus();
    input.value = ${value};
    input.dispatchEvent(new InputEvent('input', { bubbles: true, inputType: 'insertText', data: input.value }));
    input.dispatchEvent(new Event('change', { bubbles: true }));
    return 'ready';
  })()`;
  const result = await evaluate(client, expression);
  if (result !== "ready") throw new Error(`prompt submit failed: ${result}`);
  await sleep(500);
  const sent = await evaluate(client, `(() => {
    const send = document.querySelector('[data-amitia-ui="chat.composer.send"]');
    if (!send || send.disabled) return 'missing-send';
    send.click();
    return 'sent';
  })()`);
  if (sent !== "sent") throw new Error(`prompt submit failed: ${sent}`);
}

async function readStyle(client, testCase) {
  const selector = JSON.stringify(`[data-amitia-ui="${testCase.target}"]`);
  const property = JSON.stringify(testCase.property);
  return await evaluate(client, `(() => {
    const element = document.querySelector(${selector});
    if (!element) return { missing: true, value: null };
    return { missing: false, value: getComputedStyle(element)[${property}] };
  })()`);
}

async function main() {
  const client = await connect(await getPageSocket());
  const results = [];
  const requested = new Set(process.argv.slice(2));
  const selectedCases = requested.size > 0
    ? cases.filter((item) => requested.has(item.name))
    : cases;
  try {
    await client.send("Runtime.enable");
    for (const testCase of selectedCases) {
      const startedAt = Date.now();
      await submitPrompt(client, testCase.prompt);
      await sleep(WAIT_AFTER_SEND_MS);
      const observed = await readStyle(client, testCase);
      const pass = !observed.missing && observed.value === testCase.expected;
      results.push({
        name: testCase.name,
        target: testCase.target,
        property: testCase.property,
        expected: testCase.expected,
        observed: observed.value,
        pass,
        durationMs: Date.now() - startedAt,
      });
      console.log(JSON.stringify(results.at(-1)));
    }
  } finally {
    client.close();
  }
  const failed = results.filter((item) => !item.pass);
  console.log(JSON.stringify({ total: results.length, passed: results.length - failed.length, failed }, null, 2));
  if (failed.length > 0) process.exitCode = 1;
}

main().catch((error) => {
  console.error(error);
  process.exit(1);
});
