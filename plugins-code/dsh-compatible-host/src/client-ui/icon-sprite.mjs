const CONTAINER_ID = "dsh-icon-sprite";
const SYMBOL_ID = /^[A-Za-z0-9_-]+$/;

let references = 0;
let container = null;

function attributeValue(attributes, name) {
  const match = new RegExp(`\\s${name}="([^"]*)"`).exec(attributes);
  return match ? match[1] : "";
}

function symbolMarkup(id, markup) {
  const text = typeof markup === "string" ? markup.trim() : "";
  if (!SYMBOL_ID.test(id) || !text.startsWith("<svg") || !text.endsWith("</svg>")) return "";
  const open = /^<svg([^>]*)>/.exec(text);
  if (!open) return "";
  const viewBox = attributeValue(open[1], "viewBox");
  if (!viewBox) return "";
  const attributes = open[1].replace(/\s(?:width|height|xmlns|id)="[^"]*"/g, "").trim();
  const inner = text.slice(open[0].length, -"</svg>".length);
  return `<symbol id="${id}" viewBox="${viewBox}"${attributes ? ` ${attributes}` : ""}>${inner}</symbol>`;
}

export function renderDshIconSprite(symbols) {
  if (!symbols || typeof symbols !== "object") return "";
  const parts = [];
  for (const [id, markup] of Object.entries(symbols)) {
    const symbol = symbolMarkup(id, markup);
    if (symbol) parts.push(symbol);
  }
  return parts.join("");
}

function buildContainer(markup) {
  const element = document.createElement("div");
  element.id = CONTAINER_ID;
  element.setAttribute("aria-hidden", "true");
  element.style.cssText = "position:absolute;width:0;height:0;overflow:hidden;pointer-events:none";
  const svg = document.createElementNS("http://www.w3.org/2000/svg", "svg");
  svg.setAttribute("xmlns", "http://www.w3.org/2000/svg");
  svg.innerHTML = markup;
  element.append(svg);
  return element;
}

function adoptExisting() {
  const existing = document.getElementById(CONTAINER_ID);
  if (existing && existing.querySelector("symbol")) return existing;
  if (existing?.isConnected) existing.remove();
  return null;
}

export function acquireDshIconSprite(symbols) {
  if (typeof document === "undefined") return () => {};
  if (!container || !container.isConnected) container = adoptExisting();
  if (!container) {
    const markup = renderDshIconSprite(symbols);
    if (!markup) return () => {};
    container = buildContainer(markup);
    (document.body ?? document.documentElement).append(container);
  }
  references += 1;
  let released = false;
  return () => {
    if (released) return;
    released = true;
    references = Math.max(0, references - 1);
    if (references > 0) return;
    if (container?.isConnected) container.remove();
    container = null;
  };
}
