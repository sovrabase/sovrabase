// ===== GLOBAL APP STATE =====
let currentView = 'dashboard';
let allProjects = [];
let deleteTargetId = null;
let detailProjectId = null;
let detailApiKey = null;
let apiKeyRevealed = false;

let activeProjectEnv = {
  collections: [],
  selectedCollection: null,
  documents: [],
  
  users: [],
  
  buckets: [],
  selectedBucket: null,
  files: []
};

// ===== AUTH FLOW =====
function showLogin() {
  document.getElementById('app').style.display = 'none';
  document.getElementById('login-container').style.display = 'flex';
}

function showApp() {
  document.getElementById('login-container').style.display = 'none';
  document.getElementById('app').style.display = 'flex';
  loadDashboard();
}

function logout() {
  localStorage.removeItem('sovrabase_admin_token');
  showLogin();
}

async function handleLoginSubmit(e) {
  e.preventDefault();
  const email = document.getElementById('login-email').value;
  const password = document.getElementById('login-password').value;
  const btn = document.getElementById('btn-login-submit');
  btn.disabled = true;
  btn.innerHTML = '<span class="spinner"></span> Signing in...';
  
  try {
    const data = await api('/admin/login', {
      method: 'POST',
      body: JSON.stringify({ email, password })
    });
    localStorage.setItem('sovrabase_admin_token', data.token);
    showToast('Signed in successfully');
    showApp();
  } catch (err) {
    showToast(err.message, 'error');
  } finally {
    btn.disabled = false;
    btn.innerHTML = 'Sign In';
  }
}

// ===== SUB MODALS =====
function openSubModal(id) {
  document.getElementById(id).style.display = 'flex';
}

function closeSubModal(id) {
  const overlay = document.getElementById(id);
  const inner = document.getElementById(id + '-inner');
  inner.classList.add('closing');
  setTimeout(() => {
    overlay.style.display = 'none';
    inner.classList.remove('closing');
  }, 180);
}

// ===== NAVIGATION =====
function navigateTo(view) {
  currentView = view;

  // Update sidebar - highlight Projects when inside project-detail
  document.querySelectorAll('.nav-item').forEach(el => el.classList.remove('active'));
  const activeSidebarView = view === 'project-detail' ? 'projects' : view;
  const navItem = document.querySelector(`.nav-item[data-view="${activeSidebarView}"]`);
  if (navItem) navItem.classList.add('active');

  // Switch views
  document.querySelectorAll('.view').forEach(el => el.classList.remove('active'));
  const viewEl = document.getElementById('view-' + view);
  if (viewEl) viewEl.classList.add('active');

  // Load data for the view
  if (view === 'dashboard') loadDashboard();
  if (view === 'projects') loadProjects();
  if (view === 'settings') loadSettings();
}

// ===== DASHBOARD STATS =====
async function loadDashboard() {
  try {
    const data = await api('/admin/stats');
    document.getElementById('stat-projects').textContent = data.projects || 0;
    document.getElementById('stat-storage').textContent = data.storage_bytes !== undefined ? formatBytes(data.storage_bytes) : '0 Bytes';
    document.getElementById('stat-region').textContent = data.region || '—';
    const flagMap = { 'eu-west': '🇪🇺 EU West', 'eu-central': '🇪🇺 EU Central', 'eu-north': '🇪🇺 EU North' };
    document.getElementById('stat-region-flag').textContent = flagMap[data.region] || '🇪🇺 ' + (data.region || '');
    document.getElementById('sidebar-version').textContent = 'v' + (data.version || '0.3.0');
    document.getElementById('sidebar-region').textContent = data.region || 'eu-west';

    // Try health for replication info
    try {
      const health = await api('/health');
      if (health.replication) {
        const r = health.replication;
        document.getElementById('replication-info').innerHTML = `
          <div class="info-grid" style="margin-top:4px">
            <div class="detail-row"><span class="detail-label">Role</span><span class="detail-value">${escapeHtml(r.role || 'standalone')}</span></div>
            <div class="detail-row"><span class="detail-label">Active</span><span class="detail-value text-${r.active ? 'success' : 'muted'}">${r.active ? '● Yes' : '○ No'}</span></div>
            <div class="detail-row"><span class="detail-label">Connected Peers</span><span class="detail-value">${r.peers || 0}</span></div>
          </div>`;
      }
    } catch (e) { /* health endpoint may not have replication info */ }
  } catch (e) {
    document.getElementById('stat-projects').textContent = '—';
    document.getElementById('stat-storage').textContent = '— MB';
    document.getElementById('stat-region').textContent = '—';
  }
}

