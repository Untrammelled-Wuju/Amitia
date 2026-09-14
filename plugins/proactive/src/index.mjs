const STATE_KEY = "proactive-state";
const TICK_INTERVAL_MS = 30000;
const MAX_HISTORY = 500;

let tickTimer = null;
let activeHost = null;
let activeLogger = null;

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

function defaultState() {
  return {
    settings: defaultSettings(),
    nextRuleId: 1,
    rules: [],
    history: [],
    lastTickAt: "",
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
  ].map((rule) => ({
    enabled: true,
    channel: "all",
    conversationId: "",
    characterId: characterId || "",
    quietStart: "23:30",
    quietEnd: "07:00",
    maxPerDay: 1,
    lastSentAt: "",
    sentCountToday: 0,
    createdAt: new Date().toISOString(),
    updatedAt: new Date().toISOString(),
    ...rule,
  }));
}

function clone(value) {
  return JSON.parse(JSON.stringify(value));
}

function normalizeState(value) {
  const base = defaultState();
  if (!value || typeof value !== "object") return base;
  return {
    settings: { ...base.settings, ...(value.settings || {}) },
    nextRuleId: Number.isFinite(Number(value.nextRuleId)) ? Number(value.nextRuleId) : base.nextRuleId,
    rules: Array.isArray(value.rules) ? value.rules.map(normalizeRule) : [],
    history: Array.isArray(value.history) ? value.history.slice(0, MAX_HISTORY) : [],
    lastTickAt: typeof value.lastTickAt === "string" ? value.lastTickAt : "",
  };
}

function normalizeRule(rule) {
  return {
    id: Number(rule.id || 0),
    name: String(rule.name || "主动消息"),
    enabled: rule.enabled !== false,
    channel: String(rule.channel || "all"),
    conversationId: String(rule.conversationId || ""),
    characterId: String(rule.characterId || ""),
    ruleType: String(rule.ruleType || "custom"),
    scheduleCron: String(rule.scheduleCron || "0 9 * * *"),
    quietStart: String(rule.quietStart || ""),
    quietEnd: String(rule.quietEnd || ""),
    maxPerDay: Math.max(1, Number(rule.maxPerDay || 1)),
    lastSentAt: String(rule.lastSentAt || ""),
    sentCountToday: Math.max(0, Number(rule.sentCountToday || 0)),
    promptTemplate: String(rule.promptTemplate || ""),
    randomMinutes: Math.max(0, Number(rule.randomMinutes || 0)),
    createdAt: String(rule.createdAt || new Date().toISOString()),
    updatedAt: String(rule.updatedAt || new Date().toISOString()),
  };
}

async function readState(host) {
  const response = await host.call("host.state.get", { key: STATE_KEY });
  return {
    state: response && response.found ? normalizeState(response.value) : defaultState(),
    version: response && Number.isFinite(Number(response.version)) ? Number(response.version) : 0,
  };
}

async function mutateState(host, mutate) {
  for (let attempt = 0; attempt < 8; attempt += 1) {
    const snapshot = await readState(host);
    const nextState = normalizeState(mutate(clone(snapshot.state)));
    const result = await host.call("host.state.cas", {
      key: STATE_KEY,
      expectedVersion: snapshot.version,
      newValue: nextState,
    });
    if (result && result.swapped) return nextState;
  }
  throw new Error("主动消息状态写入冲突");
}

function dateKey(date) {
  const year = date.getFullYear();
  const month = String(date.getMonth() + 1).padStart(2, "0");
  const day = String(date.getDate()).padStart(2, "0");
  return `${year}-${month}-${day}`;
}

function minuteOfDay(value) {
  const text = String(value || "").trim();
  const match = text.match(/^(\d{1,2}):(\d{2})$/);
  if (!match) return -1;
  const hour = Number(match[1]);
  const minute = Number(match[2]);
  if (hour < 0 || hour > 23 || minute < 0 || minute > 59) return -1;
  return hour * 60 + minute;
}

function cronMinute(expression) {
  const parts = String(expression || "").trim().split(/\s+/);
  if (parts.length < 2) return minuteOfDay(expression);
  const minute = Number(parts[0]);
  const hour = Number(parts[1]);
  if (!Number.isFinite(minute) || !Number.isFinite(hour)) return -1;
  if (minute < 0 || minute > 59 || hour < 0 || hour > 23) return -1;
  return hour * 60 + minute;
}

