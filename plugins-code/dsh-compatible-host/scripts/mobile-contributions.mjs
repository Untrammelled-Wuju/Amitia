const supportedKinds = new Map([
  ["app.shell.quota_notice", ["status_item", "panel"]],
  ["app.shell.bottom", ["panel", "status_item"]],
  ["ui.portal.overlay", ["panel", "schema_page", "web_page"]],
  ["ui.portal.modal", ["panel", "schema_page", "web_page"]],
  ["ui.portal.drawer", ["panel", "schema_page", "web_page"]],
  ["ui.portal.toast", ["status_item", "panel", "schema_page"]],
  ["chat.header.action", ["action", "toolbar_item"]],
  ["chat.header.leading", ["action", "toolbar_item", "panel"]],
  ["chat.header.lineage", ["panel", "schema_page", "web_page"]],
  ["chat.header.corner", ["action", "toolbar_item", "panel"]],
  ["chat.sidebar.panel", ["panel", "schema_page"]],
  ["provider.app.workspace.sidebar", ["panel", "schema_page", "web_page"]],
  ["provider.conversation.messages", ["panel", "schema_page", "web_page"]],
  ["chat.message.action", ["message_action", "action"]],
  ["chat.message.renderer", ["message_renderer"]],
  ["chat.message.images", ["message_renderer"]],
  ["chat.message.user.images", ["message_renderer"]],
  ["chat.message.custom_renderer", ["message_renderer"]],
  ["chat.message.attachment_renderer", ["message_renderer"]],
  ["chat.message.badge", ["badge", "status_item", "card"]],
  ["chat.composer.action", ["composer_action", "action"]],
  ["chat.composer.dock.top", ["panel", "schema_page", "web_page", "status_item"]],
  ["chat.composer.dock.bottom", ["panel", "schema_page", "web_page", "status_item"]],
  ["chat.composer.left", ["composer_action", "action"]],
  ["chat.composer.right", ["composer_action", "action"]],
  ["chat.composer.overlay", ["panel", "schema_page", "web_page"]],
  ["chat.composer.model", ["composer_action", "action", "panel"]],
  ["chat.composer.model.label", ["status_item", "panel"]],
  ["chat.composer.model.options", ["composer_action", "action", "menu_item"]],
  ["chat.composer.permission", ["composer_action", "action", "panel"]],
  ["chat.conversation.node", ["message_renderer"]],
  ["extension.center.header.action", ["action", "menu_item"]],
  ["extension.center.card.badge", ["card", "badge"]],
  ["extension.detail.action", ["action"]],
  ["extension.detail.tab", ["schema_page", "web_page"]],
  ["extension.settings.section", ["settings_section", "schema_page", "panel"]],
  ["system.settings.section", ["settings_section", "schema_page"]],
  ["extension.settings.page", ["schema_page", "web_page"]],
  ["ui.sidebar.right.pane.tab", ["panel", "schema_page", "web_page"]],
  ["ui.sidebar.right.pane.tab.title", ["panel", "status_item"]],
]);

export function applyMobileContributionActions(manifest) {
  for (const module of manifest.modules ?? []) {
    for (const contribution of module.contributions ?? []) {
      if (!contribution.id?.startsWith("dsh-mobile-")) continue;
      const spec = contribution.spec ?? {};
      if (spec.entry?.type !== "mobile_web_runtime") continue;
      const slotId = spec.slot?.slot_id;
      const kinds = supportedKinds.get(slotId);
      if (!kinds || !kinds.includes(spec.kind)) {
        throw new Error(`DSH mobile slot ${slotId} does not support contribution kind ${spec.kind}`);
      }
      contribution.kind = "ui_panel";
      spec.actions = [
        {
          action_id: "command",
          title: { default: "执行 DSH 容器命令" },
          target: {
            type: "tool",
            tool_id: "com.amitia/dsh-compatible-host/command",
          },
          risk_level: "high",
        },
      ];
      contribution.spec = spec;
      contribution.requiredPermissions = ["tool.invoke"];
    }
  }
  return manifest;
}
