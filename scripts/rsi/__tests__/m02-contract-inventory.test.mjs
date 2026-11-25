import { test } from 'node:test'
import assert from 'node:assert/strict'
import { SOURCES, TARGET_STRUCTS, buildInventory, parseFields, parseStructs } from '../m02-contract-inventory.mjs'

const ALL_FILES = [...SOURCES.plugincontract, ...SOURCES.capability]

function fieldsOf(structs, name) {
  const entry = structs.get(name)
  assert.ok(entry, `缺少结构体 ${name}`)
  return entry.fields.filter((field) => !field.embedded).map((field) => field.name)
}

function dumpOf(structs) {
  const dump = {}
  for (const [name, entry] of structs) dump[name] = entry.fields.map((field) => field.name)
  return dump
}

test('parseFields 解析 json tag、omitempty 与嵌入字段', () => {
  const body = [
    'ID       ProviderID        `json:"id"`',
    'Metadata map[string]any    `json:"metadata,omitempty"`',
    'Embedded',
    '	Runtime RuntimeBinding `json:"runtime"`'
  ].join('\n')
  const fields = parseFields(body)
  assert.deepEqual(fields, [
    { name: 'ID', type: 'ProviderID', embedded: false, jsonName: 'id', omitempty: false },
    { name: 'Metadata', type: 'map[string]any', embedded: false, jsonName: 'metadata', omitempty: true },
    { name: 'Embedded', type: '', embedded: true, jsonName: null, omitempty: false },
    { name: 'Runtime', type: 'RuntimeBinding', embedded: false, jsonName: 'runtime', omitempty: false }
  ])
})

test('parseStructs 在真实源码上得到 ScopeRule 与 ProviderRegistry 字段', () => {
  const structs = parseStructs(ALL_FILES)
  assert.deepEqual(fieldsOf(structs, 'ScopeRule'), ['Type', 'ID'])
  assert.deepEqual(fieldsOf(structs, 'PermissionRequirement'), ['Capability', 'Description', 'Risk'])
  assert.deepEqual(fieldsOf(structs, 'ServiceOffer'), ['ServiceID', 'Version', 'ContractHash', 'Scopes', 'Placement', 'Priority'])
  assert.equal(fieldsOf(structs, 'ToolDefinition').length, 32)
  assert.equal(structs.get('ProviderRegistry').file, 'internal/extension/kernel/capability/provider_registry.go')
})

test('buildInventory 在完整输入时全部检查通过', () => {
  const structs = parseStructs(ALL_FILES)
  const inventory = buildInventory(structs, dumpOf(structs))
  assert.deepEqual(inventory.checks.filter((check) => !check.ok), [])
  assert.equal(inventory.ok, true)
  const concepts = inventory.mapping.map((row) => row.concept)
  for (const required of ['Scope', 'Placement', 'Permission', 'Version', 'ContractHash']) {
    assert.ok(concepts.includes(required), `映射表缺少 ${required}`)
  }
  assert.equal(inventory.mapping.find((row) => row.concept === 'Placement').conflict, 'enum-divergence')
})

test('buildInventory 对缺结构体与交叉校验不一致都会失败', () => {
  const structs = parseStructs(ALL_FILES)
  structs.delete(TARGET_STRUCTS[0][1])
  const missing = buildInventory(structs, dumpOf(structs))
  assert.equal(missing.ok, false)
  assert.equal(missing.checks.find((check) => check.id === 'TARGET_STRUCTS_PARSED').ok, false)

  const complete = parseStructs(ALL_FILES)
  const dump = dumpOf(complete)
  dump.ScopeRule = ['Type']
  const drifted = buildInventory(complete, dump)
  assert.equal(drifted.ok, false)
  assert.equal(drifted.checks.find((check) => check.id === 'GO_AST_CROSSCHECK').ok, false)
  assert.deepEqual(drifted.checks.find((check) => check.id === 'GO_AST_CROSSCHECK').detail, [
    { struct: 'ScopeRule', parsed: ['ID', 'Type'], dumped: ['Type'] }
  ])
})
