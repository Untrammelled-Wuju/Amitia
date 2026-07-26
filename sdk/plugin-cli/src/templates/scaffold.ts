import type { TemplateDescriptor, TemplateKind, TemplateFile, TemplateScaffoldInput } from "./types";

export function scaffoldManifest(input: TemplateScaffoldInput, kind: TemplateKind): string {
  const module = moduleForTemplate(kind);
  const permissions = permissionsForTemplate(kind);
  const manifest = {
    manifestVersion: 2,
    extensionId: input.extensionId,
    publisher: input.publisher,
    displayName: { default: input.displayName },
    description: { default: input.description },
    version: input.version,
    contractVersion: 1,
    modules: [
      {
        moduleId: module.moduleId,
        kind: module.kind,
        entry: module.entry,
        runtime: module.runtime,
        displayName: { default: input.displayName },
        description: { default: input.description },
      },
    ],
    permissions: permissions.map((p) => ({ permission: p, required: true })),
    license: input.license,
    platforms: input.targets,
    minHostVersion: "0.1.0",
  };
  return JSON.stringify(manifest, null, 2) + "\n";
}

export function scaffoldPackageJson(input: TemplateScaffoldInput): string {
  const pkg = {
    name: input.extensionId,
    version: input.version,
    description: input.description,
    license: input.license,
    type: "module",
    main: "./dist/index.js",
    scripts: {
      build: "amitia-ext build",
      dev: "amitia-ext dev",
      pack: "amitia-ext pack",
      validate: "amitia-ext validate",
      lint: "amitia-ext lint",
      test: "amitia-ext test",
    },
    devDependencies: {
      "@amitia/plugin-sdk": `^${input.sdkVersion}`,
      "@amitia/plugin-cli": `^${input.sdkVersion}`,
      typescript: "^5.4.0",
      vitest: "^1.0.0",
    },
  };
  return JSON.stringify(pkg, null, 2) + "\n";
}

export function scaffoldTsConfig(): string {
  return JSON.stringify(
    {
      compilerOptions: {
        target: "ES2022",
        module: "ESNext",
        moduleResolution: "bundler",
        lib: ["ES2022", "DOM"],
        declaration: true,
        outDir: "./dist",
        rootDir: "./src",
        strict: true,
        esModuleInterop: true,
        skipLibCheck: true,
        isolatedModules: true,
      },
      include: ["src/**/*", "manifest.ts"],
      exclude: ["node_modules", "dist", "tests"],
    },
    null,
    2,
  ) + "\n";
}

export function scaffoldEntry(input: TemplateScaffoldInput, kind: TemplateKind): TemplateFile {
  const module = moduleForTemplate(kind);
  switch (kind) {
    case "tool":
      return {
        path: `src/${module.moduleId}.ts`,
        content: toolEntry(input),
      };
    case "agent-skill":
      return {
        path: `src/${module.moduleId}.ts`,
        content: skillEntry(input),
      };
    case "workflow":
      return {
        path: `src/${module.moduleId}.ts`,
        content: workflowEntry(input),
      };
    case "mcp":
      return {
        path: `src/${module.moduleId}.ts`,
        content: mcpEntry(input),
      };
    case "schema-ui":
      return {
        path: `src/${module.moduleId}.ts`,
        content: schemaUiEntry(input),
      };
    case "web-ui":
      return {
        path: `src/${module.moduleId}.ts`,
        content: webUiEntry(input),
      };
    case "event-hook":
      return {
        path: `src/${module.moduleId}.ts`,
        content: eventHookEntry(input),
      };
    case "provider":
      return {
        path: `src/${module.moduleId}.ts`,
        content: providerEntry(input),
      };
    case "task":
      return {
        path: `src/${module.moduleId}.ts`,
        content: taskEntry(input),
      };
    case "desktop":
      return {
        path: `src/${module.moduleId}.ts`,
        content: desktopEntry(input),
      };
    case "composite":
    default:
      return {
        path: `src/${module.moduleId}.ts`,
        content: compositeEntry(input),
      };
  }
}

