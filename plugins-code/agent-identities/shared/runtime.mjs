const STATE_PREFIX = "agent-identity:v1:";

function clone(value) {
  return JSON.parse(JSON.stringify(value));
}

function normalizeObject(value) {
  return value && typeof value === "object" && !Array.isArray(value) ? value : {};
}

async function readState(host, key) {
  const response = await host.call("host.state.get", { key: STATE_PREFIX + key });
  return {
    value: response && response.found ? response.value : null,
    version: response && Number.isFinite(Number(response.version)) ? Number(response.version) : 0,
  };
}

async function writeState(host, key, value) {
  const current = await readState(host, key);
  const result = await host.call("host.state.cas", {
    key: STATE_PREFIX + key,
    expectedVersion: current.version,
    newValue: value,
  });
  if (!result || result.swapped !== true) {
    throw new Error("身份插件状态写入冲突");
  }
  return result;
}

function mergeConfig(defaultConfig, current, patch) {
  return {
    ...defaultConfig,
    ...normalizeObject(current),
    ...normalizeObject(patch),
    provider: {
      ...normalizeObject(defaultConfig.provider),
      ...normalizeObject(current && current.provider),
      ...normalizeObject(patch && patch.provider),
    },
    defaults: {
      ...normalizeObject(defaultConfig.defaults),
      ...normalizeObject(current && current.defaults),
      ...normalizeObject(patch && patch.defaults),
    },
  };
}

async function resolveSecret(host, ref) {
  if (!ref) return "";
  const response = await host.call("host.secret.get", { ref });
  if (!response || typeof response.value !== "string" || response.value.length === 0) {
    throw new Error(`凭据读取失败: ${ref}`);
  }
  return response.value;
}

async function requestJSON(url, init = {}) {
  const response = await fetch(url, {
    ...init,
    headers: {
      Accept: "application/json",
      ...(init.body ? { "Content-Type": "application/json" } : {}),
      ...(init.headers || {}),
    },
    signal: AbortSignal.timeout(60000),
  });
  const text = await response.text();
  let body = null;
  if (text) {
    try {
      body = JSON.parse(text);
    } catch {
      body = text;
    }
  }
  if (!response.ok) {
    throw new Error(`Provider HTTP ${response.status}: ${typeof body === "string" ? body : JSON.stringify(body)}`);
  }
  return body;
}

function encodeBasic(username, password) {
  return `Basic ${Buffer.from(`${username}:${password}`).toString("base64")}`;
}

export function createIdentityExtension({ kind, defaultConfig, execute }) {
  let activeHost = null;
  const configKey = `${kind}:config`;

  async function command(input) {
    const payload = normalizeObject(input);
    const action = String(payload.action || "status");
    const current = await readState(activeHost, configKey);
    const config = mergeConfig(defaultConfig, current.value, {});
    if (action === "status") {
      return {
        kind,
        configured: Boolean(config.agentId && config.provider),
        provider: config.provider,
        enabled: config.enabled !== false,
        revision: current.version,
      };
    }
    if (action === "config") {
      return { kind, config, revision: current.version };
    }
    if (action === "configure") {
      const next = mergeConfig(defaultConfig, current.value, payload.config);
      if (next.credentialRef && !String(next.credentialRef).startsWith("secret:")) {
        throw new Error("credentialRef 必须使用 secret: 作用域引用");
      }
      await writeState(activeHost, configKey, next);
      return { kind, configured: true, provider: next.provider, revision: current.version + 1 };
    }
    if (action === "disable") {
      const next = mergeConfig(defaultConfig, current.value, { enabled: false });
      await writeState(activeHost, configKey, next);
      return { kind, enabled: false, revision: current.version + 1 };
    }
    if (action === "enable") {
      const next = mergeConfig(defaultConfig, current.value, { enabled: true });
      await writeState(activeHost, configKey, next);
      return { kind, enabled: true, revision: current.version + 1 };
    }
    if (config.enabled === false) {
      throw new Error(`${kind} 插件已停用`);
    }
    if (action !== "execute") {
      throw new Error(`未知身份插件操作: ${action}`);
    }
    const credential = payload.credentialRef
      ? await resolveSecret(activeHost, payload.credentialRef)
      : config.credentialRef
        ? await resolveSecret(activeHost, config.credentialRef)
        : "";
    return execute({
      input: normalizeObject(payload.input),
      config: clone(config),
      credential,
      host: activeHost,
      requestJSON,
      encodeBasic,
    });
  }

  return {
    async activate(context) {
      activeHost = context.host;
      context.handlers.bindTool("command", command);
      context.log.info(`${kind} identity plugin activated`);
    },
    async deactivate() {
      activeHost = null;
    },
  };
}
