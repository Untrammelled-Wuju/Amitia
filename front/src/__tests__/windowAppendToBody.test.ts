import { mount } from "@vue/test-utils"
import { ElDialog, ElDrawer } from "element-plus"
import { describe, expect, it } from "vitest"
import WindowDialog from "../components/ui-runtime/WindowDialog.vue"
import WindowDrawer from "../components/ui-runtime/WindowDrawer.vue"

describe("窗口层挂载位置", () => {
  it("弹窗默认挂载到 body", () => {
    const wrapper = mount(WindowDialog, {
      props: { modelValue: false },
    })
    expect(wrapper.findComponent(ElDialog).props("appendToBody")).toBe(true)
    wrapper.unmount()
  })

  it("抽屉默认挂载到 body", () => {
    const wrapper = mount(WindowDrawer, {
      props: { modelValue: false },
    })
    expect(wrapper.findComponent(ElDrawer).props("appendToBody")).toBe(true)
    wrapper.unmount()
  })

  it("调用方仍可显式关闭 body 挂载", () => {
    const dialog = mount(WindowDialog, {
      props: { modelValue: false, appendToBody: false },
    })
    const drawer = mount(WindowDrawer, {
      props: { modelValue: false, appendToBody: false },
    })
    expect(dialog.findComponent(ElDialog).props("appendToBody")).toBe(false)
    expect(drawer.findComponent(ElDrawer).props("appendToBody")).toBe(false)
    dialog.unmount()
    drawer.unmount()
  })
})