function moduleForTemplate(kind: TemplateKind): { moduleId: string; kind: string; entry: string; runtime: string } {
  switch (kind) {
    case "tool":
      return { moduleId: "main-tool", kind: "tool", entry: "./dist/main-tool.js", runtime: "javascript_main" };
    case "agent-skill":
      return { moduleId: "main-skill", kind: "skill", entry: "./dist/main-skill.js", runtime: "javascript_main" };
    case "workflow":
      return { moduleId: "main-workflow", kind: "workflow", entry: "./dist/main-workflow.js", runtime: "task_runtime" };
    case "mcp":
      return { moduleId: "main-mcp", kind: "mcp_server", entry: "./dist/main-mcp.js", runtime: "trusted_service" };
    case "schema-ui":
      return { moduleId: "main-ui", kind: "ui_contribution", entry: "./dist/main-ui.js", runtime: "javascript_main" };
    case "web-ui":
      return { moduleId: "main-webui", kind: "ui_contribution", entry: "./dist/main-webui.js", runtime: "javascript_main" };
    case "event-hook":
      return { moduleId: "main-hook", kind: "tool", entry: "./dist/main-hook.js", runtime: "javascript_main" };
    case "provider":
      return { moduleId: "main-provider", kind: "tool", entry: "./dist/main-provider.js", runtime: "trusted_service" };
    case "task":
      return { moduleId: "main-task", kind: "task_runtime", entry: "./dist/main-task.js", runtime: "task_runtime" };
    case "desktop":
      return { moduleId: "main-desktop", kind: "ui_contribution", entry: "./dist/main-desktop.js", runtime: "javascript_main" };
    case "composite":
    default:
      return { moduleId: "main", kind: "tool", entry: "./dist/main.js", runtime: "javascript_main" };
  }
}

function permissionsForTemplate(kind: TemplateKind): string[] {
  switch (kind) {
    case "tool":
      return [];
    case "agent-skill":
      return [];
    case "workflow":
      return ["task.run"];
    case "mcp":
      return ["mcp.connect"];
    case "schema-ui":
      return [];
    case "web-ui":
      return [];
    case "event-hook":
      return ["event.subscribe"];
    case "provider":
      return ["provider.invoke"];
    case "task":
      return ["task.run"];
    case "desktop":
      return ["desktop.menu", "desktop.tray"];
    case "composite":
    default:
      return [];
  }
}

function toolEntry(input: TemplateScaffoldInput): string {
  return `import { defineExtension, defineTool, successResult } from "@amitia/plugin-sdk";

export default defineExtension({
  async activate(context) {
    defineTool(
      {
        toolId: "${input.extensionId}/hello",
        title: { default: "Hello" },
        description: { default: "Returns a greeting from ${input.displayName}" },
        parameters: { type: "object", properties: {} },
        runtimeBinding: "host_internal",
        riskLevel: "low",
        idempotent: true,
      },
      async () => successResult({ message: "hello from ${input.displayName}" }),
    );
  },
  async deactivate() {},
});
`;
}

function skillEntry(input: TemplateScaffoldInput): string {
  return `import { defineExtension } from "@amitia/plugin-sdk";

export default defineExtension({
  async activate(context) {
    context.logger.info("skill extension ${input.extensionId} activated");
  },
  async deactivate() {},
});
`;
}

function workflowEntry(input: TemplateScaffoldInput): string {
  return `import { defineExtension, defineTask, successTaskResult } from "@amitia/plugin-sdk";

export default defineExtension({
  async activate(context) {
    defineTask({
      taskId: "${input.extensionId}/workflow",
      handler: async (input, ctx) => {
        await ctx.progress.report({ current: 0, total: 1, message: "starting" });
        return successTaskResult({ done: true });
      },
      timeoutMs: 60_000,
      maxAttempts: 1,
    });
  },
  async deactivate() {},
});
`;
}

function mcpEntry(_input: TemplateScaffoldInput): string {
  return `import { defineExtension } from "@amitia/plugin-sdk";

export default defineExtension({
  async activate(context) {
    context.logger.info("mcp bridge ready");
  },
  async deactivate() {},
});
`;
}

