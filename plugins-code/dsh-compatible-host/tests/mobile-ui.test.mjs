import assert from "node:assert/strict";
import test from "node:test";
import { JSDOM } from "jsdom";

function fixture() {
  return {
    targetDshVersion: "0.2.2-rc.2",
    packageCount: 1,
    enabledCount: 1,
    compatibility: { implemented: 19 },
    packages: [
      {
        pluginId: "demo/dsh-plugin",
        name: "Demo Plugin",
        version: "1.2.3",
        enabled: true,
        entry: "index.js",
        clientEntry: "client.js",
        mappedContributionCount: 4,
      },
    ],
  };
}

function installDom() {
  const dom = new JSDOM("<!doctype html><html><body><div id=\"root\"></div></body></html>", {
    url: "http://localhost/",
    pretendToBeVisual: true,
  });
  const previous = {
    window: globalThis.window,
    document: globalThis.document,
    Element: globalThis.Element,
    requestAnimationFrame: globalThis.requestAnimationFrame,
  };
  globalThis.window = dom.window;
  globalThis.document = dom.window.document;
  globalThis.Element = dom.window.Element;
  globalThis.requestAnimationFrame = (callback) => {
    callback(0);
    return 1;
  };
  return {
    dom,
    restore() {
      globalThis.window = previous.window;
      globalThis.document = previous.document;
      globalThis.Element = previous.Element;
      globalThis.requestAnimationFrame = previous.requestAnimationFrame;
      dom.window.close();
    },
  };
}

async function flush() {
  await new Promise((resolve) => setTimeout(resolve, 0));
  await new Promise((resolve) => setTimeout(resolve, 0));
}

test("mobile management page invokes the container command through AmitiaHost", async () => {
  const environment = installDom();
  try {
    const calls = [];
    let status = fixture();
    let resizeCount = 0;
    window.AmitiaHost = {
      async invokeAction(actionId, input) {
        calls.push({ actionId, input });
        assert.equal(actionId, "command");
        if (input.action === "status") {
          return { ok: true, output: structuredClone(status) };
        }
        if (input.action === "install") {
          status = {
            ...status,
            packageCount: 2,
            packages: [
              ...status.packages,
              {
                pluginId: "demo/installed",
                name: "Installed Plugin",
                version: "2.0.0",
                enabled: false,
                entry: "index.js",
                clientEntry: "",
                mappedContributionCount: 0,
              },
            ],
          };
          return {
            ok: true,
            output: { package: status.packages.at(-1) },
          };
        }
        return {};
      },
      async resize(height) {
        resizeCount += 1;
        assert.ok(height > 0);
        return { height };
      },
      subscribeContext() {
        return () => {};
      },
    };

    const { mount } = await import("../src/mobile-ui/index.mjs");
    const root = document.getElementById("root");
    const mounted = await mount(root, { platform: "android", host: "mobile" });
    assert.match(root.textContent, /DeepSeek Harness 插件/);
    assert.match(root.textContent, /Demo Plugin/);
    assert.match(root.textContent, /已启用/);
    assert.equal(calls[0].input.action, "status");
    assert.ok(resizeCount > 0);

    const input = root.querySelector("input");
    input.value = "@deepseek-ai/dsh-example@0.2.2-rc.2";
    const installButton = Array.from(root.querySelectorAll("button")).find((item) => item.textContent === "安装 DSH 插件");
    assert.ok(installButton);
    installButton.click();
    await flush();

    assert.ok(calls.some((call) => call.input.action === "install" && call.input.spec === "@deepseek-ai/dsh-example@0.2.2-rc.2"));
    assert.match(root.textContent, /Installed Plugin/);
    assert.equal(root.querySelectorAll(".dsh-mobile__card").length, 2);
    mounted.dispose();
    assert.equal(root.childElementCount, 0);
  } finally {
    environment.restore();
  }
});

test("mobile management page preserves a pending package spec across remounts", async () => {
  const environment = installDom();
  try {
    window.AmitiaHost = {
      async invokeAction(actionId, input) {
        assert.equal(actionId, "command");
        if (input.action === "status") return fixture();
        return {};
      },
      async resize(height) {
        return { height };
      },
      subscribeContext() {
        return () => {};
      },
    };

    const { mount } = await import("../src/mobile-ui/index.mjs");
    const root = document.getElementById("root");
    const first = await mount(root, { platform: "android", host: "mobile" });
    const input = root.querySelector("input");
    input.value = "@deepseek-ai/dsh-tool-todo@0.2.2-rc.2";
    input.dispatchEvent(new window.Event("input", { bubbles: true }));
    first.dispose();

    const second = await mount(root, { platform: "android", host: "mobile" });
    assert.equal(
      root.querySelector("input")?.value,
      "@deepseek-ai/dsh-tool-todo@0.2.2-rc.2",
    );
    second.dispose();
  } finally {
    environment.restore();
  }
});

