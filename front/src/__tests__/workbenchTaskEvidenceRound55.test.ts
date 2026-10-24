import { describe, expect, it } from 'vitest';
import { parseWorkbenchTaskList } from '../components/workbench/workbench-task-list';

function response(reason: string) {
 return {protocolVersion:1,conversationId:'conv',source:'durable-checkpoint',truncated:false,tasks:[{coordinationId:'a',status:'waiting',updatedAt:'2026-10-23T00:00:00Z',recoveryState:'needs-reconciliation',recoveryReason:reason,assignmentCount:1,completedCount:0,failedCount:0,assignments:[{id:'child',status:'waiting',role:'coding_worker',objective:'inspect',reviewRequired:false}]}], summary:{total:1,active:1,paused:0,succeeded:0,failed:0,runtimeObserved:0,checkpointOnly:0,needsReconciliation:1,codingWorkersRequiringReview:0}};
}

describe('round55 durable terminal reconciliation contract', () => {
 it('accepts all server-authored terminal evidence failures without suppressing the UI', () => {
  for (const reason of ['terminal-assignment-missing','terminal-child-evidence-missing','terminal-child-reader-unavailable','terminal-child-read-budget-exceeded','terminal-child-evidence-conflict']) {
   const result = parseWorkbenchTaskList(response(reason), 'conv');
   expect(result.tasks[0].recoveryReason).toBe(reason);
   expect(result.summary.needsReconciliation).toBe(1);
  }
 });
 it('rejects arbitrary reasons to avoid displaying forged error codes', () => {
  expect(() => parseWorkbenchTaskList(response('terminal-unrecognized'), 'conv')).toThrow();
 });
});
