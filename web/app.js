'use strict';
const $ = id => document.getElementById(id);
const state = { data: null, selected: '', query: '', sort: 'balance', ascending: false, page: 1, pageSize: 50, config: null, timer: null, loadingSettings: false, saving: false };
const money = value => BigInt(value ?? '0').toLocaleString();
const stamp = value => value ? new Date(value).toLocaleString(undefined, { dateStyle: 'medium', timeStyle: 'short' }) : 'Never';
const statusNames = { ready: 'Up to date', partial: 'Incomplete scan', error: 'Scan failed', scanning: 'Scanning', disabled: 'Disabled', not_scanned: 'Not scanned' };
function node(tag, text, className) { const el = document.createElement(tag); if (text !== undefined) el.textContent = text; if (className) el.className = className; return el; }
async function api(path, options = {}) {
  const res = await fetch(path, { cache: 'no-store', ...options, headers: { 'Content-Type': 'application/json', ...options.headers } });
  const body = await res.json(); if (!res.ok) throw new Error(body.error || `Request failed (${res.status})`); return body;
}
function notice(text) { $('notice').textContent = text; $('notice').hidden = !text; }
function activeServers() { return state.data.servers.filter(s => state.selected ? s.id === state.selected : s.enabled); }
function schedulePoll() {
  clearTimeout(state.timer); if (document.hidden) return;
  const busy = state.data?.servers.some(s => s.status === 'scanning');
  state.timer = setTimeout(load, busy ? 1000 : 30000);
}
async function load() {
  const selected = state.selected;
  try {
    const data = await api('/api/dashboard' + (selected ? '?server=' + encodeURIComponent(selected) : ''));
    if (selected !== state.selected) return;
    state.data = data; render();
  } catch (err) { notice('Could not load the dashboard. ' + err.message); }
  finally { schedulePoll(); }
}
function render() {
  const d = state.data;
  const select = $('server-select'); select.replaceChildren(new Option('All servers', ''));
  d.servers.forEach(s => select.add(new Option(s.name + (s.enabled ? '' : ' (disabled)'), s.id)));
  select.value = state.selected;
  $('account-count').textContent = d.account_count.toLocaleString();
  const enabledCount = d.servers.filter(s => s.enabled).length;
  $('unique-count').textContent = state.selected ? 'Accounts on this server' : `${d.unique_players.toLocaleString()} unique ${d.unique_players === 1 ? 'player' : 'players'} · ${enabledCount} enabled ${enabledCount === 1 ? 'server' : 'servers'}`;
  $('total-balance').textContent = money(d.total); $('highest-balance').textContent = d.account_count ? money(d.highest) : '—';
  for (const id of ['total-balance', 'highest-balance']) {
    const el = $(id), long = el.textContent.length > 16;
    el.classList.toggle('long-number', long); el.closest('.stat').classList.toggle('has-long-number', long);
  }
  const servers = activeServers(), busy = servers.some(s => s.status === 'scanning');
  const badge = $('status-badge');
  badge.textContent = !servers.length ? 'No servers connected' : busy ? 'Scanning' : d.complete ? 'Up to date' : 'Needs attention';
  badge.dataset.kind = busy || !servers.length ? 'neutral' : d.complete ? 'ready' : 'warning';
  const successes = servers.filter(s => s.last_success).map(s => new Date(s.last_success).getTime());
  const oldest = successes.length === servers.length && successes.length ? Math.min(...successes) : null;
  $('last-refresh').textContent = oldest ? 'Last successful refresh: ' + stamp(oldest) : 'Last successful refresh: ' + (servers.length === 1 ? stamp(servers[0].last_success) : 'not all servers have completed a scan');
  $('refresh').disabled = !servers.some(s => s.enabled && s.status !== 'scanning');
  $('refresh').textContent = busy ? '↻ Scanning…' : state.selected ? '↻ Refresh server' : '↻ Refresh all';
  renderWarnings(servers); renderTable();
}
function renderWarnings(servers) {
  const box = $('scan-warning'); box.replaceChildren();
  for (const s of servers) {
    if (s.status === 'ready') continue;
    const article = node('div');
    article.append(node('strong', `${s.name} · ${statusNames[s.status]}`));
    if (s.error) article.append(node('p', s.error + (s.snapshot ? ' Showing cached results from ' + stamp(s.snapshot.scanned_at) + '.' : ' No cached results are available.')));
    if (s.failed_files?.length) {
      const details = node('details'); details.append(node('summary', `View ${s.failed_files.length} unreadable files from the failed scan`));
      const list = node('ul', undefined, 'issue-list');
      s.failed_files.forEach(issue => list.append(node('li', `${issue.file}: ${issue.message}`)));
      details.append(list); article.append(details);
    }
    if (s.status === 'disabled') article.append(node('p', 'Automatic and manual scans are paused. Cached accounts, if any, remain visible here and are excluded from All servers.'));
    if (s.status === 'not_scanned') article.append(node('p', 'Refresh this server to read its banking files.'));
    if (s.status === 'scanning') article.append(node('p', 'Reading accounts. Existing results stay visible until the scan finishes.'));
    if (s.snapshot?.issues.length) {
      article.append(node('p', `${s.snapshot.issues.length} of ${s.snapshot.files} files skipped. These totals are incomplete. Scan: ${stamp(s.snapshot.scanned_at)}.`));
      const details = node('details'); details.append(node('summary', 'View skipped files'));
      const list = node('ul', undefined, 'issue-list');
      s.snapshot.issues.forEach(issue => list.append(node('li', `${issue.file}: ${issue.message}`)));
      details.append(list); article.append(details);
    }
    if (s.snapshot && s.snapshot.files === 0) article.append(node('p', 'The directory contains no account JSON files.'));
    box.append(article);
  }
  box.hidden = box.childElementCount === 0;
}
function renderTable() {
  if (!state.data) return;
  const query = state.query.toLocaleLowerCase();
  const rows = state.data.accounts.filter(a => a.name.toLocaleLowerCase().includes(query) || a.steam_id.includes(query));
  rows.sort((a, b) => {
    let result;
    if (state.sort === 'name') result = a.name.localeCompare(b.name, undefined, { sensitivity: 'base' });
    else { const av = BigInt(a.balance), bv = BigInt(b.balance); result = av < bv ? -1 : av > bv ? 1 : 0; }
    return (state.ascending ? result : -result) || a.rank - b.rank;
  });
  const pages = Math.max(1, Math.ceil(rows.length / state.pageSize)); state.page = Math.min(state.page, pages);
  const start = (state.page - 1) * state.pageSize;
  $('accounts').replaceChildren();
  for (const a of rows.slice(start, start + state.pageSize)) {
    const tr = node('tr'); tr.append(node('td', String(a.rank).padStart(2, '0'), 'rank' + (a.rank <= 3 ? ' top-rank' : '')));
    const name = node('td'), button = node('button', undefined, 'player-button'); button.type = 'button';
    const initials = (a.name.trim() || '?').split(/\s+/).slice(0, 2).map(s => Array.from(s)[0]).join('').toLocaleUpperCase();
    const avatar = node('span', initials, 'avatar'); avatar.setAttribute('aria-hidden', 'true');
    button.append(avatar, node('span', a.name || 'Unnamed player')); button.addEventListener('click', () => showAccount(a)); name.append(button); tr.append(name);
    tr.append(node('td', a.steam_id, 'steam'));
    if (!state.selected) tr.append(node('td', a.server_name, 'server-name'));
    tr.append(node('td', money(a.balance), 'numeric balance')); $('accounts').append(tr);
  }
  $('server-heading').hidden = !!state.selected;
  $('result-count').textContent = rows.length.toLocaleString();
  $('empty').hidden = rows.length > 0;
  const noServers = state.data.servers.length === 0;
  $('empty-title').textContent = noServers ? 'Connect your first server' : query ? 'No matching accounts' : 'No accounts to display';
  $('empty-text').textContent = noServers ? 'Add a local banking directory to start viewing accounts.' : query ? 'Try a different player name or Steam64 ID.' : 'Check the scan information above, or refresh your banking source.';
  $('empty-settings').hidden = !noServers;
  $('page-description').textContent = rows.length ? `${start + 1}–${Math.min(start + state.pageSize, rows.length)} of ${rows.length.toLocaleString()} accounts` + (query ? ' matching your search' : '') : 'No accounts to display';
  $('page-number').textContent = `${state.page} / ${pages}`; $('previous').disabled = state.page <= 1; $('next').disabled = state.page >= pages;
  for (const field of ['name', 'balance']) {
    const active = state.sort === field;
    $(field + '-heading').setAttribute('aria-sort', active ? (state.ascending ? 'ascending' : 'descending') : 'none');
    $('sort-' + field).querySelector('span').textContent = active ? (state.ascending ? '↑' : '↓') : '↕';
  }
}
function showAccount(a) {
  $('account-title').textContent = a.name || 'Unnamed player'; $('account-details').replaceChildren();
  const fields = [['Server', a.server_name], ['Steam64 ID', a.steam_id], ['Bank balance', money(a.balance)], ['Bonus currency', a.bonus === undefined ? 'Not supplied' : money(a.bonus)], ['Paycheck bonus', a.paycheck === undefined ? 'Not supplied' : money(a.paycheck)], ['Clan ID', a.clan_id || 'None']];
  fields.forEach(([label, value]) => $('account-details').append(node('dt', label), node('dd', value)));
  $('account-dialog').showModal();
}
async function openSettings() {
  if (state.loadingSettings || $('settings').open) return;
  state.loadingSettings = true;
  try {
    state.config = await api('/api/config'); $('server-fields').replaceChildren();
    state.config.servers.forEach(addServerField);
    $('refresh-times').value = state.config.refresh_times.join(', '); $('listen').value = state.config.listen;
    $('timezone-description').textContent = `Times use the server computer’s local timezone${state.data ? ' (' + state.data.timezone + '). Current host time: ' + state.data.host_time : '.'}`;
    $('settings-error').hidden = true; $('settings').showModal();
  } catch (err) { notice('Could not open settings. ' + err.message); }
  finally { state.loadingSettings = false; }
}
let fieldSequence = 0;
function addServerField(server = {}) {
  // addEventListener passes an Event, so only explicitly supplied server data is used.
  const field = node('fieldset', undefined, 'server-fields'); field.dataset.id = server.id || '';
  const legend = node('legend', server.name || 'New server', 'sr-only'); field.append(legend);
  const header = node('div', undefined, 'server-field-header');
  const enabledLabel = node('label', undefined, 'checkbox-label'), enabled = node('input'); enabled.type = 'checkbox'; enabled.name = 'enabled'; enabled.checked = server.enabled ?? true;
  enabledLabel.append(enabled, document.createTextNode('Enabled'));
  const remove = node('button', 'Remove configuration', 'remove-button'); remove.type = 'button'; remove.addEventListener('click', () => field.remove()); header.append(enabledLabel, remove); field.append(header);
  for (const [key, label, placeholder, value] of [['name', 'Server name', 'e.g. Chernarus', server.name], ['path', 'Absolute banking directory', 'e.g. D:/DayZ/Server1/Banking', server.path]]) {
    const id = 'field-' + (++fieldSequence); const input = node('input'); input.id = id; input.name = key; input.required = true; if (key === 'name') input.maxLength = 120;
    input.placeholder = placeholder; input.value = value || ''; input.autocomplete = 'off'; input.spellcheck = false;
    const lbl = node('label', label); lbl.htmlFor = id; field.append(lbl, input);
  }
  $('server-fields').append(field);
  return field;
}
async function saveSettings(event) {
  event.preventDefault(); if (state.saving) return;
  state.saving = true; $('save-settings').disabled = true; $('settings-error').hidden = true;
  const previousListen = state.config.listen;
  const config = { listen: $('listen').value.trim(), servers: [...$('server-fields').children].map(f => ({ id: f.dataset.id, name: f.elements.name.value.trim(), path: f.elements.path.value.trim(), enabled: f.elements.enabled.checked })), refresh_times: $('refresh-times').value.split(',').map(s => s.trim()).filter(Boolean) };
  try {
    state.config = await api('/api/config', { method: 'PUT', body: JSON.stringify(config) });
    if (state.selected && !state.config.servers.some(s => s.id === state.selected)) state.selected = '';
    state.page = 1; $('settings').close();
    notice(previousListen !== state.config.listen ? 'Settings saved. Restart DayZ Bank Viewer to apply the new listen address.' : 'Settings saved. Refreshing enabled servers.');
    await api('/api/refresh', { method: 'POST', body: '{}' }); await load();
  } catch (err) {
    if ($('settings').open) { $('settings-error').textContent = err.message; $('settings-error').hidden = false; }
    else notice('Settings saved, but the refresh could not start. ' + err.message);
  } finally { state.saving = false; $('save-settings').disabled = false; }
}
$('server-select').addEventListener('change', () => { state.selected = $('server-select').value; state.page = 1; load(); });
$('search').addEventListener('input', () => { state.query = $('search').value; state.page = 1; renderTable(); });
for (const field of ['name', 'balance']) $('sort-' + field).addEventListener('click', () => { state.ascending = state.sort === field ? !state.ascending : field === 'name'; state.sort = field; state.page = 1; renderTable(); });
$('previous').addEventListener('click', () => { state.page--; renderTable(); }); $('next').addEventListener('click', () => { state.page++; renderTable(); });
$('refresh').addEventListener('click', async () => {
  $('refresh').disabled = true; notice('');
  try { const result = await api('/api/refresh', { method: 'POST', body: JSON.stringify({ server_id: state.selected }) }); if (!result.started) notice('No new scans started. Enabled servers may already be scanning.'); await load(); }
  catch (err) { notice('Could not refresh. ' + err.message); $('refresh').disabled = false; }
});
$('settings-open').addEventListener('click', openSettings); $('empty-settings').addEventListener('click', openSettings);
$('settings-close').addEventListener('click', () => $('settings').close()); $('settings-cancel').addEventListener('click', () => $('settings').close());
$('add-server').addEventListener('click', () => { const f = addServerField(); f.querySelector('input[name="name"]').focus(); });
$('settings-form').addEventListener('submit', saveSettings);
$('account-close').addEventListener('click', () => $('account-dialog').close());
document.addEventListener('visibilitychange', () => { if (document.hidden) clearTimeout(state.timer); else load(); });
load();

api('/api/version').then(data => document.querySelectorAll('.version').forEach(el => { el.textContent = 'v' + data.version; })).catch(() => document.querySelectorAll('.version').forEach(el => { el.textContent = 'Unavailable'; }));
