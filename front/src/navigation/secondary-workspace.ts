const pages = [
  { path: "/settings", title: "设置", kind: "settings", sections: [] },
  { path: "/user-settings", title: "个人资料", kind: "account", sections: [
    { id: "local-profile", label: "资料与头像" },
    { id: "space-identity", label: "Space 身份" },
    { id: "profile-device", label: "当前设备" },
  ] },
  { path: "/devices", title: "我的设备", kind: "account", sections: [
    { id: "device-pairing", label: "设备配对" },
    { id: "current-device", label: "当前设备" },
    { id: "bound-devices", label: "已绑定设备" },
  ] },
];

export function resolveSecondaryPage(target: string) {
  const path = target.split(/[?#]/)[0];
  return pages.find(page => path === page.path || path.startsWith(`${page.path}/`));
}
