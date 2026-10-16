const SETTINGS_KEY = "proactive-settings-v2";

function defaultSettings() {
  return {
    enabled: true,
    activeLevel: 40,
    quietStart: "23:00",
    quietEnd: "07:00",
    minInterval: 60,
    maxPerDay: 6,
    maxDailyCalls: 10,
    channel: "all",
    unrepliedSlowdownEnabled: true,
    unrepliedSlowdownAfter: 2,
    unrepliedCooldownMultiplier: 2,
    unrepliedRecoveryOnReply: true,
  };
}

function presetRules(characterId) {
  return [
    { name: "早安问候", ruleType: "daily_greeting", scheduleCron: "0 8 * * *", promptTemplate: "给用户发一条自然的早安问候。" },
    { name: "午间日常", ruleType: "daily_greeting", scheduleCron: "0 12 * * *", promptTemplate: "发一条轻松的午间日常消息。" },
    { name: "工作间歇", ruleType: "work_break", scheduleCron: "0 15 * * *", promptTemplate: "提醒用户短暂休息一下。" },
    { name: "晚间闲聊", ruleType: "custom", scheduleCron: "0 19 * * *", promptTemplate: "和用户自然聊起今天的事情。" },
    { name: "晚安提醒", ruleType: "sleep_reminder", scheduleCron: "30 22 * * *", promptTemplate: "发一条温和的晚安消息。" },
    { name: "睡前分享", ruleType: "sleep_reminder", scheduleCron: "0 23 * * *", promptTemplate: "分享一件轻松的小事，不要像客服。" },
  ].map((rule) => ({ enabled: true, channel: "all", conversationId: "", characterId, quietStart: "23:30", quietEnd: "07:00", maxPerDay: 1, ...rule }));
}

const clone = (value) => JSON.parse(JSON.stringify(value));

async function settings(host, characterId) {
  const result = await host.call("host.state.get", { key: `${SETTINGS_KEY}:${characterId}` });
  if (!result || typeof result.found !== "boolean") throw new Error("主动任务设置读取未确认");
  return { value: { ...defaultSettings(), ...(result.found ? result.value : {}) }, version: result.version || 0 };
}

async function saveSettings(host, characterId, next, expectedVersion) {
  const result = await host.call("host.state.cas", { key: `${SETTINGS_KEY}:${characterId}`, expectedVersion, newValue: next });
  if (!result?.swapped) throw new Error("主动任务设置发生冲突，请刷新后重试");
  return next;
}

function taskRecord(value) {
  if (typeof value?.id !== "string" || !value.id || !value.ownerId || value.saved !== true || !Number.isInteger(value.generation) || value.generation < 1) throw new Error("主动任务保存未确认");
  return value;
}

async function tasks(host) {
  const result = await host.call("host.proactive_task.list", {});
  if (!result || !Array.isArray(result.items)) throw new Error("主动任务列表响应无效");
  return result.items.map(taskRecord);
}

function ruleView(task) {
  return {
    ...clone(task),
    name: task.title,
    enabled: task.enabled === 1,
    ruleType: task.policy?.ruleType || "custom",
    scheduleCron: task.recurrence?.cron || "",
    promptTemplate: task.content,
    maxPerDay: task.policy?.ruleMaxPerDay || 1,
    randomMinutes: task.policy?.randomMinutes || 0,
    quietStart: task.policy?.ruleQuietStart || "",
    quietEnd: task.policy?.ruleQuietEnd || "",
    nextSendAt: task.remindAt || null,
    lastSentAt: task.lastTriggeredAt || "",
  };
}

