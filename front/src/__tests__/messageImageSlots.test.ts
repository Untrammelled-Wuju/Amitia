import { mount } from "@vue/test-utils";
import { describe, expect, it } from "vitest";
import MessageImageSlot from "../conversation/rendering/MessageImageSlot.vue";
import { normalizeAIMessage } from "../conversation/rendering/amrp";

function mountSlot(role: "assistant" | "user") {
  const message = normalizeAIMessage({
    id: `message-${role}`,
    conversationId: "conversation-a",
    role,
    status: "sent",
  });
  return mount(MessageImageSlot, {
    props: {
      images: [
        {
          kind: "image",
          id: "image-a",
          url: "/image-a.png",
          alt: "图片甲",
          status: "ready",
        },
      ],
      message,
      extensionContext: {
        characterId: "role-a",
        conversationId: "conversation-a",
      },
    },
    global: {
      stubs: {
        ExtensionSlot: {
          props: ["slotId", "context"],
          template:
            '<div :data-slot-id="slotId"><span data-slot-context>{{ JSON.stringify(context) }}</span><slot /></div>',
        },
        AmitiaImageBlock: {
          props: ["images"],
          template: '<div data-image-fallback="true"></div>',
        },
      },
    },
  });
}

describe("message image slots", () => {
  it.each([
    ["assistant", "chat.message.images", "incoming", "character"],
    ["user", "chat.message.user.images", "outgoing", "user"],
  ] as const)("routes %s images to %s", (role, slotId, direction, senderType) => {
    const wrapper = mountSlot(role);
    expect(wrapper.get("[data-slot-id]").attributes("data-slot-id")).toBe(slotId);
    const context = JSON.parse(wrapper.get("[data-slot-context]").text());
    expect(context).toMatchObject({
      characterId: "role-a",
      conversationId: "conversation-a",
      messageId: `message-${role}`,
      direction,
      senderType,
      images: [
        {
          id: "image-a",
          alt: "图片甲",
          status: "ready",
        },
      ],
    });
    expect(wrapper.find('[data-image-fallback="true"]').exists()).toBe(true);
    wrapper.unmount();
  });
});
