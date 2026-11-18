import assert from "node:assert/strict";
import test from "node:test";
import {
  dshOutputToJsonSchema,
  dshParametersToJsonSchema,
  mapDshTool,
} from "../src/runtime/tool-mapping.mjs";

test("maps DSH parameter DSL to Amitia JSON Schema", () => {
  assert.deepEqual(dshParametersToJsonSchema({
    path: { type: "string", required: true, description: "Absolute path" },
    limit: { type: "number" },
  }), {
    type: "object",
    properties: {
      path: { type: "string", description: "Absolute path" },
      limit: { type: "number" },
    },
    required: ["path"],
    additionalProperties: false,
  });
});

test("creates stable Amitia tool contribution descriptors", () => {
  const mapped = mapDshTool({
    pluginId: "@example/dsh-tool",
    version: "1.2.3",
  }, {
    name: "read-file",
    title: "Read file",
    description: "Read a file",
    parameters: { path: { type: "string", required: true } },
    output: { schema: { type: "string" } },
    riskLevel: "low",
    sideEffect: "read_only",
  });
  assert.equal(mapped.descriptor.kind, "tool");
  assert.equal(mapped.descriptor.adapter, "tool");
  assert.equal(mapped.descriptor.spec.handlerName, mapped.handlerName);
  assert.equal(mapped.descriptor.spec.toolId, mapped.toolId);
  assert.equal(mapped.descriptor.metadata.dshToolName, "read-file");
  assert.match(mapped.descriptor.spec.modelName, /^dsh_[a-f0-9]{10}_read_file$/);
});

test("normalizes nested DSH required markers into JSON Schema required arrays", () => {
  const parameters = dshParametersToJsonSchema({
    todos: {
      type: "array",
      required: true,
      items: {
        type: "object",
        additionalProperties: false,
        properties: {
          content: { type: "string", required: true },
          status: {
            type: "string",
            required: true,
            enum: ["pending", "in_progress", "completed"],
          },
        },
      },
    },
  });
  assert.deepEqual(parameters, {
    type: "object",
    properties: {
      todos: {
        type: "array",
        items: {
          type: "object",
          additionalProperties: false,
          properties: {
            content: { type: "string" },
            status: { type: "string", enum: ["pending", "in_progress", "completed"] },
          },
          required: ["content", "status"],
        },
      },
    },
    required: ["todos"],
    additionalProperties: false,
  });
});

test("normalizes official DSH output schemas", () => {
  const output = dshOutputToJsonSchema({
    schema: {
      type: "object",
      additionalProperties: false,
      properties: {
        todos: {
          type: "array",
          required: true,
          items: {
            type: "object",
            additionalProperties: false,
            properties: {
              content: { type: "string", required: true },
              status: { type: "string", required: true },
            },
          },
        },
        counts: {
          type: "object",
          additionalProperties: false,
          required: true,
          properties: {
            pending: { type: "integer", required: true },
            inProgress: { type: "integer", required: true },
            completed: { type: "integer", required: true },
          },
        },
      },
    },
  });
  assert.deepEqual(output.required, ["todos", "counts"]);
  assert.deepEqual(output.properties.todos.items.required, ["content", "status"]);
  assert.deepEqual(output.properties.counts.required, ["pending", "inProgress", "completed"]);
  assert.equal(output.properties.counts.properties.pending.required, undefined);
});
