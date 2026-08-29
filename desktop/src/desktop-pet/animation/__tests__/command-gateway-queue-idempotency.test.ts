import { describe, expect, it } from "vitest";
import { ActionQueue } from "../action-queue";
import { CommandGateway } from "../command-gateway";
import { InterruptArbiter } from "../interrupt-arbiter";
import type { LoadedAction, PlayActionCommand } from "../contracts";

function currentAction(): LoadedAction {
  return {
    packageId: "pkg",
    packageRevision: 1,
    actionKey: "busy",
    displayName: "Busy",
    actionVersion: 1,
    loopType: "loop",
    frames: [],
    baseDurationMs: 100,
    cycleDurationMs: 100,
    anchor: { type: "bottom_center", x: 0, y: 0 },
    interruptible: false,
    interruptAfterMs: 0,
    minimumPlayMs: 0,
    maximumPlayMs: null,
    defaultPriority: 50,
    cooldownMs: 0,
    mutexGroup: null,
    returnTarget: { type: "none" },
    supportsDefaultIdle: false,
    isStableStateCandidate: false,
    isTransitionOnly: false,
    warnings: [],
  };
}

function command(): PlayActionCommand {
  return {
    commandId: "cmd-queued",
    playbackInstanceId: "pbi-queued",
    idempotencyKey: "idem-queued",
    installationId: "inst",
    petInstanceId: "pet",
    packageRevision: 1,
    actionKey: "wave",
    priority: 80,
    queuePolicy: "replace_current",
    interruptPolicy: "respect_action",
    playbackRate: 1,
    issuedAt: new Date().toISOString(),
  };
}

describe("CommandGateway queued idempotency", () => {
  it("promotes the original queued command instead of rejecting it as its own duplicate", () => {
    const queue = new ActionQueue();
    let active: LoadedAction | null = currentAction();
    const gateway = new CommandGateway({
      queue,
      arbiter: new InterruptArbiter(),
      getPackageRevision: () => 1,
      getCurrentAction: () => active,
      getCurrentLocalElapsedMs: () => 1000,
      isAtomicPackageCommit: () => false,
      hasAction: () => true,
    });
    const cmd = command();

    const first = gateway.processCommand(cmd, 10);
    expect(first.decision).toBe("queue");
    expect(first.ack.status).toBe("queued");
    expect(queue.dequeue()?.command.commandId).toBe(cmd.commandId);

    active = null;
    const promoted = gateway.promoteQueuedCommand(cmd, 20);
    expect(promoted.decision).toBe("accept_and_load");
    expect(promoted.ack.status).toBe("accepted");
    expect(promoted.ack.playbackInstanceId).toBe("pbi-queued");

    const externalReplay = gateway.processCommand(cmd, 30);
    expect(externalReplay.decision).toBe("duplicate");
    expect(externalReplay.ack.status).toBe("accepted");
  });
});
