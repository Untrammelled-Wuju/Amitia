import assert from "node:assert/strict";
import test from "node:test";
import { createDshClientJobsController } from "../src/client-abi/jobs-controller.mjs";

test("loads watched job rows and bounded observation output", async () => {
  const calls = [];
  let jobStatus = "running";
  const controller = createDshClientJobsController(async (input) => {
    calls.push(input);
    if (input.action === "jobs-list") {
      return {
        jobs: [{
          id: "job-1",
          pluginId: "@example/jobs",
          owner: "session-1",
          kind: "bash",
          label: "npm test",
          status: jobStatus,
          startedAt: 10,
          finishedAt: jobStatus === "running" ? undefined : 20,
          output: { total: 7, earliest: 0 },
          text: "running",
        }],
      };
    }
    if (input.action === "jobs-kill") {
      jobStatus = "killed";
      return { ok: true };
    }
    throw new Error(`unexpected command ${input.action}`);
  }, { pollInterval: 10_000 });
  let invalidations = 0;
  const unsubscribe = controller.state.subscribe(() => { invalidations += 1; });
  const stopWatch = controller.watchRows("session-1");
  await new Promise((resolve) => setTimeout(resolve, 0));
  assert.equal(controller.state.getSnapshot().rows["session-1"][0].id, "job-1");
  const stopObserve = controller.observe("session-1", "job-1");
  await new Promise((resolve) => setTimeout(resolve, 0));
  assert.deepEqual(controller.state.getSnapshot().observed["job-1"], {
    jobId: "job-1",
    text: "running",
    gapBefore: false,
    streaming: true,
  });
  assert.deepEqual(await controller.kill("session-1", "job-1"), { ok: true });
  assert.equal(controller.state.getSnapshot().rows["session-1"][0].status, "killed");
  assert.equal(calls.some((input) => input.action === "jobs-kill" && input.pluginId === "@example/jobs"), true);
  assert.equal(invalidations > 0, true);
  stopObserve();
  stopWatch();
  unsubscribe();
  await controller.dispose();
});
