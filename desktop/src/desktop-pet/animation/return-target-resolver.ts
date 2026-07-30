import type {
  LoadedAction,
  ReturnTarget,
} from "./contracts";
import { FALLBACK_MAX_DEPTH } from "./contracts";

export interface ResolverDeps {
  getLoadedAction: (actionKey: string) => LoadedAction | null;
  getDefaultActionKey: () => string | null;
  getPreviousStableActionKey: () => string | null;
  getCurrentActivityActionKey?: () => string | null;
}

export interface ResolveInput {
  returnOverride?: ReturnTarget;
  actionReturnTarget: ReturnTarget;
  queueHasItems: boolean;
}

export interface ResolveResult {
  targetActionKey: string | null;
  target: ReturnTarget;
  warnings: string[];
}

export class ReturnTargetResolver {
  private readonly getLoadedAction: (actionKey: string) => LoadedAction | null;
  private readonly getDefaultActionKey: () => string | null;
  private readonly getPreviousStableActionKey: () => string | null;
  private readonly getCurrentActivityActionKey?: () => string | null;

  constructor(deps: ResolverDeps) {
    this.getLoadedAction = deps.getLoadedAction;
    this.getDefaultActionKey = deps.getDefaultActionKey;
    this.getPreviousStableActionKey = deps.getPreviousStableActionKey;
    this.getCurrentActivityActionKey = deps.getCurrentActivityActionKey;
  }

  resolve(input: ResolveInput): ResolveResult {
    const warnings: string[] = [];
    if (input.queueHasItems) {
      return {
        targetActionKey: null,
        target: { type: "none" },
        warnings,
      };
    }
    const override = input.returnOverride;
    if (override) {
      return this.resolveTarget(override, warnings);
    }
    return this.resolveTarget(input.actionReturnTarget, warnings);
  }

  updatePreviousStable(action: LoadedAction | null, isCompleting: boolean): string | null {
    if (!action) {
      return null;
    }
    if (!isCompleting) {
      return null;
    }
    if (action.isTransitionOnly) {
      return null;
    }
    if (!action.isStableStateCandidate) {
      return null;
    }
    return action.actionKey;
  }

  checkFallbackCycle(chain: string[]): boolean {
    if (chain.length > FALLBACK_MAX_DEPTH) {
      return true;
    }
    const seen = new Set<string>();
    for (const key of chain) {
      if (seen.has(key)) {
        return true;
      }
      seen.add(key);
    }
    return false;
  }

  private resolveTarget(target: ReturnTarget, warnings: string[]): ResolveResult {
    switch (target.type) {
      case "action": {
        const loaded = this.getLoadedAction(target.actionKey);
        if (!loaded) {
          warnings.push(`action_not_found:${target.actionKey}`);
          return this.fallbackDefault(warnings);
        }
        return {
          targetActionKey: target.actionKey,
          target: { type: "action", actionKey: target.actionKey },
          warnings,
        };
      }
      case "previous": {
        const key = this.getPreviousStableActionKey();
        if (!key) {
          warnings.push("previous_stable_unavailable");
          return this.fallbackDefault(warnings);
        }
        const loaded = this.getLoadedAction(key);
        if (!loaded) {
          warnings.push(`previous_action_not_found:${key}`);
          return this.fallbackDefault(warnings);
        }
        return {
          targetActionKey: key,
          target: { type: "previous" },
          warnings,
        };
      }
      case "current_activity": {
        if (this.getCurrentActivityActionKey) {
          const key = this.getCurrentActivityActionKey();
          if (key) {
            const loaded = this.getLoadedAction(key);
            if (loaded) {
              return {
                targetActionKey: key,
                target: { type: "current_activity" },
                warnings,
              };
            }
            warnings.push(`current_activity_not_found:${key}`);
          } else {
            warnings.push("current_activity_unavailable");
          }
        } else {
          warnings.push("current_activity_not_wired");
        }
        return this.fallbackDefault(warnings);
      }
      case "default": {
        return this.fallbackDefault(warnings);
      }
      case "none": {
        return {
          targetActionKey: null,
          target: { type: "none" },
          warnings,
        };
      }
      default: {
        return this.fallbackDefault(warnings);
      }
    }
  }

  private fallbackDefault(warnings: string[]): ResolveResult {
    const key = this.getDefaultActionKey();
    if (!key) {
      warnings.push("default_unavailable");
      return {
        targetActionKey: null,
        target: { type: "none" },
        warnings,
      };
    }
    const loaded = this.getLoadedAction(key);
    if (!loaded) {
      warnings.push(`default_action_not_found:${key}`);
      return {
        targetActionKey: null,
        target: { type: "none" },
        warnings,
      };
    }
    return {
      targetActionKey: key,
      target: { type: "default" },
      warnings,
    };
  }
}
