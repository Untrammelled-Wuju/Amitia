const crypto = require("crypto");

const STATE_KEY = "emote-state";
const VECTOR_COLLECTION = "emotes";
const MAX_SEND_RECORDS = 5000;
const MIN_SIMILARITY = 0.35;

const uploadSessions = new Map();

function nowISO() {
  return new Date().toISOString();
}

function clone(value) {
  return JSON.parse(JSON.stringify(value));
}

function text(value) {
  return value == null ? "" : String(value).trim();
}

function number(value, fallback = 0) {
  const parsed = Number(value);
  return Number.isFinite(parsed) ? parsed : fallback;
}

function boolInt(value) {
  return value ? 1 : 0;
}

function safeSegment(value, fallback = "item") {
  const normalized = text(value).replace(/[^a-zA-Z0-9._-]/g, "_").slice(0, 80);
  return normalized || fallback;
}

function normalizeKeywords(values) {
  const result = [];
  const seen = new Set();
  for (const value of Array.isArray(values) ? values : []) {
    for (const part of String(value).split(/[,，;；\n]/)) {
      const item = part.trim();
      if (!item || seen.has(item)) continue;
      seen.add(item);
      result.push(item);
    }
  }
  return result;
}

function defaultSettings(characterId) {
  return {
    characterId,
    enabled: true,
    baseProbability: 0.1,
    maxProbability: 0.3,
    maxPerHour: 5,
    minReplyGap: 3,
    sameEmoteCooldownMinutes: 30,
    allowEmoteOnly: false,
  };
}

function defaultState() {
  return {
    version: 1,
    emotes: [],
    groups: [],
    settings: {},
    sendRecords: [],
    updatedAt: nowISO(),
  };
}

function normalizeEmote(value) {
  return {
    id: text(value && value.id),
    name: text(value && value.name),
    meaning: text(value && value.meaning),
    keywords: normalizeKeywords(value && value.keywords),
    originalFilename: text(value && value.originalFilename),
    filePath: text(value && value.filePath),
    thumbnailPath: text(value && value.thumbnailPath),
    fallbackPath: text(value && value.fallbackPath),
    assetUrl: text(value && value.assetUrl),
    thumbnailUrl: text(value && value.thumbnailUrl),
    fallbackUrl: text(value && value.fallbackUrl),
    mimeType: text(value && value.mimeType),
    fileExtension: text(value && value.fileExtension),
    fileSize: Math.max(0, number(value && value.fileSize)),
    width: Math.max(0, Math.trunc(number(value && value.width))),
    height: Math.max(0, Math.trunc(number(value && value.height))),
    isAnimated: Boolean(value && value.isAnimated),
    durationMs: Math.max(0, Math.trunc(number(value && value.durationMs))),
    frameCount: Math.max(1, Math.trunc(number(value && value.frameCount, 1))),
    fileHash: text(value && value.fileHash),
    enabled: value && value.enabled === false ? false : true,
    aiEnabled: Boolean(value && value.aiEnabled),
    roleScope: text(value && value.roleScope) || "all_characters",
    characterIds: Array.isArray(value && value.characterIds) ? value.characterIds.map(text).filter(Boolean) : [],
    groupIds: Array.isArray(value && value.groupIds) ? value.groupIds.map(text).filter(Boolean) : [],
    vectorStatus: text(value && value.vectorStatus) || "disabled",
    vectorError: text(value && value.vectorError),
    createdAt: text(value && value.createdAt) || nowISO(),
    updatedAt: text(value && value.updatedAt) || nowISO(),
  };
}

function normalizeGroup(value) {
  return {
    id: text(value && value.id),
    name: text(value && value.name),
    coverEmoteId: text(value && value.coverEmoteId),
    sortOrder: Math.max(0, Math.trunc(number(value && value.sortOrder))),
    createdAt: text(value && value.createdAt) || nowISO(),
    updatedAt: text(value && value.updatedAt) || nowISO(),
  };
}

