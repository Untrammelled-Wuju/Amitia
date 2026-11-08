import { createServer } from "node:http";

const port = Number(process.env.AMITIA_QA_MODEL_PORT || "61377");
let toolCallSequence = 0;

function completion(content, toolCall) {
  const message = { role: "assistant", content };
  let finishReason = "stop";
  if (toolCall) {
    message.tool_calls = [
      {
        id: `qa-tool-call-${++toolCallSequence}`,
        type: "function",
        function: {
          name: toolCall.name,
          arguments: JSON.stringify({ text: "hello" }),
        },
      },
    ];
    message.content = null;
    finishReason = "tool_calls";
  }
  return {
    id: "chatcmpl-qa",
    object: "chat.completion",
    created: Math.floor(Date.now() / 1000),
    model: "qa-compatible",
    choices: [{ index: 0, message, finish_reason: finishReason }],
    usage: { prompt_tokens: 12, completion_tokens: 8, total_tokens: 20 },
  };
}

createServer((request, response) => {
  if (request.method !== "POST" || !request.url?.endsWith("/chat/completions")) {
    response.writeHead(404, { "Content-Type": "application/json" });
    response.end(JSON.stringify({ error: { message: `unexpected path: ${request.url}` } }));
    return;
  }
  let raw = "";
  request.setEncoding("utf8");
  request.on("data", (chunk) => {
    raw += chunk;
  });
  request.on("end", () => {
    let body;
    try {
      body = JSON.parse(raw);
    } catch (error) {
      response.writeHead(400, { "Content-Type": "application/json" });
      response.end(JSON.stringify({ error: { message: `invalid json: ${error.message}` } }));
      return;
    }
    const messages = Array.isArray(body.messages) ? body.messages : [];
    const text = JSON.stringify(messages);
    if (text.includes("你是 Core 的记忆计算服务") || text.includes("当前 Core 的待审核记忆提取服务")) {
      response.writeHead(200, { "Content-Type": "application/json" });
      response.end(JSON.stringify(completion(JSON.stringify({ items: [] }))));
      return;
    }
    if (text.includes("QA-MCP:hello")) {
      response.writeHead(200, { "Content-Type": "application/json" });
      response.end(JSON.stringify(completion("QA_MCP_TOOL_OK")));
      return;
    }
    if (messages.some((message) => String(message.content || "").includes("QA_USE_MCP"))) {
      const tools = Array.isArray(body.tools) ? body.tools : [];
      const name = tools.find((tool) => String(tool?.function?.name || "").includes("echo"))?.function?.name;
      if (!name) {
        response.writeHead(200, { "Content-Type": "application/json" });
        response.end(JSON.stringify(completion("NO_TOOL_AVAILABLE")));
        return;
      }
      response.writeHead(200, { "Content-Type": "application/json" });
      response.end(JSON.stringify(completion(null, { name })));
      return;
    }
    response.writeHead(200, { "Content-Type": "application/json" });
    response.end(JSON.stringify(completion("QA_MODEL_OK")));
  });
}).listen(port, "127.0.0.1", () => {
  process.stdout.write(`qa-openai-compatible http://127.0.0.1:${port}\n`);
});
