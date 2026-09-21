import Module from "node:module";
import { readFileSync } from "node:fs";
import { dirname, join } from "node:path";
import { fileURLToPath } from "node:url";

const root = dirname(dirname(fileURLToPath(import.meta.url)));
const sourcePath = join(root, "src", "index.mjs");
let extension = null;
globalThis.defineExtension = (value) => {
  extension = value;
};
const moduleInstance = new Module(sourcePath);
moduleInstance.filename = sourcePath;
moduleInstance.paths = Module._nodeModulePaths(dirname(sourcePath));
moduleInstance._compile(readFileSync(sourcePath, "utf8"), sourcePath);
delete globalThis.defineExtension;

let passed = 0;
let failed = 0;
const failures = [];

function check(name, condition, detail = "") {
  if (condition) {
    passed += 1;
    console.log(`  ✓ ${name}`);
  } else {
    failed += 1;
    failures.push(`${name}${detail ? ` — ${detail}` : ""}`);
    console.log(`  ✗ ${name}${detail ? ` — ${detail}` : ""}`);
  }
}

function makeHost(options = {}) {
  const store = new Map();
  const tools = new Map();
  const resources = new Map();
  const handles = new Map();
  const calls = { vectorUpsert: [], vectorDelete: [], resourceDelete: [], messageAppend: [] };
  let version = 0;
  let vectorSearchHandler = options.vectorSearch || (() => ({ items: [] }));
  const host = {
    calls,
    setVectorSearch(handler) {
      vectorSearchHandler = handler;
    },
    async call(method, params) {
      if (method === "host.state.get") {
        return { found: store.has(params.key), value: store.get(params.key), version };
      }
      if (method === "host.state.cas") {
        if (params.expectedVersion !== version) return { swapped: false, newVersion: version };
        store.set(params.key, params.newValue);
        version += 1;
        return { swapped: true, newVersion: version };
      }
      if (method === "host.resource.open") {
        const id = `handle-${Math.random()}`;
        if (!resources.has(params.path)) resources.set(params.path, Buffer.alloc(0));
        handles.set(id, { path: params.path, mode: params.mode, offset: 0 });
        return { handleId: id };
      }
      if (method === "host.resource.write") {
        const handle = handles.get(params.handleId);
        if (!handle) throw new Error("句柄不存在");
        const chunk = Buffer.from(params.data || "", params.encoding || "utf8");
        const current = resources.get(handle.path) || Buffer.alloc(0);
        if (handle.mode === "a") resources.set(handle.path, Buffer.concat([current, chunk]));
        else resources.set(handle.path, chunk);
        return { written: chunk.length };
      }
      if (method === "host.resource.read") {
        const handle = handles.get(params.handleId);
        if (!handle) throw new Error("句柄不存在");
        const current = resources.get(handle.path) || Buffer.alloc(0);
        const remaining = current.subarray(handle.offset);
        const length = params.length || 1024 * 1024;
        const slice = remaining.subarray(0, length);
        handle.offset += slice.length;
        const eof = handle.offset >= current.length;
        return { data: slice.toString(params.encoding || "base64"), eof, encoding: params.encoding || "base64" };
      }
      if (method === "host.resource.close") {
        handles.delete(params.handleId);
        return { ok: true };
      }
      if (method === "host.resource.link") return { url: `/api/extension/resources/${encodeURIComponent(params.path)}` };
      if (method === "host.resource.delete") {
        resources.delete(params.path);
        calls.resourceDelete.push(params.path);
        return { deleted: true };
      }
      if (method === "host.vector.upsert") {
        calls.vectorUpsert.push(params);
        return { upserted: params.points.length };
      }
      if (method === "host.vector.search") return vectorSearchHandler(params);
      if (method === "host.vector.delete") {
        calls.vectorDelete.push(params);
        return { deleted: params.ids.length };
      }
      if (method === "host.conversation.message.append") {
        calls.messageAppend.push(params);
        return { messageIds: ["message-1"], sequences: [1], deliveryGroupId: "response-1", lastSequence: 1 };
      }
      throw new Error(`unexpected host method: ${method}`);
    },
    setState(value) {
      store.set("emote-state", value);
      version += 1;
    },
    getState() {
      return store.get("emote-state");
    },
  };

  return { host, tools, resources, store };
}