// ===== QUICK START TABS =====
function switchQuickStartTab(name) {
  document.querySelectorAll('.quick-start-tabs .qs-tab').forEach(el => el.classList.remove('active'));
  document.querySelectorAll('.qs-panel').forEach(el => el.classList.remove('active'));
  const tab = document.querySelector(`.quick-start-tabs .qs-tab:nth-child(${
    name === 'create' ? 1 : name === 'flutter' ? 2 : 3})`);
  if (tab) tab.classList.add('active');
  const panel = document.getElementById('qs-' + name);
  if (panel) panel.classList.add('active');
}

// ===== PROJECTS CRUD =====
async function loadProjects() {
  const tbody = document.getElementById('projects-tbody');
  const emptyEl = document.getElementById('projects-empty');
  if (!tbody || !emptyEl) return;
  try {
    const data = await api('/admin/projects');
    allProjects = data.projects || [];
    renderProjects(allProjects, tbody, emptyEl);
  } catch (e) {
    allProjects = [];
    tbody.innerHTML = '';
    emptyEl.style.display = 'block';
    emptyEl.innerHTML = `<div class="empty-state"><div class="empty-state-icon">⚠</div><h3>Failed to load</h3><p>${escapeHtml(e.message)}</p></div>`;
    showToast('Failed to load projects', 'error');
  }
}

function renderProjects(projects, tbody, emptyEl) {
  if (projects.length === 0) {
    tbody.innerHTML = '';
    emptyEl.style.display = 'block';
    emptyEl.innerHTML = `
      <div class="empty-state">
        <div class="empty-state-icon">📦</div>
        <h3>No projects yet</h3>
        <p>Create your first project to get started with Sovrabase.</p>
        <button class="btn btn-primary" onclick="openCreateModal()">✨ Create Project</button>
      </div>`;
    return;
  }
  emptyEl.style.display = 'none';
  tbody.innerHTML = projects.map(p => {
    const date = formatDate(p.created_at);
    const shortId = (p.id || '').substring(0, 8) + '...';
    return `
      <tr data-project-id="${escapeHtml(p.id)}">
        <td class="td-name">${escapeHtml(p.name)}</td>
        <td class="td-id" title="${escapeHtml(p.id)}">${escapeHtml(shortId)}</td>
        <td><span class="badge badge-${p.status || 'active'}"><span class="badge-dot"></span>${escapeHtml(p.status || 'active')}</span></td>
        <td class="td-date">${date}</td>
        <td class="td-actions">
          <button class="btn btn-sm btn-secondary" onclick="openProjectDetailView('${escapeHtml(p.id)}')">View</button>
          <button class="btn btn-sm btn-danger" onclick="openDeleteModal('${escapeHtml(p.id)}', '${escapeHtml(p.name)}')">Delete</button>
        </td>
      </tr>`;
  }).join('');
}

function filterProjects() {
  const query = document.getElementById('project-search').value.toLowerCase();
  const filtered = allProjects.filter(p => (p.name || '').toLowerCase().includes(query));
  renderProjects(filtered, document.getElementById('projects-tbody'), document.getElementById('projects-empty'));
}

