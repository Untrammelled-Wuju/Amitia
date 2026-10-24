import { describe, expect, it } from 'vitest';
import { parseWorkbenchTaskList } from '../components/workbench/workbench-task-list';

const task = { coordinationId:'coord', status:'running', updatedAt:'2026-10-22T12:00:00Z', recoveryState:'checkpoint-only', assignmentCount:1, completedCount:0, failedCount:0, assignments:[{id:'a',status:'running',role:'coding_worker',objective:'inspect',reviewRequired:true}], goalId:'goal', goalRevision:3, executionCoreId:'core-1', deviceId:'device-2', deadlineAt:'2026-10-23T12:00:00Z', cancelRequested:true };
const response = {protocolVersion:1, conversationId:'conv', source:'durable-checkpoint', truncated:false, summary:{total:1, active:1,paused:0,succeeded:0,failed:0,runtimeObserved:0,checkpointOnly:1,needsReconciliation:0,codingWorkersRequiringReview:1}, tasks:[task]};
describe('M08 durable goal/location contract', () => {
  it('accepts the same optional projected fields as Flutter', () => {
    expect(parseWorkbenchTaskList(response,'conv').tasks[0]).toMatchObject({goalId:'goal',goalRevision:3,executionCoreId:'core-1',deviceId:'device-2',cancelRequested:true});
  });
  it('rejects malformed deadline, revision, and cancellation intent', () => {
    for (const fields of [{deadlineAt:'bad'}, {goalRevision:-1}, {cancelRequested:'false'}]) {
      expect(() => parseWorkbenchTaskList({...response, tasks:[{...task,...fields}]},'conv')).toThrow();
    }
  });
  it('keeps the old v1 contract valid when optional fields are absent', () => {
    const {goalId, goalRevision, executionCoreId, deviceId, deadlineAt, cancelRequested, ...legacy} = task;
    expect(parseWorkbenchTaskList({...response,tasks:[legacy]},'conv').tasks).toHaveLength(1);
  });
});
