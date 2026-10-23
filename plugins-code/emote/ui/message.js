const $ = (id) => document.getElementById(id);

function resolve(url) {
  if (!url) return "";
  return url;
}

function render(context) {
  const message = context.uiContext?.message || context.message || {};
  const role = message.role || "assistant";
  const direction = role === "user" ? "outgoing" : "incoming";
  $("row").classList.toggle("outgoing", direction === "outgoing");
  $("name").textContent = role === "user" ? "" : (context.characterName || "");
  $("avatar").textContent = role === "user" ? "我" : ((context.characterName || "A").slice(0, 1));
  const original = resolve(message.originalAssetReference || message.imageUrl || message.image_url);
  const fallback = resolve(message.fallbackAssetReference || "");
  const image = $("image");
  let usingFallback = !original && !!fallback;
  image.src = usingFallback ? fallback : original;
  image.alt = message.altText || message.content || "表情";
  image.onerror = () => {
    if (!usingFallback && fallback && fallback !== original) {
      usingFallback = true;
      image.src = fallback;
      return;
    }
    image.hidden = true;
    $("fallback").hidden = false;
  };
  $("fallback").textContent = message.altText || message.content || "表情加载失败";
  const status = message.status || "";
  $("status").textContent = status === "sending" ? "发送中" : status === "failed" ? "发送失败" : "";
  $("status").classList.toggle("failed", status === "failed");
  requestAnimationFrame(() => window.amitiaUI.requestResize(Math.max(80, document.documentElement.scrollWidth), Math.max(80, document.documentElement.scrollHeight)));
}

window.amitiaUI.ready().then(async () => {
  const context = await window.amitiaUI.getContext();
  render(context);
  window.amitiaUI.onHostContextChange((next) => render(next));
}).catch(() => {});
