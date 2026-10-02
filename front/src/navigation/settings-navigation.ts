export const settingsGroups = [
  { title: "使用偏好", items: [
    { label: "通用", path: "/settings/system" },
    { label: "外观", path: "/settings/theme" },
    { label: "通知", path: "/settings/notifications" },
    { label: "时间与地区", path: "/settings/temporal" },
  ] },
  { title: "AI 配置", items: [
    { label: "模型配置", path: "/settings/model" },
    { label: "搜索 API", path: "/settings/search-api" },
  ] },
  { title: "数据与安全", items: [
    { label: "数据管理", path: "/settings/data-management" },
    { label: "安全", path: "/settings/safety" },
  ] },
  { title: "隐私", items: [
    { label: "隐私说明", path: "/settings/privacy" },
    { label: "使用边界", path: "/settings/usage-boundary" },
    { label: "隐私扫描", path: "/settings/privacy-scan" },
  ] },
  { title: "运行与部署", items: [
    { label: "运行概览", path: "/settings/overview" },
    { label: "运行统计", path: "/settings/data" },
    { label: "运行维护", path: "/settings/runtime" },
    { label: "部署模式", path: "/settings/deployment" },
  ] },
  { title: "诊断与高级", items: [
    { label: "界面提供者", path: "/settings/ui-providers" },
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
