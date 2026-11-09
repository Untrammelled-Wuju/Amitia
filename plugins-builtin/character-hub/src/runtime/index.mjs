const characterHubExtension = {
  async activate(context) {
    context.log.info("character hub plugin activated");
  },
  async deactivate() {},
};

if (typeof module !== "undefined" && module.exports) {
  module.exports = characterHubExtension;
}

if (typeof globalThis.defineExtension === "function") {
  globalThis.defineExtension(characterHubExtension);
}
