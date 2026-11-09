const gameCenterHubExtension = {
  async activate(context) {
    context.log.info("game center hub plugin activated");
  },
  async deactivate() {},
};

if (typeof module !== "undefined" && module.exports) {
  module.exports = gameCenterHubExtension;
}

if (typeof globalThis.defineExtension === "function") {
  globalThis.defineExtension(gameCenterHubExtension);
}
