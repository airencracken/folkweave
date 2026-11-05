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
  server = spawn(process.env.FOLKWEAVE_BINARY || root + 'dist/folkweave', ['demo', '--listen', '127.0.0.1:0'], { env: { ...process.env, FOLKWEAVE_BACKUP_INTERVAL: '0' }, stdio: ['ignore', 'pipe', 'pipe'] });
  let errors = '';
  server.stderr.on('data', data => { errors += data; });
  origin = await new Promise((resolve, reject) => {
    const timeout = setTimeout(() => reject(new Error('Demo startup timed out: ' + errors)), 15000);
    server.once('error', reject);
    server.once('exit', code => { clearTimeout(timeout); reject(new Error('Demo exited: ' + code + ' ' + errors)); });
    server.stdout.on('data', data => { const match = data.toString().match(/listening at (http:\/\/[^\s]+)/); if (match) { clearTimeout(timeout); resolve(match[1]); } });
  });
  browser = await chromium.launch({ executablePath: process.env.CHROMIUM || (existsSync('/usr/bin/chromium') ? '/usr/bin/chromium' : undefined), headless: true, args: ['--no-sandbox'] });
  await mkdir(root + 'dist/screenshots', { recursive: true });
  const preview = await browser.newContext({ viewport: {width:1280,height:1100} });
  const page = await preview.newPage(); await signIn(page);
  await page.screenshot({path:root+'dist/screenshots/first-pass-desktop.png'});
  await page.setViewportSize({width:390,height:1000});
  await page.screenshot({path:root+'dist/screenshots/first-pass-mobile.png'});
  await preview.close();
});
after(async () => { if (browser) await browser.close(); if (server && server.exitCode === null) { const done = new Promise(resolve => server.once('exit', resolve)); server.kill('SIGTERM'); await done; } });

async function signIn(page) {
  await page.goto(origin + '/login');
  await page.getByLabel('Username', { exact: true }).fill('demo');
  await page.getByLabel('Password', { exact: true }).fill('folkweave-demo-only');
  await page.getByRole('button', { name: 'Sign in', exact: true }).click();
  await page.waitForURL(origin + '/');
}

for (const javascript of [true, false]) for (const width of [320, 390, 768, 1280]) {
  test(`timeline, photo post, reply, and bookmark: JS=${javascript}, width=${width}`, async () => {
    const context = await browser.newContext({ javaScriptEnabled: javascript, viewport: { width, height: 960 }, colorScheme: width === 390 || width === 1280 ? 'dark' : 'light' });
    const page = await context.newPage();
    const errors = []; page.on('pageerror', error => errors.push(error.message));
    if (javascript) await page.addInitScript(() => { window.cspViolations = []; document.addEventListener('securitypolicyviolation', event => window.cspViolations.push(event.violatedDirective)); });
    await signIn(page);
    await page.locator('#feed .post').first().waitFor();
    const source = await page.locator('#feed .photos img').first().getAttribute('src');
    const response = await context.request.get(origin + source); assert.equal(response.status(), 200);
    const photo = await response.body();
    const text = `A quiet first post ${javascript}-${width}`;
    await page.locator('#post-body').fill(text);
    if (javascript && width === 320) {
      await page.locator('#photos').setInputFiles({ name: 'invalid.jpg', mimeType: 'image/jpeg', buffer: Buffer.from('not a photograph') });
      await page.getByRole('button', { name: 'Share', exact: true }).click();
      await page.locator('#composer-error [role=alert]').waitFor();
      assert.equal(await page.locator('#post-body').inputValue(), text);
      assert.equal(await page.locator('#photos').evaluate(input => input.files.length), 1);
    }
    await page.locator('#photos').setInputFiles({ name: 'woodland.jpg', mimeType: 'image/jpeg', buffer: photo });
    if (javascript) {
      await page.locator('.photo-draft input').fill('A woodland path with a pale sky');
      assert.equal(await page.locator('.photo-draft').count(), 1);
    } else await page.getByLabel('First photo description', { exact: true }).fill('A woodland path with a pale sky');
    await page.getByRole('button', { name: 'Share', exact: true }).click();
    const post = page.locator('#feed .post').filter({ hasText: text });
    await post.waitFor();
    assert.equal(await post.locator('img[alt="A woodland path with a pale sky"]').count(), 1);
    assert.equal(await page.locator('#post-body').inputValue(), '');
    await post.getByRole('link', { name: 'Reply', exact: true }).click();
    await page.locator('#reply-body').fill('Good to see this here.');
    await page.getByRole('button', { name: 'Post reply', exact: true }).click();
    await page.locator('.reply .body-text').filter({ hasText: 'Good to see this here.' }).waitFor();
    assert.equal(await page.locator('#reply-body').inputValue(), '');
    assert.equal(await page.locator('#remove-post').count(), 1, 'reply refresh duplicated post controls');
    await page.getByRole('button', { name: 'Save', exact: true }).click();
    await page.getByRole('button', { name: 'Saved', exact: true }).waitFor();
    await page.getByRole('link', { name: 'Saved', exact: true }).click();
    await page.locator('#feed .post').filter({ hasText: text }).waitFor();
    const overflow = await page.evaluate(() => document.documentElement.scrollWidth > window.innerWidth);
    assert.equal(overflow, false, 'horizontal overflow');
    assert.deepEqual(errors, [], 'browser exceptions');
    if (javascript) assert.deepEqual(await page.evaluate(() => window.cspViolations), [], 'CSP violations');
    await page.screenshot({ path: root + `dist/screenshots/saved-${javascript}-${width}.png`, fullPage: true });
    await context.close();
  });
}

