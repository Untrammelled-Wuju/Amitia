export function unwrapResult(result) {
  let value = result
  for (let depth = 0; depth < 6; depth++) {
    if (!value || typeof value !== "object") break
    if (value.success === false || value.ok === false || (typeof value.code === "number" && value.code !== 0 && value.code !== 200)) {
      throw new Error(value.error?.message || value.error || value.message || value.msg || "QQ 服务请求失败")
    }
    if (!("data" in value) || value.data == null) break
    value = value.data
  }
  return value ?? {}
}

export function statusLabel(status) {
  return {
    online: "已连接",
    connected: "已连接",
    connecting: "连接中",
    disconnected: "未连接",
    error: "连接异常",
  }[status] || "未连接"
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

export function createChannelClient(bridge, { now = Date.now } = {}) {
  if (!bridge || typeof bridge.invokeAction !== "function") {
    throw new Error("插件连接尚未就绪，请关闭页面后重新打开")
  }
  const calls = []
  let blockedUntil = 0
  let lastStatusAt = -Infinity
  let lastStatus
  let pendingStatus

  function prune() {
    while (calls.length && calls[0] <= now() - 60000) calls.shift()
  }

  function limited() {
    const error = new Error("插件操作较频繁，请稍后再试")
    error.retryAfterMs = Math.max(1000, blockedUntil - now())
    return error
  }

  async function request(action, input = {}) {
    prune()
    if (now() < blockedUntil) throw limited()
    if (calls.length >= 24) {
      blockedUntil = calls[0] + 60000
      throw limited()
    }
    calls.push(now())
    try {
      const result = unwrapResult(await bridge.invokeAction(action, { channelId: "qq", ...input }))
      if (action === "status") {
        lastStatus = result
        lastStatusAt = now()
      } else {
        lastStatusAt = -Infinity
      }
      return result
    } catch (error) {
      if (/rate.?limit|429/i.test(String(error?.message || error))) {
        blockedUntil = now() + 60000
        throw limited()
      }
      throw error
    }
  }

  const client = (action, input = {}) => {
    if (action !== "status") return request(action, input)
    if (pendingStatus) return pendingStatus
    prune()
    if (lastStatus && now() - lastStatusAt < 3000) return Promise.resolve(lastStatus)
    pendingStatus = request(action, input).finally(() => {
      pendingStatus = undefined
    })
    return pendingStatus
  }

  client.nextPollDelay = (active) => Math.max(active ? 4000 : 12000, blockedUntil - now())
  return client
}
