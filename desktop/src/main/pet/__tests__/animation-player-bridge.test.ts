import { describe, expect, it, vi } from "vitest";
import { AnimationPlayerBridge } from "../animation-player-bridge";
import type { AnimationIpcAdapter, RendererDeliveryResult } from "../animation-ipc";
import type { LoadedInstallation, RuntimeAction } from "../resource-loader";
import type { PlayActionCommand } from "../../../desktop-pet/animation/contracts";

function makeAction(
  key: string,
  overrides: Partial<RuntimeAction> = {},
): RuntimeAction {
  return {
    key,
    name: key,
    version: "2",
    loopType: "loop",
    fps: 20,
    frameDurationMs: 50,
    frameCount: 1,
    frames: [`${key}/frame-0.png`],
    interruptible: true,
    available: true,
    ...overrides,
  };
}

function makeLoaded(actions: RuntimeAction[], defaultKey: string): LoadedInstallation {
  const map = new Map(actions.map((action) => [action.key, action]));
  const defaultAction = map.get(defaultKey) ?? null;
  return {
    installationId: "install-test",
    manifest: {
      packageId: "package-test",
      schemaVersion: 2,
      name: "Test Pet",
      characterId: "character-test",
      canvas: { width: 64, height: 64 },
      defaultAction: defaultKey,
      actions: [],
    },
    actions: map,
    defaultAction,
    installPath: "/tmp/install-test",
    manifestPath: "/tmp/install-test/manifest.json",
    previewPath: null,
  };
}

function makeIpc(result: RendererDeliveryResult = { status: "delivered" }) {
  const sent: PlayActionCommand[] = [];
  const adapter = {
    sendPlayAction(command: PlayActionCommand): RendererDeliveryResult {
      sent.push(command);
      return result;
    },
    sendPause: vi.fn(),
    sendResume: vi.fn(),
    sendStop: vi.fn(),
  } as unknown as AnimationIpcAdapter;
  return { adapter, sent };
}

