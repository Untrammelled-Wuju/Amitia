import { resolveConversationMediaUrl, resolveEmbeddedConversationMedia } from "../media";

export async function buildHtmlPreviewDocument(source: string, baseUrl = ""): Promise<string> {
  const document = new DOMParser().parseFromString(source, "text/html");
  const base = document.querySelector("base[href]")?.getAttribute("href") || baseUrl;
  document.querySelectorAll("base").forEach((node) => node.remove());
  await Promise.all(Array.from(document.querySelectorAll("[src],link[href],video[poster]")).map(async (element) => {
    const attribute = element.tagName === "LINK" ? "href" : element.hasAttribute("src") ? "src" : "poster";
    const raw = element.getAttribute(attribute) || "";
    if (/^amitia:\/\/artifacts\//i.test(raw) || /\/api\/artifacts\/v1\/[^/]+\/content/.test(raw)) {
      const embedded = element.tagName === "IMG" || attribute === "poster";
      element.setAttribute(attribute, await (embedded ? resolveEmbeddedConversationMedia(raw) : resolveConversationMediaUrl(raw)));
    }
  }));
  const head = document.createElement("template");
  head.innerHTML = `<meta charset="utf-8"><meta http-equiv="Content-Security-Policy" content="default-src 'none'; img-src https: http: data: blob:; style-src 'unsafe-inline' https: http:; script-src 'unsafe-inline' https: http:; font-src https: http: data:; connect-src 'none'; frame-src https: http: data: blob:; media-src https: http: data: blob:; base-uri https: http:; form-action 'none';"><meta name="viewport" content="width=device-width,initial-scale=1"><style>html,body{min-height:100%;margin:0;background:#fff;color:#19191c;font:14px/1.6 system-ui,sans-serif}body{padding:20px}*{box-sizing:border-box}</style>`;
  document.head.prepend(head.content);
  if (/^https?:\/\//i.test(base)) {
    const element = document.createElement("base");
    element.href = base;
    document.head.insertBefore(element, document.head.children[2]);
  }
  return `<!doctype html>${document.documentElement.outerHTML}`;
}