function normalizeState(value) {
  const base = defaultState();
  if (!value || typeof value !== "object") return base;
  const settings = {};
  for (const [key, setting] of Object.entries(value.settings || {})) {
    settings[key] = { ...defaultSettings(key), ...(setting || {}), characterId: key };
  }
  return {
    version: 1,
    emotes: Array.isArray(value.emotes) ? value.emotes.map(normalizeEmote) : [],
    groups: Array.isArray(value.groups) ? value.groups.map(normalizeGroup).sort((a, b) => a.sortOrder - b.sortOrder) : [],
    settings,
    sendRecords: Array.isArray(value.sendRecords) ? value.sendRecords.slice(0, MAX_SEND_RECORDS) : [],
    updatedAt: text(value && value.updatedAt) || nowISO(),
  };
}

async function readState(host) {
  const response = await host.call("host.state.get", { key: STATE_KEY });
  return {
    state: normalizeState(response && response.found ? response.value : null),
    version: response && Number.isFinite(Number(response.version)) ? Number(response.version) : 0,
  };
}

async function mutateState(host, mutate) {
  for (let attempt = 0; attempt < 8; attempt += 1) {
    const snapshot = await readState(host);
    const next = normalizeState(mutate(clone(snapshot.state)));
    next.updatedAt = nowISO();
    const result = await host.call("host.state.cas", {
      key: STATE_KEY,
      expectedVersion: snapshot.version,
      newValue: next,
    });
    if (result && result.swapped) return next;
  }
  throw new Error("表情包状态写入冲突");
}

async function openResource(host, path, mode) {
  return host.call("host.resource.open", {
    scope: "data",
    path,
    mode,
  });
}

async function writeChunk(host, handleId, base64Data) {
  return host.call("host.resource.write", {
    handleId,
    data: base64Data,
    encoding: "base64",
  });
}

async function closeResource(host, handleId) {
  try {
    await host.call("host.resource.close", { handleId });
  } catch {}
}

async function writeBase64File(host, path, base64Data) {
  const opened = await openResource(host, path, "w");
  try {
    await writeChunk(host, opened.handleId, base64Data);
  } finally {
    await closeResource(host, opened.handleId);
  }
}

async function readFileHash(host, path) {
  const opened = await openResource(host, path, "r");
  const hash = crypto.createHash("sha256");
  try {
    while (true) {
      const chunk = await host.call("host.resource.read", {
        handleId: opened.handleId,
        length: 1024 * 1024,
        encoding: "base64",
      });
      const data = Buffer.from(text(chunk && chunk.data), "base64");
      if (data.length) hash.update(data);
      if (!chunk || chunk.eof) break;
    }
  } finally {
    await closeResource(host, opened.handleId);
  }
  return hash.digest("hex");
}

async function resourceLink(host, path) {
  const result = await host.call("host.resource.link", {
    scope: "data",
    path,
  });
  return text(result && result.url);
}

async function deleteResource(host, path) {
  if (!text(path)) return;
  try {
    await host.call("host.resource.delete", {
      scope: "data",
      path,
    });
  } catch {}
}

async function hydrateUrls(host, emote) {
  const item = normalizeEmote(emote);
  if (!item.assetUrl && item.filePath) item.assetUrl = await resourceLink(host, item.filePath);
  if (!item.thumbnailUrl && item.thumbnailPath) item.thumbnailUrl = await resourceLink(host, item.thumbnailPath);
  if (!item.fallbackUrl && item.fallbackPath) item.fallbackUrl = await resourceLink(host, item.fallbackPath);
  return item;
}

function findEmote(state, id) {
  return state.emotes.find((item) => item.id === id) || null;
}

function emoteAvailableForCharacter(item, characterId) {
  if (!item || !item.enabled || !item.aiEnabled || !item.meaning) return false;
  if (item.roleScope === "all_characters") return true;
  return item.roleScope === "selected_characters" && item.characterIds.includes(characterId);
}

function inCooldown(state, item, characterId, minutes) {
  if (minutes <= 0) return false;
  const cutoff = Date.now() - minutes * 60 * 1000;
  return state.sendRecords.some((record) =>
    record.emoteId === item.id &&
    record.characterId === characterId &&
    record.hit === true &&
    Date.parse(record.createdAt || "") >= cutoff
  );
}

