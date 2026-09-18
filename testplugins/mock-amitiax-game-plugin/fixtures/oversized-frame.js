import { StdioTransport } from '@amitia/game-plugin-sdk';

async function main() {
  const transport = new StdioTransport();

  const oversizedSize = 17 * 1024 * 1024;
  const header = Buffer.alloc(4);
  header.writeUInt32BE(oversizedSize, 0);

  process.stdout.write(header);

  try {
    await transport.receive();
  } catch {
    process.exit(0);
  }
}

main().catch(() => process.exit(0));