function quietHoursAllow(start, end, nowMinute) {
  const startMinute = minuteOfDay(start);
  const endMinute = minuteOfDay(end);
  if (startMinute < 0 || endMinute < 0) return true;
  if (startMinute <= endMinute) return nowMinute < startMinute || nowMinute >= endMinute;
  return nowMinute >= endMinute && nowMinute < startMinute;
}

function parseTime(value) {
  const parsed = Date.parse(String(value || ""));
  return Number.isFinite(parsed) ? parsed : 0;
}

function ruleSentCount(rule, history, today) {
  return history.filter((item) => item.ruleId === rule.id && item.state === "sent" && String(item.createdAt || "").slice(0, 10) === today).length;
}

function nextSendAt(rule) {
  const target = cronMinute(rule.scheduleCron);
  if (target < 0) return null;
  const now = new Date();
  const targetTime = new Date(now.getFullYear(), now.getMonth(), now.getDate(), Math.floor(target / 60), target % 60, 0, 0);
  if (targetTime.getTime() <= now.getTime()) targetTime.setDate(targetTime.getDate() + 1);
  return targetTime.toISOString();
}

function ruleDue(rule, settings, history, now) {
  if (!rule.enabled) return false;
  const target = cronMinute(rule.scheduleCron);
  if (target < 0) return false;
  const nowMinute = now.getHours() * 60 + now.getMinutes();
  if (nowMinute < target || nowMinute > target + 2) return false;
  if (!quietHoursAllow(rule.quietStart || settings.quietStart, rule.quietEnd || settings.quietEnd, nowMinute)) return false;
  const today = dateKey(now);
  if (ruleSentCount(rule, history, today) >= rule.maxPerDay) return false;
  if (rule.lastSentAt && String(rule.lastSentAt).slice(0, 10) === today) return false;
  const lastSent = parseTime(rule.lastSentAt);
  const minimumGap = Math.max(5, Number(settings.minInterval || 60)) * 60 * 1000;
  return !lastSent || now.getTime() - lastSent >= minimumGap;
}

async function dispatchMessage(host, settings, rule, now) {
  const characterId = String(rule.characterId || "").trim();
  if (!characterId) throw new Error("缺少角色");
  const userId = String(rule.userId || "").trim();
  const conversationId = String(rule.conversationId || "").trim();
  const channel = String(rule.channel || settings.channel || "all");
  const content = String(rule.promptTemplate || "发一条自然、简短的主动消息。").trim();
  const requestId = `proactive:${rule.id}:${now.getTime()}`;
  const response = await host.call("host.conversation.message.send", {
    userId,
    characterId,
    conversationId,
    channel,
    content,
    requestId,
  });
  const responseContent = String(response && response.content || "").trim();
  if (!responseContent) throw new Error("主动消息被宿主抑制或未生成内容");
  return {
    requestId: String(response && response.requestId || requestId),
    content: responseContent,
  };
}

async function runDue(host, logger) {
  const snapshot = await readState(host);
  const state = snapshot.state;
  if (!state.settings.enabled) return { processed: 0, sent: 0, failed: 0 };
  const now = new Date();
  const dueRules = state.rules.filter((rule) => ruleDue(rule, state.settings, state.history, now));
  if (dueRules.length === 0) {
    await mutateState(host, (next) => {
      next.lastTickAt = now.toISOString();
      return next;
    });
    return { processed: 0, sent: 0, failed: 0 };
  }
  let sent = 0;
  let failed = 0;
  for (const rule of dueRules) {
    try {
      const result = await dispatchMessage(host, state.settings, rule, now);
      sent += 1;
      await mutateState(host, (next) => {
        const target = next.rules.find((item) => item.id === rule.id);
        if (target) {
          target.lastSentAt = now.toISOString();
          target.sentCountToday = ruleSentCount(target, next.history, dateKey(now)) + 1;
          target.updatedAt = now.toISOString();
        }
        next.history.unshift({
          id: `${rule.id}-${now.getTime()}`,
          ruleId: rule.id,
          title: rule.name,
          triggerType: rule.ruleType,
          channel: rule.channel,
          state: "sent",
          reason: "",
          requestId: result.requestId,
          attemptCount: 1,
          lastError: "",
          createdAt: now.toISOString(),
          updatedAt: now.toISOString(),
        });
        next.history = next.history.slice(0, MAX_HISTORY);
        next.lastTickAt = now.toISOString();
        return next;
      });
    } catch (error) {
      failed += 1;
      await mutateState(host, (next) => {
        next.history.unshift({
          id: `${rule.id}-${now.getTime()}`,
          ruleId: rule.id,
          title: rule.name,
          triggerType: rule.ruleType,
          channel: rule.channel,
          state: "failed",
          reason: "",
          requestId: "",
          attemptCount: 1,
          lastError: error instanceof Error ? error.message : String(error),
          createdAt: now.toISOString(),
          updatedAt: now.toISOString(),
        });
        next.history = next.history.slice(0, MAX_HISTORY);
        next.lastTickAt = now.toISOString();
        return next;
      });
      logger.warn("proactive rule dispatch failed", { ruleId: rule.id, error: error instanceof Error ? error.message : String(error) });
    }
  }
  return { processed: dueRules.length, sent, failed };
}

