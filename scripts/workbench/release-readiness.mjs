import fs from 'node:fs';
import { fileURLToPath } from 'node:url';

export const requiredProductionGates = Object.freeze([
  'go.toolchain',
  'go.workbench.targeted-tests',
  'go.backend.full-compile',
  'front.vue-typecheck',
  'front.workbench-chunk-tests',
  'electron.typecheck',
  'flutter.workbench.analyze',
  'flutter.workbench.chunk-tests',
  'flutter.workbench.task-protocol-tests',
  'flutter.workbench.terminal-tests',
  'integration.authenticated-task-read',
  'integration.agent-execution-and-merge',
  'integration.device-validation',
]);

export function evaluateWorkbenchRelease(report) {
  if (!report || report.protocolVersion !== 1 || !Array.isArray(report.results)) {
    throw new Error('Invalid Workbench integration-gate report');
  }
  const byName = new Map();
  for (const item of report.results) {
    if (!item || typeof item.name !== 'string' || typeof item.status !== 'string' || byName.has(item.name)) {
      throw new Error('Duplicate or invalid gate result');
    }
    byName.set(item.name, item);
  }
  const requirements = requiredProductionGates.map(name => {
    const actual = byName.get(name);
    const status = actual?.status === 'passed' ? 'passed' : actual?.status === 'failed' ? 'failed' : 'blocked';
    return { name, status, reason: status === 'passed' ? '' : (typeof actual?.detail === 'string' ? actual.detail.slice(0, 500) : 'No verified execution result') };
  });
  if (byName.get('go.toolchain')?.status === 'passed' && byName.get('go.version.requirement')) {
    requirements.find(item => item.name === 'go.toolchain').status = 'blocked';
    requirements.find(item => item.name === 'go.toolchain').reason = 'Go version requirement not met';
  }
  for (const name of ['go.workbench.targeted-tests']) {
    const item = byName.get(name);
    if (item?.status === 'passed' && (!(item.passingTestEvents > 0) || !(item.passingPackageEvents > 0) || item.failingEvents !== 0)) {
      const gate = requirements.find(row => row.name === name);
      gate.status = 'failed';
      gate.reason = 'Missing complete test-level and package-level evidence';
    }
  }
  const failures = requirements.filter(row => row.status === 'failed').length;
  const blocked = requirements.filter(row => row.status === 'blocked').length;
  const complete = report.productionAcceptancePassed === true && report.mode === 'targeted-integration' && failures === 0 && blocked === 0;
  return {
    protocolVersion: 1,
    status: complete ? 'passed' : failures ? 'failed' : 'blocked',
    productionReleaseReady: complete,
    passed: requirements.length - blocked - failures,
    failed: failures,
    blocked,
    requirements,
    note: 'Evidence of isolated tests or implementation scope is not production release acceptance',
  };
}

function main() {
  const argv = process.argv.slice(2);
  const inputIndex = argv.indexOf('--input');
  if (inputIndex < 0 || !argv[inputIndex + 1]) throw new Error('Usage: node release-readiness.mjs --input integration-gate.json [--output report.json]');
  const report = JSON.parse(fs.readFileSync(argv[inputIndex + 1], 'utf8'));
  const result = evaluateWorkbenchRelease(report);
  const output = JSON.stringify(result, null, 2) + '\n';
  const outputIndex = argv.indexOf('--output');
  if (outputIndex >= 0) {
    if (!argv[outputIndex + 1]) throw new Error('Missing --output path');
    fs.writeFileSync(argv[outputIndex + 1], output);
  }
  process.stdout.write(output);
  if (!result.productionReleaseReady) process.exitCode = result.status === 'failed' ? 1 : 2;
}

if (process.argv[1] && fileURLToPath(import.meta.url) === process.argv[1]) {
  try { main(); } catch (error) { process.stderr.write(String(error.message) + '\n'); process.exitCode = 1; }
}
