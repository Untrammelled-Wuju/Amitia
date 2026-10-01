import { nextTick, ref } from "vue";
import { beforeEach, describe, expect, it, vi } from "vitest";
import { useWebChatScroll } from "../composables/useWebChatScroll";

describe("useWebChatScroll", () => {
  let frameCallbacks: FrameRequestCallback[];

  beforeEach(() => {
    frameCallbacks = [];
    vi.stubGlobal("requestAnimationFrame", (callback: FrameRequestCallback) => {
      frameCallbacks.push(callback);
      return frameCallbacks.length;
    });
    vi.stubGlobal("cancelAnimationFrame", vi.fn());
  });

  it("用户向上滚动后停止自动跟随", async () => {
    const element = {
      scrollTop: 400,
      scrollHeight: 1000,
      clientHeight: 500,
      scrollTo: vi.fn(),
    };
    const msgAreaRef = ref({ rootEl: element });
    const showScrollBtn = ref(false);
    const {
      scrollToBottom,
      onWheel,
    } = useWebChatScroll(
      msgAreaRef,
      ref([]),
      ref("conversation-1"),
      showScrollBtn,
    );

    onWheel({ deltaY: -100, preventDefault: vi.fn() } as unknown as WheelEvent);
    scrollToBottom();
    await nextTick();

    expect(frameCallbacks).toHaveLength(0);
  });

  it("回到底部后恢复自动跟随", async () => {
    const element = {
      scrollTop: 480,
      scrollHeight: 1000,
      clientHeight: 500,
      scrollTo: vi.fn(),
    };
    const msgAreaRef = ref({ rootEl: element });
    const showScrollBtn = ref(false);
    const {
      scrollToBottom,
      onScroll,
    } = useWebChatScroll(
      msgAreaRef,
      ref([]),
      ref("conversation-1"),
      showScrollBtn,
    );

    onScroll();
    scrollToBottom();
    await nextTick();
    frameCallbacks.shift()?.(0);

    expect(element.scrollTo).toHaveBeenCalledWith({
      top: 1000,
      behavior: "auto",
    });
  });

  it("内容重排导致位置短暂变化时不会停止跟随", async () => {
    const element = {
      scrollTop: 500,
      scrollHeight: 1000,
      clientHeight: 500,
      scrollTo: vi.fn(),
    };
    const msgAreaRef = ref({ rootEl: element });
    const showScrollBtn = ref(false);
    const {
      scrollToBottom,
      onScroll,
    } = useWebChatScroll(
      msgAreaRef,
      ref([]),
      ref("conversation-1"),
      showScrollBtn,
    );

    onScroll();
    element.scrollTop = 490;
    onScroll();
    scrollToBottom();
    await nextTick();

    expect(frameCallbacks).toHaveLength(1);
  });
});
