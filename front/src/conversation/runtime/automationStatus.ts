import type { AssistantTurnData } from "../rendering/types";

export interface AutomationStatus {
  phase: "observing" | "acting" | "waiting" | "cancelling";
  label: string;
  detail: string;
  count: number;
}

export function automationAction(name: string): string | null {
  const normalized = name.toLowerCase().replaceAll("_", ".");
  if (normalized === "browser.agent.run") return "正在自动化操作";
  if (!/^(android\.(interaction|virtual\.display|ui\.tree|screen)\.|browser\.(interact|dom|navigate|resource\.screenshot|press\.key))/.test(normalized)) return null;
  if (/\.(status|list|get|permission|create|release|resize|start|stop)(\.|$)/.test(normalized)) return null;
  if (/snapshot|screenshot|capture|visual\.locate|dom\.find|screen\.frame\.latest/.test(normalized)) return "正在观察画面";
  if (/click|\.tap$/.test(normalized)) return "正在点击";
  if (/input|\.text$|fill|clear\.text/.test(normalized)) return "正在输入";
  if (/scroll|swipe/.test(normalized)) return "正在滚动";
  if (/navigate|launch/.test(normalized)) return "正在打开页面";
  return "正在操作";
}

export function projectAutomationStatus(turns: AssistantTurnData[]): AutomationStatus | null {
  const root = turns.filter((turn) => !turn.parentTurnId).sort((a, b) => b.sequence - a.sequence)[0];
  if (!root || ["completed", "failed", "interrupted"].includes(root.status)) return null;
  const byId = new Map(turns.map((turn) => [turn.id, turn]));
  const belongsToRoot = (turn: AssistantTurnData): boolean => {
    const visited = new Set<string>();
    let current: AssistantTurnData | undefined = turn;
    while (current && !visited.has(current.id)) {
      if (["completed", "failed", "interrupted"].includes(current.status)) return false;
      if (current.id === root.id) return true;
      visited.add(current.id);
      current = byId.get(current.parentTurnId || "");
    }
    return false;
  };
  const calls = turns.filter(belongsToRoot).flatMap((turn) => turn.items
    .filter((item) => item.type === "tool_call" && ["queued", "running", "waiting_approval"].includes(item.status))
    .map((item) => ({ turn, item, label: automationAction(item.toolName || "") }))
    .filter((call) => call.label !== null))
    .sort((a, b) => b.turn.sequence - a.turn.sequence || b.item.sequence - a.item.sequence);
  const latest = calls[0];
  if (!latest) return null;
  const approval = root.status === "waiting_approval" || latest.turn.status === "waiting_approval" || latest.item.status === "waiting_approval";
  const waiting = approval || latest.item.status === "queued";
  const cancelling = root.status === "cancelling" || latest.turn.status === "cancelling";
  const phase = cancelling ? "cancelling" : waiting ? "waiting" : latest.label === "正在观察画面" ? "observing" : "acting";
  return {
    phase,
    label: cancelling ? "正在停止自动化" : approval ? "等待授权" : waiting ? "等待自动化执行" : latest.label!,
    detail: cancelling ? "操作结束后撤下指示" : approval ? "请查看对话中的授权提示" : waiting ? "操作已排队" : "AI 正在执行界面操作",
    count: calls.length,
  };
}
