import assert from 'node:assert/strict'
import { test } from 'node:test'
import { mkdtempSync, mkdirSync, readFileSync, writeFileSync } from 'node:fs'
import { tmpdir } from 'node:os'
import { join } from 'node:path'
import {
  EXIT_CORE_INVENTORY,
  HOSTS,
  INVENTORY_PATH,
  ROUTE_SNAPSHOT_PATH,
  RUN_PATTERN,
  SIDE_EFFECTS,
  readInventory,
  renderInventory,
  summarizeInventory,
  parseGoTestOutput,
  validateInventory
} from '../m03-core-inventory.mjs'
import { repoRoot } from '../toolchain-preflight.mjs'

const HEX = 'a'.repeat(64)

function fixture(overrides = {}) {
  return {
    schemaVersion: 1,
    routes: 3,
    mapped: 3,
    unmapped: 0,
    domains: [
      { domain: 'artifact', routes: 2, mapped: 2, unmapped: 0 },
      { domain: 'system', routes: 1, mapped: 1, unmapped: 0 }
    ],
    handlerKinds: { method: 3 },
    sideEffects: { 'db.read': 2, 'db.write': 1, 'file.write': 1, 'stream.write': 1 },
    uiHosts: [
      { host: 'web', files: 2, calls: 5, linked: 4, prefixes: 0, unlinked: 1, unlinkedPaths: ['/api/ghost'], digest: HEX },
      { host: 'flutter', files: 1, calls: 3, linked: 3, prefixes: 0, unlinked: 0, unlinkedPaths: [], digest: HEX }
    ],
    digest: HEX,
    ...overrides
  }
}

test('constants pin the deliverable surface', () => {
  assert.equal(EXIT_CORE_INVENTORY, 28)
  assert.deepEqual(HOSTS, ['web', 'flutter'])
  assert.deepEqual(SIDE_EFFECTS, ['db.read', 'db.write', 'file.write', 'stream.write'])
  assert.equal(INVENTORY_PATH, 'backend/internal/extension/kernel/plugincontract/testdata/core-call-inventory.json')
  assert.equal(ROUTE_SNAPSHOT_PATH, 'artifacts/rsi/baseline/final-1.json')
  assert.ok(RUN_PATTERN.includes('TestCoreCallInventoryCoversEveryBusinessDomain'))
  assert.ok(RUN_PATTERN.includes('TestUIHostAuditTracksWebAndFlutter'))
})

test('validateInventory accepts a consistent fixture', () => {
  assert.deepEqual(validateInventory(fixture()), [])
  assert.deepEqual(validateInventory(fixture(), [{ path: '/a' }, { path: '/b' }, { path: '/c' }]), [])
})

test('validateInventory rejects non-objects', () => {
  assert.deepEqual(validateInventory(null), ['inventory is not an object'])
  assert.deepEqual(validateInventory([]), ['inventory is not an object'])
})

test('validateInventory rejects a schema drift', () => {
  const problems = validateInventory(fixture({ schemaVersion: 2 }))
  assert.ok(problems.some((problem) => problem.includes('schemaVersion=2')))
})

test('validateInventory rejects coverage arithmetic drift', () => {
  const problems = validateInventory(fixture({ mapped: 2 }))
  assert.ok(problems.some((problem) => problem.includes('mapped+unmapped=2 != routes=3')))
})

test('validateInventory rejects unmapped routes that break the full coverage claim', () => {
  const problems = validateInventory(fixture({ unmapped: 1, mapped: 2 }))
  assert.ok(problems.some((problem) => problem.includes('unmapped=1 blocks the full coverage claim')))
})

test('validateInventory rejects a domain without mapped routes', () => {
  const problems = validateInventory(
    fixture({
      domains: [
        { domain: 'artifact', routes: 2, mapped: 2, unmapped: 0 },
        { domain: 'ghost', routes: 1, mapped: 0, unmapped: 1 }
      ]
    })
  )
  assert.ok(problems.some((problem) => problem.includes('ghost has no mapped route')))
})

test('validateInventory rejects duplicate and empty domain names', () => {
  const duplicate = validateInventory(
    fixture({
      domains: [
        { domain: 'artifact', routes: 2, mapped: 2, unmapped: 0 },
        { domain: 'artifact', routes: 1, mapped: 1, unmapped: 0 }
      ]
    })
  )
  assert.ok(duplicate.some((problem) => problem.includes('duplicate artifact')))
  const empty = validateInventory(
    fixture({
      domains: [
        { domain: ' ', routes: 2, mapped: 2, unmapped: 0 },
        { domain: 'system', routes: 1, mapped: 1, unmapped: 0 }
      ]
    })
  )
  assert.ok(empty.some((problem) => problem.includes('name is empty')))
})