function hourlyLimitReached(state, characterId, limit) {
  if (limit <= 0) return true;
  const cutoff = Date.now() - 60 * 60 * 1000;
  const count = state.sendRecords.filter((record) =>
    record.characterId === characterId &&
    record.triggerType === "ai_random" &&
    record.hit === true &&
    Date.parse(record.createdAt || "") >= cutoff
  ).length;
  return count >= limit;
}

function replyGapBlocked(records, conversationId, characterId, gap) {
  if (gap <= 0) return false;
  const last = records.find((record) =>
    record.conversationId === conversationId &&
    record.characterId === characterId &&
    record.hit === true
  );
  return Boolean(last && number(last.replyCountAfter, gap) < gap);
}

function weightedSelect(items, random = Math.random) {
  const eligible = items.filter((item) => item.score >= MIN_SIMILARITY).slice(0, 5);
  if (!eligible.length) return null;
  const total = eligible.reduce((sum, item) => sum + item.score, 0);
  if (total <= 0) return null;
  let sample = random() * total;
  for (const item of eligible) {
    sample -= item.score;
    if (sample < 0) return item;
  }
  return eligible[eligible.length - 1];
}

function contextFactor(userMessage, reply, source) {
  if (["system", "tool", "error"].includes(source)) return 0;
  const length = Array.from(reply || "").length;
  if (length > 500) return 0.2;
  if (length > 240) return 0.3;
  if (String(userMessage || "").includes("[表情") || String(userMessage || "").includes("表情包")) return 1.3;
  if (length <= 60) return 1.2;
  return 1;
}

function finalProbability(base, maxValue, factor) {
  return Math.max(0, Math.min(number(maxValue), number(base) * number(factor, 1)));
}

function suppressedContext(source, reply) {
  if (["system", "tool", "error"].includes(source)) return true;
  const lower = String(reply || "").toLowerCase();
  return ["报错", "错误", "失败", "安全提示", "风险", "正式通知", "系统通知", "紧急", "报警", "急救", "验证码", "密码", "error:", "warning:", "traceback", "stack trace"]
    .some((marker) => lower.includes(marker));
}

async function syncVector(host, item) {
  if (!item.aiEnabled || !item.enabled || !item.meaning) {
    try {
      await host.call("host.vector.delete", {
        collection: VECTOR_COLLECTION,
        ids: [item.id],
      });
    } catch {}
    return "disabled";
  }
  try {
    await host.call("host.vector.upsert", {
      collection: VECTOR_COLLECTION,
      points: [{
        id: item.id,
        text: `名称：${item.name}\n含义：${item.meaning}\n关键词：${item.keywords.join("、")}`,
        payload: {
          enabled: "true",
          ai_enabled: "true",
          role_scope: item.roleScope,
        },
      }],
    });
    return "ready";
  } catch (error) {
    return `failed:${error instanceof Error ? error.message : String(error)}`;
  }
}

function filterList(state, payload) {
  const groupId = text(payload && payload.groupId);
  const view = text(payload && payload.view);
  const query = text(payload && payload.q).toLowerCase();
  let items = [...state.emotes];
  if (groupId) items = items.filter((item) => item.groupIds.includes(groupId));
  if (view === "unassigned") items = items.filter((item) => item.groupIds.length === 0);
  if (view === "recent") {
    const order = new Map();
    for (const record of state.sendRecords) {
      if (!record.emoteId) continue;
      const value = Date.parse(record.createdAt || "") || 0;
      order.set(record.emoteId, Math.max(order.get(record.emoteId) || 0, value));
    }
    items.sort((a, b) => (order.get(b.id) || 0) - (order.get(a.id) || 0));
  } else {
    items.sort((a, b) => String(b.createdAt).localeCompare(String(a.createdAt)));
  }
  if (query) {
    items = items.filter((item) =>
      [item.name, item.meaning, item.keywords.join(" ")].join(" ").toLowerCase().includes(query)
    );
  }
  const page = Math.max(1, Math.trunc(number(payload && payload.page, 1)));
  const pageSize = Math.min(200, Math.max(1, Math.trunc(number(payload && payload.pageSize, 60))));
  return {
    items: items.slice((page - 1) * pageSize, page * pageSize),
    total: items.length,
    page,
    pageSize,
  };
}

