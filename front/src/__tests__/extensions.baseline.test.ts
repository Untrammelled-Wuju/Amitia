import { describe, expect, it } from "vitest"

describe("Extension Center Baseline", () => {
  describe("Router", () => {
    it("resolves extension center route", { timeout: 30000 }, async () => {
      const router = (await import("@/router")).default
      const route = router.resolve("/extensions")
      expect(route.name).toBe("extensionCenter")
    })

    it("resolves MCP server route", async () => {
      const router = (await import("@/router")).default
      const route = router.resolve("/extensions/mcp")
      expect(route.name).toBe("extensionMCP")
    })

    it("resolves package manager route", async () => {
      const router = (await import("@/router")).default
      const route = router.resolve("/extensions/packages")
      expect(route.name).toBe("extensionPackages")
    })

    it("resolves workflow routes from extension center", async () => {
      const router = (await import("@/router")).default
      expect(router.resolve("/extensions/workflows").name).toBe("extensionWorkflows")
      expect(router.resolve("/extensions/workflows/test-workflow-id").name).toBe("extensionWorkflowBuilder")
    })

    it("resolves character card workshop route", async () => {
      const router = (await import("@/router")).default
      expect(router.resolve("/creative-workshop/character-cards").name).toBe("characterCardWorkshop")
    })

    it("redirects legacy workflow routes to extension center", async () => {
      const router = (await import("@/router")).default
      const legacyList = router.resolve("/creative-workshop/workflows").matched.at(-1)
      const legacyBuilder = router.resolve("/creative-workshop/workflows/test-workflow-id").matched.at(-1)
      expect(typeof legacyList?.redirect).toBe("function")
      expect(typeof legacyBuilder?.redirect).toBe("function")
    })

    it("removes legacy skill list route", async () => {
      const router = (await import("@/router")).default
      const route = router.resolve("/extensions/skills")
      expect(route.name).not.toBe("extensionSkills")
    })

    it("removes legacy skill detail route", async () => {
      const router = (await import("@/router")).default
      const route = router.resolve("/extensions/skills/test-skill-id")
      expect(route.name).not.toBe("extensionSkillDetail")
    })

    it("resolves agent skill list route", async () => {
      const router = (await import("@/router")).default
      const route = router.resolve("/extensions/agent-skills")
      expect(route.name).toBe("extensionAgentSkills")
    })

    it("removes legacy workshop route", async () => {
      const router = (await import("@/router")).default
      const route = router.resolve("/extensions/workshop")
      expect(route.name).not.toBe("extensionWorkshop")
    })

    it("removes legacy creative workshop skill route", async () => {
      const router = (await import("@/router")).default
      const route = router.resolve("/creative-workshop/skills")
      expect(route.name).not.toBe("extensionWorkshop")
    })

    it("removes legacy workshop session route", async () => {
      const router = (await import("@/router")).default
      const route = router.resolve("/extensions/workshop/test-session-id")
      expect(route.name).not.toBe("extensionWorkshopSession")
    })

    it("removes legacy run history route", async () => {
      const router = (await import("@/router")).default
      const route = router.resolve("/extensions/runs")
      expect(route.name).not.toBe("extensionRuns")
    })

    it("all extension routes require auth", async () => {
      const router = (await import("@/router")).default
      const extRoutes = [
        "/extensions",
        "/extensions/mcp",
        "/extensions/packages",
        "/extensions/agent-skills",
        "/extensions/workflows",
      ]
      for (const path of extRoutes) {
        const route = router.resolve(path)
        const matched = route.matched[route.matched.length - 1]
        expect(matched.meta.requiresAuth, `route ${path} should require auth`).toBe(true)
      }
    })
  })

  describe("API client", () => {
    it("exports extension API module", async () => {
      const api = await import("@/views/extensions/api")
      expect(api).toBeDefined()
    })

    it("exports extension types module", async () => {
      const types = await import("@/views/extensions/types")
      expect(types).toBeDefined()
    })
  })

  describe("Component imports", () => {
    const extensionViews = [
      { name: "ExtensionCenterView", path: "@/views/extensions/ExtensionCenterView.vue" },
      { name: "AgentSkillListView", path: "@/views/extensions/agent-skills/AgentSkillListView.vue" },
      { name: "PackageManagerView", path: "@/views/extensions/packages/PackageManagerView.vue" },
      { name: "WorkflowListView", path: "@/views/extensions/workflows/WorkflowListView.vue" },
      { name: "WorkflowBuilderView", path: "@/views/extensions/workflows/WorkflowBuilderView.vue" },
      { name: "CharacterConfigView", path: "@/views/character-config/CharacterConfigView.vue" },
    ]

    for (const { name, path } of extensionViews) {
      it(`loads ${name}`, async () => {
        const mod = await import(path)
        expect(mod.default).toBeDefined()
      })
    }
  })

  describe("Navigation", () => {
    it("getPageTitle resolves extension paths", async () => {
      const { getPageTitle } = await import("@/navigation/app-nav")
      const title = getPageTitle("/extensions")
      expect(typeof title).toBe("string")
      expect(title.length).toBeGreaterThan(0)
    })

    it("getPageTitle resolves MCP path", async () => {
      const { getPageTitle } = await import("@/navigation/app-nav")
      const title = getPageTitle("/extensions/mcp")
      expect(typeof title).toBe("string")
      expect(title.length).toBeGreaterThan(0)
    })

    it("getPageTitle resolves creative workshop card paths", async () => {
      const { getPageTitle } = await import("@/navigation/app-nav")
      expect(getPageTitle("/creative-workshop/pet")).toBe("桌宠")
      expect(getPageTitle("/creative-workshop/character-cards")).toBe("角色卡工坊")
      expect(getPageTitle("/extensions/workflows")).toBe("工作流")
    })
  })
})
