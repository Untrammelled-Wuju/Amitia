export function apply(ctx) {
  ctx.amitiaTools.register({
    name: "start-job",
    execute() {
      const job = ctx.jobs.start({
        id: "job-1",
        sessionId: "session-1",
        kind: "bash",
        label: "fixture job",
        run({ signal }) {
          return new Promise((resolve) => {
            signal.addEventListener("abort", () => resolve("stopped"), { once: true });
          });
        },
      });
      return { jobId: job.jobId };
    },
  });
}
