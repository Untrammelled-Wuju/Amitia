import type { ToolContributionDefinition, JSONSchema } from "./manifest";
import type { RuntimeContext } from "./runtime";

export interface ToolHandlerResult {
  ok: boolean;
  result?: unknown;
  error?: { code: string; message: string };
  audit?: Record<string, unknown>;
}

export interface ToolHandlerContext {
  runtime: RuntimeContext;
  toolCallId: string;
  idempotencyKey?: string;
  deadline?: number;
}

export type ToolHandler = (input: unknown, ctx: ToolHandlerContext) => Promise<ToolHandlerResult> | ToolHandlerResult;

export interface ToolRegistration {
  definition: ToolContributionDefinition;
  handler: ToolHandler;
}

const toolRegistry = new Map<string, ToolRegistration>();

export function defineTool(
  definition: ToolContributionDefinition,
  handler: ToolHandler
): ToolRegistration {
  const registration: ToolRegistration = { definition, handler };
  toolRegistry.set(definition.toolId, registration);
  return registration;
}

export function getTool(toolId: string): ToolRegistration | undefined {
  return toolRegistry.get(toolId);
}

export function listTools(): ToolRegistration[] {
  return Array.from(toolRegistry.values());
}

export function unregisterTool(toolId: string): boolean {
  return toolRegistry.delete(toolId);
}

export function clearTools(): void {
  toolRegistry.clear();
}

export function validateToolInput(
  input: unknown,
  schema?: JSONSchema
): { ok: boolean; errors: string[] } {
  if (!schema) return { ok: true, errors: [] };
  const errors: string[] = [];
  validateValue(input, schema, "", errors);
  return { ok: errors.length === 0, errors };
}

function validateValue(value: unknown, schema: JSONSchema, path: string, errors: string[]): void {
  if (schema.type) {
    const actualType = Array.isArray(value) ? "array" : typeof value;
    if (actualType !== schema.type) {
      errors.push(`${path || "root"}: expected ${schema.type}, got ${actualType}`);
      return;
    }
  }
  if (schema.enum && !schema.enum.includes(value)) {
    errors.push(`${path || "root"}: value ${String(value)} not in enum`);
  }
  if (schema.type === "object" && schema.properties) {
    const obj = value as Record<string, unknown>;
    if (schema.required) {
      for (const req of schema.required) {
        if (!(req in obj)) {
          errors.push(`${path}.${req}: required field missing`);
        }
      }
    }
    for (const [key, subSchema] of Object.entries(schema.properties)) {
      if (key in obj) {
        validateValue(obj[key], subSchema, path ? `${path}.${key}` : key, errors);
      }
    }
  }
  if (schema.type === "string") {
    const str = value as string;
    if (schema.minLength !== undefined && str.length < schema.minLength) {
      errors.push(`${path}: min length ${schema.minLength}`);
    }
    if (schema.maxLength !== undefined && str.length > schema.maxLength) {
      errors.push(`${path}: max length ${schema.maxLength}`);
    }
    if (schema.pattern) {
      const re = new RegExp(schema.pattern);
      if (!re.test(str)) {
        errors.push(`${path}: pattern mismatch`);
      }
    }
  }
  if (schema.type === "number" || schema.type === "integer") {
    const num = value as number;
    if (schema.minimum !== undefined && num < schema.minimum) {
      errors.push(`${path}: min ${schema.minimum}`);
    }
    if (schema.maximum !== undefined && num > schema.maximum) {
      errors.push(`${path}: max ${schema.maximum}`);
    }
  }
  if (schema.type === "array" && schema.items && Array.isArray(value)) {
    const arr = value as unknown[];
    for (let i = 0; i < arr.length; i++) {
      validateValue(arr[i], schema.items, `${path}[${i}]`, errors);
    }
  }
}

export function successResult(result: unknown, audit?: Record<string, unknown>): ToolHandlerResult {
  return { ok: true, result, audit };
}

export function errorResult(code: string, message: string): ToolHandlerResult {
  return { ok: false, error: { code, message } };
}