// ===== CREATE PROJECT =====
function openCreateModal() {
  document.getElementById('modal-create').style.display = 'flex';
  document.getElementById('create-form-section').style.display = 'block';
  document.getElementById('create-success-section').style.display = 'none';
  document.getElementById('new-project-name').value = '';
  document.getElementById('new-project-owner').value = '';
  setTimeout(() => document.getElementById('new-project-name').focus(), 100);
}

function closeCreateModal() {
  const overlay = document.getElementById('modal-create');
  const inner = document.getElementById('modal-create-inner');
  inner.classList.add('closing');
  setTimeout(() => {
    overlay.style.display = 'none';
    inner.classList.remove('closing');
  }, 180);
}

async function createProject() {
  const nameInput = document.getElementById('new-project-name');
  const ownerInput = document.getElementById('new-project-owner');
  const name = nameInput.value.trim();
  const owner = ownerInput.value.trim() || 'default';

  if (!name) { showToast('Please enter a project name', 'error'); return; }

  const btn = document.getElementById('btn-create-submit');
  btn.disabled = true;
  btn.innerHTML = '<span class="spinner"></span> Creating...';

  try {
    const data = await api('/admin/projects', {
      method: 'POST',
      body: JSON.stringify({ name, owner_id: owner })
    });

    // Show success
    document.getElementById('create-form-section').style.display = 'none';
    document.getElementById('create-success-section').style.display = 'block';
    document.getElementById('create-api-key-value').textContent = data.api_key;
    document.getElementById('create-project-id').textContent = data.project_id || data.project?.id || '';
    document.getElementById('create-api-url').textContent = data.api_url || '';

    showToast(`Project "${name}" created successfully`);
    allProjects = [];
    loadProjects();
    loadDashboard();
  } catch (e) {
    showToast(e.message, 'error');
    btn.disabled = false;
    btn.innerHTML = 'Create Project';
  }
}

// ===== PROJECT DETAIL =====
async function openProjectDetailView(id) {
  detailProjectId = id;
  apiKeyRevealed = false;
  navigateTo('project-detail');

  // Reset tabs
  document.querySelectorAll('#view-project-detail .qs-tab').forEach(el => el.classList.remove('active'));
  const firstTab = document.querySelector('#view-project-detail .qs-tab[data-pdtab="overview"]');
  if (firstTab) firstTab.classList.add('active');
  document.querySelectorAll('#view-project-detail .modal-tab-panel').forEach(el => el.classList.remove('active'));
  const firstPanel = document.getElementById('pdtab-overview');
  if (firstPanel) firstPanel.classList.add('active');

  // Reset API SDK tabs
  document.querySelectorAll('#api-sdk-tabs .qs-tab').forEach(el => el.classList.remove('active'));
  const apiFirstTab = document.querySelector('#api-sdk-tabs .qs-tab');
  if (apiFirstTab) apiFirstTab.classList.add('active');
  document.querySelectorAll('#view-project-detail .qs-panel').forEach(el => el.classList.remove('active'));
  const apiFirstPanel = document.getElementById('api-sdk-curl');
  if (apiFirstPanel) apiFirstPanel.classList.add('active');

  document.getElementById('detail-title').textContent = 'Project Details';

  try {
    const data = await api('/admin/projects/' + encodeURIComponent(id));
    const p = data.project || data;
    detailApiKey = data.api_key || p.api_key || '';

    // Populate overview
    document.getElementById('pdtab-overview-content').innerHTML = `
      <div class="detail-row"><span class="detail-label">Name</span><span class="detail-value">${escapeHtml(p.name)}</span></div>
      <div class="detail-row"><span class="detail-label">ID</span><span class="detail-value mono">${escapeHtml(p.id)}</span></div>
      <div class="detail-row"><span class="detail-label">Status</span><span class="detail-value"><span class="badge badge-${p.status || 'active'}"><span class="badge-dot"></span>${escapeHtml(p.status || 'active')}</span></span></div>
      <div class="detail-row"><span class="detail-label">Created</span><span class="detail-value">${formatDate(p.created_at)}</span></div>
      <div class="detail-row"><span class="detail-label">API URL</span><span class="detail-value mono">${escapeHtml(data.api_url || p.api_url || '')}</span></div>
      <div class="detail-row">
        <span class="detail-label">API Key</span>
        <span class="detail-value">
          <span class="mask-toggle" id="pdtab-apikey-mask" onclick="toggleApiKeyReveal()">•••••••••••• (click to reveal)</span>
          <span id="pdtab-apikey-full" style="display:none;font-family:var(--font-mono);font-size:12px;word-break:break-all;"></span>
        </span>
      </div>`;

    // Update API code snippets
    const ak = detailApiKey || 'API_KEY';
    const pid = p.id || 'PROJECT_ID';
    const hostPort = window.location.origin;
    document.getElementById('api-curl-snippet').textContent = `# Initialize with your project credentials
curl -H "X-Project-Key: ${ak}" \\
  -H "Authorization: Bearer USER_TOKEN" \\
  ${hostPort}/api/v1`;
    document.getElementById('api-dart-snippet').textContent = `final client = SovrabaseClient(
  apiKey: '${ak}',
  projectId: '${pid}',
);`;
    document.getElementById('api-js-snippet').textContent = `const sovrabase = new SovrabaseClient({
  apiKey: '${ak}',
  projectId: '${pid}',
});`;

    document.getElementById('detail-title').textContent = escapeHtml(p.name);
  } catch (e) {
    showToast('Failed to load project details: ' + e.message, 'error');
  }
}

