export interface ControlEffectSinkDescriptor {
  sinkId: string;
  kind: string;
  serviceId?: string;
  description?: string;
}

export interface ControlAuthoritySnapshot {
  runtimeId: string;
  pluginId: string;
  mode: string;
  epoch: number;
  serviceId?: string;
  updatedAt?: number;
  valid: boolean;
}
