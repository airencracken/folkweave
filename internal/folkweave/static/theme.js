// SPDX-License-Identifier: AGPL-3.0-or-later
// Loaded without defer in <head> so a saved theme applies before first paint.
// Without storage the picker still works for the current page.
(function (root) {
  'use strict';
  const key = 'folkweave-theme';
  const normalize = value => value === 'light' || value === 'dark' ? value : 'system';
  function install(env) {
    const element = env.document.documentElement;
    let preference = 'system';
    try { preference = normalize(env.localStorage.getItem(key)); } catch { /* Storage blocked: use the system theme. */ }
    const apply = () => {
      if (preference === 'system') element.removeAttribute('data-theme');
      else element.setAttribute('data-theme', preference);
    };
    const sync = () => {
      for (const control of env.document.querySelectorAll('[data-theme-select]')) {
        control.value = preference;
        const label = control.closest('.theme-picker');
        if (label) label.hidden = false;
      }
    };
    apply();
    env.document.addEventListener('DOMContentLoaded', sync);
    env.document.addEventListener('htmx:afterSwap', sync);
    env.document.addEventListener('change', event => {
      if (!event.target.matches || !event.target.matches('[data-theme-select]')) return;
      preference = normalize(event.target.value);
      apply(); sync();
      try {
        if (preference === 'system') env.localStorage.removeItem(key);
        else env.localStorage.setItem(key, preference);
      } catch { /* Keep the choice for this page when storage is blocked. */ }
    });
    env.addEventListener('storage', event => {
      if (event.key !== key && event.key !== null) return;
      preference = normalize(event.newValue);
      apply(); sync();
    });
    return { get preference() { return preference; } };
  }
  if (typeof module !== 'undefined' && module.exports) module.exports = { install, normalize, key };
  if (root.document) install(root);
})(typeof window === 'undefined' ? globalThis : window);