function toggleApiKeyReveal() {
  apiKeyRevealed = !apiKeyRevealed;
  const mask = document.getElementById('pdtab-apikey-mask');
  const full = document.getElementById('pdtab-apikey-full');
  if (apiKeyRevealed) {
    mask.style.display = 'none';
    full.style.display = 'inline';
    full.textContent = detailApiKey || '(not available)';
  } else {
    mask.style.display = 'inline';
    full.style.display = 'none';
  }
}

async function switchProjectDetailTab(name) {
  document.querySelectorAll('#view-project-detail .qs-tab').forEach(el => el.classList.remove('active'));
  const tab = document.querySelector(`#view-project-detail .qs-tab[data-pdtab="${name}"]`);
  if (tab) tab.classList.add('active');
  document.querySelectorAll('#view-project-detail .modal-tab-panel').forEach(el => el.classList.remove('active'));
  const panel = document.getElementById('pdtab-' + name);
  if (panel) panel.classList.add('active');

  if (name === 'database') await loadCollections();
  if (name === 'auth') await loadUsers();
  if (name === 'storage') await loadBuckets();
  if (name === 'logs') await loadProjectLogs();
}

async function loadProjectLogs() {
  const tbody = document.getElementById('logs-tbody');
  if (!tbody) return;

  try {
    const logs = await api(`/admin/projects/${detailProjectId}/logs`);
    
    // Sort logs descending by timestamp
    logs.sort((a, b) => new Date(b.timestamp) - new Date(a.timestamp));

    if (logs.length === 0) {
      tbody.innerHTML = '<tr><td colspan="5" class="text-muted" style="text-align:center;">No logs recorded yet</td></tr>';
      document.getElementById('metrics-total-requests').textContent = '0';
      document.getElementById('metrics-success-rate').textContent = '100%';
      document.getElementById('metrics-error-rate').textContent = '0%';
      return;
    }

    // Calculate metrics
    let total = logs.length;
    let errors = logs.filter(l => l.status >= 400).length;
    let success = total - errors;
    let successRate = ((success / total) * 100).toFixed(1) + '%';
    let errorRate = ((errors / total) * 100).toFixed(1) + '%';

    document.getElementById('metrics-total-requests').textContent = total;
    
    const successRateEl = document.getElementById('metrics-success-rate');
    successRateEl.textContent = successRate;
    if (successRate === '100.0%') successRateEl.textContent = '100%';
    
    const errorRateEl = document.getElementById('metrics-error-rate');
    errorRateEl.textContent = errorRate;
    if (errorRate === '0.0%') errorRateEl.textContent = '0%';
    if (errors > 0) {
      errorRateEl.style.color = 'var(--danger)';
    } else {
      errorRateEl.style.color = 'var(--text-primary)';
    }

    // Render log rows
    tbody.innerHTML = logs.map(l => {
      const date = new Date(l.timestamp).toLocaleString();
      let statusClass = 'text-success';
      if (l.status >= 400) statusClass = 'text-danger';
      else if (l.status >= 300) statusClass = 'text-warning';

      return `<tr>
        <td class="td-date" style="font-size:11.5px;">${escapeHtml(date)}</td>
        <td style="font-weight:700; font-size:11.5px; font-family:var(--font-mono);">${escapeHtml(l.method)}</td>
        <td class="mono" style="font-size:11.5px; max-width:240px; overflow:hidden; text-overflow:ellipsis; white-space:nowrap;" title="${escapeHtml(l.path)}">${escapeHtml(l.path)}</td>
        <td class="${statusClass}" style="font-family:var(--font-mono); font-weight:700; font-size:11.5px;">${l.status}</td>
        <td class="mono" style="font-size:11.5px; color:var(--text-secondary);">${escapeHtml(l.duration)}</td>
      </tr>`;
    }).join('');
  } catch (err) {
    showToast('Failed to load logs: ' + err.message, 'error');
  }
}

