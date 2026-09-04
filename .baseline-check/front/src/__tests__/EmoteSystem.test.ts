import { describe, expect, it } from "vitest";
import { flushPromises, mount } from "@vue/test-utils";
import EmoteMessage from "../components/chat-bubble/EmoteMessage.vue";
import { resetRuntimeConnectionCache } from "../runtime/runtime-adapter";
import routerSource from "../router/index.ts?raw";
import managerSource from "../views/emotes/EmoteManagerView.vue?raw";
import pickerSource from "../components/EmotePicker.vue?raw";
import inputSource from "../components/ChatInput.vue?raw";
import bubbleSource from "../components/ChatBubble.vue?raw";
import webChatSource from "../views/web-chat/WebChatView.vue?raw";
import sideNavSource from "../components/SideNav.vue?raw";
import settingsSource from "../views/settings/SettingsView.vue?raw";
import {
  compareChatMessages,
  mergeChatMessage,
  normalizeRealtimeMessage,
} from "../utils/message-order";

describe("表情包前端", () => {
  it("管理页面可进入且包含导入、分组、未分组、多选和批量设置", async () => {
    expect(routerSource).toContain('path: "/emotes"');
    expect(managerSource).not.toContain("<h1>表情包管理</h1>");
    expect(managerSource).toContain('class="toolbar"');
    expect(managerSource).toContain("导入文件夹");
    expect(managerSource).toContain("导入表情");
    expect(managerSource).toContain('key: "unassigned"');
    expect(managerSource).toContain("selectedIds");
    expect(managerSource).toContain("batch-update");
    expect(managerSource).toContain("createGroup");
    expect(managerSource).toContain("deleteGroup");
    const module = await import("../views/emotes/EmoteManagerView.vue");
    expect(module.default).toBeDefined();
  }, 15000);

  it("导入预览、逐项结果、动图预览和详情编辑可用", () => {
    expect(managerSource).toContain("导入预览");
    expect(managerSource).toContain('class="defaults-row"');
    expect(managerSource).toContain("settings-row");
    expect(managerSource).toContain("中文或英文逗号分隔");
    expect(managerSource).toContain("createObjectURL");
    expect(managerSource).toContain("batch-upload");
    expect(managerSource).toContain("item.status");
    expect(managerSource).toContain(
      "assetUrl(hoveredId === item.id ? item.filePath : item.thumbnailPath)",
    );
    expect(managerSource).toContain("saveDetail");
    expect(managerSource).toContain("roleScope");
  });

  it("表情包入口归属角色与记忆且不再出现在设置页签", () => {
    expect(sideNavSource).toContain('<el-sub-menu index="char-memory">');
    expect(sideNavSource).toContain(
      '<el-menu-item index="/emotes">表情包管理</el-menu-item>',
    );
    expect(settingsSource).not.toContain("/settings/emotes");
    expect(routerSource).toContain(
      '{ path: "/settings/emotes", redirect: "/emotes" }',
    );
  });

  it("聊天输入区提供最近、分组和搜索表情面板", () => {
    expect(inputSource).toContain("EmotePicker");
    expect(pickerSource).toContain("最近使用");
    expect(pickerSource).toContain("/api/emote-groups");
    expect(pickerSource).toContain("/api/emotes");
    expect(pickerSource).toContain("assetUrl(item.thumbnailPath)");
    expect(pickerSource).toContain('emit("select", item)');
  });

  it("实时和历史表情消息使用同一专用渲染组件", () => {
    expect(bubbleSource).toContain("EmoteMessage");
    expect(bubbleSource).toContain('msgType === "emote"');
    expect(webChatSource).toContain("send-emote");
    expect(webChatSource).toContain("emoteId");
  });

  it("表情消息渲染原图、状态和降级文本", async () => {
    (window as any).amitiaDesktop = {
      getDeploymentConfig: async () => ({ mode: "local" }),
    };
    resetRuntimeConnectionCache();
    const wrapper = mount(EmoteMessage, {
      props: {
        message: {
          msgType: "emote",
          content: "[表情：开心]",
          altText: "开心",
          originalAssetReference: "/original.gif",
          fallbackAssetReference: "/fallback.png",
          status: "sending",
          width: 120,
          height: 80,
        },
      },
    });
    await flushPromises();
    expect(wrapper.get("img").attributes("src")).toBe(
      "http://127.0.0.1:18899/original.gif",
    );
    expect(wrapper.text()).toContain("发送中");
    await wrapper.get("img").trigger("error");
    expect(wrapper.get("img").attributes("src")).toBe(
      "http://127.0.0.1:18899/fallback.png",
    );
    await wrapper.get("img").trigger("error");
    expect(wrapper.text()).toContain("开心");
    delete (window as any).amitiaDesktop;
    resetRuntimeConnectionCache();
  });

  it("表情缺少原图时直接使用降级图且兼容下划线字段", async () => {
    (window as any).amitiaDesktop = {
      getDeploymentConfig: async () => ({ mode: "local" }),
    };
    resetRuntimeConnectionCache();
    const wrapper = mount(EmoteMessage, {
      props: {
        message: {
          msg_type: "emote",
          alt_text: "晚安",
          fallback_asset_reference: "/fallback.png",
        },
      },
    });
    await flushPromises();
    expect(wrapper.get("img").attributes("src")).toBe(
      "http://127.0.0.1:18899/fallback.png",
    );
    await wrapper.get("img").trigger("error");
    expect(wrapper.text()).toContain("晚安");
    delete (window as any).amitiaDesktop;
    resetRuntimeConnectionCache();
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

  it("主动消息保留表情字段并能被完整消息补全", () => {
    const proactive = normalizeRealtimeMessage({
      messageId: "emote-1",
      conversationId: "conv-1",
      msg_type: "emote",
      emote_id: "asset-1",
      original_asset_reference: "/emote-assets/original.gif",
      response_group_id: "response-1",
      delivery_sequence: 2,
    });
    expect(proactive).toMatchObject({
      id: "emote-1",
      msgType: "emote",
      emoteId: "asset-1",
      originalAssetReference: "/emote-assets/original.gif",
      responseGroupId: "response-1",
      deliverySequence: 2,
    });

    const messages = [
      {
        id: "emote-1",
        role: "assistant",
        content: "[表情]",
        source: "proactive",
      },
    ];
    expect(mergeChatMessage(messages, proactive)).toBe(true);
    expect(messages[0]).toMatchObject({
      msgType: "emote",
      emoteId: "asset-1",
      source: "proactive",
    });
  });

  it("外部实时事件的内容类型和媒体尺寸会统一为前端字段", () => {
    expect(
      normalizeRealtimeMessage({
        content_type: "emote",
        media_width: 240,
        media_height: 160,
      }),
    ).toMatchObject({ contentType: "emote", width: 240, height: 160 });
  });
});
