import { applyWebIndexInjections, instantiateClient, rawError, setDshActiveSession } from "./index.mjs";

async function resolveStatusPayload(response) {
  if (response && typeof response === "object" && Array.isArray(response.packages)) return response;
  const content = response?.result?.content ?? response?.content;
  const text = Array.isArray(content) ? content.find((item) => typeof item?.text === "string")?.text : "";
  if (text) return JSON.parse(text);
  if (typeof response === "string") return JSON.parse(response);
  return response?.output ?? response?.Output ?? response?.data ?? response;
}

function shellMounts() {
  if (!Array.isArray(globalThis.__dshShellMounts)) globalThis.__dshShellMounts = [];
  return globalThis.__dshShellMounts;
}

function topShell() {
  const mounts = shellMounts();
  return mounts[mounts.length - 1];
}

const shellCommand = (input) => {
  const top = topShell();
  if (!top) throw new Error("DSH shell command is unavailable: no DSH shell is mounted");
  return top.command(input);
};

const shellNavigate = (target) => {
  const top = topShell();
  if (!top) throw new Error("DSH shell navigate is unavailable: no DSH shell is mounted");
  if (typeof top.uiNavigate !== "function") throw new Error("DSH shell navigate is unavailable: uiNavigate action is missing");
  return top.uiNavigate(target);
};

function publishShell(mount) {
  globalThis.__dshHostCommand = shellCommand;
  globalThis.__dshUiActions = mount.uiActions;
  globalThis.__dshUiNavigate = shellNavigate;
  globalThis.__dshCurrentSessionId = mount.conversationId;
}

function withdrawShell(mount) {
  const mounts = shellMounts();
  const index = mounts.indexOf(mount);
  if (index >= 0) mounts.splice(index, 1);
  const top = topShell();
  if (top) {
    publishShell(top);
    return;
  }
  if (globalThis.__dshUiActions === mount.uiActions) delete globalThis.__dshUiActions;
  if (globalThis.__dshCurrentSessionId === mount.conversationId) delete globalThis.__dshCurrentSessionId;
}

export default {
  name: "DshClientShellHost",
  props: {
    actions: { type: Object, default: () => ({}) },
    uiActions: { type: Object, default: () => ({}) },
    uiContext: { type: Object, default: () => ({}) },
    uiNavigate: { type: Function, default: null },
  },
  setup(props) {
    const Vue = globalThis.__AMITIA_VUE__;
    if (!Vue) throw new Error("Amitia Vue runtime is unavailable");
    const disposers = Vue.shallowRef([]);
    let mount = null;
    const commandOf = () => {
      const actions = props.actions && typeof props.actions.command === "function" ? props.actions : props.uiActions;
      return actions && typeof actions.command === "function" ? actions.command : null;
    };
    Vue.onMounted(async () => {
      const command = commandOf();
      if (!command) throw new Error("DSH shell command is unavailable: host command action is missing");
      const bumpAfterMutation = async (payload) => {
        const response = await command(payload);
        const action = typeof payload?.action === "string" ? payload.action : "";
        if (/^conversation-(archive|pin)$/.test(action)) {
          globalThis.__dshBumpSlots?.();
          setTimeout(() => globalThis.__dshBumpSlots?.(), 900);
          setTimeout(() => globalThis.__dshBumpSlots?.(), 2500);
        }
        return response;
      };
      mount = {
        command: bumpAfterMutation,
        uiActions: props.uiActions,
        uiNavigate: props.uiNavigate,
        conversationId: props.uiContext.conversationId || undefined,
      };
      shellMounts().push(mount);
      setDshActiveSession(mount.conversationId);
      publishShell(mount);
      const response = await command({ action: "status" });
      const status = await resolveStatusPayload(response);
      const packages = Array.isArray(status?.packages) ? status.packages : [];
      const active = [];
      const failures = [];
      try {
        await applyWebIndexInjections();
      } catch (error) {
        failures.push(`web-index-injection: ${rawError(error)}`);
        console.error(`[dsh-shell] 宿主索引注入失败\n[original-error] ${rawError(error)}`);
      }
      for (const plugin of packages) {
        if (!plugin.enabled) continue;
        try {
          const instance = await instantiateClient(plugin);
          if (instance && typeof instance.dispose === "function") active.push(() => instance.dispose());
        } catch (error) {
          const pluginId = plugin.pluginId ?? plugin.id ?? "unknown";
          failures.push(`plugin ${pluginId}: ${rawError(error)}`);
          console.error(`[dsh-shell] DSH 客户端模块 ${pluginId} 装载失败\n[original-error] ${rawError(error)}`);
        }
      }
      disposers.value = active;
      if (failures.length > 0) {
        throw new Error(`DSH 客户端模块装载失败 ${failures.length} 个:\n${failures.join("\n\n")}`);
      }
    });
    Vue.onBeforeUnmount(async () => {
      const list = disposers.value;
      disposers.value = [];
      for (const dispose of list) await dispose();
      if (mount) {
        withdrawShell(mount);
        mount = null;
      }
    });
    return () => null;
  },
};
