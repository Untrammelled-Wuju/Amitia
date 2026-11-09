let activeHost = null;

function asObject(value) {
  return value && typeof value === "object" && !Array.isArray(value) ? value : {};
}

function requireString(value, name) {
  const result = String(value ?? "").trim();
  if (!result) throw new Error(`${name} is required`);
  return result;
}

async function importAction(host, operation, input = {}) {
  return host.call("host.import.execute", { operation, input });
}

async function command(host, inputValue) {
  const input = asObject(inputValue);
  const payload = asObject(input.payload);
  switch (String(input.action || "batches")) {
    case "characters": {
      const result = asObject(await host.call("host.character.list", { includeDisabled: false }));
      const items = Array.isArray(result.items) ? result.items : [];
      return items.map((item) => ({
        ...asObject(item),
        label: String(asObject(item).name || asObject(item).id || "未命名角色"),
        value: String(asObject(item).id || ""),
      })).filter((item) => item.value);
    }
    case "parse":
      return importAction(host, "parse", {
        rawText: requireString(payload.rawText, "rawText"),
        format: String(payload.format || "auto"),
        defaultRole: String(payload.defaultRole || "user"),
        title: String(payload.title || ""),
        userSpeakerNames: String(payload.userSpeakerNames || "").split(",").map((item) => item.trim()).filter(Boolean),
        assistantSpeakerNames: String(payload.assistantSpeakerNames || "").split(",").map((item) => item.trim()).filter(Boolean),
      });
    case "parse_file": {
      const file = Array.isArray(input.files) ? asObject(input.files[0]) : {};
      const encoded = String(file.dataBase64 || "");
      if (!encoded) throw new Error("请选择要导入的文本文件");
      const bytes = Uint8Array.from(atob(encoded), (char) => char.charCodeAt(0));
      return importAction(host, "parse", {
        rawText: new TextDecoder().decode(bytes),
        format: String(payload.format || "auto"),
        defaultRole: String(payload.defaultRole || "user"),
        title: String(payload.title || String(file.name || "").replace(/\.[^.]+$/, "")),
        userSpeakerNames: String(payload.userSpeakerNames || "").split(",").map((item) => item.trim()).filter(Boolean),
        assistantSpeakerNames: String(payload.assistantSpeakerNames || "").split(",").map((item) => item.trim()).filter(Boolean),
      });
    }
    case "batches":
      return importAction(host, "list_batches");
    case "batch_detail":
      return importAction(host, "batch_detail", { batchId: requireString(payload.batchId, "batchId") });
    case "batch_summary":
      return importAction(host, "batch_summary", { batchId: requireString(payload.batchId, "batchId") });
    case "generate_summary":
      return importAction(host, "generate_summary", { batchId: requireString(payload.batchId, "batchId") });
    case "delete_batch":
      return importAction(host, "delete_batch", { batchId: requireString(payload.batchId, "batchId") });
    case "confirm":
      return importAction(host, "confirm", {
        title: String(payload.title || "已导入的聊天"),
        characterId: requireString(payload.characterId, "characterId"),
        defaultRole: String(payload.defaultRole || "user"),
        detectedFormat: String(payload.detectedFormat || "auto"),
        items: Array.isArray(payload.items) ? payload.items : [],
      });
    case "promote":
      return importAction(host, "promote", {
        batchId: requireString(payload.batchId, "batchId"),
        roleId: String(payload.roleId || ""),
      });
    case "navigate":
      return host.call("host.ui.navigate", { target: requireString(payload.route, "route") });
    default:
      throw new Error(`未知聊天导入操作: ${String(input.action)}`);
  }
}

const chatImportExtension = {
  async activate(context) {
    activeHost = context.host;
    context.handlers.bindTool("command", (input) => command(activeHost, input));
    context.log.info("chat import plugin activated");
  },
  async deactivate() {
    activeHost = null;
  },
};

if (typeof module !== "undefined" && module.exports) {
  module.exports = chatImportExtension;
}

if (typeof globalThis.defineExtension === "function") {
  globalThis.defineExtension(chatImportExtension);
}