async function activate(makeHostResult) {
  await extension.activate({
    host: makeHostResult.host,
    handlers: { bindTool(name, handler) { makeHostResult.tools.set(name, handler); } },
    log: { info() {}, warn() {}, error() {}, debug() {} },
  });
  return makeHostResult.tools;
}

function pngBuffer() {
  return Buffer.from("89504e470d0a1a0a0000000d4948445200000001000000010806000000", "hex");
}

async function uploadEmote(tools, host, { name, meaning, keywords = [], aiEnabled = true, extension = "png", content = null }) {
  const begin = await tools.get("command")({ action: "upload.begin", payload: { extension } });
  const original = content || pngBuffer();
  await tools.get("command")({ action: "upload.chunk", payload: { uploadId: begin.uploadId, kind: "original", data: original.toString("base64") } });
  await tools.get("command")({ action: "upload.chunk", payload: { uploadId: begin.uploadId, kind: "thumbnail", data: original.toString("base64") } });
  await tools.get("command")({ action: "upload.chunk", payload: { uploadId: begin.uploadId, kind: "fallback", data: original.toString("base64") } });
  return tools.get("command")({
    action: "upload.complete",
    payload: {
      uploadId: begin.uploadId,
      name,
      meaning,
      keywords,
      originalFilename: `${name}.${extension}`,
      mimeType: "image/png",
      fileExtension: `.${extension}`,
      fileSize: original.length,
      width: 1,
      height: 1,
      isAnimated: false,
      frameCount: 1,
      aiEnabled,
    },
  });
}

async function maxProbability(tools) {
  await tools.get("command")({
    action: "settings.save",
    payload: { baseProbability: 1, maxProbability: 1, minReplyGap: 0, maxPerHour: 999, sameEmoteCooldownMinutes: 0 },
  });
}

console.log("\n[1] 添加表情包（upload.begin → chunk → complete）");
{
  const env = makeHost();
  const tools = await activate(env);
  const result = await uploadEmote(tools, env.host, { name: "微笑", meaning: "表达开心与友好", keywords: ["开心", "笑"] });
  check("upload.complete 返回 success", result.status === "success");
  check("表情 id 已生成", Boolean(result.item?.id));
  check("含义正确存储", result.item?.meaning === "表达开心与友好");
  check("关键词已规范化", Array.isArray(result.item?.keywords) && result.item.keywords.join(",") === "开心,笑");
  check("默认全角色可用", result.item?.enabled === true && result.item?.aiEnabled === true);
  check("向量已同步 ready", result.item?.vectorStatus === "ready");
  check("assetUrl 已生成", String(result.item?.assetUrl).includes("/api/extension/resources/"));
  check("无 roleScope/characterIds 字段", !("roleScope" in result.item) && !("characterIds" in result.item));

  const list = await tools.get("command")({ action: "emotes.list", payload: {} });
  check("emotes.list 可见 1 条", list.items.length === 1 && list.total === 1);

  const duplicate = await uploadEmote(tools, env.host, { name: "微笑", meaning: "表达开心与友好", keywords: ["开心"] });
  check("重复上传返回 duplicate", duplicate.status === "duplicate" && duplicate.emoteId === result.item.id);
  const afterDup = await tools.get("command")({ action: "emotes.list", payload: {} });
  check("去重后仍只有 1 条", afterDup.items.length === 1);
}

