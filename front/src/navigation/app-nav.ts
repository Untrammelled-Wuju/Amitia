import type { Component } from "vue"
import {
  ChatDotRound,
  ChatDotSquare,
  Connection,
  Odometer,
  Opportunity,
  Setting,
  Share,
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
      { key: "dashboard", to: "/dashboard/data", label: "概览", icon: Odometer, match: ["/dashboard/data", "/dashboard/run"] },
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
  { path: "/user-settings", label: "用户信息" },
  { path: "/emotes", label: "表情包管理" },
  { path: "/extensions/mcp", label: "MCP 服务" },
  { path: "/extensions/skills", label: "技能管理" },
  { path: "/extensions/plugins", label: "插件管理" },
  { path: "/extensions/workshop", label: "扩展工坊" },
  { path: "/extensions/runs", label: "技能执行记录" },
  { path: "/extensions", label: "扩展中心" },
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
