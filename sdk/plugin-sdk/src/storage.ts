import { AmitiaError, ValidationError, StorageConflictError, RateLimitError } from "./errors";

export type StorageScope = "extension" | "module" | "character" | "conversation" | "invocation";

export interface StorageValue<T = unknown> {
  readonly key: string;
  readonly value: T;
  readonly version: number;
  readonly updatedAt: string;
  readonly scope: StorageScope;
  readonly etag?: string;
}

export interface StorageReadOptions {
  readonly scope?: StorageScope;
  readonly version?: number;
  readonly ifNoneMatch?: string;
}

export interface StorageDeleteOptions {
  readonly scope?: StorageScope;
  readonly ifMatch?: string;
  readonly version?: number;
}

export interface StorageListQuery {
  readonly prefix?: string;
  readonly scope?: StorageScope;
  readonly limit?: number;
  readonly cursor?: string;
  readonly includeValues?: boolean;
}

export interface StoragePage<T = unknown> {
  readonly items: StorageValue<T>[];
  readonly nextCursor?: string;
  readonly hasMore: boolean;
}

export interface StorageCASRequest<T = unknown> {
  readonly key: string;
  readonly expectedVersion?: number;
  readonly expectedEtag?: string;
  readonly newValue: T;
  readonly scope?: StorageScope;
}

export interface StorageTransactionContext {
  get<T>(key: string, options?: StorageReadOptions): Promise<StorageValue<T> | null>;
  set<T>(key: string, value: T, options?: StorageSetOptions): Promise<StorageValue<T>>;
  delete(key: string, options?: StorageDeleteOptions): Promise<void>;
  list<T>(query?: StorageListQuery): Promise<StoragePage<T>>;
}

export type StorageTransactionCallback<T> = (ctx: StorageTransactionContext) => Promise<T>;

export interface StorageSetOptions {
  readonly scope?: StorageScope;
  readonly ttlMs?: number;
  readonly ifNotExists?: boolean;
  readonly metadata?: Record<string, unknown>;
}

export interface StorageClient {
  get<T>(key: string, options?: StorageReadOptions): Promise<StorageValue<T> | null>;
  set<T>(key: string, value: T, options?: StorageSetOptions): Promise<StorageValue<T>>;
  compareAndSwap<T>(request: StorageCASRequest<T>): Promise<StorageValue<T>>;
  delete(key: string, options?: StorageDeleteOptions): Promise<void>;
  list<T>(query?: StorageListQuery): Promise<StoragePage<T>>;
  transaction<T>(callback: StorageTransactionCallback<T>): Promise<T>;
}

export interface StorageBackend {
  get(namespace: string, key: string, options?: StorageReadOptions): Promise<StorageValue | null>;
  set(namespace: string, key: string, value: unknown, options?: StorageSetOptions): Promise<StorageValue>;
  cas(namespace: string, request: StorageCASRequest): Promise<StorageValue>;
  delete(namespace: string, key: string, options?: StorageDeleteOptions): Promise<void>;
  list(namespace: string, query?: StorageListQuery): Promise<StoragePage>;
  transaction(namespace: string, callback: StorageTransactionCallback<unknown>): Promise<unknown>;
}

const SECRET_KEY_PATTERNS = [
  /secret/i,
  /password/i,
  /token/i,
  /credential/i,
  /api[-_]?key/i,
  /private[-_]?key/i,
  /access[-_]?key/i,
];

export function assertNotSecretKey(key: string): void {
  for (const pattern of SECRET_KEY_PATTERNS) {
    if (pattern.test(key)) {
      throw new ValidationError(
        `key '${key}' looks like a secret; use the Secret client instead`,
        { details: { key } },
      );
    }
  }
}

export function assertKeyValid(key: string): void {
  if (!key) {
    throw new ValidationError("storage key must not be empty");
  }
  if (key.length > 256) {
    throw new ValidationError("storage key must not exceed 256 characters");
  }
  if (!/^[a-zA-Z0-9._\-/]+$/.test(key)) {
    throw new ValidationError("storage key contains invalid characters");
  }
  if (key.startsWith("/") || key.endsWith("/")) {
    throw new ValidationError("storage key must not start or end with '/'");
  }
}

export function mapStorageError(cause: unknown): AmitiaError {
  if (cause instanceof AmitiaError) return cause;
  const message = cause instanceof Error ? cause.message : String(cause);
  if (/conflict|version|etag/i.test(message)) {
    return new StorageConflictError(message);
  }
  if (/quota|rate|limit/i.test(message)) {
    return new RateLimitError(message);
  }
  return new ValidationError(message);
}

export function namespacedKey(namespace: string, key: string): string {
  return `${namespace}::${key}`;
}

export class NamespacedStorageClient implements StorageClient {
  constructor(
    private readonly namespace: string,
    private readonly backend: StorageBackend,
  ) {}

  async get<T>(key: string, options?: StorageReadOptions): Promise<StorageValue<T> | null> {
    assertKeyValid(key);
    assertNotSecretKey(key);
    const entry = await this.backend.get<T>(this.namespace, key, options);
    return entry;
  }

  async set<T>(key: string, value: T, options?: StorageSetOptions): Promise<StorageValue<T>> {
    assertKeyValid(key);
    assertNotSecretKey(key);
    return this.backend.set<T>(this.namespace, key, value, options);
  }

  async compareAndSwap<T>(request: StorageCASRequest<T>): Promise<StorageValue<T>> {
    assertKeyValid(request.key);
    assertNotSecretKey(request.key);
    return this.backend.cas<T>(this.namespace, request);
  }

  async delete(key: string, options?: StorageDeleteOptions): Promise<void> {
    assertKeyValid(key);
    return this.backend.delete(this.namespace, key, options);
  }

  async list<T>(query?: StorageListQuery): Promise<StoragePage<T>> {
    return this.backend.list<T>(this.namespace, query);
  }

  async transaction<T>(callback: StorageTransactionCallback<T>): Promise<T> {
    const result = await this.backend.transaction<T>(this.namespace, callback as StorageTransactionCallback<unknown>);
    return result as T;
  }
}
