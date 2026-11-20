import type { Component } from "vue";

export interface MainPanelDefinition {
  id: string;
  title: string;
  component: Component | (() => Promise<{ default: Component }>);
}

export interface MainPanelSummary {
  id: string;
  title: string;
}

const panels = new Map<string, MainPanelDefinition>();

export function registerMainPanel(definition: MainPanelDefinition): () => void {
  const id = String(definition?.id ?? "").trim();
  if (id === "") throw new Error("主面板 id 不能为空");
  const title = String(definition?.title ?? "").trim();
  if (title === "") throw new Error(`主面板 ${id} 需要标题`);
  if (definition.component === undefined || definition.component === null) {
    throw new Error(`主面板 ${id} 需要组件`);
  }
  if (panels.has(id)) throw new Error(`主面板 ${id} 已注册`);
  const registered: MainPanelDefinition = { id, title, component: definition.component };
  panels.set(id, registered);
  return () => {
    if (panels.get(id) === registered) panels.delete(id);
  };
}

export function resolveMainPanel(id: string): MainPanelDefinition | undefined {
  return panels.get(String(id ?? "").trim());
}

export function listMainPanels(): MainPanelSummary[] {
  return Array.from(panels.values()).map((panel) => ({ id: panel.id, title: panel.title }));
}