async function command(host, input) {
  const action = text(input && input.action);
  const payload = input && input.payload && typeof input.payload === "object" ? input.payload : {};
  switch (action) {
    case "emotes.list": {
      const snapshot = await readState(host);
      const page = filterList(snapshot.state, payload);
      page.items = await Promise.all(page.items.map((item) => hydrateUrls(host, item)));
      return page;
    }
    case "groups.list": {
      const snapshot = await readState(host);
      return snapshot.state.groups;
    }
    case "characters.list": {
      return host.call("host.character.list", {
        includeDisabled: Boolean(payload.includeDisabled),
      });
    }
    case "settings.get": {
      const characterId = text(payload.characterId);
      if (!characterId) throw new Error("缺少角色");
      const snapshot = await readState(host);
      return snapshot.state.settings[characterId] || defaultSettings(characterId);
    }
    case "settings.save": {
      const characterId = text(payload.characterId);
      if (!characterId) throw new Error("缺少角色");
      let settings = null;
      await mutateState(host, (state) => {
        settings = { ...defaultSettings(characterId), ...payload, characterId };
        state.settings[characterId] = settings;
        return state;
      });
      return settings;
    }
    case "groups.create": {
      const name = text(payload.name);
      if (!name) throw new Error("分组名称不能为空");
      let group = null;
      await mutateState(host, (state) => {
        const now = nowISO();
        group = {
          id: crypto.randomUUID(),
          name,
          coverEmoteId: "",
          sortOrder: state.groups.length,
          createdAt: now,
          updatedAt: now,
        };
        state.groups.push(group);
        return state;
      });
      return group;
    }
    case "groups.update": {
      const id = text(payload.id);
      let group = null;
      await mutateState(host, (state) => {
        group = state.groups.find((item) => item.id === id) || null;
        if (!group) throw new Error("分组不存在");
        if (payload.name !== undefined) group.name = text(payload.name);
        if (payload.coverEmoteId !== undefined) group.coverEmoteId = text(payload.coverEmoteId);
        group.updatedAt = nowISO();
        return state;
      });
      return group;
    }
    case "groups.delete": {
      const id = text(payload.id);
      await mutateState(host, (state) => {
        state.groups = state.groups.filter((item) => item.id !== id);
        for (const item of state.emotes) item.groupIds = item.groupIds.filter((groupId) => groupId !== id);
        return state;
      });
      return { deleted: true };
    }
    case "groups.reorder": {
      const ids = Array.isArray(payload.ids) ? payload.ids.map(text) : [];
      await mutateState(host, (state) => {
        const order = new Map(ids.map((id, index) => [id, index]));
        state.groups.sort((a, b) => (order.get(a.id) ?? a.sortOrder) - (order.get(b.id) ?? b.sortOrder));
        state.groups.forEach((group, index) => { group.sortOrder = index; });
        return state;
      });
      return { updated: true };
    }
    case "groups.add": {
      const groupId = text(payload.groupId);
      const ids = Array.isArray(payload.emoteIds) ? payload.emoteIds.map(text) : [];
      await mutateState(host, (state) => {
        if (!state.groups.some((item) => item.id === groupId)) throw new Error("分组不存在");
        for (const item of state.emotes) {
          if (ids.includes(item.id) && !item.groupIds.includes(groupId)) item.groupIds.push(groupId);
        }
        return state;
      });
      return { updated: true };
    }
    case "groups.remove": {
      const groupId = text(payload.groupId);
      const emoteId = text(payload.emoteId);
      await mutateState(host, (state) => {
        const item = findEmote(state, emoteId);
        if (item) item.groupIds = item.groupIds.filter((id) => id !== groupId);
        return state;
      });
      return { updated: true };
    }
    case "upload.begin": {
      const uploadId = crypto.randomUUID();
      const ext = safeSegment(text(payload.extension).replace(/^\./, "").toLowerCase(), "bin").slice(0, 10);
      const paths = {
        original: `uploads/${uploadId}/original.${ext}`,
        thumbnail: `uploads/${uploadId}/thumbnail.png`,
        fallback: `uploads/${uploadId}/fallback.png`,
      };
      uploadSessions.set(uploadId, { paths });
      return { uploadId, paths };
    }
    case "upload.chunk": {
      const uploadId = text(payload.uploadId);
      const kind = text(payload.kind);
      const session = uploadSessions.get(uploadId);
      if (!session || !session.paths[kind]) throw new Error("上传会话不存在");
      const opened = await openResource(host, session.paths[kind], "a");
      try {
        await writeChunk(host, opened.handleId, text(payload.data));
      } finally {
        await closeResource(host, opened.handleId);
      }
      return { written: true };
    }
    case "upload.cancel": {
      const uploadId = text(payload.uploadId);
      const session = uploadSessions.get(uploadId);
      if (session) {
        for (const path of Object.values(session.paths)) await deleteResource(host, path);
        uploadSessions.delete(uploadId);
      }
      return { cancelled: true };
    }
    case "upload.complete": {
      const uploadId = text(payload.uploadId);
      const session = uploadSessions.get(uploadId);
      if (!session) throw new Error("上传会话不存在");
      const fileHash = await readFileHash(host, session.paths.original);
      let duplicate = null;
      let created = null;
      await mutateState(host, (state) => {
        duplicate = state.emotes.find((item) => item.fileHash === fileHash) || null;
        if (duplicate) {
          const groups = Array.isArray(payload.groupIds) ? payload.groupIds.map(text) : [];
          for (const groupId of groups) {
            if (!duplicate.groupIds.includes(groupId)) duplicate.groupIds.push(groupId);
          }
          duplicate.updatedAt = nowISO();
          return state;
        }
        const now = nowISO();
        created = normalizeEmote({
          id: crypto.randomUUID(),
          name: text(payload.name) || "表情",
          meaning: text(payload.meaning),
          keywords: payload.keywords,
          originalFilename: text(payload.originalFilename),
          filePath: session.paths.original,
          thumbnailPath: session.paths.thumbnail,
          fallbackPath: session.paths.fallback,
          mimeType: text(payload.mimeType),
          fileExtension: text(payload.fileExtension),
          fileSize: number(payload.fileSize),
          width: number(payload.width),
          height: number(payload.height),
          isAnimated: Boolean(payload.isAnimated),
          durationMs: number(payload.durationMs),
          frameCount: number(payload.frameCount, 1),
          fileHash,
          enabled: true,
          aiEnabled: Boolean(payload.aiEnabled && text(payload.meaning)),
          roleScope: text(payload.roleScope) || "all_characters",
          characterIds: Array.isArray(payload.characterIds) ? payload.characterIds.map(text) : [],
          groupIds: Array.isArray(payload.groupIds) ? payload.groupIds.map(text) : [],
          vectorStatus: "disabled",
          createdAt: now,
          updatedAt: now,
        });
        state.emotes.unshift(created);
        return state;
      });
      uploadSessions.delete(uploadId);
      if (duplicate) {
        for (const path of Object.values(session.paths)) await deleteResource(host, path);
        return { status: "duplicate", emoteId: duplicate.id };
      }
      const vectorStatus = await syncVector(host, created);
      created.vectorStatus = vectorStatus.startsWith("failed:") ? "failed" : vectorStatus;
      created.vectorError = vectorStatus.startsWith("failed:") ? vectorStatus.slice(7) : "";
      created.assetUrl = await resourceLink(host, created.filePath);
      created.thumbnailUrl = await resourceLink(host, created.thumbnailPath);
      created.fallbackUrl = await resourceLink(host, created.fallbackPath);
      await mutateState(host, (state) => {
        const index = state.emotes.findIndex((item) => item.id === created.id);
        if (index >= 0) state.emotes[index] = created;
        return state;
      });
      return { status: "success", item: created };
    }
    case "emotes.update": {
      const id = text(payload.id);
      let item = null;
      await mutateState(host, (state) => {
        item = findEmote(state, id);
        if (!item) throw new Error("表情不存在");
        for (const key of ["name", "meaning", "roleScope"]) {
          if (payload[key] !== undefined) item[key] = text(payload[key]);
        }
        if (payload.keywords !== undefined) item.keywords = normalizeKeywords(payload.keywords);
        if (payload.enabled !== undefined) item.enabled = Boolean(payload.enabled);
        if (payload.aiEnabled !== undefined) item.aiEnabled = Boolean(payload.aiEnabled);
        if (!item.meaning) item.aiEnabled = false;
        if (payload.characterIds !== undefined) item.characterIds = payload.characterIds.map(text);
        if (payload.groupIds !== undefined) item.groupIds = payload.groupIds.map(text);
        item.updatedAt = nowISO();
        return state;
      });
      const vectorStatus = await syncVector(host, item);
      item.vectorStatus = vectorStatus.startsWith("failed:") ? "failed" : vectorStatus;
      item.vectorError = vectorStatus.startsWith("failed:") ? vectorStatus.slice(7) : "";
      await mutateState(host, (state) => {
        const index = state.emotes.findIndex((value) => value.id === id);
        if (index >= 0) state.emotes[index] = item;
        return state;
      });
      return hydrateUrls(host, item);
    }
    case "emotes.batch_update": {
      const ids = Array.isArray(payload.ids) ? payload.ids.map(text) : [];
      const update = payload.update && typeof payload.update === "object" ? payload.update : {};
      const selected = [];
      await mutateState(host, (state) => {
        for (const item of state.emotes) {
          if (!ids.includes(item.id)) continue;
          if (update.aiEnabled !== undefined) item.aiEnabled = Boolean(update.aiEnabled);
          if (update.enabled !== undefined) item.enabled = Boolean(update.enabled);
          item.updatedAt = nowISO();
          selected.push(clone(item));
        }
        return state;
      });
      await Promise.all(selected.map((item) => syncVector(host, item)));
      return { updated: selected.length };
    }
    case "emotes.delete": {
      const ids = Array.isArray(payload.ids) ? payload.ids.map(text) : [text(payload.id)].filter(Boolean);
      const removed = [];
      await mutateState(host, (state) => {
        removed.push(...state.emotes.filter((item) => ids.includes(item.id)));
        state.emotes = state.emotes.filter((item) => !ids.includes(item.id));
        state.sendRecords = state.sendRecords.filter((record) => !ids.includes(record.emoteId));
        return state;
      });
      for (const item of removed) {
        await host.call("host.vector.delete", { collection: VECTOR_COLLECTION, ids: [item.id] }).catch(() => {});
        await deleteResource(host, item.filePath);
        await deleteResource(host, item.thumbnailPath);
        await deleteResource(host, item.fallbackPath);
      }
      return { deleted: removed.length };
    }
    case "emotes.send": {
      const snapshot = await readState(host);
      const item = findEmote(snapshot.state, text(payload.emoteId));
      if (!item || !item.enabled) throw new Error("表情不存在");
      const assetUrl = item.assetUrl || await resourceLink(host, item.filePath);
      const fallbackUrl = item.fallbackUrl || await resourceLink(host, item.fallbackPath);
      const result = await host.call("host.conversation.message.append", {
        conversationId: text(payload.conversationId),
        characterId: text(payload.characterId),
        channel: text(payload.channel) || "web",
        role: "user",
        source: "extension:com.amitia/emote",
        replyToMessageId: text(payload.replyToMessageId) || undefined,
        requestId: `emote-manual-${crypto.randomUUID()}`,
        parts: [{
          type: "image",
          extensionType: "emote",
          url: assetUrl,
          fallbackUrl,
          altText: `[表情：${item.name}]`,
          mimeType: item.mimeType,
          width: item.width,
          height: item.height,
          isAnimated: item.isAnimated,
        }],
      });
      await mutateState(host, (state) => {
        state.sendRecords.unshift({
          id: crypto.randomUUID(),
          emoteId: item.id,
          characterId: text(payload.characterId),
          conversationId: text(payload.conversationId),
          triggerType: "manual",
          hit: true,
          createdAt: nowISO(),
          messageId: Array.isArray(result && result.messageIds) ? result.messageIds[0] : "",
        });
        state.sendRecords = state.sendRecords.slice(0, MAX_SEND_RECORDS);
        return state;
      });
      return result;
    }
    default:
      throw new Error(`未知表情包操作: ${action}`);
  }
}

