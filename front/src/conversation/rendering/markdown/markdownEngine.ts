import DOMPurify from "dompurify";
import katex from "katex";
import MarkdownIt from "markdown-it";
import footnote from "markdown-it-footnote";
import taskLists from "markdown-it-task-lists";
import type Token from "markdown-it/lib/token.mjs";
import type { MarkdownSegment } from "../types";

const mathPlaceholderPrefix = "AMITIAMATHTOKEN";

const markdown = new MarkdownIt("default", {
  html: false,
  linkify: true,
  typographer: true,
  breaks: true,
})
  .use(taskLists, { enabled: true, label: true, labelAfter: true })
  .use(footnote);

markdown.validateLink = (url: string) => {
  const value = String(url ?? "").trim().toLowerCase();
  return (
    value.startsWith("http://") ||
    value.startsWith("https://") ||
    value.startsWith("mailto:") ||
    value.startsWith("amitia://") ||
    value.startsWith("/")
  );
};

markdown.renderer.rules.link_open = (tokens, index, options, _env, renderer) => {
  const token = tokens[index];
  token.attrSet("target", "_blank");
  token.attrSet("rel", "noopener noreferrer");
  return renderer.renderToken(tokens, index, options);
};

markdown.renderer.rules.image = (tokens, index, options, _env, renderer) => {
  const token = tokens[index];
  token.attrSet("loading", "lazy");
  token.attrSet("decoding", "async");
  token.attrSet("data-amitia-image", "true");
  return renderer.renderToken(tokens, index, options);
};

markdown.renderer.rules.table_open = () => '<div class="amrp-table-scroll"><table>';
markdown.renderer.rules.table_close = () => "</table></div>";

markdown.inline.ruler.before("link", "amrp_citation", (state, silent) => {
  if (state.src[state.pos] !== "[") return false;
  const match = /^\[(\d+)\]/.exec(state.src.slice(state.pos));
  if (!match) return false;
  if (!silent) {
    const token = state.push("amrp_citation", "button", 0);
    token.content = match[1];
  }
  state.pos += match[0].length;
  return true;
});

markdown.renderer.rules.amrp_citation = (tokens, index, _options, env) => {
  const id = escapeHtml(tokens[index].content);
  const citationIds = Array.isArray(env?.citationIds)
    ? new Set(env.citationIds.map((value: unknown) => String(value)))
    : null;
  if (citationIds && citationIds.size > 0 && !citationIds.has(tokens[index].content)) {
    return `[${id}]`;
  }
  return `<button type="button" class="amrp-citation-ref" data-citation-id="${id}">[${id}]</button>`;
};

