import net from 'node:net';
import dgram from 'node:dgram';
import http from 'node:http';
import crypto from 'node:crypto';

const TCP_PORT = 18901;
const UDP_PORT = 18902;
const WS_PORT = 18903;
const HOST = '127.0.0.1';

const tcp = net.createServer((socket) => {
  socket.on('data', (chunk) => socket.write(Buffer.concat([Buffer.from('game-tcp:'), chunk])));
});

const udp = dgram.createSocket('udp4');
udp.on('message', (message, remote) => {
  const response = Buffer.concat([Buffer.from('game-udp:'), message]);
  udp.send(response, remote.port, remote.address);
});

function encodeWebSocketFrame(opcode, payload) {
  const body = Buffer.from(payload);
  let header;
  if (body.length < 126) {
    header = Buffer.from([0x80 | opcode, body.length]);
  } else if (body.length <= 0xffff) {
    header = Buffer.alloc(4);
    header[0] = 0x80 | opcode;
    header[1] = 126;
    header.writeUInt16BE(body.length, 2);
  } else {
    throw new Error('generic fixture websocket payload is too large');
  }
  return Buffer.concat([header, body]);
}

function decodeClientFrames(state, incoming, onFrame) {
  state.buffer = Buffer.concat([state.buffer, incoming]);
  for (;;) {
    if (state.buffer.length < 2) return;
    const first = state.buffer[0];
    const second = state.buffer[1];
    const fin = (first & 0x80) !== 0;
    const opcode = first & 0x0f;
    const masked = (second & 0x80) !== 0;
    let length = second & 0x7f;
    let offset = 2;
    if (!fin || !masked) throw new Error('generic fixture requires final masked client frames');
    if (length === 126) {
      if (state.buffer.length < 4) return;
      length = state.buffer.readUInt16BE(2);
      offset = 4;
    } else if (length === 127) {
      throw new Error('generic fixture websocket frame exceeds supported size');
    }
    if (state.buffer.length < offset + 4 + length) return;
    const mask = state.buffer.subarray(offset, offset + 4);
    offset += 4;
    const payload = Buffer.from(state.buffer.subarray(offset, offset + length));
    for (let i = 0; i < payload.length; i += 1) payload[i] ^= mask[i % 4];
    state.buffer = state.buffer.subarray(offset + length);
    onFrame(opcode, payload);
  }
}

const wsHttp = http.createServer((_req, res) => {
  res.writeHead(426, { 'content-type': 'text/plain' });
  res.end('websocket upgrade required');
});

wsHttp.on('upgrade', (req, socket) => {
  const key = req.headers['sec-websocket-key'];
  if (typeof key !== 'string' || req.url !== '/echo') {
    socket.destroy();
    return;
  }
  const accept = crypto.createHash('sha1').update(`${key}258EAFA5-E914-47DA-95CA-C5AB0DC85B11`).digest('base64');
  socket.write(
    'HTTP/1.1 101 Switching Protocols\r\n' +
    'Upgrade: websocket\r\n' +
    'Connection: Upgrade\r\n' +
    `Sec-WebSocket-Accept: ${accept}\r\n\r\n`,
  );
  const state = { buffer: Buffer.alloc(0) };
  socket.on('data', (chunk) => {
    try {
      decodeClientFrames(state, chunk, (opcode, payload) => {
        if (opcode === 0x8) {
          socket.end(encodeWebSocketFrame(0x8, Buffer.alloc(0)));
          return;
        }
        if (opcode === 0x9) {
          socket.write(encodeWebSocketFrame(0xA, payload));
          return;
        }
        if (opcode !== 0x1 && opcode !== 0x2) {
          socket.destroy();
          return;
        }
        socket.write(encodeWebSocketFrame(opcode, Buffer.concat([Buffer.from('game-ws:'), payload])));
      });
    } catch {
      socket.destroy();
    }
  });
});

await Promise.all([
  new Promise((resolve, reject) => {
    tcp.once('error', reject);
    tcp.listen(TCP_PORT, HOST, resolve);
  }),
  new Promise((resolve, reject) => {
    udp.once('error', reject);
    udp.bind(UDP_PORT, HOST, resolve);
  }),
  new Promise((resolve, reject) => {
    wsHttp.once('error', reject);
    wsHttp.listen(WS_PORT, HOST, resolve);
  }),
]);

process.stdout.write(JSON.stringify({ ready: true, tcp: TCP_PORT, udp: UDP_PORT, websocket: WS_PORT }) + '\n');

let stopping = false;
function stop() {
  if (stopping) return;
  stopping = true;
  tcp.close();
  udp.close();
  wsHttp.close(() => process.exit(0));
  setTimeout(() => process.exit(0), 1000).unref();
}
process.on('SIGTERM', stop);
process.on('SIGINT', stop);
