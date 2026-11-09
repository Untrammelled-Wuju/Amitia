let activeHost = null;

async function command(host, inputValue) {
  const input = inputValue && typeof inputValue === "object" ? inputValue : {};
  switch (String(input.action || "snapshot")) {
    case "snapshot":
      return host.call("host.runtime.snapshot", {});
    case "maintenance.status":
      return host.call("host.runtime.maintenance.status", {});
    case "maintenance.overview":
      return host.call("host.runtime.maintenance.overview", {});
    case "maintenance.config":
      return host.call("host.runtime.maintenance.config", {});
    case "maintenance.update":
      return host.call("host.runtime.maintenance.update", input.payload || {});
    case "maintenance.cleanup-temp":
      return host.call("host.runtime.cleanup_temp", {});
    case "maintenance.rotate-logs":
      return host.call("host.runtime.rotate_logs", {});
    case "maintenance.check-db":
      return host.call("host.runtime.check_db_integrity", {});
    default:
      throw new Error(`未知运行时观察操作: ${String(input.action)}`);
  }
}

const runtimeInsightsExtension = {
  async activate(context) {
    activeHost = context.host;
    context.handlers.bindTool("command", (input) => command(activeHost, input));
    context.log.info("runtime insights plugin activated");
  },
  async deactivate() {
    activeHost = null;
  },
};

if (typeof module !== "undefined" && module.exports) {
  module.exports = runtimeInsightsExtension;
}

if (typeof globalThis.defineExtension === "function") {
  globalThis.defineExtension(runtimeInsightsExtension);
}
