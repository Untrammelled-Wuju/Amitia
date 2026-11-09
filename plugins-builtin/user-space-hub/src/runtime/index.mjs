const userSpaceHubExtension = {
  async activate(context) {
    context.log.info("user space hub plugin activated");
  },
  async deactivate() {},
};

if (typeof module !== "undefined" && module.exports) {
  module.exports = userSpaceHubExtension;
}

if (typeof globalThis.defineExtension === "function") {
  globalThis.defineExtension(userSpaceHubExtension);
}
