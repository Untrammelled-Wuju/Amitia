const bridge = window.amitiaUI;

const state = {
  context: {},
  settings: {},
  rules: [],
  history: [],
  status: {},
  queue: {},
  busy: false,
};

const $ = (id) => document.getElementById(id);

function escapeHTML(value) {
  return String(value ?? "").replace(/[&<>"']/g, (char) => ({
    "&": "&amp;",
    "<": "&lt;",
    ">": "&gt;",
    '"': "&quot;",
    "'": "&#39;",
  }[char]));
}

function text(value, fallback = "") {
  const normalized = value == null ? "" : String(value).trim();
  return normalized || fallback;
}

function number(value, fallback = 0) {
  const parsed = Number(value);
  return Number.isFinite(parsed) ? parsed : fallback;
}

function setMessage(message, error = false) {
  $("message").textContent = message || "";
  $("message").className = error ? "message error" : "message";
}

function toast(message) {
  const item = document.createElement("div");
  item.textContent = message;
  $("toast").appendChild(item);
  setTimeout(() => item.remove(), 2800);
}

async function call(action, payload = {}) {
  if (!bridge) throw new Error("宿主桥接不可用");
  const response = await bridge.invokeAction("command", {
    action,
    payload: {
      ...payload,
      characterId: state.context.characterId || "",
      scope: {
        spaceId: state.context.spaceId || "",
        characterId: state.context.characterId || "",
        conversationId: state.context.conversationId || "",
      },
    },
  });
  if (response && response.ok === false) {
    throw new Error(response.error || "操作失败");
  }
  return response && Object.prototype.hasOwnProperty.call(response, "result")
    ? response.result
    : response;
}

function metric(label, value) {
  return `<div class="metric"><span>${escapeHTML(label)}</span><strong>${escapeHTML(value)}</strong></div>`;
}

function renderMetrics() {
  const status = state.status || {};
  const queue = state.queue || {};
  $("metrics").innerHTML = [
    metric("调度器", status.schedulerRunning ? "运行中" : "未运行"),
    metric("启用规则", `${number(status.enabledRuleCount, state.rules.filter((item) => item.enabled).length)} / ${number(status.totalRuleCount, state.rules.length)}`),
    metric("今日已发送", number(status.sentToday, 0)),
    metric("待处理", number(queue.pendingCount ?? queue.depth, 0)),
  ].join("");
}

function setRangeOutput(id, valueId, suffix) {
  $(valueId).textContent = `${$(id).value}${suffix}`;
}

function syncRangeOutputs() {
  setRangeOutput("active-level", "active-level-value", "%");
  setRangeOutput("min-interval", "min-interval-value", " 分钟");
  setRangeOutput("max-per-day", "max-per-day-value", " 次");
  setRangeOutput("max-daily-calls", "max-daily-calls-value", " 次");
}

function fillSettings(settings) {
  state.settings = { ...state.settings, ...settings };
  $("master-enabled").checked = settings.enabled !== false;
  $("active-level").value = number(settings.activeLevel, 40);
  $("min-interval").value = number(settings.minInterval, 60);
  $("max-per-day").value = number(settings.maxPerDay, 6);
  $("max-daily-calls").value = number(settings.maxDailyCalls, 10);
  $("quiet-start").value = text(settings.quietStart, "23:00");
  $("quiet-end").value = text(settings.quietEnd, "07:00");
  $("channel").value = text(settings.channel, "all");
  $("unreplied-enabled").checked = settings.unrepliedSlowdownEnabled !== false;
  syncRangeOutputs();
}

function ruleTypeLabel(type) {
  return {
    daily_greeting: "每日问候",
    sleep_reminder: "休息提醒",
    study_checkin: "学习提醒",
    work_break: "工作间歇",
    custom: "自定义",
    cron: "Cron",
  }[type] || text(type, "自定义");
}

