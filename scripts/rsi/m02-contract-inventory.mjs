import { spawnSync } from 'node:child_process'
import { createHash } from 'node:crypto'
import { existsSync, mkdirSync, readFileSync, writeFileSync } from 'node:fs'
import { resolve } from 'node:path'
import { fileURLToPath } from 'node:url'
import { cleanEnv, repoRoot } from './toolchain-preflight.mjs'

const backendRoot = resolve(repoRoot, 'backend')
const outDir = resolve(repoRoot, 'artifacts', 'rsi')
const outPath = resolve(outDir, 'm02-s01-contract-inventory.json')
const crosscheckDir = resolve(outDir, 'm02-s01-crosscheck')

export const SOURCES = {
  plugincontract: [
    'internal/extension/kernel/plugincontract/types.go',
    'internal/extension/kernel/plugincontract/service_catalog.go',
    'internal/extension/kernel/plugincontract/service_graph.go',
    'internal/extension/kernel/plugincontract/manifest_graph.go',
    'internal/extension/kernel/plugincontract/validate.go',
    'internal/extension/kernel/plugincontract/identity.go',
    'internal/extension/kernel/plugincontract/contract_hash.go',
    'internal/extension/kernel/plugincontract/descriptor.go'
  ],
  capability: [
    'internal/extension/kernel/capability/definition.go',
    'internal/extension/kernel/capability/provider.go',
    'internal/extension/kernel/capability/provider_registry.go',
    'internal/extension/kernel/capability/registry.go',
    'internal/extension/kernel/capability/service.go',
    'internal/extension/kernel/capability/tool.go',
    'internal/extension/kernel/capability/owner.go',
    'internal/extension/kernel/capability/id.go'
  ]
}

export const TARGET_STRUCTS = [
  ['plugincontract', 'Manifest'],
  ['plugincontract', 'ProviderMetadata'],
  ['plugincontract', 'ServiceOffer'],
  ['plugincontract', 'ServiceRequirement'],
  ['plugincontract', 'ServiceDescriptor'],
  ['plugincontract', 'ServiceBinding'],
  ['plugincontract', 'ServiceGraphRequest'],
  ['plugincontract', 'ServiceCatalog'],
  ['capability', 'CapabilityProviderDefinition'],
  ['capability', 'CapabilityProviderInstance'],
  ['capability', 'CapabilityDefinition'],
  ['capability', 'ToolDefinition'],
  ['capability', 'ToolVersion'],
  ['capability', 'ProviderRegistry'],
  ['capability', 'ToolRegistry'],
  ['capability', 'CapabilityDescriptor'],
  ['capability', 'ScopeRule'],
  ['capability', 'PermissionRequirement'],
  ['capability', 'ResourceOwner']
]

function stripComments(source) {
  let out = ''
  for (let index = 0; index < source.length; index += 1) {
    const char = source[index]
    const next = source[index + 1]
    if (char === '/' && next === '/') {
      while (index < source.length && source[index] !== '\n') index += 1
      out += '\n'
      continue
    }
    if (char === '/' && next === '*') {
      index += 2
      while (index < source.length && !(source[index] === '*' && source[index + 1] === '/')) index += 1
      index += 1
      continue
    }
    out += char
  }
  return out
}

function matchBraces(source, openIndex) {
  let depth = 0
  let inString = null
  for (let index = openIndex; index < source.length; index += 1) {
    const char = source[index]
    if (inString) {
      if (char === '\\' && inString !== '`') {
        index += 1
        continue
      }
      if (char === inString) inString = null
      continue
    }
    if (char === '"' || char === '\'' || char === '`') {
      inString = char
      continue
    }
    if (char === '{') depth += 1
    if (char === '}') {
      depth -= 1
      if (depth === 0) return index
    }
  }
  throw new Error(`第 ${openIndex} 字节处的结构体括号未闭合`)
}

