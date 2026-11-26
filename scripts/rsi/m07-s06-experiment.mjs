import { spawnSync } from 'node:child_process'
import { readFileSync, readdirSync, statSync } from 'node:fs'
import { join, resolve } from 'node:path'
import { cleanEnv, repoRoot } from './toolchain-preflight.mjs'

export const PACKAGE_PATH = './internal/extension/kernel/task_runtime'
export const RUN_PATTERN = 'TestExperimentGovernor'
export const BUILD_TAG = 'rsi_experiment'
export const EXIT_EXPERIMENT = 78
export const EXPECTED_PASSED = 14
export const GO_TEST_FLAGS = ['-tags', BUILD_TAG, '-race', '-count=1', '-timeout', '600s', '-run', RUN_PATTERN, '-v']
export const GOVERNOR_FILE = 'experiment_governor.go'
export const GOVERNOR_TEST_FILE = 'experiment_governor_test.go'
export const LADDER_PACKAGE_PATH = './internal/extension/kernel/enablement'
export const LADDER_TEST_FILE = 'experiment_ladder_test.go'
export const LADDER_TAGS = 'rsi_lifecycle rsi_experiment'
export const LADDER_RUN = 'TestExperimentRealPluginLadder'
export const LADDER_FLAGS = ['-tags', LADDER_TAGS, '-race', '-count=1', '-timeout', '600s', '-run', LADDER_RUN, '-v']

export const EVIDENCE_ROWS = [
  'idle paused=2 active=0',
  'foreground paused=2 activity_blocks_idle=true',
  'checkpoint progress=1 effects=1 stamped=true',
  'resume applied=2 skipped=2 total=4',
  'resume_twice applied=3 calls=1',
  'stale rejected=true',
  'expired rejected=true',
  'not_paused already_active=true',
  'pause errors=1 paused=2',
  'capacity rejected=true active=2',
  'load paused=2 denied=true code=task_resume_incompatible',
  'inflight rejected=true calls=0',
  'concurrent jobs=8 consistent=true',
  'real_ladder_zero plugins=0 processes=0 workers=0 spawn_calls=0',
  'real_ladder tiers=1,3,5,9 available=9 requested=10 gap=1 load_split=true'
]

export const CASES = [
  'TestExperimentGovernorIdleReclaimsAfterTTL',
  'TestExperimentGovernorForegroundPausesAndResetsIdle',
  'TestExperimentGovernorCheckpointIsStamped',
  'TestExperimentGovernorResumeAppliesOnlyPending',
  'TestExperimentGovernorResumeTwiceIsNoOp',
  'TestExperimentGovernorRejectsStaleLease',
  'TestExperimentGovernorRejectsExpiredCheckpoint',
  'TestExperimentGovernorResumeActiveIsNoOp',
  'TestExperimentGovernorPauseErrorIsolation',
  'TestExperimentGovernorCapacityRejects',
  'TestExperimentGovernorLoadDeniesResume',
  'TestExperimentGovernorRejectsInFlightEffect',
  'TestExperimentGovernorConcurrentPauseResume',
  'TestExperimentRealPluginLadder'
]

export const SOURCE_GUARDS = [
  'if checkpoint.Epoch != job.Epoch() {',
  'if g.config.CheckpointTTL > 0 && g.now().Sub(checkpoint.CreatedAt) > g.config.CheckpointTTL {',
  'if completed[effect] {',
  'sort.Strings(pending)',
  'if g.load >= ExperimentLoadHigh {',
  'if idleFor < g.config.IdleAfter {',
  'if active >= g.config.MaxActive {',
  'state.status = experimentPaused'
]

export function parseRows(log) {
  const body = typeof log === 'string' ? log : ''
  const rows = []
  for (const line of body.split('\n')) {
    const index = line.indexOf('m07-s06 ')
    if (index < 0) continue
    rows.push(line.slice(index + 'm07-s06 '.length).trim())
  }
  return rows
}

export function parseGoTestOutput(log) {
  const body = typeof log === 'string' ? log : ''
  return {
    passed: (body.match(/--- PASS: /g) ?? []).length,
    failed: (body.match(/--- FAIL: /g) ?? []).length
  }
}

export function parseGoListFiles(output) {
  const body = String(output ?? '').trim()
  if (!body.startsWith('[') || !body.endsWith(']')) return []
  const inner = body.slice(1, -1).trim()
  if (inner === '') return []
  return inner.split(/\s+/).sort()
}

export function countPluginPackages(root) {
  const found = new Set()
  const walk = (directory) => {
    let names
    try {
      names = readdirSync(directory)
    } catch {
      return
    }
    for (const name of names) {
      const path = join(directory, name)
      let stats
      try {
        stats = statSync(path)
      } catch {
        continue
      }
      if (stats.isDirectory()) {
        if (name === 'node_modules' || name === '.git' || name === 'assets') continue
        walk(path)
        continue
      }
      if (name === 'amitia-extension.json') found.add(path)
    }
  }
  for (const directory of ['plugins-code', 'plugins-builtin', 'testplugins']) walk(resolve(root, directory))
  return found.size
}

