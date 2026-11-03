import { createIdentityExtension } from "./common.mjs";

const extension = createIdentityExtension({
  kind: "wallet",
  defaultConfig: {
    agentId: "",
    enabled: false,
    provider: { type: "generic", baseUrl: "", accountId: "" },
    credentialRef: "",
    defaults: { currency: "USD", allowTransfers: false },
  },
  async execute({ input, config, credential, requestJSON }) {
    const operation = String(input.operation || "account.get");
    if (!config.provider.baseUrl) throw new Error("钱包 Provider 地址未配置");
    if (!credential) throw new Error("钱包 Provider 凭据未配置");
    const base = config.provider.baseUrl.replace(/\/+$/, "");
    const headers = { Authorization: `Bearer ${credential}` };
    const accountId = encodeURIComponent(config.provider.accountId || input.accountId || "");
    if (operation === "account.get") {
      return requestJSON(`${base}/accounts/${accountId}`, { headers });
    }
    if (operation === "transactions.list") {
      return requestJSON(`${base}/accounts/${accountId}/transactions?limit=${Math.min(Number(input.limit || 50), 200)}`, { headers });
    }
    if (operation === "payment.preview") {
      return requestJSON(`${base}/payments/preview`, {
        method: "POST",
        headers,
        body: JSON.stringify({ accountId: config.provider.accountId, amount: input.amount, currency: input.currency || config.defaults.currency, payee: input.payee }),
      });
    }
    if (operation === "payment.execute") {
      if (!config.defaults.allowTransfers) throw new Error("钱包转账默认关闭");
      if (!input.approvalToken) throw new Error("钱包资产操作缺少独立审批令牌");
      return requestJSON(`${base}/payments`, {
        method: "POST",
        headers: { ...headers, "X-Amitia-Approval": input.approvalToken },
        body: JSON.stringify({ accountId: config.provider.accountId, amount: input.amount, currency: input.currency || config.defaults.currency, payee: input.payee, memo: input.memo || "", idempotencyKey: input.idempotencyKey }),
      });
    }
    throw new Error(`不支持的钱包操作: ${operation}`);
  },
});

if (typeof module !== "undefined" && module.exports) module.exports = extension;
if (typeof globalThis.defineExtension === "function") globalThis.defineExtension(extension);