function renderRules() {
  $("rule-summary").textContent = state.rules.length
    ? `共 ${state.rules.length} 条规则`
    : "暂无规则";
  $("rules").innerHTML = state.rules.length
    ? state.rules.map((rule) => `
      <article class="list-item" data-rule-id="${escapeHTML(rule.id)}">
        <div>
          <div class="list-title">${escapeHTML(text(rule.name, "未命名规则"))}</div>
          <div class="list-meta">${escapeHTML(ruleTypeLabel(rule.ruleType))} · ${escapeHTML(text(rule.scheduleCron, "未设置时间"))} · ${escapeHTML(text(rule.channel, "all"))}</div>
          <div class="list-meta">每天最多 ${number(rule.maxPerDay, 1)} 次 · 随机偏移 ${number(rule.randomMinutes, 0)} 分钟</div>
          ${text(rule.promptTemplate) ? `<div class="list-meta">${escapeHTML(rule.promptTemplate)}</div>` : ""}
        </div>
        <div class="rule-actions">
          <span class="badge ${rule.enabled ? "enabled" : ""}">${rule.enabled ? "已启用" : "已停用"}</span>
          <button type="button" data-command="toggle">${rule.enabled ? "停用" : "启用"}</button>
          <button type="button" data-command="edit">编辑</button>
          <button type="button" data-command="test">测试</button>
          <button type="button" data-command="trigger">立即触发</button>
          <button type="button" class="danger" data-command="delete">删除</button>
        </div>
      </article>
    `).join("")
    : '<div class="empty">暂无主动消息规则，点击“新建规则”开始配置。</div>';
}

function renderHistory() {
  $("history").innerHTML = state.history.length
    ? state.history.map((item) => `
      <div class="list-item">
        <div>
          <div class="list-title">${escapeHTML(text(item.title, text(item.triggerType, "主动消息")))}</div>
          <div class="list-meta">${escapeHTML(text(item.createdAt))}${text(item.lastError) ? ` · ${escapeHTML(item.lastError)}` : ""}</div>
        </div>
        <span class="badge ${text(item.state) === "sent" || text(item.state) === "success" ? "sent" : text(item.state) === "failed" ? "failed" : ""}">${escapeHTML(text(item.state, "unknown"))}</span>
      </div>
    `).join("")
    : '<div class="empty">暂无运行历史</div>';
}

function render() {
  $("subtitle").textContent = state.context.characterName
    ? `${state.context.characterName} 的主动消息规则与发送状态`
    : "主动消息规则、参数与发送状态";
  renderMetrics();
  renderRules();
  renderHistory();
}

async function load() {
  if (state.busy) return;
  state.busy = true;
  setMessage("正在读取...");
  try {
    const [settings, rules, status, queue, history] = await Promise.all([
      call("settings.get"),
      call("rules.list"),
      call("status"),
      call("queue.summary"),
      call("history.list", { page: 1, pageSize: 5 }),
    ]);
    fillSettings(settings || {});
    state.rules = Array.isArray(rules) ? rules : [];
    state.status = status || {};
    state.queue = queue || {};
    state.history = Array.isArray(history?.items) ? history.items : [];
    setMessage("已更新");
    render();
  } catch (error) {
    setMessage(error instanceof Error ? error.message : String(error), true);
  } finally {
    state.busy = false;
  }
}

async function saveSettings(enabled = $("master-enabled").checked) {
  const payload = {
    enabled,
    activeLevel: number($("active-level").value, 40),
    minInterval: number($("min-interval").value, 60),
    maxPerDay: number($("max-per-day").value, 6),
    maxDailyCalls: number($("max-daily-calls").value, 10),
    quietStart: $("quiet-start").value,
    quietEnd: $("quiet-end").value,
    channel: $("channel").value,
    unrepliedSlowdownEnabled: $("unreplied-enabled").checked,
  };
  const saved = await call("settings.update", payload);
  fillSettings(saved || payload);
  toast("主动消息参数已保存");
}

