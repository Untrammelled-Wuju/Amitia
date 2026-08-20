import type { Component } from "vue";
import {
  ChatDotRound,
  ChatDotSquare,
  Connection,
  Odometer,
  Opportunity,
  Setting,
  Share,
  MagicStick,
} from "@element-plus/icons-vue";

export type AppNavItem = {
  key: string;
  to: string;
  label: string;
  icon: Component;
  match?: string[];
  mobile?: boolean;
};

export type AppNavGroup = {
  key: string;
  items: AppNavItem[];
};

export const desktopNavGroups: AppNavGroup[] = [
  {
    key: "core",
    items: [
      {
        key: "chat",
        to: "/chat",
        label: "聊天",
        icon: ChatDotRound,
        mobile: true,
      },
      {
        key: "dashboard",
        to: "/dashboard/data",
        label: "概览",
        icon: Odometer,
        match: ["/dashboard/data", "/dashboard/run"],
      },
      {
        key: "creativeWorkshop",
        to: "/creative-workshop",
        label: "创意工坊",
        icon: MagicStick,
        match: ["/creative-workshop"],
      },
    ],
  },
  {
    key: "links",
    items: [
      { key: "wechat", to: "/wechat", label: "微信连接", icon: Connection },
      { key: "qq", to: "/qq", label: "QQ 连接", icon: ChatDotSquare },
    ],
  },
  {
    key: "system",
    items: [
      { key: "gameCenter", to: "/game-center", label: "游戏中心", icon: MagicStick },
      { key: "devices", to: "/devices", label: "我的设备", icon: Connection },
      {
        key: "runtimeDebug",
        to: "/runtime-debug",
        label: "运行时调试",
        icon: Opportunity,
      },
      {
        key: "decisionViz",
        to: "/decision-viz",
        label: "决策可视化",
        icon: Share,
      },
      {
        key: "settings",
        to: "/settings",
        label: "设置",
        icon: Setting,
        mobile: true,
      },
    ],
  },
];

export const mobileNavItems = desktopNavGroups.flatMap((group) =>
  group.items.filter((item) => item.mobile),
);

const titleItems = desktopNavGroups.flatMap((group) => group.items);

const extraTitles = [
  { path: "/login", label: "登录" },
  { path: "/onboarding", label: "引导" },
  { path: "/setup", label: "初始化" },
  { path: "/privacy", label: "隐私说明" },
  { path: "/usage-boundary", label: "使用边界" },
  { path: "/storage", label: "存储清理" },
  { path: "/runtime-mode", label: "运行模式" },
  { path: "/runtime-debug", label: "运行时调试" },
  { path: "/user-settings", label: "用户信息" },
  { path: "/creative-workshop", label: "创意工坊" },
  { path: "/creative-workshop/pet", label: "桌宠" },
  { path: "/creative-workshop/skills", label: "技能制作" },
  { path: "/emotes", label: "表情包管理" },
  { path: "/extensions/mcp", label: "MCP 服务" },
  { path: "/extensions/packages", label: "扩展包" },
  { path: "/extensions/skills", label: "技能管理" },
  { path: "/extensions/plugins", label: "系统插件" },
  { path: "/extensions/workshop", label: "技能制作" },
  { path: "/extensions/runs", label: "技能执行记录" },
  { path: "/extensions", label: "扩展中心" },
  { path: "/kernel", label: "扩展包" },
  { path: "/kernel/trusted-services", label: "可信服务运行时" },
  { path: "/kernel/wasm", label: "WASM 运行时" },
  { path: "/kernel/hooks", label: "Hook 中心" },
  { path: "/kernel/tasks", label: "任务运行时" },
  { path: "/kernel/events", label: "事件中心" },
  { path: "/kernel/schedules", label: "调度中心" },
  { path: "/kernel/desktop", label: "桌面贡献中心" },
  { path: "/kernel/updates", label: "扩展更新中心" },
  { path: "/kernel/dev-console", label: "开发者诊断控制台" },
  { path: "/kernel/migrations", label: "迁移与灰度中心" },
  { path: "/kernel/dev-mode", label: "开发模式中心" },
  { path: "/game-center", label: "游戏中心" },
  { path: "/devices", label: "我的设备" },
  { path: "/asr", label: "语音识别" },
  { path: "/realtime-voice", label: "实时语音" },
  { path: "/long-running", label: "长期运行维护" },
];

export function isNavItemActive(path: string, item: AppNavItem) {
  if (path === item.to) {
    return true;
  }
  return (item.match || []).some((prefix) => path.startsWith(prefix));
}

export function getPageTitle(path: string) {
  const extra = extraTitles.find((item) => item.path === path);
  if (extra) {
    return extra.label;
  }
  const navItem = titleItems.find((item) => isNavItemActive(path, item));
  if (navItem) {
    return navItem.label;
  }
  return "AI-Amitia";
}
