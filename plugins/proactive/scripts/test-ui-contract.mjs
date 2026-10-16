import assert from 'node:assert/strict';
import { readFileSync } from 'node:fs';
const app = readFileSync('src/ui/app.js', 'utf8');
const requiredIDs = [...app.matchAll(/\$\("([a-z0-9-]+)"\)/g)].map(match => match[1]);
for (const file of ['src/ui/index.html', 'src/ui/mobile/index.html']) {
  const html = readFileSync(file, 'utf8');
  const ids = new Set([...html.matchAll(/\bid="([^"]+)"/g)].map(match => match[1]));
  for (const id of requiredIDs) assert.ok(ids.has(id), `${file} missing ${id}`);
}
const requiredSettings = ['enabled', 'activeLevel', 'minInterval', 'maxPerDay', 'maxDailyCalls', 'quietStart', 'quietEnd', 'unrepliedSlowdownEnabled', 'unrepliedSlowdownAfter', 'unrepliedCooldownMultiplier', 'unrepliedRecoveryOnReply'];
const requiredRules = ['id', 'name', 'scheduleCron', 'promptTemplate', 'conversationId', 'timezone', 'channelTargetsText', 'endAt', 'maxOccurrences', 'systemNotification', 'maxPerDay', 'randomMinutes', 'quietStart', 'quietEnd'];
for (const file of ['ui/schema/desktop.json', 'ui/schema/mobile.json']) {
  const schema = JSON.parse(readFileSync(file, 'utf8'));
  const nodes = [];
  function walk(node) {
    if (!node || typeof node !== 'object') return;
    nodes.push(node);
    for (const value of Object.values(node)) if (Array.isArray(value)) value.forEach(walk); else walk(value);
  }
  walk(schema);
  const settings = nodes.find(node => node.action === 'settings.update');
  const rules = nodes.find(node => node.action === 'rules.save');
  for (const key of requiredSettings) assert.equal(settings.payload[key], `$form.settings.${key}`, `${file} setting ${key}`);
  for (const key of requiredRules) assert.equal(rules.payload[key], `$form.rule.${key}`, `${file} rule ${key}`);
  assert.equal(nodes.find(node => node.props?.label === '规则 ID').props.variant, undefined);
  assert.ok(nodes.some(node => node.prop === 'result'), `${file} missing delivery result`);
}
const manifest = JSON.parse(readFileSync('amitia-extension.json', 'utf8'));
const command = manifest.modules.flatMap(module => module.contributions || []).find(item => item.spec?.handlerName === 'command');
assert.equal(command.spec.inputSchema.properties.payload.properties.scope.type, 'object');
assert.equal(command.spec.inputSchema.properties.ui_state.type, 'boolean');
console.log('proactive-ui-contract=ok');