async function flushProjectLogs() {
  if (!await showConfirm('Clear Request Logs', 'Are you sure you want to clear all request logs for this project?', 'Clear', true)) return;
  try {
    await api(`/admin/projects/${detailProjectId}/logs`, {
      method: 'DELETE'
    });
    showToast('Request logs cleared');
    await loadProjectLogs();
  } catch (err) {
    showToast(err.message, 'error');
  }
}

function switchApiSdkTab(name) {
  document.querySelectorAll('#api-sdk-tabs .qs-tab').forEach(el => el.classList.remove('active'));
  document.querySelectorAll('#view-project-detail .qs-panel').forEach(el => el.classList.remove('active'));
  const tabs = document.querySelectorAll('#api-sdk-tabs .qs-tab');
  const idx = { curl: 0, dart: 1, js: 2 }[name] || 0;
  if (tabs[idx]) tabs[idx].classList.add('active');
  const panel = document.getElementById('api-sdk-' + name);
  if (panel) panel.classList.add('active');
}

// ===== DELETE PROJECT CONFIRMATION =====
function openDeleteModal(id, name) {
  deleteTargetId = id;
  document.getElementById('delete-project-name').textContent = name;
  document.getElementById('modal-delete').style.display = 'flex';
}

function closeDeleteModal() {
  const overlay = document.getElementById('modal-delete');
  const inner = document.getElementById('modal-delete-inner');
  inner.classList.add('closing');
  setTimeout(() => {
    overlay.style.display = 'none';
    inner.classList.remove('closing');
  }, 180);
  deleteTargetId = null;
}

async function confirmDeleteProject() {
  if (!deleteTargetId) return;
  const btn = document.getElementById('btn-confirm-delete');
  btn.disabled = true;
  btn.innerHTML = '<span class="spinner"></span> Deleting...';

  try {
    await api('/admin/projects/' + encodeURIComponent(deleteTargetId), { method: 'DELETE' });
    closeDeleteModal();
    showToast('Project deleted');
    allProjects = [];
    loadProjects();
    loadDashboard();
  } catch (e) {
    showToast(e.message, 'error');
    btn.disabled = false;
    btn.innerHTML = 'Delete Project';
  }
}

