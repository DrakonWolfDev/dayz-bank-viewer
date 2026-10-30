// Optional development check. Requires Playwright outside the application runtime.
// NODE_PATH=/path/to/node_modules node tools/browser-smoke.cjs /path/to/executable
const { chromium } = require('playwright');
const assert = require('node:assert/strict');
const fs = require('node:fs/promises');
const os = require('node:os');
const path = require('node:path');
const { spawn } = require('node:child_process');

async function main() {
  const temp = await fs.mkdtemp(path.join(os.tmpdir(), 'dayz-bank-browser-'));
  const alpha = path.join(temp, 'Alpha banking'), bravo = path.join(temp, 'Bravo banking');
  await fs.mkdir(alpha); await fs.mkdir(bravo);
  const account = (id, name, balance) => `{"m_Steam64ID":"${id}","m_PlayerName":${JSON.stringify(name)},"m_OwnedCurrency":${balance},"m_BonusCurrency":900,"m_PayCheckBonus":600,"m_ClanID":"Synthetic clan"}`;
  for (let i = 0; i < 65; i++) await fs.writeFile(path.join(alpha, `${i}.json`), account(String(76561198000000000n + BigInt(i)), i === 0 ? '<img src=x onerror=alert(1)>' : `Synthetic Player ${String(i).padStart(2, '0')}`, i === 0 ? 9007199254740993n : i === 1 ? 9007199254740992n : i * 100));
  await fs.writeFile(path.join(bravo, 'shared.json'), account('76561198000000000', 'Shared synthetic player', 300));
  await fs.writeFile(path.join(bravo, 'extra.json'), account('76561198000000999', 'Bravo player', 200));
  const original = new Map(); for (const name of await fs.readdir(alpha)) original.set(name, await fs.readFile(path.join(alpha, name), 'utf8'));
  const server = spawn(path.resolve(process.argv[2] || 'dist/linux-amd64/dayz-bank-viewer'), ['-config', path.join(temp, 'config.json'), '-listen', '127.0.0.1:0']);
  let browser;
  try {
    const url = await new Promise((resolve, reject) => {
      const timer = setTimeout(() => reject(new Error('Server startup timed out')), 10000);
      let logs = ''; server.stderr.on('data', b => { logs += b.toString(); const found = logs.match(/http:\/\/127\.0\.0\.1:\d+/); if (found) { clearTimeout(timer); resolve(found[0]); } });
      server.once('exit', code => { clearTimeout(timer); reject(new Error('Server exited: ' + code + ' ' + logs)); });
    });
    browser = await chromium.launch({ executablePath: process.env.DAYZ_BROWSER || '/usr/bin/google-chrome', headless: true, args: ['--no-sandbox', '--disable-dev-shm-usage'] });
    const page = await browser.newPage({ viewport: { width: 1440, height: 1100 } });
    const errors = [], external = [];
    page.on('pageerror', e => errors.push(e.message)); page.on('request', r => { if (!r.url().startsWith(url)) external.push(r.url()); });
    await page.goto(url); await page.getByRole('heading', { name: 'Connect your first server' }).waitFor();
    await page.getByRole('button', { name: 'Add a server', exact: true }).click();
    await page.getByRole('button', { name: '+ Add server', exact: true }).click();
    await page.getByLabel('Server name', { exact: true }).fill('Alpha'); await page.getByLabel('Absolute banking directory', { exact: true }).fill(alpha);
    await page.getByRole('button', { name: '+ Add server', exact: true }).click();
    await page.getByLabel('Server name', { exact: true }).nth(1).fill('Bravo'); await page.getByLabel('Absolute banking directory', { exact: true }).nth(1).fill(bravo);
    await page.getByLabel('Daily scan times').fill('06:00, 18:00');
    await page.getByRole('button', { name: 'Save settings' }).click();
    await page.waitForFunction(() => document.querySelector('#account-count').textContent === '67' && document.querySelector('#status-badge').textContent === 'Up to date');
    assert.match(await page.locator('#unique-count').textContent(), /66 unique players/);
    assert.equal(await page.locator('#highest-balance').textContent(), '9,007,199,254,740,993');
    assert.match(await page.locator('#accounts tr').first().textContent(), /9,007,199,254,740,993/);
    await page.getByRole('button', { name: '<img src=x onerror=alert(1)>', exact: true }).click();
    assert.equal(await page.locator('#account-dialog img').count(), 0);
    assert.match(await page.locator('#account-details').textContent(), /Bonus currency900Paycheck bonus600/);
    await page.keyboard.press('Escape');
    await page.getByRole('button', { name: 'Next page', exact: true }).click(); assert.match(await page.locator('#page-description').textContent(), /51–67/);
    await page.getByLabel('Search player name or Steam64 ID').fill('76561198000000999'); assert.equal(await page.locator('#accounts tr').count(), 1);
    await page.getByLabel('Search player name or Steam64 ID').fill('no match'); await page.getByRole('heading', { name: 'No matching accounts' }).waitFor();
    await page.getByLabel('Search player name or Steam64 ID').fill('');
    await page.getByRole('button', { name: 'Bank balance', exact: false }).click(); assert.match(await page.locator('#accounts tr').first().textContent(), /200/);
    await page.getByRole('button', { name: 'Player name', exact: false }).click(); assert.equal(await page.locator('#name-heading').getAttribute('aria-sort'), 'ascending');
    await page.getByLabel('Banking source').selectOption({ label: 'Alpha' }); await page.waitForFunction(() => document.querySelector('#account-count').textContent === '65');
    await page.getByRole('button', { name: 'Bank balance', exact: false }).click();
    await page.getByRole('button', { name: 'Bank balance', exact: false }).click();
    // Partial scan -> total failure retaining partial snapshot -> recovery.
    await fs.writeFile(path.join(alpha, 'bad.json'), '{bad'); await page.getByRole('button', { name: 'Refresh server' }).click();
    await page.getByText('Alpha · Incomplete scan', { exact: true }).waitFor(); await page.getByText('View skipped files', { exact: true }).click();
    assert.match(await page.locator('.issue-list').textContent(), /bad.json/);
    for (const name of original.keys()) await fs.writeFile(path.join(alpha, name), '{}');
    await page.getByRole('button', { name: 'Refresh server' }).click(); await page.getByText('Alpha · Scan failed', { exact: true }).waitFor({ timeout: 20000 });
    assert.equal(await page.locator('#account-count').textContent(), '65');
    assert.match(await page.locator('#scan-warning').textContent(), /Showing cached results/);
    for (const [name, text] of original) await fs.writeFile(path.join(alpha, name), text); await fs.unlink(path.join(alpha, 'bad.json'));
    await page.getByRole('button', { name: 'Refresh server' }).click(); await page.waitForFunction(() => document.querySelector('#status-badge').textContent === 'Up to date');
    await page.getByLabel('Banking source').selectOption(''); await page.waitForFunction(() => document.querySelector('#account-count').textContent === '67');
    await page.getByRole('button', { name: 'Settings', exact: true }).click();
    assert.equal(await page.getByLabel('Daily scan times').inputValue(), '06:00, 18:00');
    await page.getByLabel('Daily scan times').fill('25:00'); await page.getByRole('button', { name: 'Save settings' }).click(); await page.locator('#settings-error').waitFor({ state: 'visible' });
    await page.getByLabel('Daily scan times').fill(''); await page.getByLabel('Enabled', { exact: true }).nth(1).uncheck();
    await page.getByRole('button', { name: 'Save settings' }).click(); await page.waitForFunction(() => document.querySelector('#account-count').textContent === '65');
    await page.getByRole('button', { name: 'Bank balance', exact: false }).click();
    await fs.mkdir('artifacts', { recursive: true });
    await page.locator('#notice').evaluate(el => { el.hidden = true; });
    await page.screenshot({ path: 'artifacts/dashboard-desktop.png' });
    await page.setViewportSize({ width: 820, height: 1180 }); await page.screenshot({ path: 'artifacts/dashboard-tablet.png' });
    assert.equal(await page.evaluate(() => document.documentElement.scrollWidth <= innerWidth), true);
    await page.setViewportSize({ width: 390, height: 844 }); await page.screenshot({ path: 'artifacts/dashboard-mobile.png' });
    assert.equal(await page.evaluate(() => document.documentElement.scrollWidth <= innerWidth), true);
    await page.getByRole('button', { name: 'Settings', exact: true }).click(); await page.getByLabel('Server name', { exact: true }).first().fill('Renamed Alpha');
    await page.getByRole('button', { name: 'Save settings' }).click(); await page.waitForFunction(() => [...document.querySelector('#server-select').options].some(o => o.text === 'Renamed Alpha'));
    await page.getByRole('button', { name: 'Settings', exact: true }).click(); await page.getByRole('button', { name: 'Remove configuration' }).first().click();
    await page.getByRole('button', { name: 'Save settings' }).click(); await page.waitForFunction(() => document.querySelector('#account-count').textContent === '0');
    assert.equal((await fs.readdir(alpha)).length, 65);
    for (const [name, text] of original) assert.equal(await fs.readFile(path.join(alpha, name), 'utf8'), text);
    assert.deepEqual(errors, []); assert.deepEqual(external, []);
    console.log('Browser checks passed: onboarding, settings, exact balances, search/sort/pages, details, partial/failure/recovery, disable/rename/remove, responsive layouts, no external requests or page errors.');
  } finally {
    if (browser) await browser.close(); server.kill('SIGTERM');
    await new Promise(resolve => { if (server.exitCode !== null) resolve(); else server.once('exit', resolve); });
    await fs.rm(temp, { recursive: true, force: true });
  }
}
main().catch(e => { console.error(e); process.exitCode = 1; });