describe("AnimationPlayerBridge", () => {
  it("routes play through renderer IPC and commits state only after renderer start", () => {
    const idle = makeAction("idle");
    const loaded = makeLoaded([idle], "idle");
    const { adapter, sent } = makeIpc();
    const bridge = new AnimationPlayerBridge();

    bridge.attachLoaded(loaded);
    bridge.setAnimationIpc(adapter);
    bridge.setInstallationContext("install-test", "pet-test", 11);
    bridge.play(idle);

    expect(bridge.getState()).toBe("loading");
    expect(sent).toHaveLength(1);
    expect(sent[0]).toMatchObject({
      installationId: "install-test",
      petInstanceId: "pet-test",
      packageRevision: 11,
      actionKey: "idle",
      queuePolicy: "replace_current",
      interruptPolicy: "respect_action",
    });

    bridge.handlePlaybackEvent({
      type: "playback.action_started",
      actionKey: "idle",
      playbackInstanceId: sent[0].playbackInstanceId,
      commandId: sent[0].commandId,
      timestamp: Date.now(),
    });

    expect(bridge.getState()).toBe("playing");
    expect(bridge.getCurrentAction()?.key).toBe("idle");
    expect(bridge.getCurrentPlaybackId()).toBe(sent[0].playbackInstanceId);
  });

  it("fails closed when renderer delivery rejects a command", () => {
    const wave = makeAction("wave", { loopType: "once" });
    const loaded = makeLoaded([wave], "wave");
    const onError = vi.fn();
    const { adapter } = makeIpc({
      status: "rejected",
      reason: "renderer_not_ready",
      error: "renderer unavailable",
    });
    const bridge = new AnimationPlayerBridge({ onError });

    bridge.attachLoaded(loaded);
    bridge.setAnimationIpc(adapter);
    bridge.setInstallationContext("install-test", "pet-test", 3);
    bridge.play(wave);

    expect(bridge.getState()).toBe("stopped");
    expect(onError).toHaveBeenCalledTimes(1);
    expect(String(onError.mock.calls[0]?.[0])).toContain("DELIVERY_FAILED");
  });

  it("uses return action, sustained action, default action, then remaining actions as fallback order", () => {
    const source = makeAction("source", { returnAction: "return" });
    const returnAction = makeAction("return");
    const sustained = makeAction("sustained");
    const idle = makeAction("idle");
    const other = makeAction("other");
    const loaded = makeLoaded([source, returnAction, sustained, idle, other], "idle");
    const bridge = new AnimationPlayerBridge();

    bridge.attachLoaded(loaded);
    bridge.setSustainedActionMap({ source: "sustained" });

    expect(bridge.getFallbackChain(source).map((action) => action.key)).toEqual([
      "return",
      "sustained",
      "idle",
      "other",
    ]);
  });

  it("tracks frame snapshots and completes once actions", () => {
    const wave = makeAction("wave", { loopType: "once" });
    const loaded = makeLoaded([wave], "wave");
    const onCompleted = vi.fn();
    const bridge = new AnimationPlayerBridge({ onActionCompleted: onCompleted });

    bridge.attachLoaded(loaded);
    bridge.handlePlaybackEvent({
      type: "playback.action_started",
      actionKey: "wave",
      playbackInstanceId: "playback-wave",
      timestamp: Date.now(),
    });
    bridge.handleSnapshotUpdate({
      phase: "playing",
      packageId: "package-test",
      packageRevision: 1,
      currentActionKey: "wave",
      currentCommandId: "command-wave",
      frameIndex: 4,
      localElapsedMs: 120,
      cycleIndex: 0,
      playbackRate: 1,
      queueLength: 0,
      previousStableActionKey: null,
      defaultActionKey: "wave",
      lastTransitionAtMonotonicMs: 120,
    });
    bridge.handlePlaybackEvent({
      type: "playback.action_completed",
      actionKey: "wave",
      timestamp: Date.now(),
    });

    expect(bridge.getCurrentFrameIndex()).toBe(4);
    expect(bridge.getLoopCount()).toBe(1);
    expect(bridge.getState()).toBe("stopped");
    expect(onCompleted).toHaveBeenCalledWith("wave", 1, "playback-wave");
  });
  it("preserves package return/timing semantics when Runtime does not override them", () => {
    const action = makeAction("contract", {
      returnTo: { type: "previous" },
      minimumPlayMs: 0,
      interruptAfterMs: 0,
      maximumPlayMs: null,
    });
    const loaded = makeLoaded([action], action.key);
    const { adapter, sent } = makeIpc();
    const bridge = new AnimationPlayerBridge();

    bridge.attachLoaded(loaded);
    bridge.setAnimationIpc(adapter);
    bridge.setInstallationContext("install-test", "pet-test", 7);
    bridge.play(action, { priority: 61, queuePolicy: "enqueue" });

    expect(sent).toHaveLength(1);
    expect(sent[0]).toMatchObject({
      actionKey: action.key,
      priority: 61,
      queuePolicy: "enqueue",
      returnOverride: { type: "previous" },
      minimumPlayMs: 0,
      interruptAfterMs: 0,
    });
    expect(sent[0].maximumPlayMs).toBeUndefined();
  });

  it("explicit Runtime return override wins over package returnTo", () => {
    const action = makeAction("contract", { returnTo: { type: "previous" } });
    const loaded = makeLoaded([action], action.key);
    const { adapter, sent } = makeIpc();
    const bridge = new AnimationPlayerBridge();

    bridge.attachLoaded(loaded);
    bridge.setAnimationIpc(adapter);
    bridge.setInstallationContext("install-test", "pet-test", 7);
    bridge.play(action, { returnOverride: { type: "none" } });

    expect(sent[0].returnOverride).toEqual({ type: "none" });
  });

  it("does not duplicate Runtime failure callbacks for renderer lifecycle failures", () => {
    const action = makeAction("active");
    const loaded = makeLoaded([action], action.key);
    const onFailed = vi.fn();
    const onError = vi.fn();
    const { adapter, sent } = makeIpc();
    const bridge = new AnimationPlayerBridge({
      onActionFailed: onFailed,
      onError,
    });

    bridge.attachLoaded(loaded);
    bridge.setAnimationIpc(adapter);
    bridge.setInstallationContext("install-test", "pet-test", 7);
    bridge.play(action);
    bridge.handlePlaybackEvent({
      type: "playback.action_started",
      actionKey: action.key,
      playbackInstanceId: sent[0].playbackInstanceId,
      commandId: sent[0].commandId,
      timestamp: Date.now(),
    });

    bridge.handlePlaybackEvent({
      type: "playback.action_failed",
      actionKey: action.key,
      playbackInstanceId: sent[0].playbackInstanceId,
      commandId: sent[0].commandId,
      reason: "renderer_delivery_failed",
      error: { code: "renderer_delivery_failed", message: "delivery failed" },
      timestamp: Date.now(),
    });

    expect(onFailed).not.toHaveBeenCalled();
    expect(onError).toHaveBeenCalledTimes(1);
    expect(bridge.getCurrentPlaybackId()).toBeNull();
    expect(bridge.getState()).toBe("stopped");
  });

  it("terminalizes stop delivery failure with the active playback identity", () => {
    const action = makeAction("active");
    const loaded = makeLoaded([action], action.key);
    const onFailed = vi.fn();
    const { adapter, sent } = makeIpc();
    (adapter as unknown as { sendStop: ReturnType<typeof vi.fn> }).sendStop = vi.fn(() => ({
      status: "rejected",
      reason: "renderer_not_ready",
    }));
    const bridge = new AnimationPlayerBridge({ onActionFailed: onFailed });

    bridge.attachLoaded(loaded);
    bridge.setAnimationIpc(adapter);
    bridge.setInstallationContext("install-test", "pet-test", 7);
    bridge.play(action);
    bridge.handlePlaybackEvent({
      type: "playback.action_started",
      actionKey: action.key,
      playbackInstanceId: sent[0].playbackInstanceId,
      commandId: sent[0].commandId,
      timestamp: Date.now(),
    });

    bridge.stop("runtime_stop");

    expect(onFailed).toHaveBeenCalledWith(
      action.key,
      "STOP_DELIVERY_FAILED:renderer_not_ready",
      sent[0].playbackInstanceId,
    );
    expect(bridge.getCurrentPlaybackId()).toBeNull();
  });

});
