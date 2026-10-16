import test from 'node:test';
import assert from 'node:assert/strict';
import { evaluateWorkbenchRelease, requiredProductionGates } from './release-readiness.mjs';

function verified() {
  return {
    protocolVersion: 1,
    mode: 'targeted-integration',
    productionAcceptancePassed: true,
    results: requiredProductionGates.map(name => ({ name, status: 'passed', passingTestEvents: 3, passingPackageEvents: 1, failingEvents: 0 })),
  };
}

test('a fully verified targeted integration report meets production gate', () => {
  const output = evaluateWorkbenchRelease(verified());
  assert.equal(output.productionReleaseReady, true);
  assert.equal(output.status, 'passed');
});

test('isolated development completion cannot assert production readiness', () => {
  const report = verified();
  report.mode = 'development-validation';
  assert.equal(evaluateWorkbenchRelease(report).productionReleaseReady, false);
});

test('missing live authorization and device checks fail closed', () => {
  const report = verified();
  report.results = report.results.filter(row => !row.name.startsWith('integration.'));
  const result = evaluateWorkbenchRelease(report);
  assert.equal(result.productionReleaseReady, false);
  assert.equal(result.blocked, 3);
});

test('failed compile takes priority over absent production checks', () => {
  const report = verified();
  report.results.find(row => row.name === 'go.backend.full-compile').status = 'failed';
  assert.equal(evaluateWorkbenchRelease(report).status, 'failed');
});

test('a Go compiler version mismatch is never a success', () => {
  const report = verified();
  report.results.push({ name: 'go.version.requirement', status: 'blocked' });
  assert.equal(evaluateWorkbenchRelease(report).productionReleaseReady, false);
});

test('empty Go test event evidence is not a release pass', () => {
  const report = verified();
  report.results.find(row => row.name === 'go.workbench.targeted-tests').passingTestEvents = 0;
  assert.equal(evaluateWorkbenchRelease(report).status, 'failed');
});

test('the tool requires a positive source report decision', () => {
  const report = verified();
  report.productionAcceptancePassed = false;
  assert.equal(evaluateWorkbenchRelease(report).productionReleaseReady, false);
});

test('duplicate and malformed evidence is rejected', () => {
  const report = verified();
  report.results.push(report.results[0]);
  assert.throws(() => evaluateWorkbenchRelease(report));
  assert.throws(() => evaluateWorkbenchRelease({ protocolVersion: 1 }));
});