console.log("\n[2] AI 抉择发送表情（output）");
{
  const env = makeHost();
  const tools = await activate(env);
  const smile = await uploadEmote(tools, env.host, { name: "微笑", meaning: "表达开心与友好", keywords: ["开心", "笑"], content: Buffer.from("smile-png") });
  const cry = await uploadEmote(tools, env.host, { name: "流泪", meaning: "表达伤心难过", keywords: ["难过", "哭"], content: Buffer.from("cry-png") });
  await maxProbability(tools);
  env.host.setVectorSearch(() => ({ items: [{ id: smile.item.id, score: 0.9 }] }));

  const result = await tools.get("output")({
    characterId: "char-1",
    conversationId: "conv-1",
    reply: "今天真的太开心啦",
    userMessage: "哈哈",
    source: "ai",
    requestId: "req-1",
  });
  check("AI 抉择输出 1 条", result.outputs.length === 1);
  const part = result.outputs[0]?.part;
  check("输出为 image 类型且 extensionType=emote", part?.type === "image" && part?.extensionType === "emote");
  check("altText 包含表情名", part?.altText === "[表情：微笑]" || part?.altText === "[表情：流泪]");
  check("placement 为 after_text", result.outputs[0]?.placement === "after_text");
  check("sendRecords 已记录 ai_random", env.host.getState().sendRecords.some((r) => r.triggerType === "ai_random" && r.hit === true));

  const suppressed = await tools.get("output")({
    characterId: "char-1",
    conversationId: "conv-1",
    reply: "操作失败了，出现错误",
    source: "ai",
  });
  check("负面语境（错误/失败）被抑制", suppressed.outputs.length === 0);

  env.host.setVectorSearch(() => ({ items: [{ id: smile.item.id, score: 0.2 }] }));
  const lowScore = await tools.get("output")({
    characterId: "char-1",
    conversationId: "conv-1",
    reply: "这是一条普通回复",
    source: "ai",
  });
  check("向量低分候选被拒（<0.35）", lowScore.outputs.length === 0);

  env.host.setVectorSearch(() => ({ items: [{ id: smile.item.id, score: 0.9 }] }));
  await tools.get("command")({ action: "settings.save", payload: { sameEmoteCooldownMinutes: 30 } });
  const cooldownBlocked = await tools.get("output")({
    characterId: "char-1",
    conversationId: "conv-1",
    reply: "又一条开心回复",
    lines: ["又一条开心回复"],
    source: "ai",
  });
  check("同一表情冷却期内不重复发送（默认 30 分钟）", cooldownBlocked.outputs.length === 0);

  const otherCharacter = await tools.get("output")({
    characterId: "char-2",
    conversationId: "conv-1",
    reply: "另一角色的开心回复",
    lines: ["另一角色的开心回复"],
    source: "ai",
  });
  check("冷却按角色隔离，其他角色仍可发送", otherCharacter.outputs.length === 1);
}

console.log("\n[3] AI 抉择约束（频率/开关/概率）");
{
  const env = makeHost();
  const tools = await activate(env);
  const uploaded = [];
  for (const [index] of Array.from({ length: 6 }).entries()) {
    uploaded.push(await uploadEmote(tools, env.host, { name: `表情${index}`, meaning: `第${index}个表情`, content: Buffer.from(`emote-${index}`) }));
  }
  env.host.setVectorSearch(() => ({ items: uploaded.map((item) => ({ id: item.item.id, score: 0.9 })) }));
  await maxProbability(tools);
  await tools.get("command")({ action: "settings.save", payload: { maxPerHour: 2 } });

  await tools.get("output")({ characterId: "c1", conversationId: "cv1", reply: "一", lines: ["一"], source: "ai" });
  await tools.get("output")({ characterId: "c1", conversationId: "cv1", reply: "二", lines: ["二"], source: "ai" });
  const third = await tools.get("output")({ characterId: "c1", conversationId: "cv1", reply: "三", lines: ["三"], source: "ai" });
  check("maxPerHour=2 时第 3 次被限制", third.outputs.length === 0);
  check("前两次确实已发送并计数", env.host.getState().sendRecords.filter((record) => record.characterId === "c1" && record.triggerType === "ai_random").length === 2);

  await tools.get("command")({ action: "settings.save", payload: { enabled: false } });
  const disabled = await tools.get("output")({ characterId: "c2", conversationId: "cv2", reply: "四", lines: ["四"], source: "ai" });
  check("总开关关闭后不发送", disabled.outputs.length === 0);
  await tools.get("command")({ action: "settings.save", payload: { enabled: true } });

  await tools.get("command")({ action: "settings.save", payload: { baseProbability: 0, maxProbability: 0 } });
  const zeroProb = await tools.get("output")({ characterId: "c2", conversationId: "cv2", reply: "五", lines: ["五"], source: "ai" });
  check("概率为 0 时不发送", zeroProb.outputs.length === 0);
}

