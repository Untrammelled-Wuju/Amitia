export type TemplateKind =
  | "tool"
  | "agent-skill"
  | "workflow"
  | "mcp"
  | "schema-ui"
  | "web-ui"
  | "event-hook"
  | "provider"
  | "task"
  | "desktop"
  | "composite";

export interface TemplateDescriptor {
  readonly kind: TemplateKind;
  readonly name: string;
  readonly description: string;
  readonly moduleKind: string;
  readonly defaultRuntime: string;
  readonly requiredPermissions: readonly string[];
  readonly highRiskPermissions: readonly string[];
  readonly files: TemplateFile[];
}

export interface TemplateFile {
  readonly path: string;
  readonly content: string;
  readonly executable?: boolean;
}

export interface TemplateScaffoldInput {
  readonly extensionId: string;
  readonly publisher: string;
  readonly displayName: string;
  readonly description: string;
  readonly version: string;
  readonly license: string;
  readonly targets: readonly string[];
  readonly sdkVersion: string;
}

export function describeTemplate(kind: TemplateKind): string {
  const map: Record<TemplateKind, string> = {
    tool: "纯 Tool 扩展：注册一个或多个模型可调用的 Tool",
    "agent-skill": "Agent Skill 扩展：组合 Tool 实现可被触发的能力",
    workflow: "Workflow 扩展：定义多步骤、可恢复的任务编排",
    mcp: "MCP 集成扩展：桥接外部 MCP 服务并暴露为 Tool",
    "schema-ui": "Schema UI 扩展：使用 SchemaUI 渲染配置或详情面板",
    "web-ui": "Restricted Web UI 扩展：在沙箱中渲染 HTML/CSS/JS",
    "event-hook": "Event/Hook 扩展：订阅事件或注入 Hook",
    provider: "Provider 扩展：实现图像、语音等外部 Provider 适配",
    task: "Task 扩展：实现长时运行、可恢复的任务",
    desktop: "Desktop 扩展：贡献菜单、托盘、快捷键等桌面扩展点",
    composite: "复合扩展：组合多种 Contribution",
  };
  return map[kind];
}

export function listTemplateKinds(): TemplateKind[] {
  return [
    "tool",
    "agent-skill",
    "workflow",
    "mcp",
    "schema-ui",
    "web-ui",
    "event-hook",
    "provider",
    "task",
    "desktop",
    "composite",
  ];
}
