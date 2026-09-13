import { StdioTransport } from '@amitia/game-plugin-sdk';

async function main() {
  const transport = new StdioTransport();

  const badHello = {
    protocol: 'amitia-game-host/99',
    type: 'request',
    id: 'wrong-proto-1',
    method: 'control.handshake.hello',
    payload: {
      supportedProtocols: ['amitia-game-host/99'],
      capabilities: ['custom_rpc'],
      rpcNamespaces: ['mockgame.echo'],
    },
  };

  await transport.send(badHello);

  try {
    const response = await transport.receive();
    const data = JSON.stringify(response);
    const header = Buffer.alloc(4);
    header.writeUInt32BE(Buffer.byteLength(data), 0);
    process.stdout.write(Buffer.concat([header, Buffer.from(data)]));
  } catch (err) {
    process.exit(1);
  }
}

main().catch(() => process.exit(1));