export function validateIsolation(evidence) {
  const problems = []
  const defaultFiles = Array.isArray(evidence?.defaultFiles) ? evidence.defaultFiles : []
  const taggedFiles = Array.isArray(evidence?.taggedFiles) ? evidence.taggedFiles : []
  const defaultTests = Array.isArray(evidence?.defaultTests) ? evidence.defaultTests : []
  const taggedTests = Array.isArray(evidence?.taggedTests) ? evidence.taggedTests : []
  if (defaultFiles.includes(GOVERNOR_FILE)) problems.push(`default build leaked ${GOVERNOR_FILE}`)
  if (!taggedFiles.includes(GOVERNOR_FILE)) problems.push(`tagged build missing ${GOVERNOR_FILE}`)
  if (defaultTests.includes(GOVERNOR_TEST_FILE)) problems.push(`default tests leaked ${GOVERNOR_TEST_FILE}`)
  if (!taggedTests.includes(GOVERNOR_TEST_FILE)) problems.push(`tagged tests missing ${GOVERNOR_TEST_FILE}`)
  if (taggedFiles.length !== defaultFiles.length + 1) problems.push(`tagged files=${taggedFiles.length} want default+1=${defaultFiles.length + 1}`)
  return problems
}

export function validateLadderIsolation(evidence) {
  const problems = []
  const defaultTests = Array.isArray(evidence?.ladderDefaultTests) ? evidence.ladderDefaultTests : []
  const taggedTests = Array.isArray(evidence?.ladderTaggedTests) ? evidence.ladderTaggedTests : []
  if (defaultTests.includes(LADDER_TEST_FILE)) problems.push(`default tests leaked ${LADDER_TEST_FILE}`)
  if (!taggedTests.includes(LADDER_TEST_FILE)) problems.push(`tagged tests missing ${LADDER_TEST_FILE}`)
  if (taggedTests.length <= defaultTests.length) problems.push(`ladder tagged tests=${taggedTests.length} want > default=${defaultTests.length}`)
  return problems
}

export function validateSource(root) {
  const path = resolve(root, 'backend', 'internal', 'extension', 'kernel', 'task_runtime', GOVERNOR_FILE)
  const body = readFileSync(path, 'utf8')
  return SOURCE_GUARDS.filter((guard) => !body.includes(guard)).map((guard) => `experiment governor is missing ${guard}`)
}

export function validatePendingFilter(root) {
  const path = resolve(root, 'backend', 'internal', 'extension', 'kernel', 'task_runtime', GOVERNOR_FILE)
  const body = readFileSync(path, 'utf8')
  const problems = []
  if (body.split('completed[effect]').length - 1 !== 3) problems.push('completed-effect filter must appear exactly three times (build + inflight guard + pending skip)')
  if (!body.includes('skipped++')) problems.push('resume must count skipped completed effects')
  return problems
}

export function validateLadderSource(root) {
  const path = resolve(root, 'backend', 'internal', 'extension', 'kernel', 'enablement', LADDER_TEST_FILE)
  const body = readFileSync(path, 'utf8')
  const problems = []
  if (!body.includes('plugincontract.Parse(data)')) problems.push('ladder must parse real manifests with plugincontract.Parse')
  if (!body.includes('InstallBatch(context.Background(), manifests[:count])')) problems.push('ladder must install real manifests')
  if (!body.includes('installer.Activate(context.Background(), manifests[i].ID, 2)')) problems.push('ladder must activate with a fresh epoch')
  if (!body.includes('if activations > 3 {')) problems.push('ladder must keep activation below installation for large tiers')
  return problems
}

export function validateCaseNames(text) {
  const body = String(text ?? '')
  return CASES.filter((name) => !body.includes(`func ${name}(t *testing.T) {`)).map((name) => `experiment test is missing ${name}`)
}

export function validateEvidence(evidence) {
  const problems = []
  const rows = Array.isArray(evidence?.rows) ? evidence.rows : []
  if (rows.length !== EVIDENCE_ROWS.length) problems.push(`evidence rows=${rows.length} want ${EVIDENCE_ROWS.length}`)
  for (const expected of EVIDENCE_ROWS) {
    const count = rows.filter((row) => row === expected).length
    if (count !== 1) problems.push(`evidence row must print exactly once (${count}): ${expected}`)
  }
  if (evidence?.passed !== EXPECTED_PASSED) problems.push(`passed=${evidence?.passed} want ${EXPECTED_PASSED}`)
  if (evidence?.failed !== 0) problems.push(`failed=${evidence?.failed} want 0`)
  if (evidence?.exitCode !== 0) problems.push(`go test exit=${evidence?.exitCode} want 0`)
  if (evidence?.race !== true) problems.push('go test must run with -race')
  return problems
}

