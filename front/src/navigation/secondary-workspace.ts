const pages = [
  { path: "/settings", title: "设置", kind: "settings", sections: [] },
  { path: "/my-space", title: "我的空间", kind: "account", sections: [
    { id: "profile", path: "/my-space/profile", label: "资料与头像" },
    { id: "identity", path: "/my-space/identity", label: "Space 身份" },
    { id: "runtime", path: "/my-space/runtime", label: "运行模式" },
    { id: "devices", path: "/my-space/devices", label: "设备配对" },
  ] },
];

export function resolveSecondaryPage(target: string) {
  const path = target.split(/[?#]/)[0];
  return pages.find(page => path === page.path || path.startsWith(`${page.path}/`));
}
