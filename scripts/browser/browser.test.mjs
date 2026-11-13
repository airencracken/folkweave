import { findBrowser, reservePort, spawnServer, stopProcess } from '../browser-runtime.mjs';
import { act, checkSuiteControls } from '../browser-controls.mjs';
import { before, after, test } from 'node:test';
import assert from 'node:assert/strict';
import { spawn } from 'node:child_process';
import { mkdir } from 'node:fs/promises';
import { existsSync } from 'node:fs';
import { fileURLToPath } from 'node:url';
import { chromium } from 'playwright';

const root = fileURLToPath(new URL('../../', import.meta.url));
let server, browser, origin;
before(async () => {
  const port=await reservePort();origin='http://127.0.0.1:'+port;
  server=await spawnServer(process.env.FOLKWEAVE_BINARY || root+'dist/folkweave',['demo','--listen','127.0.0.1:'+port],{env:{...process.env,FOLKWEAVE_BACKUP_INTERVAL:'0',FOLKWEAVE_APPS:'[{"id":"witmoot","url":"https://suite.example.org"}]'},health:origin+'/healthz'});
  browser = await chromium.launch({ executablePath: await findBrowser({ required: true }), headless: true, args: ['--no-sandbox'] });
  await mkdir(root + 'dist/screenshots', { recursive: true });
  const preview = await browser.newContext({ viewport: {width:1280,height:1100} });
  const page = await preview.newPage(); await signIn(page);
  await page.screenshot({path:root+'dist/screenshots/first-pass-desktop.png'});
  await page.setViewportSize({width:390,height:1000});
  await page.screenshot({path:root+'dist/screenshots/first-pass-mobile.png'});
  await preview.close();
});
after(async () => { if (browser) await browser.close(); await stopProcess(server); });

async function signIn(page) {
  await page.goto(origin + '/login');
  await act(page.getByLabel('Username', { exact: true }), 'fill', 'demo');
  await act(page.getByLabel('Password', { exact: true }), 'fill', 'folkweave-demo-only');
  await act(page.getByRole('button', { name: 'Sign in', exact: true }), 'click');
  await page.waitForURL(origin + '/');
}

test('mascot and plain account controls work on desktop and mobile', async () => {
  for (const width of [320, 1280]) for (const colorScheme of ['light', 'dark']) {
    const context = await browser.newContext({ viewport: { width, height: 1000 }, colorScheme });
    const page = await context.newPage();
    await page.goto(origin + '/login');
    const mascot = page.getByRole('img', { name: 'Folkweave’s loom mascot', exact: true });
    assert.equal(await mascot.evaluate(img => img.complete && img.naturalWidth === 256), true);
    assert.equal(await page.evaluate(() => document.documentElement.scrollWidth > innerWidth), false);
    await page.screenshot({ path: root + `dist/screenshots/login-${width}-${colorScheme}.png`, fullPage: true });
    await signIn(page);
    const administration = page.getByRole('navigation', { name: 'Main', exact: true }).getByRole('link', { name: 'Admin', exact: true });
    assert.equal(await administration.isVisible(), true);
    await act(administration, 'click');
    await page.getByRole('heading', { name: 'Settings', exact: true }).waitFor();
    await page.getByRole('heading', { name: 'Backups', exact: true }).waitFor();
    await page.goto(origin + '/');
    await checkSuiteControls(page, { apps: true });
    await act(page.getByRole('link', { name: 'Your account', exact: true }), 'click');
    await page.getByRole('heading', { name: 'Account', exact: true }).waitFor();
    await page.getByRole('heading', { name: 'Export your data', exact: true }).waitFor();
    await page.getByRole('heading', { name: 'Times and dates', exact: true }).waitFor();
    await act(page.getByRole('navigation', { name: 'Main', exact: true }).getByRole('link', { name: 'Admin', exact: true }), 'click');
    const adminNav = page.getByRole('navigation', { name: 'Administration', exact: true });
    for (const [name, heading] of [['Invitations', 'Invitations'], ['Metrics', 'Instance metrics'], ['Settings', 'Settings']]) {
      await act(adminNav.getByRole('link', { name, exact: true }), 'click');
      await page.getByRole('heading', { name: heading, exact: true }).waitFor();
      assert.equal(await page.evaluate(() => document.documentElement.scrollWidth > innerWidth), false, `${name} overflow`);
    }
    await page.getByRole('heading', { name: 'Backups', exact: true }).waitFor();
    assert.equal(await page.getByLabel('Group name', { exact: true }).inputValue(), 'The Sunday Folks');
    assert.equal(await page.locator('.brand img').evaluate(img => img.complete && img.naturalWidth === 256), true);
    assert.equal(await page.evaluate(() => document.documentElement.scrollWidth > innerWidth), false);
    await page.screenshot({ path: root + `dist/screenshots/settings-${width}-${colorScheme}.png`, fullPage: true });
    await context.close();
  }
});

