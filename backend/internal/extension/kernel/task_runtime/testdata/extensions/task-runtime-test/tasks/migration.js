const { defineTask, successTaskResult } = require("@amitia/plugin-sdk");

module.exports = defineTask({
  taskId: "migration_task",
  handler: async (input, context) => {
    const total = (input && typeof input.total === "number") ? input.total : 10;

    let startIdx = 0;
    let migrated = 0;

    const loaded = await context.checkpoint.load();
    if (loaded) {
      if (typeof loaded.cursor === "number") {
        startIdx = loaded.cursor;
      }
      if (typeof loaded.migrated === "number") {
        migrated = loaded.migrated;
      }
      context.logger.info("从 checkpoint 恢复迁移", { cursor: startIdx, migrated });
    }

    for (let i = startIdx; i < total; i++) {
      const sourceKey = `source:record:${i}`;
      const record = await context.storage.get(sourceKey);
      if (record !== null && record !== undefined) {
        const targetKey = `target:record:${i}`;
        await context.storage.set(targetKey, record);
        migrated++;
        context.logger.debug("记录迁移完成", { index: i, targetKey });
      } else {
        context.logger.warn("源记录缺失，跳过", { index: i, sourceKey });
      }

      await context.checkpoint.save({
        cursor: i + 1,
        migrated,
        savedAt: new Date().toISOString(),
      });

      await context.progress.report({
        current: i + 1,
        total,
        message: `迁移记录 ${i + 1}/${total}`,
      });

      await new Promise((resolve) => setTimeout(resolve, 20));
    }

    return successTaskResult({ migrated });
  },
});
