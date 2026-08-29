import type {
  PlayActionCommand,
  CommandAck,
  LoadedAction,
} from "./contracts";
import { IDEMPOTENCY_MAX_ENTRIES, IDEMPOTENCY_TTL_MS } from "./contracts";
import { ActionQueue } from "./action-queue";
import { InterruptArbiter } from "./interrupt-arbiter";

export type CommandDecision =
  | "accept_and_load"
  | "queue"
  | "reject"
  | "satisfied"
  | "expired"
  | "duplicate";

export interface ProcessCommandResult {
  ack: CommandAck;
  decision: CommandDecision;
}

export interface GatewayDeps {
  queue: ActionQueue;
  arbiter: InterruptArbiter;
  getPackageRevision: () => number;
  getCurrentAction: () => LoadedAction | null;
  getCurrentLocalElapsedMs: () => number;
  isAtomicPackageCommit: () => boolean;
  hasAction?: (actionKey: string) => boolean;
}

interface IdempotencyEntry {
  ack: CommandAck;
  decision: CommandDecision;
  recordedAtMs: number;
}

export class CommandGateway {
  private readonly queue: ActionQueue;
  private readonly arbiter: InterruptArbiter;
  private readonly getPackageRevision: () => number;
  private readonly getCurrentAction: () => LoadedAction | null;
  private readonly getCurrentLocalElapsedMs: () => number;
  private readonly isAtomicPackageCommit: () => boolean;
  private readonly hasAction: (actionKey: string) => boolean;
  private readonly idempotency: Map<string, IdempotencyEntry> = new Map();

  constructor(deps: GatewayDeps) {
    this.queue = deps.queue;
    this.arbiter = deps.arbiter;
    this.getPackageRevision = deps.getPackageRevision;
    this.getCurrentAction = deps.getCurrentAction;
    this.getCurrentLocalElapsedMs = deps.getCurrentLocalElapsedMs;
    this.isAtomicPackageCommit = deps.isAtomicPackageCommit;
    this.hasAction = deps.hasAction ?? (() => true);
  }

  processCommand(
    command: PlayActionCommand,
    monotonicMs: number,
  ): ProcessCommandResult {
    this.clearExpiredIdempotency(Date.now());

    if (!this.isValidCommand(command)) {
      return this.buildReject(command, "invalid_command_fields");
    }

    const actualRevision = this.getPackageRevision();
    if (command.packageRevision !== actualRevision) {
      return this.buildReject(command, `package_revision_mismatch:${actualRevision}`);
    }

    const existing = this.getIdempotencyResult(command.idempotencyKey);
    if (existing) {
      return { ack: existing, decision: "duplicate" };
    }

    if (this.isCommandExpired(command, new Date())) {
      const ack = this.buildAck(command, "expired", "command_expired", command.playbackInstanceId, 0);
      this.rememberIdempotency(command.idempotencyKey, ack, monotonicMs);
      return { ack, decision: "expired" };
    }

    if (!this.hasAction(command.actionKey)) {
      return this.buildReject(command, "action_not_found");
    }

    const currentAction = this.getCurrentAction();
    if (currentAction && this.isSameLoopAction(currentAction, command)) {
      const ack = this.buildAck(command, "satisfied", "already_playing_loop", command.playbackInstanceId, 0);
      this.rememberIdempotency(command.idempotencyKey, ack, monotonicMs);
      return { ack, decision: "satisfied" };
    }

    return this.applyQueuePolicy(command, currentAction, monotonicMs);
  }

