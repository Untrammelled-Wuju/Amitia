const crypto = require("crypto");

const STATE_KEY = "emote-state";
const MAX_SEND_RECORDS = 5000;
const FILE_WRITE_CHUNK_CHARS = 256 * 1024;
const MAX_IMPORT_FILE_BYTES = 32 * 1024 * 1024;

const uploadSessions = new Map();
let commandQueue = Promise.resolve();

function enqueueCommand(task) {
  const run = commandQueue.then(task, task);
  commandQueue = run.then(
    () => undefined,
    () => undefined,
  );
  return run;
}

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

function safeSegment(value, fallback = "item") {
  const normalized = text(value).replace(/[^a-zA-Z0-9._-]/g, "_").slice(0, 80);
  return normalized || fallback;
}

function normalizeKeywords(values) {
  const result = [];
  const seen = new Set();
  const source = typeof values === "string" ? [values] : Array.isArray(values) ? values : [];
  for (const value of source) {
    for (const part of String(value).split(/[,，;；\n]/)) {
      const item = part.trim();
      if (!item || seen.has(item)) continue;
      seen.add(item);
      result.push(item);
    }
  }
  return result;
}

function defaultSettings() {
  return {
    enabled: true,
    maxPerHour: 5,
    sameEmoteCooldownMinutes: 30,
  };
}

