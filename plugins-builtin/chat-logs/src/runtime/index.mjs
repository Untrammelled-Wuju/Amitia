let activeHost = null;

function asObject(value) {
  return value && typeof value === "object" && !Array.isArray(value) ? value : {};
}

function requireString(value, name) {
  const result = String(value ?? "").trim();
  if (!result) throw new Error(`${name} is required`);
  return result;
}

async function logs(host, method, operation, input = {}) {
  return host.call(method, { operation, input });
}

async function command(host, inputValue) {
  const input = asObject(inputValue);
  const payload = asObject(input.payload);
  switch (String(input.action || "list")) {
    case "characters": {
      const result = asObject(await host.call("host.character.list", { includeDisabled: false }));
      return (Array.isArray(result.items) ? result.items : []).map((item) => ({
        ...asObject(item),
        label: String(asObject(item).name || asObject(item).id || "未命名角色"),
        value: String(asObject(item).id || ""),
      })).filter((item) => item.value);
    }
    case "list":
      return logs(host, "host.conversation_logs.read", "list", {
        roleId: requireString(payload.roleId, "roleId"),
        page: Number(payload.page || 1),
        pageSize: Number(payload.pageSize || 128),
      });
    case "messages":
      return logs(host, "host.conversation_logs.read", "messages", {
        roleId: requireString(payload.roleId, "roleId"),
        conversationId: requireString(payload.conversationId, "conversationId"),
        limit: Number(payload.limit || 500),
      });
    case "restore":
      return logs(host, "host.conversation_logs.manage", "restore", {
        roleId: requireString(payload.roleId, "roleId"),
        conversationId: requireString(payload.conversationId, "conversationId"),
      });
    case "delete":
      return logs(host, "host.conversation_logs.manage", "delete", {
        roleId: requireString(payload.roleId, "roleId"),
        conversationId: requireString(payload.conversationId, "conversationId"),
      });
    case "summary":
      return logs(host, "host.conversation_logs.manage", "summary", {
        roleId: requireString(payload.roleId, "roleId"),
        conversationId: requireString(payload.conversationId, "conversationId"),
      });
    case "export": {
      const result = asObject(await logs(host, "host.conversation_logs.export", "export", {
        roleId: requireString(payload.roleId, "roleId"),
        conversationId: requireString(payload.conversationId, "conversationId"),
      }));
      return {
        ...result,
        download: {
          content: String(result.content || ""),
          filename: String(result.filename || "amitia-conversation.json"),
          mimeType: String(result.mimeType || "application/json; charset=utf-8"),
        },
      };
    }
    default:
      throw new Error(`未知归档对话操作: ${String(input.action)}`);
  }
}

const chatLogsExtension = {
  async activate(context) {
    activeHost = context.host;
    context.handlers.bindTool("command", (handlerInput) => command(activeHost, handlerInput));
    context.log.info("chat logs plugin activated");
  },
  async deactivate() {
    activeHost = null;
  },
};

if (typeof module !== "undefined" && module.exports) {
  module.exports = chatLogsExtension;
}

if (typeof globalThis.defineExtension === "function") {
  globalThis.defineExtension(chatLogsExtension);
}