  /** Promote an already-queued command without passing through the external
   * duplicate guard again. The queued idempotency entry is transitioned to
   * accepted in-place, preserving the original command/playback identity. */
  promoteQueuedCommand(command: PlayActionCommand, monotonicMs: number): ProcessCommandResult {
    this.clearExpiredIdempotency(Date.now());
    if (!this.isValidCommand(command)) return this.buildReject(command, "invalid_command_fields");
    const actualRevision = this.getPackageRevision();
    if (command.packageRevision !== actualRevision) {
      const ack = this.buildAck(command, "stale", `package_revision_mismatch:${actualRevision}`, command.playbackInstanceId, 0);
      this.rememberIdempotency(command.idempotencyKey, ack, monotonicMs);
      return { ack, decision: "reject" };
    }
    if (this.isCommandExpired(command, new Date())) {
      const ack = this.buildAck(command, "expired", "command_expired", command.playbackInstanceId, 0);
      this.rememberIdempotency(command.idempotencyKey, ack, monotonicMs);
      return { ack, decision: "expired" };
    }
    if (!this.hasAction(command.actionKey)) {
      const ack = this.buildAck(command, "rejected", "action_not_found", command.playbackInstanceId, 0);
      this.rememberIdempotency(command.idempotencyKey, ack, monotonicMs);
      return { ack, decision: "reject" };
    }
    const existing = this.idempotency.get(command.idempotencyKey);
    if (!existing || existing.ack.status !== "queued" || existing.ack.commandId !== command.commandId) {
      const ack = this.buildAck(command, "rejected", "queued_state_lost", command.playbackInstanceId, 0);
      this.rememberIdempotency(command.idempotencyKey, ack, monotonicMs);
      return { ack, decision: "reject" };
    }
    return this.accept(command, monotonicMs);
  }

  getIdempotencyResult(idempotencyKey: string): CommandAck | null {
    const entry = this.idempotency.get(idempotencyKey);
    if (!entry) {
      return null;
    }
    if (Date.now() - entry.recordedAtMs > IDEMPOTENCY_TTL_MS) {
      this.idempotency.delete(idempotencyKey);
      return null;
    }
    return entry.ack;
  }

  rememberIdempotency(
    idempotencyKey: string,
    ack: CommandAck,
    monotonicMs: number,
  ): void {
    void monotonicMs;
    if (this.idempotency.size >= IDEMPOTENCY_MAX_ENTRIES) {
      this.evictOldestIdempotency();
    }
    this.idempotency.set(idempotencyKey, {
      ack,
      decision: this.ackStatusToDecision(ack.status),
      recordedAtMs: Date.now(),
    });
  }

  clearExpiredIdempotency(now: number): void {
    for (const [key, entry] of this.idempotency) {
      if (now - entry.recordedAtMs > IDEMPOTENCY_TTL_MS) {
        this.idempotency.delete(key);
      }
    }
  }

  isCommandExpired(command: PlayActionCommand, now: Date): boolean {
    if (!command.expiresAt) {
      return command.requiresAuthoritativeExpiry === true;
    }
    const expiresAtMs = Date.parse(command.expiresAt);
    if (Number.isNaN(expiresAtMs)) {
      return true;
    }
    return now.getTime() >= expiresAtMs;
  }

