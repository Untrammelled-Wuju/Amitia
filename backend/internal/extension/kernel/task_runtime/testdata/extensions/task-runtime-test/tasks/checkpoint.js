const { defineTask, successTaskResult } = require("@amitia/plugin-sdk");

module.exports = defineTask({
  taskId: "checkpoint_task",
  handler: async (input, context) => {
    let startStep = 0;

    const checkpointEnv = process.env.AMITIA_TASK_CHECKPOINT;
    if (checkpointEnv) {
      try {
        const parsed = JSON.parse(checkpointEnv);
        if (parsed && typeof parsed.cursor === "number") {
          startStep = parsed.cursor;
          context.logger.info("从环境变量 checkpoint 恢复", { cursor: startStep });
        }
      } catch (err) {
        context.logger.warn("环境变量 checkpoint 解析失败，从头开始", { error: err.message });
      }
    } else {
      const loaded = await context.checkpoint.load();
      if (loaded && typeof loaded.cursor === "number") {
        startStep = loaded.cursor;
        context.logger.info("从 context.checkpoint 恢复", { cursor: startStep });
      }
    }

    for (let step = startStep; step < 10; step++) {
      await context.progress.report({
        current: step + 1,
        total: 10,
        message: `处理第 ${step + 1} 步`,
      });

      if ((step + 1) % 2 === 0) {
        await context.checkpoint.save({
          cursor: step + 1,
          savedAt: new Date().toISOString(),
        });
      }

      await new Promise((resolve) => setTimeout(resolve, 50));
    }

    return successTaskResult({ completed: true, steps: 10 });
  },
});
