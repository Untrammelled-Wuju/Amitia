window.__ModuleLoader__.load({
  id: "@amitia-test/dsh-fixture",
  factory(require) {
    const React = require("react");
    return {
      inject: ["slots"],
      apply(ctx) {
        ctx.slots.register({
          name: "shell.overlay",
          id: "fixture-overlay",
          order: 10,
        }, () => React.createElement("button", { type: "button", "data-dsh-fixture": "overlay" }, "DSH Fixture"));
      },
    };
  },
});