function escapeHtml(value: string): string {
  return value
    .replace(/&/g, "&amp;")
    .replace(/</g, "&lt;")
    .replace(/>/g, "&gt;")
    .replace(/"/g, "&quot;");
}

function fenceSegment(
  id: string,
  language: string,
  filename: string,
  content: string,
  streaming: boolean,
): MarkdownSegment {
  const normalized = language.trim().toLowerCase();
  if (normalized === "diff" || normalized === "patch") {
    return { id, type: "diff", content, language: "diff", filename: filename || undefined, streaming };
  }
  if (["terminal", "console", "shell", "bash", "sh", "powershell", "cmd"].includes(normalized)) {
    return { id, type: "terminal", content, language: normalized || "shell", filename: filename || undefined, streaming };
  }
  if (normalized === "mermaid") {
    return { id, type: "mermaid", content, language: "mermaid", filename: filename || undefined, streaming };
  }
  if (normalized === "html-preview") {
    return { id, type: "html-preview", content, language: "html", filename: filename || undefined, streaming };
  }
  if (["latex", "tex", "math"].includes(normalized)) {
    return { id, type: "latex", content, language: normalized, filename: filename || undefined, streaming };
  }
  return {
    id,
    type: "code",
    content,
    language: normalized || "text",
    filename: filename || undefined,
    streaming,
  };
}

export function splitMarkdownSegments(source: string): MarkdownSegment[] {
  const value = String(source ?? "").replace(/\r\n?/g, "\n");
  const lines = value.split("\n");
  const segments: MarkdownSegment[] = [];
  let markdownLines: string[] = [];
  let segmentIndex = 0;

  const flushMarkdown = () => {
    if (markdownLines.length === 0) return;
    const content = markdownLines.join("\n");
    if (content.trim()) {
      segments.push({ id: `md-${segmentIndex++}`, type: "markdown", content });
    }
    markdownLines = [];
  };

  for (let index = 0; index < lines.length; index += 1) {
    const line = lines[index];
    const fence = /^\s*(`{3,}|~{3,})\s*([^\s`]*)?\s*(.*?)\s*$/.exec(line);
    if (fence) {
      flushMarkdown();
      const marker = fence[1];
      const language = fence[2] ?? "";
      const meta = fence[3] ?? "";
      const body: string[] = [];
      let closed = false;
      while (index + 1 < lines.length) {
        index += 1;
        const current = lines[index];
        if (new RegExp(`^\\s*${marker[0]}{${marker.length},}\\s*$`).test(current)) {
          closed = true;
          break;
        }
        body.push(current);
      }
      const filenameMatch = /(?:filename|file|title)=("[^"]+"|'[^']+'|\S+)/i.exec(meta);
      const filename = filenameMatch?.[1]?.replace(/^["']|["']$/g, "") ?? "";
      segments.push(
        fenceSegment(
          `fence-${segmentIndex++}`,
          language,
          filename,
          body.join("\n"),
          !closed,
        ),
      );
      continue;
    }

    const blockMathStart = /^\s*(\$\$|\\\[)\s*$/.exec(line);
    if (blockMathStart) {
      flushMarkdown();
      const marker = blockMathStart[1];
      const closePattern = marker === "$$" ? /^\s*\$\$\s*$/ : /^\s*\\\]\s*$/;
      const body: string[] = [];
      let closed = false;
      while (index + 1 < lines.length) {
        index += 1;
        if (closePattern.test(lines[index])) {
          closed = true;
          break;
        }
        body.push(lines[index]);
      }
      segments.push({
        id: `latex-${segmentIndex++}`,
        type: "latex",
        content: body.join("\n"),
        language: "latex",
        streaming: !closed,
      });
      continue;
    }

    markdownLines.push(line);
  }

  flushMarkdown();
  return segments;
}

export function renderMarkdownSegment(source: string, citationIds: string[] = []): string {
  const mathTokens: string[] = [];
  let protectedSource = String(source ?? "").replace(/\$\$([\s\S]+?)\$\$/g, (_match, formula: string) => {
    const index = mathTokens.length;
    mathTokens.push(
      katex.renderToString(formula.trim(), {
        displayMode: true,
        throwOnError: false,
        strict: "ignore",
        trust: false,
        output: "htmlAndMathml",
      }),
    );
    return `\n${mathPlaceholderPrefix}${index}END\n`;
  });
  protectedSource = protectedSource.replace(/(?<!\\)\$([^$\n]+?)\$/g, (_match, formula: string) => {
    const index = mathTokens.length;
    mathTokens.push(
      katex.renderToString(formula.trim(), {
        displayMode: false,
        throwOnError: false,
        strict: "ignore",
        trust: false,
        output: "htmlAndMathml",
      }),
    );
    return `${mathPlaceholderPrefix}${index}END`;
  });

  let html = markdown.render(protectedSource, { citationIds });
  html = html.replace(new RegExp(`${mathPlaceholderPrefix}(\\d+)END`, "g"), (_match, index: string) => {
    const rendered = mathTokens[Number(index)] ?? "";
    return `<span class="amrp-katex">${rendered}</span>`;
  });
  return DOMPurify.sanitize(html, {
    USE_PROFILES: { html: true, svg: true, mathMl: true },
    ADD_ATTR: [
      "target",
      "rel",
      "loading",
      "decoding",
      "data-amitia-image",
      "data-citation-id",
      "aria-hidden",
      "focusable",
    ],
  });
}

export function isKnownCodeLanguage(language: string, supported: string[]): boolean {
  const normalized = language.trim().toLowerCase();
  return supported.includes(normalized);
}

export function tokenText(token: Token): string {
  return token.content;
}
