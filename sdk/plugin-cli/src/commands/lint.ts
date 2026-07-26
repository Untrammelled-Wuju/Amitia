import * as fs from "node:fs";
import * as path from "node:path";
import type { CliCommand, CliContext, CliCommandResult, CliReport } from "../types";
import { EXIT_CODES } from "../exit-codes";

const FORBIDDEN_BUILTINS = ["child_process", "fs", "net", "http", "https", "os", "path"];
const FORBIDDEN_GLOBALS = ["require", "process", "global", "Buffer"];
const SECRET_PATTERNS = [
  /(?:api[_-]?key|secret|token|password|credential|private[_-]?key)\s*[:=]\s*["'][^"']+["']/i,
];

export const lintCommand: CliCommand = {
  name: "lint",
  description: "Lint extension source for dangerous APIs and quality issues",
  usage: "amitia-ext lint [options]",
  options: [
    { name: "--src", shortName: "-s", description: "Source directory", takesValue: true, defaultValue: "./src" },
    { name: "--format", description: "Output format", takesValue: true, choices: ["human", "json", "sarif"], defaultValue: "human" },
  ],
  async run(ctx: CliContext, args: string[]): Promise<CliCommandResult> {
    const opts = parseOptions(args);
    const srcDir = path.resolve(ctx.cwd, opts["--src"] ?? "./src");
    if (!fs.existsSync(srcDir)) {
      return {
        exitCode: EXIT_CODES.CONFIGURATION_ERROR,
        message: `source directory not found: ${srcDir}`,
      };
    }
    const reports: CliReport[] = [];
    walk(srcDir, (filePath) => {
      const content = fs.readFileSync(filePath, "utf-8");
      reports.push(...checkForbiddenBuiltins(filePath, content));
      reports.push(...checkForbiddenGlobals(filePath, content));
      reports.push(...checkDynamicImport(filePath, content));
      reports.push(...checkSecretLeaks(filePath, content));
      reports.push(...checkMissingAbortSignal(filePath, content));
    });

    const hasErrors = reports.some((r) => r.severity === "error");
    return {
      exitCode: hasErrors ? EXIT_CODES.VALIDATION_OR_BUILD_FAILURE : EXIT_CODES.SUCCESS,
      message: hasErrors
        ? `lint failed with ${reports.length} issue(s)`
        : `lint passed (${reports.length} informational)`,
      reports,
    };
  },
};

function walk(dir: string, cb: (filePath: string) => void): void {
  for (const entry of fs.readdirSync(dir, { withFileTypes: true })) {
    const full = path.join(dir, entry.name);
    if (entry.isDirectory()) {
      walk(full, cb);
    } else if (entry.isFile() && /\.(ts|tsx|js|mjs|cjs)$/.test(entry.name)) {
      cb(full);
    }
  }
}

function checkForbiddenBuiltins(file: string, content: string): CliReport[] {
  const reports: CliReport[] = [];
  for (const builtin of FORBIDDEN_BUILTINS) {
    const re = new RegExp(`\\bfrom\\s+["']node:${builtin}["']|\\brequire\\(["']${builtin}["']\\)`, "g");
    if (re.test(content)) {
      reports.push({
        ruleId: "lint.forbidden_builtin",
        severity: "error",
        message: `forbidden Node builtin usage: ${builtin}`,
        file,
      });
    }
  }
  return reports;
}

function checkForbiddenGlobals(file: string, content: string): CliReport[] {
  const reports: CliReport[] = [];
  for (const global of FORBIDDEN_GLOBALS) {
    const re = new RegExp(`\\b${global}\\b`, "g");
    const matches = content.match(re);
    if (matches && matches.length > 0) {
      reports.push({
        ruleId: "lint.forbidden_global",
        severity: "warning",
        message: `forbidden global usage: ${global} (${matches.length} occurrence(s))`,
        file,
      });
    }
  }
  return reports;
}

function checkDynamicImport(file: string, content: string): CliReport[] {
  const reports: CliReport[] = [];
  const re = /\bimport\s*\(\s*[^)]+\s*\)/g;
  const matches = content.match(re);
  if (matches) {
    reports.push({
      ruleId: "lint.dynamic_import",
      severity: "warning",
      message: `dynamic import detected (${matches.length} occurrence(s))`,
      file,
    });
  }
  return reports;
}

function checkSecretLeaks(file: string, content: string): CliReport[] {
  const reports: CliReport[] = [];
  for (const pattern of SECRET_PATTERNS) {
    if (pattern.test(content)) {
      reports.push({
        ruleId: "lint.suspected_secret",
        severity: "error",
        message: "suspected secret literal in source",
        file,
      });
    }
  }
  return reports;
}

function checkMissingAbortSignal(file: string, content: string): CliReport[] {
  const reports: CliReport[] = [];
  const asyncFnRe = /async\s+function\s+\w+\s*\([^)]*\)/g;
  const matches = content.match(asyncFnRe) ?? [];
  for (const match of matches) {
    if (!/signal|AbortSignal|ctx|context/.test(match)) {
      const fnName = /function\s+(\w+)/.exec(match)?.[1] ?? "<anonymous>";
      reports.push({
        ruleId: "lint.missing_abort_signal",
        severity: "info",
        message: `async function ${fnName} does not accept abort signal`,
        file,
      });
    }
  }
  return reports;
}

function parseOptions(args: string[]): Record<string, string | undefined> {
  const opts: Record<string, string | undefined> = {};
  for (let i = 0; i < args.length; i++) {
    const arg = args[i];
    if (arg?.startsWith("--")) {
      const next = args[i + 1];
      if (next && !next.startsWith("--")) {
        opts[arg] = next;
        i++;
      } else {
        opts[arg] = "true";
      }
    }
  }
  return opts;
}
