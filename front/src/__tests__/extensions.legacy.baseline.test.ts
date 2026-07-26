import { describe, expect, it } from "vitest"

describe("Extension Center — Legacy Baseline", () => {
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

    it("resolves skill list route", async () => {
      const router = (await import("@/router")).default
      const route = router.resolve("/extensions/skills")
      expect(route.name).toBe("extensionSkills")
    })

    it("resolves skill detail route with parameter", async () => {
      const router = (await import("@/router")).default
      const route = router.resolve("/extensions/skills/test-skill-id")
      expect(route.name).toBe("extensionSkillDetail")
      expect(route.params.id).toBe("test-skill-id")
    })

    it("resolves agent skill list route", async () => {
      const router = (await import("@/router")).default
      const route = router.resolve("/extensions/agent-skills")
      expect(route.name).toBe("extensionAgentSkills")
    })

    it("resolves plugin list route", async () => {
      const router = (await import("@/router")).default
      const route = router.resolve("/extensions/plugins")
      expect(route.name).toBe("extensionPlugins")
    })

    it("resolves plugin detail route with parameter", async () => {
      const router = (await import("@/router")).default
      const route = router.resolve("/extensions/plugins/test-plugin-id")
      expect(route.name).toBe("extensionPluginDetail")
      expect(route.params.id).toBe("test-plugin-id")
    })

    it("resolves workshop list route", async () => {
      const router = (await import("@/router")).default
      const route = router.resolve("/extensions/workshop")
      expect(route.name).toBe("extensionWorkshop")
    })

    it("resolves creative workshop skill route", async () => {
      const router = (await import("@/router")).default
      const route = router.resolve("/creative-workshop/skills")
      expect(route.name).toBe("extensionWorkshop")
    })

    it("resolves workshop session route with parameter", async () => {
      const router = (await import("@/router")).default
      const route = router.resolve("/extensions/workshop/test-session-id")
      expect(route.name).toBe("extensionWorkshopSession")
      expect(route.params.id).toBe("test-session-id")
    })

    it("resolves run history route", async () => {
      const router = (await import("@/router")).default
      const route = router.resolve("/extensions/runs")
      expect(route.name).toBe("extensionRuns")
    })

    it("all extension routes require auth", async () => {
      const router = (await import("@/router")).default
      const extRoutes = [
        "/extensions",
        "/extensions/mcp",
        "/extensions/packages",
        "/extensions/skills",
        "/extensions/agent-skills",
        "/extensions/plugins",
        "/extensions/workshop",
        "/extensions/runs",
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
      { name: "SkillListView", path: "@/views/extensions/SkillListView.vue" },
      { name: "SkillDetailView", path: "@/views/extensions/SkillDetailView.vue" },
      { name: "PluginListView", path: "@/views/extensions/PluginListView.vue" },
      { name: "PluginDetailView", path: "@/views/extensions/PluginDetailView.vue" },
      { name: "AgentSkillListView", path: "@/views/extensions/agent-skills/AgentSkillListView.vue" },
      { name: "PackageManagerView", path: "@/views/extensions/packages/PackageManagerView.vue" },
      { name: "WorkshopListView", path: "@/views/extensions/workshop/WorkshopListView.vue" },
      { name: "WorkshopSessionView", path: "@/views/extensions/workshop/WorkshopSessionView.vue" },
      { name: "RunHistoryView", path: "@/views/extensions/RunHistoryView.vue" },
    ]

    for (const { name, path } of extensionViews) {
      it(`loads ${name}`, async () => {
        const mod = await import(path)
        expect(mod.default).toBeDefined()
      })
    }
  })

  describe("Schema Surface Renderer", () => {
    it("exports SchemaSurfaceRenderer component", async () => {
      const mod = await import("@/views/extensions/components/SchemaSurfaceRenderer.vue")
      expect(mod.default).toBeDefined()
    })

    it("exports SurfaceAction component", async () => {
      const mod = await import("@/views/extensions/components/SurfaceAction.vue")
      expect(mod.default).toBeDefined()
    })

    it("exports SurfaceForm component", async () => {
      const mod = await import("@/views/extensions/components/SurfaceForm.vue")
      expect(mod.default).toBeDefined()
    })

    it("exports SurfaceStatus component", async () => {
      const mod = await import("@/views/extensions/components/SurfaceStatus.vue")
      expect(mod.default).toBeDefined()
    })

    it("exports SurfaceTable component", async () => {
      const mod = await import("@/views/extensions/components/SurfaceTable.vue")
      expect(mod.default).toBeDefined()
    })

    it("exports PermissionDialog component", async () => {
      const mod = await import("@/views/extensions/components/PermissionDialog.vue")
      expect(mod.default).toBeDefined()
    })
  })

  describe("Workshop components", () => {
    it("exports CapabilityRiskList", async () => {
      const mod = await import("@/views/extensions/workshop/components/CapabilityRiskList.vue")
      expect(mod.default).toBeDefined()
    })

    it("exports StructuredDraftEditor", async () => {
      const mod = await import("@/views/extensions/workshop/components/StructuredDraftEditor.vue")
      expect(mod.default).toBeDefined()
    })

    it("exports TestResultViewer", async () => {
      const mod = await import("@/views/extensions/workshop/components/TestResultViewer.vue")
      expect(mod.default).toBeDefined()
    })
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

    it("getPageTitle resolves workshop path", async () => {
      const { getPageTitle } = await import("@/navigation/app-nav")
      const title = getPageTitle("/extensions/workshop")
      expect(typeof title).toBe("string")
      expect(title.length).toBeGreaterThan(0)
    })

    it("getPageTitle resolves creative workshop card paths", async () => {
      const { getPageTitle } = await import("@/navigation/app-nav")
      expect(getPageTitle("/creative-workshop/pet")).toBe("桌宠")
      expect(getPageTitle("/creative-workshop/skills")).toBe("技能制作")
    })
  })
})
