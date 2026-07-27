const { defineTask, successTaskResult, failureTaskResult } = require("@amitia/plugin-sdk");

module.exports = defineTask({
  taskId: "cancel_task",
  handler: async (input, context) => {
    for (let i = 0; i < 1000; i++) {
      if (context.signal.aborted) {
        context.logger.info("收到取消信号，执行清理并退出", { step: i });
        return failureTaskResult("cancelled", "task cancelled by signal");
      }
      await context.progress.report({ current: i, total: 1000, message: `步骤 ${i}` });
      await new Promise((resolve) => setTimeout(resolve, 500));
    }
    return successTaskResult({ completed: true });
  },
});
