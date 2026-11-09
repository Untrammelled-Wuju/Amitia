const memoryHubExtension = {
  async activate(context) {
    context.log.info("memory hub plugin activated");
  },
  async deactivate() {},
};

if (typeof module !== "undefined" && module.exports) {
  module.exports = memoryHubExtension;
}

if (typeof globalThis.defineExtension === "function") {
  globalThis.defineExtension(memoryHubExtension);
}
