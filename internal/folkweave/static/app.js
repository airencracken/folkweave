(function (root) {
  'use strict';
  function composer(env) {
    let sequence = 0;
    return {
      body: '', photos: [], error: '', clean: false, unload: null,
      init() {
        const input = this.$el && this.$el.querySelector('[name=body]');
        if (input) this.body = input.value;
        this.unload = event => { if (!this.clean && (this.body.trim() || this.photos.length)) { event.preventDefault(); event.returnValue = ''; } };
        env.addEventListener('beforeunload', this.unload);
      },
      choose(event) {
        const input = event.target, data = input.dataset || {};
        const limit = Number(data.maxBytes) || 5242880, label = data.maxLabel || '5 MiB';
        const files = Array.from(input.files || []);
        if (files.length > 4 || files.some(file => file.size > limit || !['image/jpeg', 'image/png'].includes(file.type))) {
          this.error = 'Choose up to four JPEG or PNG photos, no larger than ' + label + ' each.';
          this.sync(); return;
        }
        this.photos.forEach(photo => env.URL.revokeObjectURL(photo.preview));
        this.photos = files.map(file => { const id = ++sequence; return { id, inputID: 'photo-alt-' + id, file, alt: '', preview: env.URL.createObjectURL(file) }; });
        this.error = ''; this.clean = false;
      },
      sync() { const data = new env.DataTransfer(); this.photos.forEach(photo => data.items.add(photo.file)); if (this.$refs && this.$refs.files) this.$refs.files.files = data.files; },
      describe(id, value) { const photo = this.photos.find(photo => photo.id === id); if (photo) photo.alt = value; },
      move(id, direction) { const index = this.photos.findIndex(photo => photo.id === id); const next = index + direction; if (index < 0 || next < 0 || next >= this.photos.length) return; const copy = this.photos.slice(); [copy[index], copy[next]] = [copy[next], copy[index]]; this.photos = copy; this.sync(); },
      remove(id) { const photo = this.photos.find(photo => photo.id === id); if (!photo) return; env.URL.revokeObjectURL(photo.preview); this.photos = this.photos.filter(photo => photo.id !== id); this.sync(); },
      posted() { this.clean = true; },
      destroy() { this.photos.forEach(photo => env.URL.revokeObjectURL(photo.preview)); if (this.unload) env.removeEventListener('beforeunload', this.unload); }
    };
  }
  if (typeof module !== 'undefined' && module.exports) module.exports = { composer };
  if (!root.document) return;
  root.document.addEventListener('alpine:init', () => root.Alpine.data('composer', () => composer(root)));
  root.document.addEventListener('htmx:beforeSwap', event => {
    const status = event.detail.xhr.status;
    if ([400, 403, 409, 413, 422].includes(status) && event.detail.xhr.getResponseHeader('HX-Retarget')) { event.detail.shouldSwap = true; event.detail.isError = false; }
  });
  root.document.addEventListener('htmx:responseError', event => {
    const form = event.detail.elt.closest('form');
    if (!form) return;
    const error = form.querySelector('[aria-live]');
    if (error && event.detail.xhr.status >= 500) error.textContent = 'That could not be saved. Your draft is still here; try again.';
  });
  root.document.addEventListener('htmx:sendError', event => {
    const form = event.detail.elt.closest('form');
    const error = form && form.querySelector('[aria-live]');
    if (error) error.textContent = 'The connection was interrupted. Your draft is still here; try again.';
  });
})(typeof window === 'undefined' ? globalThis : window);
