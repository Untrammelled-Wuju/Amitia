import { describe, expect, it } from 'vitest';
import { projectWorkbenchDependencies } from '../components/workbench/workbench-task-dependency-view';
import type { WorkbenchTaskAssignment } from '../components/workbench/workbench-task-list';

function child(id: string, status: string, dependencies: string[] = []): WorkbenchTaskAssignment {
  return { id, status, dependencies, role: 'worker', objective: id, reviewRequired: false };
}

describe('M08 persisted DAG presentation', () => {
  it('orders an unsorted DAG by prerequisites without changing its stored identity', () => {
    const source = [child('third', 'pending', ['second']), child('second', 'pending', ['first']), child('first', 'succeeded')];
    const rows = projectWorkbenchDependencies(source);
    expect(rows.map(row => row.assignment.id)).toEqual(['first', 'second', 'third']);
    expect(rows.map(row => row.depth)).toEqual([0, 1, 2]);
    expect(rows[1].notice).toBe('');
    expect(rows[2].notice).toContain('等待');
    expect(source.map(row => row.id)).toEqual(['third', 'second', 'first']);
  });
  it('distinguishes blocked, failed predecessor and inconsistent success', () => {
    const rows = projectWorkbenchDependencies([child('blocked', 'pending', ['pending']), child('failed', 'cancelled'), child('pending', 'waiting'), child('needs-review', 'waiting', ['failed']), child('invalid-success', 'succeeded', ['pending'])]);
    expect(rows.find(row => row.assignment.id === 'blocked')?.notice).toContain('等待');
    expect(rows.find(row => row.assignment.id === 'needs-review')?.notice).toContain('失败');
    expect(rows.find(row => row.assignment.id === 'invalid-success')?.inconsistent).toBe(true);
  });
  it('rejects missing or cyclic prerequisites', () => {
    expect(() => projectWorkbenchDependencies([child('one', 'pending', ['missing'])])).toThrow();
    expect(() => projectWorkbenchDependencies([child('one', 'pending', ['two']), child('two', 'pending', ['one'])])).toThrow();
  });
});
