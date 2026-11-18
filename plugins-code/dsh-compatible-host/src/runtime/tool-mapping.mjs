import { createHash } from "node:crypto";

function text(value) {
  return value == null ? "" : String(value).trim();
}

function safeIdentifier(value, fallback = "tool") {
  const normalized = text(value).replace(/[^A-Za-z0-9_]+/g, "_").replace(/^_+|_+$/g, "");
  return normalized || fallback;
}

function schemaType(value) {
  const type = text(value);
  return ["string", "number", "integer", "boolean", "array", "object", "null"].includes(type) ? type : "string";
}

function schemaRecord(value) {
  return value && typeof value === "object" && !Array.isArray(value) ? value : {};
}

export function normalizeDshSchema(value) {
  if (Array.isArray(value)) return value.map((item) => normalizeDshSchema(item));
  if (!value || typeof value !== "object") return value;
  const required = new Set(
    Array.isArray(value.required)
      ? value.required.filter((item) => typeof item === "string" && item)
      : [],
  );
  const normalized = {};
  for (const [key, raw] of Object.entries(value)) {
    if (key === "required") continue;
    if (key === "properties") {
      const properties = {};
      for (const [name, property] of Object.entries(schemaRecord(raw))) {
        if (property && typeof property === "object" && property.required === true) required.add(name);
        properties[name] = normalizeDshSchema(property);
      }
      normalized.properties = properties;
      continue;
    }
    normalized[key] = normalizeDshSchema(raw);
  }
  if (required.size) normalized.required = [...required];
  return normalized;
}

export function dshParametersToJsonSchema(parameters) {
  if (parameters && typeof parameters === "object" && !Array.isArray(parameters)) {
    if (text(parameters.type) === "object" && parameters.properties) return normalizeDshSchema(parameters);
    const properties = {};
    const required = [];
    for (const [name, raw] of Object.entries(parameters)) {
      const spec = raw && typeof raw === "object" && !Array.isArray(raw) ? raw : {};
      const property = normalizeDshSchema({
        ...spec,
        type: text(spec.type) ? schemaType(spec.type) : undefined,
      });
      if (property.type === undefined) delete property.type;
      properties[name] = property;
      if (spec.required === true) required.push(name);
    }
    return {
      type: "object",
      properties,
      ...(required.length ? { required } : {}),
      additionalProperties: false,
    };
  }
  return { type: "object", properties: {}, additionalProperties: false };
}

export function dshOutputToJsonSchema(output) {
  const schema = output?.schema;
  return schema && typeof schema === "object" && !Array.isArray(schema)
    ? normalizeDshSchema(schema)
    : { type: "object", additionalProperties: true };
}

export function mapDshTool(plugin, tool) {
  const toolName = text(tool?.name);
  if (!toolName) throw new Error(`DSH 插件 ${plugin.pluginId} 注册了缺少 name 的工具`);
  const hash = createHash("sha256").update(`${plugin.pluginId}@${plugin.version}`).digest("hex").slice(0, 10);
  const safeToolName = safeIdentifier(toolName);
  const contributionId = `dsh.${hash}.${safeToolName}`;
  const toolId = `dsh/${hash}/${safeToolName}`;
  const handlerName = `dsh__${hash}__${safeToolName}`;
  const outputSchema = dshOutputToJsonSchema(tool.output);
  const sideEffect = ["read_only", "write", "external", "system"].includes(text(tool.sideEffect))
    ? text(tool.sideEffect)
    : "external";
  return {
    contributionId,
    toolId,
    handlerName,
    descriptor: {
      id: contributionId,
      kind: "tool",
      adapter: "tool",
      contractVersion: 1,
      name: {
        default: text(tool.title) || toolName,
      },
      description: {
        default: text(tool.description),
      },
      spec: {
        toolId,
        modelName: `dsh_${hash}_${safeToolName}`.slice(0, 64),
        handlerName,
        inputSchema: dshParametersToJsonSchema(tool.parameters ?? tool.inputSchema),
        outputSchema,
        riskLevel: ["low", "medium", "high", "critical"].includes(text(tool.riskLevel)) ? text(tool.riskLevel) : "medium",
        sideEffect,
        runtime: {
          runtimeType: "javascript",
          handlerName,
        },
      },
      metadata: {
        dshPluginId: plugin.pluginId,
        dshPluginVersion: plugin.version,
        dshToolName: toolName,
      },
    },
  };
}
