'use strict';
const test = require('node:test');
const assert = require('node:assert/strict');
const { install, normalize, key } = require('./theme.js');

function environment(stored, { blocked = false } = {}) {
  const listeners = {}, windowListeners = {}, attributes = {};
  const label = { hidden: true };
  const control = { value: 'system', closest: () => label, matches: selector => selector === '[data-theme-select]' };
  const storage = new Map(stored === undefined ? [] : [[key, stored]]);
  const guard = fn => (...args) => { if (blocked) throw new Error('SecurityError'); return fn(...args); };
  return {
    attributes, label, control, storage, listeners, windowListeners,
    document: {
      documentElement: { setAttribute: (n, v) => { attributes[n] = v; }, removeAttribute: n => { delete attributes[n]; } },
      querySelectorAll: () => [control],
      addEventListener: (name, fn) => { listeners[name] = fn; }
    },
    localStorage: { getItem: guard(k => storage.has(k) ? storage.get(k) : null), setItem: guard((k, v) => storage.set(k, v)), removeItem: guard(k => storage.delete(k)) },
    addEventListener: (name, fn) => { windowListeners[name] = fn; }
  };
}

test('saved themes apply before the page loads and the picker shows with JavaScript', () => {
  for (const theme of ['light', 'dark']) {
    const env = environment(theme);
    install(env);
    assert.equal(env.attributes['data-theme'], theme);
    assert.equal(env.label.hidden, true);
    env.listeners.DOMContentLoaded();
    assert.equal(env.label.hidden, false);
    assert.equal(env.control.value, theme);
  }
});

test('system is the default and removes the attribute', () => {
  const env = environment(undefined);
  install(env);
  assert.equal('data-theme' in env.attributes, false);
  env.control.value = 'dark';
  env.listeners.change({ target: env.control });
  assert.equal(env.attributes['data-theme'], 'dark');
  assert.equal(env.storage.get(key), 'dark');
  env.control.value = 'system';
  env.listeners.change({ target: env.control });
  assert.equal('data-theme' in env.attributes, false);
  assert.equal(env.storage.has(key), false);
});

test('blocked storage falls back to system and keeps the choice for the page', () => {
  const env = environment('dark', { blocked: true });
  const state = install(env);
  assert.equal('data-theme' in env.attributes, false);
  env.control.value = 'light';
  assert.doesNotThrow(() => env.listeners.change({ target: env.control }));
  assert.equal(env.attributes['data-theme'], 'light');
  assert.equal(state.preference, 'light');
});

test('adversarial stored and submitted values normalize to system', () => {
  for (const value of ['', 'DARK', 'dark ', '<script>', 'system', null, undefined, '__proto__']) assert.equal(normalize(value), 'system');
  const env = environment('"><img src=x>');
  install(env);
  assert.equal('data-theme' in env.attributes, false);
  env.control.value = 'purple';
  env.listeners.change({ target: env.control });
  assert.equal('data-theme' in env.attributes, false);
});

test('other tabs update the theme and unrelated events are ignored', () => {
  const env = environment(undefined);
  install(env);
  env.windowListeners.storage({ key: 'other', newValue: 'dark' });
  assert.equal('data-theme' in env.attributes, false);
  env.windowListeners.storage({ key, newValue: 'dark' });
  assert.equal(env.attributes['data-theme'], 'dark');
  env.windowListeners.storage({ key: null, newValue: null });
  assert.equal('data-theme' in env.attributes, false);
  env.listeners.change({ target: { matches: () => false, value: 'light' } });
  assert.equal('data-theme' in env.attributes, false);
});
