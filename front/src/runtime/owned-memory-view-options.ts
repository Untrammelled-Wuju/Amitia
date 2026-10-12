export function ownedMemoryViewOptions(source: string, roles: Array<{ id: string; name: string }>, historicalRoles: Array<{ id: string; name: string }>, kind = "memory") {
  if (source !== "current") {
    const role = historicalRoles.find(item => source === `history:${item.id}`);
    return [{ label: kind === "profile" ? "原设备用户画像" : role ? `原设备历史 · ${role.name}` : "原设备历史", value: "all" }];
  }
  if (kind === "profile") return [{ label: "全局用户画像", value: "global" }];
  const noun = kind === "continuity" ? "事项" : "记忆";
  return [
    { label: `全部${noun}`, value: "all" },
    { label: `全局${noun}`, value: "global" },
    ...roles.map(role => ({ label: role.name, value: `role:${role.id}` })),
  ];
}
