import { describe, expect, it } from "vitest";
import routerSource from "../router/builtinRoutes.ts?raw";
import managerSource from "../../../plugins/emote/ui/index.html?raw";
import managerScriptSource from "../../../plugins/emote/ui/index.js?raw";
import composerSource from "../../../plugins/emote/ui/composer.html?raw";
import composerScriptSource from "../../../plugins/emote/ui/composer.js?raw";
import messageSource from "../../../plugins/emote/ui/message.html?raw";
import messageScriptSource from "../../../plugins/emote/ui/message.js?raw";
import manifestSource from "../../../plugins/emote/amitia-extension.json?raw";
import inputSource from "../components/ChatInput.vue?raw";
import composerActionHostSource from "../components/extension/chat/ComposerActionExtensionHost.vue?raw";
import composerExtensionHostSource from "../components/extension/chat/ComposerExtensionHost.vue?raw";
import composerActionProxySource from "../components/extension/WebComposerActionProxy.vue?raw";
import sandboxFrameSource from "../components/extension/SandboxWebUIFrame.vue?raw";
import bubbleSource from "../components/ChatBubble.vue?raw";
import sideNavSource from "../components/SideNav.vue?raw";
import navigationSource from "../ui-runtime/navigationRegistry.ts?raw";
import navigationPrewarmSource from "../ui-runtime/navigationPrewarm.ts?raw";
import webChatSource from "../views/web-chat/BuiltinWebChatView.vue?raw";
import settingsSource from "../views/settings/SettingsView.vue?raw";
import { compareChatMessages, normalizeRealtimeMessage } from "../utils/message-order";