console.log("\n[4] 编辑 / 批量 / 手动发送");
{
  const env = makeHost();
  const tools = await activate(env);
  const created = await uploadEmote(tools, env.host, { name: "初始", meaning: "", keywords: [], aiEnabled: true });
  check("meaning 为空时 aiEnabled 强制关闭", created.item?.aiEnabled === false);

  const updated = await tools.get("command")({
    action: "emotes.update",
    payload: { id: created.item.id, name: "改名", meaning: "新含义", keywords: ["新词"], aiEnabled: true },
  });
  check("emotes.update 生效", updated.name === "改名" && updated.meaning === "新含义" && updated.aiEnabled === true);

  const batch = await tools.get("command")({ action: "emotes.batch_update", payload: { ids: [created.item.id], update: { aiEnabled: false } } });
  check("emotes.batch_update 返回更新数", batch.updated === 1);
  const list = await tools.get("command")({ action: "emotes.list", payload: {} });
  check("批量更新后 aiEnabled=false", list.items[0].aiEnabled === false);

  const manual = await tools.get("command")({
    action: "emotes.send",
    payload: { emoteId: created.item.id, characterId: "c1", conversationId: "cv1", channel: "web" },
  });
  check("emotes.send 返回消息 id", Array.isArray(manual.messageIds) && manual.messageIds[0] === "message-1");
  check("手动发送记录 triggerType=manual", env.host.getState().sendRecords.some((r) => r.triggerType === "manual"));

  const search = await tools.get("command")({ action: "emotes.list", payload: { q: "改名" } });
  check("搜索命中更新后的名称", search.items.length === 1);
  const searchMiss = await tools.get("command")({ action: "emotes.list", payload: { q: "不存在关键词xyz" } });
  check("搜索无命中返回空", searchMiss.items.length === 0);
}

console.log("\n[5] 分组管理");
{
  const env = makeHost();
  const tools = await activate(env);
  const g1 = await tools.get("command")({ action: "groups.create", payload: { name: "开心组" } });
  const g2 = await tools.get("command")({ action: "groups.create", payload: { name: "伤心组" } });
  const emote = await uploadEmote(tools, env.host, { name: "笑", meaning: "开心", keywords: [] });

  await tools.get("command")({ action: "groups.add", payload: { groupId: g1.id, emoteIds: [emote.item.id] } });
  const inGroup = await tools.get("command")({ action: "emotes.list", payload: { groupId: g1.id } });
  check("groups.add 后分组可见表情", inGroup.items.length === 1);

  const unassigned = await tools.get("command")({ action: "emotes.list", payload: { view: "unassigned" } });
  check("未分组视图排除已分组表情", unassigned.items.length === 0);

  await tools.get("command")({ action: "groups.remove", payload: { groupId: g1.id, emoteId: emote.item.id } });
  const afterRemove = await tools.get("command")({ action: "emotes.list", payload: { groupId: g1.id } });
  check("groups.remove 后分组为空", afterRemove.items.length === 0);

  const renamed = await tools.get("command")({ action: "groups.update", payload: { id: g1.id, name: "改名组", coverEmoteId: emote.item.id } });
  check("groups.update 重命名与封面", renamed.name === "改名组" && renamed.coverEmoteId === emote.item.id);

  await tools.get("command")({ action: "groups.reorder", payload: { ids: [g2.id, g1.id] } });
  const groups = await tools.get("command")({ action: "groups.list", payload: {} });
  check("groups.reorder 生效", groups[0].id === g2.id && groups[1].id === g1.id);

  let addError = "";
  try {
    await tools.get("command")({ action: "groups.add", payload: { groupId: "missing", emoteIds: [emote.item.id] } });
  } catch (error) {
    addError = error.message;
  }
  check("分组不存在时报错", addError === "分组不存在");
}