async function status(host) {
  const snapshot = await readState(host);
  const state = snapshot.state;
  const today = dateKey(new Date());
  const todayHistory = state.history.filter((item) => String(item.createdAt || "").slice(0, 10) === today);
  const enabled = state.rules.filter((rule) => rule.enabled);
  return {
    schedulerRunning: Boolean(tickTimer),
    enabledRuleCount: enabled.length,
    totalRuleCount: state.rules.length,
    sentToday: todayHistory.filter((item) => item.state === "sent").length,
    failedToday: todayHistory.filter((item) => item.state === "failed").length,
    nextRules: enabled.map((rule) => ({ id: rule.id, nextSendAt: nextSendAt(rule) })),
  };
}

async function dispatchCommand(host, logger, input) {
  const action = String(input && input.action || "");
  const payload = input && input.payload && typeof input.payload === "object" ? input.payload : {};
  const scope = payload.scope && typeof payload.scope === "object" ? payload.scope : {};
  const characterId = String(scope.characterId || payload.characterId || "").trim();
  switch (action) {
    case "settings.get": {
      const snapshot = await readState(host);
      return snapshot.state.settings;
    }
    case "settings.update": {
      const next = await mutateState(host, (state) => {
        state.settings = { ...state.settings, ...payload };
        delete state.settings.scope;
        return state;
      });
      return next.settings;
    }
    case "rules.list": {
      const snapshot = await readState(host);
      const rules = characterId
        ? snapshot.state.rules.filter((rule) => !rule.characterId || rule.characterId === characterId)
        : snapshot.state.rules;
      return clone(rules);
    }
    case "rules.create": {
      const now = new Date().toISOString();
      let created = null;
      await mutateState(host, (state) => {
        const id = state.nextRuleId || 1;
        created = normalizeRule({
          ...payload,
          id,
          userId: String(scope.userId || payload.userId || ""),
          characterId: payload.characterId || characterId,
          createdAt: now,
          updatedAt: now,
        });
        state.nextRuleId = id + 1;
        state.rules.push(created);
        return state;
      });
      return clone(created);
    }
    case "rules.update": {
      const id = Number(payload.id || 0);
      let updated = null;
      await mutateState(host, (state) => {
        const index = state.rules.findIndex((rule) => rule.id === id);
        if (index < 0) throw new Error("规则不存在");
        updated = normalizeRule({ ...state.rules[index], ...payload, id, updatedAt: new Date().toISOString() });
        state.rules[index] = updated;
        return state;
      });
      return clone(updated);
    }
    case "rules.delete": {
      const id = Number(payload.id || 0);
      await mutateState(host, (state) => {
        state.rules = state.rules.filter((rule) => rule.id !== id);
        state.history = state.history.filter((item) => item.ruleId !== id);
        return state;
      });
      return { deleted: true, id };
    }
    case "rules.toggle": {
      const id = Number(payload.id || 0);
      let updated = null;
      await mutateState(host, (state) => {
        const rule = state.rules.find((item) => item.id === id);
        if (!rule) throw new Error("规则不存在");
        rule.enabled = !rule.enabled;
        rule.updatedAt = new Date().toISOString();
        updated = clone(rule);
        return state;
      });
      return updated;
    }
    case "rules.test":
    case "rules.trigger": {
      const id = Number(payload.id || 0);
      const snapshot = await readState(host);
      const rule = snapshot.state.rules.find((item) => item.id === id);
      if (!rule) throw new Error("规则不存在");
      const now = new Date();
      const result = await dispatchMessage(host, snapshot.state.settings, rule, now);
      if (action === "rules.trigger") {
        await mutateState(host, (state) => {
          const target = state.rules.find((item) => item.id === id);
          if (target) {
            target.lastSentAt = now.toISOString();
            target.sentCountToday = ruleSentCount(target, state.history, dateKey(now)) + 1;
            target.updatedAt = now.toISOString();
          }
          state.history.unshift({
            id: `${id}-${now.getTime()}`,
            ruleId: id,
            title: rule.name,
            triggerType: rule.ruleType,
            channel: rule.channel,
            state: "sent",
            reason: "manual",
            requestId: result.requestId,
            attemptCount: 1,
            lastError: "",
            createdAt: now.toISOString(),
            updatedAt: now.toISOString(),
          });
          state.history = state.history.slice(0, MAX_HISTORY);
          return state;
        });
      }
      return { ok: true, requestId: result.requestId };
    }
    case "rules.messages": {
      const id = Number(payload.id || 0);
      const snapshot = await readState(host);
      return snapshot.state.history.filter((item) => item.ruleId === id).slice(0, 100);
    }
    case "history.list": {
      const snapshot = await readState(host);
      const page = Math.max(1, Number(payload.page || 1));
      const pageSize = Math.min(100, Math.max(1, Number(payload.pageSize || 20)));
      const stateFilter = String(payload.state || "");
      const filtered = stateFilter ? snapshot.state.history.filter((item) => item.state === stateFilter) : snapshot.state.history;
      return {
        items: clone(filtered.slice((page - 1) * pageSize, page * pageSize)),
        total: filtered.length,
        page,
        pageSize,
      };
    }
    case "queue.summary": {
      const snapshot = await readState(host);
      const today = dateKey(new Date());
      return {
        depth: snapshot.state.rules.filter((rule) => rule.enabled).length,
        pendingCount: 0,
        recentFailures: snapshot.state.history.filter((item) => item.state === "failed" && String(item.createdAt || "").slice(0, 10) === today).length,
        backpressure: false,
      };
    }
    case "presets.reset": {
      const next = await mutateState(host, (state) => {
        const rules = presetRules(characterId);
        let nextId = state.nextRuleId || 1;
        for (const rule of rules) {
          rule.id = nextId;
          rule.userId = String(scope.userId || "");
          nextId += 1;
        }
        state.nextRuleId = nextId;
        state.rules = rules;
        state.history = [];
        return state;
      });
      return { ok: true, count: next.rules.length };
    }
    case "tick":
      return runDue(host, logger);
    case "status":
      return status(host);
    default:
      throw new Error(`未知主动消息操作: ${action}`);
  }
}

const proactiveExtension = {
  async activate(context) {
    activeHost = context.host;
    activeLogger = context.log;
    context.handlers.bindTool("command", async (input) => dispatchCommand(activeHost, activeLogger, input));
    tickTimer = setInterval(() => {
      if (!activeHost || !activeLogger) return;
      void runDue(activeHost, activeLogger).catch((error) => {
        activeLogger.warn("proactive tick failed", { error: error instanceof Error ? error.message : String(error) });
      });
    }, TICK_INTERVAL_MS);
    setTimeout(() => {
      if (!activeHost || !activeLogger) return;
      void runDue(activeHost, activeLogger).catch((error) => {
        activeLogger.warn("proactive initial tick failed", { error: error instanceof Error ? error.message : String(error) });
      });
    }, 0);
    context.log.info("proactive plugin activated");
  },
  async deactivate() {
    if (tickTimer) clearInterval(tickTimer);
    tickTimer = null;
    activeHost = null;
    activeLogger = null;
  },
};

if (typeof module !== "undefined" && module.exports) {
  module.exports = proactiveExtension;
}

if (typeof globalThis.defineExtension === "function") {
  globalThis.defineExtension(proactiveExtension);
}
