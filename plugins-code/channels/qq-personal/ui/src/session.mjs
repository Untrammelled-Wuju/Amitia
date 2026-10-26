export function unwrapResult(result) {
  let value = result
  for (let depth = 0; depth < 6; depth += 1) {
    if (!value || typeof value !== "object") break
    if (value.success === false || value.ok === false || (typeof value.code === "number" && value.code !== 0 && value.code !== 200)) {
      throw new Error(value.error?.message || value.error || value.message || value.msg || "QQ 个人号服务请求失败")
    }
    if (!("data" in value) || value.data == null) break
    value = value.data
  }
  return value ?? {}
}

export function createChannelClient(bridge) {
  if (!bridge || typeof bridge.invokeAction !== "function") {
    throw new Error("插件连接尚未就绪")
  }
  return async (action, input = {}) => unwrapResult(await bridge.invokeAction(action, { channelId: "qq_personal", ...input }))
}

export function createHostBridge(actions, navigate) {
  return {
    ready: async () => {},
    invokeAction: async (action, input = {}) => {
      const invoke = actions?.[action]
      if (typeof invoke !== "function") throw new Error(`宿主未提供操作：${action}`)
      return invoke(input)
    },
    navigate: async (target, type) => {
      if (typeof navigate !== "function") throw new Error("当前宿主不支持页面跳转")
      return navigate(target, type)
    },
  }
}

export function statusLabel(status) {
  return {
    online: "已连接",
    waiting: "等待 SnowLuma",
    disconnected: "已停用",
    error: "连接异常",
  }[status] || "等待 SnowLuma"
}
