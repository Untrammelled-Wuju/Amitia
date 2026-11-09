const channelsHubExtension = {
  async activate(context) {
    context.log.info("channels hub plugin activated");
  },
  async deactivate() {},
};

if (typeof module !== "undefined" && module.exports) {
  module.exports = channelsHubExtension;
}

if (typeof globalThis.defineExtension === "function") {
  globalThis.defineExtension(channelsHubExtension);
}
