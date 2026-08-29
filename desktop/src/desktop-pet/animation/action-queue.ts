import type {
  PlayActionCommand,
  QueuedCommand,
  CommandAck,
} from "./contracts";
import { DEFAULT_QUEUE_LIMITS } from "./contracts";

export interface QueueLimits {
  total: number;
  perActionKey: number;
  perMutexGroup: number;
}

export type MutexGroupResolver = (actionKey: string) => string | null;

export type QueueRemovalReason =
  | "expired"
  | "coalesced"
  | "per_action_limit"
  | "mutex_limit"
  | "capacity"
  | "cleared";

export interface QueueRemoval {
  item: QueuedCommand;
  reason: QueueRemovalReason;
}

/**
 * Bounded renderer command queue. Every command removed after it was accepted
 * into the queue is retained in the removal journal until the engine drains it
 * and emits a terminal lifecycle event. This prevents silent accepted-command
 * loss during coalesce/expiry/eviction/package switch.
 */
export class ActionQueue {
  private items: QueuedCommand[] = [];
  private removals: QueueRemoval[] = [];
  private readonly limits: QueueLimits;
  private readonly resolveMutexGroup: MutexGroupResolver;

  constructor(input?: {
    limits?: QueueLimits;
    resolveMutexGroup?: MutexGroupResolver;
  }) {
    this.limits = input?.limits ?? { ...DEFAULT_QUEUE_LIMITS };
    this.resolveMutexGroup = input?.resolveMutexGroup ?? (() => null);
  }

  enqueue(command: PlayActionCommand, monotonicMs: number): QueuedCommand | null {
    if (this.isExpired(command)) {
      return null;
    }
    this.evictForInsertion(command);
    if (this.items.length >= this.limits.total) {
      if (!this.evictWeakest("capacity")) {
        return null;
      }
    }
    const queued = this.buildQueued(command, monotonicMs);
    this.insertSorted(queued);
    this.reindex();
    return this.findInQueue(command.commandId) ?? queued;
  }

  dequeue(): QueuedCommand | null {
    const head = this.items.shift();
    if (!head) {
      return null;
    }
    this.reindex();
    return head;
  }

  peek(): QueuedCommand | null {
    return this.items.length > 0 ? this.items[0] : null;
  }

  remove(commandId: string): boolean {
    const index = this.items.findIndex((item) => item.command.commandId === commandId);
    if (index < 0) {
      return false;
    }
    this.items.splice(index, 1);
    this.reindex();
    return true;
  }

  coalesce(command: PlayActionCommand, monotonicMs: number): QueuedCommand {
    this.removeCoalescable(command);
    this.evictForInsertion(command);
    while (this.items.length >= this.limits.total) {
      if (!this.evictWeakest("capacity")) {
        break;
      }
    }
    const queued = this.buildQueued(command, monotonicMs);
    this.insertSorted(queued);
    this.reindex();
    return this.findInQueue(command.commandId) ?? queued;
  }

  clear(reason: QueueRemovalReason = "cleared"): void {
    if (this.items.length > 0) {
      for (const item of this.items) this.recordRemoval(item, reason);
    }
    this.items = [];
  }

  drainRemovals(): QueueRemoval[] {
    if (this.removals.length === 0) return [];
    const out = this.removals;
    this.removals = [];
    return out;
  }

  size(): number {
    return this.items.length;
  }

  getByActionKey(actionKey: string): QueuedCommand[] {
    return this.items.filter((item) => item.command.actionKey === actionKey);
  }

  getByMutexGroup(mutexGroup: string): QueuedCommand[] {
    return this.items.filter(
      (item) => (this.resolveMutexGroup(item.command.actionKey) ?? null) === mutexGroup,
    );
  }

  toArray(): QueuedCommand[] {
    return [...this.items];
  }

  private buildQueued(command: PlayActionCommand, monotonicMs: number): QueuedCommand {
    return {
      command,
      acceptedMonotonicMs: monotonicMs,
      ack: {
        commandId: command.commandId,
        status: "queued",
        reason: null,
        playbackInstanceId: command.playbackInstanceId,
        queuePosition: 0,
        actualPackageRevision: command.packageRevision,
      },
    };
  }

  private findInQueue(commandId: string): QueuedCommand | undefined {
    return this.items.find((item) => item.command.commandId === commandId);
  }

