let activeHost = null;

function asObject(value) {
  return value && typeof value === "object" && !Array.isArray(value) ? value : {};
}

function requireString(value, name) {
  const result = String(value ?? "").trim();
  if (!result) throw new Error(`${name} is required`);
  return result;
}

async function storage(host, method, operation, input = {}) {
  return host.call(method, { operation, input });
}

async function command(host, inputValue) {
  const input = asObject(inputValue);
  const payload = asObject(input.payload);
  switch (String(input.action || "overview")) {
    case "characters": {
      const result = asObject(await host.call("host.character.list", { includeDisabled: false }));
      return (Array.isArray(result.items) ? result.items : []).map((item) => ({
        ...asObject(item),
        label: String(asObject(item).name || asObject(item).id || "未命名角色"),
        value: String(asObject(item).id || ""),
      })).filter((item) => item.value);
    }
    case "overview":
      return {
        info: await storage(host, "host.storage.cleanup", "info"),
        backups: await storage(host, "host.storage.backup", "list"),
      };
    case "stats":
      return storage(host, "host.storage.cleanup", "stats", { roleId: requireString(payload.roleId, "roleId") });
    case "vacuum":
      return storage(host, "host.storage.cleanup", "vacuum", { roleId: requireString(payload.roleId, "roleId") });
    case "cleanup_preview":
      return storage(host, "host.storage.cleanup", "preview", {
        roleId: requireString(payload.roleId, "roleId"),
        beforeDate: String(payload.beforeDate || ""),
        olderThanDays: Number(payload.olderThanDays || 0),
        includeMemories: payload.includeMemories === true,
        channels: Array.isArray(payload.channels) ? payload.channels : [],
        sources: Array.isArray(payload.sources) ? payload.sources : [],
      });
    case "cleanup_confirm":
      return storage(host, "host.storage.cleanup", "confirm", {
        roleId: requireString(payload.roleId, "roleId"),
        previewId: requireString(payload.previewId, "previewId"),
        confirmText: requireString(payload.confirmText, "confirmText"),
      });
    case "list_backups":
      return storage(host, "host.storage.backup", "list");
    case "create_backup":
      return storage(host, "host.storage.backup", "create", { passphrase: String(payload.passphrase || "") });
    case "delete_backup":
      return storage(host, "host.storage.backup", "delete", { name: requireString(payload.name, "name") });
    case "preview_restore":
      return storage(host, "host.storage.restore", "preview", {
        name: requireString(payload.name, "name"),
        passphrase: String(payload.passphrase || ""),
      });
    case "execute_restore":
      return storage(host, "host.storage.restore", "execute", {
        operationId: requireString(payload.operationId, "operationId"),
      });
    case "cancel_restore":
      return storage(host, "host.storage.restore", "cancel");
    default:
      throw new Error(`未知存储操作: ${String(input.action)}`);
  }
}

const storageExtension = {
  async activate(context) {
    activeHost = context.host;
    context.handlers.bindTool("command", (handlerInput) => command(activeHost, handlerInput));
    context.log.info("storage plugin activated");
  },
  async deactivate() {
    activeHost = null;
  },
};

if (typeof module !== "undefined" && module.exports) {
  module.exports = storageExtension;
}

if (typeof globalThis.defineExtension === "function") {
  globalThis.defineExtension(storageExtension);
}
