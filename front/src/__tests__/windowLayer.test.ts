import { mount } from "@vue/test-utils"
import { defineComponent, ref } from "vue"
import { describe, expect, it } from "vitest"
import { useWindowLayerBlocker, windowLayerBlocked } from "../composables/useWindowLayer"

describe("窗口阻塞层", () => {
  it("弹窗注册与销毁会同步切换全局阻塞状态", () => {
    const active = ref(false)
    const wrapper = mount(defineComponent({
      setup() {
        useWindowLayerBlocker("window-layer-test", active)
        return () => null
      },
    }))
    expect(windowLayerBlocked.value).toBe(false)
    active.value = true
    expect(windowLayerBlocked.value).toBe(true)
    wrapper.unmount()
    expect(windowLayerBlocked.value).toBe(false)
  })
})