export function renderEvidence(evidence) {
  const guards = CASES.length + SOURCE_GUARDS.length + 2
  const unique = evidence?.pluginPackages ?? 0
  const lines = [
    `invocation=experiment package=${PACKAGE_PATH} ladder=${LADDER_PACKAGE_PATH} run=${RUN_PATTERN} rows=${EVIDENCE_ROWS.length} guards=${guards}`,
    `isolation: default_files=${evidence?.defaultFiles?.length ?? 0} tagged_files=${evidence?.taggedFiles?.length ?? 0} default_tests=${evidence?.defaultTests?.length ?? 0} tagged_tests=${evidence?.taggedTests?.length ?? 0}`,
    `ladder_isolation: default_tests=${evidence?.ladderDefaultTests?.length ?? 0} tagged_tests=${evidence?.ladderTaggedTests?.length ?? 0}`,
    `plugin_supply unique=${unique} required=10 gap=${unique < 10} source=amitia-extension`
  ]
  for (const row of evidence?.rows ?? []) lines.push(`m07-s06 ${row}`)
  lines.push(`passed=${evidence?.passed ?? 0} failed=${evidence?.failed ?? 0} exit=${evidence?.exitCode ?? 1} race=${evidence?.race === true}`)
  return lines.join('\n') + '\n'
}

function goList(root, pkg, extraArgs) {
  const result = spawnSync('go', ['list', '-f', '{{.GoFiles}}|{{.TestGoFiles}}|{{.XTestGoFiles}}', ...extraArgs, pkg], {
    cwd: resolve(root, 'backend'),
    env: cleanEnv(),
    encoding: 'utf8',
    timeout: 120000
  })
  const line = `${result.stdout ?? ''}`.trim()
  const [files = '', tests = '', external = ''] = line.split('|')
  return {
    files: parseGoListFiles(files),
    tests: [...parseGoListFiles(tests), ...parseGoListFiles(external)].sort()
  }
}

function runGoTest(root, flags, pkg) {
  return spawnSync('go', ['test', ...flags, pkg], {
    cwd: resolve(root, 'backend'),
    env: cleanEnv(),
    encoding: 'utf8',
    timeout: 1200000
  })
}

export function runExperimentEvidence(root) {
  const defaultList = goList(root, PACKAGE_PATH, [])
  const taggedList = goList(root, PACKAGE_PATH, ['-tags', BUILD_TAG])
  const ladderDefault = goList(root, LADDER_PACKAGE_PATH, [])
  const ladderTagged = goList(root, LADDER_PACKAGE_PATH, ['-tags', LADDER_TAGS])
  const testSource = readFileSync(resolve(root, 'backend', 'internal', 'extension', 'kernel', 'task_runtime', GOVERNOR_TEST_FILE), 'utf8') + '\n' + readFileSync(resolve(root, 'backend', 'internal', 'extension', 'kernel', 'enablement', LADDER_TEST_FILE), 'utf8')
  const governorResult = runGoTest(root, GO_TEST_FLAGS, PACKAGE_PATH)
  const ladderResult = runGoTest(root, LADDER_FLAGS, LADDER_PACKAGE_PATH)
  const log = `${governorResult.stdout ?? ''}${governorResult.stderr ?? ''}\n${ladderResult.stdout ?? ''}${ladderResult.stderr ?? ''}`
  const counts = parseGoTestOutput(log)
  return {
    testSource,
    rows: parseRows(log),
    passed: counts.passed,
    failed: counts.failed,
    exitCode: governorResult.status === 0 && ladderResult.status === 0 ? 0 : 1,
    race: GO_TEST_FLAGS.includes('-race') && LADDER_FLAGS.includes('-race'),
    defaultFiles: defaultList.files,
    taggedFiles: taggedList.files,
    defaultTests: defaultList.tests,
    taggedTests: taggedList.tests,
    ladderDefaultTests: ladderDefault.tests,
    ladderTaggedTests: ladderTagged.tests,
    pluginPackages: countPluginPackages(root),
    log
  }
}

export function main(root = repoRoot) {
  const evidence = runExperimentEvidence(root)
  const problems = [
    ...validateIsolation(evidence),
    ...validateLadderIsolation(evidence),
    ...validateEvidence(evidence),
    ...validateSource(root),
    ...validatePendingFilter(root),
    ...validateLadderSource(root),
    ...validateCaseNames(evidence.testSource)
  ]
  process.stdout.write(renderEvidence(evidence))
  for (const problem of problems) process.stdout.write(`PROBLEM ${problem}\n`)
  if (problems.length > 0) {
    process.exit(EXIT_EXPERIMENT)
  }
  process.exit(0)
}

if (process.argv[1] && resolve(process.argv[1]) === resolve(new URL(import.meta.url).pathname)) {
  main()
}
