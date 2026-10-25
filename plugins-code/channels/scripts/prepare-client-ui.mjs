import fs from "node:fs"
import path from "node:path"
import { createHash } from "node:crypto"

function hashFile(file) {
  return `sha256:${createHash("sha256").update(fs.readFileSync(file)).digest("hex")}`
}

export function prepareClientUI({ manifest, stagingRoot, moduleId, contributionId, assetDir = "desktop" }) {
  const moduleRoot = path.join(stagingRoot, "modules", moduleId, assetDir)
  const entryPath = `modules/${moduleId}/${assetDir}/index.mjs`
  const stylePath = `modules/${moduleId}/${assetDir}/styles.css`
  const entryHash = hashFile(path.join(stagingRoot, entryPath))
  const styleHash = hashFile(path.join(stagingRoot, stylePath))
  let updatedContribution = false
  for (const module of manifest.modules || []) {
    for (const contribution of module.contributions || []) {
      if (contribution.id === contributionId) {
        contribution.spec.entry = {
          ...(contribution.spec.entry || {}),
          type: "client_module",
          path: entryPath,
          content_hash: entryHash,
        }
        contribution.spec.sandbox = { type: "client_module" }
        updatedContribution = true
      }
      if (contribution.kind !== "ui_provider") continue
      const entries = contribution.spec?.entries
      if (!entries) continue
      for (const [platform, entry] of Object.entries(entries)) {
        if (entry?.contributionId !== contributionId && entry?.type !== "web_module") continue
        entries[platform] = {
          type: "web_module",
          contributionId,
          path: entryPath,
          stylePath,
          contentHash: entryHash,
          styleHash,
        }
      }
    }
  }
  if (!updatedContribution || !fs.existsSync(moduleRoot)) {
    throw new Error(`client UI contribution not prepared: ${contributionId}`)
  }
  return manifest
}