test('Alpine ordering survives a timeline update without losing the draft', async () => {
  const context = await browser.newContext({ viewport: { width: 1280, height: 1000 } });
  const page = await context.newPage(); await signIn(page);
  const src = await page.locator('#feed .photos img').first().getAttribute('src');
  const image = await (await context.request.get(origin + src)).body();
  await page.locator('#post-body').fill('Do not lose this draft.');
  await page.locator('#photos').setInputFiles([{ name: 'first.jpg', mimeType: 'image/jpeg', buffer: image }, { name: 'second.jpg', mimeType: 'image/jpeg', buffer: image }]);
  await page.locator('.photo-draft input').nth(0).fill('First description');
  await page.locator('.photo-draft input').nth(1).fill('Second description');
  await page.locator('.photo-draft').nth(1).getByRole('button', { name: 'Earlier', exact: true }).click();
  assert.equal(await page.locator('.photo-draft input').nth(0).inputValue(), 'Second description');
  await page.evaluate(() => window.htmx.ajax('GET', '/', { target: '#feed', swap: 'outerHTML' }));
  assert.equal(await page.locator('#post-body').inputValue(), 'Do not lose this draft.');
  assert.equal(await page.locator('#photos').evaluate(input => input.files[0].name), 'second.jpg');
  assert.equal(await page.locator('.photo-draft input').nth(0).inputValue(), 'Second description');
  await page.getByRole('button', { name: 'Share', exact: true }).click();
  const post = page.locator('#feed .post').filter({ hasText: 'Do not lose this draft.' }); await post.waitFor();
  assert.equal(await post.locator('img').first().getAttribute('alt'), 'Second description');
  await page.screenshot({ path: root + 'dist/screenshots/timeline-desktop.png', fullPage: true });
  await context.close();
});

test('invitation joins the private gathering without exposing owner controls', async () => {
  const owner = await browser.newContext(); const page = await owner.newPage(); await signIn(page);
  await page.goto(origin + '/invitations'); await page.getByRole('button', { name: 'Create invitation', exact: true }).click();
  const invitation = await page.getByLabel('Invitation link', { exact: true }).inputValue();
  const guest = await browser.newContext(); const join = await guest.newPage(); await join.goto(invitation);
  await join.getByLabel('Your name', { exact: true }).fill('Avery');
  await join.getByLabel('Username', { exact: true }).fill('avery');
  await join.getByLabel('Choose a password', { exact: true }).fill('a-private-long-password');
  await join.getByRole('button', { name: 'Join this gathering', exact: true }).click();
  await join.waitForURL(origin + '/'); assert.equal(await join.getByRole('link', { name: 'Invite someone', exact: true }).count(), 0);
  assert.equal((await join.goto(origin + '/invitations')).status(), 403);
  await owner.close(); await guest.close();
});

test('rapid reply submits issue one request and display one saved reply', async () => {
  const context = await browser.newContext(); const page = await context.newPage(); await signIn(page);
  await page.locator('#feed .post').first().getByRole('link', { name: /repl/i }).first().click();
  await page.locator('#reply-body').fill('Only once, even when the connection is slow.');
  let requests = 0;
  await page.route('**/posts/*/replies', async route => { requests++; const response = await route.fetch(); await new Promise(resolve => setTimeout(resolve, 200)); await route.fulfill({ response }); });
  await page.locator('.reply-form').evaluate(form => { form.requestSubmit(); form.requestSubmit(); });
  await page.locator('.reply .body-text').filter({ hasText: 'Only once, even when the connection is slow.' }).waitFor();
  assert.equal(requests, 1); assert.equal(await page.locator('.reply .body-text').filter({ hasText: 'Only once, even when the connection is slow.' }).count(), 1);
  await context.close();
});
