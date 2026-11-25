import assert from 'node:assert/strict'
import { test } from 'node:test'
import { compareSnapshots, schemaDelta, volatileTables } from '../featuregate-sideeffect-guard.mjs'

function snapshot(digest, entries, rows) {
  return { digest, entries, rowCounts: rows }
}

const tableEntry = (name, sql) => ({ type: 'table', name, table: name, sql })

test('schema 差分识别新增/删除/改写', () => {
  const baseline = [tableEntry('t1', 'CREATE TABLE t1 (id INTEGER)'), tableEntry('t2', 'CREATE TABLE t2 (id INTEGER)')]
  const current = [tableEntry('t1', 'CREATE TABLE t1 (id INTEGER, note TEXT)'), tableEntry('t3', 'CREATE TABLE t3 (id INTEGER)')]
  const delta = schemaDelta(baseline, current)
  assert.deepEqual(delta.added, ['table:t3'])
  assert.deepEqual(delta.removed, ['table:t2'])
  assert.deepEqual(delta.changed.map((item) => item.name), ['table:t1'])
})

test('关闭态快照对比能同时报出 schema 与行数漂移', () => {
  const baseline = { 'app.db': snapshot('aaa', [tableEntry('t1', 'CREATE TABLE t1 (id INTEGER)')], { t1: 2 }) }
  const current = { 'app.db': snapshot('bbb', [tableEntry('t1', 'CREATE TABLE t1 (id INTEGER)'), tableEntry('extra', 'CREATE TABLE extra (id INTEGER)')], { t1: 3, extra: 0 }) }
  const drift = compareSnapshots(baseline, current, new Set())
  const kinds = drift.map((item) => item.kind).sort()
  assert.deepEqual(kinds, ['ROWS_CHANGED', 'SCHEMA_CHANGED'])
  assert.deepEqual(drift.find((item) => item.kind === 'SCHEMA_CHANGED').delta.added, ['table:extra'])
  assert.deepEqual(
    { table: drift.find((item) => item.kind === 'ROWS_CHANGED').table, baseline: drift.find((item) => item.kind === 'ROWS_CHANGED').baseline, current: drift.find((item) => item.kind === 'ROWS_CHANGED').current },
    { table: 't1', baseline: 2, current: 3 }
  )
})

test('缺失或新增数据库文件必须报出', () => {
  const baseline = { 'app.db': snapshot('aaa', [], {}) }
  const current = { 'app.db': snapshot('aaa', [], {}), 'extensions-v2/kernel.db': snapshot('ccc', [], {}) }
  const drift = compareSnapshots(baseline, current, new Set())
  assert.deepEqual(drift.map((item) => item.kind), ['DATABASE_ADDED'])
  const reverse = compareSnapshots(current, baseline, new Set())
  assert.deepEqual(reverse.map((item) => item.kind), ['DATABASE_REMOVED'])
})

test('易变表在两次同构启动间被识别并从行数对比中排除', () => {
  const first = { 'app.db': snapshot('aaa', [], { t1: 1, volatile: 3 }) }
  const second = { 'app.db': snapshot('aaa', [], { t1: 1, volatile: 5 }) }
  const volatile = volatileTables(first, second)
  assert.deepEqual(volatile, ['app.db::volatile'])
  const baseline = { 'app.db': snapshot('aaa', [], { t1: 1, volatile: 3 }) }
  assert.deepEqual(compareSnapshots(baseline, second, new Set(volatile)), [])
  assert.equal(compareSnapshots(baseline, second, new Set()).length, 1)
})