// ===== SETTINGS VIEW =====
async function loadSettings() {
  try {
    // System stats (version, region etc.)
    const stats = await api('/admin/stats');
    document.getElementById('sys-version').textContent = stats.version || '0.3.0';
    document.getElementById('sys-go-version').textContent = stats.go_version || 'go1.23+';
    document.getElementById('sys-region').textContent = stats.region || 'eu-west';
    const driverMap = { 'local': 'Local Filesystem', 's3': 'S3 Object Storage' };
    document.getElementById('sys-storage-driver').textContent = driverMap[stats.storage_driver] || 'Local Filesystem';
    if (stats.replication) {
      document.getElementById('sys-repl-role').textContent = stats.replication.role || 'standalone';
    }
  } catch (e) { /* ignore stats errors */ }

  try {
    // Editable config
    const cfg = await api('/admin/config');

    // Populate info row
    if (document.getElementById('sys-listen-addr')) document.getElementById('sys-listen-addr').textContent = cfg.listen_addr || '—';
    if (document.getElementById('sys-data-dir')) document.getElementById('sys-data-dir').textContent = cfg.data_dir || '—';
    if (document.getElementById('sys-config-file')) document.getElementById('sys-config-file').textContent = cfg.config_file || 'data/config.yaml';

    // Admin account
    document.getElementById('cfg-admin-email').value = cfg.admin_email || '';
    document.getElementById('cfg-admin-password').value = '';
    document.getElementById('cfg-admin-password-confirm').value = '';
    document.getElementById('cfg-session-duration').value = cfg.session_duration || '24h';

    // S3 fields
    const s3Enabled = cfg.s3_enabled === true;
    document.getElementById('cfg-s3-enabled').checked = s3Enabled;
    document.getElementById('s3-form-fields').style.display = s3Enabled ? 'grid' : 'none';
    document.getElementById('cfg-s3-endpoint').value = cfg.s3_endpoint || '';
    document.getElementById('cfg-s3-prefix').value = cfg.s3_bucket_prefix || '';
    document.getElementById('cfg-s3-access-key').value = cfg.s3_access_key || '';
    document.getElementById('cfg-s3-secret-key').value = cfg.s3_secret_key || '';
    document.getElementById('cfg-s3-ssl').checked = cfg.s3_use_ssl !== false;

    // Replication fields
    const roleEl = document.getElementById('cfg-role');
    roleEl.value = cfg.role || '';
    document.getElementById('cfg-node-id').value = cfg.node_id || '';
    document.getElementById('cfg-repl-addr').value = cfg.repl_addr || '';
    document.getElementById('cfg-peers').value = (cfg.peers || []).join('\n');

  } catch (e) {
    showToast('Failed to load config: ' + e.message, 'error');
  }
}

function toggleS3Form() {
  const enabled = document.getElementById('cfg-s3-enabled').checked;
  document.getElementById('s3-form-fields').style.display = enabled ? 'grid' : 'none';
}

async function saveConfig() {
  const btn = document.getElementById('btn-save-config');
  btn.disabled = true;
  btn.textContent = 'Saving…';

  // Admin password validation
  const newPwd = document.getElementById('cfg-admin-password').value;
  const confirmPwd = document.getElementById('cfg-admin-password-confirm').value;
  if (newPwd && newPwd !== confirmPwd) {
    showToast('Passwords do not match', 'error');
    btn.disabled = false;
    btn.innerHTML = '💾 Save Config';
    return;
  }

  const peersRaw = document.getElementById('cfg-peers').value;
  const peers = peersRaw.split('\n').map(p => p.trim()).filter(p => p !== '');
  const secretKey = document.getElementById('cfg-s3-secret-key').value;

  try {
    await api('/admin/config', {
      method: 'POST',
      body: JSON.stringify({
        admin_email:      document.getElementById('cfg-admin-email').value,
        admin_password:   newPwd || '••••••••',
        session_duration: document.getElementById('cfg-session-duration').value || '24h',
        s3_enabled:       document.getElementById('cfg-s3-enabled').checked,
        s3_endpoint:      document.getElementById('cfg-s3-endpoint').value,
        s3_access_key:    document.getElementById('cfg-s3-access-key').value,
        s3_secret_key:    secretKey || '••••••••',
        s3_bucket_prefix: document.getElementById('cfg-s3-prefix').value,
        s3_use_ssl:       document.getElementById('cfg-s3-ssl').checked,
        role:             document.getElementById('cfg-role').value,
        node_id:          document.getElementById('cfg-node-id').value,
        repl_addr:        document.getElementById('cfg-repl-addr').value,
        peers:            peers,
      })
    });
    showToast('Config saved to config.yaml ✓', 'success');
    // Clear password fields after save
    document.getElementById('cfg-admin-password').value = '';
    document.getElementById('cfg-admin-password-confirm').value = '';
  } catch (e) {
    showToast('Save failed: ' + e.message, 'error');
  } finally {
    btn.disabled = false;
    btn.innerHTML = '💾 Save Config';
  }
}