test("mobile management page unwraps successful bridge responses with a null error", async () => {
  const environment = installDom();
  try {
    const responses = [
      { ok: true, output: structuredClone(fixture()) },
      { status: "succeeded", data: structuredClone(fixture()), error: null },
      {
        invocationId: "invoke-status",
        status: "success",
        content: [],
        structured: structuredClone(fixture()),
        error: null,
      },
    ];
    for (const response of responses) {
      window.AmitiaHost = {
        async invokeAction(actionId, input) {
          assert.equal(actionId, "command");
          assert.equal(input.action, "status");
          return structuredClone(response);
        },
        async resize(height) {
          return { height };
        },
        subscribeContext() {
          return () => {};
        },
      };
      const { mount } = await import("../src/mobile-ui/index.mjs");
      const root = document.getElementById("root");
      const mounted = await mount(root, { platform: "android", host: "mobile" });
      assert.match(root.textContent, /Demo Plugin/);
      assert.equal(root.querySelector(".dsh-mobile__error"), null);
      mounted.dispose();
    }
  } finally {
    environment.restore();
  }
});

test("mobile management page ignores a standalone null error and preserves real bridge errors", async () => {
  const environment = installDom();
  try {
    window.AmitiaHost = {
      async invokeAction(actionId, input) {
        assert.equal(actionId, "command");
        assert.equal(input.action, "status");
        return { error: null };
      },
      async resize(height) {
        return { height };
      },
      subscribeContext() {
        return () => {};
      },
    };
    const { mount } = await import("../src/mobile-ui/index.mjs");
    const root = document.getElementById("root");
    const mounted = await mount(root, { platform: "android", host: "mobile" });
    assert.equal(root.querySelector(".dsh-mobile__error"), null);
    mounted.dispose();

    window.AmitiaHost.invokeAction = async () => ({
      ok: false,
      error: {
        code: "DSH_COMMAND_FAILED",
        message: "原始命令错误",
        cause: { message: "底层错误" },
      },
    });
    const failed = await mount(root, { platform: "android", host: "mobile" });
    assert.match(root.querySelector(".dsh-mobile__error").textContent, /DSH_COMMAND_FAILED/);
    assert.match(root.querySelector(".dsh-mobile__error").textContent, /原始命令错误/);
    assert.match(root.querySelector(".dsh-mobile__error").textContent, /底层错误/);
    failed.dispose();
  } finally {
    environment.restore();
  }
});

test("mobile management page surfaces activation state and runs enable, disable and remove", async () => {
  const environment = installDom();
  try {
    const calls = [];
    let status = {
      targetDshVersion: "0.2.2-rc.2",
      packageCount: 1,
      enabledCount: 1,
      compatibility: { implemented: 19 },
      packages: [
        {
          pluginId: "demo/broken",
          name: "Broken Plugin",
          version: "3.0.0",
          enabled: true,
          active: false,
          activationError: "Error: 启动失败: connect ECONNREFUSED 127.0.0.1:9",
          entry: "index.js",
          clientEntry: "",
          mappedContributionCount: 0,
        },
      ],
    };
    window.confirm = () => true;
    window.AmitiaHost = {
      async invokeAction(actionId, input) {
        calls.push({ actionId, input });
        assert.equal(actionId, "command");
        if (input.action === "status") return { ok: true, output: structuredClone(status) };
        if (input.action === "set-enabled") {
          status = {
            ...status,
            packages: status.packages.map((item) =>
              item.pluginId === input.pluginId ? { ...item, enabled: input.enabled } : item,
            ),
          };
          return { ok: true, output: { package: status.packages[0] } };
        }
        if (input.action === "remove") {
          status = { ...status, packageCount: 0, enabledCount: 0, packages: [] };
          return { ok: true, output: {} };
        }
        return {};
      },
      async resize(height) {
        return { height };
      },
      subscribeContext() {
        return () => {};
      },
    };

    const { mount } = await import("../src/mobile-ui/index.mjs");
    const root = document.getElementById("root");
    const mounted = await mount(root, { platform: "android", host: "mobile" });
    assert.match(root.textContent, /已启用·未运行/);
    assert.match(root.textContent, /ECONNREFUSED 127\.0\.0\.1:9/);

    const disable = Array.from(root.querySelectorAll("button")).find((item) => item.textContent === "停用");
    assert.ok(disable);
    disable.click();
    await flush();
    assert.ok(calls.some((call) => call.input.action === "set-enabled" && call.input.enabled === false));
    assert.match(root.textContent, /已停用/);

    const enable = Array.from(root.querySelectorAll("button")).find((item) => item.textContent === "启用");
    assert.ok(enable);
    enable.click();
    await flush();
    assert.ok(calls.some((call) => call.input.action === "set-enabled" && call.input.enabled === true));

    const remove = Array.from(root.querySelectorAll("button")).find((item) => item.textContent === "移除");
    assert.ok(remove);
    remove.click();
    await flush();
    assert.ok(calls.some((call) => call.input.action === "remove" && call.input.pluginId === "demo/broken"));
    assert.match(root.textContent, /尚未安装 DSH 子插件/);
    mounted.dispose();
  } finally {
    environment.restore();
  }
});
