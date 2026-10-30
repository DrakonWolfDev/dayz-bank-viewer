'use strict';
const $ = id => document.getElementById(id);
const state = { data: null, selected: '', accountType: 'player', query: '', sort: 'balance', ascending: false, page: 1, pageSize: 50, config: null, timer: null, loadingSettings: false, saving: false };
const money = value => BigInt(value ?? '0').toLocaleString();
const stamp = value => value ? new Date(value).toLocaleString(undefined, { dateStyle: 'medium', timeStyle: 'short' }) : 'Never';
const statusNames = { ready: 'Up to date', partial: 'Incomplete scan', error: 'Scan failed', scanning: 'Scanning', disabled: 'Disabled', not_scanned: 'Not scanned' };
const formatNames = { banking: 'Banking / KR Banking compatible', dc_banking: 'DayZ-Banking (DC Banking)', expansion: 'DayZ Expansion ATM', kr_clan: 'KR Banking clan', expansion_party: 'Expansion party' };
const playerID = account => account.group_id || account.steam_id || account.player_id || '';
function node(tag, text, className) { const el = document.createElement(tag); if (text !== undefined) el.textContent = text; if (className) el.className = className; return el; }
async function api(path, options = {}) {
  const res = await fetch(path, { cache: 'no-store', ...options, headers: { 'Content-Type': 'application/json', ...options.headers } });
  const body = await res.json(); if (!res.ok) throw new Error(body.error || `Request failed (${res.status})`); return body;
}
function notice(text) { $('notice').textContent = text; $('notice').hidden = !text; }
function viewServers() { return state.data.servers.filter(s => (s.account_type || 'player') === state.accountType); }
function activeServers() { return viewServers().filter(s => state.selected ? s.id === state.selected : s.enabled); }
function schedulePoll() {
  clearTimeout(state.timer); if (document.hidden) return;
  const busy = state.data?.servers.some(s => s.status === 'scanning');
  state.timer = setTimeout(load, busy ? 1000 : 30000);
}
async function load() {
  const selected = state.selected, accountType = state.accountType;
  try {
    const data = await api('/api/dashboard?account_type=' + accountType + (selected ? '&server=' + encodeURIComponent(selected) : ''));
    if (selected !== state.selected || accountType !== state.accountType) return;
    state.data = data; render();
  } catch (err) { notice('Could not load the dashboard. ' + err.message); }
  finally { schedulePoll(); }
}
function render() {
  const d = state.data, clans = state.accountType === 'clan';
  $('account-category').textContent = clans ? 'CLANS & PARTIES' : 'PLAYER ACCOUNTS';
  $('name-column-label').textContent = clans ? 'Clan / party name' : 'Player name';
  $('id-heading').textContent = clans ? 'Clan / party ID' : 'Player ID';
  $('search-label').textContent = clans ? 'Search clan or party name or ID' : 'Search player name or player ID';
  $('search').placeholder = clans ? 'Search clan / party name or ID…' : 'Search name or player ID…';
  $('highest-description').textContent = clans ? 'Largest shared account' : 'Largest individual account';
  $('account-caption').textContent = clans ? 'Shared accounts ranked by bank balance. Select a clan or party for details.' : 'Player accounts ranked by owned currency. Select a player for account details.';
  const select = $('server-select'); select.replaceChildren(new Option(clans ? 'All clan sources' : 'All servers', ''));
  viewServers().forEach(s => select.add(new Option(s.name + (s.enabled ? '' : ' (disabled)'), s.id)));
  select.value = state.selected;
  $('account-count').textContent = d.account_count.toLocaleString();
  const enabledCount = viewServers().filter(s => s.enabled).length;
  $('total-balance').textContent = money(d.total); $('highest-balance').textContent = d.account_count ? money(d.highest) : '—';
  for (const id of ['total-balance', 'highest-balance']) {
    const el = $(id), long = el.textContent.length > 16;
    el.classList.toggle('long-number', long); el.closest('.stat').classList.toggle('has-long-number', long);
  }
  const servers = activeServers(), busy = servers.some(s => s.status === 'scanning');
  $('banking-formats').textContent = servers.filter(s => s.snapshot?.formats?.length).map(s => `${s.name}: ${s.snapshot.formats.map(f => formatNames[f] || f).join(', ')}`).join(' · ');
  $('banking-formats').hidden = !$('banking-formats').textContent;
  $('unique-count').textContent = state.selected ? (clans ? 'Shared accounts on this source' : 'Accounts on this server') : clans ? `Shared accounts · ${enabledCount} enabled clan sources` : `${d.unique_players.toLocaleString()} unique player IDs · ${enabledCount} enabled ${enabledCount === 1 ? 'server' : 'servers'}`;
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
    const lookup = s.snapshot?.name_lookup;
    if (s.status === 'ready' && !lookup?.issues.length) continue;
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
    if (lookup?.issues.length) {
      const details = node('details'); details.append(node('summary', 'Player name lookup information'));
      const list = node('ul', undefined, 'issue-list');
      lookup.issues.forEach(issue => list.append(node('li', `${issue.file}: ${issue.message}`)));
      details.append(list); article.append(node('p', 'Some admin logs could not be searched fully. Bank balances are unaffected.'), details);
    }
    box.append(article);
  }
  box.hidden = box.childElementCount === 0;
}
function renderTable() {
  if (!state.data) return;
  const query = state.query.toLocaleLowerCase();
  const rows = state.data.accounts.filter(a => a.name.toLocaleLowerCase().includes(query) || playerID(a).toLocaleLowerCase().includes(query));
  rows.sort((a, b) => {
    let result;
    if (state.sort === 'rank') result = a.rank - b.rank;
    else if (state.sort === 'name') result = a.name.localeCompare(b.name, undefined, { sensitivity: 'base' });
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
    button.append(avatar, node('span', a.name || (a.group_id ? 'Unnamed clan / party' : 'Unnamed player'))); button.addEventListener('click', () => showAccount(a)); name.append(button); tr.append(name);
    const id = node('td', playerID(a), 'steam'); id.title = a.group_id ? 'Clan / party ID' : a.player_id ? 'DayZ player ID (Steam64 ID not supplied)' : 'Steam64 ID'; tr.append(id);
    if (!state.selected) tr.append(node('td', a.server_name, 'server-name'));
    tr.append(node('td', money(a.balance), 'numeric balance')); $('accounts').append(tr);
  }
  $('server-heading').hidden = !!state.selected;
  $('result-count').textContent = rows.length.toLocaleString();
  $('empty').hidden = rows.length > 0;
  const noServers = viewServers().length === 0;
  $('empty-title').textContent = noServers ? (state.accountType === 'clan' ? 'Connect a clan account folder' : 'Connect your first server') : query ? 'No matching accounts' : 'No accounts to display';
  $('empty-text').textContent = noServers ? (state.accountType === 'clan' ? 'In Settings, add a source with account type Clans & parties. Choose KR_BANKING/ClanDataBase or ExpansionMod/Groups.' : 'Add a local banking directory to start viewing accounts.') : query ? 'Try a different player name or player ID.' : 'Check the scan information above, or refresh your banking source.';
  $('empty-settings').hidden = !noServers;
  $('page-description').textContent = rows.length ? `${start + 1}–${Math.min(start + state.pageSize, rows.length)} of ${rows.length.toLocaleString()} accounts` + (query ? ' matching your search' : '') : 'No accounts to display';
  $('page-number').textContent = `${state.page} / ${pages}`; $('previous').disabled = state.page <= 1; $('next').disabled = state.page >= pages;
  for (const field of ['rank', 'name', 'balance']) {
    const active = state.sort === field;
    $(field + '-heading').setAttribute('aria-sort', active ? (state.ascending ? 'ascending' : 'descending') : 'none');
    $('sort-' + field).querySelector('span[aria-hidden]').textContent = active ? (state.ascending ? '↑' : '↓') : '↕';
  }
}
function showAccount(a) {
  $('account-title').textContent = a.name || (a.group_id ? 'Unnamed clan / party' : 'Unnamed player'); $('account-details').replaceChildren();
  const fields = [['Server', a.server_name], ['Banking format', formatNames[a.format] || 'Unknown'],
    ...(a.group_id ? [['Clan / party ID', a.group_id], ['Tag', a.tag || 'Not supplied'], ['Owner ID', a.owner_id || 'Not supplied'], ['Owner name', a.owner_name || 'Not supplied']] : [['Steam64 ID', a.steam_id || 'Not supplied'], ...(a.player_id ? [['DayZ player ID', a.player_id], ['Player name', a.name || 'Not found in local admin logs'], ['Name source', a.name_source === 'admin_log' ? 'Local ADM log (last observed name)' : 'Not supplied by this format']] : [])]),
    ['Bank balance', money(a.balance)],
    ...(!a.group_id ? [['Bonus currency', a.bonus === undefined ? 'Not supplied' : money(a.bonus)], ['Paycheck bonus', a.paycheck === undefined ? 'Not supplied' : money(a.paycheck)], ['Account capacity bonus', a.capacity_bonus === undefined ? 'Not supplied' : money(a.capacity_bonus)], ['Clan ID', a.clan_id || 'None']] : [])];
  fields.forEach(([label, value]) => $('account-details').append(node('dt', label), node('dd', value)));
  $('account-dialog').showModal();
}
async function openSettings() {
  if (state.loadingSettings || $('settings').open) return;
  state.loadingSettings = true;
  try {
    state.config = await api('/api/config'); resetDiscovery();
    for (const id of ['discovery-root', 'discovery-config', 'discovery-name']) $(id).value = '';
    $('discovery-config').closest('details').open = false;
    $('server-fields').replaceChildren();
    state.config.servers.forEach(addServerField);
    setupMode = ''; $('settings-details').hidden = !state.config.servers.length; $('source-editor').hidden = !state.config.servers.length; $('source-editor').open = false; $('add-server').hidden = true; $('connection-settings').open = false; updateSourceSummary(); updateDiscoveryChoice();
    $('refresh-times').value = state.config.refresh_times.join(', '); $('listen').value = state.config.listen;
    $('timezone-description').textContent = `Times use the server computer’s local timezone${state.data ? ' (' + state.data.timezone + '). Current host time: ' + state.data.host_time : '.'}`;
    $('settings-error').hidden = true; $('settings').showModal();
  } catch (err) { notice('Could not open settings. ' + err.message); }
  finally { state.loadingSettings = false; }
}
let setupMode = '';
let discoveryGeneration = 0;
let discovery = null, discoveryRows = [], detecting = false;
function sourcePathKey(path) {
  let key = path.trim().replaceAll('\\', '/').replace(/\/$/, '');
  return /^[A-Za-z]:\//.test(key) ? key.toLocaleLowerCase() : key;
}
function configuredSourcePaths() { return new Set([...$('server-fields').children].map(f => sourcePathKey(f.elements.path.value))); }
function resetDiscovery() {
  discoveryGeneration++; discovery = null; discoveryRows = [];
  $('discovery-preview').hidden = true; $('discovery-status').textContent = ''; updateDiscoveryChoice();
}
function showDiscoveryMission() {
  const value = $('discovery-mission').value;
  const mission = discovery && value !== '' ? discovery.missions[Number(value)] : null;
  $('discovery-mission-details').textContent = mission ? `Mission: ${mission.template}. Configuration: ${mission.config_path}` : discovery?.missions.length ? 'Choose the configuration and mission to review.' : 'The server name can be entered manually when no mission setting is available.';
}
function updateSourceSummary() {
  const count = $('server-fields').children.length;
  $('source-summary').textContent = count ? `${count} account ${count === 1 ? 'source' : 'sources'} in these settings.` : '';
}
function chooseSetupMode(mode) {
  setupMode = mode; $('settings-details').hidden = false;
  $('source-editor').hidden = mode !== 'manual'; $('source-editor').open = mode === 'manual'; $('add-server').hidden = mode !== 'manual';

}
function discoveryChoicesValid() {
  return discovery && !(discovery.missions.length > 1 && !$('discovery-mission').value) && $('discovery-name').value.trim() && discoveryRows.some(row => row.checkbox.checked && !row.checkbox.disabled);
}
function updateDiscoveryChoice() {
  const needsMission = discovery?.missions.length > 1 && !$('discovery-mission').value;
  $('add-detected').disabled = detecting || state.saving || needsMission || !discoveryChoicesValid();
  const pendingChoices = setupMode === 'detect' && discoveryRows.some(row => !row.checkbox.disabled);
  const noSources = !$('server-fields').children.length;
  $('save-settings').disabled = detecting || state.saving || (setupMode === 'detect' && ((pendingChoices && !discoveryChoicesValid()) || (noSources && !discoveryChoicesValid())));
}
async function detectServerFolders() {
  if (detecting || state.saving) return;
  // An untouched manual row can be abandoned; retain any partially entered row.
  for (const field of [...$('server-fields').children]) {
    if (!field.dataset.id && !field.elements.name.value.trim() && !field.elements.path.value.trim()) field.remove();
    else if (!field.elements.name.validity.valid || !field.elements.path.validity.valid) {
      chooseSetupMode('manual'); field.querySelector('input:invalid').reportValidity(); return;
    }
  }
  updateSourceSummary(); chooseSetupMode('detect'); resetDiscovery(); const generation = discoveryGeneration; detecting = true;
  $('discover-server').disabled = true; $('save-settings').disabled = true;
  $('discovery-status').textContent = 'Checking server folders…';
  try {
    const result = await api('/api/discover', { method: 'POST', body: JSON.stringify({ root: $('discovery-root').value.trim(), config_path: $('discovery-config').value.trim() }) });
    if (generation !== discoveryGeneration || !$('settings').open) return;
    discovery = result;
    const missionSelect = $('discovery-mission'); missionSelect.replaceChildren();
    if (result.missions.length !== 1) missionSelect.add(new Option(result.missions.length ? 'Choose the configuration and mission' : 'No mission found — enter a server name below', ''));
    result.missions.forEach((mission, index) => missionSelect.add(new Option(`${mission.map} · ${mission.template} · ${mission.config_path}`, String(index))));
    missionSelect.disabled = !result.missions.length; showDiscoveryMission();
    $('discovery-name').value = result.missions.length === 1 ? result.missions[0].map : '';
    $('discovery-warnings').replaceChildren(...result.warnings.map(text => node('li', text)));
    $('discovery-sources').replaceChildren(); discoveryRows = [];
    const existing = configuredSourcePaths();
    result.sources.forEach((source, index) => {
      const article = node('div', undefined, 'discovery-source');
      const checkbox = node('input'); checkbox.type = 'checkbox';
      const duplicate = source.already_configured || existing.has(sourcePathKey(source.path));
      checkbox.checked = source.verified && !duplicate; checkbox.disabled = duplicate;
      const label = node('label', undefined, 'checkbox-label');
      label.append(checkbox, document.createTextNode(`${source.label} · ${source.account_type === 'clan' ? 'Clans & parties' : 'Players'}`));
      article.append(label, node('p', source.path), node('p', duplicate ? 'Already in your settings.' : source.note, 'muted'));
      let logSelect = null;
      if (source.format === 'expansion') {
        const id = 'discovery-log-' + index, logLabel = node('label', 'ADM log folders'); logLabel.htmlFor = id;
        logSelect = node('select'); logSelect.id = id;
        logSelect.add(new Option('Automatic — combine recognised folders', ''));
        source.log_directories.forEach(path => logSelect.add(new Option(path, path)));
        article.append(logLabel, logSelect, node('p', source.log_directories.length ? 'Automatic lookup: ' + source.log_directories.join(' · ') : 'No automatic log folders found. You can supply a path in the source settings.', 'muted'));
      }
      checkbox.addEventListener('change', updateDiscoveryChoice);
      discoveryRows.push({ source, checkbox, logSelect }); $('discovery-sources').append(article);
    });
    $('discovery-preview').hidden = false;
    $('discovery-status').textContent = result.sources.length ? `Found ${result.sources.length} account folders. Review the mission and tick the sources you want.` : 'No supported account folders found in the recognised locations. You can still add a source manually below.';
  } catch (err) { if (generation === discoveryGeneration && $('settings').open) $('discovery-status').textContent = 'Could not detect server folders. ' + err.message; }
  finally { detecting = false; $('discover-server').disabled = false; $('save-settings').disabled = state.saving; updateDiscoveryChoice(); }
}
for (const id of ['discovery-root', 'discovery-config']) $(id).addEventListener('keydown', event => { if (event.key === 'Enter') { event.preventDefault(); detectServerFolders(); } });
$('discovery-root').addEventListener('input', resetDiscovery);
$('discovery-config').addEventListener('input', resetDiscovery);
$('discover-server').addEventListener('click', detectServerFolders);
$('discovery-mission').addEventListener('change', () => {
  const value = $('discovery-mission').value;
  if (discovery && value !== '') $('discovery-name').value = discovery.missions[Number(value)].map;
  showDiscoveryMission(); updateDiscoveryChoice();
});
$('discovery-name').addEventListener('input', updateDiscoveryChoice);
function selectedDetectedSources() {
  if (!discoveryChoicesValid()) return [];
  const existing = configuredSourcePaths();
  return discoveryRows.filter(row => row.checkbox.checked && !row.checkbox.disabled && !existing.has(sourcePathKey(row.source.path))).map(row => ({ name: `${$('discovery-name').value.trim()} — ${row.source.label}`, path: row.source.path, account_type: row.source.account_type, log_path: row.logSelect?.value || '', enabled: true }));
}
function stageDetectedSources() {
  if (!discoveryChoicesValid()) return 0;
  const existing = configuredSourcePaths(); let added = 0;
  for (const row of discoveryRows) {
    if (!row.checkbox.checked || row.checkbox.disabled || existing.has(sourcePathKey(row.source.path))) continue;
    addServerField({ name: `${$('discovery-name').value.trim()} — ${row.source.label}`, path: row.source.path, account_type: row.source.account_type, log_path: row.logSelect?.value || '', enabled: true });
    existing.add(sourcePathKey(row.source.path)); row.checkbox.checked = false; row.checkbox.disabled = true; added++;
  }
  updateSourceSummary(); updateDiscoveryChoice(); return added;
}
$('add-detected').addEventListener('click', () => {
  if (detecting || state.saving || !discoveryChoicesValid()) return;
  stageDetectedSources(); resetDiscovery();
  $('discovery-root').value = ''; $('discovery-config').value = ''; $('discovery-config').closest('details').open = false;
  $('discovery-status').textContent = 'Server added to these settings. Enter another server directory, or save when you are ready.';
  $('discovery-root').focus();
});
function manuallyAddServer() {
  if (detecting || state.saving) return;
  resetDiscovery(); chooseSetupMode('manual');
  const f = addServerField(); updateSourceSummary(); updateDiscoveryChoice(); f.querySelector('input[name="name"]').focus();
}
$('manual-server').addEventListener('click', manuallyAddServer);
let fieldSequence = 0;
function addServerField(server = {}) {
  // addEventListener passes an Event, so only explicitly supplied server data is used.
  const field = node('fieldset', undefined, 'server-fields'); field.dataset.id = server.id || '';
  const legend = node('legend', server.name || 'New server', 'sr-only'); field.append(legend);
  const header = node('div', undefined, 'server-field-header');
  const enabledLabel = node('label', undefined, 'checkbox-label'), enabled = node('input'); enabled.type = 'checkbox'; enabled.name = 'enabled'; enabled.checked = server.enabled ?? true;
  enabledLabel.append(enabled, document.createTextNode('Enabled'));
  const remove = node('button', 'Remove configuration', 'remove-button'); remove.type = 'button'; remove.addEventListener('click', () => { field.remove(); updateSourceSummary(); updateDiscoveryChoice(); }); header.append(enabledLabel, remove); field.append(header);
  const typeID = 'field-' + (++fieldSequence), typeLabel = node('label', 'Account type'), typeSelect = node('select');
  typeLabel.htmlFor = typeID; typeSelect.id = typeID; typeSelect.name = 'account_type';
  typeSelect.add(new Option('Players', 'player')); typeSelect.add(new Option('Clans & parties', 'clan')); typeSelect.value = server.account_type || 'player';
  field.append(typeLabel, typeSelect);
  for (const [key, label, placeholder, value] of [['name', 'Server name', 'e.g. Chernarus', server.name], ['path', 'Absolute banking directory', 'e.g. D:/DayZ/Server1/Banking', server.path], ['log_path', 'Admin log directory (optional)', 'e.g. D:/DayZ/Server1/profiles', server.log_path]]) {
    const id = 'field-' + (++fieldSequence); const input = node('input'); input.id = id; input.name = key; input.required = key !== 'log_path'; if (key === 'name') input.maxLength = 120;
    input.placeholder = placeholder; input.value = value || ''; input.autocomplete = 'off'; input.spellcheck = false;
    const lbl = node('label', label); lbl.htmlFor = id; field.append(lbl, input);
  }
  field.append(node('small', 'ADM logs can supply names for Expansion accounts. Leave blank to check profiles and recognised log_storage folders automatically for an ExpansionMod/ATM source. Logs are read only when accounts refresh.', 'field-help'));
  $('server-fields').append(field);
  return field;
}
async function saveSettings(event) {
  event.preventDefault(); if (state.saving || detecting || $('save-settings').disabled) return;
  const detectedSources = setupMode === 'detect' ? selectedDetectedSources() : [];
  state.saving = true; $('save-settings').disabled = true; $('settings-error').hidden = true;
  const previousListen = state.config.listen;
  const config = { listen: $('listen').value.trim(), servers: [...$('server-fields').children].map(f => ({ id: f.dataset.id, name: f.elements.name.value.trim(), path: f.elements.path.value.trim(), account_type: f.elements.account_type.value, log_path: f.elements.log_path.value.trim(), enabled: f.elements.enabled.checked })).concat(detectedSources), refresh_times: $('refresh-times').value.split(',').map(s => s.trim()).filter(Boolean) };
  try {
    state.config = await api('/api/config', { method: 'PUT', body: JSON.stringify(config) });
    if (state.selected && !state.config.servers.some(s => s.id === state.selected && (s.account_type || 'player') === state.accountType)) state.selected = '';
    state.page = 1; $('settings').close();
    notice(previousListen !== state.config.listen ? 'Settings saved. Restart DayZ Bank Viewer to apply the new listen address.' : 'Settings saved. Refreshing enabled servers.');
    await api('/api/refresh', { method: 'POST', body: '{}' }); await load();
  } catch (err) {
    if ($('settings').open) { $('settings-error').textContent = err.message; $('settings-error').hidden = false; }
    else notice('Settings saved, but the refresh could not start. ' + err.message);
  } finally { state.saving = false; updateDiscoveryChoice(); }
}
$('account-view').addEventListener('change', () => { state.accountType = $('account-view').value; state.selected = ''; state.query = ''; $('search').value = ''; state.page = 1; load(); });
$('server-select').addEventListener('change', () => { state.selected = $('server-select').value; state.page = 1; load(); });
$('search').addEventListener('input', () => { state.query = $('search').value; state.page = 1; renderTable(); });
for (const field of ['rank', 'name', 'balance']) $('sort-' + field).addEventListener('click', () => { state.ascending = state.sort === field ? !state.ascending : field !== 'balance'; state.sort = field; state.page = 1; renderTable(); });
$('previous').addEventListener('click', () => { state.page--; renderTable(); }); $('next').addEventListener('click', () => { state.page++; renderTable(); });
$('refresh').addEventListener('click', async () => {
  $('refresh').disabled = true; notice('');
  try { const result = await api('/api/refresh', { method: 'POST', body: JSON.stringify({ server_id: state.selected }) }); if (!result.started) notice('No new scans started. Enabled servers may already be scanning.'); await load(); }
  catch (err) { notice('Could not refresh. ' + err.message); $('refresh').disabled = false; }
});
$('settings-open').addEventListener('click', openSettings); $('empty-settings').addEventListener('click', openSettings);
for (const id of ['help-open', 'settings-help']) $(id).addEventListener('click', () => $('help-dialog').showModal());
$('help-close').addEventListener('click', () => $('help-dialog').close());
$('settings-close').addEventListener('click', () => $('settings').close()); $('settings-cancel').addEventListener('click', () => $('settings').close());
$('add-server').addEventListener('click', manuallyAddServer);
$('settings-form').addEventListener('submit', saveSettings);
$('settings-form').addEventListener('invalid', event => { const details = event.target.closest('details'); if (details) details.open = true; }, true);
$('settings').addEventListener('close', resetDiscovery);
$('account-close').addEventListener('click', () => $('account-dialog').close());
document.addEventListener('visibilitychange', () => { if (document.hidden) clearTimeout(state.timer); else load(); });
load();

api('/api/version').then(data => document.querySelectorAll('.version').forEach(el => { el.textContent = 'v' + data.version; })).catch(() => document.querySelectorAll('.version').forEach(el => { el.textContent = 'Unavailable'; }));
$('check-updates').addEventListener('click', async () => {
  $('check-updates').disabled = true; $('update-link').hidden = true; $('update-result').textContent = 'Checking GitHub…';
  try { const result = await api('/api/updates', { method: 'POST', body: '{}' }); $('update-result').textContent = result.message;
    if (result.status === 'available') { $('update-link').href = result.url; $('update-link').hidden = false; }
  } catch (err) { $('update-result').textContent = 'Could not check for updates. ' + err.message; }
  finally { $('check-updates').disabled = false; }
});