async function output(host, input) {
  const event = input && typeof input === "object" ? input : {};
  const characterId = text(event.characterId);
  const conversationId = text(event.conversationId);
  if (!characterId || !conversationId) return { outputs: [] };
  const snapshot = await readState(host);
  const state = snapshot.state;
  const settings = state.settings[characterId] || defaultSettings(characterId);
  const lines = Array.isArray(event.lines) ? event.lines : [];
  const reply = text(event.reply);
  const emoteOnly = lines.length === 0 && !reply && settings.allowEmoteOnly;
  if (!settings.enabled || (!reply && !emoteOnly) || event.forceVoice || suppressedContext(event.source, reply)) {
    return { outputs: [] };
  }
  if (!state.emotes.some((item) => emoteAvailableForCharacter(item, characterId))) {
    return { outputs: [] };
  }
  if (hourlyLimitReached(state, characterId, settings.maxPerHour)) return { outputs: [] };
  if (replyGapBlocked(state.sendRecords, conversationId, characterId, settings.minReplyGap)) {
    return { outputs: [] };
  }
  const probability = finalProbability(
    settings.baseProbability,
    settings.maxProbability,
    contextFactor(event.userMessage, reply, event.source)
  );
  const sample = Math.random();
  if (sample >= probability) return { outputs: [] };
  let results = [];
  try {
    const search = await host.call("host.vector.search", {
      collection: VECTOR_COLLECTION,
      query: `${text(event.userMessage)}\n${reply}`,
      limit: 10,
      filter: { enabled: "true", ai_enabled: "true" },
    });
    results = Array.isArray(search && search.items) ? search.items : [];
  } catch {}
  const candidates = [];
  for (const result of results) {
    const item = findEmote(state, text(result.id));
    if (!emoteAvailableForCharacter(item, characterId)) continue;
    if (inCooldown(state, item, characterId, settings.sameEmoteCooldownMinutes)) continue;
    candidates.push({ item, score: number(result.score) });
  }
  const selected = weightedSelect(candidates);
  if (!selected || selected.score < MIN_SIMILARITY) return { outputs: [] };
  const item = selected.item;
  const assetUrl = item.assetUrl || await resourceLink(host, item.filePath);
  const fallbackUrl = item.fallbackUrl || await resourceLink(host, item.fallbackPath);
  let insertAfter = lines.length;
  let sendMode = emoteOnly ? "emote_only" : "after_all_text";
  if (!emoteOnly && lines.length >= 2 && reply.length <= 240 && Math.random() < 0.25) {
    insertAfter = 1 + Math.floor(Math.random() * (lines.length - 1));
    sendMode = "between_text_messages";
  }
  await mutateState(host, (next) => {
    next.sendRecords.unshift({
      id: crypto.randomUUID(),
      emoteId: item.id,
      characterId,
      conversationId,
      responseId: text(event.requestId),
      triggerType: "ai_random",
      probability,
      sample,
      score: selected.score,
      hit: true,
      sendMode,
      createdAt: nowISO(),
    });
    next.sendRecords = next.sendRecords.slice(0, MAX_SEND_RECORDS);
    return next;
  });
  return {
    outputs: [{
      outputId: text(event.requestId) || crypto.randomUUID(),
      extensionId: "com.amitia/emote",
      insertAfter,
      sendMode,
      part: {
        type: "image",
        extensionType: "emote",
        url: assetUrl,
        fallbackUrl,
        altText: `[表情：${item.name}]`,
        mimeType: item.mimeType,
        width: item.width,
        height: item.height,
        isAnimated: item.isAnimated,
        metadata: {
          emoteId: item.id,
          score: selected.score,
        },
      },
    }],
  };
}

const emoteExtension = {
  async activate(context) {
    context.handlers.bindTool("command", (input) => command(context.host, input));
    context.handlers.bindTool("output", (input) => output(context.host, input));
  },
  async deactivate() {
    uploadSessions.clear();
  },
};

if (typeof module !== "undefined" && module.exports) {
  module.exports = emoteExtension;
}

if (typeof globalThis.defineExtension === "function") {
  globalThis.defineExtension(emoteExtension);
}
