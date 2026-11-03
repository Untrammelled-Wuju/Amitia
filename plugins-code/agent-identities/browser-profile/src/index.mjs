import { createIdentityExtension } from "./common.mjs";

async function browserCommand(requestJSON, config, credential, input) {
  const base = config.provider.baseUrl.replace(/\/+$/, "");
  if (!base) throw new Error("浏览器 Profile Provider 地址未配置");
  const headers = credential ? { Authorization: `Bearer ${credential}` } : {};
  const operation = String(input.operation || "pages.list");
  if (config.provider.type === "cdp") {
    if (operation === "pages.list") return requestJSON(`${base}/json/list`, { headers });
    if (operation === "page.open") {
      return requestJSON(`${base}/json/new?${encodeURIComponent(input.url || "about:blank")}`, { method: "PUT", headers });
    }
    if (operation === "page.close") {
      return requestJSON(`${base}/json/close/${encodeURIComponent(input.targetId)}`, { headers });
    }
    if (operation === "profile.status") return requestJSON(`${base}/json/version`, { headers });
  }
  if (config.provider.type === "generic") {
    if (operation === "pages.list") {
      return requestJSON(`${base}/profiles/${encodeURIComponent(config.provider.profileId)}/pages`, { headers });
    }
    if (operation === "page.open") {
      return requestJSON(`${base}/profiles/${encodeURIComponent(config.provider.profileId)}/pages`, {
        method: "POST", headers, body: JSON.stringify({ url: input.url }),
      });
    }
    if (operation === "page.close") {
      return requestJSON(`${base}/profiles/${encodeURIComponent(config.provider.profileId)}/pages/${encodeURIComponent(input.targetId)}`, { method: "DELETE", headers });
    }
    if (operation === "profile.status") {
      return requestJSON(`${base}/profiles/${encodeURIComponent(config.provider.profileId)}`, { headers });
    }
  }
  throw new Error(`浏览器 Profile Provider 不支持操作: ${operation}`);
}

const extension = createIdentityExtension({
  kind: "browser-profile",
  defaultConfig: {
    agentId: "",
    enabled: true,
    provider: { type: "cdp", baseUrl: "", profileId: "" },
    credentialRef: "",
    defaults: { homePage: "about:blank" },
  },
  async execute({ input, config, credential, requestJSON }) {
    return browserCommand(requestJSON, config, credential, input);
  },
});

if (typeof module !== "undefined" && module.exports) module.exports = extension;
if (typeof globalThis.defineExtension === "function") globalThis.defineExtension(extension);
