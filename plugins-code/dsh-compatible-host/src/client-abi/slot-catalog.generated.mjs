export const DSH_UI_SLOT_RESOLUTIONS = Object.freeze({
  "contextMenu.content": {
    "id": "DSH-UI-SLOT-001",
    "key": "contextMenu.content",
    "kind": "keyed",
    "scope": "root",
    "status": "composite",
    "phase": "P12",
    "targets": [
      "desktop.context_menu.content"
    ],
    "reason": ""
  },
  "contextMenu.icon": {
    "id": "DSH-UI-SLOT-002",
    "key": "contextMenu.icon",
    "kind": "keyed",
    "scope": "root",
    "status": "composite",
    "phase": "P12",
    "targets": [
      "desktop.context_menu.icon"
    ],
    "reason": ""
  },
  "conversation.approval.detail": {
    "id": "DSH-UI-SLOT-003",
    "key": "conversation.approval.detail",
    "kind": "single",
    "scope": "session",
    "status": "composite",
    "phase": "P12",
    "targets": [
      "chat.approval.detail"
    ],
    "reason": ""
  },
  "conversation.chat.assistant-actions": {
    "id": "DSH-UI-SLOT-004",
    "key": "conversation.chat.assistant-actions",
    "kind": "list",
    "scope": "session",
    "status": "direct",
    "phase": "P10",
    "targets": [
      "chat.message.action"
    ],
    "reason": ""
  },
  "conversation.chat.commandview": {
    "id": "DSH-UI-SLOT-005",
    "key": "conversation.chat.commandview",
    "kind": "keyed",
    "scope": "session",
    "status": "composite",
    "phase": "P12",
    "targets": [
      "chat.message.command"
    ],
    "reason": ""
  },
  "conversation.chat.flow": {
    "id": "DSH-UI-SLOT-006",
    "key": "conversation.chat.flow",
    "kind": "single",
    "scope": "session",
    "status": "composite",
    "phase": "P12",
    "targets": [
      "chat.conversation.flow"
    ],
    "reason": ""
  },
  "conversation.chat.markdown.fence": {
    "id": "DSH-UI-SLOT-007",
    "key": "conversation.chat.markdown.fence",
    "kind": "hoc",
    "scope": "session",
    "status": "composite",
    "phase": "P12",
    "targets": [
      "chat.message.markdown.fence"
    ],
    "reason": ""
  },
  "conversation.chat.message.images": {
    "id": "DSH-UI-SLOT-008",
    "key": "conversation.chat.message.images",
    "kind": "single",
    "scope": "session",
    "status": "composite",
    "phase": "P12",
    "targets": [
      "chat.message.images"
    ],
    "reason": ""
  },
  "conversation.chat.node": {
    "id": "DSH-UI-SLOT-009",
    "key": "conversation.chat.node",
    "kind": "keyed",
    "scope": "session",
    "status": "direct",
    "phase": "P10",
    "targets": [
      "chat.conversation.node"
    ],
    "reason": ""
  },
  "conversation.chat.reasoning.body": {
    "id": "DSH-UI-SLOT-010",
    "key": "conversation.chat.reasoning.body",
    "kind": "single",
    "scope": "session",
    "status": "composite",
    "phase": "P12",
    "targets": [
      "chat.message.reasoning.body"
    ],
    "reason": ""
  },
  "conversation.chat.text-interaction": {
    "id": "DSH-UI-SLOT-011",
    "key": "conversation.chat.text-interaction",
    "kind": "keyed",
    "scope": "session",
    "status": "composite",
    "phase": "P12",
    "targets": [
      "chat.message.text.interaction"
    ],
    "reason": ""
  },
  "conversation.chat.turnTail": {
    "id": "DSH-UI-SLOT-012",
    "key": "conversation.chat.turnTail",
    "kind": "list",
    "scope": "session",
    "status": "direct",
    "phase": "P10",
    "targets": [
      "chat.message.badge"
    ],
    "reason": ""
  },
  "conversation.chat.user.before": {
    "id": "DSH-UI-SLOT-013",
    "key": "conversation.chat.user.before",
    "kind": "chain",
    "scope": "session",
    "status": "direct",
    "phase": "P10",
    "targets": [
      "chat.message.badge"
    ],
    "reason": ""
  },
  "conversation.chat.user.images": {
    "id": "DSH-UI-SLOT-014",
    "key": "conversation.chat.user.images",
    "kind": "single",
    "scope": "session",
    "status": "composite",
    "phase": "P12",
    "targets": [
      "chat.message.user.images"
    ],
    "reason": ""
  },
  "conversation.composer": {
    "id": "DSH-UI-SLOT-015",
    "key": "conversation.composer",
    "kind": "chain",
    "scope": "session",
    "status": "composite",
    "phase": "P12",
    "targets": [
      "chat.composer.root"
    ],
    "reason": ""
  },
  "conversation.composer.bar": {
    "id": "DSH-UI-SLOT-016",
    "key": "conversation.composer.bar",
    "kind": "single",
    "scope": "session-maybe",
    "status": "composite",
    "phase": "P12",
    "targets": [
      "chat.composer.bar"
    ],
    "reason": ""
  },
  "conversation.composer.dock": {
    "id": "DSH-UI-SLOT-017",
    "key": "conversation.composer.dock",
    "kind": "list",
    "scope": "session",
    "status": "composite",
    "phase": "P12",
    "targets": [
      "chat.composer.dock.bottom"
    ],
    "reason": ""
  },
  "conversation.header": {
    "id": "DSH-UI-SLOT-018",
    "key": "conversation.header",
    "kind": "single",
    "scope": "session-maybe",
    "status": "composite",
    "phase": "P12",
    "targets": [
      "chat.conversation.header"
    ],
    "reason": ""
  },
  "conversation.header.leading": {
    "id": "DSH-UI-SLOT-019",
    "key": "conversation.header.leading",
    "kind": "single",
    "scope": "root",
    "status": "direct",
    "phase": "P12",
    "targets": [
      "chat.header.leading"
    ],
    "reason": ""
  },
  "conversation.hero.agentPreset": {
    "id": "DSH-UI-SLOT-020",
    "key": "conversation.hero.agentPreset",
    "kind": "single",
    "scope": "session-maybe",
    "status": "composite",
    "phase": "P12",
    "targets": [
      "chat.empty.agent_preset"
    ],
    "reason": ""
  },
  "conversation.hero.brand.mark": {
    "id": "DSH-UI-SLOT-021",
    "key": "conversation.hero.brand.mark",
    "kind": "single",
    "scope": "root",
    "status": "composite",
    "phase": "P12",
    "targets": [
      "chat.empty.brand_mark"
    ],
    "reason": ""
  },
  "conversation.hero.frame": {
    "id": "DSH-UI-SLOT-022",
    "key": "conversation.hero.frame",
    "kind": "hoc",
    "scope": "root",
    "status": "composite",
    "phase": "P12",
    "targets": [
      "chat.empty.frame"
    ],
    "reason": ""
  },
  "conversation.hero.options": {
    "id": "DSH-UI-SLOT-023",
    "key": "conversation.hero.options",
    "kind": "list",
    "scope": "session",
    "status": "composite",
    "phase": "P12",
    "targets": [
      "chat.empty.options"
    ],
    "reason": ""
  },
  "conversation.hero.workspace": {
    "id": "DSH-UI-SLOT-024",
    "key": "conversation.hero.workspace",
    "kind": "single",
    "scope": "root",
    "status": "composite",
    "phase": "P12",
    "targets": [
      "chat.empty.workspace"
    ],
    "reason": ""
  },
  "conversation.hero.workspace.directoryFlow": {
    "id": "DSH-UI-SLOT-025",
    "key": "conversation.hero.workspace.directoryFlow",
    "kind": "single",
    "scope": "root",
    "status": "composite",
    "phase": "P12",
    "targets": [
      "chat.empty.workspace.directory_flow"
    ],
    "reason": ""
  },
  "conversation.input.activity": {
    "id": "DSH-UI-SLOT-026",
    "key": "conversation.input.activity",
    "kind": "single",
    "scope": "session",
    "status": "direct",
    "phase": "P10",
    "targets": [
      "chat.composer.action"
    ],
    "reason": ""
  },
  "conversation.input.attachments": {
    "id": "DSH-UI-SLOT-027",
    "key": "conversation.input.attachments",
    "kind": "single",
    "scope": "session-maybe",
    "status": "composite",
    "phase": "P12",
    "targets": [
      "chat.composer.attachments"
    ],
    "reason": ""
  },
  "conversation.input.dock": {
    "id": "DSH-UI-SLOT-028",
    "key": "conversation.input.dock",
    "kind": "list",
    "scope": "session",
    "status": "composite",
    "phase": "P12",
    "targets": [
      "chat.composer.dock.top"
    ],
    "reason": ""
  },
  "conversation.input.left": {
    "id": "DSH-UI-SLOT-029",
    "key": "conversation.input.left",
    "kind": "list",
    "scope": "session",
    "status": "composite",
    "phase": "P12",
    "targets": [
      "chat.composer.left"
    ],
    "reason": ""
  },
  "conversation.input.model": {
    "id": "DSH-UI-SLOT-030",
    "key": "conversation.input.model",
    "kind": "single",
    "scope": "session",
    "status": "composite",
    "phase": "P12",
    "targets": [
      "chat.composer.model"
    ],
    "reason": ""
  },
  "conversation.input.model.label": {
    "id": "DSH-UI-SLOT-031",
    "key": "conversation.input.model.label",
    "kind": "single",
    "scope": "session",
    "status": "composite",
    "phase": "P12",
    "targets": [
      "chat.composer.model.label"
    ],
    "reason": ""
  },
  "conversation.input.model.options": {
    "id": "DSH-UI-SLOT-032",
    "key": "conversation.input.model.options",
    "kind": "list",
    "scope": "session",
    "status": "composite",
    "phase": "P12",
    "targets": [
      "chat.composer.model.options"
    ],
    "reason": ""
  },
  "conversation.input.overlay": {
    "id": "DSH-UI-SLOT-033",
    "key": "conversation.input.overlay",
    "kind": "list",
    "scope": "session",
    "status": "composite",
    "phase": "P12",
    "targets": [
      "chat.composer.overlay"
    ],
    "reason": ""
  },
  "conversation.input.permission": {
    "id": "DSH-UI-SLOT-034",
    "key": "conversation.input.permission",
    "kind": "single",
    "scope": "session",
    "status": "composite",
    "phase": "P12",
    "targets": [
      "chat.composer.permission"
    ],
    "reason": ""
  },
  "conversation.input.plan": {
    "id": "DSH-UI-SLOT-035",
    "key": "conversation.input.plan",
    "kind": "single",
    "scope": "session",
    "status": "direct",
    "phase": "P12",
    "targets": [
      "chat.composer.action"
    ],
    "reason": ""
  },
  "conversation.input.right": {
    "id": "DSH-UI-SLOT-036",
    "key": "conversation.input.right",
    "kind": "list",
    "scope": "session",
    "status": "composite",
    "phase": "P12",
    "targets": [
      "chat.composer.right"
    ],
    "reason": ""
  },
  "conversation.plan-review.actions": {
    "id": "DSH-UI-SLOT-037",
    "key": "conversation.plan-review.actions",
    "kind": "list",
    "scope": "session",
    "status": "direct",
    "phase": "P10",
    "targets": [
      "chat.message.action"
    ],
    "reason": ""
  },
  "conversation.session": {
    "id": "DSH-UI-SLOT-038",
    "key": "conversation.session",
    "kind": "single",
    "scope": "session",
    "status": "composite",
    "phase": "P12",
    "targets": [
      "chat.conversation.session"
    ],
    "reason": ""
  },
  "conversation.session.header": {
    "id": "DSH-UI-SLOT-039",
    "key": "conversation.session.header",
    "kind": "single",
    "scope": "session",
    "status": "composite",
    "phase": "P12",
    "targets": [
      "chat.conversation.session.header"
    ],
    "reason": ""
  },
  "conversation.session.header.actions": {
    "id": "DSH-UI-SLOT-040",
    "key": "conversation.session.header.actions",
    "kind": "list",
    "scope": "session",
    "status": "direct",
    "phase": "P10",
    "targets": [
      "chat.header.action"
    ],
    "reason": ""
  },
  "conversation.session.header.actions.subagent.footer": {
    "id": "DSH-UI-SLOT-041",
    "key": "conversation.session.header.actions.subagent.footer",
    "kind": "list",
    "scope": "session",
    "status": "composite",
    "phase": "P12",
    "targets": [
      "chat.conversation.subagent.footer"
    ],
    "reason": ""
  },
  "conversation.session.header.corner": {
    "id": "DSH-UI-SLOT-042",
    "key": "conversation.session.header.corner",
    "kind": "single",
    "scope": "session",
    "status": "direct",
    "phase": "P12",
    "targets": [
      "chat.header.corner"
    ],
    "reason": ""
  },
  "conversation.session.header.lineage": {
    "id": "DSH-UI-SLOT-043",
    "key": "conversation.session.header.lineage",
    "kind": "single",
    "scope": "session",
    "status": "composite",
    "phase": "P12",
    "targets": [
      "chat.header.lineage"
    ],
    "reason": ""
  },
  "conversation.session.header.utilities": {
    "id": "DSH-UI-SLOT-044",
    "key": "conversation.session.header.utilities",
    "kind": "list",
    "scope": "session",
    "status": "composite",
    "phase": "P12",
    "targets": [
      "chat.header.action"
    ],
    "reason": ""
  },
  "conversation.session.header.views.actions": {
    "id": "DSH-UI-SLOT-045",
    "key": "conversation.session.header.views.actions",
    "kind": "list",
    "scope": "root",
    "status": "direct",
    "phase": "P10",
    "targets": [
      "chat.header.action"
    ],
    "reason": ""
  },
  "conversation.trajectory.images": {
    "id": "DSH-UI-SLOT-046",
    "key": "conversation.trajectory.images",
    "kind": "single",
    "scope": "session",
    "status": "composite",
    "phase": "P12",
    "targets": [
      "chat.message.trajectory.images"
    ],
    "reason": ""
  },
  "conversation.user.text.node": {
    "id": "DSH-UI-SLOT-047",
    "key": "conversation.user.text.node",
    "kind": "keyed",
    "scope": "session",
    "status": "composite",
    "phase": "P12",
    "targets": [
      "chat.message.user.text"
    ],
    "reason": ""
  },
  "conversation.view": {
    "id": "DSH-UI-SLOT-048",
    "key": "conversation.view",
    "kind": "list",
    "scope": "session",
    "status": "composite",
    "phase": "P12",
    "targets": [
      "chat.conversation.views"
    ],
    "reason": ""
  },
  "deliverables.file.actions": {
    "id": "DSH-UI-SLOT-049",
    "key": "deliverables.file.actions",
    "kind": "list",
    "scope": "session",
    "status": "composite",
    "phase": "P12",
    "targets": [
      "workbench.file.actions"
    ],
    "reason": ""
  },
  "deliverables.review.file.actions": {
    "id": "DSH-UI-SLOT-050",
    "key": "deliverables.review.file.actions",
    "kind": "list",
    "scope": "session",
    "status": "composite",
    "phase": "P12",
    "targets": [
      "workbench.review.file.actions"
    ],
    "reason": ""
  },
  "main": {
    "id": "DSH-UI-SLOT-051",
    "key": "main",
    "kind": "keyed",
    "scope": "root",
    "status": "composite",
    "phase": "P12",
    "targets": [
      "provider.page.provider"
    ],
    "reason": ""
  },
  "main.conversation": {
    "id": "DSH-UI-SLOT-052",
    "key": "main.conversation",
    "kind": "single",
    "scope": "session-maybe",
    "status": "composite",
    "phase": "P12",
    "targets": [
      "chat.conversation.root"
    ],
    "reason": ""
  },
  "plugins.add.actions": {
    "id": "DSH-UI-SLOT-053",
    "key": "plugins.add.actions",
    "kind": "list",
    "scope": "root",
    "status": "direct",
    "phase": "P10",
    "targets": [
      "extension.center.header.action"
    ],
    "reason": ""
  },
  "plugins.bundle.activation": {
    "id": "DSH-UI-SLOT-054",
    "key": "plugins.bundle.activation",
    "kind": "keyed",
    "scope": "root",
    "status": "composite",
    "phase": "P12",
    "targets": [
      "extension.plugin.bundle.activation"
    ],
    "reason": ""
  },
  "plugins.bundle.config": {
    "id": "DSH-UI-SLOT-055",
    "key": "plugins.bundle.config",
    "kind": "keyed",
    "scope": "root",
    "status": "composite",
    "phase": "P12",
    "targets": [
      "extension.plugin.bundle.config"
    ],
    "reason": ""
  },
  "plugins.detail.actions": {
    "id": "DSH-UI-SLOT-056",
    "key": "plugins.detail.actions",
    "kind": "list",
    "scope": "root",
    "status": "direct",
    "phase": "P10",
    "targets": [
      "extension.detail.action"
    ],
    "reason": ""
  },
  "plugins.detail.badge": {
    "id": "DSH-UI-SLOT-057",
    "key": "plugins.detail.badge",
    "kind": "list",
    "scope": "root",
    "status": "direct",
    "phase": "P10",
    "targets": [
      "extension.center.card.badge"
    ],
    "reason": ""
  },
  "plugins.detail.section": {
    "id": "DSH-UI-SLOT-058",
    "key": "plugins.detail.section",
    "kind": "list",
    "scope": "root",
    "status": "composite",
    "phase": "P10",
    "targets": [
      "extension.detail.tab"
    ],
    "reason": ""
  },
  "plugins.item": {
    "id": "DSH-UI-SLOT-059",
    "key": "plugins.item",
    "kind": "list",
    "scope": "root",
    "status": "composite",
    "phase": "P12",
    "targets": [
      "extension.plugin.item"
    ],
    "reason": ""
  },
  "plugins.row.config": {
    "id": "DSH-UI-SLOT-060",
    "key": "plugins.row.config",
    "kind": "keyed",
    "scope": "root",
    "status": "composite",
    "phase": "P12",
    "targets": [
      "extension.plugin.row.config"
    ],
    "reason": ""
  },
  "plugins.tab": {
    "id": "DSH-UI-SLOT-061",
    "key": "plugins.tab",
    "kind": "list",
    "scope": "root",
    "status": "composite",
    "phase": "P12",
    "targets": [
      "extension.plugin.tab"
    ],
    "reason": ""
  },
  "rightbar": {
    "id": "DSH-UI-SLOT-062",
    "key": "rightbar",
    "kind": "single",
    "scope": "root",
    "status": "composite",
    "phase": "P12",
    "targets": [
      "app.rightbar"
    ],
    "reason": ""
  },
  "rightbar.session": {
    "id": "DSH-UI-SLOT-063",
    "key": "rightbar.session",
    "kind": "single",
    "scope": "session",
    "status": "composite",
    "phase": "P12",
    "targets": [
      "app.rightbar.session"
    ],
    "reason": ""
  },
  "root": {
    "id": "DSH-UI-SLOT-064",
    "key": "root",
    "kind": "single",
    "scope": "root",
    "status": "direct",
    "phase": "P10",
    "targets": [
      "root"
    ],
    "reason": ""
  },
  "settings.action": {
    "id": "DSH-UI-SLOT-065",
    "key": "settings.action",
    "kind": "list",
    "scope": "root",
    "status": "composite",
    "phase": "P12",
    "targets": [
      "settings.shell.action"
    ],
    "reason": ""
  },
  "settings.close": {
    "id": "DSH-UI-SLOT-066",
    "key": "settings.close",
    "kind": "single",
    "scope": "root",
    "status": "composite",
    "phase": "P12",
    "targets": [
      "settings.shell.close"
    ],
    "reason": ""
  },
  "settings.general.item": {
    "id": "DSH-UI-SLOT-067",
    "key": "settings.general.item",
    "kind": "list",
    "scope": "root",
    "status": "composite",
    "phase": "P10",
    "targets": [
      "system.settings.section"
    ],
    "reason": ""
  },
  "settings.header": {
    "id": "DSH-UI-SLOT-068",
    "key": "settings.header",
    "kind": "single",
    "scope": "root",
    "status": "composite",
    "phase": "P12",
    "targets": [
      "settings.shell.header"
    ],
    "reason": ""
  },
  "settings.launcher": {
    "id": "DSH-UI-SLOT-069",
    "key": "settings.launcher",
    "kind": "single",
    "scope": "root",
    "status": "composite",
    "phase": "P12",
    "targets": [
      "settings.shell.launcher"
    ],
    "reason": ""
  },
  "settings.launcher.notifications": {
    "id": "DSH-UI-SLOT-070",
    "key": "settings.launcher.notifications",
    "kind": "single",
    "scope": "root",
    "status": "composite",
    "phase": "P12",
    "targets": [
      "settings.shell.launcher.notifications"
    ],
    "reason": ""
  },
  "settings.models.footer": {
    "id": "DSH-UI-SLOT-071",
    "key": "settings.models.footer",
    "kind": "list",
    "scope": "root",
    "status": "composite",
    "phase": "P12",
    "targets": [
      "settings.model.footer"
    ],
    "reason": ""
  },
  "settings.models.provider-card": {
    "id": "DSH-UI-SLOT-072",
    "key": "settings.models.provider-card",
    "kind": "keyed",
    "scope": "root",
    "status": "composite",
    "phase": "P12",
    "targets": [
      "settings.model.provider"
    ],
    "reason": ""
  },
  "settings.models.sign-in": {
    "id": "DSH-UI-SLOT-073",
    "key": "settings.models.sign-in",
    "kind": "single",
    "scope": "root",
    "status": "composite",
    "phase": "P12",
    "targets": [
      "settings.model.sign-in"
    ],
    "reason": ""
  },
  "settings.onboarding": {
    "id": "DSH-UI-SLOT-074",
    "key": "settings.onboarding",
    "kind": "list",
    "scope": "root",
    "status": "composite",
    "phase": "P12",
    "targets": [
      "settings.onboarding"
    ],
    "reason": ""
  },
  "settings.plugins.tab": {
    "id": "DSH-UI-SLOT-075",
    "key": "settings.plugins.tab",
    "kind": "list",
    "scope": "root",
    "status": "composite",
    "phase": "P12",
    "targets": [
      "extension.settings.tab"
    ],
    "reason": ""
  },
  "settings.section": {
    "id": "DSH-UI-SLOT-076",
    "key": "settings.section",
    "kind": "list",
    "scope": "root",
    "status": "direct",
    "phase": "P10",
    "targets": [
      "extension.settings.section"
    ],
    "reason": ""
  },
  "settings.section.icon": {
    "id": "DSH-UI-SLOT-077",
    "key": "settings.section.icon",
    "kind": "list",
    "scope": "root",
    "status": "composite",
    "phase": "P12",
    "targets": [
      "settings.section.icon"
    ],
    "reason": ""
  },
  "settings.trigger": {
    "id": "DSH-UI-SLOT-078",
    "key": "settings.trigger",
    "kind": "single",
    "scope": "root",
    "status": "composite",
    "phase": "P12",
    "targets": [
      "settings.shell.trigger"
    ],
    "reason": ""
  },
  "shell.bottom": {
    "id": "DSH-UI-SLOT-079",
    "key": "shell.bottom",
    "kind": "single",
    "scope": "root",
    "status": "composite",
    "phase": "P12",
    "targets": [
      "app.shell.bottom"
    ],
    "reason": ""
  },
  "shell.leading": {
    "id": "DSH-UI-SLOT-080",
    "key": "shell.leading",
    "kind": "single",
    "scope": "root",
    "status": "composite",
    "phase": "P10",
    "targets": [
      "chat.header.action"
    ],
    "reason": ""
  },
  "shell.overlay": {
    "id": "DSH-UI-SLOT-081",
    "key": "shell.overlay",
    "kind": "list",
    "scope": "root",
    "status": "direct",
    "phase": "P10",
    "targets": [
      "ui.portal.overlay"
    ],
    "reason": ""
  },
  "shell.quota-notice": {
    "id": "DSH-UI-SLOT-082",
    "key": "shell.quota-notice",
    "kind": "chain",
    "scope": "root",
    "status": "composite",
    "phase": "P12",
    "targets": [
      "app.shell.quota_notice"
    ],
    "reason": ""
  },
  "sidebar": {
    "id": "DSH-UI-SLOT-083",
    "key": "sidebar",
    "kind": "single",
    "scope": "root",
    "status": "composite",
    "phase": "P10",
    "targets": [
      "provider.app.workspace.sidebar"
    ],
    "reason": ""
  },
  "sidebar.brand.mark": {
    "id": "DSH-UI-SLOT-084",
    "key": "sidebar.brand.mark",
    "kind": "single",
    "scope": "root",
    "status": "composite",
    "phase": "P12",
    "targets": [
      "provider.app.workspace.sidebar"
    ],
    "reason": ""
  },
  "sidebar.brand.name": {
    "id": "DSH-UI-SLOT-085",
    "key": "sidebar.brand.name",
    "kind": "single",
    "scope": "root",
    "status": "composite",
    "phase": "P12",
    "targets": [
      "provider.app.workspace.sidebar"
    ],
    "reason": ""
  },
  "sidebar.chat.conversation": {
    "id": "DSH-UI-SLOT-086",
    "key": "sidebar.chat.conversation",
    "kind": "single",
    "scope": "session",
    "status": "composite",
    "phase": "P12",
    "targets": [
      "provider.app.workspace.sidebar"
    ],
    "reason": ""
  },
  "sidebar.footer.action": {
    "id": "DSH-UI-SLOT-087",
    "key": "sidebar.footer.action",
    "kind": "list",
    "scope": "root",
    "status": "composite",
    "phase": "P12",
    "targets": [
      "provider.app.workspace.sidebar"
    ],
    "reason": ""
  },
  "sidebar.panellist": {
    "id": "DSH-UI-SLOT-088",
    "key": "sidebar.panellist",
    "kind": "list",
    "scope": "root",
    "status": "composite",
    "phase": "P12",
    "targets": [
      "chat.sidebar.panel"
    ],
    "reason": ""
  },
  "sidebar.panellist.actions": {
    "id": "DSH-UI-SLOT-089",
    "key": "sidebar.panellist.actions",
    "kind": "list",
    "scope": "root",
    "status": "composite",
    "phase": "P12",
    "targets": [
      "chat.sidebar.panel"
    ],
    "reason": ""
  },
  "sidebar.right.pane.tab": {
    "id": "DSH-UI-SLOT-090",
    "key": "sidebar.right.pane.tab",
    "kind": "keyed",
    "scope": "session",
    "status": "direct",
    "phase": "P10",
    "targets": [
      "ui.sidebar.right.pane.tab"
    ],
    "reason": ""
  },
  "sidebar.right.pane.tab.title": {
    "id": "DSH-UI-SLOT-091",
    "key": "sidebar.right.pane.tab.title",
    "kind": "keyed",
    "scope": "session",
    "status": "direct",
    "phase": "P10",
    "targets": [
      "ui.sidebar.right.pane.tab.title"
    ],
    "reason": ""
  },
  "sidebar.right.tab.document": {
    "id": "DSH-UI-SLOT-092",
    "key": "sidebar.right.tab.document",
    "kind": "keyed",
    "scope": "session",
    "status": "composite",
    "phase": "P12",
    "targets": [
      "ui.sidebar.right.pane.document"
    ],
    "reason": ""
  },
  "sidebar.right.tab.document.action": {
    "id": "DSH-UI-SLOT-093",
    "key": "sidebar.right.tab.document.action",
    "kind": "keyed",
    "scope": "session",
    "status": "composite",
    "phase": "P12",
    "targets": [
      "ui.sidebar.right.pane.document.action"
    ],
    "reason": ""
  },
  "sidebar.right.tab.document.actions": {
    "id": "DSH-UI-SLOT-094",
    "key": "sidebar.right.tab.document.actions",
    "kind": "list",
    "scope": "session",
    "status": "composite",
    "phase": "P12",
    "targets": [
      "ui.sidebar.right.pane.document.actions"
    ],
    "reason": ""
  },
  "sidebar.right.tab.document.office.pdf": {
    "id": "DSH-UI-SLOT-095",
    "key": "sidebar.right.tab.document.office.pdf",
    "kind": "keyed",
    "scope": "session",
    "status": "composite",
    "phase": "P12",
    "targets": [
      "ui.sidebar.right.pane.document.pdf"
    ],
    "reason": ""
  },
  "sidebar.right.tab.document.unpreviewable": {
    "id": "DSH-UI-SLOT-096",
    "key": "sidebar.right.tab.document.unpreviewable",
    "kind": "list",
    "scope": "session",
    "status": "composite",
    "phase": "P12",
    "targets": [
      "ui.sidebar.right.pane.document.unpreviewable"
    ],
    "reason": ""
  },
  "sidebar.right.tab.files.actions": {
    "id": "DSH-UI-SLOT-097",
    "key": "sidebar.right.tab.files.actions",
    "kind": "list",
    "scope": "session",
    "status": "composite",
    "phase": "P12",
    "targets": [
      "ui.sidebar.right.pane.files.actions"
    ],
    "reason": ""
  },
  "sidebar.right.tab.guide": {
    "id": "DSH-UI-SLOT-098",
    "key": "sidebar.right.tab.guide",
    "kind": "chain",
    "scope": "session",
    "status": "composite",
    "phase": "P12",
    "targets": [
      "ui.sidebar.right.pane.guide"
    ],
    "reason": ""
  },
  "sidebar.right.tab.guide.actions": {
    "id": "DSH-UI-SLOT-099",
    "key": "sidebar.right.tab.guide.actions",
    "kind": "list",
    "scope": "root",
    "status": "composite",
    "phase": "P12",
    "targets": [
      "ui.sidebar.right.pane.guide.actions"
    ],
    "reason": ""
  },
  "sidebar.right.tab.guide.entry": {
    "id": "DSH-UI-SLOT-100",
    "key": "sidebar.right.tab.guide.entry",
    "kind": "keyed",
    "scope": "session",
    "status": "composite",
    "phase": "P12",
    "targets": [
      "ui.sidebar.right.pane.guide.entry"
    ],
    "reason": ""
  },
  "sidebar.right.tab.menu.item": {
    "id": "DSH-UI-SLOT-101",
    "key": "sidebar.right.tab.menu.item",
    "kind": "list",
    "scope": "session",
    "status": "composite",
    "phase": "P12",
    "targets": [
      "ui.sidebar.right.pane.tab.menu.item"
    ],
    "reason": ""
  },
  "sidebar.session.row.hover": {
    "id": "DSH-UI-SLOT-102",
    "key": "sidebar.session.row.hover",
    "kind": "list",
    "scope": "root",
    "status": "composite",
    "phase": "P12",
    "targets": [
      "chat.sidebar.panel"
    ],
    "reason": ""
  },
  "sidebar.session.row.leading": {
    "id": "DSH-UI-SLOT-103",
    "key": "sidebar.session.row.leading",
    "kind": "list",
    "scope": "root",
    "status": "composite",
    "phase": "P12",
    "targets": [
      "chat.sidebar.panel"
    ],
    "reason": ""
  },
  "sidebar.session.row.trailing": {
    "id": "DSH-UI-SLOT-104",
    "key": "sidebar.session.row.trailing",
    "kind": "list",
    "scope": "root",
    "status": "composite",
    "phase": "P10",
    "targets": [
      "chat.sidebar.panel"
    ],
    "reason": ""
  },
  "sidebar.settings": {
    "id": "DSH-UI-SLOT-105",
    "key": "sidebar.settings",
    "kind": "single",
    "scope": "root",
    "status": "composite",
    "phase": "P12",
    "targets": [
      "settings.shell.trigger"
    ],
    "reason": ""
  },
  "sidebar.toggle.badge": {
    "id": "DSH-UI-SLOT-106",
    "key": "sidebar.toggle.badge",
    "kind": "single",
    "scope": "root",
    "status": "composite",
    "phase": "P12",
    "targets": [
      "chat.header.action"
    ],
    "reason": ""
  },
  "sidebar.workspaces": {
    "id": "DSH-UI-SLOT-107",
    "key": "sidebar.workspaces",
    "kind": "single",
    "scope": "root",
    "status": "composite",
    "phase": "P12",
    "targets": [
      "provider.app.workspace.sidebar"
    ],
    "reason": ""
  },
  "sidebar.workspaces.directoryFlow": {
    "id": "DSH-UI-SLOT-108",
    "key": "sidebar.workspaces.directoryFlow",
    "kind": "single",
    "scope": "root",
    "status": "composite",
    "phase": "P12",
    "targets": [
      "provider.app.workspace.sidebar"
    ],
    "reason": ""
  },
  "sidebar.workspaces.session.menu.item": {
    "id": "DSH-UI-SLOT-109",
    "key": "sidebar.workspaces.session.menu.item",
    "kind": "list",
    "scope": "root",
    "status": "composite",
    "phase": "P10",
    "targets": [
      "chat.header.action"
    ],
    "reason": ""
  },
  "sidebar.workspaces.session.row.action": {
    "id": "DSH-UI-SLOT-110",
    "key": "sidebar.workspaces.session.row.action",
    "kind": "list",
    "scope": "root",
    "status": "composite",
    "phase": "P12",
    "targets": [
      "chat.header.action"
    ],
    "reason": ""
  },
  "tool.call.images": {
    "id": "DSH-UI-SLOT-111",
    "key": "tool.call.images",
    "kind": "single",
    "scope": "session",
    "status": "composite",
    "phase": "P12",
    "targets": [
      "chat.message.tool.images"
    ],
    "reason": ""
  },
  "tool.call.toolview": {
    "id": "DSH-UI-SLOT-112",
    "key": "tool.call.toolview",
    "kind": "keyed",
    "scope": "session",
    "status": "direct",
    "phase": "P10",
    "targets": [
      "chat.message.custom_renderer"
    ],
    "reason": ""
  },
  "tool.view.cordis": {
    "id": "DSH-UI-SLOT-113",
    "key": "tool.view.cordis",
    "kind": "keyed",
    "scope": "session",
    "status": "composite",
    "phase": "P12",
    "targets": [
      "chat.message.tool.cordis"
    ],
    "reason": ""
  },
  "workspace.toast": {
    "id": "DSH-UI-SLOT-114",
    "key": "workspace.toast",
    "kind": "keyed",
    "scope": "root",
    "status": "composite",
    "phase": "P12",
    "targets": [
      "app.workspace.toast"
    ],
    "reason": ""
  },
  "conversation.chat.reasoning.content": {
    "id": "DSH-UI-SLOT-115",
    "key": "conversation.chat.reasoning.content",
    "kind": "factory",
    "scope": "session",
    "status": "composite",
    "phase": "P10",
    "targets": [
      "chat.message.renderer"
    ],
    "reason": ""
  },
  "conversation.chat.user.content": {
    "id": "DSH-UI-SLOT-116",
    "key": "conversation.chat.user.content",
    "kind": "factory",
    "scope": "session",
    "status": "composite",
    "phase": "P10",
    "targets": [
      "chat.message.renderer"
    ],
    "reason": ""
  },
  "conversation.content": {
    "id": "DSH-UI-SLOT-117",
    "key": "conversation.content",
    "kind": "factory",
    "scope": "session-maybe",
    "status": "composite",
    "phase": "P10",
    "targets": [
      "provider.conversation.messages"
    ],
    "reason": ""
  },
  "conversation.images.content": {
    "id": "DSH-UI-SLOT-118",
    "key": "conversation.images.content",
    "kind": "factory",
    "scope": "session",
    "status": "composite",
    "phase": "P10",
    "targets": [
      "chat.message.attachment_renderer"
    ],
    "reason": ""
  },
  "conversation.user.text": {
    "id": "DSH-UI-SLOT-119",
    "key": "conversation.user.text",
    "kind": "factory",
    "scope": "session",
    "status": "composite",
    "phase": "P10",
    "targets": [
      "chat.message.renderer"
    ],
    "reason": ""
  },
  "sidebar.panel.icon": {
    "id": "DSH-UI-SLOT-120",
    "key": "sidebar.panel.icon",
    "kind": "factory",
    "scope": "root",
    "status": "composite",
    "phase": "P12",
    "targets": [
      "chat.sidebar.panel"
    ],
    "reason": ""
  }
});
