export type ErrorCategory =
  | "permission"
  | "scope"
  | "validation"
  | "conflict"
  | "timeout"
  | "cancelled"
  | "dependency"
  | "runtime"
  | "rate_limit"
  | "storage"
  | "internal";

export interface AmitiaErrorDetails {
  readonly code: string;
  readonly category: ErrorCategory;
  readonly retryable: boolean;
  readonly details?: Record<string, unknown>;
}

export class AmitiaError extends Error {
  readonly code: string;
  readonly category: ErrorCategory;
  readonly retryable: boolean;
  readonly details?: Record<string, unknown>;

  constructor(message: string, init: AmitiaErrorDetails) {
    super(message);
    this.name = "AmitiaError";
    this.code = init.code;
    this.category = init.category;
    this.retryable = init.retryable;
    this.details = init.details;
  }

  static from(cause: unknown): AmitiaError {
    if (cause instanceof AmitiaError) return cause;
    if (cause instanceof Error) {
      return new InternalError(cause.message, { cause });
    }
    return new InternalError(String(cause));
  }
}

export interface AmitiaErrorInit {
  code: string;
  retryable?: boolean;
  details?: Record<string, unknown>;
  cause?: unknown;
}

type SpecializedErrorInit = Omit<AmitiaErrorInit, "code"> & { code?: string };

export class PermissionDeniedError extends AmitiaError {
  constructor(message: string, init?: SpecializedErrorInit) {
    super(message, {
      code: init?.code ?? "permission_denied",
      category: "permission",
      retryable: init?.retryable ?? false,
      details: init?.details,
    });
    this.name = "PermissionDeniedError";
  }
}

export class ScopeDeniedError extends AmitiaError {
  constructor(message: string, init?: SpecializedErrorInit) {
    super(message, {
      code: init?.code ?? "scope_denied",
      category: "scope",
      retryable: init?.retryable ?? false,
      details: init?.details,
    });
    this.name = "ScopeDeniedError";
  }
}

export class ValidationError extends AmitiaError {
  constructor(message: string, init?: SpecializedErrorInit) {
    super(message, {
      code: init?.code ?? "invalid_input",
      category: "validation",
      retryable: init?.retryable ?? false,
      details: init?.details,
    });
    this.name = "ValidationError";
  }
}

export class ConflictError extends AmitiaError {
  constructor(message: string, init?: SpecializedErrorInit) {
    super(message, {
      code: init?.code ?? "conflict",
      category: "conflict",
      retryable: init?.retryable ?? false,
      details: init?.details,
    });
    this.name = "ConflictError";
  }
}

export class TimeoutError extends AmitiaError {
  constructor(message: string, init?: SpecializedErrorInit) {
    super(message, {
      code: init?.code ?? "timeout",
      category: "timeout",
      retryable: init?.retryable ?? true,
      details: init?.details,
    });
    this.name = "TimeoutError";
  }
}

export class CancelledError extends AmitiaError {
  constructor(message: string, init?: SpecializedErrorInit) {
    super(message, {
      code: init?.code ?? "cancelled",
      category: "cancelled",
      retryable: init?.retryable ?? false,
      details: init?.details,
    });
    this.name = "CancelledError";
  }
}

export class DependencyUnavailableError extends AmitiaError {
  constructor(message: string, init?: SpecializedErrorInit) {
    super(message, {
      code: init?.code ?? "dependency_unavailable",
      category: "dependency",
      retryable: init?.retryable ?? true,
      details: init?.details,
    });
    this.name = "DependencyUnavailableError";
  }
}

export class RuntimeUnavailableError extends AmitiaError {
  constructor(message: string, init?: SpecializedErrorInit) {
    super(message, {
      code: init?.code ?? "runtime_unavailable",
      category: "runtime",
      retryable: init?.retryable ?? true,
      details: init?.details,
    });
    this.name = "RuntimeUnavailableError";
  }
}

export class RateLimitError extends AmitiaError {
  constructor(message: string, init?: SpecializedErrorInit) {
    super(message, {
      code: init?.code ?? "rate_limit",
      category: "rate_limit",
      retryable: init?.retryable ?? true,
      details: init?.details,
    });
    this.name = "RateLimitError";
  }
}

export class StorageConflictError extends AmitiaError {
  constructor(message: string, init?: SpecializedErrorInit) {
    super(message, {
      code: init?.code ?? "storage_conflict",
      category: "storage",
      retryable: init?.retryable ?? false,
      details: init?.details,
    });
    this.name = "StorageConflictError";
  }
}

export class InternalError extends AmitiaError {
  constructor(message: string, init?: SpecializedErrorInit) {
    super(message, {
      code: init?.code ?? "internal_error",
      category: "internal",
      retryable: init?.retryable ?? false,
      details: init?.details,
    });
    this.name = "InternalError";
  }
}

export function isAmitiaError(cause: unknown): cause is AmitiaError {
  return cause instanceof AmitiaError;
}

export function isRetryable(cause: unknown): boolean {
  if (cause instanceof AmitiaError) return cause.retryable;
  return false;
}
