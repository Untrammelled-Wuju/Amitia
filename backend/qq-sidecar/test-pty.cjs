// SPDX-FileCopyrightText: 2026 彭旭
// SPDX-License-Identifier: AGPL-3.0-only
const { spawn } = require('node-pty');
const fs = require('fs');
console.log('Starting Lagrange via PTY...');
const p = spawn('D:/桌面/跟进项目/U-Ai/backend/lagrange/bin/Lagrange.OneBot.exe', [], {
  name: 'Lagrange',
  cwd: 'D:/桌面/跟进项目/U-Ai/backend/lagrange/bin',
  cols: 120,
  rows: 40,
});
let output = '';
p.onData((data) => { output += data; });
setTimeout(() => {
  console.log('Output length:', output.length);
  console.log('--- First 2000 chars ---');
  console.log(output.substring(0, 2000));
  const urls = output.match(/https?:\/\/[^\s]{10,}/g) || [];
  console.log('URLs found:', JSON.stringify(urls));
  fs.writeFileSync('D:/桌面/跟进项目/U-Ai/backend/lagrange/bin/pty-out.txt', output);
  console.log('Saved to pty-out.txt');
  p.kill();
  process.exit(0);
}, 25000);
