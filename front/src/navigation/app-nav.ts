import type { Component } from "vue"
import {
  Bell,
  ChatDotRound,
  ChatDotSquare,
  ChatLineSquare,
  Collection,
  Connection,
  Cpu,
  Histogram,
  Lock,
  Monitor,
  Opportunity,
  Notebook,
  Odometer,
  Setting,
  Share,
  Timer,
  Upload,
  User,
  UserFilled,
} from "@element-plus/icons-vue"

export type AppNavItem = {
  key: string
  to: string
  label: string
  icon: Component
  match?: string[]
  mobile?: boolean
}

export type AppNavGroup = {
  key: string
  items: AppNavItem[]
}

export const desktopNavGroups: AppNavGroup[] = [
  {
    key: "core",
    items: [
      { key: "chat", to: "/chat", label: "聊天", icon: ChatDotRound, mobile: true },
      { key: "dashboard", to: "/dashboard", label: "概览", icon: Odometer },
    ],
  },
  {
    key: "setup",
    items: [
      { key: "wechat", to: "/wechat", label: "微信连接", icon: Connection },
      { key: "qq", to: "/qq", label: "QQ 连接", icon: ChatDotSquare },
      { key: "model", to: "/model", label: "模型配置", icon: Cpu, match: ["/model/"] },
      { key: "character", to: "/character", label: "角色管理", icon: UserFilled, match: ["/character/"], mobile: true },
      { key: "reminders", to: "/reminders", label: "日程提醒", icon: Bell },
    ],
  },
  {
    key: "records",
    items: [
      { key: "logs", to: "/logs", label: "聊天记录", icon: ChatLineSquare, mobile: true },
      { key: "import", to: "/import", label: "导入记录", icon: Upload },
    ],
  },
  {
    key: "memory",
    items: [
      { key: "memoryManager", to: "/memory-manager", label: "记忆总览", icon: Collection, mobile: true },
      { key: "profiles", to: "/profiles", label: "用户画像", icon: User },
      { key: "episodic", to: "/episodic", label: "情景记忆", icon: Timer },
      { key: "worldBook", to: "/world-book", label: "世界书", icon: Notebook },
      { key: "graph", to: "/graph", label: "记忆图谱", icon: Share },
      { key: "memoryTimeline", to: "/memory-timeline", label: "时间线", icon: Histogram },
    ],
  },
  {
    key: "system",
    items: [
      { key: "safety", to: "/safety", label: "安全设置", icon: Lock },
      { key: "maintenance", to: "/maintenance", label: "维护诊断", icon: Monitor },
      { key: "runtimeDebug", to: "/runtime-debug", label: "运行时调试", icon: Opportunity },
      { key: 'decisionViz', to: '/decision-viz', label: '决策可视化', icon: Share },
      { key: "settings", to: "/settings", label: "设置", icon: Setting, mobile: true },
    ],
  },
]

export const mobileNavItems = desktopNavGroups.flatMap((group) =>
  group.items.filter((item) => item.mobile)
)

const titleItems = desktopNavGroups.flatMap((group) => group.items)

const extraTitles = [
  { path: "/login", label: "登录" },
  { path: "/onboarding", label: "引导" },
  { path: "/setup", label: "初始化" },
  { path: "/privacy", label: "隐私说明" },
  { path: "/usage-boundary", label: "使用边界" },
  { path: "/storage", label: "存储清理" },
  { path: "/runtime-mode", label: "运行模式" },
  { path: "/runtime-debug", label: "运行时调试" },
]

export function isNavItemActive(path: string, item: AppNavItem) {
  if (path === item.to) {
    return true
  }
  return (item.match || []).some((prefix) => path.startsWith(prefix))
}

export function getPageTitle(path: string) {
  const navItem = titleItems.find((item) => isNavItemActive(path, item))
  if (navItem) {
    return navItem.label
  }
  const extra = extraTitles.find((item) => item.path === path)
  if (extra) {
    return extra.label
  }
  return "AI-Amitia"
}
