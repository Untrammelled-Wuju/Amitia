import assert from "node:assert/strict";
import test from "node:test";
import { mapDshSlot } from "../src/client-abi/slot-map.mjs";

test("maps known DSH UI slots to generic Amitia surfaces", () => {
  assert.equal(mapDshSlot("shell.overlay"), "ui.portal.overlay");
  assert.equal(mapDshSlot("shell.bottom"), "app.shell.bottom");
  assert.equal(mapDshSlot("shell.quota-notice"), "app.shell.quota_notice");
  assert.equal(mapDshSlot("rightbar"), "app.rightbar");
  assert.equal(mapDshSlot("rightbar.session"), "app.rightbar.session");
  assert.equal(mapDshSlot("contextMenu.content"), "desktop.context_menu.content");
  assert.equal(mapDshSlot("contextMenu.icon"), "desktop.context_menu.icon");
  assert.equal(mapDshSlot("deliverables.file.actions"), "workbench.file.actions");
  assert.equal(mapDshSlot("deliverables.review.file.actions"), "workbench.review.file.actions");
  assert.equal(mapDshSlot("settings.models.provider-card"), "settings.model.provider");
  assert.equal(mapDshSlot("settings.models.footer"), "settings.model.footer");
  assert.equal(mapDshSlot("sidebar.settings"), "settings.shell.trigger");
  assert.equal(mapDshSlot("settings.action"), "settings.shell.action");
  assert.equal(mapDshSlot("settings.close"), "settings.shell.close");
  assert.equal(mapDshSlot("settings.header"), "settings.shell.header");
  assert.equal(mapDshSlot("settings.launcher"), "settings.shell.launcher");
  assert.equal(mapDshSlot("settings.launcher.notifications"), "settings.shell.launcher.notifications");
  assert.equal(mapDshSlot("settings.models.sign-in"), "settings.model.sign-in");
  assert.equal(mapDshSlot("settings.onboarding"), "settings.onboarding");
  assert.equal(mapDshSlot("settings.section.icon"), "settings.section.icon");
  assert.equal(mapDshSlot("settings.trigger"), "settings.shell.trigger");
  assert.equal(mapDshSlot("settings.plugins.tab"), "extension.settings.tab");
  assert.equal(mapDshSlot("plugins.bundle.activation"), "extension.plugin.bundle.activation");
  assert.equal(mapDshSlot("plugins.bundle.config"), "extension.plugin.bundle.config");
  assert.equal(mapDshSlot("plugins.row.config"), "extension.plugin.row.config");
  assert.equal(mapDshSlot("plugins.item"), "extension.plugin.item");
  assert.equal(mapDshSlot("plugins.tab"), "extension.plugin.tab");
  assert.equal(mapDshSlot("main.conversation"), "chat.conversation.root");
  assert.equal(mapDshSlot("conversation.composer"), "chat.composer.root");
  assert.equal(mapDshSlot("conversation.composer.bar"), "chat.composer.bar");
  assert.equal(mapDshSlot("conversation.header"), "chat.conversation.header");
  assert.equal(mapDshSlot("conversation.hero.agentPreset"), "chat.empty.agent_preset");
  assert.equal(mapDshSlot("conversation.hero.brand.mark"), "chat.empty.brand_mark");
  assert.equal(mapDshSlot("conversation.hero.frame"), "chat.empty.frame");
  assert.equal(mapDshSlot("conversation.hero.options"), "chat.empty.options");
  assert.equal(mapDshSlot("conversation.hero.workspace"), "chat.empty.workspace");
  assert.equal(mapDshSlot("conversation.hero.workspace.directoryFlow"), "chat.empty.workspace.directory_flow");
  assert.equal(mapDshSlot("conversation.input.attachments"), "chat.composer.attachments");
  assert.equal(mapDshSlot("conversation.session"), "chat.conversation.session");
  assert.equal(mapDshSlot("conversation.session.header"), "chat.conversation.session.header");
  assert.equal(mapDshSlot("conversation.session.header.actions.subagent.footer"), "chat.conversation.subagent.footer");
  assert.equal(mapDshSlot("conversation.view"), "chat.conversation.views");
  assert.equal(mapDshSlot("conversation.approval.detail"), "chat.approval.detail");
  assert.equal(mapDshSlot("conversation.chat.commandview"), "chat.message.command");
  assert.equal(mapDshSlot("conversation.chat.flow"), "chat.conversation.flow");
  assert.equal(mapDshSlot("conversation.chat.markdown.fence"), "chat.message.markdown.fence");
  assert.equal(mapDshSlot("conversation.chat.reasoning.body"), "chat.message.reasoning.body");
  assert.equal(mapDshSlot("conversation.chat.text-interaction"), "chat.message.text.interaction");
  assert.equal(mapDshSlot("conversation.trajectory.images"), "chat.message.trajectory.images");
  assert.equal(mapDshSlot("tool.call.images"), "chat.message.tool.images");
  assert.equal(mapDshSlot("tool.view.cordis"), "chat.message.tool.cordis");
  assert.equal(mapDshSlot("sidebar.right.tab.document"), "ui.sidebar.right.pane.document");
  assert.equal(mapDshSlot("sidebar.right.tab.document.action"), "ui.sidebar.right.pane.document.action");
  assert.equal(mapDshSlot("sidebar.right.tab.document.actions"), "ui.sidebar.right.pane.document.actions");
  assert.equal(mapDshSlot("sidebar.right.tab.document.office.pdf"), "ui.sidebar.right.pane.document.pdf");
  assert.equal(mapDshSlot("sidebar.right.tab.document.unpreviewable"), "ui.sidebar.right.pane.document.unpreviewable");
  assert.equal(mapDshSlot("sidebar.right.tab.files.actions"), "ui.sidebar.right.pane.files.actions");
  assert.equal(mapDshSlot("sidebar.right.tab.guide"), "ui.sidebar.right.pane.guide");
  assert.equal(mapDshSlot("sidebar.right.tab.guide.actions"), "ui.sidebar.right.pane.guide.actions");
  assert.equal(mapDshSlot("sidebar.right.tab.guide.entry"), "ui.sidebar.right.pane.guide.entry");
  assert.equal(mapDshSlot("sidebar.right.tab.menu.item"), "ui.sidebar.right.pane.tab.menu.item");
  assert.equal(mapDshSlot("plugins.detail.actions"), "extension.detail.action");
  assert.equal(mapDshSlot("conversation.input.activity"), "chat.composer.action");
  assert.equal(mapDshSlot("tool.call.toolview"), "chat.message.custom_renderer");
  assert.equal(mapDshSlot("sidebar.right.pane.tab"), "ui.sidebar.right.pane.tab");
  assert.equal(mapDshSlot("sidebar.right.pane.tab.title"), "ui.sidebar.right.pane.tab.title");
  assert.equal(mapDshSlot("conversation.input.dock"), "chat.composer.dock.top");
  assert.equal(mapDshSlot("conversation.composer.dock"), "chat.composer.dock.bottom");
  assert.equal(mapDshSlot("conversation.input.left"), "chat.composer.left");
  assert.equal(mapDshSlot("conversation.input.right"), "chat.composer.right");
  assert.equal(mapDshSlot("conversation.input.model"), "chat.composer.model");
  assert.equal(mapDshSlot("conversation.input.model.label"), "chat.composer.model.label");
  assert.equal(mapDshSlot("conversation.input.model.options"), "chat.composer.model.options");
  assert.equal(mapDshSlot("conversation.input.overlay"), "chat.composer.overlay");
  assert.equal(mapDshSlot("conversation.input.permission"), "chat.composer.permission");
  assert.equal(mapDshSlot("conversation.chat.message.images"), "chat.message.images");
  assert.equal(mapDshSlot("conversation.chat.user.images"), "chat.message.user.images");
  assert.equal(mapDshSlot("conversation.header.leading"), "chat.header.leading");
  assert.equal(mapDshSlot("conversation.session.header.lineage"), "chat.header.lineage");
  assert.equal(mapDshSlot("conversation.session.header.corner"), "chat.header.corner");
});

test("uses catalog mappings for additional DSH surfaces", () => {
  assert.equal(mapDshSlot("root"), "root");
  assert.equal(mapDshSlot("settings.general.item"), "system.settings.section");
});

test("fails explicitly for unknown and unavailable targets", () => {
  assert.throws(
    () => mapDshSlot("shell.overlay", () => false),
    /maps to unavailable Amitia slot/,
  );
  assert.throws(
    () => mapDshSlot("unclassified.surface"),
    /is not in the 0.2.2-rc.2 catalog/,
  );
  assert.equal(mapDshSlot("conversation.chat.flow"), "chat.conversation.flow");
});
