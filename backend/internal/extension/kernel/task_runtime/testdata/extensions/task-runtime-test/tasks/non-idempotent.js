const { defineTask } = require("@amitia/plugin-sdk");

module.exports = defineTask({
  taskId: "non_idempotent_task",
  idempotent: false,
  handler: async (input, context) => {
    await context.progress.report({ current: 30, total: 100, message: "执行前半部分（模拟副作用）" });
    context.logger.warn("副作用已执行，即将模拟崩溃");
    await new Promise((resolve) => setTimeout(resolve, 100));
    context.logger.error("模拟崩溃，进程即将退出");
    process.exit(1);
  },
});
