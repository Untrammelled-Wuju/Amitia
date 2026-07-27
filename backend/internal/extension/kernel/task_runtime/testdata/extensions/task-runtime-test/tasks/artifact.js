const { defineTask, successTaskResult } = require("@amitia/plugin-sdk");

module.exports = defineTask({
  taskId: "artifact_task",
  handler: async (input, context) => {
    const unit = "Amitia-runtime-test-payload-chunk-";
    const targetSize = 300 * 1024;
    const repeatCount = Math.ceil(targetSize / unit.length) + 1;
    const content = unit.repeat(repeatCount);

    await context.progress.report({ current: 50, total: 100, message: "生成大数据载荷" });

    const artifact = await context.artifacts.saveData(
      "large-payload.json",
      { content, size: content.length, generatedAt: new Date().toISOString() },
      { kind: "data", mimeType: "application/json" }
    );

    await context.progress.report({ current: 100, total: 100, message: "artifact 保存完成" });

    return successTaskResult({ artifactId: artifact.artifactId, size: content.length });
  },
});
