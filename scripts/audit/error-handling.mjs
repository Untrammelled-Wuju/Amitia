import fs from "node:fs";
import path from "node:path";

const root = path.resolve(process.argv[2] || ".");
const mode = process.argv[3] || "audit";
const roots = [
  "front/src",
  "desktop/src",
  "admin-system/src",
  "mobile_app/lib",
  "sdk",
  "plugins-code",
  "runtime/plugin-host/src",
  "runtime/task-host/src",
  "backend/pkg/gameplugin/sdk",
  "scripts",
  "front/scripts",
  "desktop/scripts",
  "backend/scripts",
];
const extensions = new Set([".ts", ".tsx", ".js", ".mjs", ".cjs", ".vue", ".dart"]);
const excludedSegments = new Set(["node_modules", "dist", "build", ".build", ".next", ".turbo", ".dart_tool", "coverage", "release", "release-mac-platform", "release-mac-sim"]);

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

function isTestFile(file) {
  return /(?:^|[/\\])(?:__tests__|test|tests)(?:[/\\]|$)/.test(file)
    || /\.(?:test|spec)\.[^.]+$/.test(file)
    || /_test\.dart$/.test(file);
}

function silencePatterns(content, file) {
  const remaining = [];
  const patterns = file.endsWith(".dart")
    ? [
        /catch\s*\{/g,
        /catch\s*\(\s*_\s*\)/g,
        /\.catchError\s*\(\s*\(\s*(?:Object\s+)?_\s*\)/g,
        /catchError\s*\(\s*\(\s*Object\s+_\s*\)\s*=>\s*null\s*\)/g,
      ]
    : [
        /catch\s*\{/g,
        /catch\s*\(\s*_\s*\)/g,
        /\.catch\s*\(\s*\(\s*\)\s*=>/g,
        /\.catch\s*\(\s*\(\s*\)\s*=>\s*(?:\{\s*\}|undefined|null|false|\[\]|\(\{\}\))\s*\)/g,
      ];
  for (const pattern of patterns) {
    for (const match of content.matchAll(pattern)) {
      remaining.push(`${file}: ${match[0].replace(/\s+/g, " ")}`);
    }
  }
  return remaining;
}

function catchBodyEnd(content, openIndex) {
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
    if (char === "'" || char === '"' || char === "`") {
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

function unusedCatchBindings(content, file) {
  const findings = [];
  const insertions = [];
  const pattern = /catch\s*\(([^)]*)\)\s*\{/g;
  for (const match of content.matchAll(pattern)) {
    const params = match[1].trim();
    const name = params.split(/[,\s:]/)[0];
    if (!name || name === "_") continue;
    const openIndex = content.indexOf("{", match.index);
    const endIndex = catchBodyEnd(content, openIndex);
    if (endIndex < 0) continue;
    const body = content.slice(openIndex + 1, endIndex);
    const usesError = new RegExp(`\\b${name}\\b`).test(body);
    if (usesError) continue;
    const line = content.slice(0, match.index).split("\n").length;
    findings.push(`${file}:${line}: unused catch binding ${name}`);
    const report = file.endsWith(".dart")
      ? `\n  print("[original-error] $${name}");`
      : `\n  console.error("[original-error]", ${name});`;
    insertions.push({ index: openIndex + 1, report });
  }
  return { findings, insertions };
}

function genericCatchMessages(content, file) {
  const findings = [];
  const pattern = /catch\s*\(([^)]*)\)\s*\{/g;
  const displayPattern = /(?:ElMessage\.(?:error|warning)|showSnackBar|SnackBar\s*\(\s*content:\s*Text|Message\.(?:error|warning)|message\.(?:error|warning)|toast\.(?:error|warning))\s*\(([\s\S]{0,500}?)\)/g;
  for (const match of content.matchAll(pattern)) {
    const params = match[1].trim();
    const name = params.split(/[,\s:]/)[0];
    if (!name || name === "_") continue;
    const openIndex = content.indexOf("{", match.index);
    const endIndex = catchBodyEnd(content, openIndex);
    if (endIndex < 0) continue;
    const body = content.slice(openIndex + 1, endIndex);
    for (const display of body.matchAll(displayPattern)) {
      const argument = display[1];
      if (new RegExp(`\\b${name}\\b`).test(argument)) continue;
      if (/(?:originalError|errorText|errorMessage|String\s*\(|\.toString\s*\()/.test(argument)) continue;
      if (!/(?:失败|错误|异常|无法|不可用|无效|failed|error|unavailable|invalid)/i.test(argument)) continue;
      const line = content.slice(0, openIndex + 1 + display.index).split("\n").length;
      findings.push(`${file}:${line}: generic catch message does not include original ${name}`);
    }
  }
  const promisePattern = /\.catch\s*\(\s*(?:\(\s*([^)]*?)\s*\)|([A-Za-z_$][\w$]*))\s*=>\s*/g;
  for (const match of content.matchAll(promisePattern)) {
    const params = String(match[1] || match[2] || "").trim();
    const name = params.split(/[,\s:]/)[0];
    if (!name || name === "_") continue;
    const bodyStart = match.index + match[0].length;
    let body = "";
    if (content[bodyStart] === "{") {
      const endIndex = catchBodyEnd(content, bodyStart);
      if (endIndex < 0) continue;
      body = content.slice(bodyStart + 1, endIndex);
    } else {
      body = content.slice(bodyStart, bodyStart + 1000).split(/\n\s*\)/)[0];
    }
    for (const display of body.matchAll(displayPattern)) {
      const argument = display[1];
      if (new RegExp(`\\b${name}\\b`).test(argument)) continue;
      if (/(?:originalError|errorText|errorMessage|String\s*\(|\.toString\s*\()/.test(argument)) continue;
      if (!/(?:失败|错误|异常|无法|不可用|无效|failed|error|unavailable|invalid)/i.test(argument)) continue;
      const line = content.slice(0, bodyStart + display.index).split("\n").length;
      findings.push(`${file}:${line}: generic promise catch message does not include original ${name}`);
    }
  }
  return findings;
}

function rewrittenCatchErrors(content, file) {
  const findings = [];
  const catchPattern = /catch\s*\(([^)]*)\)\s*\{/g;
  const throwPattern = /throw\s+(?:new\s+)?(?:Error|StateError|Exception|TypeError|RangeError|FormatException)\s*\(([\s\S]{0,500}?)\)/g;
  for (const match of content.matchAll(catchPattern)) {
    const params = match[1].trim();
    const name = params.split(/[,\s:]/)[0];
    if (!name || name === "_") continue;
    const openIndex = content.indexOf("{", match.index);
    const endIndex = catchBodyEnd(content, openIndex);
    if (endIndex < 0) continue;
    const body = content.slice(openIndex + 1, endIndex);
    const nestedNames = new Set();
    for (const nested of body.matchAll(/catch\s*\(([^)]*)\)/g)) {
      const nestedName = nested[1].trim().split(/[,\s:]/)[0];
      if (nestedName && nestedName !== "_") nestedNames.add(nestedName);
    }
    for (const thrown of body.matchAll(throwPattern)) {
      const argument = thrown[1];
      if (new RegExp(`\\b${name}\\b`).test(argument)) continue;
      if ([...nestedNames].some((nestedName) => new RegExp(`\\b${nestedName}\\b`).test(argument))) continue;
      if (!/(?:失败|错误|异常|无法|不可用|无效|failed|error|unavailable|invalid)/i.test(argument)) continue;
      const line = content.slice(0, openIndex + 1 + thrown.index).split("\n").length;
      findings.push(`${file}:${line}: rewritten catch error does not include original ${name}`);
    }
  }
  return findings;
}

function rewrittenCatchReturns(content, file) {
  const findings = [];
  const catchPattern = /catch\s*\(([^)]*)\)\s*\{/g;
  const returnPattern = /return\s+/g;
  for (const match of content.matchAll(catchPattern)) {
    const params = match[1].trim();
    const name = params.split(/[,\s:]/)[0];
    if (!name || name === "_") continue;
    const openIndex = content.indexOf("{", match.index);
    const endIndex = catchBodyEnd(content, openIndex);
    if (endIndex < 0) continue;
    const body = content.slice(openIndex + 1, endIndex);
    const aliases = new Set([name]);
    for (const assignment of body.matchAll(/(?:const|let|var|final|val|dynamic)\s+([A-Za-z_$][\w$]*)\s*=\s*([^\n;]+)/g)) {
      if (new RegExp(`\\b${name}\\b`).test(assignment[2])) aliases.add(assignment[1]);
    }
    for (const returned of body.matchAll(returnPattern)) {
      const value = body.slice(returned.index + returned[0].length, returned.index + returned[0].length + 600).split(/\n\s*return\b/)[0];
      if ([...aliases].some((alias) => new RegExp(`\\b${alias}\\b`).test(value))) continue;
      if (/^\s*[A-Za-z_$][\w$]*(?:Status|State)\.[A-Za-z_$][\w$]*/.test(value)) continue;
      if (!/(?:失败|错误|异常|无法|不可用|无效|failed|error|unavailable|invalid)/i.test(value)) continue;
      const line = content.slice(0, openIndex + 1 + returned.index).split("\n").length;
      findings.push(`${file}:${line}: rewritten catch return does not include original ${name}`);
    }
  }
  return findings;
}

function replaceSilentHandlers(content, file) {
  if (file.endsWith(".dart")) {
    return content
      .replace(
        /catch\s*\{\s*/g,
        'catch (error, stackTrace) {\n  print("[original-error] $error\\n$stackTrace");\n',
      )
      .replace(
        /catch\s*\(\s*_\s*\)\s*\{\s*/g,
        'catch (error, stackTrace) {\n  print("[original-error] $error\\n$stackTrace");\n',
      )
      .replace(
        /\.catchError\s*\(\s*\(\s*Object\s+_\s*\)\s*(async\s*)?\{\s*/g,
        '.catchError((Object error, StackTrace stackTrace) $1{\n  print("[original-error] $error\\n$stackTrace");\n',
      )
      .replace(
        /\.catchError\s*\(\s*\(\s*_\s*\)\s*(async\s*)?\{\s*/g,
        '.catchError((Object error, StackTrace stackTrace) $1{\n  print("[original-error] $error\\n$stackTrace");\n',
      )
      .replace(
        /\.catchError\s*\(\s*\(\s*_\s*\)\s*=>\s*(null|false)\s*\)/g,
        '.catchError((Object error, StackTrace stackTrace) {\n  print("[original-error] $error\\n$stackTrace");\n  return $1;\n})',
      )
      .replace(
        /catch\s*(?:\([^)]*\))?\s*\{\s*\}/g,
        'catch (error, stackTrace) {\n  print("[original-error] $error\\n$stackTrace");\n}',
      )
      .replace(
        /catchError\s*\(\s*\(\s*Object\s+_\s*\)\s*=>\s*null\s*\)/g,
        'catchError((Object error, StackTrace stackTrace) {\n  print("[original-error] $error\\n$stackTrace");\n  return null;\n})',
      );
  }
  let next = content
    .replace(
      /catch\s*\{\s*/g,
      'catch (error) {\n  console.error("[original-error]", error);\n',
    )
    .replace(
      /catch\s*\(\s*_\s*\)\s*\{\s*/g,
      'catch (error) {\n  console.error("[original-error]", error);\n',
    )
    .replace(
      /\.catch\s*\(\s*\(\s*\)\s*=>\s*\{\s*/g,
      '.catch((error) => {\n  console.error("[original-error]", error);\n',
    )
    .replace(
    /catch\s*(?:\([^)]*\))?\s*\{\s*\}/g,
    'catch (error) {\n  console.error("[original-error]", error);\n}',
  );
  const promiseFallbacks = [
    [/\.catch\s*\(\s*\(\s*\)\s*=>\s*\{\s*\}\s*\)/g, 'console.error("[original-error]", error);'],
    [/\.catch\s*\(\s*\(\s*\)\s*=>\s*undefined\s*\)/g, 'console.error("[original-error]", error);\n  return undefined;'],
    [/\.catch\s*\(\s*\(\s*\)\s*=>\s*null\s*\)/g, 'console.error("[original-error]", error);\n  return null;'],
    [/\.catch\s*\(\s*\(\s*\)\s*=>\s*false\s*\)/g, 'console.error("[original-error]", error);\n  return false;'],
    [/\.catch\s*\(\s*\(\s*\)\s*=>\s*\[\]\s*\)/g, 'console.error("[original-error]", error);\n  return [];'],
  ];
  for (const [pattern, body] of promiseFallbacks) {
    next = next.replace(pattern, `.catch((error) => {\n  ${body}\n})`);
  }
  return next;
}

const files = [];
for (const entry of roots) walk(path.join(root, entry), files);

const findings = [];
let changed = 0;
for (const file of files) {
  if (isTestFile(file)) continue;
  const content = fs.readFileSync(file, "utf8");
  const unused = unusedCatchBindings(content, path.relative(root, file));
  findings.push(...unused.findings);
  findings.push(...genericCatchMessages(content, path.relative(root, file)));
  findings.push(...rewrittenCatchErrors(content, path.relative(root, file)));
  findings.push(...rewrittenCatchReturns(content, path.relative(root, file)));
  if (mode === "fix") {
    let next = replaceSilentHandlers(content, file);
    const refreshed = unusedCatchBindings(next, path.relative(root, file));
    for (const item of refreshed.insertions.sort((left, right) => right.index - left.index)) {
      next = `${next.slice(0, item.index)}${item.report}${next.slice(item.index)}`;
    }
    if (next !== content) {
      fs.writeFileSync(file, next);
      changed += 1;
    }
  }
  findings.push(...silencePatterns(mode === "fix" ? fs.readFileSync(file, "utf8") : content, path.relative(root, file)));
}

if (mode === "fix") console.log(`updated files: ${changed}`);
if (findings.length) {
  console.log(findings.join("\n"));
  process.exitCode = 1;
} else {
  console.log("silent error handlers: 0");
}
