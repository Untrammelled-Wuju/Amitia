// SPDX-FileCopyrightText: 2026 彭旭
// SPDX-License-Identifier: AGPL-3.0-only
import { describe, it, expect } from "vitest"
import { mount } from "@vue/test-utils"

describe("Web App", () => {
  it("should render simple component", () => {
    const TestComponent = {
      template: "<div>Hello Test</div>",
    }
    const wrapper = mount(TestComponent)
    expect(wrapper.text()).toBe("Hello Test")
  })

  it("should import pinia store", async () => {
    const { useAppStore } = await import("../stores/app.js")
    expect(useAppStore).toBeDefined()
    // Create store instance
    const { createPinia, setActivePinia } = await import("pinia")
    const pinia = createPinia()
    setActivePinia(pinia)
    const store = useAppStore()
    expect(store.characters).toBeDefined()
    expect(store.conversations).toBeDefined()
  })

  it("should import chat store", async () => {
    const { useChatStore } = await import("../stores/chat.js")
    expect(useChatStore).toBeDefined()
  })
})
