window.__ModuleLoader__.load({
  id: "@amitia-test/conversation-node-plugin",
  factory(require) {
    const React = require("react");
    return {
      inject: ["slots"],
      async apply(ctx) {
        ctx.slots.register({
          name: "conversation.chat.node",
          key: "user.text",
          id: "node-user-text",
        }, () => React.createElement("span", null, "user-text"));
        ctx.slots.register({
          name: "conversation.chat.node",
          key: "assistant.reasoning",
          id: "node-reasoning",
        }, () => React.createElement("span", null, "reasoning"));
        ctx.slots.register({
          name: "conversation.chat.node",
          key: "notice.info",
          id: "node-notice",
          order: 15,
        }, ({ content }) => React.createElement("span", null, content.display));
        ctx.slots.register({
          name: "conversation.chat.node",
          key: "tool.status",
          id: "node-tool",
          ordering: 25,
        }, () => React.createElement("span", null, "tool"));
      },
    };
  },
});
