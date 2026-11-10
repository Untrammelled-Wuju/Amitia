import { flushPromises, mount } from "@vue/test-utils";
import ElementPlus from "element-plus";
import { createPinia } from "pinia";
import { beforeEach, describe, expect, it, vi } from "vitest";

const fetchSchemaDocument = vi.fn();
const post = vi.fn();
const del = vi.fn();

vi.mock("@/api/extension", () => ({
  fetchSchemaDocument: (...args: unknown[]) => fetchSchemaDocument(...args),
}));

vi.mock("@/composables/useApi", () => ({
  apiClient: {
    post: (...args: unknown[]) => post(...args),
    delete: (...args: unknown[]) => del(...args),
  },
}));

import SchemaUIRenderer from "../components/extension/SchemaUIRenderer.vue";

const EMPTY_TEXT = "管理端尚未配置插件分类";
const UPSTREAM_BODY = '{"code":500,"msg":"读取插件市场分类: dial tcp 127.0.0.1:3306: connect: connection refused"}';
const UPSTREAM_MESSAGE = `upstream HTTP 500 Internal Server Error: ${UPSTREAM_BODY}`;

const contribution = {
  extensionId: "com.amitia/plugin-market",
  contributionId: "plugin-market-desktop-page",
  moduleId: "plugin-market",
  generation: 1,
  contractVersion: 1,
  slotId: "main",
  permissions: ["network.request"],
  runtimeReady: true,
  enabled: true,
  effective: true,
  sandbox: "declarative",
} as never;

function catalogSchema() {
  return {
    schemaVersion: "schema-ui/1",
    locale: { current: "zh-CN" },
    theme: { mode: "light" },
    lifecycle: {
      onMount: [
        {
          action_id: "command",
          target: "tool",
          input: { action: "catalog" },
          statePath: "market",
        },
      ],
    },
    children: [
      {
        id: "plugin-market-categories",
        type: "gallery",
        props: { items: [], emptyText: EMPTY_TEXT },
        bindings: [{ path: "market.categories", source: "state", sourcePath: "market.categories" }],
      },
    ],
  };
}

function actionSchema() {
  return {
    schemaVersion: "schema-ui/1",
    locale: { current: "zh-CN" },
    theme: { mode: "light" },
    children: [
      {
        id: "refresh-catalog",
        type: "button",
        props: { label: "刷新目录" },
        actions: [{ action_id: "command", target: "tool", input: { action: "catalog" } }],
      },
    ],
  };
}

function bridgeEnvelope(code: string, message: string, detail?: unknown) {
  return { data: { ok: false, error: { code, message, detail } } };
}

function sessionResponse() {
  return {
    data: {
      sessionId: "session-1",
      origin: "amitia-host",
      contractVersion: 1,
      generation: 1,
      token: "token-1",
    },
  };
}

async function settle(times = 6) {
  for (let index = 0; index < times; index += 1) {
    await flushPromises();
  }
}

async function mountRenderer(schema: unknown) {
  fetchSchemaDocument.mockResolvedValue(schema);
  const wrapper = mount(SchemaUIRenderer, {
    props: {
      contribution,
      context: { platform: "desktop" },
      slotId: "main",
    },
    global: { plugins: [createPinia(), ElementPlus] },
  });
  await settle();
  return wrapper;
}

function failureCode(wrapper: Awaited<ReturnType<typeof mountRenderer>>) {
  const panel = wrapper.find("[data-schema-ui-error]");
  return panel.exists() ? panel.attributes("data-schema-ui-error") : "";
}

function failureRaw(wrapper: Awaited<ReturnType<typeof mountRenderer>>) {
  const raw = wrapper.find("[data-schema-ui-error-raw]");
  return raw.exists() ? raw.text() : "";
}

describe("SchemaUI 失败展示", () => {
  beforeEach(() => {
    fetchSchemaDocument.mockReset();
    post.mockReset();
    del.mockReset();
    del.mockResolvedValue({ data: {} });
  });

  it("上游 5xx 时展示 upstream_error 与完整原始错误，不渲染空数据态", async () => {
    post.mockImplementation((url: string) => {
      if (url === "/api/extensions/ui/sessions") return Promise.resolve(sessionResponse());
      return Promise.resolve(bridgeEnvelope("upstream_error", UPSTREAM_MESSAGE, UPSTREAM_BODY));
    });
    const wrapper = await mountRenderer(catalogSchema());
    try {
      expect(failureCode(wrapper)).toBe("upstream_error");
      const raw = failureRaw(wrapper);
      expect(raw).toContain("读取插件市场分类");
      expect(raw).toContain("dial tcp 127.0.0.1:3306: connect: connection refused");
      expect(raw).toContain("upstream HTTP 500");
      expect(wrapper.text()).not.toContain(EMPTY_TEXT);
    } finally {
      wrapper.unmount();
    }
  });

  it("权限被拒绝时展示 permission_denied 与完整原始错误", async () => {
    const message = "action command failed: permission_denied: host_api: permission denied: network.request";
    post.mockImplementation((url: string) => {
      if (url === "/api/extensions/ui/sessions") return Promise.resolve(sessionResponse());
      return Promise.resolve(bridgeEnvelope("permission_denied", message));
    });
    const wrapper = await mountRenderer(catalogSchema());
    try {
      expect(failureCode(wrapper)).toBe("permission_denied");
      expect(failureRaw(wrapper)).toContain("permission denied: network.request");
      expect(wrapper.text()).not.toContain(EMPTY_TEXT);
    } finally {
      wrapper.unmount();
    }
  });

  it("超时时展示 action_timeout 与完整原始错误", async () => {
    const message = "action command failed: timeout: context deadline exceeded after 20s";
    post.mockImplementation((url: string) => {
      if (url === "/api/extensions/ui/sessions") return Promise.resolve(sessionResponse());
      return Promise.resolve(bridgeEnvelope("action_timeout", message));
    });
    const wrapper = await mountRenderer(catalogSchema());
    try {
      expect(failureCode(wrapper)).toBe("action_timeout");
      expect(failureRaw(wrapper)).toContain("context deadline exceeded after 20s");
      expect(wrapper.text()).not.toContain(EMPTY_TEXT);
    } finally {
      wrapper.unmount();
    }
  });

  it("取消请求后展示 action_cancelled 与完整原始错误", async () => {
    let bridgeSignal: AbortSignal | undefined;
    post.mockImplementation((url: string, _body: unknown, config?: { signal?: AbortSignal }) => {
      if (url === "/api/extensions/ui/sessions") return Promise.resolve(sessionResponse());
      bridgeSignal = config?.signal;
      return new Promise((_resolve, reject) => {
        config?.signal?.addEventListener("abort", () => {
          reject({ code: "ERR_CANCELED", message: "canceled by user" });
        });
      });
    });
    const wrapper = await mountRenderer(actionSchema());
    try {
      await wrapper.find("button").trigger("click");
      await settle(2);
      expect(bridgeSignal?.aborted).toBe(false);
      const cancel = wrapper.findAll("button").find(item => item.text().includes("取消请求"));
      expect(cancel).toBeTruthy();
      await cancel!.trigger("click");
      await settle();
      expect(failureCode(wrapper)).toBe("action_cancelled");
      expect(failureRaw(wrapper)).toContain("canceled by user");
    } finally {
      wrapper.unmount();
    }
  });
});