for (const javascript of [true, false]) for (const width of [320, 390, 768, 1280]) {
  test(`timeline, photo post, reply, and bookmark: JS=${javascript}, width=${width}`, async () => {
    const context = await browser.newContext({ javaScriptEnabled: javascript, viewport: { width, height: 960 }, colorScheme: width === 390 || width === 1280 ? 'dark' : 'light' });
    const page = await context.newPage();
    const errors = []; page.on('pageerror', error => errors.push(error.message));
    if (javascript) await page.addInitScript(() => { window.cspViolations = []; document.addEventListener('securitypolicyviolation', event => window.cspViolations.push(event.violatedDirective)); });
    await signIn(page);
    const adminLink = page.getByRole('navigation', { name: 'Main', exact: true }).getByRole('link', { name: 'Admin', exact: true });
    assert.equal(await adminLink.isVisible(), true);
    await act(adminLink, 'click');
    await page.getByRole('heading', { name: 'Settings', exact: true }).waitFor();
    await page.goto(origin + '/');
    await page.locator('#feed .post').first().waitFor();
    const source = await page.locator('#feed .photos img').first().getAttribute('src');
    const response = await context.request.get(origin + source); assert.equal(response.status(), 200);
    const photo = await response.body();
    const text = `A quiet first post ${javascript}-${width}`;
    await act(page.locator('#post-body'), 'fill', text);
    if (javascript && width === 320) {
      await act(page.locator('#photos'), 'setInputFiles', { name: 'invalid.jpg', mimeType: 'image/jpeg', buffer: Buffer.from('not a photograph') });
      await act(page.getByRole('button', { name: 'Share', exact: true }), 'click');
      await page.locator('#composer-error [role=alert]').waitFor();
      assert.equal(await page.locator('#post-body').inputValue(), text);
      assert.equal(await page.locator('#photos').evaluate(input => input.files.length), 1);
    }
    await act(page.locator('#photos'), 'setInputFiles', { name: 'woodland.jpg', mimeType: 'image/jpeg', buffer: photo });
    if (javascript) {
      await act(page.locator('.photo-draft input'), 'fill', 'A woodland path with a pale sky');
      assert.equal(await page.locator('.photo-draft').count(), 1);
    } else await act(page.getByLabel('First photo description', { exact: true }), 'fill', 'A woodland path with a pale sky');
    await act(page.getByRole('button', { name: 'Share', exact: true }), 'click');
    const post = page.locator('#feed .post').filter({ hasText: text });
    await post.waitFor();
    assert.equal(await post.locator('img[alt="A woodland path with a pale sky"]').count(), 1);
    assert.equal(await page.locator('#post-body').inputValue(), '');
    await act(post.getByRole('link', { name: 'Reply', exact: true }), 'click');
    await act(page.locator('#reply-body'), 'fill', 'Good to see this here.');
    await act(page.getByRole('button', { name: 'Post reply', exact: true }), 'click');
    await page.locator('.reply .body-text').filter({ hasText: 'Good to see this here.' }).waitFor();
    assert.equal(await page.locator('#reply-body').inputValue(), '');
    assert.equal(await page.locator('#remove-post').count(), 1, 'reply refresh duplicated post controls');
    await act(page.getByRole('button', { name: 'Save', exact: true }), 'click');
    await page.getByRole('button', { name: 'Saved', exact: true }).waitFor();
    await act(page.getByRole('link', { name: 'Saved', exact: true }), 'click');
    await page.locator('#feed .post').filter({ hasText: text }).waitFor();
    const overflow = await page.evaluate(() => document.documentElement.scrollWidth > window.innerWidth);
    assert.equal(overflow, false, 'horizontal overflow');
    assert.deepEqual(errors, [], 'browser exceptions');
    if (javascript) assert.deepEqual(await page.evaluate(() => window.cspViolations), [], 'CSP violations');
    await page.screenshot({ path: root + `dist/screenshots/saved-${javascript}-${width}.png`, fullPage: true });
    await context.close();
  });
}

test('theme picker applies before paint, persists, and survives blocked storage', async () => {
  const context = await browser.newContext({ colorScheme: 'light' });
  const page = await context.newPage(); await signIn(page);
  const background = () => page.evaluate(() => getComputedStyle(document.body).backgroundColor);
  const light = await background();
  await act(page.getByLabel('Color theme', { exact: true }), 'selectOption', 'dark');
  assert.equal(await page.evaluate(() => document.documentElement.dataset.theme), 'dark');
  const dark = await background();
  assert.notEqual(dark, light);
  const painted = [];
  await page.exposeFunction('reportTheme', value => painted.push(value));
  await page.addInitScript(() => new MutationObserver((_, observer) => { if (document.body) { window.reportTheme(document.documentElement.dataset.theme || 'system'); observer.disconnect(); } }).observe(document, { childList: true, subtree: true }));
  await page.goto(origin + '/people');
  assert.equal(painted[0], 'dark', 'saved theme applied before the body rendered');
  assert.equal(await background(), dark);
  await page.goto(origin + '/admin/metrics');
  assert.equal(await page.locator('.comfy-admin-section').first().evaluate(element => getComputedStyle(element).backgroundColor), await page.locator('main').evaluate(() => getComputedStyle(document.querySelector('.site-header')).backgroundColor), 'metrics panels use the Folkweave paper colour');
  await context.close();
  const blocked = await browser.newContext({ colorScheme: 'dark' });
  await blocked.addInitScript(() => { Object.defineProperty(window, 'localStorage', { get() { throw new DOMException('blocked', 'SecurityError'); } }); });
  const second = await blocked.newPage(); const errors = []; second.on('pageerror', error => errors.push(error.message));
  await signIn(second);
  assert.equal(await second.evaluate(() => document.documentElement.hasAttribute('data-theme')), false);
  await act(second.getByLabel('Color theme', { exact: true }), 'selectOption', 'light');
  assert.equal(await second.evaluate(() => document.documentElement.dataset.theme), 'light');
  assert.deepEqual(errors, []);
  await blocked.close();
  const noscript = await browser.newContext({ javaScriptEnabled: false });
  const third = await noscript.newPage(); await third.goto(origin + '/login');
  assert.equal(await third.getByLabel('Color theme', { exact: true }).isVisible(), false, 'picker hidden without JavaScript');
  await noscript.close();
});