async function restartServer() {
  const ok = await showConfirm(
    '⚡ Restart Server',
    'The config will be saved and the server will restart. The dashboard will reconnect automatically in a few seconds.',
    'Save & Restart',
    false
  );
  if (!ok) return;

  const btn = document.getElementById('btn-restart-server');
  btn.disabled = true;
  btn.textContent = 'Restarting…';

  try {
    // Save first, then restart
    await saveConfig();
    await api('/admin/restart', { method: 'POST' });
  } catch (e) {
    // The server will drop the connection — that's expected
  }

  showToast('Server restarting… reconnecting shortly', 'info');

  // Poll until server is back
  setTimeout(async function poll() {
    try {
      await fetch('/admin/health');
      showToast('Server is back online ✓', 'success');
      btn.disabled = false;
      btn.innerHTML = '⚡ Save &amp; Restart';
      await loadSettings();
    } catch (_) {
      setTimeout(poll, 1500);
    }
  }, 2000);
}

// ===== MODAL OVERLAY CLICK TO CLOSE =====
document.addEventListener('click', function(e) {
  if (e.target.classList.contains('modal-overlay')) {
    if (e.target.id === 'modal-create') closeCreateModal();
    if (e.target.id === 'modal-delete') closeDeleteModal();
    if (e.target.id === 'modal-create-col') closeSubModal('modal-create-col');
    if (e.target.id === 'modal-create-doc') closeSubModal('modal-create-doc');
    if (e.target.id === 'modal-import-col') closeSubModal('modal-import-col');
    if (e.target.id === 'modal-create-user') closeSubModal('modal-create-user');
    if (e.target.id === 'modal-create-bucket') closeSubModal('modal-create-bucket');
    if (e.target.id === 'modal-preview-file') closeSubModal('modal-preview-file');
  }
});

// ===== KEYBOARD SHORTCUTS =====
document.addEventListener('keydown', function(e) {
  if (e.key === 'Escape') {
    if (document.getElementById('modal-create').style.display === 'flex') closeCreateModal();
    if (document.getElementById('modal-delete').style.display === 'flex') closeDeleteModal();
    if (document.getElementById('modal-create-col').style.display === 'flex') closeSubModal('modal-create-col');
    if (document.getElementById('modal-create-doc').style.display === 'flex') closeSubModal('modal-create-doc');
    if (document.getElementById('modal-import-col').style.display === 'flex') closeSubModal('modal-import-col');
    if (document.getElementById('modal-create-user').style.display === 'flex') closeSubModal('modal-create-user');
    if (document.getElementById('modal-create-bucket').style.display === 'flex') closeSubModal('modal-create-bucket');
    if (document.getElementById('modal-preview-file').style.display === 'flex') closeSubModal('modal-preview-file');
  }
  // Enter in create modal
  if (e.key === 'Enter' && document.getElementById('modal-create').style.display === 'flex') {
    const nameEl = document.getElementById('new-project-name');
    if (document.activeElement === nameEl) {
      createProject();
    }
  }
});

// ===== INITIAL LOAD =====
function checkAuth() {
  const token = localStorage.getItem('sovrabase_admin_token');
  if (!token) {
    showLogin();
  } else {
    showApp();
  }
}
checkAuth();