test('validateInventory rejects handlerKinds totals that drift', () => {
  const problems = validateInventory(fixture({ handlerKinds: { method: 2 } }))
  assert.ok(problems.some((problem) => problem.includes('handlerKinds total=2 != routes=3')))
  const unknown = validateInventory(fixture({ handlerKinds: { method: 3, bogus: 0 } }))
  assert.ok(unknown.some((problem) => problem.includes('unknown kind bogus')))
})

test('validateInventory rejects empty side effect classes', () => {
  const problems = validateInventory(fixture({ sideEffects: { 'db.read': 2, 'db.write': 1, 'file.write': 1, 'stream.write': 0 } }))
  assert.ok(problems.some((problem) => problem.includes('sideEffects.stream.write is empty')))
})

test('validateInventory rejects a missing ui host', () => {
  const inventory = fixture()
  inventory.uiHosts = inventory.uiHosts.filter((host) => host.host === 'web')
  const problems = validateInventory(inventory)
  assert.ok(problems.some((problem) => problem.includes('uiHosts missing flutter')))
})

test('validateInventory rejects ui host arithmetic and digest drift', () => {
  const inventory = fixture()
  inventory.uiHosts[0] = { ...inventory.uiHosts[0], linked: 3, digest: 'short' }
  const problems = validateInventory(inventory)
  assert.ok(problems.some((problem) => problem.includes('linked+prefixes+unlinked != calls')))
  assert.ok(problems.some((problem) => problem.includes('uiHosts[0] digest is not a 64 char lowercase hex string')))
})

test('validateInventory rejects a route snapshot drift', () => {
  const problems = validateInventory(fixture(), [{ path: '/a' }])
  assert.ok(problems.some((problem) => problem.includes('route snapshot entries=1 != inventory routes=3')))
})

test('summarizeInventory derives coverage and host lines', () => {
  const summary = summarizeInventory(fixture())
  assert.equal(summary.coverage, '100.00%')
  assert.equal(summary.domains, 2)
  assert.deepEqual(summary.hosts, ['web 4/5', 'flutter 3/3'])
  assert.equal(summary.smallestDomain.domain, 'system')
  const empty = summarizeInventory(fixture({ routes: 0, mapped: 0, domains: [] }))
  assert.equal(empty.coverage, '0.00%')
  assert.equal(empty.smallestDomain, null)
})

test('renderInventory prints the pinned digest and domain table', () => {
  const listing = renderInventory(fixture())
  assert.ok(listing.includes('routes 3/3（100.00%）'))
  assert.ok(listing.includes('db.read 2'))
  assert.ok(listing.includes('web 4/5'))
  assert.ok(listing.includes(HEX))
  assert.ok(listing.includes('artifact'))
  assert.ok(listing.includes('unmapped'))
})

test('parseGoTestOutput counts verdicts', () => {
  const counts = parseGoTestOutput('--- PASS: TestA\n--- PASS: TestB\n--- FAIL: TestC\n')
  assert.deepEqual(counts, { passed: 2, failed: 1 })
  assert.deepEqual(parseGoTestOutput(null), { passed: 0, failed: 0 })
})

test('readInventory reads the committed inventory', () => {
  const inventory = readInventory(repoRoot)
  assert.equal(inventory.schemaVersion, 1)
  assert.equal(inventory.routes, 1201)
  assert.equal(inventory.unmapped, 0)
  assert.equal(inventory.domains.length, 46)
  assert.equal(inventory.uiHosts.length, 2)
})

test('readInventory surfaces a missing file as a stack-bearing error', () => {
  const root = mkdtempSync(join(tmpdir(), 'm03-inventory-'))
  assert.throws(() => readInventory(root), /core call inventory missing/u)
})

test('validateInventory accepts the committed inventory against the real route snapshot', () => {
  const inventory = readInventory(repoRoot)
  const snapshot = JSON.parse(readFileSync(join(repoRoot, ROUTE_SNAPSHOT_PATH), 'utf8'))
  assert.deepEqual(validateInventory(inventory, snapshot.routes.entries), [])
})

test('fixture writes stay inside the temp root', () => {
  const root = mkdtempSync(join(tmpdir(), 'm03-inventory-'))
  mkdirSync(join(root, 'nested'), { recursive: true })
  writeFileSync(join(root, 'nested', 'inventory.json'), JSON.stringify(fixture()), 'utf8')
  assert.deepEqual(validateInventory(fixture()), [])
})
