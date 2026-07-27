const { defineTask, successTaskResult } = require("@amitia/plugin-sdk");

module.exports = defineTask({
  taskId: "timeout_task",
  handler: async (input, context) => {
    for (let i = 0; i < 1000; i++) {
      await context.progress.report({ current: i, total: 1000, message: `步骤 ${i}` });
      await new Promise((resolve) => setTimeout(resolve, 1000));
    }
    return successTaskResult({ completed: true });
  },
});