  private applyQueuePolicy(
    command: PlayActionCommand,
    currentAction: LoadedAction | null,
    monotonicMs: number,
  ): ProcessCommandResult {
    const policy = command.queuePolicy;

    if (policy === "replace_current") {
      if (!currentAction) {
        return this.accept(command, monotonicMs);
      }
      const interruptResult = this.arbiter.canInterrupt({
        currentAction,
        currentLocalElapsedMs: this.getCurrentLocalElapsedMs(),
        newCommand: command,
        isAtomicPackageCommit: this.isAtomicPackageCommit(),
      });
      if (interruptResult.canInterrupt) {
        return this.accept(command, monotonicMs);
      }
      const rejectResult = this.arbiter.shouldRejectOrQueue({
        currentAction,
        newCommand: command,
        canInterrupt: false,
      });
      if (rejectResult.decision === "reject") {
        return this.buildReject(command, rejectResult.reason ?? "cannot_interrupt_rejected");
      }
      return this.enqueueCommand(command, monotonicMs);
    }

    if (policy === "enqueue") {
      return this.enqueueCommand(command, monotonicMs);
    }

    if (policy === "play_after_current") {
      return this.enqueueCommand(command, monotonicMs);
    }

    if (policy === "drop_if_busy") {
      if (!currentAction) {
        return this.accept(command, monotonicMs);
      }
      if (this.isDefaultStableAction(currentAction)) {
        return this.accept(command, monotonicMs);
      }
      return this.buildReject(command, "dropped_busy_non_default");
    }

    if (policy === "coalesce") {
      if (currentAction && this.isSameLoopAction(currentAction, command)) {
        const ack = this.buildAck(command, "satisfied", "coalesce_same_loop", command.playbackInstanceId, 0);
        this.rememberIdempotency(command.idempotencyKey, ack, monotonicMs);
        return { ack, decision: "satisfied" };
      }
      const queued = this.queue.coalesce(command, monotonicMs);
      const ack = queued.ack;
      this.rememberIdempotency(command.idempotencyKey, ack, monotonicMs);
      return { ack, decision: "queue" };
    }

    return this.buildReject(command, "unknown_queue_policy");
  }

  private accept(command: PlayActionCommand, monotonicMs: number): ProcessCommandResult {
    const ack = this.buildAck(command, "accepted", null, command.playbackInstanceId, 0);
    this.rememberIdempotency(command.idempotencyKey, ack, monotonicMs);
    return { ack, decision: "accept_and_load" };
  }

  private enqueueCommand(
    command: PlayActionCommand,
    monotonicMs: number,
  ): ProcessCommandResult {
    const queued = this.queue.enqueue(command, monotonicMs);
    if (!queued) {
      return this.buildReject(command, "queue_full");
    }
    const ack = queued.ack;
    this.rememberIdempotency(command.idempotencyKey, ack, monotonicMs);
    return { ack, decision: "queue" };
  }

  private buildReject(command: PlayActionCommand, reason: string): ProcessCommandResult {
    const ack = this.buildAck(command, "rejected", reason, null, 0);
    return { ack, decision: "reject" };
  }

  private buildAck(
    command: PlayActionCommand,
    status: CommandAck["status"],
    reason: string | null,
    playbackInstanceId: string | null,
    queuePosition: number,
  ): CommandAck {
    return {
      commandId: command.commandId,
      status,
      reason,
      playbackInstanceId,
      queuePosition,
      actualPackageRevision: this.getPackageRevision(),
    };
  }

  private isValidCommand(command: PlayActionCommand): boolean {
    if (!command.commandId) {
      return false;
    }
    if (!command.idempotencyKey) {
      return false;
    }
    if (!command.playbackInstanceId) {
      return false;
    }
    if (!command.actionKey) {
      return false;
    }
    return true;
  }

  private isSameLoopAction(action: LoadedAction, command: PlayActionCommand): boolean {
    return action.actionKey === command.actionKey && action.loopType === "loop";
  }

  private isDefaultStableAction(action: LoadedAction): boolean {
    return action.isStableStateCandidate && !action.isTransitionOnly;
  }

  private ackStatusToDecision(status: CommandAck["status"]): CommandDecision {
    switch (status) {
      case "accepted":
        return "accept_and_load";
      case "queued":
        return "queue";
      case "rejected":
        return "reject";
      case "satisfied":
        return "satisfied";
      case "expired":
        return "expired";
      case "duplicate":
        return "duplicate";
      case "cancelled":
        return "reject";
      case "stale":
        return "reject";
      default:
        return "reject";
    }
  }

  private evictOldestIdempotency(): void {
    let oldestKey: string | null = null;
    let oldestTime = Number.POSITIVE_INFINITY;
    for (const [key, entry] of this.idempotency) {
      if (entry.recordedAtMs < oldestTime) {
        oldestTime = entry.recordedAtMs;
        oldestKey = key;
      }
    }
    if (oldestKey) {
      this.idempotency.delete(oldestKey);
    }
  }
}
