import { createHighlighter, type Highlighter } from "shiki";

const languages = [
  "dart",
  "go",
  "java",
  "javascript",
  "typescript",
  "vue",
  "jsx",
  "tsx",
  "python",
  "shellscript",
  "powershell",
  "sql",
  "json",
  "yaml",
  "xml",
  "html",
  "css",
  "markdown",
  "diff",
];

const aliases: Record<string, string> = {
  js: "javascript",
  ts: "typescript",
  py: "python",
  bash: "shellscript",
  sh: "shellscript",
  shell: "shellscript",
  ps1: "powershell",
  yml: "yaml",
  md: "markdown",
  patch: "diff",
};

let highlighterPromise: Promise<Highlighter> | null = null;
const highlightCache = new Map<string, string>();

function getHighlighter(): Promise<Highlighter> {
  highlighterPromise ??= createHighlighter({
    themes: ["github-dark", "github-light"],
    langs: languages,
  });
  return highlighterPromise;
}

export async function highlightCode(
  code: string,
  language: string,
  dark: boolean,
): Promise<string> {
  const normalized = aliases[language.trim().toLowerCase()] ?? language.trim().toLowerCase();
  const key = `${dark ? "dark" : "light"}:${normalized}:${code}`;
  const cached = highlightCache.get(key);
  if (cached) return cached;
  try {
    const highlighter = await getHighlighter();
    const loaded = highlighter.getLoadedLanguages();
    if (!loaded.includes(normalized)) return "";
    const html = highlighter.codeToHtml(code, {
      lang: normalized,
      theme: dark ? "github-dark" : "github-light",
      transformers: [
        {
          pre(node) {
            node.properties.class = `${String(node.properties.class ?? "")} amrp-shiki`;
          },
        },
      ],
    });
    highlightCache.set(key, html);
    return html;
  } catch {
    return "";
  }
}