function taskInput(rule, config, characterId, current) {
  const id = String(rule.id || "");
  const requestId = current?.requestId || rule.requestId || `proactive-create-${Date.now()}-${Math.random().toString(36).slice(2)}`;
  return {
    title: String(rule.name || current?.title || "主动任务"),
    content: String(rule.promptTemplate ?? current?.content ?? "发一条自然、简短的主动任务消息。"),
    characterId: String(rule.characterId || current?.characterId || characterId),
    conversationId: String(rule.conversationId ?? current?.conversationId ?? ""),
    channel: String(rule.channel || current?.channel || config.channel || "all"),
    channelTargets: channelTargets(rule.channelTargets ?? rule.channelTargetsText ?? current?.channelTargets ?? []),
    timezone: String(rule.timezone || current?.timezone || Intl.DateTimeFormat().resolvedOptions().timeZone || "Asia/Shanghai"),
    remindAt: String(rule.remindAt || current?.anchorAt || new Date(Date.now() + 1000).toISOString()),
    recurrence: { type: "cron", cron: String(rule.scheduleCron ?? current?.recurrence?.cron ?? "0 9 * * *") },
    enabled: rule.enabled ?? (current ? current.enabled === 1 : true),
    systemNotification: rule.systemNotification ?? current?.systemNotification ?? true,
    endAt: String(rule.endAt ?? current?.endAt ?? ""),
    maxOccurrences: Number(rule.maxOccurrences ?? current?.maxOccurrences ?? 0),
    requestId,
    ...(id && current ? { id, generation: current.generation } : {}),
    policy: {
      ...config,
      ruleMaxPerDay: Math.max(1, Number(rule.maxPerDay ?? current?.policy?.ruleMaxPerDay ?? 1)),
      ruleType: String(rule.ruleType ?? current?.policy?.ruleType ?? "custom"),
      randomMinutes: Math.max(0, Number(rule.randomMinutes ?? current?.policy?.randomMinutes ?? 0)),
      ruleQuietStart: String(rule.quietStart ?? current?.policy?.ruleQuietStart ?? ""),
      ruleQuietEnd: String(rule.quietEnd ?? current?.policy?.ruleQuietEnd ?? ""),
    },
  };
}

function channelTargets(value) {
  const targets = typeof value === "string" ? (value.trim() ? JSON.parse(value) : []) : value;
  if (!Array.isArray(targets) || targets.some((target) => !target?.channelId || !target?.peerId)) throw new Error("渠道目标须为包含 channelId 和 peerId 的 JSON 数组");
  return targets;
}

async function dashboard(host, characterId) {
  const [stored, records, status, queue, history] = await Promise.all([
    settings(host, characterId), tasks(host), host.call("host.proactive_task.status", {}), host.call("host.proactive_task.queue", {}), host.call("host.proactive_task.history", { page: 1, pageSize: 20 }),
  ]);
  if (!status || !queue || !Array.isArray(history?.items)) throw new Error("主动任务状态未接通");
  const rules = records.filter((task) => !characterId || task.characterId === characterId).map(ruleView);
  return {
    settings: clone(stored.value), rules, status, queue, history,
    form_state: {
      settings: clone(stored.value),
      rule: { id: "", name: "", ruleType: "daily_greeting", scheduleCron: "0 9 * * *", channel: "web", timezone: "Asia/Shanghai", channelTargetsText: "[]", endAt: "", maxOccurrences: 0, systemNotification: true, maxPerDay: 1, randomMinutes: 0, promptTemplate: "" },
    },
  };
}