export function parseStructs(files) {
  const structs = new Map()
  for (const file of files) {
    const absolute = resolve(backendRoot, file)
    const source = stripComments(readFileSync(absolute, 'utf8'))
    const pattern = /type\s+([A-Z]\w*)\s+struct\s*\{/g
    let match = pattern.exec(source)
    while (match) {
      const name = match[1]
      const open = match.index + match[0].length - 1
      const close = matchBraces(source, open)
      const body = source.slice(open + 1, close)
      structs.set(name, { name, file, fields: parseFields(body) })
      pattern.lastIndex = close
      match = pattern.exec(source)
    }
  }
  return structs
}

export function parseFields(body) {
  const fields = []
  for (const rawLine of body.split('\n')) {
    const line = rawLine.trim().replace(/\/\/.*$/, '').trim()
    if (!line) continue
    const tagMatch = line.match(/`([^`]*)`/)
    const tag = tagMatch ? tagMatch[1] : null
    const declaration = (tagMatch ? line.slice(0, tagMatch.index) : line).trim()
    const tokens = declaration.split(/\s+/).filter(Boolean)
    if (tokens.length === 0) continue
    const json = tag ? tag.match(/json:"([^"]*)"/) : null
    const jsonParts = json ? json[1].split(',') : []
    fields.push({
      name: tokens[0],
      type: tokens.slice(1).join(' '),
      embedded: tokens.length === 1,
      jsonName: jsonParts[0] || null,
      omitempty: jsonParts.includes('omitempty')
    })
  }
  return fields
}

export function parseConstValues(packageFiles, typeName) {
  const values = []
  const pattern = new RegExp(`^\\s*(\\w+)\\s+${typeName}\\s*=\\s*"([^"]*)"`, 'gm')
  for (const file of packageFiles) {
    const source = stripComments(readFileSync(resolve(backendRoot, file), 'utf8'))
    let match = pattern.exec(source)
    while (match) {
      values.push({ constant: match[1], value: match[2] })
      match = pattern.exec(source)
    }
  }
  return values.sort((left, right) => left.value.localeCompare(right.value))
}

export function countPattern(packageFiles, pattern) {
  let count = 0
  const files = []
  for (const file of packageFiles) {
    const source = stripComments(readFileSync(resolve(backendRoot, file), 'utf8'))
    const matches = source.match(pattern)
    if (matches) {
      count += matches.length
      files.push(file)
    }
  }
  return { count, files }
}

const GO_DUMP = `package main

import (
	"encoding/json"
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
)

func main() {
	out := map[string][]string{}
	for _, path := range os.Args[1:] {
		file, err := parser.ParseFile(token.NewFileSet(), path, nil, 0)
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
		ast.Inspect(file, func(node ast.Node) bool {
			decl, ok := node.(*ast.TypeSpec)
			if !ok {
				return true
			}
			structType, ok := decl.Type.(*ast.StructType)
			if !ok || decl.Name == nil {
				return true
			}
			names := []string{}
			for _, field := range structType.Fields.List {
				if len(field.Names) == 0 {
					names = append(names, fmt.Sprintf("~%T", field.Type))
					continue
				}
				for _, name := range field.Names {
					names = append(names, name.Name)
				}
			}
			out[decl.Name.Name] = names
			return true
		})
	}
	encoded, err := json.Marshal(out)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	fmt.Println(string(encoded))
}
`

export function goParserDump() {
  mkdirSync(crosscheckDir, { recursive: true })
  const programPath = resolve(crosscheckDir, 'dump_structs.go')
  writeFileSync(programPath, GO_DUMP, 'utf8')
  writeFileSync(resolve(crosscheckDir, 'go.mod'), 'module m02crosscheck\n\ngo 1.21\n', 'utf8')
  const files = [...SOURCES.plugincontract, ...SOURCES.capability].map((file) => resolve(backendRoot, file))
  const result = spawnSync('go', ['run', '.', ...files], {
    cwd: crosscheckDir,
    encoding: 'utf8',
    env: cleanEnv(),
    maxBuffer: 16 * 1024 * 1024
  })
  if (result.status !== 0) {
    throw new Error(`go/ast 交叉校验失败: exit=${result.status}\n${result.stdout}\n${result.stderr}`)
  }
  return JSON.parse(result.stdout.trim().split('\n').pop())
}

