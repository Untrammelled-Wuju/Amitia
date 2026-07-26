import { AmitiaError, ValidationError, PermissionDeniedError } from "./errors";

export interface SecretReference {
  readonly referenceId: string;
  readonly name: string;
  readonly version: number;
  readonly createdAt: string;
  readonly rotationPolicy?: string;
  readonly expiresAt?: string;
}

export interface SecretCreateRequest {
  readonly name: string;
  readonly value: string;
  readonly scope?: "extension" | "character" | "conversation";
  readonly rotationPolicy?: "never" | "daily" | "weekly" | "monthly";
  readonly expiresAt?: string;
  readonly metadata?: Record<string, unknown>;
}

export interface SecretLeaseHandle {
  readonly leaseId: string;
  readonly reference: SecretReference;
  readonly purpose: string;
  readonly expiresAt: string;
  readonly getValue(): string;
  readonly renew(durationMs: number): Promise<void>;
  readonly release(): Promise<void>;
}

export interface SecretReferenceClient {
  create(request: SecretCreateRequest): Promise<SecretReference>;
  use<T>(
    reference: SecretReference,
    purpose: string,
    callback: (lease: SecretLeaseHandle) => Promise<T>,
  ): Promise<T>;
  revoke(reference: SecretReference): Promise<void>;
  list(): Promise<SecretReference[]>;
  rotate(reference: SecretReference, newValue: string): Promise<SecretReference>;
}

export interface SecretBackend {
  create(namespace: string, request: SecretCreateRequest): Promise<SecretReference>;
  lease(namespace: string, reference: SecretReference, purpose: string, ttlMs: number): Promise<SecretLeaseHandle>;
  revoke(namespace: string, reference: SecretReference): Promise<void>;
  list(namespace: string): Promise<SecretReference[]>;
  rotate(namespace: string, reference: SecretReference, newValue: string): Promise<SecretReference>;
}

const DEFAULT_LEASE_TTL_MS = 60_000;
const MAX_SECRET_VALUE_LENGTH = 64 * 1024;

export function assertSecretNameValid(name: string): void {
  if (!name) {
    throw new ValidationError("secret name must not be empty");
  }
  if (name.length > 128) {
    throw new ValidationError("secret name must not exceed 128 characters");
  }
  if (!/^[a-zA-Z0-9._\-]+$/.test(name)) {
    throw new ValidationError("secret name contains invalid characters");
  }
}

export function assertSecretValueValid(value: string): void {
  if (!value) {
    throw new ValidationError("secret value must not be empty");
  }
  if (value.length > MAX_SECRET_VALUE_LENGTH) {
    throw new ValidationError(`secret value must not exceed ${MAX_SECRET_VALUE_LENGTH} characters`);
  }
}

export function assertPurposeValid(purpose: string): void {
  if (!purpose) {
    throw new ValidationError("secret purpose must not be empty");
  }
  if (purpose.length > 256) {
    throw new ValidationError("secret purpose must not exceed 256 characters");
  }
}

export class NamespacedSecretClient implements SecretReferenceClient {
  constructor(
    private readonly namespace: string,
    private readonly backend: SecretBackend,
  ) {}

  async create(request: SecretCreateRequest): Promise<SecretReference> {
    assertSecretNameValid(request.name);
    assertSecretValueValid(request.value);
    return this.backend.create(this.namespace, request);
  }

  async use<T>(
    reference: SecretReference,
    purpose: string,
    callback: (lease: SecretLeaseHandle) => Promise<T>,
  ): Promise<T> {
    assertPurposeValid(purpose);
    const lease = await this.backend.lease(this.namespace, reference, purpose, DEFAULT_LEASE_TTL_MS);
    try {
      return await callback(lease);
    } finally {
      try {
        await lease.release();
      } catch {
        // ignore release errors
      }
    }
  }

  async revoke(reference: SecretReference): Promise<void> {
    return this.backend.revoke(this.namespace, reference);
  }

  async list(): Promise<SecretReference[]> {
    return this.backend.list(this.namespace);
  }

  async rotate(reference: SecretReference, newValue: string): Promise<SecretReference> {
    assertSecretValueValid(newValue);
    return this.backend.rotate(this.namespace, reference, newValue);
  }
}

export function mapSecretError(cause: unknown): AmitiaError {
  if (cause instanceof AmitiaError) return cause;
  const message = cause instanceof Error ? cause.message : String(cause);
  if (/permission|denied|forbidden/i.test(message)) {
    return new PermissionDeniedError(message);
  }
  return new ValidationError(message);
}

export class InMemorySecretBackend implements SecretBackend {
  private readonly secrets = new Map<string, { value: string; reference: SecretReference }>();
  private readonly leases = new Map<string, SecretLeaseHandle>();

  async create(namespace: string, request: SecretCreateRequest): Promise<SecretReference> {
    const referenceId = `${namespace}/${request.name}`;
    if (this.secrets.has(referenceId)) {
      throw new ValidationError(`secret ${request.name} already exists`);
    }
    const reference: SecretReference = {
      referenceId,
      name: request.name,
      version: 1,
      createdAt: new Date().toISOString(),
      rotationPolicy: request.rotationPolicy,
      expiresAt: request.expiresAt,
    };
    this.secrets.set(referenceId, { value: request.value, reference });
    return reference;
  }

  async lease(
    namespace: string,
    reference: SecretReference,
    purpose: string,
    ttlMs: number,
  ): Promise<SecretLeaseHandle> {
    const entry = this.secrets.get(reference.referenceId);
    if (!entry) {
      throw new ValidationError(`secret ${reference.name} not found`);
    }
    const leaseId = `${reference.referenceId}:${purpose}:${Date.now()}`;
    const expiresAt = new Date(Date.now() + ttlMs).toISOString();
    const handle: SecretLeaseHandle = {
      leaseId,
      reference: entry.reference,
      purpose,
      expiresAt,
      getValue: () => entry.value,
      renew: async (durationMs: number) => {
        // in-memory: no-op, expiry is only checked on access
      },
      release: async () => {
        this.leases.delete(leaseId);
      },
    };
    this.leases.set(leaseId, handle);
    return handle;
  }

  async revoke(namespace: string, reference: SecretReference): Promise<void> {
    this.secrets.delete(reference.referenceId);
  }

  async list(namespace: string): Promise<SecretReference[]> {
    const refs: SecretReference[] = [];
    for (const [id, entry] of this.secrets.entries()) {
      if (id.startsWith(`${namespace}/`)) {
        refs.push(entry.reference);
      }
    }
    return refs;
  }

  async rotate(namespace: string, reference: SecretReference, newValue: string): Promise<SecretReference> {
    const entry = this.secrets.get(reference.referenceId);
    if (!entry) {
      throw new ValidationError(`secret ${reference.name} not found`);
    }
    const updated: SecretReference = {
      ...entry.reference,
      version: entry.reference.version + 1,
    };
    this.secrets.set(reference.referenceId, { value: newValue, reference: updated });
    return updated;
  }
}