console.log("\n[6] 删除表情包");
{
  const env = makeHost();
  const tools = await activate(env);
  const g = await tools.get("command")({ action: "groups.create", payload: { name: "组" } });
  const e1 = await uploadEmote(tools, env.host, { name: "待删1", meaning: "删除测试", keywords: [], content: Buffer.from("delete-me") });
  const e2 = await uploadEmote(tools, env.host, { name: "保留", meaning: "保留测试", keywords: [], content: Buffer.from("keep-me") });
  env.host.calls.vectorUpsert.length = 0;

  await tools.get("command")({ action: "emotes.send", payload: { emoteId: e1.item.id, characterId: "c1", conversationId: "cv1" } });
  check("删除前 sendRecords 有记录", env.host.getState().sendRecords.length === 1);

  const removed = await tools.get("command")({ action: "emotes.delete", payload: { ids: [e1.item.id] } });
  check("emotes.delete 返回删除数", removed.deleted === 1);

  const list = await tools.get("command")({ action: "emotes.list", payload: {} });
  check("删除后列表只剩保留项", list.items.length === 1 && list.items[0].id === e2.item.id);
  check("向量点被删除", env.host.calls.vectorDelete.some((call) => call.ids.includes(e1.item.id)));
  check("原文件/缩略图/降级图资源全部删除", env.host.calls.resourceDelete.length >= 3);
  check("sendRecords 同步清理", env.host.getState().sendRecords.length === 0);

  const again = await tools.get("command")({ action: "emotes.delete", payload: { ids: [e1.item.id] } });
  check("重复删除幂等返回 0", again.deleted === 0);

  let missingError = "";
  try {
    await tools.get("command")({ action: "emotes.update", payload: { id: e1.item.id, name: "x" } });
  } catch (error) {
    missingError = error.message;
  }
  check("更新已删除表情报错", missingError === "表情不存在");

  let sendError = "";
  try {
    await tools.get("command")({ action: "emotes.send", payload: { emoteId: e1.item.id, characterId: "c1", conversationId: "cv1" } });
  } catch (error) {
    sendError = error.message;
  }
  check("发送已删除表情报错", sendError === "表情不存在");
}

console.log("\n[7] 状态持久化与并发 CAS");
{
  const env = makeHost();
  const tools = await activate(env);
  const emote = await uploadEmote(tools, env.host, { name: "持久", meaning: "持久化测试", keywords: [] });
  const stored = env.host.getState();
  check("state 已持久化到 store", Boolean(stored) && stored.emotes.length === 1 && Boolean(stored.emotes[0].fileHash));

  const env2 = makeHost();
  env2.host.setState(stored);
  const tools2 = await activate(env2);
  const list = await tools2.get("command")({ action: "emotes.list", payload: {} });
  check("重启后状态可恢复", list.items.length === 1 && list.items[0].id === emote.item.id);
}

console.log(`\n========== 结果：${passed} 通过 / ${failed} 失败 ==========`);
if (failures.length) {
  console.log("失败项：");
  for (const item of failures) console.log(`  - ${item}`);
  process.exitCode = 1;
}
