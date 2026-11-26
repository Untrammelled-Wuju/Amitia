import { test } from 'node:test'
import assert from 'node:assert/strict'
import {
  BUILD_TAGS,
  EXIT_SOAK,
  FD_DELTA,
  GOROUTINE_DELTA,
  HEAP_DELTA_LIMIT,
  HEAP_SLOPE_LIMIT,
  MIN_SAMPLES,
  PACKAGE_PATH,
  RUN_PATTERN,
  SOAK_FILE,
  SOAK_SECONDS,
  TIMERS_MAX,
  buildBaseline,
  linearSlope,
  parseDetail,
  parseSamples,
  percentile,
  renderEvidence,
  validateCurve
} from '../m07-s08-soak.mjs'

function samples(count = 120) {
  const list = []
  for (let i = 0; i < count; i++) {
    list.push({ ms: 250 * i, heap: 2_000_000 + i * 1024, goroutines: 8, fds: 8, timers: i % 20 === 0 ? 1 : 0, depth: 10, residue: 0 })
  }
  return list
}

test('S08 退出码为该步独占', () => {
  assert.equal(EXIT_SOAK, 80)
})

test('S08 窗口与阈值固定', () => {
  assert.equal(SOAK_SECONDS, 30)
  assert.equal(MIN_SAMPLES, 40)
  assert.equal(HEAP_SLOPE_LIMIT, 256 * 1024)
  assert.equal(HEAP_DELTA_LIMIT, 8 * 1024 * 1024)
  assert.equal(GOROUTINE_DELTA, 2)
  assert.equal(FD_DELTA, 2)
  assert.equal(TIMERS_MAX, 8)
  assert.equal(BUILD_TAGS, 'rsi_backpressure rsi_soak')
  assert.equal(PACKAGE_PATH, './internal/extension/kernel/task_runtime')
  assert.equal(RUN_PATTERN, 'TestSoakCurve')
  assert.equal(SOAK_FILE, 'soak_curve_test.go')
})

test('S08 样本解析只认 m07-s08-sample 前缀', () => {
  const log = [
    'm07-s08-sample 250 2000000 8 7 0 5 0',
    'm07-s08-sample bad line',
    'm07-s08-detail window_seconds=30 samples=1',
    'm07-s08-sample 500 2100000 8 7 1 6 0'
  ].join('\n')
  assert.equal(parseSamples(log).length, 2)
  assert.deepEqual(parseSamples(log)[1], { ms: 500, heap: 2100000, goroutines: 8, fds: 7, timers: 1, depth: 6, residue: 0 })
})

test('S08 detail 解析为数值键值', () => {
  const detail = parseDetail('m07-s08-detail window_seconds=30 samples=119 base_goroutines=8 base_fds=8 residue=0')
  assert.deepEqual(detail, { window_seconds: 30, samples: 119, base_goroutines: 8, base_fds: 8, residue: 0 })
  assert.equal(parseDetail('nothing'), null)
})

test('S08 百分位与斜率计算', () => {
  assert.equal(percentile([], 0.5), 0)
  assert.equal(percentile([1, 2, 3, 4], 0.5), 2)
  assert.equal(percentile([1, 2, 3, 4], 0.95), 4)
  assert.equal(linearSlope([1, 2, 3]), 1)
  assert.equal(linearSlope([5]), 0)
})

test('S08 平稳曲线通过', () => {
  assert.deepEqual(validateCurve(samples()), [])
})

test('S08 样本不足被拦下', () => {
  assert.ok(validateCurve(samples(20)).some((p) => p.includes('samples=20')))
})

test('S08 堆斜率超限被拦下', () => {
  const list = samples()
  for (let i = 0; i < list.length; i++) list[i].heap = 2_000_000 + i * 2_000_000
  assert.ok(validateCurve(list).some((p) => p.includes('heap slope=')))
})

test('S08 堆增量超限被拦下', () => {
  const list = samples()
  list[0].heap = 0
  list[list.length - 1].heap = HEAP_DELTA_LIMIT + 1
  const problems = validateCurve(list)
  assert.ok(problems.some((p) => p.includes('heap delta=')) || problems.some((p) => p.includes('heap slope=')))
})

test('S08 goroutine 与 FD 增长被拦下', () => {
  const list = samples()
  list[list.length - 1].goroutines = list[0].goroutines + GOROUTINE_DELTA + 1
  list[list.length - 1].fds = list[0].fds + FD_DELTA + 1
  const problems = validateCurve(list)
  assert.ok(problems.some((p) => p.includes('goroutines delta=')))
  assert.ok(problems.some((p) => p.includes('fds delta=')))
})

test('S08 FD 不可用与计时器超限被拦下', () => {
  const list = samples()
  list[0].fds = -1
  list[1].timers = TIMERS_MAX + 1
  const problems = validateCurve(list)
  assert.ok(problems.some((p) => p.includes('fd curve unavailable')))
  assert.ok(problems.some((p) => p.includes('timers max=')))
})

test('S08 残留非零被拦下', () => {
  const list = samples()
  list[10].residue = 1
  assert.ok(validateCurve(list).some((p) => p.includes('residue max=1')))
  const last = samples()
  last[last.length - 1].timers = 1
  assert.ok(validateCurve(last).some((p) => p.includes('timers last=1')))
})

test('S08 基线 JSON 记录五类曲线与 24h BLOCKED', () => {
  const list = samples()
  const baseline = buildBaseline(list, { residue: 0 }, 30)
  assert.equal(baseline.module, 'M07')
  assert.equal(baseline.step, 'S08')
  assert.equal(baseline.samples, list.length)
  assert.equal(baseline.fds.available, true)
  assert.equal(baseline.timers.max, 1)
  assert.equal(baseline.residue.max, 0)
  assert.equal(baseline.soak24h.status, 'BLOCKED')
  assert.equal(baseline.soak24h.reason, 'environment_not_available')
})

test('S08 渲染证据恒定 8 行且不写动态数字', () => {
  const list = samples()
  const baseline = buildBaseline(list, { residue: 0 }, 30)
  const text = renderEvidence(list, { residue: 0 }, baseline, 30)
  assert.equal(text.trim().split('\n').filter((line) => line.startsWith('m07-s08 ')).length, 8)
  assert.ok(text.includes('m07-s08 window seconds=30'))
  assert.ok(text.includes('m07-s08 memory slope_within=true delta_within=true'))
  assert.ok(text.includes('m07-s08 residue max=0'))
  assert.ok(text.includes('m07-s08 soak_24h status=BLOCKED reason=environment_not_available'))
  assert.ok(!text.includes('samples=120'))
})
