import { MockAmitiaXGamePlugin } from './plugin.js';

async function main(): Promise<void> {
  const plugin = new MockAmitiaXGamePlugin();

  const shutdown = (): void => {
    process.exit(0);
  };

  process.on('SIGINT', shutdown);
  process.on('SIGTERM', shutdown);

  try {
    await plugin.run();
  } catch (err) {
    console.error(`plugin error: ${err}`);
    process.exit(1);
  }
}

main().catch((err) => {
  console.error(`fatal error: ${err}`);
  process.exit(1);
});