test('Alpine ordering survives a timeline update without losing the draft', async () => {
  const context = await browser.newContext({ viewport: { width: 1280, height: 1000 } });
  const page = await context.newPage(); await signIn(page);
  const src = await page.locator('#feed .photos img').first().getAttribute('src');
  const image = await (await context.request.get(origin + src)).body();
  await act(page.locator('#post-body'), 'fill', 'Do not lose this draft.');
  await act(page.locator('#photos'), 'setInputFiles', [{ name: 'first.jpg', mimeType: 'image/jpeg', buffer: image }, { name: 'second.jpg', mimeType: 'image/jpeg', buffer: image }]);
  await act(page.locator('.photo-draft input').nth(0), 'fill', 'First description');
  await act(page.locator('.photo-draft input').nth(1), 'fill', 'Second description');
  await act(page.locator('.photo-draft').nth(1).getByRole('button', { name: 'Earlier', exact: true }), 'click');
  assert.equal(await page.locator('.photo-draft input').nth(0).inputValue(), 'Second description');
  await page.evaluate(() => window.htmx.ajax('GET', '/', { target: '#feed', swap: 'outerHTML' }));
  assert.equal(await page.locator('#post-body').inputValue(), 'Do not lose this draft.');
  assert.equal(await page.locator('#photos').evaluate(input => input.files[0].name), 'second.jpg');
  assert.equal(await page.locator('.photo-draft input').nth(0).inputValue(), 'Second description');
  await act(page.getByRole('button', { name: 'Share', exact: true }), 'click');
  const post = page.locator('#feed .post').filter({ hasText: 'Do not lose this draft.' }); await post.waitFor();
  assert.equal(await post.locator('img').first().getAttribute('alt'), 'Second description');
  await page.screenshot({ path: root + 'dist/screenshots/timeline-desktop.png', fullPage: true });
  await context.close();
});

test('invitation joins the private group without exposing owner controls', async () => {
  const owner = await browser.newContext(); const page = await owner.newPage(); await signIn(page);
  await page.goto(origin + '/invitations'); await act(page.getByRole('button', { name: 'Create invitation', exact: true }), 'click');
  const invitation = await page.getByLabel('Invitation link', { exact: true }).inputValue();
  const guest = await browser.newContext(); const join = await guest.newPage(); await join.goto(invitation);
  await act(join.getByLabel('Your name', { exact: true }), 'fill', 'Avery');
  await act(join.getByLabel('Username', { exact: true }), 'fill', 'avery');
  await act(join.getByLabel('Choose a password', { exact: true }), 'fill', 'a-private-long-password');
  await act(join.getByRole('button', { name: 'Create account', exact: true }), 'click');
  await join.waitForURL(origin + '/'); assert.equal(await join.getByRole('link', { name: 'Invite someone', exact: true }).count(), 0);
  assert.equal((await join.goto(origin + '/invitations')).status(), 403);
  await owner.close(); await guest.close();
});

test('rapid reply submits issue one request and display one saved reply', async () => {
  const context = await browser.newContext(); const page = await context.newPage(); await signIn(page);
  await act(page.locator('#feed .post').first().getByRole('link', { name: /repl/i }).first(), 'click');
  await act(page.locator('#reply-body'), 'fill', 'Only once, even when the connection is slow.');
  let requests = 0;
  await page.route('**/posts/*/replies', async route => { requests++; const response = await route.fetch(); await new Promise(resolve => setTimeout(resolve, 200)); await route.fulfill({ response }); });
  await page.locator('.reply-form').evaluate(form => { form.requestSubmit(); form.requestSubmit(); });
  await page.locator('.reply .body-text').filter({ hasText: 'Only once, even when the connection is slow.' }).waitFor();
  assert.equal(requests, 1); assert.equal(await page.locator('.reply .body-text').filter({ hasText: 'Only once, even when the connection is slow.' }).count(), 1);
  await context.close();
});
