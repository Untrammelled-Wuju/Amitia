import { isIP } from 'net';
import { posix as posixPath } from 'path';
import type {
  PluginArtifact,
  PluginChannelSpec,
  PluginHostSpec,
  PluginNetworkPolicy,
  PluginServiceSpec,
} from './game';
import {
  PROTOCOL_VERSION,
  ChannelDescriptor,
  ServiceDescriptor,
  HostFeature,
} from './protocol';

const MAX_ID_LENGTH = 256;
const MAX_SERVICE_NAME_LENGTH = 256;
const MAX_CHANNEL_SCHEMA_ID_LENGTH = 1024;
const MAX_DNS_NAME_LENGTH = 253;
const MAX_INT64 = BigInt('9223372036854775807');

function utf8Length(value: string): number {
  return Buffer.byteLength(value, 'utf8');
}

function containsUnicodeControl(value: string): boolean {
  return /\p{Cc}/u.test(value);
}

function containsChannelControl(value: string): boolean {
  for (const ch of value) {
    const codePoint = ch.codePointAt(0)!;
    if (codePoint < 32 || codePoint === 127) return true;
  }
  return false;
}

export function validateMessageId(id: string): string | null {
  if (!id) return 'message id must not be empty';
  if (utf8Length(id) > MAX_ID_LENGTH) return `message id exceeds maximum length of ${MAX_ID_LENGTH}`;
  if (containsUnicodeControl(id)) return 'message id contains control character';
  return null;
}

export function validateMethod(method: string): string | null {
  if (!method) return 'method must not be empty';
  const parts = method.split('.');
  if (parts.length < 2) return 'method must have at least two parts separated by dots';
  for (const part of parts) {
    if (!part) return 'method parts must not be empty';
    if (/\p{Lu}/u.test(part)) return 'method must be lowercase';
  }
  return null;
}

export function isReservedNamespace(method: string): boolean {
  const reserved = ['host.', 'plugin.', 'runtime.', 'service.', 'channel.', 'control.', 'binary.', 'emergency.', 'secret.', 'artifact.', 'permission.'];
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
  if (utf8Length(id) > MAX_ID_LENGTH) return `service id exceeds maximum length of ${MAX_ID_LENGTH}`;
  if (containsUnicodeControl(id)) return 'service id contains control character';
  if (id.includes(' ')) return 'service id must not contain spaces';
  return null;
}

export function validateServiceName(name: string): string | null {
  if (utf8Length(name) > MAX_SERVICE_NAME_LENGTH) {
    return `service name exceeds maximum length of ${MAX_SERVICE_NAME_LENGTH}`;
  }
  if (containsUnicodeControl(name)) return 'service name contains control character';
  return null;
}

export function validateServices(services: ServiceDescriptor[]): string[] {
  const errors: string[] = [];
  const seen = new Set<string>();

  for (const svc of services) {
    const idErr = validateServiceId(svc.id);
    if (idErr) errors.push(`service '${svc.id}': ${idErr}`);

    const nameErr = validateServiceName(svc.name ?? '');
    if (nameErr) errors.push(`service '${svc.id}': ${nameErr}`);

    if (svc.kind !== 'process') {
      errors.push(`service '${svc.id}': invalid kind '${svc.kind}'`);
    }

    if (seen.has(svc.id)) {
      errors.push(`duplicate service id '${svc.id}'`);
    }
    seen.add(svc.id);

    if (svc.dependsOn) {
      const depSeen = new Set<string>();
      for (const dep of svc.dependsOn) {
        const depErr = validateServiceId(dep);
        if (depErr) errors.push(`service '${svc.id}' dependency '${dep}': ${depErr}`);
        if (dep === svc.id) {
          errors.push(`service '${svc.id}' depends on itself`);
        }
        if (depSeen.has(dep)) {
          errors.push(`service '${svc.id}' has duplicate dependency '${dep}'`);
        }
        depSeen.add(dep);
      }
    }

    if (svc.capabilities) {
      const capErrors = validateCapability(svc.capabilities);
      errors.push(...capErrors.map((error) => `service '${svc.id}': ${error}`));
    }
  }

  return errors;
}

