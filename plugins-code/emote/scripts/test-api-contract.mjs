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

function makeHost() {
  const store = new Map();
  const tools = new Map();
  const resources = new Map();
  const handles = new Map();
  const calls = { resourceDelete: [], messageAppend: [] };
  let version = 0;
  const host = {
    calls,
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
  check("不再产生向量状态字段", !("vectorStatus" in result.item) && !("vectorError" in result.item));
  check("assetUrl 已生成", String(result.item?.assetUrl).includes("/api/extension/resources/"));
  check("无 roleScope/characterIds 字段", !("roleScope" in result.item) && !("characterIds" in result.item));

  const list = await tools.get("command")({ action: "emotes.list", payload: {} });
  check("emotes.list 可见 1 条", list.items.length === 1 && list.total === 1);

  const duplicate = await uploadEmote(tools, env.host, { name: "微笑", meaning: "表达开心与友好", keywords: ["开心"] });
  check("重复上传返回 duplicate", duplicate.status === "duplicate" && duplicate.emoteId === result.item.id);
  const afterDup = await tools.get("command")({ action: "emotes.list", payload: {} });
  check("去重后仍只有 1 条", afterDup.items.length === 1);
}

console.log("\n[2] AI 直接获取参数并自主发送");
{
  const env = makeHost();
  const tools = await activate(env);
  const smile = await uploadEmote(tools, env.host, { name: "微笑", meaning: "表达开心与友好", keywords: ["开心", "笑"], content: Buffer.from("smile-png") });
  const hidden = await uploadEmote(tools, env.host, { name: "隐藏", meaning: "不应提供给 AI", keywords: ["隐藏"], aiEnabled: false, content: Buffer.from("hidden-png") });
  await tools.get("command")({ action: "settings.save", payload: { maxPerHour: 10, sameEmoteCooldownMinutes: 0 } });

  const catalog = await tools.get("ai")({ action: "list", characterId: "char-1" });
  check("AI 工具可读取表情参数", catalog.emotes.length === 1 && catalog.emotes[0].id === smile.item.id);
  check("参数包含名称含义关键词", catalog.emotes[0].name === "微笑" && catalog.emotes[0].meaning === "表达开心与友好" && catalog.emotes[0].keywords.join(",") === "开心,笑");
  check("AI 未启用表情不会暴露", !catalog.emotes.some((item) => item.id === hidden.item.id));

  const sent = await tools.get("ai")({
    action: "send",
    emoteId: smile.item.id,
    characterId: "char-1",
    conversationId: "conv-1",
    channel: "qq_personal",
    reason: "当前语境适合微笑",
    requestId: "req-ai-1",
  });
  check("AI 自主发送返回 message id", sent.status === "sent" && sent.messageIds[0] === "message-1");
  const append = env.host.calls.messageAppend.at(-1);
  check("发送内容为标准 emote image", append.parts[0].type === "image" && append.parts[0].extensionType === "emote");
  check("AI 发送记录使用 ai_tool", env.host.getState().sendRecords.some((record) => record.triggerType === "ai_tool" && record.reason === "当前语境适合微笑"));

  const contextual = await tools.get("ai")({
    action: "send",
    emoteId: smile.item.id,
    __amitiaContext: {
      characterId: "char-context",
      conversationId: "conv-context",
      channel: "qq_personal",
      spaceId: "space-context",
      operationId: "op-context",
    },
  });
  const contextualAppend = env.host.calls.messageAppend.at(-1);
  check("AI 发送自动继承宿主会话上下文", contextual.status === "sent" && contextualAppend.conversationId === "conv-context" && contextualAppend.channel === "qq_personal" && contextualAppend.characterId === "char-context" && contextualAppend.spaceId === "space-context");

  await tools.get("command")({ action: "settings.save", payload: { sameEmoteCooldownMinutes: 30 } });
  let cooldownError = "";
  try {
    await tools.get("ai")({ action: "send", emoteId: smile.item.id, characterId: "char-1", conversationId: "conv-1" });
  } catch (error) {
    cooldownError = error.message;
  }
  check("AI 发送遵守同表情冷却", cooldownError === "该表情仍在冷却中");

  await tools.get("command")({ action: "settings.save", payload: { sameEmoteCooldownMinutes: 0, maxPerHour: 1 } });
  let limitError = "";
  try {
    await tools.get("ai")({ action: "send", emoteId: smile.item.id, characterId: "char-1", conversationId: "conv-1" });
  } catch (error) {
    limitError = error.message;
  }
  check("AI 发送遵守每小时上限", limitError === "本小时表情发送次数已达上限");

  await tools.get("command")({ action: "settings.save", payload: { enabled: false } });
  let disabledError = "";
  try {
    await tools.get("ai")({ action: "send", emoteId: smile.item.id, characterId: "char-2", conversationId: "conv-2" });
  } catch (error) {
    disabledError = error.message;
  }
  check("AI 发送遵守总开关", disabledError === "表情发送已关闭");
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
  check("表情消息使用 assistant 出站角色", env.host.calls.messageAppend.at(-1)?.role === "assistant");

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
  await tools.get("command")({ action: "emotes.send", payload: { emoteId: e1.item.id, characterId: "c1", conversationId: "cv1" } });
  check("删除前 sendRecords 有记录", env.host.getState().sendRecords.length === 1);

  const removed = await tools.get("command")({ action: "emotes.delete", payload: { ids: [e1.item.id] } });
  check("emotes.delete 返回删除数", removed.deleted === 1);

  const list = await tools.get("command")({ action: "emotes.list", payload: {} });
  check("删除后列表只剩保留项", list.items.length === 1 && list.items[0].id === e2.item.id);
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

console.log("\n[8] Schema UI 文件导入契约");
{
  const env = makeHost();
  const tools = await activate(env);
  const content = pngBuffer();
  const input = {
    action: "ui.import",
    node_id: "import-files",
    form_state: {},
    files: [{ name: "契约.png", mimeType: "image/png", size: content.length, dataBase64: content.toString("base64") }],
  };
  const imported = await tools.get("command")(input);
  check("完整 UI 文件字段完成导入", imported.imported === 1 && imported.total === 1 && imported.duplicate === 0);
  const list = await tools.get("command")({ action: "ui.composer", node_id: "lifecycle-onMount", form_state: {} });
  check("导入表情可在 composer 读取", list.emotes.length === 1 && list.emotes[0].originalFilename === "契约.png");
  check("composer 默认进入未分组", list.activeGroupId === "__ungrouped__" && list.form_state.groupId === "__ungrouped__" && list.groups[0].name === "未分组");
  check("composer 分组栏保留最近使用和自定义分组", list.groups[1].name === "最近使用" && Array.isArray(list.groups));
  const item = list.emotes[0];
  check("文件名称类型大小保留", item.name === "契约" && item.mimeType === "image/png" && item.fileSize === content.length);
  check("原图及衍生图保留完整字节", [item.filePath, item.thumbnailPath, item.fallbackPath].every((path) => env.resources.get(path)?.equals(content)));
  const repeated = await tools.get("command")(input);
  check("重复 UI 文件导入去重", repeated.imported === 0 && repeated.duplicate === 1 && env.host.getState().emotes.length === 1);
}

console.log("\n[9] 分组浏览与组内导入");
{
  const env = makeHost();
  const tools = await activate(env);
  const run = (action, payload = {}, extra = {}) => tools.get("command")({ action, payload, ...extra });
  const group = await run("groups.create", { name: "常用" });
  const other = await run("groups.create", { name: "其他" });
  const dialog = await run("ui.group_dialog", { open: true });
  check("新建分组弹窗打开时清空名称", dialog.form_state.groupDialog.open === true && dialog.form_state.group.name === "");
  const content = pngBuffer();
  const files = [{ name: "组内.png", mimeType: "image/png", size: content.length, dataBase64: content.toString("base64") }];
  await run("ui.import", { groupId: group.id }, { files });
  let dashboard = await run("ui.dashboard");
  check("组内导入直接关联当前分组", dashboard.emotes.length === 1 && dashboard.emotes[0].groupIds.includes(group.id));
  const composer = await run("ui.composer");
  check("composer 未分组不混入自定义分组表情", composer.emotes.length === 0 && composer.groups.some((entry) => entry.id === group.id && entry.name === "常用"));
  const composerGroup = await run("ui.composer", { groupId: group.id });
  check("composer 可切换自定义分组", composerGroup.emotes.length === 1 && composerGroup.form_state.groupId === group.id && composerGroup.groups[0].id === "__ungrouped__");
  check("分组返回数量和拼贴封面", dashboard.groups[0].count === 1 && dashboard.groups[0].covers.length === 1 && dashboard.groups[1].count === 0);
  const selected = await run("ui.select_group", { id: group.id });
  check("进入分组保留名称并清空图片选中", selected.form_state.browser.groupId === group.id && selected.form_state.browser.groupName === "常用" && selected.form_state.selected.id === "");
  dashboard = await run("ui.dashboard", {}, { form_state: selected.form_state });
  check("刷新后保留当前分组", dashboard.form_state.browser.groupId === group.id);
  const duplicate = await run("ui.import", { groupId: other.id }, { files });
  dashboard = await run("ui.dashboard");
  check("重复图片导入另一分组建立关联且不重复创建", duplicate.duplicate === 1 && dashboard.emotes.length === 1 && dashboard.groups.every((item) => item.count === 1));
  await run("groups.remove", { groupId: group.id, emoteId: dashboard.emotes[0].id });
  dashboard = await run("ui.dashboard");
  check("移出分组保留原图及其他分组关联", dashboard.emotes.length === 1 && dashboard.groups[0].count === 0 && dashboard.groups[1].count === 1);
  const back = await run("ui.select_group", { id: "" });
  check("返回分组概览清空当前分组", back.form_state.browser.groupId === "");
  let error = "";
  try { await run("ui.select_group", { id: "missing" }); } catch (failure) { error = failure.message; }
  check("失效分组返回明确错误", error === "分组不存在");
  await run("groups.delete", { id: other.id });
  dashboard = await run("ui.dashboard");
  check("删除分组保留图片并清理分组关联", dashboard.emotes.length === 1 && !dashboard.groups.some((item) => item.id === other.id) && !dashboard.emotes[0].groupIds.includes(other.id));
}

console.log(`\n========== 结果：${passed} 通过 / ${failed} 失败 ==========`);
if (failures.length) {
  console.log("失败项：");
  for (const item of failures) console.log(`  - ${item}`);
  process.exitCode = 1;
}