function fieldsOf(structs, name) {
  const entry = structs.get(name)
  if (!entry) return []
  return entry.fields.filter((field) => !field.embedded).map((field) => field.name)
}

export function buildInventory(structs, dump) {
  const servicePlacement = parseConstValues(SOURCES.plugincontract, 'ServicePlacement')
  const providerPlacement = parseConstValues(SOURCES.capability, 'ProviderPlacement')
  const serviceScopes = parseConstValues(SOURCES.plugincontract, 'ServiceScope')
  const contractHashInCapability = countPattern(SOURCES.capability, /ContractHash/g)
  const contractHashInContract = countPattern(SOURCES.plugincontract, /ContractHash/g)
  const providerIDValidation = countPattern(SOURCES.capability, /func\s+ValidateProviderID/g)
  const catalogDerivedProviderID = countPattern(SOURCES.plugincontract, /providerID\s*:=\s*key\s*\+\s*"\/"\s*\+\s*serviceID/g)
  const contractImportsCapability = countPattern(SOURCES.plugincontract, /internal\/extension\/kernel\/capability/g)
  const capabilityImportsContract = countPattern(SOURCES.capability, /internal\/extension\/kernel\/plugincontract/g)
  const hashProducersInContract = countPattern(SOURCES.plugincontract, /func\s+[A-Za-z0-9_]*ContractHash[A-Za-z0-9_]*\s*\(/g)
  const providerIDValidationInContract = countPattern(SOURCES.plugincontract, /func\s+ValidateProviderID/g)
  const protocolVersionInContract = countPattern(SOURCES.plugincontract, /ProtocolVersion|protocolVersion/g)

  const missing = TARGET_STRUCTS.filter(([pkg, name]) => {
    const entry = structs.get(name)
    return !entry || entry.file === undefined || !SOURCES[pkg].includes(entry.file) || entry.fields.length === 0
  }).map(([pkg, name]) => `${pkg}.${name}`)

  const crosscheckMismatches = []
  for (const [, name] of TARGET_STRUCTS) {
    const parsed = fieldsOf(structs, name).slice().sort()
    const dumped = (dump[name] ?? []).slice().sort()
    if (JSON.stringify(parsed) !== JSON.stringify(dumped)) {
      crosscheckMismatches.push({ struct: name, parsed, dumped })
    }
  }

  const placementOnlyService = servicePlacement.filter((entry) => !providerPlacement.some((other) => other.value === entry.value))
  const placementOnlyProvider = providerPlacement.filter((entry) => !servicePlacement.some((other) => other.value === entry.value))

  const mapping = [
    {
      concept: 'Scope',
      serviceSide: { type: 'ServiceOffer.Scopes []ServiceScope', enum: serviceScopes.map((entry) => entry.value), authority: 'plugincontract' },
      providerSide: { type: 'CapabilityDefinition.ScopeRule / ToolDefinition.Scope', enum: null, authority: 'capability' },
      authority: 'plugincontract 定义范围词表；capability 只承载 `{type,id}` 的可扩展规则，不重复枚举',
      conflict: 'none',
      detail: `ServiceScope 有 ${serviceScopes.length} 个固定值；ScopeRule 仅有 Type/ID 两个字段（${fieldsOf(structs, 'ScopeRule').join(', ')}），Type 为自由字符串，不做枚举校验`
    },
    {
      concept: 'Placement',
      serviceSide: { type: 'ServiceOffer.Placement / ServiceDescriptor.Placement', enum: servicePlacement.map((entry) => entry.value), authority: 'plugincontract' },
      providerSide: { type: 'CapabilityProviderDefinition.Placement', enum: providerPlacement.map((entry) => entry.value), authority: 'capability' },
      authority: '两处各自维护，尚无归一化函数',
      conflict: 'enum-divergence',
      detail: `service 侧独有 [${placementOnlyService.map((entry) => entry.value).join(', ')}]；provider 侧的 ${providerPlacement.length} 个值全部被 service 侧覆盖（独有 [${placementOnlyProvider.map((entry) => entry.value).join(', ')}]）；声明为 cloud/hybrid 的服务无法直接得到合法的 ProviderPlacement`
    },
    {
      concept: 'Permission',
      serviceSide: { type: '（无）', enum: null, authority: '—' },
      providerSide: { type: 'CapabilityDefinition.Permissions []PermissionRequirement', enum: null, authority: 'capability' },
      authority: 'capability.ToolDefinition/CapabilityDefinition 唯一持有权限声明',
      conflict: 'none',
      detail: `PermissionRequirement 字段为 ${fieldsOf(structs, 'PermissionRequirement').join(', ')}；plugincontract 的 ServiceOffer/ServiceRequirement 不含权限概念`
    },
    {
      concept: 'Version',
      serviceSide: { type: 'ServiceOffer.Version / ServiceRequirement.VersionRange', enum: null, authority: 'plugincontract' },
      providerSide: { type: 'ToolDefinition.Version / ToolVersion.{SchemaVersion,Revision}', enum: null, authority: 'capability' },
      authority: '服务侧管声明与区间匹配；工具侧管 schema 版本与内容修订',
      conflict: 'multi-notion',
      detail: `同时存在 4 种版本语义：offer 精确版本、requirement 区间、tool 显示版本、schema+revision；没有任何一处把它们归一`
    },
    {
      concept: 'ContractHash',
      serviceSide: { type: 'ServiceOffer.ContractHash / ServiceRequirement.ContractHash', enum: null, authority: 'plugincontract' },
      providerSide: { type: '（无）', enum: null, authority: '—' },
      authority: 'plugincontract 唯一持有',
      conflict: 'none',
      detail: `capability 包内 ContractHash 出现 ${contractHashInCapability.count} 次，plugincontract 内 ${contractHashInContract.count} 次；哈希格式由 validate.go 的 hashPattern 校验，Provider 侧完全不感知`
    },
    {
      concept: 'ProviderIdentity',
      serviceSide: { type: 'ServiceDescriptor.ProviderID string（拼接 moduleKey/serviceID）', enum: null, authority: 'plugincontract' },
      providerSide: { type: 'CapabilityProviderDefinition.ID ProviderID', enum: null, authority: 'capability' },
      authority: '两者同名不同源：服务图里的 providerId 是派生字符串，注册表里的是稳定 ID',
      conflict: 'duplicate-concept',
      detail: `ServiceCatalog 用 \`key + "/" + serviceID\` 现造 providerId（命中 ${catalogDerivedProviderID.count} 处）；capability.ParseProviderID 只做 TrimSpace，ValidateProviderID 定义数 ${providerIDValidation.count}，即没有格式与稳定性约束`
    },
    {
      concept: 'Priority',
      serviceSide: { type: 'ServiceOffer.Priority / ServiceDescriptor.Priority', enum: null, authority: 'plugincontract' },
      providerSide: { type: 'CapabilityProviderDefinition.Priority', enum: null, authority: 'capability' },
      authority: '路由选择以 capability 侧为准，服务侧 priority 仅参与图解析',
      conflict: 'duplicate-concept',
      detail: `两侧各有一个 int Priority，语义都是“越大越优先”，但排序代码分别实现在 service_graph.go 与 provider_registry.go，未共享比较器`
    }
  ]

  const checks = [
    { id: 'TARGET_STRUCTS_PARSED', ok: missing.length === 0, detail: missing },
    { id: 'GO_AST_CROSSCHECK', ok: crosscheckMismatches.length === 0, detail: crosscheckMismatches },
    { id: 'PLACEMENT_ENUM_DIVERGES', ok: placementOnlyService.length > 0, detail: { onlyService: placementOnlyService.map((entry) => entry.value), onlyProvider: placementOnlyProvider.map((entry) => entry.value) } },
    { id: 'PROVIDER_ID_UNVALIDATED', ok: providerIDValidation.count === 0, detail: { runtimeValidateFunctions: providerIDValidation.count, contractValidateFunctions: providerIDValidationInContract.count, derivedInCatalog: catalogDerivedProviderID.count } },
    { id: 'CONTRACT_HASH_SINGLE_OWNER', ok: contractHashInCapability.count === 0 && contractHashInContract.count > 0, detail: { capability: contractHashInCapability.count, plugincontract: contractHashInContract.count } },
    { id: 'SCOPE_NOT_DUPLICATED', ok: fieldsOf(structs, 'ScopeRule').join(',') === 'Type,ID', detail: fieldsOf(structs, 'ScopeRule') },
    { id: 'MAPPING_COVERS_REQUIRED_FIELDS', ok: ['Scope', 'Placement', 'Permission', 'Version', 'ContractHash'].every((concept) => mapping.some((row) => row.concept === concept)), detail: mapping.map((row) => row.concept) },
    { id: 'PACKAGES_INDEPENDENT', ok: contractImportsCapability.count === 0 && capabilityImportsContract.count === 0, detail: { contractImportsCapability: contractImportsCapability.count, capabilityImportsContract: capabilityImportsContract.count } },
    { id: 'CONTRACT_HASH_PRODUCER_PRESENT', ok: hashProducersInContract.count > 0, detail: { producers: hashProducersInContract.count, protocolVersionMentions: protocolVersionInContract.count } },
    { id: 'CONTRACT_PROTOCOL_DECLARED', ok: protocolVersionInContract.count > 0, detail: { protocolVersionMentions: protocolVersionInContract.count } }
  ]

  return {
    mapping,
    checks,
    enums: { servicePlacement, providerPlacement, serviceScopes },
    structs: [...structs.values()].map((entry) => ({ name: entry.name, file: entry.file, fields: entry.fields.length })),
    ok: checks.every((check) => check.ok)
  }
}

function sha256File(path) {
  return createHash('sha256').update(readFileSync(path)).digest('hex')
}

export function runInventory() {
  const structs = parseStructs([...SOURCES.plugincontract, ...SOURCES.capability])
  const dump = goParserDump()
  const inventory = buildInventory(structs, dump)
  const sourceHashes = [...SOURCES.plugincontract, ...SOURCES.capability].map((file) => ({
    file,
    sha256: sha256File(resolve(backendRoot, file))
  }))
  return {
    schemaVersion: 1,
    generatedAt: new Date().toISOString(),
    sourceRoot: 'backend',
    sourceHashes,
    structCount: structs.size,
    ...inventory
  }
}

function main() {
  try {
    const inventory = runInventory()
    mkdirSync(outDir, { recursive: true })
    writeFileSync(outPath, `${JSON.stringify(inventory, null, 2)}\n`, 'utf8')
    for (const check of inventory.checks) {
      if (!check.ok) process.stdout.write(`FAILED ${check.id} ${JSON.stringify(check.detail)}\n`)
    }
    process.stdout.write(`structs=${inventory.structCount} rows=${inventory.mapping.length} checks=${inventory.checks.length} ok=${inventory.ok}\n`)
    process.stdout.write(`artifact=${outPath}\n`)
    if (!inventory.ok) process.exit(24)
  } catch (error) {
    process.stderr.write(`${error.stack ?? error.message}\n`)
    if (error.cause) process.stderr.write(`cause: ${error.cause.stack ?? error.cause.message ?? error.cause}\n`)
    process.exit(25)
  }
}

if (process.argv[1] && resolve(process.argv[1]) === fileURLToPath(import.meta.url)) {
  main()
}
