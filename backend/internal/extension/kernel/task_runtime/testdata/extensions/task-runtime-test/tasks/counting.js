const { defineTask, successTaskResult } = require("@amitia/plugin-sdk");

module.exports = defineTask({
  taskId: "counting_task",
  handler: async (input, context) => {
    for (let i = 0; i <= 100; i += 10) {
      await context.progress.report({ current: i, total: 100, message: "处理中" });
      await new Promise((resolve) => setTimeout(resolve, 100));
    }
    return successTaskResult({ count: 100 });
  },
});
