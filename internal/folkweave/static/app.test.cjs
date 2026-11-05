const { test } = require('node:test');
const assert = require('node:assert/strict');
const { composer } = require('./app.js');

function setup(initial = '') {
  const revoked = [], listeners = new Map(); let sequence = 0;
  const env = { URL: { createObjectURL: () => 'blob:' + ++sequence, revokeObjectURL: value => revoked.push(value) },
    DataTransfer: class { constructor() { this.values = []; this.items = { add: value => this.values.push(value) }; } get files() { return this.values; } },
    addEventListener: (name, fn) => listeners.set(name, fn), removeEventListener: name => listeners.delete(name) };
  const model = composer(env); model.$refs = { files: { files: [] } }; model.$el = { querySelector: () => ({ value: initial }) };
  model.init(); return { model, revoked, listeners };
}
const photo = (name, size = 100, type = 'image/jpeg') => ({ name, size, type });
test('server-rendered failed draft survives initialization', () => { const { model } = setup('keep this'); assert.equal(model.body, 'keep this'); });
test('selection previews only bounded images', () => { const { model } = setup(); model.choose({ target: { files: [photo('one'), photo('two')] } }); assert.equal(model.photos.length, 2); assert.equal(model.photos[0].preview, 'blob:1'); });
test('photo order and descriptions follow the file', () => { const { model } = setup(); model.choose({ target: { files: [photo('one'), photo('two')] } }); const id = model.photos[1].id; model.describe(id, 'second picture'); model.move(id, -1); assert.equal(model.photos[0].file.name, 'two'); assert.equal(model.photos[0].alt, 'second picture'); assert.deepEqual(model.$refs.files.files.map(f => f.name), ['two', 'one']); });
test('invalid selections preserve the previous files and descriptions', () => { const { model, revoked } = setup(); model.choose({ target: { files: [photo('keep')] } }); model.describe(model.photos[0].id, 'keep the description'); for (const files of [[photo('large', 5 * 1024 * 1024 + 1)], [photo('script', 3, 'image/svg+xml')], Array.from({ length: 5 }, () => photo('too-many'))]) { model.choose({ target: { files } }); assert.ok(model.error); assert.equal(model.photos[0].alt, 'keep the description'); assert.equal(model.$refs.files.files[0].name, 'keep'); } assert.deepEqual(revoked, []); });
test('remove revokes its preview and updates submission order', () => { const { model, revoked } = setup(); model.choose({ target: { files: [photo('one'), photo('two')] } }); model.remove(model.photos[0].id); assert.deepEqual(revoked, ['blob:1']); assert.equal(model.$refs.files.files.length, 1); assert.equal(model.$refs.files.files[0].name, 'two'); });
test('moving beyond either end preserves all selected files', () => { const { model } = setup(); model.choose({ target: { files: [photo('one'), photo('two')] } }); model.move(model.photos[0].id, -1); model.move(model.photos[1].id, 1); assert.deepEqual(model.photos.map(p => p.file.name), ['one', 'two']); });
test('draft prompts remain active until a successful post', () => { const { model, listeners } = setup('draft'); let prevented = 0; listeners.get('beforeunload')({ preventDefault: () => prevented++ }); assert.equal(prevented, 1); model.posted(); listeners.get('beforeunload')({ preventDefault: () => prevented++ }); assert.equal(prevented, 1); });
test('destroy cleans previews and its event listener', () => { const { model, listeners, revoked } = setup(); model.choose({ target: { files: [photo('one'), photo('two')] } }); model.destroy(); assert.equal(listeners.has('beforeunload'), false); assert.deepEqual(revoked, ['blob:1', 'blob:2']); });
test('reselection releases all superseded previews', () => { const { model, revoked } = setup(); model.choose({ target: { files: [photo('one'), photo('two')] } }); model.choose({ target: { files: [photo('three')] } }); assert.deepEqual(revoked, ['blob:1', 'blob:2']); assert.equal(model.photos[0].file.name, 'three'); });