  private removeCoalescable(command: PlayActionCommand): void {
    const incomingMutex = this.resolveMutexGroup(command.actionKey) ?? null;
    for (let i = this.items.length - 1; i >= 0; i--) {
      const existing = this.items[i];
      const existingMutex = this.resolveMutexGroup(existing.command.actionKey) ?? null;
      const sameActionKey = existing.command.actionKey === command.actionKey;
      const sameMutex = incomingMutex !== null && existingMutex === incomingMutex;
      if (sameActionKey || sameMutex) {
        const [removed] = this.items.splice(i, 1);
        if (removed) this.recordRemoval(removed, "coalesced");
      }
    }
    this.reindex();
  }

  private insertSorted(queued: QueuedCommand): void {
    let insertIndex = this.items.length;
    for (let i = this.items.length - 1; i >= 0; i--) {
      const existing = this.items[i];
      if (queued.command.priority > existing.command.priority) {
        insertIndex = i;
        continue;
      }
      if (queued.command.priority === existing.command.priority) {
        if (queued.acceptedMonotonicMs < existing.acceptedMonotonicMs) {
          insertIndex = i;
          continue;
        }
      }
      insertIndex = i + 1;
      break;
    }
    this.items.splice(insertIndex, 0, queued);
  }

  private reindex(): void {
    for (let i = 0; i < this.items.length; i++) {
      const item = this.items[i];
      this.items[i] = {
        ...item,
        ack: { ...item.ack, queuePosition: i },
      };
    }
  }

  private isExpired(command: PlayActionCommand): boolean {
    if (!command.expiresAt) {
      return false;
    }
    const expiresAtMs = Date.parse(command.expiresAt);
    if (Number.isNaN(expiresAtMs)) {
      return false;
    }
    return Date.now() > expiresAtMs;
  }

  private evictForInsertion(command: PlayActionCommand): void {
    this.removeExpired();
    this.enforcePerActionKeyLimit(command.actionKey);
    const mutexGroup = this.resolveMutexGroup(command.actionKey);
    if (mutexGroup) {
      this.enforcePerMutexGroupLimit(mutexGroup);
    }
  }

  private removeExpired(): void {
    for (let i = this.items.length - 1; i >= 0; i--) {
      const item = this.items[i];
      if (!this.isExpired(item.command)) continue;
      this.items.splice(i, 1);
      this.recordRemoval(item, "expired");
    }
    this.reindex();
  }

  private enforcePerActionKeyLimit(actionKey: string): void {
    const matches = this.items.filter((item) => item.command.actionKey === actionKey);
    while (matches.length >= this.limits.perActionKey) {
      const victim = matches.shift();
      if (!victim) break;
      this.removeTracked(victim.command.commandId, "per_action_limit");
    }
  }

  private enforcePerMutexGroupLimit(mutexGroup: string): void {
    const matches = this.items.filter(
      (item) => (this.resolveMutexGroup(item.command.actionKey) ?? null) === mutexGroup,
    );
    while (matches.length >= this.limits.perMutexGroup) {
      const victim = matches.shift();
      if (!victim) break;
      this.removeTracked(victim.command.commandId, "mutex_limit");
    }
  }

  private evictWeakest(reason: QueueRemovalReason): boolean {
    if (this.items.length === 0) return false;
    let victimIndex = -1;
    let victimScore = Number.POSITIVE_INFINITY;
    for (let i = 0; i < this.items.length; i++) {
      const item = this.items[i];
      const expired = this.isExpired(item.command);
      const score = this.computeEvictionScore(item, expired, i);
      if (score < victimScore) {
        victimScore = score;
        victimIndex = i;
      }
    }
    if (victimIndex < 0) return false;
    const [victim] = this.items.splice(victimIndex, 1);
    if (victim) this.recordRemoval(victim, reason);
    this.reindex();
    return true;
  }

  private removeTracked(commandId: string, reason: QueueRemovalReason): void {
    const index = this.items.findIndex((item) => item.command.commandId === commandId);
    if (index < 0) return;
    const [item] = this.items.splice(index, 1);
    if (item) this.recordRemoval(item, reason);
    this.reindex();
  }

  private recordRemoval(item: QueuedCommand, reason: QueueRemovalReason): void {
    this.removals.push({ item, reason });
  }

  private computeEvictionScore(item: QueuedCommand, isExpired: boolean, index: number): number {
    if (isExpired) return -1000000 - index;
    const priorityScore = item.command.priority * 1000;
    const recencyScore = index;
    return priorityScore + recencyScore;
  }
}

export function buildCommandAck(
  commandId: string,
  status: CommandAck["status"],
  reason: string | null,
  packageRevision: number,
  queuePosition: number = 0,
  playbackInstanceId: string | null = null,
): CommandAck {
  return {
    commandId,
    status,
    reason,
    playbackInstanceId,
    queuePosition,
    actualPackageRevision: packageRevision,
  };
}
