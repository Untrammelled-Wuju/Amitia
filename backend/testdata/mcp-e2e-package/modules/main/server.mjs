import readline from "node:readline";

const input = readline.createInterface({
  input: process.stdin,
  crlfDelay: Infinity,
});

const respond = (id, result) => {
  process.stdout.write(
    `${JSON.stringify({ jsonrpc: "2.0", id, result })}\n`,
  );
};

input.on("line", (line) => {
  if (!line.trim()) return;
  const message = JSON.parse(line);
  if (message.method === "notifications/initialized") return;
  switch (message.method) {
    case "initialize":
      respond(message.id, {
        protocolVersion: "2025-11-25",
        capabilities: { tools: {} },
        serverInfo: { name: "amitia-e2e", version: "1.0.0" },
      });
      return;
    case "ping":
      respond(message.id, {});
      return;
    case "tools/list":
      respond(message.id, {
        tools: [
          {
            name: "echo",
            title: "Echo",
            description: "Echo a value through the real MCP package runtime",
            inputSchema: {
              type: "object",
              properties: { value: { type: "string" } },
              required: ["value"],
            },
            annotations: {
              readOnlyHint: true,
              idempotentHint: true,
            },
          },
        ],
      });
      return;
    case "tools/call":
      respond(message.id, {
        content: [
          {
            type: "text",
            text: String(message.params?.arguments?.value ?? ""),
          },
        ],
        structuredContent: {
          ok: true,
          value: message.params?.arguments?.value ?? "",
        },
      });
      return;
    default:
      process.stdout.write(
        `${JSON.stringify({
          jsonrpc: "2.0",
          id: message.id,
          error: { code: -32601, message: "method not found" },
        })}\n`,
      );
  }
});
