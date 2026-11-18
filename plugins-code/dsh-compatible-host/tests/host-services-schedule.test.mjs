import assert from "node:assert/strict";
import test from "node:test";
import { createAmitiaDshHostServices } from "../src/runtime/host-services.mjs";

function catalogEntry(scheduleId = "sched-1", state = {}) {
  return {
    schedules: [
      {
        scheduleId,
        name: "每日提醒",
        description: "提醒喝水",
        timezone: "Asia/Shanghai",
        trigger: { type: "daily", daily: { time: "09:00" } },
        target: {
          type: "session_prompt",
          targetId: "conv-1",
          inputTemplate: {
            prompt: "喝水",
            dshTargetKind: "session",
            dshSessionId: "conv-1",
            dshTitle: "每日提醒",
            dshTiming: { kind: "daily", daily: { time: "09:00", time_zone: "Asia/Shanghai" } },
          },
        },
      },
    ],
    states: { [scheduleId]: state },
  };
}

test("schedule.list 把宿主定时任务映射为 DSH 条目并按 active 过滤", async () => {
  const catalog = catalogEntry("sched-1", { status: "active", nextScheduledAt: "2026-11-19T01:00:00.000Z" });
  const calls = [];
  const host = {
    calls,
    async call(method, input) {
      calls.push({ method, input });
      if (method === "host.schedule.list") return catalog;
      return { method, input };
    },
  };
  const services = createAmitiaDshHostServices(host);
  const page = await services.schedule.list({});
  const items = page.items;
  assert.equal(page.total, 1);
  assert.equal(items.length, 1);
  assert.equal(items[0].id, "sched-1");
  assert.equal(items[0].kind, "daily");
  assert.equal(items[0].status, "active");
  assert.equal(items[0].target.kind, "session");
  assert.equal(items[0].target.sessionId, "conv-1");
  const active = await services.schedule.list({ active: true });
  assert.equal(active.total, 1);
  const pausedHost = {
    async call(method) {
      if (method === "host.schedule.list") return catalogEntry("sched-1", { status: "paused" });
      return {};
    },
  };
  const pausedServices = createAmitiaDshHostServices(pausedHost);
  const pausedAll = await pausedServices.schedule.list({});
  assert.equal(pausedAll.items[0].status, "paused");
  const pausedActive = await pausedServices.schedule.list({ active: true });
  assert.equal(pausedActive.total, 0);
});

test("schedule.create 调用 host.schedule.create 并回读目录条目", async () => {
  const created = [];
  const host = {
    async call(method, input) {
      if (method === "host.schedule.create") {
        created.push(input);
        const definition = {
          ...input,
          scheduleId: input.scheduleId,
          target: {
            ...input.target,
            inputTemplate: { ...input.target.inputTemplate, dshTiming: { kind: "daily", daily: { time: "09:00", time_zone: "Asia/Shanghai" } } },
          },
        };
        host.definition = definition;
        return { scheduleId: input.scheduleId };
      }
      if (method === "host.schedule.list") {
        return { schedules: host.definition ? [host.definition] : [], states: { [host.definition?.scheduleId]: { status: "active" } } };
      }
      return {};
    },
  };
  const services = createAmitiaDshHostServices(host);
  const entry = await services.schedule.create({
    title: "每日提醒",
    prompt: "喝水",
    timing: { kind: "daily", daily: { time: "09:00", time_zone: "Asia/Shanghai" } },
    target: { kind: "session", sessionId: "conv-1" },
  });
  assert.equal(created.length, 1);
  assert.equal(created[0].name, "每日提醒");
  assert.equal(created[0].target.type, "session_prompt");
  assert.equal(entry.title, "每日提醒");
  assert.equal(entry.target.sessionId, "conv-1");
  assert.equal(entry.status, "active");
});
