import { markRaw, reactive, type Component } from "vue";

export interface ContextMenuAction {
  id: string;
  label: string;
  icon?: Component;
  shortcut?: string;
  disabled?: boolean;
  danger?: boolean;
  separatorBefore?: boolean;
  action?: () => void | Promise<void>;
  children?: ContextMenuAction[];
}

interface ContextMenuState {
  visible: boolean;
  x: number;
  y: number;
  title: string;
  items: ContextMenuAction[];
}

const state = reactive<ContextMenuState>({
  visible: false,
  x: 0,
  y: 0,
  title: "",
  items: [],
});

export const contextMenuState = state;

function normalizeAction(action: ContextMenuAction): ContextMenuAction {
  return {
    ...action,
    icon: action.icon ? markRaw(action.icon) : undefined,
    children: action.children?.map(normalizeAction),
  };
}

export function openContextMenu(request: {
  x: number;
  y: number;
  title?: string;
  items?: ContextMenuAction[];
}) {
  state.x = request.x;
  state.y = request.y;
  state.title = request.title || "";
  state.items = (request.items || []).map(normalizeAction);
  state.visible = true;
}

export function setContextMenuPosition(x: number, y: number) {
  state.x = x;
  state.y = y;
}

export function closeContextMenu() {
  state.visible = false;
}
