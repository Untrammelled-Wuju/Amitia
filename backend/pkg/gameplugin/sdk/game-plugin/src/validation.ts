import { PROTOCOL_VERSION, ChannelDescriptor, ServiceDescriptor, HostFeature } from './protocol';

const MAX_ID_LENGTH = 256;

export function validateMessageId(id: string): string | null {
  if (!id) return 'message id must not be empty';
  if (id.length > MAX_ID_LENGTH) return `message id exceeds maximum length of ${MAX_ID_LENGTH}`;
  if (/[\x00-\x1f]/.test(id)) return 'message id contains control character';
  return null;
}

export function validateMethod(method: string): string | null {
  if (!method) return 'method must not be empty';
  const parts = method.split('.');
  if (parts.length < 2) return 'method must have at least two parts separated by dots';
  for (const part of parts) {
    if (!part) return 'method parts must not be empty';
    if (/[A-Z]/.test(part)) return 'method must be lowercase';
  }
  return null;
}

export function isReservedNamespace(method: string): boolean {
  const reserved = ['host.', 'plugin.', 'runtime.', 'service.', 'channel.', 'control.', 'emergency.', 'secret.', 'artifact.', 'permission.', 'binary.'];
  return reserved.some((ns) => method.startsWith(ns));
}

export function validatePluginMethod(method: string): string | null {
  const methodErr = validateMethod(method);
  if (methodErr) return methodErr;
  if (isReservedNamespace(method)) return `method '${method}' uses reserved namespace`;
  return null;
}

export function validateServiceId(id: string): string | null {
  if (!id) return 'service id must not be empty';
  if (id.length > MAX_ID_LENGTH) return `service id exceeds maximum length of ${MAX_ID_LENGTH}`;
  if (/[\x00-\x1f]/.test(id)) return 'service id contains control character';
  if (/\s/.test(id)) return 'service id must not contain spaces';
  return null;
}

export function validateServices(services: ServiceDescriptor[]): string[] {
  const errors: string[] = [];
  const seen = new Set<string>();

  for (const svc of services) {
    const idErr = validateServiceId(svc.id);
    if (idErr) errors.push(`service '${svc.id}': ${idErr}`);

    if (svc.kind !== 'process') {
      errors.push(`service '${svc.id}': invalid kind '${svc.kind}'`);
    }

    if (seen.has(svc.id)) {
      errors.push(`duplicate service id '${svc.id}'`);
    }
    seen.add(svc.id);

    if (svc.dependsOn) {
      if (svc.dependsOn.includes(svc.id)) {
        errors.push(`service '${svc.id}' depends on itself`);
      }
      const depSeen = new Set<string>();
      for (const dep of svc.dependsOn) {
        if (depSeen.has(dep)) {
          errors.push(`service '${svc.id}' has duplicate dependency '${dep}'`);
        }
        depSeen.add(dep);
      }
    }

    if (svc.capabilities) {
      const capSeen = new Set<string>();
      for (const cap of svc.capabilities) {
        if (capSeen.has(cap)) {
          errors.push(`service '${svc.id}' has duplicate capability '${cap}'`);
        }
        capSeen.add(cap);
      }
    }
  }

  return errors;
}

export function validateChannel(channel: ChannelDescriptor): string[] {
  const errors: string[] = [];

  if (!channel.id) {
    errors.push('channel id must not be empty');
  }

  const validKinds = ['event', 'state', 'log', 'metric', 'custom', 'binary'];
  if (!validKinds.includes(channel.kind)) {
    errors.push(`channel '${channel.id}': invalid kind '${channel.kind}'`);
  }

  const validDirections = ['plugin_to_host', 'host_to_plugin', 'bidirectional'];
  if (channel.direction && !validDirections.includes(channel.direction)) {
    errors.push(`channel '${channel.id}': invalid direction '${channel.direction}'`);
  }

  const validHints = ['low', 'normal', 'high', 'realtime'];
  if (channel.frequencyHint && !validHints.includes(channel.frequencyHint)) {
    errors.push(`channel '${channel.id}': invalid frequency hint '${channel.frequencyHint}'`);
  }

  return errors;
}

export function validateCapability(capabilities: string[]): string[] {
  const errors: string[] = [];
  const seen = new Set<string>();
  const known = new Set<string>(Object.values(HostFeature));

  for (const cap of capabilities) {
    if (!known.has(cap)) {
      errors.push(`unknown host feature '${cap}'`);
      continue;
    }
    if (seen.has(cap)) {
      errors.push(`duplicate host feature '${cap}'`);
    }
    seen.add(cap);
  }

  return errors;
}

export function validateEnvelope(envelope: unknown): string | null {
  if (!envelope || typeof envelope !== 'object') {
    return 'envelope must be an object';
  }
  const env = envelope as Record<string, unknown>;
  if (env.protocol !== PROTOCOL_VERSION) {
    return `invalid protocol: ${env.protocol}, expected: ${PROTOCOL_VERSION}`;
  }
  return null;
}
