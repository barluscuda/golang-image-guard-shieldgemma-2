const assert = require('node:assert/strict');
const fs = require('node:fs');
const path = require('node:path');
const test = require('node:test');
const vm = require('node:vm');

const html = fs.readFileSync(path.join(__dirname, 'web.html'), 'utf8');
const script = Array.from(html.matchAll(/<script>([\s\S]*?)<\/script>/g)).at(-1)[1];
const origin = 'http://guard.test';
const cached = { clientId: 'client-1', imageId: 'image-1', fileName: 'fixture.png', status: 'processed', outcome: 'success', moderationResult: { verdict: 'allowed', model: 'shieldgemma' } };
const decision = (verdict = 'allowed') => ({ image_id: 'image-1', status: 'processed', outcome: 'success', moderation_result: { verdict, model: 'shieldgemma' } });
const response = (payload, status = 200) => ({ ok: status >= 200 && status < 300, status, json: async () => payload });
const flush = () => new Promise((resolve) => setImmediate(resolve));

function workspace(fetchImage, records = [cached]) {
  const elements = new Map();
  const calls = [];
  const storage = new Map([['image-guard-browser-jobs-v1', JSON.stringify(records)]]);
  function element(id) {
    if (!elements.has(id)) elements.set(id, {
      value: '', textContent: '', innerHTML: '', className: '', title: '',
      classList: { toggle() {}, add() {}, remove() {} },
      events: {}, addEventListener(name, fn) { this.events[name] = fn; },
      querySelector() { return element(id + '-span'); }, scrollIntoView() {},
    });
    return elements.get(id);
  }
  vm.runInNewContext(script, {
    document: { getElementById: element },
    localStorage: { getItem: (key) => storage.get(key), setItem: (key, value) => storage.set(key, value) },
    window: { location: { protocol: 'http:', origin }, setInterval() {} },
    fetch: async (url) => {
      calls.push(url);
      return url.endsWith('/health/ready') ? response({ status: 'ready' }) : fetchImage(url);
    },
    setTimeout() {}, URL, console,
  });
  return { element, calls, storage };
}

test('cached approval waits for a fresh API result', async () => {
  let resolveImage;
  const app = workspace(() => new Promise((resolve) => { resolveImage = resolve; }));
  assert.equal(Number(app.element('stat-allowed').textContent), 0);
  assert.doesNotMatch(app.element('queue-list').innerHTML, />Allowed</);
  assert.ok(app.calls.includes(origin + '/v1/images/image-1'));
  resolveImage(response(decision()));
  await flush();
  assert.equal(Number(app.element('stat-allowed').textContent), 1);
  assert.match(app.element('queue-list').innerHTML, />Allowed</);
});

test('missing image invalidates a cached approval', async () => {
  const app = workspace(() => response({ error: 'image not found' }, 404));
  await flush();
  assert.equal(Number(app.element('stat-allowed').textContent), 0);
  assert.doesNotMatch(app.element('queue-list').innerHTML, />Allowed</);
  const record = JSON.parse(app.storage.get('image-guard-browser-jobs-v1'))[0];
  assert.equal(record.status, 'unavailable');
  assert.equal(record.moderationResult, undefined);
});

test('API errors cannot display or count an allowed result', async () => {
  const app = workspace(() => response({ ...decision(), outcome: 'error', processing_error: 'moderation_failed' }));
  await flush();
  assert.equal(Number(app.element('stat-allowed').textContent), 0);
  assert.match(app.element('queue-list').innerHTML, />Could not process</);
});

test('reconnecting revalidates completed jobs', async () => {
  const app = workspace((url) => response(decision(url.startsWith(origin) ? 'allowed' : 'blocked')));
  await flush();
  assert.equal(Number(app.element('stat-allowed').textContent), 1);
  app.element('api-base').value = 'http://other.test';
  await app.element('save-api').events.click();
  await flush();
  assert.ok(app.calls.includes('http://other.test/v1/images/image-1'));
  assert.equal(Number(app.element('stat-allowed').textContent), 0);
  assert.equal(Number(app.element('stat-blocked').textContent), 1);
});

test('late responses from an old API cannot restore approval', async () => {
  let resolveOld;
  const app = workspace((url) => url.startsWith(origin)
    ? new Promise((resolve) => { resolveOld = resolve; })
    : response({ error: 'image not found' }, 404));
  app.element('api-base').value = 'http://other.test';
  await app.element('save-api').events.click();
  await flush();
  resolveOld(response(decision()));
  await flush();
  assert.equal(Number(app.element('stat-allowed').textContent), 0);
  assert.doesNotMatch(app.element('queue-list').innerHTML, />Allowed</);
});