function openRuleDialog(rule = null) {
  $("rule-dialog-title").textContent = rule ? "编辑主动消息规则" : "新建主动消息规则";
  $("rule-id").value = text(rule?.id);
  $("rule-name").value = text(rule?.name);
  $("rule-type").value = text(rule?.ruleType, "daily_greeting");
  $("rule-cron").value = text(rule?.scheduleCron, "0 9 * * *");
  $("rule-channel").value = text(rule?.channel, "all");
  $("rule-quiet-start").value = text(rule?.quietStart, "23:00");
  $("rule-quiet-end").value = text(rule?.quietEnd, "07:00");
  $("rule-max-per-day").value = number(rule?.maxPerDay, 1);
  $("rule-random-minutes").value = number(rule?.randomMinutes, 0);
  $("rule-prompt").value = text(rule?.promptTemplate);
  $("rule-dialog").showModal();
}

function closeRuleDialog() {
  $("rule-dialog").close();
  $("rule-form").reset();
  $("rule-id").value = "";
}

async function saveRule() {
  const payload = {
    id: number($("rule-id").value, 0),
    name: $("rule-name").value.trim(),
    ruleType: $("rule-type").value,
    scheduleCron: $("rule-cron").value.trim(),
    channel: $("rule-channel").value,
    quietStart: $("rule-quiet-start").value,
    quietEnd: $("rule-quiet-end").value,
    maxPerDay: number($("rule-max-per-day").value, 1),
    randomMinutes: number($("rule-random-minutes").value, 0),
    promptTemplate: $("rule-prompt").value.trim(),
  };
  if (!payload.name || !payload.scheduleCron) return;
  await call(payload.id ? "rules.update" : "rules.create", payload);
  closeRuleDialog();
  await load();
  toast("规则已保存");
}

async function ruleCommand(rule, command) {
  if (command === "edit") {
    openRuleDialog(rule);
    return;
  }
  if (command === "delete") {
    if (!confirm(`确定删除“${text(rule.name, "未命名规则")}”吗？`)) return;
    await call("rules.delete", { id: rule.id });
  } else if (command === "toggle") {
    await call("rules.toggle", { id: rule.id });
  } else if (command === "test") {
    const result = await call("rules.test", { id: rule.id });
    alert(text(result?.content, "测试完成，未返回预览内容"));
    return;
  } else if (command === "trigger") {
    await call("rules.trigger", { id: rule.id });
  }
  await load();
}

async function resetPresets() {
  if (!confirm("恢复系统预设会替换当前角色的主动消息规则，继续吗？")) return;
  await call("presets.reset");
  await load();
  toast("系统预设已恢复");
}

$("settings-form").addEventListener("submit", async (event) => {
  event.preventDefault();
  try {
    await saveSettings();
  } catch (error) {
    toast(error instanceof Error ? error.message : String(error));
  }
});

$("master-enabled").addEventListener("change", async () => {
  try {
    await saveSettings($("master-enabled").checked);
  } catch (error) {
    $("master-enabled").checked = !$("master-enabled").checked;
    toast(error instanceof Error ? error.message : String(error));
  }
});

["active-level", "min-interval", "max-per-day", "max-daily-calls"].forEach((id) => {
  $(id).addEventListener("input", syncRangeOutputs);
});

$("rules").addEventListener("click", async (event) => {
  const button = event.target.closest("[data-command]");
  if (!button) return;
  const row = button.closest("[data-rule-id]");
  const rule = state.rules.find((item) => String(item.id) === row?.dataset.ruleId);
  if (!rule) return;
  try {
    await ruleCommand(rule, button.dataset.command);
  } catch (error) {
    toast(error instanceof Error ? error.message : String(error));
  }
});

$("add-rule").addEventListener("click", () => openRuleDialog());
$("close-rule-dialog").addEventListener("click", closeRuleDialog);
$("cancel-rule").addEventListener("click", closeRuleDialog);
$("rule-form").addEventListener("submit", async (event) => {
  event.preventDefault();
  try {
    await saveRule();
  } catch (error) {
    toast(error instanceof Error ? error.message : String(error));
  }
});
$("refresh").addEventListener("click", load);
$("reset-presets").addEventListener("click", async () => {
  try {
    await resetPresets();
  } catch (error) {
    toast(error instanceof Error ? error.message : String(error));
  }
});

async function boot() {
  if (!bridge) {
    setMessage("宿主桥接不可用", true);
    return;
  }
  await bridge.ready();
  state.context = await bridge.getContext();
  render();
  await load();
}

void boot();
