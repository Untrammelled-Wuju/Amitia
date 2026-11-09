const creativeWorkshopHubExtension = {
  async activate(context) {
    context.log.info("creative workshop hub plugin activated");
  },
  async deactivate() {},
};

if (typeof module !== "undefined" && module.exports) {
  module.exports = creativeWorkshopHubExtension;
}

if (typeof globalThis.defineExtension === "function") {
  globalThis.defineExtension(creativeWorkshopHubExtension);
}
