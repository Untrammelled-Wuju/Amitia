import { readdirSync, readFileSync, statSync, writeFileSync } from "node:fs"
import { join, resolve } from "node:path"

const root = resolve(new URL("..", import.meta.url).pathname, "dist")

function rewriteSpecifier(specifier) {
  if (!specifier.startsWith(".")) return specifier
  if (/\.(?:js|json|node|mjs|cjs)$/.test(specifier)) return specifier
  return `${specifier}.js`
}

function rewriteFile(file) {
  const source = readFileSync(file, "utf8")
  const output = source
    .replace(/(\bfrom\s*["'])(\.[^"']+)(["'])/g, (_match, prefix, specifier, suffix) => `${prefix}${rewriteSpecifier(specifier)}${suffix}`)
    .replace(/(\bimport\s*["'])(\.[^"']+)(["'])/g, (_match, prefix, specifier, suffix) => `${prefix}${rewriteSpecifier(specifier)}${suffix}`)
    .replace(/(\bimport\s*\(\s*["'])(\.[^"']+)(["']\s*\))/g, (_match, prefix, specifier, suffix) => `${prefix}${rewriteSpecifier(specifier)}${suffix}`)
  if (output !== source) writeFileSync(file, output)
}

function walk(directory) {
  for (const name of readdirSync(directory)) {
    const file = join(directory, name)
    if (statSync(file).isDirectory()) {
      walk(file)
    } else if (file.endsWith(".js") || file.endsWith(".d.ts")) {
      rewriteFile(file)
    }
  }
}

walk(root)
