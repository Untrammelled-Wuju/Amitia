const serviceTransitionMarkers = [
  "服务已变化，旧项目列表已丢弃",
  "服务状态或加载请求已变化",
  "服务状态已变化",
  "云端服务连接正在变化",
  "当前回复已中断",
  "调用角色已切换",
];

export function isServiceTransitionMessage(value: unknown): boolean {
  const message = value instanceof Error ? value.message : String(value || "");
  return serviceTransitionMarkers.some((marker) => message.includes(marker));
}
