import { createIdentityExtension } from "./common.mjs";

function modelPath(provider) {
  return provider === "microsoft" ? "me" : "me";
}

async function gmail(requestJSON, credential, config, input, operation) {
  const base = "https://gmail.googleapis.com/gmail/v1/users/me";
  const headers = { Authorization: `Bearer ${credential}` };
  if (operation === "inbox.list") {
    const query = input.query ? `&q=${encodeURIComponent(input.query)}` : "";
    return requestJSON(`${base}/messages?maxResults=${Math.min(Number(input.limit || 20), 100)}${query}`, { headers });
  }
  if (operation === "message.get") {
    return requestJSON(`${base}/messages/${encodeURIComponent(input.messageId)}?format=full`, { headers });
  }
  if (operation === "message.send") {
    const raw = Buffer.from(
      [
        `To: ${input.to}`,
        `From: ${input.from || "me"}`,
        `Subject: ${input.subject || ""}`,
        "MIME-Version: 1.0",
        "Content-Type: text/plain; charset=UTF-8",
        "",
        [input.body || "", config.provider.signature ? `\n-- \n${config.provider.signature}` : ""].join(""),
      ].join("\r\n"),
    )
      .toString("base64")
      .replace(/\+/g, "-")
      .replace(/\//g, "_")
      .replace(/=+$/, "");
    return requestJSON(`${base}/messages/send`, {
      method: "POST",
      headers,
      body: JSON.stringify({ raw }),
    });
  }
  throw new Error(`不支持的 Gmail 操作: ${operation}`);
}

async function microsoft(requestJSON, credential, config, input, operation) {
  const base = `https://graph.microsoft.com/v1.0/${modelPath("microsoft")}`;
  const headers = { Authorization: `Bearer ${credential}` };
  if (operation === "inbox.list") {
    return requestJSON(`${base}/mailFolders/inbox/messages?$top=${Math.min(Number(input.limit || 20), 100)}`, { headers });
  }
  if (operation === "message.get") {
    return requestJSON(`${base}/messages/${encodeURIComponent(input.messageId)}`, { headers });
  }
  if (operation === "message.send") {
    return requestJSON(`${base}/sendMail`, {
      method: "POST",
      headers,
      body: JSON.stringify({
        message: {
          subject: input.subject || "",
          body: {
            contentType: "Text",
            content: [input.body || "", config.provider.signature ? `\n-- \n${config.provider.signature}` : ""].join(""),
          },
          toRecipients: [{ emailAddress: { address: input.to } }],
        },
        saveToSentItems: true,
      }),
    });
  }
  throw new Error(`不支持的 Microsoft Graph 邮箱操作: ${operation}`);
}

const extension = createIdentityExtension({
  kind: "email",
  defaultConfig: {
    agentId: "",
    enabled: true,
    provider: { type: "gmail", address: "", signature: "" },
    credentialRef: "",
    defaults: { from: "", replyTo: "" },
  },
  async execute({ input, config, credential, requestJSON }) {
    const operation = String(input.operation || "inbox.list");
    if (!credential) throw new Error("邮箱凭据未配置");
    if (config.provider.type === "gmail") {
      return gmail(requestJSON, credential, config, input, operation);
    }
    if (config.provider.type === "microsoft") {
      return microsoft(requestJSON, credential, config, input, operation);
    }
    throw new Error(`不支持的邮箱 Provider: ${config.provider.type}`);
  },
});

if (typeof module !== "undefined" && module.exports) module.exports = extension;
if (typeof globalThis.defineExtension === "function") globalThis.defineExtension(extension);
