export class JsonSchemaError extends Error {
  constructor(message, details) {
    super(message);
    this.name = "JsonSchemaError";
    this.code = "JSON_SCHEMA_INVALID";
    this.details = details;
  }
}

export class ToolArgsError extends Error {
  constructor(messages) {
    const list = Array.isArray(messages) ? messages : [String(messages)];
    super(list.join("; "));
    this.name = "ToolArgsError";
    this.code = "TOOL_ARGS_INVALID";
    this.details = { messages: list };
  }
}

export class ToolContextUnavailableError extends Error {
  constructor(message) {
    super(message);
    this.name = "ToolContextUnavailableError";
    this.code = "TOOL_CONTEXT_UNAVAILABLE";
  }
}

export class ToolExecutorUnavailableError extends Error {
  constructor(message) {
    super(message);
    this.name = "ToolExecutorUnavailableError";
    this.code = "TOOL_EXECUTOR_UNAVAILABLE";
  }
}

export class ToolActivationError extends Error {
  constructor(message) {
    super(message);
    this.name = "ToolActivationError";
    this.code = "TOOL_ACTIVATION_FAILED";
  }
}

export class ToolNotFoundError extends Error {
  constructor(message) {
    super(message);
    this.name = "ToolNotFoundError";
    this.code = "TOOL_NOT_FOUND";
  }
}

export class ToolOutputError extends Error {
  constructor(message) {
    super(message);
    this.name = "ToolOutputError";
    this.code = "TOOL_OUTPUT_INVALID";
  }
}

export class ToolBatchExecutionError extends Error {
  constructor(message) {
    super(message);
    this.name = "ToolBatchExecutionError";
    this.code = "TOOL_BATCH_FAILED";
  }
}

export const RUN_CODE_NAME = "run_code";
export const TOOL_ABORTED = "TOOL_ABORTED";
export const TOOL_ABORTED_BEFORE_DISPATCH = "TOOL_ABORTED_BEFORE_DISPATCH";

export function defineTool(options) {
  if (!options || typeof options !== "object") throw new Error("defineTool requires an object");
  if (typeof options.name !== "string" || !options.name.trim()) throw new Error("defineTool requires a name");
  if (typeof options.execute !== "function") throw new Error(`defineTool(${options.name}) requires execute`);
  return options;
}

export function parameterSchemaSpecToJsonSchema(spec) {
  return spec;
}

export function projectToolSchema(tool) {
  return {
    name: tool.name,
    description: tool.description,
    parameters: tool.parameters,
  };
}

export function validateArgs() {
  return undefined;
}

export function validateJsonSchemaValue() {
  return undefined;
}

export function assertSupportedJsonSchema(schema) {
  return schema;
}

export function assertObjectJsonSchema(schema) {
  return schema;
}

export function isJsonSchemaRecord(value) {
  return Boolean(value && typeof value === "object" && !Array.isArray(value));
}

export function isPlainJsonRecord(value) {
  return Boolean(value && typeof value === "object" && !Array.isArray(value));
}

export function isPlainJsonArray(value) {
  return Array.isArray(value);
}

export class ToolsService {
  constructor() {
    this.definitions = new Map();
  }

  register(definition) {
    if (!definition || !definition.name) throw new Error("tool definition requires a name");
    this.definitions.set(definition.name, definition);
    return () => this.definitions.delete(definition.name);
  }

  get(name) {
    return this.definitions.get(name);
  }

  list() {
    return Array.from(this.definitions.values());
  }
}

export const ToolRuntime = ToolsService;
export default ToolsService;
