import { randomUUID } from 'node:crypto'
import { existsSync, readFileSync } from 'node:fs'
import { basename, resolve } from 'node:path'
import { fileURLToPath } from 'node:url'

const ORIGIN = 'http://127.0.0.1:5178'

function sleep(ms) {
  return new Promise((done) => setTimeout(done, ms))
}

function headers(token, extra = {}) {
  return { Origin: ORIGIN, 'X-Amitia-Local-Token': token, ...extra }
}

async function request(url, options, label) {
  const response = await fetch(url, { ...options, signal: AbortSignal.timeout(120000) })
  const text = await response.text()
  let body = null
  try {
    body = JSON.parse(text)
  } catch {
    body = text
  }
  if (!response.ok) {
    throw new Error(`${label} 失败：HTTP ${response.status}\n${typeof body === 'string' ? body : JSON.stringify(body, null, 2)}`)
  }
  return body
}

export async function previewPackage(base, token, packagePath) {
  const form = new FormData()
  form.append('file', new Blob([readFileSync(packagePath)]), basename(packagePath))
  form.append('scopeType', 'global')
  const body = await request(`${base}/api/extensions/packages/previews`, { method: 'POST', headers: headers(token), body: form }, `预览 ${basename(packagePath)}`)
  return body
}

export async function confirmPackage(base, token, sessionId, confirmations) {
  return request(
    `${base}/api/extensions/packages/previews/${sessionId}/confirm`,
    { method: 'POST', headers: headers(token, { 'Content-Type': 'application/json' }), body: JSON.stringify({ scopeType: 'global', confirmations }) },
    `确认 ${sessionId}`
  )
}

export async function installPackage(base, token, sessionId, confirmationToken, expectedExtensionId) {
  return request(
    `${base}/api/extensions/packages/operations/install`,
    {
      method: 'POST',
      headers: headers(token, { 'Content-Type': 'application/json', 'X-Idempotency-Key': randomUUID() }),
      body: JSON.stringify({ sessionId, scopeType: 'global', confirmations: {}, confirmationToken, expectedExtensionId, idempotencyKey: randomUUID() })
    },
    `安装 ${sessionId}`
  )
}

export async function waitOperation(base, token, operationId, timeoutMs = 180000) {
  const deadline = Date.now() + timeoutMs
  while (Date.now() < deadline) {
    const body = await request(`${base}/api/extensions/packages/operations/${operationId}`, { headers: headers(token) }, `查询操作 ${operationId}`)
    const status = body?.status
    if (status === 'completed' || status === 'failed' || status === 'requires_recovery') return body
    await sleep(500)
  }
  throw new Error(`操作 ${operationId} 在 ${timeoutMs}ms 内未完成`)
}

export async function enableExtension(base, token, extensionId) {
  return request(
    `${base}/api/extensions/kernel/extensions/enable`,
    { method: 'POST', headers: headers(token, { 'Content-Type': 'application/json' }), body: JSON.stringify({ id: extensionId }) },
    `启用 ${extensionId}`
  )
}

export async function installAndEnable(base, token, packagePath) {
  const preview = await previewPackage(base, token, packagePath)
  const detail = preview?.preview ?? {}
  const record = { package: packagePath, id: detail.id, version: detail.version, sessionId: detail.sessionId, signature: detail.signature?.status, compatible: detail.compatible, compatibility: detail.compatibility, errors: detail.errors ?? [] }
  if (detail.compatible !== true || !(detail.availableActions ?? []).includes('install')) {
    return { ...record, installed: false, reason: `不可安装：${detail.compatibility ?? 'unknown'} ${JSON.stringify(detail.errors ?? [])}` }
  }
  const confirmations = Object.fromEntries((detail.capabilityConfirmations ?? []).map((code) => [code, true]))
  const confirmation = await confirmPackage(base, token, detail.sessionId, confirmations)
  const operation = await installPackage(base, token, detail.sessionId, confirmation.confirmationToken, detail.id)
  const final = await waitOperation(base, token, operation.operationId)
  if (final.status !== 'completed') {
    return { ...record, installed: false, reason: `安装操作状态 ${final.status}`, operation: final }
  }
  const enabled = await enableExtension(base, token, detail.id)
  return { ...record, installed: true, enabled: enabled.enablement === 'enabled', operation: { operationId: operation.operationId, status: final.status } }
}

function parseArgs(argv) {
  const options = { packages: [] }
  for (let index = 0; index < argv.length; index += 1) {
    const arg = argv[index]
    const next = argv[index + 1]
    if (arg === '--base') options.base = next
    else if (arg === '--data-root') options.dataRoot = next
    else if (arg === '--package') options.packages.push(next)
    else if (arg === '--package-list') options.packageList = next
  }
  return options
}

async function main() {
  const options = parseArgs(process.argv.slice(2))
  const base = options.base
  if (!base || (options.packages.length === 0 && !options.packageList)) {
    process.stderr.write('用法：node scripts/rsi/plugin-seed.mjs --base <httpBaseURL> --data-root <路径> --package <包路径> [--package ...]\n')
    process.exit(2)
  }
  const tokenPath = resolve(options.dataRoot ?? '.', 'security', 'local-token')
  if (!existsSync(tokenPath)) throw new Error(`缺少本地凭据 ${tokenPath}`)
  const token = readFileSync(tokenPath, 'utf8').trim()
  const packages = [...options.packages]
  if (options.packageList) {
    for (const line of readFileSync(options.packageList, 'utf8').split('\n')) {
      const trimmed = line.trim()
      if (trimmed && !trimmed.startsWith('#')) packages.push(trimmed)
    }
  }
  const results = []
  for (const packagePath of packages) {
    const result = await installAndEnable(base, token, resolve(packagePath))
    results.push(result)
    process.stdout.write(`${result.installed ? 'OK  ' : 'SKIP'} ${result.id}@${result.version} ${result.installed ? '已安装并启用' : result.reason}\n`)
  }
  process.stdout.write(`${JSON.stringify({ base, results }, null, 2)}\n`)
  if (results.some((item) => !item.installed)) process.exit(8)
}

if (process.argv[1] && resolve(process.argv[1]) === fileURLToPath(import.meta.url)) {
  main().catch((error) => {
    process.stderr.write(`插件播种失败：${error.message}\n${error.stack ?? ''}\n`)
    process.exit(8)
  })
}
