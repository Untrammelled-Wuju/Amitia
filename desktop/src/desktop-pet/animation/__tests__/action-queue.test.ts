import { describe, it, expect, beforeEach } from "vitest";
import { ActionQueue } from "../action-queue";
import type { PlayActionCommand } from "../contracts";

function makeCommand(overrides?: Partial<PlayActionCommand>): PlayActionCommand {
  return {
    commandId: `cmd_${Math.random().toString(36).slice(2, 8)}`,
    playbackInstanceId: `pbi_${Math.random().toString(36).slice(2, 8)}`,
    idempotencyKey: `idem_${Math.random().toString(36).slice(2, 8)}`,
    installationId: "inst-1",
    petInstanceId: "pet-1",
    packageRevision: 1,
    actionKey: "wave",
    priority: 50,
    queuePolicy: "enqueue",
    interruptPolicy: "respect_action",
    playbackRate: 1,
    issuedAt: new Date().toISOString(),
    ...overrides,
  };
}

describe("ActionQueue", () => {
  let queue: ActionQueue;

  beforeEach(() => {
    queue = new ActionQueue();
  });

  describe("empty queue", () => {
    it("size is 0, peek returns null, dequeue returns null", () => {
      expect(queue.size()).toBe(0);
      expect(queue.peek()).toBeNull();
      expect(queue.dequeue()).toBeNull();
    });
  });

  describe("enqueue", () => {
    it("adds an item and increases size", () => {
      const cmd = makeCommand({ commandId: "cmd_1" });
      const queued = queue.enqueue(cmd, 100);
      expect(queued).not.toBeNull();
      expect(queued?.command.commandId).toBe("cmd_1");
      expect(queued?.ack.status).toBe("queued");
      expect(queue.size()).toBe(1);
      expect(queue.peek()?.command.commandId).toBe("cmd_1");
    });

    it("returns an ack with the correct queue position", () => {
      queue.enqueue(makeCommand({ commandId: "first", actionKey: "act_first" }), 100);
      const second = queue.enqueue(
        makeCommand({ commandId: "second", actionKey: "act_second" }),
        200,
      );
      const third = queue.enqueue(
        makeCommand({ commandId: "third", actionKey: "act_third" }),
        300,
      );
      expect(second?.ack.queuePosition).toBe(1);
      expect(third?.ack.queuePosition).toBe(2);
    });
  });

  describe("dequeue", () => {
    it("removes and returns the head (FIFO for same priority)", () => {
      const a = makeCommand({ commandId: "cmd_a" });
      const b = makeCommand({ commandId: "cmd_b" });
      queue.enqueue(a, 100);
      queue.enqueue(b, 200);
      expect(queue.size()).toBe(2);

      const head = queue.dequeue();
      expect(head?.command.commandId).toBe("cmd_a");
      expect(queue.size()).toBe(1);

      const second = queue.dequeue();
      expect(second?.command.commandId).toBe("cmd_b");
      expect(queue.size()).toBe(0);
      expect(queue.dequeue()).toBeNull();
    });
  });

  describe("peek", () => {
    it("returns the head without removing it", () => {
      const a = makeCommand({ commandId: "cmd_a" });
      queue.enqueue(a, 100);
      expect(queue.peek()?.command.commandId).toBe("cmd_a");
      expect(queue.size()).toBe(1);
      expect(queue.peek()?.command.commandId).toBe("cmd_a");
      expect(queue.size()).toBe(1);
    });
  });

  describe("remove", () => {
    it("removes the correct item by commandId", () => {
      const a = makeCommand({ commandId: "cmd_a", actionKey: "act_a" });
      const b = makeCommand({ commandId: "cmd_b", actionKey: "act_b" });
      const c = makeCommand({ commandId: "cmd_c", actionKey: "act_c" });
      queue.enqueue(a, 100);
      queue.enqueue(b, 200);
      queue.enqueue(c, 300);

      expect(queue.remove("cmd_b")).toBe(true);
      expect(queue.size()).toBe(2);
      expect(queue.toArray().map((q) => q.command.commandId)).toEqual([
        "cmd_a",
        "cmd_c",
      ]);
      expect(queue.peek()?.command.commandId).toBe("cmd_a");
    });

    it("returns false for a non-existent commandId", () => {
      queue.enqueue(makeCommand({ commandId: "cmd_a" }), 100);
      expect(queue.remove("does_not_exist")).toBe(false);
      expect(queue.size()).toBe(1);
    });
  });

  describe("priority ordering", () => {
    it("places higher priority items at the front", () => {
      const low = makeCommand({ commandId: "low", priority: 10, actionKey: "act_low" });
      const mid = makeCommand({ commandId: "mid", priority: 50, actionKey: "act_mid" });
      const high = makeCommand({ commandId: "high", priority: 90, actionKey: "act_high" });
      queue.enqueue(low, 100);
      queue.enqueue(mid, 200);
      queue.enqueue(high, 300);

      const order = queue.toArray().map((q) => q.command.commandId);
      expect(order).toEqual(["high", "mid", "low"]);
      expect(queue.peek()?.command.commandId).toBe("high");
    });

    it("preserves insertion order (FIFO) for same priority", () => {
      queue.enqueue(makeCommand({ commandId: "first", actionKey: "act_first" }), 100);
      queue.enqueue(makeCommand({ commandId: "second", actionKey: "act_second" }), 200);
      queue.enqueue(makeCommand({ commandId: "third", actionKey: "act_third" }), 300);

      const ids: string[] = [];
      let item = queue.dequeue();
      while (item) {
        ids.push(item.command.commandId);
        item = queue.dequeue();
      }
      expect(ids).toEqual(["first", "second", "third"]);
    });
  });

  describe("coalesce", () => {
    it("replaces items with the same actionKey", () => {
      queue = new ActionQueue({
        limits: { total: 16, perActionKey: 16, perMutexGroup: 16 },
      });
      queue.enqueue(makeCommand({ commandId: "a", actionKey: "wave" }), 100);
      queue.enqueue(makeCommand({ commandId: "b", actionKey: "wave" }), 200);
      expect(queue.size()).toBe(2);

      const result = queue.coalesce(
        makeCommand({ commandId: "c", actionKey: "wave" }),
        300,
      );
      expect(result.command.commandId).toBe("c");
      expect(queue.size()).toBe(1);
      expect(queue.peek()?.command.commandId).toBe("c");
    });

    it("replaces items within the same mutexGroup", () => {
      const resolveMutex = (actionKey: string) =>
        actionKey === "wave" || actionKey === "nod" ? "head" : null;
      queue = new ActionQueue({
        limits: { total: 16, perActionKey: 16, perMutexGroup: 16 },
        resolveMutexGroup: resolveMutex,
      });
      queue.enqueue(makeCommand({ commandId: "a", actionKey: "wave" }), 100);
      queue.enqueue(makeCommand({ commandId: "b", actionKey: "nod" }), 200);
      expect(queue.size()).toBe(2);

      queue.coalesce(makeCommand({ commandId: "c", actionKey: "wave" }), 300);
      expect(queue.size()).toBe(1);
      expect(queue.peek()?.command.commandId).toBe("c");
    });
  });

  describe("clear", () => {
    it("empties the queue", () => {
      queue.enqueue(makeCommand({ commandId: "a" }), 100);
      queue.enqueue(makeCommand({ commandId: "b" }), 200);
      expect(queue.size()).toBe(2);

      queue.clear();
      expect(queue.size()).toBe(0);
      expect(queue.peek()).toBeNull();
      expect(queue.toArray()).toEqual([]);
    });
  });

  describe("getByActionKey", () => {
    it("returns matching items", () => {
      queue = new ActionQueue({
        limits: { total: 16, perActionKey: 16, perMutexGroup: 16 },
      });
      queue.enqueue(makeCommand({ commandId: "a", actionKey: "wave" }), 100);
      queue.enqueue(makeCommand({ commandId: "b", actionKey: "nod" }), 200);
      queue.enqueue(makeCommand({ commandId: "c", actionKey: "wave" }), 300);

      expect(queue.getByActionKey("wave").map((q) => q.command.commandId)).toEqual([
        "a",
        "c",
      ]);
      expect(queue.getByActionKey("nod").map((q) => q.command.commandId)).toEqual([
        "b",
      ]);
      expect(queue.getByActionKey("missing")).toEqual([]);
    });
  });

  describe("getByMutexGroup", () => {
    it("returns matching items", () => {
      const resolveMutex = (actionKey: string) =>
        actionKey === "wave" || actionKey === "nod" ? "head" : null;
      queue = new ActionQueue({
        limits: { total: 16, perActionKey: 16, perMutexGroup: 16 },
        resolveMutexGroup: resolveMutex,
      });
      queue.enqueue(makeCommand({ commandId: "a", actionKey: "wave" }), 100);
      queue.enqueue(makeCommand({ commandId: "b", actionKey: "nod" }), 200);
      queue.enqueue(makeCommand({ commandId: "c", actionKey: "jump" }), 300);

      expect(
        queue.getByMutexGroup("head").map((q) => q.command.commandId),
      ).toEqual(["a", "b"]);
      expect(queue.getByMutexGroup("head").length).toBe(2);
    });
  });

  describe("total limit enforcement", () => {
    it("evicts the weakest item when full", () => {
      queue = new ActionQueue({
        limits: { total: 2, perActionKey: 10, perMutexGroup: 10 },
      });
      queue.enqueue(makeCommand({ commandId: "a", priority: 50 }), 100);
      queue.enqueue(makeCommand({ commandId: "b", priority: 30 }), 200);
      expect(queue.size()).toBe(2);

      const enqueued = queue.enqueue(
        makeCommand({ commandId: "c", priority: 70 }),
        300,
      );
      expect(enqueued).not.toBeNull();
      expect(queue.size()).toBe(2);

      const ids = queue.toArray().map((q) => q.command.commandId);
      expect(ids).toEqual(["c", "a"]);
      expect(
        queue.toArray().find((q) => q.command.commandId === "b"),
      ).toBeUndefined();
    });
  });

  describe("per-actionKey limit enforcement", () => {
    it("evicts oldest same-actionKey item when limit reached", () => {
      queue = new ActionQueue({
        limits: { total: 16, perActionKey: 2, perMutexGroup: 16 },
      });
      queue.enqueue(makeCommand({ commandId: "a", actionKey: "wave" }), 100);
      queue.enqueue(makeCommand({ commandId: "b", actionKey: "wave" }), 200);
      expect(queue.size()).toBe(2);

      queue.enqueue(makeCommand({ commandId: "c", actionKey: "wave" }), 300);
      expect(queue.size()).toBe(2);
      expect(
        queue.getByActionKey("wave").map((q) => q.command.commandId),
      ).toEqual(["b", "c"]);
    });
  });

  describe("per-mutexGroup limit enforcement", () => {
    it("evicts oldest same-mutexGroup item when limit reached", () => {
      const resolveMutex = (actionKey: string) =>
        actionKey === "wave" || actionKey === "nod" ? "head" : null;
      queue = new ActionQueue({
        limits: { total: 16, perActionKey: 16, perMutexGroup: 2 },
        resolveMutexGroup: resolveMutex,
      });
      queue.enqueue(makeCommand({ commandId: "a", actionKey: "wave" }), 100);
      queue.enqueue(makeCommand({ commandId: "b", actionKey: "nod" }), 200);
      expect(queue.size()).toBe(2);

      queue.enqueue(makeCommand({ commandId: "c", actionKey: "wave" }), 300);
      expect(queue.size()).toBe(2);
      expect(
        queue.getByMutexGroup("head").map((q) => q.command.commandId),
      ).toEqual(["b", "c"]);
    });
  });

  describe("expired commands", () => {
    it("does not enqueue expired commands", () => {
      const expired = makeCommand({
        expiresAt: new Date(Date.now() - 10000).toISOString(),
      });
      expect(queue.enqueue(expired, 100)).toBeNull();
      expect(queue.size()).toBe(0);

      const valid = makeCommand({
        expiresAt: new Date(Date.now() + 60000).toISOString(),
      });
      expect(queue.enqueue(valid, 100)).not.toBeNull();
      expect(queue.size()).toBe(1);
    });
  });

  describe("reindex", () => {
    it("reindexes queue positions after operations", () => {
      queue.enqueue(makeCommand({ commandId: "a", actionKey: "act_a" }), 100);
      queue.enqueue(makeCommand({ commandId: "b", actionKey: "act_b" }), 200);
      queue.enqueue(makeCommand({ commandId: "c", actionKey: "act_c" }), 300);

      const initial = queue.toArray();
      expect(initial.map((q) => q.ack.queuePosition)).toEqual([0, 1, 2]);

      queue.remove("b");
      const after = queue.toArray();
      expect(after.map((q) => q.ack.queuePosition)).toEqual([0, 1]);
      expect(after[0].command.commandId).toBe("a");
      expect(after[1].command.commandId).toBe("c");

      const enqueued = queue.enqueue(makeCommand({ commandId: "d", actionKey: "act_d" }), 400);
      expect(enqueued).not.toBeNull();
      expect(enqueued?.ack.queuePosition).toBe(2);
      const finalItems = queue.toArray();
      expect(finalItems.map((q) => q.ack.queuePosition)).toEqual([0, 1, 2]);
      expect(finalItems[2].command.commandId).toBe("d");
    });
  });
});