async function dispatchCommand(host, input) {
  const action = String(input?.action || "");
  const payload = input?.payload && typeof input.payload === "object" ? input.payload : {};
  const characterId = String(payload.scope?.characterId || payload.characterId || "").trim();
  const id = String(payload.id || "");
  const done = async (result) => input?.ui_state === true ? dashboard(host, characterId) : result;
  if (action === "ui.dashboard") return dashboard(host, characterId);
  if (action === "settings.get") return (await settings(host, characterId)).value;
  if (action === "settings.update") {
    const stored = await settings(host, characterId);
    const next = { ...stored.value };
    for (const key of Object.keys(defaultSettings())) if (Object.prototype.hasOwnProperty.call(payload, key)) next[key] = payload[key];
    const changed = [];
    try {
      for (const current of await tasks(host)) {
        if (characterId && current.characterId !== characterId) continue;
        const updated = taskRecord(await host.call("host.proactive_task.update", { ...taskInput(ruleView(current), next, characterId, current), id: current.id, generation: current.generation }));
        changed.push({ current, updated });
      }
      await saveSettings(host, characterId, next, stored.version);
    } catch (error) {
      const failures = [];
      for (const { current, updated } of changed.reverse()) {
        try {
          taskRecord(await host.call("host.proactive_task.update", { ...taskInput(ruleView(current), stored.value, characterId, current), id: current.id, generation: updated.generation }));
        } catch (rollbackError) { failures.push(`${current.id}: ${String(rollbackError)}`); }
      }
      if (failures.length) throw new Error(`设置未保存，部分任务恢复失败，请刷新检查：${failures.join("；")}`);
      throw error;
    }
    return done(next);
  }
  if (action === "rules.list") return (await tasks(host)).filter((task) => !characterId || task.characterId === characterId).map(ruleView);
  if (action === "rules.create" || action === "rules.save" || action === "rules.update") {
    const current = id ? taskRecord(await host.call("host.proactive_task.get", { id })) : null;
    if (action === "rules.update" && !current) throw new Error("规则不存在");
    const config = (await settings(host, characterId)).value;
    const request = taskInput(payload, config, characterId, current);
    const result = taskRecord(await host.call(current ? "host.proactive_task.update" : "host.proactive_task.create", request));
    return done(ruleView(result));
  }
  if (action === "rules.delete" || action === "rules.toggle" || action === "rules.trigger") {
    const current = taskRecord(await host.call("host.proactive_task.get", { id }));
    if (action === "rules.delete") return done(taskRecord(await host.call("host.proactive_task.cancel", { id, generation: current.generation })));
    if (action === "rules.trigger") {
      const result = await host.call("host.proactive_task.execute", { id, generation: current.generation });
      if (!result?.triggerId) throw new Error("任务执行请求未确认");
      return done({ accepted: true, triggered: false, triggerId: result.triggerId, state: result.status });
    }
    if (current.enabled === 1) return done(ruleView(taskRecord(await host.call("host.proactive_task.pause", { id, generation: current.generation }))));
    if (current.paused) return done(ruleView(taskRecord(await host.call("host.proactive_task.resume", { id, generation: current.generation }))));
    const config = (await settings(host, characterId)).value;
    return done(ruleView(taskRecord(await host.call("host.proactive_task.update", { ...taskInput({ ...ruleView(current), enabled: true }, config, characterId, current), id, generation: current.generation }))));
  }
  if (action === "rules.test") return done(await host.call("host.proactive_task.preview", { id }));
  if (action === "rules.messages") return (await host.call("host.proactive_task.history", { id, page: 1, pageSize: 100 })).items;
  if (action === "history.list") return host.call("host.proactive_task.history", { page: payload.page || 1, pageSize: payload.pageSize || 20, state: payload.state || "" });
  if (action === "queue.summary") return host.call("host.proactive_task.queue", {});
  if (action === "queue.clear") return done(await host.call("host.proactive_task.clear", {}));
  if (action === "status") return host.call("host.proactive_task.status", {});
  if (action === "presets.reset") {
    if (!characterId) throw new Error("请先选择所属角色");
    for (const current of await tasks(host)) {
      if (current.characterId === characterId) taskRecord(await host.call("host.proactive_task.cancel", { id: current.id, generation: current.generation }));
    }
    const config = (await settings(host, characterId)).value;
    for (const rule of presetRules(characterId)) taskRecord(await host.call("host.proactive_task.create", taskInput(rule, config, characterId, null)));
    return done({ saved: true, count: 6 });
  }
  throw new Error(`未知主动任务操作: ${action}`);
}

const proactiveExtension = {
  async activate(context) {
    context.handlers.bindTool("command", async (input) => dispatchCommand(context.host, input));
    context.log.info("主动任务已接入宿主统一调度");
  },
  async deactivate() {},
};

if (typeof module !== "undefined" && module.exports) module.exports = proactiveExtension;
if (typeof globalThis.defineExtension === "function") globalThis.defineExtension(proactiveExtension);
