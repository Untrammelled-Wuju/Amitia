import fs from "node:fs";
import path from "node:path";

const root = path.resolve(process.argv[2] || ".");
const mode = process.argv[3] || "audit";
const roots = ["mobile_app/android", "mobile_app/ios/Runner", "mobile_app/ios/AmitiaNotificationService"];
const extensions = new Set([".kt", ".java", ".swift"]);
const excludedSegments = new Set(["build", "generated", "test", "tests", "third_party"]);

function walk(directory, files) {
  if (!fs.existsSync(directory)) return;
  for (const entry of fs.readdirSync(directory, { withFileTypes: true })) {
    if (excludedSegments.has(entry.name)) continue;
    const absolute = path.join(directory, entry.name);
    if (entry.isDirectory()) {
      walk(absolute, files);
      continue;
    }
    if (entry.isFile() && extensions.has(path.extname(entry.name))) files.push(absolute);
  }
}

function blockEnd(content, openIndex) {
  let depth = 0;
  let quote = "";
  let escaped = false;
  let lineComment = false;
  let blockComment = false;
  for (let index = openIndex; index < content.length; index += 1) {
    const char = content[index];
    const next = content[index + 1];
    if (lineComment) {
      if (char === "\n") lineComment = false;
      continue;
    }
    if (blockComment) {
      if (char === "*" && next === "/") {
        blockComment = false;
        index += 1;
      }
      continue;
    }
    if (quote) {
      if (escaped) {
        escaped = false;
      } else if (char === "\\") {
        escaped = true;
      } else if (char === quote) {
        quote = "";
      }
      continue;
    }
    if (char === "/" && next === "/") {
      lineComment = true;
      index += 1;
      continue;
    }
    if (char === "/" && next === "*") {
      blockComment = true;
      index += 1;
      continue;
    }
    if (char === "\"" || char === "'" || char === "`") {
      quote = char;
      continue;
    }
    if (char === "{") depth += 1;
    if (char === "}") {
      depth -= 1;
      if (depth === 0) return index;
    }
  }
  return -1;
}

const files = [];
for (const entry of roots) walk(path.join(root, entry), files);

const findings = [];
let changed = 0;
for (const file of files) {
  const content = fs.readFileSync(file, "utf8");
  const relative = path.relative(root, file);
  const replacements = [];
  for (const match of content.matchAll(/catch\s*\(\s*([^):\s]+)\s*(?::\s*[^)]+)?\s*\)\s*\{/g)) {
    const name = match[1];
    const openIndex = content.indexOf("{", match.index);
    const endIndex = blockEnd(content, openIndex);
    if (endIndex < 0) continue;
    const body = content.slice(openIndex + 1, endIndex);
    if (name === "_" || !new RegExp(`\\b${name.replace(/[$]/g, "\\$&")}\\b`).test(body)) {
      const line = content.slice(0, match.index).split("\n").length;
      findings.push(`${relative}:${line}: native catch hides ${name}`);
      if (mode === "fix") {
        const replacementName = `originalError${line}`;
        const nameStart = match.index + match[0].indexOf(name);
        if (name === "_") replacements.push({ start: nameStart, end: nameStart + name.length, text: replacementName });
        const lineStart = content.lastIndexOf("\n", match.index) + 1;
        const indent = content.slice(lineStart, match.index).match(/^\s*/)[0];
        const report = `\n${indent}  ${name === "_" ? replacementName : name}.printStackTrace()\n${indent}`;
        replacements.push({ start: openIndex + 1, end: openIndex + 1, text: report });
      }
    } else {
      const withoutStackTrace = body
        .split("\n")
        .filter((line) => !line.includes(`${name}.printStackTrace()`))
        .join("\n");
      const returnsFailure = /\b(?:Result\.failure|completeExceptionally|failure\s*\(|fail\s*\(|error\s*\(|\.error\s*\(|errorResponse\s*\(|throw\b)/i.test(withoutStackTrace);
      if (returnsFailure && !new RegExp(`\\b${name.replace(/[$]/g, "\\$&")}\\b`).test(withoutStackTrace)) {
        const line = content.slice(0, match.index).split("\n").length;
        findings.push(`${relative}:${line}: native catch result hides ${name}`);
      }
    }
  }
  for (const match of content.matchAll(/catch\s*\{/g)) {
    const openIndex = match.index + match[0].length - 1;
    const endIndex = blockEnd(content, openIndex);
    if (endIndex < 0) continue;
    const body = content.slice(openIndex + 1, endIndex);
    if (!/\berror\b/.test(body)) {
      const line = content.slice(0, match.index).split("\n").length;
      findings.push(`${relative}:${line}: Swift catch hides implicit error`);
      if (mode === "fix") {
        const lineStart = content.lastIndexOf("\n", match.index) + 1;
        const indent = content.slice(lineStart, match.index).match(/^\s*/)[0];
        replacements.push({ start: openIndex + 1, end: openIndex + 1, text: `\n${indent}  print("[original-error] \\(error)")\n${indent}` });
      }
    } else {
      const withoutPrint = body
        .split("\n")
        .filter((line) => !line.includes('print("[original-error] \\(error)")'))
        .join("\n");
      const returnsFailure = /\b(?:fail\s*\(|sendError\s*\(|errorResponse\s*\(|throw\b)/.test(withoutPrint);
      if (returnsFailure && !/\berror\b/.test(withoutPrint)) {
        const line = content.slice(0, match.index).split("\n").length;
        findings.push(`${relative}:${line}: Swift catch result hides implicit error`);
      }
    }
  }
  if (mode === "fix" && replacements.length) {
    replacements.sort((left, right) => right.start - left.start);
    let next = content;
    for (const replacement of replacements) {
      next = `${next.slice(0, replacement.start)}${replacement.text}${next.slice(replacement.end)}`;
    }
    fs.writeFileSync(file, next);
    changed += 1;
  }
}

if (mode === "fix") console.log(`updated files: ${changed}`);
if (findings.length) {
  console.log(findings.join("\n"));
  process.exitCode = 1;
} else {
  console.log("silent native error handlers: 0");
}
