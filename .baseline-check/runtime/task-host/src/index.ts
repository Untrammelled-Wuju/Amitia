import { bootstrap } from "./bootstrap.js";

bootstrap().catch((error: unknown) => {
  const message = error instanceof Error ? error.message : String(error);
  process.stderr.write(`task-host bootstrap failed: ${message}\n`);
  process.exit(1);
});
