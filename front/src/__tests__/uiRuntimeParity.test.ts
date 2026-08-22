import { describe, expect, it } from "vitest";
import { BrowserClientPluginRuntime, BrowserSlotRuntime } from "@/ui-runtime/clientPluginRuntime";
import { buildUnifiedSlotItems } from "@/ui-runtime/slotLedger";
import { clientSlotStoreResource } from "@/ui-runtime/slotContract";
import { providerSlotId } from "@/ui-runtime/providerSlotAdapter";
import {
  ConversationNodeAssembler,
  listProgrammaticConversationNodeDefinitions,
  mergeConversationEvents,
  registerProgrammaticConversationNodeDefinition,
  type RuntimeConversationEvent,
} from "@/ui-runtime/conversationProjection";

function event(id: string, eventType: string, sequence: number, payload: Record<string, unknown>): RuntimeConversationEvent {
  return {
    id,
    eventType,
    conversationId: "conversation-1",
    timestamp: new Date(sequence * 1000).toISOString(),
    sequence,
    payload,
  };
}

describe("DSH parity runtime contracts", () => {
  it("restarts inject effects when only the declaration epoch changes", async () => {
    const slots = new BrowserSlotRuntime();
    let activated = 0;
    let cleaned = 0;
    const snapshot = (epoch: number) => ({
      slots: [{
        slotId: "test.slot",
        contractVersion: 1,
        supportedKinds: ["panel"],
        multiplicity: "ordered_multiple" as const,
        layout: "stack" as const,
        fallbackPolicy: "empty" as const,
        declarationEpoch: epoch,
        contributions: [],
        generatedAt: new Date(0).toISOString(),
      }],
      generatedAt: new Date(0).toISOString(),
      version: 1,
    });

    await slots.syncSnapshot(snapshot(1));
    await slots.inject("test.slot", () => {
      activated += 1;
      return () => { cleaned += 1; };
    });
    await slots.syncSnapshot(snapshot(2));

    expect(activated).toBe(2);
    expect(cleaned).toBe(1);
  });

  it("owns contribution store and business injection for the contribution lifetime", async () => {
    const runtime = new BrowserClientPluginRuntime();
    await runtime.slots.declare({ slotId: "typed.store.slot", contractVersion: 1, supportedKinds: ["panel"] });
    let disposed = 0;

    runtime.define({
      id: "store-plugin",
      setup(ctx) {
        ctx.services.provide("demo.greeting", "hello");
        ctx.slots.register("typed.store.slot", "view", {} as any, {
          store: () => clientSlotStoreResource({ count: 7 }, () => { disposed += 1; }),
          inject: ({ store, services }) => ({
            count: (store as { count: number }).count,
            greeting: services.get<string>("demo.greeting"),
          }),
        });
      },
    });

    await runtime.run("store-plugin");
    const contribution = runtime.slots.listContributions("typed.store.slot")[0];
    expect(contribution?.store).toEqual({ count: 7 });
    expect(contribution?.injected).toEqual({ count: 7, greeting: "hello" });

    await runtime.stop("store-plugin");
    expect(runtime.slots.listContributions("typed.store.slot")).toHaveLength(0);
    expect(disposed).toBe(1);
  });

  it("keeps lifecycle observation separate from business injection", async () => {
    const slots = new BrowserSlotRuntime();
    let observed = 0;
    await slots.observe("late.slot", () => { observed += 1; });
    await slots.declare({ slotId: "late.slot", contractVersion: 1, supportedKinds: ["panel"] });
    expect(observed).toBe(1);
  });

  it("maps every provider capability onto the unified provider slot namespace", () => {
    expect(providerSlotId("conversation.composer")).toBe("provider.conversation.composer");
    expect(providerSlotId("app.shell")).toBe("provider.app.shell");
  });

  it("keeps the current atomic package alive when a candidate version fails", async () => {
    const runtime = new BrowserClientPluginRuntime();
    let currentAlive = 0;
    runtime.definePackage({
      id: "demo",
      version: "1",
      atomicUpdate: true,
      setup: () => {
        currentAlive += 1;
        return () => { currentAlive -= 1; };
      },
    });
    await runtime.runPackage("demo", "1");
    runtime.definePackage({
      id: "demo",
      version: "2",
      atomicUpdate: true,
      setup: () => { throw new Error("candidate failed"); },
    });

    await expect(runtime.runPackage("demo", "2")).rejects.toThrow("candidate failed");
    expect(currentAlive).toBe(1);
    expect(runtime.inspect().packages.find((item) => item.id === "demo")?.activeVersion).toBe("1");
  });

  it("enforces stable context identity and exposes previous/location data", () => {
    let previousContext = "";
    let locationValue: unknown;
    const disposeProducer = registerProgrammaticConversationNodeDefinition({
      id: "producer",
      contributionId: "producer-view",
      extensionId: "test",
      match: (item) => item.eventType.startsWith("producer.")
        ? { contextId: String(item.payload.id), phase: item.eventType.endsWith("start") ? "start" : "end" }
        : null,
      create: () => ({ value: 1 }),
      update: (state) => state,
      project: (context) => ({ payload: { value: context.state.value } }),
      publication: () => ({ visibility: "conversation", target: "artifact" }),
      buildLocationData: (context) => ({ value: context.state.value }),
    });
    const disposeConsumer = registerProgrammaticConversationNodeDefinition({
      id: "consumer",
      contributionId: "consumer-view",
      extensionId: "test",
      match: (item) => item.eventType.startsWith("consumer.")
        ? { contextId: String(item.payload.id), phase: item.eventType.endsWith("start") ? "start" : "end" }
        : null,
      create: (item, _match, reader) => {
        if (item.payload.id === "consumer-2") {
          previousContext = reader?.previous()?.contextId ?? "";
          locationValue = reader?.location("artifact")?.data.value;
        }
        return { value: String(item.payload.id) };
      },
      update: (state) => state,
      project: (context) => ({ payload: { value: context.state.value } }),
    });

    try {
      const assembler = new ConversationNodeAssembler([]);
      assembler.setProgrammaticDefinitions(listProgrammaticConversationNodeDefinitions());
      [
        event("1", "producer.start", 1, { id: "producer-1" }),
        event("2", "producer.end", 2, { id: "producer-1" }),
        event("3", "producer.start", 3, { id: "producer-1" }), // invalid duplicate stable id
        event("4", "producer.start", 4, { id: "producer-2" }),
        event("5", "producer.end", 5, { id: "producer-2" }),
        event("6", "consumer.start", 6, { id: "consumer-1" }),
        event("7", "consumer.end", 7, { id: "consumer-1" }),
        event("8", "consumer.start", 8, { id: "consumer-2" }),
      ].forEach((item) => assembler.append(item));

      expect(assembler.diagnostics()).toEqual(expect.arrayContaining([
        expect.objectContaining({ code: "duplicate_start", contextId: "producer-1" }),
      ]));
      expect(previousContext).toBe("consumer-1");
      expect(locationValue).toBe(1);
    } finally {
      disposeConsumer();
      disposeProducer();
    }
  });
  it("arbitrates server and client contributions in one replaceable slot ledger", () => {
    const server = [{
      contributionId: "server-view",
      extensionId: "server",
      moduleId: "server",
      kind: "panel",
      slotId: "chat.message.renderer",
      contractVersion: 1,
      generation: 1,
      title: "Server",
      ordering: 0,
      priority: 10,
      visible: true,
      effective: true,
      enabled: true,
      runtimeReady: true,
    }];
    const client = [{
      contributionId: "client:test:chat.message.renderer:view",
      pluginId: "test",
      key: "view",
      slotId: "chat.message.renderer",
      component: {} as any,
      ordering: 0,
      priority: 20,
    }];
    const resolved = buildUnifiedSlotItems({ multiplicity: "replaceable_single" }, server, client);
    expect(resolved).toHaveLength(1);
    expect(resolved[0]?.source).toBe("client");
  });

  it("rolls back iterable slot injection setup in reverse order", async () => {
    const slots = new BrowserSlotRuntime();
    await slots.declare({ slotId: "transaction.slot", contractVersion: 1, supportedKinds: ["panel"] });
    const cleanup: string[] = [];
    await expect(slots.inject("transaction.slot", () => (function* () {
      yield () => { cleanup.push("first"); };
      yield () => { cleanup.push("second"); };
      throw new Error("setup failed");
    })())).rejects.toThrow("setup failed");
    expect(cleanup).toEqual(["second", "first"]);
  });

  it("prepends an older durable page without duplicating the current event window", () => {
    const assembler = new ConversationNodeAssembler([]);
    assembler.replaceEvents([
      event("3", "history", 3, {}),
      event("4", "history", 4, {}),
    ]);
    assembler.prepend([
      event("1", "history", 1, {}),
      event("2", "history", 2, {}),
      event("3", "history", 3, {}),
    ]);
    expect(assembler.listEvents().map((item) => item.sequence)).toEqual([1, 2, 3, 4]);
    expect(assembler.window()).toEqual({ firstSequence: 1, lastSequence: 4, size: 4 });
  });

  it("keeps canonical durable evidence when local journal/live copies share an event identity", () => {
    const local = event("message-1", "message_created", 999, { messageId: "m1", content: "local" });
    local.source = "journal";
    const durable = event("durable-1", "message_created", 42, { messageId: "m1", content: "canonical" });
    durable.source = "durable";
    const merged = mergeConversationEvents([durable], [local]);
    expect(merged).toHaveLength(1);
    expect(merged[0]?.sequence).toBe(42);
    expect(merged[0]?.payload.content).toBe("canonical");
  });

  it("binds child slot declaration and authorization to the parent client contribution", async () => {
    const slots = new BrowserSlotRuntime();
    await slots.declare({ slotId: "parent.slot", contractVersion: 1, supportedKinds: ["panel"] });
    let childActivations = 0;
    let childCleanups = 0;
    await slots.inject("parent.child", () => {
      childActivations += 1;
      return () => { childCleanups += 1; };
    });
    const dispose = slots.register("plugin", "parent.slot", "parent-view", {} as any, {
      children: [{ slotId: "parent.child", contractVersion: 1, supportedKinds: ["action"] }],
    });
    await Promise.resolve();
    expect(slots.getDefinition("parent.child")?.parentSlotId).toBe("parent.slot");
    expect(childActivations).toBe(1);
    await dispose();
    expect(slots.getDefinition("parent.child")).toBeUndefined();
    expect(childCleanups).toBe(1);
  });

  it("accepts buildViewNode as the DSH-compatible projection callback", () => {
    const dispose = registerProgrammaticConversationNodeDefinition({
      id: "build-view",
      contributionId: "build-view-contribution",
      extensionId: "test",
      match: (item) => item.eventType.startsWith("build.")
        ? { contextId: "job-1", phase: item.eventType.endsWith("start") ? "start" : "end" }
        : null,
      create: () => ({ value: 1 }),
      update: (state) => state,
      buildViewNode: (context) => ({ key: context.contextId, target: "turn:1", payload: { value: context.state.value } }),
    });
    try {
      const assembler = new ConversationNodeAssembler([]);
      assembler.setProgrammaticDefinitions(listProgrammaticConversationNodeDefinitions());
      assembler.append(event("build-1", "build.start", 1, {}));
      const node = assembler.nodes().find((item) => item.contributionId === "build-view-contribution");
      expect(node?.viewKey).toBe("job-1");
      expect(node?.target).toBe("turn:1");
    } finally {
      dispose();
    }
  });

  it("rehydrates the same server revision after a conversation scope was intentionally unloaded", async () => {
    const runtime = new BrowserClientPluginRuntime();
    const state = (conversationId: string, revision: number, running: boolean, version = "1@beta") => ({
      conversationId,
      revision,
      packages: conversationId === "conversation-a" ? [{
        id: "demo",
        versions: [{ id: "demo", version, contributions: [] }],
        activeVersion: version,
        running,
      }] : [],
    });

    await runtime.activateConversationScope("conversation-a");
    expect(await runtime.synchronizeSession(state("conversation-a", 1, true))).toBe(true);
    const scopedId = runtime.scopedPackageId("conversation-a", "demo");
    expect(runtime.inspect().packages.find((item) => item.id === scopedId)?.running).toBe(true);

    await runtime.activateConversationScope("conversation-b");
    expect(await runtime.synchronizeSession(state("conversation-b", 1, false))).toBe(true);
    expect(runtime.getSessionRevision("conversation-a")).toBeUndefined();
    expect(runtime.inspect().packages.find((item) => item.id === scopedId)).toBeUndefined();

    await runtime.activateConversationScope("conversation-a");
    expect(await runtime.synchronizeSession(state("conversation-a", 1, true))).toBe(true);
    expect(runtime.getSessionRevision("conversation-a")).toBe(1);
    expect(runtime.inspect().packages.find((item) => item.id === scopedId)?.running).toBe(true);
  });

  it("rejects stale session snapshots without rolling a live package backwards", async () => {
    const runtime = new BrowserClientPluginRuntime();
    const runningState = {
      conversationId: "conversation-a",
      revision: 2,
      packages: [{
        id: "demo",
        versions: [{ id: "demo", version: "2", contributions: [] }],
        activeVersion: "2",
        running: true,
      }],
    };
    const staleState = {
      conversationId: "conversation-a",
      revision: 1,
      packages: [{
        id: "demo",
        versions: [{ id: "demo", version: "1", contributions: [] }],
        activeVersion: "1",
        running: true,
      }],
    };

    await runtime.activateConversationScope("conversation-a");
    expect(await runtime.synchronizeSession(runningState)).toBe(true);
    expect(await runtime.synchronizeSession(staleState)).toBe(false);
    expect(runtime.getSessionRevision("conversation-a")).toBe(2);
    const scopedId = runtime.scopedPackageId("conversation-a", "demo");
    const pkg = runtime.inspect().packages.find((item) => item.id === scopedId);
    expect(pkg?.activeVersion).toBe("2");
    expect(pkg?.versions).toEqual(["2"]);
  });

  it("does not let an in-flight old conversation hydrate resurrect packages after a scope switch", async () => {
    const runtime = new BrowserClientPluginRuntime();
    const originalRunPackage = runtime.runPackage.bind(runtime);
    let release!: () => void;
    const gate = new Promise<void>((resolve) => { release = resolve; });
    let entered = false;
    (runtime as unknown as { runPackage: (id: string, version?: string) => Promise<void> }).runPackage = async (id, version) => {
      if (id.includes("conversation-a")) {
        entered = true;
        await gate;
      }
      return originalRunPackage(id, version);
    };

    await runtime.activateConversationScope("conversation-a");
    const syncA = runtime.synchronizeSession({
      conversationId: "conversation-a",
      revision: 1,
      packages: [{
        id: "demo",
        versions: [{ id: "demo", version: "1", contributions: [] }],
        activeVersion: "1",
        running: true,
      }],
    });
    for (let index = 0; index < 20 && !entered; index++) await Promise.resolve();
    expect(entered).toBe(true);

    await runtime.activateConversationScope("conversation-b");
    release();
    expect(await syncA).toBe(false);
    expect(runtime.isActiveConversationScope("conversation-b")).toBe(true);
    expect(runtime.getSessionRevision("conversation-a")).toBeUndefined();
    const scopedA = runtime.scopedPackageId("conversation-a", "demo");
    expect(runtime.inspect().packages.find((item) => item.id === scopedA)).toBeUndefined();
  });

  it("rejects a late inactive-session snapshot instead of stealing the active scope", async () => {
    const runtime = new BrowserClientPluginRuntime();
    await runtime.activateConversationScope("conversation-a");
    await runtime.activateConversationScope("conversation-b");

    const applied = await runtime.synchronizeSession({
      conversationId: "conversation-a",
      revision: 9,
      packages: [{
        id: "late",
        versions: [{ id: "late", version: "1", contributions: [] }],
        activeVersion: "1",
        running: true,
      }],
    });

    expect(applied).toBe(false);
    expect(runtime.isActiveConversationScope("conversation-b")).toBe(true);
    expect(runtime.getSessionRevision("conversation-a")).toBeUndefined();
    expect(runtime.inspect().packages.some((item) => item.id.includes("conversation-a"))).toBe(false);
  });

  it("cancels an in-flight package setup when undefine supersedes its lifecycle", async () => {
    const runtime = new BrowserClientPluginRuntime();
    let release!: () => void;
    const gate = new Promise<void>((resolve) => { release = resolve; });
    let entered = false;
    let cleanupCount = 0;

    runtime.definePackage({
      id: "slow-package",
      version: "1",
      setup: async () => {
        entered = true;
        await gate;
        return () => { cleanupCount += 1; };
      },
    });

    const run = runtime.runPackage("slow-package", "1");
    for (let index = 0; index < 20 && !entered; index++) await Promise.resolve();
    expect(entered).toBe(true);

    await runtime.undefinePackage("slow-package", "1");
    release();
    await expect(run).rejects.toThrow(/superseded/);

    expect(runtime.inspect().packages.find((item) => item.id === "slow-package")).toBeUndefined();
    expect(runtime.inspect().plugins.find((item) => item.id === "slow-package")).toBeUndefined();
    expect(runtime.inspect().fibers.find((item) => item.pluginId === "slow-package")).toBeUndefined();
    expect(cleanupCount).toBe(1);
  });

  it("restarts the current package when the server opens a new runId for the same immutable version", async () => {
    const runtime = new BrowserClientPluginRuntime();
    const base = {
      conversationId: "conversation-restart",
      revision: 1,
      packages: [{
        id: "demo",
        versions: [{ id: "demo", version: "1", contributions: [] }],
        activeVersion: "1",
        running: true,
      }],
    };
    await runtime.activateConversationScope("conversation-restart");
    expect(await runtime.synchronizeSession(base)).toBe(true);

    const originalRestart = runtime.restartPackage.bind(runtime);
    let restartCount = 0;
    (runtime as unknown as { restartPackage: (id: string, version?: string) => Promise<void> }).restartPackage = async (id, version) => {
      restartCount += 1;
      await originalRestart(id, version);
    };

    expect(await runtime.synchronizeSession({
      ...base,
      revision: 2,
      packages: [{
        ...base.packages[0],
        targetVersion: "1",
        transitionState: "starting",
        transitionMode: "run",
        runId: "run-2",
        pluginRunId: "run-2",
      }],
    })).toBe(true);
    expect(restartCount).toBe(1);
    const scopedId = runtime.scopedPackageId("conversation-restart", "demo");
    const pkg = runtime.inspect().packages.find((item) => item.id === scopedId);
    expect(pkg?.activeVersion).toBe("1");
    expect(pkg?.running).toBe(true);
  });

  it("scopes programmatic locations by conversation turn and hides location-only nodes", () => {
    let implicitValue: unknown;
    let explicitValue: unknown;
    const disposeProducer = registerProgrammaticConversationNodeDefinition({
      id: "turn-producer",
      contributionId: "turn-producer-view",
      extensionId: "test",
      match: (item) => item.eventType.startsWith("turn-producer.")
        ? { contextId: String(item.payload.id), phase: item.eventType.endsWith("start") ? "start" : "end" }
        : null,
      create: () => ({ value: 7 }),
      update: (state) => state,
      buildViewNode: () => ({ visibility: "hidden", payload: {} }),
      publication: () => ({ visibility: "turn", target: "artifact" }),
      buildLocationData: (context) => ({ value: context.state.value }),
    });
    const disposeConsumer = registerProgrammaticConversationNodeDefinition({
      id: "turn-consumer",
      contributionId: "turn-consumer-view",
      extensionId: "test",
      match: (item) => item.eventType === "turn-consumer.start"
        ? { contextId: String(item.payload.id), phase: "start" }
        : null,
      create: (_item, _match, reader) => {
        implicitValue = reader?.location("artifact", "turn")?.data.value;
        explicitValue = reader?.location("artifact", "turn", "turn-1")?.data.value;
        return {};
      },
      update: (state) => state,
      buildViewNode: () => ({ payload: {} }),
    });
    try {
      const assembler = new ConversationNodeAssembler([]);
      assembler.setProgrammaticDefinitions(listProgrammaticConversationNodeDefinitions());
      assembler.append(event("tp1", "turn-producer.start", 1, { id: "producer", turnId: "turn-1" }));
      assembler.append(event("tp2", "turn-producer.end", 2, { id: "producer", turnId: "turn-1" }));
      assembler.append(event("tc1", "turn-consumer.start", 3, { id: "consumer", turnId: "turn-2" }));
      expect(implicitValue).toBeUndefined();
      expect(explicitValue).toBe(7);
      expect(assembler.nodes().some((item) => item.contributionId === "turn-producer-view")).toBe(false);
    } finally {
      disposeConsumer();
      disposeProducer();
    }
  });

});