function schemaUiEntry(_input: TemplateScaffoldInput): string {
  return `import { defineExtension } from "@amitia/plugin-sdk";

export default defineExtension({
  async activate(context) {
    context.logger.info("schema-ui contribution ready");
  },
  async deactivate() {},
});
`;
}

function webUiEntry(_input: TemplateScaffoldInput): string {
  return `import { defineExtension, createAmitiaUI } from "@amitia/plugin-sdk";

export default defineExtension({
  async activate(context) {
    if (context.ui) {
      const ui = createAmitiaUI(context.ui);
      await ui.ready();
    }
  },
  async deactivate() {},
});
`;
}

function eventHookEntry(_input: TemplateScaffoldInput): string {
  return `import { defineExtension, defineEvent } from "@amitia/plugin-sdk";

export default defineExtension({
  async activate(context) {
    defineEvent({
      eventType: "extension.ready",
      handler: async (event, ctx) => {
        ctx.logger.info("event received", { type: event.type });
      },
    });
  },
  async deactivate() {},
});
`;
}

function providerEntry(_input: TemplateScaffoldInput): string {
  return `import { defineExtension } from "@amitia/plugin-sdk";

export default defineExtension({
  async activate(context) {
    context.logger.info("provider ready");
  },
  async deactivate() {},
});
`;
}

function taskEntry(input: TemplateScaffoldInput): string {
  return `import { defineExtension, defineTask, successTaskResult } from "@amitia/plugin-sdk";

export default defineExtension({
  async activate(context) {
    defineTask({
      taskId: "${input.extensionId}/task",
      handler: async (input, ctx) => {
        await ctx.progress.report({ current: 0, total: 1 });
        return successTaskResult({ ok: true });
      },
    });
  },
  async deactivate() {},
});
`;
}

function desktopEntry(_input: TemplateScaffoldInput): string {
  return `import { defineExtension } from "@amitia/plugin-sdk";

export default defineExtension({
  async activate(context) {
    context.logger.info("desktop extension ready");
  },
  async deactivate() {},
});
`;
}

function compositeEntry(input: TemplateScaffoldInput): string {
  return `import { defineExtension, defineTool, successResult } from "@amitia/plugin-sdk";

export default defineExtension({
  async activate(context) {
    defineTool(
      {
        toolId: "${input.extensionId}/ping",
        title: { default: "Ping" },
        description: { default: "Composite extension ping" },
        parameters: { type: "object", properties: {} },
        runtimeBinding: "host_internal",
        riskLevel: "low",
      },
      async () => successResult({ ok: true }),
    );
  },
  async deactivate() {},
});
`;
}

export function getTemplateDescriptor(kind: TemplateKind): TemplateDescriptor {
  const module = moduleForTemplate(kind);
  const input: TemplateScaffoldInput = {
    extensionId: "publisher/my-extension",
    publisher: "publisher",
    displayName: "My Extension",
    description: "Scaffolded extension",
    version: "0.1.0",
    license: "MIT",
    targets: ["windows-x64"],
    sdkVersion: "1.0.0",
  };
  const entry = scaffoldEntry(input, kind);
  const manifest = scaffoldManifest(input, kind);
  const packageJson = scaffoldPackageJson(input);
  const tsconfig = scaffoldTsConfig();
  return {
    kind,
    name: kind,
    description: "template",
    moduleKind: module.kind,
    defaultRuntime: module.runtime,
    requiredPermissions: permissionsForTemplate(kind),
    highRiskPermissions: [],
    files: [
      { path: "manifest.json", content: manifest },
      { path: "package.json", content: packageJson },
      { path: "tsconfig.json", content: tsconfig },
      entry,
      { path: "README.md", content: `# My Extension\n\nScaffolded by amitia-ext.\n` },
      { path: "LICENSE", content: "MIT License\n" },
      { path: ".gitignore", content: "node_modules/\ndist/\npackage/\n*.amitiax\n" },
    ],
  };
}
