window.__ModuleLoader__.load({
  id: "@amitia-test/complex-client-plugin",
  factory(require) {
    const React = require("react");
    return {
      inject: ["slots", "locale"],
      async apply(ctx) {
        ctx.locale.register("complexFixture", {
          zh: { action: "复杂夹具操作", badge: "复杂夹具徽章" },
          en: { action: "Complex fixture action", badge: "Complex fixture badge" },
        });
        ctx.slots.register({
          name: "plugins.detail.actions",
          id: "complex-action",
          order: 20,
          locale: "complexFixture",
          store: { count: 0 },
          inject: () => ({ label: "injected" }),
        }, ({ t }) => React.createElement("button", { type: "button" }, t("action")));
        ctx.slots.register({
          name: "plugins.detail.badge",
          id: "complex-badge",
          order: 30,
          locale: "complexFixture",
        }, ({ t }) => React.createElement("span", null, t("badge")));
        ctx.slots.register({
          name: "sidebar.session.row.trailing",
          id: "complex-sidebar",
          order: 40,
        }, () => React.createElement("span", null, "sidebar"));
        if (ctx.configForms) {
          const form = ctx.configForms.get("complexFixture.settings");
          await form.set("value", "ready");
          ctx.slots.register({
            name: "settings.general.item",
            id: "complex-settings",
            order: 50,
          }, () => React.createElement("span", null, `config-${form.getSnapshot().value.value}`));
        }
        ctx.effect(() => () => {
          globalThis.__complexClientDisposed = (globalThis.__complexClientDisposed ?? 0) + 1;
        });
      },
    };
  },
});
