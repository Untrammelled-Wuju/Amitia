export const settingsGroups = [
  { title: "AI 配置", items: [
    { label: "模型配置", path: "/settings/model" },
    { label: "搜索 API", path: "/settings/search-api" },
  ] },
  { title: "外观与通知", items: [
    { label: "系统设置", path: "/settings/system" },
    { label: "通知", path: "/settings/notifications" },
    { label: "界面提供者", path: "/settings/ui-providers" },
  ] },
  { title: "运行与部署", items: [
    { label: "运行概览", path: "/settings/overview" },
    { label: "运行数据", path: "/settings/data" },
    { label: "运行维护", path: "/settings/runtime" },
    { label: "部署模式", path: "/settings/deployment" },
  ] },
  { title: "安全", items: [{ label: "安全", path: "/settings/safety" }] },
  { title: "诊断与高级", items: [
    { label: "维护诊断", path: "/settings/maintenance" },
    { label: "运行日志", path: "/settings/system-logs" },
    { label: "Prompt Trace", path: "/settings/prompt-trace" },
    { label: "高级系统", path: "/settings/advanced" },
  ] },
  { title: "关于", items: [{ label: "关于", path: "/settings/about" }] },
];

export function resolveSettingsEntry(path: string): string {
  return settingsGroups.flatMap(group => group.items).find(
    item => path === item.path || path.startsWith(`${item.path}/`),
  )?.path ?? "";
}
