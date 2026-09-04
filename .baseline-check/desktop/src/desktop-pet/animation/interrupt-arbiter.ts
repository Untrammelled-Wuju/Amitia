import type {
  LoadedAction,
  PlayActionCommand,
  QueuePolicy,
} from "./contracts";

export interface CanInterruptInput {
  currentAction: LoadedAction;
  currentLocalElapsedMs: number;
  newCommand: PlayActionCommand;
  isAtomicPackageCommit: boolean;
}

export interface CanInterruptResult {
  canInterrupt: boolean;
  reason?: string;
}

export interface ShouldRejectOrQueueInput {
  currentAction: LoadedAction;
  newCommand: PlayActionCommand;
  canInterrupt: boolean;
}

export interface ShouldRejectOrQueueResult {
  decision: "reject" | "queue" | "interrupt";
  reason?: string;
}

export class InterruptArbiter {
  canInterrupt(input: CanInterruptInput): CanInterruptResult {
    if (input.isAtomicPackageCommit) {
      return { canInterrupt: false, reason: "atomic_package_commit_in_progress" };
    }
    const policy = input.newCommand.interruptPolicy;
    if (policy === "force_system") {
      return { canInterrupt: true };
    }
    if (policy === "never_interrupt") {
      return { canInterrupt: false, reason: "interrupt_policy_never_interrupt" };
    }
    const action = input.currentAction;
    if (!action.interruptible) {
      return { canInterrupt: false, reason: "action_not_interruptible" };
    }
    if (input.currentLocalElapsedMs < action.interruptAfterMs) {
      return {
        canInterrupt: false,
        reason: `interrupt_after_ms_not_reached:${action.interruptAfterMs}`,
      };
    }
    if (input.currentLocalElapsedMs < action.minimumPlayMs) {
      return {
        canInterrupt: false,
        reason: `minimum_play_ms_not_reached:${action.minimumPlayMs}`,
      };
    }
    if (input.newCommand.priority < action.defaultPriority) {
      return {
        canInterrupt: false,
        reason: `priority_below_default:${action.defaultPriority}`,
      };
    }
    return { canInterrupt: true };
  }

  shouldRejectOrQueue(input: ShouldRejectOrQueueInput): ShouldRejectOrQueueResult {
    if (input.canInterrupt) {
      return { decision: "interrupt" };
    }
    const queuePolicy: QueuePolicy = input.newCommand.queuePolicy;
    if (queuePolicy === "drop_if_busy") {
      return { decision: "reject", reason: "cannot_interrupt_drop_if_busy" };
    }
    if (queuePolicy === "replace_current") {
      return { decision: "queue", reason: "cannot_interrupt_queued" };
    }
    if (queuePolicy === "enqueue" || queuePolicy === "play_after_current") {
      return { decision: "queue", reason: "queued_by_policy" };
    }
    if (queuePolicy === "coalesce") {
      return { decision: "queue", reason: "coalesce_queued" };
    }
    return { decision: "queue", reason: "default_queued" };
  }
}
