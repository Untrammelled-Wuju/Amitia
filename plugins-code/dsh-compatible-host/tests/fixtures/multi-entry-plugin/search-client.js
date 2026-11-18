window.__ModuleLoader__.load({
  id: "@amitia-test/multi-entry-plugin/search",
  factory(require) {
    const React = require("react");
    return {
      inject: ["slots"],
      apply(ctx) {
        ctx.slots.register({
          name: "plugins.detail.actions",
          id: "multi-search-action",
        }, () => React.createElement("button", { type: "button" }, "search"));
      },
    };
  },
});