export function validateChannelId(id: string): string | null {
  if (!id) return 'channel id must not be empty';
  if (utf8Length(id) > MAX_ID_LENGTH) return `channel id exceeds maximum length of ${MAX_ID_LENGTH}`;
  if (containsChannelControl(id)) return 'channel id contains control character';
  return null;
}

export function validateChannelSchemaId(schemaId: string): string | null {
  if (utf8Length(schemaId) > MAX_CHANNEL_SCHEMA_ID_LENGTH) {
    return `channel schema id exceeds maximum length of ${MAX_CHANNEL_SCHEMA_ID_LENGTH}`;
  }
  if (containsChannelControl(schemaId)) return 'channel schema id contains control character';
  return null;
}

export function validateChannel(channel: ChannelDescriptor): string[] {
  const errors: string[] = [];

  const idErr = validateChannelId(channel.id);
  if (idErr) errors.push(idErr);

  const validKinds = ['event', 'state', 'log', 'metric', 'custom', 'binary'];
  if (!validKinds.includes(channel.kind)) {
    errors.push(`channel '${channel.id}': invalid kind '${channel.kind}'`);
  }

  const schemaIdErr = validateChannelSchemaId(channel.schemaId ?? '');
  if (schemaIdErr) errors.push(`channel '${channel.id}': ${schemaIdErr}`);

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

export function validateChannels(channels: ChannelDescriptor[]): string[] {
  const errors: string[] = [];
  const seen = new Set<string>();
  for (const channel of channels) {
    errors.push(...validateChannel(channel));
    if (seen.has(channel.id)) {
      errors.push(`duplicate channel id '${channel.id}'`);
    }
    seen.add(channel.id);
  }
  return errors;
}

export function validateCapability(capabilities: readonly string[]): string[] {
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

function validatePluginServiceSpec(
  service: PluginServiceSpec,
  index: number,
  seenServices: Set<string>,
  seenProcessModules: Set<string>,
): string[] {
  const errors: string[] = [];
  const id = (service.id ?? '').trim();
  const moduleId = (service.moduleId ?? '').trim();
  if (!id || !moduleId) {
    errors.push(`services[${index}] id and moduleId are required`);
    return errors;
  }

  const idErr = validateServiceId(id);
  if (idErr) errors.push(`services[${index}] id: ${idErr}`);
  const nameErr = validateServiceName(service.name ?? '');
  if (nameErr) errors.push(`services[${index}] name: ${nameErr}`);

  if (seenServices.has(id)) errors.push(`duplicate service id '${id}'`);
  seenServices.add(id);

  const kind = (service.kind ?? '').trim();
  if (kind && kind !== 'process') {
    errors.push(`services[${index}] unsupported kind '${service.kind}'; protocol v1 services are process-backed`);
  }

  if (seenProcessModules.has(moduleId)) {
    errors.push(`services[${index}] reuses runtime module '${moduleId}'; each process service requires a distinct runtime module`);
  }
  seenProcessModules.add(moduleId);
  return errors;
}

function findServiceDependencyCycle(graph: Map<string, string[]>): string[] {
  const UNVISITED = 0;
  const VISITING = 1;
  const VISITED = 2;
  const state = new Map<string, number>();
  const stack: string[] = [];
  const stackIndex = new Map<string, number>();

  const visit = (id: string): string[] => {
    const current = state.get(id) ?? UNVISITED;
    if (current === VISITED) return [];
    if (current === VISITING) {
      const start = stackIndex.get(id) ?? 0;
      return [...stack.slice(start), id];
    }
    state.set(id, VISITING);
    stackIndex.set(id, stack.length);
    stack.push(id);
    const deps = [...(graph.get(id) ?? [])].sort();
    for (const dep of deps) {
      const cycle = visit(dep);
      if (cycle.length > 0) return cycle;
    }
    stack.pop();
    stackIndex.delete(id);
    state.set(id, VISITED);
    return [];
  };

  for (const id of [...graph.keys()].sort()) {
    const cycle = visit(id);
    if (cycle.length > 0) return cycle;
  }
  return [];
}

function validateDNSName(value: string): boolean {
  if (!value || value.length > MAX_DNS_NAME_LENGTH) return false;
  const labels = value.split('.');
  for (const label of labels) {
    if (!label || label.length > 63 || label.startsWith('-') || label.endsWith('-')) return false;
    if (!/^[a-z0-9-]+$/.test(label)) return false;
  }
  return true;
}

function canonicalIP(raw: string): string | null {
  const value = raw.trim();
  if (!value || value.includes('%') || isIP(value) === 0) return null;

  if (isIP(value) === 4) {
    const octets = value.split('.').map((part) => Number(part));
    if (octets.length !== 4 || octets.some((part) => !Number.isInteger(part) || part < 0 || part > 255)) return null;
    if (octets.every((part) => part === 0)) return null;
    if (octets[0] >= 224 && octets[0] <= 239) return null;
    return octets.join('.');
  }

  const lower = value.toLowerCase();
  if (lower === '::' || lower.startsWith('ff')) return null;
  const mapped = lower.match(/^::ffff:(\d{1,3}(?:\.\d{1,3}){3})$/);
  if (mapped && isIP(mapped[1]) === 4) return canonicalIP(mapped[1]);
  try {
    const url = new URL(`http://[${value}]/`);
    return url.hostname.replace(/^\[|\]$/g, '').toLowerCase();
  } catch {
    return null;
  }
}

function validateRestrictedNetworkAllowlist(
  domains: readonly string[] = [],
  ips: readonly string[] = [],
  ports: readonly number[] = [],
  transports: readonly string[] = [],
  allowHostLoopback = false,
  maxConnections = 0,
): string[] {
  const errors: string[] = [];
  if (domains.length === 0 && ips.length === 0 && !allowHostLoopback) {
    errors.push('restricted mode requires at least one network.allowedDomains, network.allowedIPs, or network.allowHostLoopback destination');
  }
  if (ports.length === 0) {
    errors.push('network.allowedPorts is required for restricted mode');
  }

  const seenDomains = new Set<string>();
  for (const raw of domains) {
    let domain = raw.toLowerCase().trim();
    if (!domain || /[\/:@?#\\]/.test(domain) || domain.endsWith('.')) {
      errors.push(`invalid restricted network domain '${raw}'`);
      continue;
    }
    if (domain.startsWith('*.')) domain = domain.slice(2);
    if (!domain || domain.includes('*') || !validateDNSName(domain)) {
      errors.push(`invalid restricted network domain '${raw}'`);
      continue;
    }
    const key = raw.toLowerCase().trim();
    if (seenDomains.has(key)) errors.push(`duplicate restricted network domain '${raw}'`);
    seenDomains.add(key);
  }

  const seenIPs = new Set<string>();
  for (const raw of ips) {
    const canonical = canonicalIP(raw);
    if (!canonical) {
      errors.push(`invalid restricted network IP '${raw}'`);
      continue;
    }
    if (seenIPs.has(canonical)) errors.push(`duplicate restricted network IP '${raw}'`);
    seenIPs.add(canonical);
  }

  const seenPorts = new Set<number>();
  for (const port of ports) {
    if (!Number.isInteger(port) || port < 1 || port > 65535) {
      errors.push(`invalid restricted network port ${port}`);
      continue;
    }
    if (seenPorts.has(port)) errors.push(`duplicate restricted network port ${port}`);
    seenPorts.add(port);
  }

  const seenTransports = new Set<string>();
  for (const raw of transports) {
    const transport = raw.toLowerCase().trim();
    if (!['http', 'https', 'tcp', 'udp', 'websocket'].includes(transport)) {
      errors.push(`invalid restricted network transport '${raw}'`);
      continue;
    }
    if (seenTransports.has(transport)) errors.push(`duplicate restricted network transport '${raw}'`);
    seenTransports.add(transport);
  }
  if (!Number.isInteger(maxConnections) || maxConnections < 0 || maxConnections > 64) {
    errors.push('network.maxConnections must be between 0 and 64; 0 uses the host default');
  }
  return errors;
}

function validateNetworkPolicy(network: PluginNetworkPolicy | undefined): string[] {
  if (!network) {
    return ['network policy is required; choose none, loopback, restricted, or unrestricted explicitly'];
  }
  const mode = (network.mode ?? '').toLowerCase().trim();
  if (!mode) return ['network.mode is required'];

  const domains = network.allowedDomains ?? [];
  const ips = network.allowedIPs ?? [];
  const ports = network.allowedPorts ?? [];
  const transports = network.allowedTransports ?? [];
  const allowHostLoopback = network.allowHostLoopback ?? false;
  const maxConnections = network.maxConnections ?? 0;
  if (mode === 'restricted') {
    return validateRestrictedNetworkAllowlist(domains, ips, ports, transports, allowHostLoopback, maxConnections);
  }
  if (mode === 'none' || mode === 'loopback' || mode === 'unrestricted') {
    if (domains.length || ips.length || ports.length || transports.length || allowHostLoopback || maxConnections) {
      return ['network allowlists and mediated transport limits are only valid for restricted mode'];
    }
    return [];
  }
  return [`unsupported network mode '${network.mode}'`];
}

function safePackageRelativePath(raw: string): boolean {
  const normalized = raw.trim().replace(/\\/g, '/');
  if (!normalized || normalized.startsWith('/') || normalized.includes(':')) return false;
  const clean = posixPath.normalize(normalized);
  return clean !== '.' && clean !== '..' && !clean.startsWith('../');
}

function parseInt64(raw: string): boolean {
  if (!/^\d+$/.test(raw)) return false;
  try {
    const value = BigInt(raw);
    return value >= 0n && value <= MAX_INT64;
  } catch {
    return false;
  }
}

function parseComparableVersion(raw: string): boolean {
  let value = raw.trim();
  if (!value) return false;
  if (value.startsWith('v') || value.startsWith('V')) value = value.slice(1);
  const plus = value.indexOf('+');
  if (plus >= 0) value = value.slice(0, plus);

  let base = value;
  let pre = '';
  const dash = value.indexOf('-');
  if (dash >= 0) {
    base = value.slice(0, dash);
    pre = value.slice(dash + 1);
    if (!pre) return false;
  }
  const parts = base.split('.');
  if (parts.length === 0 || parts.length > 3) return false;
  if (parts.some((part) => !part || !parseInt64(part))) return false;

  if (pre) {
    for (const id of pre.split('.')) {
      if (!id) return false;
      if (/^\d+$/.test(id) && !parseInt64(id)) return false;
    }
  }
  return true;
}

function parseHyphenRange(expr: string): boolean {
  const parts = expr.split(' - ');
  return parts.length === 2 && parseComparableVersion(parts[0].trim()) && parseComparableVersion(parts[1].trim());
}

function containsWildcardSegment(raw: string): boolean {
  let value = raw.trim();
  if (!value) return false;
  if (value.startsWith('v') || value.startsWith('V')) value = value.slice(1);
  return value.split('.').some((part) => part.trim() === '*' || part.trim().toLowerCase() === 'x');
}

function validWildcardConstraint(raw: string): boolean {
  let value = raw.trim();
  if (!value) return false;
  if (value.startsWith('v') || value.startsWith('V')) value = value.slice(1);
  const parts = value.split('.');
  if (parts.length === 0 || parts.length > 3) return false;
  let wildcardSeen = false;
  for (const rawPart of parts) {
    const part = rawPart.trim();
    if (part === '*' || part.toLowerCase() === 'x') {
      wildcardSeen = true;
      continue;
    }
    if (wildcardSeen || !part || !parseInt64(part)) return false;
  }
  return wildcardSeen;
}

function validateConstraintToken(token: string): boolean {
  if (token === '*' || token.toLowerCase() === 'x') return true;
  for (const prefix of ['>=', '<=', '!=', '==', '>', '<', '=', '^', '~']) {
    if (token.startsWith(prefix)) return parseComparableVersion(token.slice(prefix.length).trim());
  }
  if (/[x*]/i.test(token)) return validWildcardConstraint(token);
  return parseComparableVersion(token);
}

export function validateCompatibilityConstraint(constraint: string): string | null {
  const value = constraint.trim();
  if (!value) return 'compatibility constraint must not be empty';
  if (value === '*' || value.toLowerCase() === 'x') return null;

  if (!/[<>^~*=|, ]/.test(value) && !containsWildcardSegment(value)) return null;

  for (const rawAlternative of value.split('||')) {
    const alternative = rawAlternative.trim();
    if (!alternative) return `invalid empty compatibility alternative in '${value}'`;
    if (parseHyphenRange(alternative)) continue;
    if (alternative.includes(' - ')) return `invalid compatibility hyphen range '${alternative}'`;
    const tokens = alternative.replace(/,/g, ' ').trim().split(/\s+/).filter(Boolean);
    if (tokens.length === 0) return `invalid compatibility constraint '${value}'`;
    if (tokens.length === 1 && !/[<>^~*=,]/.test(tokens[0]) && !containsWildcardSegment(tokens[0])) continue;
    for (const token of tokens) {
      if (!validateConstraintToken(token)) return `invalid compatibility token '${token}'`;
    }
  }
  return null;
}

function validateArtifact(artifact: PluginArtifact, seenArtifacts: Set<string>): string[] {
  const errors: string[] = [];
  const id = (artifact.id ?? '').trim();
  const source = (artifact.source ?? '').trim();
  const target = (artifact.target ?? '').trim();
  const type = (artifact.type ?? '').toLowerCase().trim();
  if (!id || !type || !source || !target) {
    errors.push('artifact id, type, source and target are required');
    return errors;
  }
  if (!['file', 'directory', 'zip'].includes(type)) {
    errors.push(`artifact '${id}' has unsupported type '${artifact.type}'; protocol v1 supports file, directory and zip`);
  }
  if (seenArtifacts.has(id)) errors.push(`duplicate artifact id '${id}'`);
  seenArtifacts.add(id);
  if (!safePackageRelativePath(artifact.source ?? '')) errors.push(`artifact '${id}' source must be a safe package-relative path`);
  if (!safePackageRelativePath(target)) errors.push(`artifact '${id}' target must be a safe target-relative path`);
  for (let i = 0; i < (artifact.compatibilityVersions ?? []).length; i++) {
    const err = validateCompatibilityConstraint(artifact.compatibilityVersions![i]);
    if (err) errors.push(`artifact '${id}' compatibilityVersions[${i}]: ${err}`);
  }
  return errors;
}

function isRecord(value: unknown): value is Record<string, unknown> {
  return typeof value === 'object' && value !== null && !Array.isArray(value);
}

function rejectUnknownKeys(value: Record<string, unknown>, allowed: ReadonlySet<string>, path: string, errors: string[]): void {
  for (const key of Object.keys(value)) {
    if (!allowed.has(key)) errors.push(`${path} contains unknown field '${key}'`);
  }
}

function validateOptionalString(value: Record<string, unknown>, key: string, path: string, errors: string[]): void {
  if (value[key] !== undefined && typeof value[key] !== 'string') errors.push(`${path}.${key} must be a string`);
}

function validateOptionalBoolean(value: Record<string, unknown>, key: string, path: string, errors: string[]): void {
  if (value[key] !== undefined && typeof value[key] !== 'boolean') errors.push(`${path}.${key} must be a boolean`);
}

function validateOptionalStringArray(value: Record<string, unknown>, key: string, path: string, errors: string[]): void {
  const candidate = value[key];
  if (candidate === undefined) return;
  if (!Array.isArray(candidate) || candidate.some((item) => typeof item !== 'string')) {
    errors.push(`${path}.${key} must be an array of strings`);
  }
}

function validateStringMap(value: unknown, path: string, errors: string[]): void {
  if (!isRecord(value)) {
    errors.push(`${path} must be an object with string values`);
    return;
  }
  for (const [key, item] of Object.entries(value)) {
    if (typeof item !== 'string') errors.push(`${path}.${key} must be a string`);
  }
}

function validatePluginHostSpecShape(input: unknown): string[] {
  const errors: string[] = [];
  if (!isRecord(input)) return ['game plugin spec is required'];

  rejectUnknownKeys(input, new Set([
    'protocolVersion', 'runtimeModuleId', 'hostFeatures', 'services', 'channels',
    'controlEffectSinks', 'artifacts', 'network', 'metadata',
  ]), 'PluginHostSpec', errors);
  validateOptionalString(input, 'protocolVersion', 'PluginHostSpec', errors);
  validateOptionalString(input, 'runtimeModuleId', 'PluginHostSpec', errors);
  validateOptionalStringArray(input, 'hostFeatures', 'PluginHostSpec', errors);
  if (input.metadata !== undefined && !isRecord(input.metadata)) errors.push('PluginHostSpec.metadata must be an object');

  if (input.services !== undefined) {
    if (!Array.isArray(input.services)) {
      errors.push('PluginHostSpec.services must be an array');
    } else {
      input.services.forEach((raw, index) => {
        const path = `PluginHostSpec.services[${index}]`;
        if (!isRecord(raw)) {
          errors.push(`${path} must be an object`);
          return;
        }
        rejectUnknownKeys(raw, new Set(['id', 'moduleId', 'name', 'kind', 'required', 'dependsOn', 'metadata']), path, errors);
        for (const key of ['id', 'moduleId', 'name', 'kind']) validateOptionalString(raw, key, path, errors);
        validateOptionalBoolean(raw, 'required', path, errors);
        validateOptionalStringArray(raw, 'dependsOn', path, errors);
        if (raw.metadata !== undefined) validateStringMap(raw.metadata, `${path}.metadata`, errors);
      });
    }
  }

  if (input.channels !== undefined) {
    if (!Array.isArray(input.channels)) {
      errors.push('PluginHostSpec.channels must be an array');
    } else {
      input.channels.forEach((raw, index) => {
        const path = `PluginHostSpec.channels[${index}]`;
        if (!isRecord(raw)) {
          errors.push(`${path} must be an object`);
          return;
        }
        rejectUnknownKeys(raw, new Set(['id', 'serviceId', 'kind', 'schemaId', 'direction', 'frequencyHint', 'metadata']), path, errors);
        for (const key of ['id', 'serviceId', 'kind', 'schemaId', 'direction', 'frequencyHint']) validateOptionalString(raw, key, path, errors);
        if (raw.metadata !== undefined) validateStringMap(raw.metadata, `${path}.metadata`, errors);
      });
    }
  }

  if (input.controlEffectSinks !== undefined) {
    if (!Array.isArray(input.controlEffectSinks)) {
      errors.push('PluginHostSpec.controlEffectSinks must be an array');
    } else {
      input.controlEffectSinks.forEach((raw, index) => {
        const path = `PluginHostSpec.controlEffectSinks[${index}]`;
        if (!isRecord(raw)) {
          errors.push(`${path} must be an object`);
          return;
        }
        rejectUnknownKeys(raw, new Set(['id', 'serviceId', 'description']), path, errors);
        for (const key of ['id', 'serviceId', 'description']) validateOptionalString(raw, key, path, errors);
      });
    }
  }

  if (input.artifacts !== undefined) {
    if (!Array.isArray(input.artifacts)) {
      errors.push('PluginHostSpec.artifacts must be an array');
    } else {
      input.artifacts.forEach((raw, index) => {
        const path = `PluginHostSpec.artifacts[${index}]`;
        if (!isRecord(raw)) {
          errors.push(`${path} must be an object`);
          return;
        }
        rejectUnknownKeys(raw, new Set([
          'id', 'type', 'platforms', 'architectures', 'compatibilityVersions',
          'source', 'target', 'required', 'sha256',
        ]), path, errors);
        for (const key of ['id', 'type', 'source', 'target', 'sha256']) validateOptionalString(raw, key, path, errors);
        for (const key of ['platforms', 'architectures', 'compatibilityVersions']) validateOptionalStringArray(raw, key, path, errors);
        validateOptionalBoolean(raw, 'required', path, errors);
      });
    }
  }

  if (input.network !== undefined) {
    const raw = input.network;
    if (!isRecord(raw)) {
      errors.push('PluginHostSpec.network must be an object');
    } else {
      rejectUnknownKeys(raw, new Set(['mode', 'allowedDomains', 'allowedIPs', 'allowedPorts', 'allowedTransports', 'allowHostLoopback', 'maxConnections']), 'PluginHostSpec.network', errors);
      validateOptionalString(raw, 'mode', 'PluginHostSpec.network', errors);
      validateOptionalStringArray(raw, 'allowedDomains', 'PluginHostSpec.network', errors);
      validateOptionalStringArray(raw, 'allowedIPs', 'PluginHostSpec.network', errors);
      validateOptionalStringArray(raw, 'allowedTransports', 'PluginHostSpec.network', errors);
      validateOptionalBoolean(raw, 'allowHostLoopback', 'PluginHostSpec.network', errors);
      if (raw.maxConnections !== undefined && (typeof raw.maxConnections !== 'number' || !Number.isInteger(raw.maxConnections))) {
        errors.push('PluginHostSpec.network.maxConnections must be an integer');
      }
      if (raw.allowedPorts !== undefined) {
        if (!Array.isArray(raw.allowedPorts) || raw.allowedPorts.some((item) => typeof item !== 'number' || !Number.isInteger(item))) {
          errors.push('PluginHostSpec.network.allowedPorts must be an array of integers');
        }
      }
    }
  }

  return errors;
}

/**
 * Validate the public PluginHostSpec with the same parse/semantic boundaries
 * enforced by Go ParsePluginHostSpec + PluginHostSpec.Validate(). Unknown
 * fields and wrong JSON types are rejected before semantic validation.
 */
export function validatePluginHostSpec(input: unknown): string[] {
  const shapeErrors = validatePluginHostSpecShape(input);
  if (shapeErrors.length > 0) return shapeErrors;
  const spec = input as PluginHostSpec;
  const errors: string[] = [];
  if (!spec.protocolVersion) return ['protocolVersion is required'];
  if (spec.protocolVersion !== PROTOCOL_VERSION) return [`unsupported protocolVersion '${spec.protocolVersion}'`];

  const runtimeModuleId = (spec.runtimeModuleId ?? '').trim();
  const services = spec.services ?? [];
  if (!runtimeModuleId && services.length === 0) return ['runtimeModuleId or services is required'];

  const hostFeatures = spec.hostFeatures ?? [];
  const seenFeatures = new Set<string>();
  const knownFeatures = new Set<string>(Object.values(HostFeature));
  for (const raw of hostFeatures) {
    const feature = String(raw).trim();
    if (!knownFeatures.has(feature)) errors.push(`unsupported host feature '${String(raw)}' for ${PROTOCOL_VERSION}`);
    if (seenFeatures.has(feature)) errors.push(`duplicate host feature '${feature}'`);
    seenFeatures.add(feature);
  }
  if (services.length > 1 && !seenFeatures.has(HostFeature.MULTI_SERVICE)) {
    errors.push(`multiple services require hostFeatures to include '${HostFeature.MULTI_SERVICE}'`);
  }

  const seenServices = new Set<string>();
  const seenProcessModules = new Set<string>();
  services.forEach((service, index) => {
    errors.push(...validatePluginServiceSpec(service, index, seenServices, seenProcessModules));
  });

  if (services.length > 0 && runtimeModuleId && !seenProcessModules.has(runtimeModuleId)) {
    errors.push(`runtimeModuleId '${runtimeModuleId}' must reference one of services[].moduleId when services are declared`);
  }

  const dependencyGraph = new Map<string, string[]>();
  services.forEach((service, index) => {
    const serviceId = (service.id ?? '').trim();
    const seenDeps = new Set<string>();
    for (const rawDep of service.dependsOn ?? []) {
      const dep = rawDep.trim();
      if (!dep) {
        errors.push(`services[${index}] contains empty dependsOn id`);
        continue;
      }
      const depErr = validateServiceId(dep);
      if (depErr) errors.push(`services[${index}] dependsOn id: ${depErr}`);
      if (dep === serviceId) errors.push(`service '${service.id}' cannot depend on itself`);
      if (!seenServices.has(dep)) errors.push(`service '${service.id}' depends on unknown service '${dep}'`);
      if (seenDeps.has(dep)) errors.push(`service '${service.id}' contains duplicate dependency '${dep}'`);
      seenDeps.add(dep);
      const current = dependencyGraph.get(serviceId) ?? [];
      current.push(dep);
      dependencyGraph.set(serviceId, current);
    }
  });
  const cycle = findServiceDependencyCycle(dependencyGraph);
  if (cycle.length > 0) errors.push(`service dependency cycle detected: ${cycle.join(' -> ')}`);

  const seenChannels = new Set<string>();
  let hasBinaryChannel = false;
  (spec.channels ?? []).forEach((channel: PluginChannelSpec, index) => {
    const id = (channel.id ?? '').trim();
    const serviceId = (channel.serviceId ?? '').trim();
    const kind = (channel.kind ?? '').trim();
    if (!id || !kind) {
      errors.push(`channels[${index}] id and kind are required`);
      return;
    }
    const idErr = validateChannelId(id);
    if (idErr) errors.push(`channels[${index}] id: ${idErr}`);
    const schemaIdErr = validateChannelSchemaId((channel.schemaId ?? '').trim());
    if (schemaIdErr) errors.push(`channels[${index}] schemaId: ${schemaIdErr}`);
    if (channel.direction && !['plugin_to_host', 'host_to_plugin', 'bidirectional'].includes(channel.direction)) {
      errors.push(`channels[${index}] direction: invalid channel direction '${channel.direction}'`);
    }
    if (channel.frequencyHint && !['low', 'normal', 'high', 'realtime'].includes(channel.frequencyHint)) {
      errors.push(`channels[${index}] frequencyHint: invalid frequency hint: ${channel.frequencyHint}`);
    }
    if (services.length > 1 && !serviceId) errors.push(`channels[${index}] serviceId is required when multiple services are declared`);
    if (serviceId) {
      const serviceErr = validateServiceId(serviceId);
      if (serviceErr) errors.push(`channels[${index}] serviceId: ${serviceErr}`);
      if (seenServices.size > 0 && !seenServices.has(serviceId)) errors.push(`channel '${id}' references unknown service '${serviceId}'`);
    }

    switch (kind) {
      case 'state':
        if (!seenFeatures.has(HostFeature.STATE_STREAMING)) errors.push(`state channel '${id}' requires hostFeatures to include '${HostFeature.STATE_STREAMING}'`);
        break;
      case 'event':
      case 'log':
      case 'metric':
      case 'custom':
        if (!seenFeatures.has(HostFeature.EVENT_STREAMING)) errors.push(`${kind} channel '${id}' requires hostFeatures to include '${HostFeature.EVENT_STREAMING}'`);
        break;
      case 'binary':
        hasBinaryChannel = true;
        break;
      default:
        errors.push(`channels[${index}] unsupported kind '${kind}'`);
    }

    let channelScope = serviceId;
    if (!channelScope && services.length === 1) channelScope = (services[0].id ?? '').trim();
    const key = `${channelScope}\u0000${id}`;
    if (seenChannels.has(key)) errors.push(`duplicate channel id '${id}' within service '${channelScope}'`);
    seenChannels.add(key);
  });

  const sinks = spec.controlEffectSinks ?? [];
  if (sinks.length > 0 && !seenFeatures.has(HostFeature.REALTIME_CONTROL)) {
    errors.push(`controlEffectSinks require hostFeatures to include '${HostFeature.REALTIME_CONTROL}'`);
  }
  if (hasBinaryChannel && !seenFeatures.has(HostFeature.BINARY_STREAMING)) {
    errors.push(`binary channel requires hostFeatures to include '${HostFeature.BINARY_STREAMING}'`);
  }

  const seenSinks = new Set<string>();
  sinks.forEach((sink, index) => {
    const id = (sink.id ?? '').trim();
    const serviceId = (sink.serviceId ?? '').trim();
    if (!id || !serviceId) {
      errors.push(`controlEffectSinks[${index}] id and serviceId are required`);
      return;
    }
    const serviceErr = validateServiceId(serviceId);
    if (serviceErr) errors.push(`controlEffectSinks[${index}] serviceId: ${serviceErr}`);
    if (seenSinks.has(id)) errors.push(`duplicate control effect sink id '${id}'`);
    seenSinks.add(id);
    if (seenServices.size > 0 && !seenServices.has(serviceId)) errors.push(`control effect sink '${id}' references unknown service '${serviceId}'`);
  });

  errors.push(...validateNetworkPolicy(spec.network));

  const seenArtifacts = new Set<string>();
  for (const artifact of spec.artifacts ?? []) errors.push(...validateArtifact(artifact, seenArtifacts));
  return errors;
}

export function assertPluginHostSpec(spec: unknown): PluginHostSpec {
  const errors = validatePluginHostSpec(spec);
  if (errors.length > 0) {
    throw new Error(`plugin host spec validation failed: ${errors.join('; ')}`);
  }
  return spec as PluginHostSpec;
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
