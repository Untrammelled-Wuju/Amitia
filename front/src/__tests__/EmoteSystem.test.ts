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
import bubbleSource from "../components/ChatBubble.vue?raw";
import navigationSource from "../ui-runtime/navigationRegistry.ts?raw";
import webChatSource from "../views/web-chat/BuiltinWebChatView.vue?raw";
import settingsSource from "../views/settings/SettingsView.vue?raw";
import {
  compareChatMessages,
  mergeChatMessage,
  normalizeRealtimeMessage,
} from "../utils/message-order";

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
    expect(managerScriptSource).toContain("roleScope");
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
    expect(manifestSource).toContain('"kind": "composer_action"');
    expect(manifestSource).toContain('"path": "modules/emote-ui/ui/composer.html"');
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

  it("同一回复组严格按消息计划顺序显示", () => {
    const createdAt = "2026-07-18 14:00:00";
    const messages = [
      {
        id: "text-2",
        responseGroupId: "response-1",
        deliverySequence: 3,
        sequence: 13,
        createdAt,
      },
      {
        id: "emote",
        responseGroupId: "response-1",
        deliverySequence: 2,
        sequence: 12,
        createdAt,
      },
      {
        id: "text-1",
        responseGroupId: "response-1",
        deliverySequence: 1,
        sequence: 11,
        createdAt,
      },
    ].sort(compareChatMessages);
    expect(messages.map((message) => message.id)).toEqual([
      "text-1",
      "emote",
      "text-2",
    ]);
  });

  it("外部实时图片消息保留通用媒体字段并能被完整消息补全", () => {
    const proactive = normalizeRealtimeMessage({
      messageId: "image-1",
      conversationId: "conv-1",
      msg_type: "image",
      extension_type: "emote",
      original_asset_reference: "/extension-assets/original.gif",
      response_group_id: "response-1",
      delivery_sequence: 2,
    });
    expect(proactive).toMatchObject({
      id: "image-1",
      msgType: "image",
      extensionType: "emote",
      originalAssetReference: "/extension-assets/original.gif",
      responseGroupId: "response-1",
      deliverySequence: 2,
    });

    const messages = [
      {
        id: "image-1",
        role: "assistant",
        content: "[图片]",
        source: "proactive",
      },
    ];
    expect(mergeChatMessage(messages, proactive)).toBe(true);
    expect(messages[0]).toMatchObject({
      msgType: "image",
      extensionType: "emote",
      source: "proactive",
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
