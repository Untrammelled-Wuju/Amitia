window.__ModuleLoader__.load({
  id: "@amitia-test/child-slot-fixture",
  factory(require) {
    const React = require("react");
    return {
      inject: ["slots"],
      async apply(ctx) {
        ctx.slots.register({
          name: "shell.overlay",
          id: "child-slot-fixture",
          children: {
            "workspace.toast": { kind: "keyed", scope: "root" },
          },
        }, () => React.createElement("div", null, "child slot fixture"));
      },
    };
  },
});
