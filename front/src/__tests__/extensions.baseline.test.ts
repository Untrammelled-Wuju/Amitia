import { describe, expect, it } from "vitest"
import { readFileSync } from "node:fs"
import { resolve } from "node:path"
import builtinRoutesSource from "../router/builtinRoutes.ts?raw"
import { validateDocument } from "@/components/extension/schema-ui-utils"

function extensionPluginRoutes(): string[] {
  const manifest = JSON.parse(readFileSync(resolve(__dirname, "../../../plugins-builtin/plugin-market/amitia-extension.json"), "utf8"))
  return manifest.modules.flatMap((module: any) => module.contributions)
    .flatMap((contribution: any) => contribution.spec?.metadata?.routes ?? [])
    .map((item: any) => item.path)
}

describe("Extension Center Baseline", () => {
  describe("Router", () => {
    it("leaves extension and kernel roots to the plugin route registry", { timeout: 30000 }, async () => {
      const router = (await import("@/router")).default
      const route = router.resolve("/extensions")
      expect(route.name).not.toBe("extensionCenter")
      expect(builtinRoutesSource).not.toContain('path: "/extensions"')
      const manifest = JSON.parse(readFileSync(resolve(__dirname, "../../../plugins-builtin/plugin-market/amitia-extension.json"), "utf8"))
      const routes = manifest.modules.flatMap((module: any) => module.contributions)
        .flatMap((contribution: any) => contribution.spec?.metadata?.routes ?? [])
        .map((item: any) => item.path)
      expect(routes).toEqual(expect.arrayContaining(["/extensions", "/kernel"]))
    })

    it("leaves MCP server route to the extension plugin", async () => {
      const router = (await import("@/router")).default
      const route = router.resolve("/extensions/mcp")
      expect(route.name).not.toBe("extensionMCP")
      expect(extensionPluginRoutes()).toContain("/extensions/mcp")
    })

    it("validates the plugin-owned extension and kernel hub schemas", () => {
      for (const name of ["extensions-hub.json", "kernel-hub.json"]) {
        const document = JSON.parse(readFileSync(resolve(__dirname, "../../../plugins-builtin/plugin-market/ui/schema", name), "utf8"))
        expect(validateDocument(document)).toEqual(expect.objectContaining({ valid: true, errors: [] }))
      }
    })

    it("leaves package manager route to the extension plugin", async () => {
      const router = (await import("@/router")).default
      const route = router.resolve("/extensions/packages")
      expect(route.name).not.toBe("extensionPackages")
      expect(extensionPluginRoutes()).toContain("/extensions/packages")
    })

    it("leaves workflow routes to the extension plugin", async () => {
      const router = (await import("@/router")).default
      expect(router.resolve("/extensions/workflows").name).not.toBe("extensionWorkflows")
      expect(router.resolve("/extensions/workflows/test-workflow-id").name).not.toBe("extensionWorkflowBuilder")
      expect(extensionPluginRoutes()).toEqual(expect.arrayContaining([
        "/extensions/workflows",
        "/extensions/workflows/:id",
      ]))
    })

    it("leaves character card workshop to the creative plugin route registry", async () => {
      const router = (await import("@/router")).default
      expect(router.resolve("/creative-workshop/character-cards").name).not.toBe("characterCardWorkshop")
      const manifest = JSON.parse(readFileSync(resolve(__dirname, "../../../plugins-builtin/creative-workshop-hub/amitia-extension.json"), "utf8"))
      const routes = manifest.modules.flatMap((module: any) => module.contributions)
        .flatMap((contribution: any) => contribution.spec?.metadata?.routes ?? [])
        .map((item: any) => item.path)
      expect(routes).toContain("/creative-workshop/character-cards")
    })

    it("does not redirect retired workflow routes", async () => {
      const router = (await import("@/router")).default
      expect(router.resolve("/creative-workshop/workflows").name).toBe("catchAll")
      expect(router.resolve("/creative-workshop/workflows/test-workflow-id").name).toBe("catchAll")
    })

    it("removes retired skill list route", async () => {
      const router = (await import("@/router")).default
      const route = router.resolve("/extensions/skills")
      expect(route.name).not.toBe("extensionSkills")
    })

    it("removes retired skill detail route", async () => {
      const router = (await import("@/router")).default
      const route = router.resolve("/extensions/skills/test-skill-id")
      expect(route.name).not.toBe("extensionSkillDetail")
    })

    it("leaves agent skill list route to the extension plugin", async () => {
      const router = (await import("@/router")).default
      const route = router.resolve("/extensions/agent-skills")
      expect(route.name).not.toBe("extensionAgentSkills")
      expect(extensionPluginRoutes()).toContain("/extensions/agent-skills")
    })

    it("removes retired workshop route", async () => {
      const router = (await import("@/router")).default
      const route = router.resolve("/extensions/workshop")
      expect(route.name).not.toBe("extensionWorkshop")
    })

    it("removes retired creative workshop skill route", async () => {
      const router = (await import("@/router")).default
      const route = router.resolve("/creative-workshop/skills")
      expect(route.name).not.toBe("extensionWorkshop")
    })

    it("removes retired workshop session route", async () => {
      const router = (await import("@/router")).default
      const route = router.resolve("/extensions/workshop/test-session-id")
      expect(route.name).not.toBe("extensionWorkshopSession")
    })

    it("removes retired run history route", async () => {
      const router = (await import("@/router")).default
      const route = router.resolve("/extensions/runs")
      expect(route.name).not.toBe("extensionRuns")
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
    it("resolvePageTitle resolves extension paths", async () => {
      const { resolvePageTitle } = await import("@/ui-runtime/navigationRegistry")
      const title = resolvePageTitle("/extensions")
      expect(title).toBe("扩展中心")
      expect(resolvePageTitle("/kernel")).toBe("内核工具")
    })

    it("resolvePageTitle resolves MCP path", async () => {
      const { resolvePageTitle } = await import("@/ui-runtime/navigationRegistry")
      const title = resolvePageTitle("/extensions/mcp")
      expect(title).toBe("MCP 服务")
    })

    it("resolvePageTitle resolves creative workshop card paths", async () => {
      const { resolvePageTitle } = await import("@/ui-runtime/navigationRegistry")
      expect(resolvePageTitle("/chat")).toBe("新对话")
      expect(resolvePageTitle("/creative-workshop/pet")).toBe("桌宠")
      expect(resolvePageTitle("/creative-workshop/character-cards")).toBe("角色卡工坊")
      expect(resolvePageTitle("/extensions/workflows")).toBe("工作流")
    })
  })
})
