import { Envelope } from '@amitia/game-plugin-sdk';

export interface ControlEffectPayload {
  action?: string;
  value?: number;
}

export interface TakeoverPayload {
  targetMode?: string;
  actor?: string;
}

export interface ReleasePayload {
  targetMode?: string;
  actor?: string;
}

export interface CheckPayload {
  permissionId?: string;
}

export interface EffectResult {
  allowed: boolean;
  reason: string;
  epoch?: number;
  currentEpoch?: number;
  mode?: string;
  outputId?: string;
}

export interface AuthorityState {
  mode: string;
  epoch: number;
  valid: boolean;
}

export class ControlService {
  private authorityMode: string = 'observe_only';
  private authorityEpoch: number = 0;
  private sinkRegistered: boolean = false;
  private emergencyActive: boolean = false;

  getAuthorityMode(): string {
    return this.authorityMode;
  }

  getAuthorityEpoch(): number {
    return this.authorityEpoch;
  }

  isEmergencyActive(): boolean {
    return this.emergencyActive;
  }

  isSinkRegistered(): boolean {
    return this.sinkRegistered;
  }

  isControlAllowed(): boolean {
    return !this.emergencyActive &&
      (this.authorityMode === 'shared_control' || this.authorityMode === 'plugin_control');
  }

  updateAuthority(mode: string, epoch: number): void {
    this.authorityMode = mode;
    this.authorityEpoch = epoch;
  }

  setEmergencyActive(active: boolean): void {
    this.emergencyActive = active;
    if (active) {
      this.authorityMode = 'suspended';
    }
  }

  setSinkRegistered(registered: boolean): void {
    this.sinkRegistered = registered;
  }

  handleControlProbe(request: Envelope): Record<string, unknown> {
    const payload = (request.payload as ControlEffectPayload) || {};
    return {
      probe: payload.action || 'noop',
      mode: this.authorityMode,
      epoch: this.authorityEpoch,
      description: 'opaque action',
    };
  }

  checkPreconditions(): EffectResult {
    if (this.emergencyActive) {
      return {
        allowed: false,
        reason: 'emergency_stop_active',
      };
    }
    if (!this.isControlAllowed()) {
      return {
        allowed: false,
        reason: 'authority_mode_denied',
        mode: this.authorityMode,
        epoch: this.authorityEpoch,
      };
    }
    return {
      allowed: true,
      reason: 'preconditions_met',
      mode: this.authorityMode,
      epoch: this.authorityEpoch,
    };
  }
}

export class SecurityService {
  private leaseId: string = '';
  private secretRef: string = 'secret://mock.provider.credential';

  hasActiveLease(): boolean {
    return this.leaseId !== '';
  }

  getLeaseId(): string {
    return this.leaseId;
  }

  getSecretRef(): string {
    return this.secretRef;
  }

  handleSecretProbe(_request: Envelope): Record<string, unknown> {
    return {
      leaseId: this.leaseId,
      hasLease: this.hasActiveLease(),
      leaseStatus: 'granted',
      ref: this.secretRef,
    };
  }

  handleReleaseLease(): Record<string, unknown> {
    if (!this.hasActiveLease()) {
      return {
        released: false,
        reason: 'no_active_lease',
      };
    }
    this.leaseId = '';
    return {
      released: true,
      reason: 'ok',
    };
  }
}