describe("表情包前端", () => {
  it("管理页面可进入且包含导入、分组、未分组、多选和批量设置", async () => {
    expect(routerSource).not.toContain('path: "/emotes"');
    expect(managerSource).not.toContain("<h1>表情包管理</h1>");
    expect(managerSource).toContain('class="toolbar"');
    expect(managerSource).toContain("导入文件夹");
    expect(managerSource).toContain("导入表情");
    expect(managerSource).toContain('<script src="./index.js" defer></script>');
    expect(managerSource).not.toContain("<style");
    expect(managerSource).toContain('data-view="unassigned"');
    expect(managerScriptSource).toContain("selectView");
    expect(managerScriptSource).toContain("state.selected");
    expect(managerScriptSource).toContain("emotes.batch_update");
    expect(managerScriptSource).toContain("createGroup");
    expect(managerScriptSource).toContain("groupMenu");
    expect(manifestSource).toContain('"route": "/emotes"');
    expect(manifestSource).toContain('"id": "emote-page"');
  }, 15000);

  it("导入预览、逐项结果、动图预览和详情编辑可用", () => {
    expect(managerSource).toContain("导入预览");
    expect(managerSource).toContain('class="defaults"');
    expect(managerScriptSource).toContain("中文或英文逗号分隔");
    expect(managerScriptSource).toContain("createObjectURL");
    expect(managerScriptSource).toContain("upload.complete");
    expect(managerScriptSource).toContain("item.status");
    expect(managerScriptSource).toContain("item.assetUrl");
    expect(managerScriptSource).toContain("item.thumbnailUrl");
    expect(managerScriptSource).toContain("saveDetail");
    expect(managerScriptSource).toContain("emotes.batch_update");
  });

  it("表情包入口归属角色与记忆且不再出现在设置页签", () => {
    expect(navigationSource).not.toContain('id: "character.emotes"');
    expect(settingsSource).not.toContain("/settings/emotes");
    expect(manifestSource).toContain('"group": "character"');
    expect(manifestSource).toContain('"groupLabel": "角色"');
  });

  it("聊天输入区提供最近、分组和搜索表情面板", () => {
    expect(inputSource).not.toContain("EmotePicker");
    expect(inputSource).toContain("ComposerExtensionHost");
    expect(composerSource).toContain("最近使用");
    expect(composerSource).toContain('<script src="./composer.js" defer></script>');
    expect(composerSource).not.toContain("<style");
    expect(composerScriptSource).toContain('call("groups.list")');
    expect(composerScriptSource).toContain('call("emotes.list"');
    expect(composerScriptSource).toContain("item.thumbnailUrl || item.assetUrl");
    expect(composerScriptSource).toContain('call("emotes.send"');
    expect(composerScriptSource).toContain("let loadPromise = null");
    expect(composerScriptSource).toContain("void load().catch");
    expect(composerScriptSource).toContain("applyHostSurfaceState");
    expect(composerScriptSource).toContain("surfaceMetrics");
    expect(composerScriptSource).toContain("requestResize(32, 32)");
    expect(composerScriptSource).toContain("requestResize(360, 480)");
    expect(manifestSource).toContain('"kind": "composer_action"');
    expect(manifestSource).toContain('"path": "modules/emote-ui/ui/composer.html"');
  });

  it("扩展导航悬停时预热通用沙箱会话", () => {
    expect(sideNavSource).toContain('@mouseenter="prewarmItem(item)"');
    expect(navigationPrewarmSource).toContain("collectEffectiveProviderRoutes");
    expect(navigationPrewarmSource).toContain("prewarmSandboxSession");
    expect(sandboxFrameSource).toContain("getOrCreateSandboxSession");
    expect(sandboxFrameSource).toContain("putCachedSandboxSession");
  });

  it("表情按钮位于加号左侧且使用不撑开输入栏的覆盖层", () => {
    const actionHostIndex = inputSource.indexOf("<ComposerActionExtensionHost");
    const addButtonIndex = inputSource.indexOf("<el-popover");
    expect(actionHostIndex).toBeGreaterThan(-1);
    expect(addButtonIndex).toBeGreaterThan(actionHostIndex);
    expect(composerActionHostSource).toContain('slot-id="chat.composer.action"');
    expect(composerExtensionHostSource).not.toContain('slot-id="chat.composer.action"');
    expect(sandboxFrameSource).toContain("sandbox-webui-frame--overlay");
    expect(sandboxFrameSource).toContain("position: absolute");
    expect(composerActionProxySource).not.toContain("web-composer-action-proxy__button");
    expect(composerActionProxySource).not.toContain("warmed");
    expect(composerActionProxySource).toContain("<SandboxWebUIFrame");
    expect(sandboxFrameSource).toContain("sandboxSessionCache");
  });

  it("实时和历史表情消息使用同一专用渲染组件", () => {
    expect(bubbleSource).not.toContain("EmoteMessage");
    expect(webChatSource).not.toContain("send-emote");
    expect(messageSource).toContain('<script src="./message.js" defer></script>');
    expect(messageSource).not.toContain("<style");
    expect(messageScriptSource).toContain("requestResize");
    expect(messageScriptSource).toContain("originalAssetReference");
    expect(manifestSource).toContain('"kind": "message_renderer"');
    expect(manifestSource).toContain('"path": "modules/emote-ui/ui/message.html"');
  });

  it("文本与 Rich Block 投影按持久 messageSequence 稳定排序", () => {
    const createdAt = "2026-07-18 14:00:00";
    const messages = [
      { id: "after-text", sequence: 13, createdAt },
      { id: "emote", sequence: 12, createdAt },
      { id: "text", sequence: 11, createdAt },
    ].sort(compareChatMessages);
    expect(messages.map((message) => message.id)).toEqual(["text", "emote", "after-text"]);
  });

  it("外部图片消息仅归一化通用媒体字段，不依赖回复分组做占位替换", () => {
    const normalized = normalizeRealtimeMessage({
      messageId: "image-1",
      conversation_id: "conv-1",
      msg_type: "image",
      extension_type: "emote",
      original_asset_reference: "/extension-assets/original.gif",
      created_at: "2026-07-18 14:00:00",
    });
    expect(normalized).toMatchObject({
      id: "image-1",
      conversationId: "conv-1",
      msgType: "image",
      extensionType: "emote",
      originalAssetReference: "/extension-assets/original.gif",
    });
  });

  it("外部实时事件的内容类型和媒体尺寸会统一为前端字段", () => {
    expect(
      normalizeRealtimeMessage({
        content_type: "image",
        media_width: 240,
        media_height: 160,
      }),
    ).toMatchObject({ contentType: "image", width: 240, height: 160 });
  });
});
