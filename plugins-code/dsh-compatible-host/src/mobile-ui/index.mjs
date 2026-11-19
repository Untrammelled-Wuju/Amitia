function text(value) {
  return value == null ? "" : String(value).trim();
}

function rawError(error) {
  if (error instanceof Error) {
    const cause = error.cause ? `\ncause: ${rawError(error.cause)}` : "";
    return `${error.name}: ${error.message}\n${error.stack ?? ""}${cause}`;
  }
  if (error && typeof error === "object") {
    try {
      return JSON.stringify(error, null, 2);
    } catch {
      return String(error);
    }
  }
  return String(error);
}

function element(tag, className, value) {
  const node = document.createElement(tag);
  if (className) node.className = className;
  if (value !== undefined) node.textContent = value;
  return node;
}

function button(label, onClick, options = {}) {
  const node = element(
    "button",
    `dsh-mobile__button${options.primary ? " dsh-mobile__button--primary" : ""}${options.danger ? " dsh-mobile__button--danger" : ""}`,
    label,
  );
  node.type = "button";
  node.disabled = options.disabled === true;
  node.addEventListener("click", onClick);
  return node;
}

export async function mount(root, context = {}) {
  if (!(root instanceof Element)) throw new Error("DSH mobile UI root element is required");
  if (!window.AmitiaHost || typeof window.AmitiaHost.invokeAction !== "function") {
    throw new Error("AmitiaHost mobile bridge is unavailable");
  }
  root.replaceChildren();
  const shell = element("section", "dsh-mobile");
  root.append(shell);
  const pendingErrorKey = "__AMITIA_DSH_MOBILE_ERROR__";
  let status = {
    packages: [],
    packageCount: 0,
    enabledCount: 0,
    targetDshVersion: "",
    compatibility: {},
  };
  const pendingSpecKey = "__AMITIA_DSH_MOBILE_PENDING_SPEC__";
  let spec = text(window[pendingSpecKey]);
  let error = text(window[pendingErrorKey]);
  let busy = false;
  let disposed = false;

  async function command(input) {
    let output = await window.AmitiaHost.invokeAction("command", input);
    console.error(`[dsh-command] action=${text(input?.action)} raw=${JSON.stringify(output).slice(0, 4000)}`);
    for (let depth = 0; depth < 6; depth += 1) {
      if (typeof output === "string") {
        try {
          output = JSON.parse(output);
        } catch {
          break;
        }
      }
      if (!output || typeof output !== "object") break;
      if (output.ok === false) {
        throw new Error(rawError(output.error ?? output));
      }
      if (output.ok === true && output.output !== undefined) {
        output = output.output;
        continue;
      }
      if ((output.status === "failed" || output.status === "error") && output.error != null) {
        throw new Error(rawError(output.error));
      }
      if (output.packageCount !== undefined || output.packages !== undefined || output.package !== undefined) {
        break;
      }
      const nested = ["output", "data", "result", "structured"].find((key) => output[key] !== undefined);
      if (!nested) break;
      output = output[nested];
    }
    if (
      output &&
      typeof output === "object" &&
      (output.ok === false || output.status === "failed" || output.status === "error") &&
      output.error != null
    ) {
      throw new Error(rawError(output.error));
    }
    console.error(`[dsh-command] action=${text(input?.action)} unwrapped=${JSON.stringify(output).slice(0, 4000)}`);
    return output;
  }

  function resize() {
    if (disposed) return;
    requestAnimationFrame(() => {
      const height = Math.max(1, Math.ceil(document.documentElement.scrollHeight));
      void window.AmitiaHost.resize(height).catch((failure) => {
        console.error("[original-error]", failure);
      });
    });
  }

  function metric(value, label) {
    const card = element("div", "dsh-mobile__metric");
    card.append(
      element("span", "dsh-mobile__metric-value", String(value ?? 0)),
      element("span", "dsh-mobile__metric-label", label),
    );
    return card;
  }

  function renderPackage(plugin) {
    const card = element("article", "dsh-mobile__card");
    const header = element("div", "dsh-mobile__header");
    const identity = element("div");
    identity.append(
      element("h3", "dsh-mobile__card-title", text(plugin.name) || text(plugin.pluginId)),
      element("p", "dsh-mobile__card-id", text(plugin.pluginId)),
    );
    const stateText = plugin.enabled === true
      ? (plugin.active === true ? "运行中" : "已启用·未运行")
      : "已停用";
    const state = element(
      "span",
      `dsh-mobile__status${plugin.enabled === true ? " dsh-mobile__status--enabled" : ""}`,
      stateText,
    );
    header.append(identity, state);
    const details = element("div", "dsh-mobile__details");
    details.append(
      element("span", "", `版本 ${text(plugin.version) || "未知"}`),
      element("span", "", `宿主入口 ${text(plugin.entry) || "无"}`),
      element("span", "", `客户端入口 ${text(plugin.clientEntry) || "无"}`),
      element("span", "", `映射贡献 ${Number(plugin.mappedContributionCount) || 0}`),
      element("span", "", `运行状态 ${plugin.active === true ? "运行中" : "未运行"}`),
    );
    if (text(plugin.activationError)) {
      details.append(element("pre", "dsh-mobile__error", text(plugin.activationError)));
    }
    const actions = element("div", "dsh-mobile__actions");
    actions.append(
      button(
        plugin.enabled ? "停用" : "启用",
        () => run(async () => {
          await command({
            action: "set-enabled",
            pluginId: plugin.pluginId,
            enabled: !plugin.enabled,
          });
        }),
        { disabled: busy },
      ),
      button(
        "移除",
        () => run(async () => {
          if (!window.confirm(`确定移除 ${text(plugin.name) || text(plugin.pluginId)}？`)) return;
          await command({ action: "remove", pluginId: plugin.pluginId });
        }),
        { danger: true, disabled: busy },
      ),
    );
    card.append(header, details, actions);
    return card;
  }

  function render() {
    shell.replaceChildren();
    const header = element("header", "dsh-mobile__header");
    const heading = element("div");
    heading.append(
      element("h2", "dsh-mobile__title", "DeepSeek Harness 插件"),
      element(
        "p",
        "dsh-mobile__meta",
        `目标 ABI ${text(status.targetDshVersion) || "未报告"} · mobile`,
      ),
    );
    header.append(
      heading,
      button("刷新", () => run(refresh), { disabled: busy }),
    );
    const install = element("div", "dsh-mobile__stack");
    const input = element("input", "dsh-mobile__input");
    input.value = spec;
    input.placeholder = "npm 包、Git 地址或本地 tarball";
    input.autocomplete = "off";
    input.spellcheck = false;
    const persistSpec = () => {
      spec = input.value;
      window[pendingSpecKey] = spec;
    };
    input.addEventListener("input", persistSpec);
    input.addEventListener("change", persistSpec);
    input.addEventListener("compositionend", persistSpec);
    input.addEventListener("blur", persistSpec);
    const installRow = element("div", "dsh-mobile__row");
    const installButton = button(
      busy ? "处理中…" : "安装 DSH 插件",
      () => {
        persistSpec();
        setTimeout(() => {
          const value = text(input.value || window[pendingSpecKey] || spec);
          void run(async () => {
            if (!value) throw new Error("请输入 DSH 插件包地址");
            const installed = await command({ action: "install", spec: value });
            if (!installed || typeof installed !== "object" || !installed.package) {
              throw new Error(`DSH 插件安装未返回插件记录：${JSON.stringify(installed)}`);
            }
            spec = "";
            window[pendingSpecKey] = "";
          });
        }, 0);
      },
      { primary: true, disabled: busy },
    );
    installButton.addEventListener("pointerdown", () => input.blur());
    installRow.append(
      input,
      installButton,
    );
    install.append(
      installRow,
      button(
        "安装官方 Todo 示例",
        () => run(async () => {
          const installed = await command({
            action: "install",
            spec: "@deepseek-ai/dsh-tool-todo@0.2.2-rc.2",
          });
          if (!installed || typeof installed !== "object" || !installed.package) {
            throw new Error(`DSH 插件安装未返回插件记录：${JSON.stringify(installed)}`);
          }
        }),
        { disabled: busy },
      ),
    );
    const summary = element("div", "dsh-mobile__summary");
    summary.append(
      metric(status.packageCount, "已安装"),
      metric(status.enabledCount, "已启用"),
      metric(
        Number(status.compatibility?.implemented) || 0,
        "ABI 已实现",
      ),
    );
    shell.append(header, install, summary);
    if (error) {
      shell.append(element("pre", "dsh-mobile__error", error));
    }
    const packages = Array.isArray(status.packages) ? status.packages : [];
    const list = element("div", "dsh-mobile__stack");
    if (packages.length === 0) {
      list.append(element("div", "dsh-mobile__empty", "尚未安装 DSH 子插件"));
    } else {
      for (const plugin of packages) list.append(renderPackage(plugin));
    }
    shell.append(list);
    resize();
  }

  async function refresh() {
    status = await command({ action: "status" });
  }

  async function run(task) {
    if (busy || disposed) return;
    busy = true;
    error = "";
    window[pendingErrorKey] = "";
    render();
    try {
      await task();
      await refresh();
    } catch (failure) {
      error = rawError(failure);
      window[pendingErrorKey] = error;
    } finally {
      busy = false;
      if (!disposed) render();
    }
  }

  const unsubscribeContext = window.AmitiaHost.subscribeContext((next) => {
    context = next && typeof next === "object" ? next : context;
    resize();
  });
  window.addEventListener("resize", resize, { passive: true });
  render();
  await run(refresh);
  return {
    dispose() {
      if (disposed) return;
      disposed = true;
      unsubscribeContext();
      window.removeEventListener("resize", resize);
      root.replaceChildren();
    },
  };
}
