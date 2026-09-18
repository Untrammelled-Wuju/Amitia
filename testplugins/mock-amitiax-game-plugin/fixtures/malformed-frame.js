import { StdioTransport } from '@amitia/game-plugin-sdk';

async function main() {
  const transport = new StdioTransport();

  const payload = Buffer.from('this is not valid json {{{', 'utf8');
  const header = Buffer.alloc(4);
  header.writeUInt32BE(payload.length, 0);

  process.stdout.write(Buffer.concat([header, payload]));

  try {
    await transport.receive();
  } catch {
    process.exit(0);
  }
}

main().catch(() => process.exit(0));
