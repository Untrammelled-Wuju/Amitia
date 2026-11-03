import { createIdentityExtension } from "./common.mjs";

async function twilio(requestJSON, credential, config, input, operation, encodeBasic) {
  const accountSid = config.provider.accountSid;
  if (!accountSid) throw new Error("Twilio Account SID 未配置");
  const base = `https://api.twilio.com/2010-04-01/Accounts/${encodeURIComponent(accountSid)}`;
  const headers = { Authorization: encodeBasic(accountSid, credential) };
  if (operation === "sms.send") {
    const body = new URLSearchParams({
      To: input.to,
      From: input.from || config.provider.phoneNumber,
      Body: input.body || "",
    });
    return requestJSON(`${base}/Messages.json`, {
      method: "POST",
      headers: { ...headers, "Content-Type": "application/x-www-form-urlencoded" },
      body: body.toString(),
    });
  }
  if (operation === "messages.list") {
    return requestJSON(`${base}/Messages.json?PageSize=${Math.min(Number(input.limit || 20), 100)}`, { headers });
  }
  if (operation === "call.create") {
    const body = new URLSearchParams({
      To: input.to,
      From: input.from || config.provider.phoneNumber,
      Twiml: input.twiml || `<Response><Say language="zh-CN">${String(input.message || "Amitia Agent 来电").replace(/[<>&]/g, "")}</Say></Response>`,
    });
    return requestJSON(`${base}/Calls.json`, {
      method: "POST",
      headers: { ...headers, "Content-Type": "application/x-www-form-urlencoded" },
      body: body.toString(),
    });
  }
  if (operation === "calls.list") {
    return requestJSON(`${base}/Calls.json?PageSize=${Math.min(Number(input.limit || 20), 100)}`, { headers });
  }
  throw new Error(`不支持的手机号操作: ${operation}`);
}

const extension = createIdentityExtension({
  kind: "phone",
  defaultConfig: {
    agentId: "",
    enabled: true,
    provider: { type: "twilio", accountSid: "", phoneNumber: "" },
    credentialRef: "",
    defaults: { from: "" },
  },
  async execute({ input, config, credential, requestJSON, encodeBasic }) {
    const operation = String(input.operation || "messages.list");
    if (!credential) throw new Error("电话 Provider 凭据未配置");
    if (config.provider.type === "twilio") {
      return twilio(requestJSON, credential, config, input, operation, encodeBasic);
    }
    throw new Error(`不支持的手机号 Provider: ${config.provider.type}`);
  },
});

if (typeof module !== "undefined" && module.exports) module.exports = extension;
if (typeof globalThis.defineExtension === "function") globalThis.defineExtension(extension);
