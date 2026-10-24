export interface AccountNavigationItem {
  id: string;
  path: string;
  label: string;
}

export const accountNavigation: AccountNavigationItem[] = [
  { id: "profile", path: "/my-space/profile", label: "资料与头像" },
  { id: "identity", path: "/my-space/identity", label: "Space 身份" },
  { id: "runtime", path: "/my-space/runtime", label: "运行模式" },
  { id: "devices", path: "/my-space/devices", label: "设备配对" },
];

export function resolveAccountEntry(path: string): string {
  return accountNavigation.find(item => path === item.path)?.path ?? accountNavigation[0].path;
}