function defaultState() {
  return {
    version: 1,
    emotes: [],
    groups: [],
    settings: defaultSettings(),
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
    groupIds: Array.isArray(value && value.groupIds) ? value.groupIds.map(text).filter(Boolean) : [],
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
  const rawSettings = value.settings && typeof value.settings === "object" && !Array.isArray(value.settings)
    ? value.settings
    : {};
  return {
    version: 1,
    emotes: Array.isArray(value.emotes) ? value.emotes.map(normalizeEmote) : [],
    groups: Array.isArray(value.groups) ? value.groups.map(normalizeGroup).sort((a, b) => a.sortOrder - b.sortOrder) : [],
    settings: {
      enabled: rawSettings.enabled !== false,
      maxPerHour: Math.max(0, Math.trunc(number(rawSettings.maxPerHour, base.settings.maxPerHour))),
      sameEmoteCooldownMinutes: Math.max(0, Math.trunc(number(rawSettings.sameEmoteCooldownMinutes, base.settings.sameEmoteCooldownMinutes))),
    },
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
    for (let offset = 0; offset < base64Data.length; offset += FILE_WRITE_CHUNK_CHARS) {
      await writeChunk(host, opened.handleId, base64Data.slice(offset, offset + FILE_WRITE_CHUNK_CHARS));
    }
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

function emoteAvailable(item) {
  return Boolean(item && item.enabled && item.aiEnabled && item.meaning);
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
    (record.triggerType === "ai_random" || record.triggerType === "ai_tool") &&
    record.hit === true &&
    Date.parse(record.createdAt || "") >= cutoff
  ).length;
  return count >= limit;
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

async function sendEmote(host, payload, triggerType = "manual", enforceLimits = false) {
  const snapshot = await readState(host);
  const state = snapshot.state;
  if (enforceLimits && !state.settings.enabled) throw new Error("表情发送已关闭");
  const item = findEmote(state, text(payload.emoteId));
  if (!item || !item.enabled) throw new Error("表情不存在");
  const characterId = text(payload.characterId);
  const conversationId = text(payload.conversationId);
  if (enforceLimits && inCooldown(state, item, characterId, state.settings.sameEmoteCooldownMinutes)) {
    throw new Error("该表情仍在冷却中");
  }
  if (enforceLimits && hourlyLimitReached(state, characterId, state.settings.maxPerHour)) {
    throw new Error("本小时表情发送次数已达上限");
  }
  const assetUrl = item.assetUrl || await resourceLink(host, item.filePath);
  const fallbackUrl = item.fallbackUrl || await resourceLink(host, item.fallbackPath);
  const result = await host.call("host.conversation.message.append", {
    spaceId: text(payload.spaceId),
    conversationId,
    characterId,
    channel: text(payload.channel) || "web",
    role: "user",
    source: "extension:com.amitia/emote",
    replyToMessageId: text(payload.replyToMessageId) || undefined,
    requestId: text(payload.requestId) || `emote-${triggerType}-${crypto.randomUUID()}`,
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
  await mutateState(host, (next) => {
    next.sendRecords.unshift({
      id: crypto.randomUUID(),
      emoteId: item.id,
      characterId,
      conversationId,
      responseId: text(payload.requestId),
      triggerType,
      hit: true,
      reason: text(payload.reason),
      createdAt: nowISO(),
      messageId: Array.isArray(result && result.messageIds) ? result.messageIds[0] : "",
    });
    next.sendRecords = next.sendRecords.slice(0, MAX_SEND_RECORDS);
    return next;
  });
  return result;
}

async function command(host, input) {
  const action = text(input && input.action);
  const payload = input && input.payload && typeof input.payload === "object" ? input.payload : {};
  switch (action) {
    case "ui.dashboard": {
      const snapshot = await readState(host);
      const items = await Promise.all(snapshot.state.emotes.map((item) => hydrateUrls(host, item)));
      return {
        emotes: items,
        groups: snapshot.state.groups,
        settings: snapshot.state.settings,
        form_state: {
          settings: clone(snapshot.state.settings),
          selected: {
            id: "",
            name: "",
            meaning: "",
            keywords: "",
            aiEnabled: false,
          },
          group: {
            name: "",
          },
        },
      };
    }
    case "ui.select_emote": {
      const snapshot = await readState(host);
      const item = findEmote(snapshot.state, text(payload.id));
      if (!item) throw new Error("表情不存在");
      const hydrated = await hydrateUrls(host, item);
      return {
        form_state: {
          selected: {
            id: hydrated.id,
            name: hydrated.name,
            meaning: hydrated.meaning,
            keywords: hydrated.keywords.join("，"),
            aiEnabled: hydrated.aiEnabled,
            groupIds: hydrated.groupIds,
          },
        },
      };
    }
    case "ui.composer": {
      const snapshot = await readState(host);
      const page = filterList(snapshot.state, {
        groupId: text(payload.groupId),
        view: text(payload.groupId) === "recent" ? "recent" : "",
        page: 1,
        pageSize: 200,
      });
      return {
        emotes: await Promise.all(page.items.map((item) => hydrateUrls(host, item))),
        groups: snapshot.state.groups,
        form_state: {
          groupId: text(payload.groupId),
        },
      };
    }
    case "ui.import": {
      const files = Array.isArray(input && input.files) ? input.files : [];
      let imported = 0;
      let duplicate = 0;
      let rejected = 0;
      for (const file of files) {
        const dataBase64 = text(file && file.dataBase64);
        if (!dataBase64) {
          rejected += 1;
          continue;
        }
        const declaredSize = Math.max(0, number(file && file.size));
        const estimatedBytes = Math.floor((dataBase64.length * 3) / 4);
        if (Math.max(declaredSize, estimatedBytes) > MAX_IMPORT_FILE_BYTES) {
          rejected += 1;
          continue;
        }
        const extension = safeSegment(text(file && file.name).split(".").pop() || "png", "png").toLowerCase();
        const uploadId = crypto.randomUUID();
        const paths = {
          original: `uploads/${uploadId}/original.${extension}`,
          thumbnail: `uploads/${uploadId}/thumbnail.${extension}`,
          fallback: `uploads/${uploadId}/fallback.${extension}`,
        };
        await writeBase64File(host, paths.original, dataBase64);
        await writeBase64File(host, paths.thumbnail, dataBase64);
        await writeBase64File(host, paths.fallback, dataBase64);
        const fileHash = await readFileHash(host, paths.original);
        let existed = false;
        let created = null;
        await mutateState(host, (state) => {
          existed = state.emotes.some((item) => item.fileHash === fileHash);
          if (existed) return state;
          const now = nowISO();
          created = normalizeEmote({
            id: crypto.randomUUID(),
            name: text(file && file.name).replace(/\.[^.]+$/, "") || "表情",
            meaning: "",
            keywords: [],
            originalFilename: text(file && file.name),
            filePath: paths.original,
            thumbnailPath: paths.thumbnail,
            fallbackPath: paths.fallback,
            mimeType: text(file && file.mimeType),
            fileExtension: `.${extension}`,
            fileSize: Math.max(0, number(file && file.size)),
            width: 0,
            height: 0,
            isAnimated: extension === "gif",
            frameCount: 1,
            fileHash,
            enabled: true,
            aiEnabled: false,
            groupIds: [],
            createdAt: now,
            updatedAt: now,
          });
          state.emotes.unshift(created);
          return state;
        });
        if (existed) {
          duplicate += 1;
          await deleteResource(host, paths.original);
          await deleteResource(host, paths.thumbnail);
          await deleteResource(host, paths.fallback);
          continue;
        }
        created.assetUrl = await resourceLink(host, created.filePath);
        created.thumbnailUrl = await resourceLink(host, created.thumbnailPath);
        created.fallbackUrl = await resourceLink(host, created.fallbackPath);
        await mutateState(host, (state) => {
          const index = state.emotes.findIndex((item) => item.id === created.id);
          if (index >= 0) state.emotes[index] = created;
          return state;
        });
        imported += 1;
      }
      const parts = [];
      if (imported > 0) parts.push(`已导入 ${imported} 张`);
      if (duplicate > 0) parts.push(`已存在 ${duplicate} 张`);
      if (rejected > 0) parts.push(`${rejected} 张因格式或体积超限(${Math.round(MAX_IMPORT_FILE_BYTES / 1024 / 1024)}MB)被跳过`);
      return {
        imported,
        duplicate,
        rejected,
        total: files.length,
        message: parts.length > 0 ? parts.join("，") : "没有可导入的图片",
      };
    }
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
    case "settings.get": {
      const snapshot = await readState(host);
      return snapshot.state.settings;
    }
    case "settings.save": {
      let settings = null;
      await mutateState(host, (state) => {
        settings = { ...state.settings, ...payload };
        state.settings = settings;
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
          groupIds: Array.isArray(payload.groupIds) ? payload.groupIds.map(text) : [],
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
        for (const key of ["name", "meaning"]) {
          if (payload[key] !== undefined) item[key] = text(payload[key]);
        }
        if (payload.keywords !== undefined) item.keywords = normalizeKeywords(payload.keywords);
        if (payload.enabled !== undefined) item.enabled = Boolean(payload.enabled);
        if (payload.aiEnabled !== undefined) item.aiEnabled = Boolean(payload.aiEnabled);
        if (!item.meaning) item.aiEnabled = false;
        if (payload.groupIds !== undefined) item.groupIds = payload.groupIds.map(text);
        item.updatedAt = nowISO();
        return state;
      });
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
        await deleteResource(host, item.filePath);
        await deleteResource(host, item.thumbnailPath);
        await deleteResource(host, item.fallbackPath);
      }
      return { deleted: removed.length };
    }
    case "emotes.send": {
      return sendEmote(host, payload, "manual", false);
    }
    default:
      throw new Error(`未知表情包操作: ${action}`);
  }
}

async function ai(host, input) {
  const action = text(input && input.action);
  const payload = input && typeof input === "object" ? input : {};
  const invocationContext = payload.__amitiaContext && typeof payload.__amitiaContext === "object" ? payload.__amitiaContext : {};
  const snapshot = await readState(host);
  const state = snapshot.state;
  const characterId = text(payload.characterId) || text(invocationContext.characterId);
  if (action === "list") {
    const groups = new Map(state.groups.map((group) => [group.id, group.name]));
    return {
      emotes: state.emotes.filter(emoteAvailable).map((item) => ({
        id: item.id,
        name: item.name,
        meaning: item.meaning,
        keywords: item.keywords,
        groups: item.groupIds.map((id) => groups.get(id)).filter(Boolean),
        mimeType: item.mimeType,
        width: item.width,
        height: item.height,
        isAnimated: item.isAnimated,
        sendableNow: !inCooldown(state, item, characterId, state.settings.sameEmoteCooldownMinutes),
      })),
      settings: state.settings,
    };
  }
  if (action === "send") {
    const emoteId = text(payload.emoteId);
    const item = findEmote(state, emoteId);
    if (!emoteAvailable(item)) throw new Error("表情不存在、未启用或未开放给 AI");
    const result = await sendEmote(host, {
      ...payload,
      characterId,
      spaceId: text(payload.spaceId) || text(invocationContext.spaceId),
      conversationId: text(payload.conversationId) || text(invocationContext.conversationId),
      channel: text(payload.channel) || text(invocationContext.channel),
      requestId: text(payload.requestId) || text(invocationContext.operationId),
    }, "ai_tool", true);
    return {
      status: "sent",
      emoteId,
      name: item.name,
      messageIds: result && Array.isArray(result.messageIds) ? result.messageIds : [],
    };
  }
  throw new Error(`未知 AI 表情操作: ${action}`);
}

const emoteExtension = {
  async activate(context) {
    context.handlers.bindTool("command", (input) => enqueueCommand(() => command(context.host, input)));
    context.handlers.bindTool("ai", (input) => enqueueCommand(() => ai(context.host, input)));
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
