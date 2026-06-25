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
  oauthProviders: [],
  
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
  if (view === 'plugins') loadPlugins();
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

    // Load usage stats
    try {
      const usage = await api('/admin/stats/usage');
      if (usage.enabled) {
        document.getElementById('usage-total-requests').textContent = (usage.total_requests || 0).toLocaleString();
        document.getElementById('usage-bandwidth-up').textContent = formatBytes(usage.total_bandwidth_up || 0);
        document.getElementById('usage-bandwidth-down').textContent = formatBytes(usage.total_bandwidth_down || 0);
        document.getElementById('usage-tracking-status').textContent = 'Active';
        document.getElementById('usage-tracking-status').style.color = 'var(--success)';
      } else {
        document.getElementById('usage-total-requests').textContent = '—';
        document.getElementById('usage-bandwidth-up').textContent = '—';
        document.getElementById('usage-bandwidth-down').textContent = '—';
        document.getElementById('usage-tracking-status').textContent = 'Disabled';
        document.getElementById('usage-tracking-status').style.color = 'var(--text-muted)';
      }
    } catch (e) {
      // Usage endpoint may not be available
    }
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
      <!-- Left Card: Project Info & Credentials -->
      <div class="overview-card" style="background: var(--bg-card-elevated); border: 1px solid var(--border); border-radius: var(--radius); padding: 20px; display: flex; flex-direction: column; gap: 12px;">
        <h3 style="font-size: 13px; font-weight: 700; color: var(--accent); margin: 0 0 8px 0; text-transform: uppercase; letter-spacing: 0.5px;">📋 Project Information</h3>
        <div class="detail-row"><span class="detail-label">Name</span><span class="detail-value">${escapeHtml(p.name)}</span></div>
        <div class="detail-row"><span class="detail-label">ID</span><span class="detail-value mono" style="font-size:11px;">${escapeHtml(p.id)}</span></div>
        <div class="detail-row"><span class="detail-label">Status</span><span class="detail-value"><span class="badge badge-${p.status || 'active'}"><span class="badge-dot"></span>${escapeHtml(p.status || 'active')}</span></span></div>
        <div class="detail-row"><span class="detail-label">Created</span><span class="detail-value">${formatDate(p.created_at)}</span></div>
        
        <h3 style="font-size: 13px; font-weight: 700; color: var(--accent); margin: 16px 0 8px 0; text-transform: uppercase; letter-spacing: 0.5px;">🔑 API Credentials</h3>
        <div class="detail-row"><span class="detail-label">API URL</span><span class="detail-value mono" style="font-size:11px;">${escapeHtml(data.api_url || p.api_url || '')}</span></div>
        <div class="detail-row">
          <span class="detail-label">API Key</span>
          <span class="detail-value">
            <span class="mask-toggle" id="pdtab-apikey-mask" onclick="toggleApiKeyReveal()">•••••••••••• (click to reveal)</span>
            <span id="pdtab-apikey-full" style="display:none;font-family:var(--font-mono);font-size:12px;word-break:break-all;"></span>
          </span>
        </div>
      </div>

      <!-- Right Card: Project Settings -->
      <div class="overview-card" style="background: var(--bg-card-elevated); border: 1px solid var(--border); border-radius: var(--radius); padding: 20px; display: flex; flex-direction: column; justify-content: space-between; gap: 16px;">
        <div>
          <h3 style="font-size: 13px; font-weight: 700; color: var(--accent); margin: 0 0 16px 0; text-transform: uppercase; letter-spacing: 0.5px;">⚙️ Project Settings</h3>
          <div class="form-group" style="margin: 0 0 16px 0;">
            <label for="pd-allow-origins" style="font-size: 10px; font-weight: 700; letter-spacing: 0.8px; color: var(--text-muted); text-transform: uppercase; display: block; margin-bottom: 6px;">Allowed Origins (CORS)</label>
            <input type="text" id="pd-allow-origins" value="${escapeHtml(data.allow_origins || '*')}" style="width:100%; font-family: var(--font-mono); font-size: 12px; padding: 8px 12px; background: var(--bg-input); border: 1px solid var(--border); border-radius: var(--radius); color: var(--text-primary);">
          </div>
          <div class="form-group" style="margin: 0;">
            <label for="pd-storage-quota" style="font-size: 10px; font-weight: 700; letter-spacing: 0.8px; color: var(--text-muted); text-transform: uppercase; display: block; margin-bottom: 6px;">Storage Quota Limit</label>
            <div style="display:flex;gap:8px;align-items:stretch;">
              <input type="number" id="pd-storage-quota" value="${humanizeBytes(data.storage_quota || 104857600).val}" min="0" style="flex:1; font-family: var(--font-mono); font-size: 12px; padding: 8px 12px; background: var(--bg-input); border: 1px solid var(--border); border-radius: var(--radius); color: var(--text-primary);">
              <select id="pd-storage-quota-unit" style="width:80px; background:var(--bg-input); border:1px solid var(--border); border-radius:var(--radius); color:var(--text-primary); font-size:12px; padding:8px;">
                <option value="MB" ${humanizeBytes(data.storage_quota || 104857600).unit === 'MB' ? 'selected' : ''}>MB</option>
                <option value="GB" ${humanizeBytes(data.storage_quota || 104857600).unit === 'GB' ? 'selected' : ''}>GB</option>
                <option value="TB" ${humanizeBytes(data.storage_quota || 104857600).unit === 'TB' ? 'selected' : ''}>TB</option>
              </select>
            </div>
          </div>
        </div>
        <div style="display: flex; align-items: center; gap: 12px; margin-top: auto;">
          <button class="btn btn-primary" id="btn-save-project-settings" onclick="saveProjectSettings()" style="padding: 10px 20px;">💾 Save Settings</button>
          <span id="pd-save-status" style="font-size: 12px; color: var(--text-secondary);"></span>
        </div>
      </div>

      <!-- Bottom Card: Database Storage Analysis -->
      <div class="overview-card" style="background: var(--bg-card-elevated); border: 1px solid var(--border); border-radius: var(--radius); padding: 20px; grid-column: span 2; display: flex; flex-direction: column; gap: 12px;">
        <div style="display: flex; justify-content: space-between; align-items: center;">
          <div>
            <h3 style="font-size: 13px; font-weight: 700; color: var(--accent); margin: 0; text-transform: uppercase; letter-spacing: 0.5px;">🗄️ Database Storage Analysis</h3>
            <p style="font-size: 12px; color: var(--text-muted); margin: 4px 0 0 0;">
              Calculate real disk usage details for this project's database, including metadata, secondary index overhead, and per-collection breakdowns.
            </p>
          </div>
          <button class="btn btn-secondary btn-sm" onclick="runDeepDbAnalysis()">🔍 Run Deep Analysis</button>
        </div>
        <div id="db-analysis-result" style="display:none; margin-top:8px; padding:16px; background: rgba(0,0,0,0.2); border: 1px solid var(--border); border-radius: var(--radius);">
        </div>
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

async function saveProjectSettings() {
  if (!detailProjectId) return;
  const btn = document.getElementById('btn-save-project-settings');
  const status = document.getElementById('pd-save-status');
  btn.disabled = true;
  btn.textContent = 'Saving...';
  status.textContent = '';

  try {
    const allowOrigins = document.getElementById('pd-allow-origins').value.trim();
    const quotaVal = parseFloat(document.getElementById('pd-storage-quota').value) || 0;
    const quotaUnit = document.getElementById('pd-storage-quota-unit').value;
    const storageQuota = quotaToBytes(quotaVal, quotaUnit);

    const data = await api('/admin/projects/' + encodeURIComponent(detailProjectId), {
      method: 'PUT',
      body: JSON.stringify({
        allow_origins: allowOrigins,
        storage_quota: storageQuota
      })
    });

    showToast('Project settings saved');
    status.textContent = '✓ Saved';
    status.style.color = 'var(--success)';
    // Refresh the view to show updated values
    openProjectDetailView(detailProjectId);
  } catch (e) {
    showToast('Failed to save settings: ' + e.message, 'error');
    status.textContent = '✗ ' + e.message;
    status.style.color = 'var(--danger)';
  } finally {
    btn.disabled = false;
    btn.textContent = '💾 Save Settings';
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
  if (name === 'team') await loadTeam();
  if (name === 'auth') { await loadUsers(); await loadOAuthProviders(); }
  if (name === 'storage') await loadBuckets();
  if (name === 'config') await loadRemoteConfig();
  if (name === 'cron') await loadCronJobs();
  if (name === 'webhooks') await loadWebhooks();
  if (name === 'analytics') await loadAnalytics();
  if (name === 'queues') await loadQueues();
  if (name === 'logs') await loadProjectLogs();
}

// ===== TEAM MANAGEMENT =====
async function loadTeam() {
  const tbody = document.getElementById('team-members-tbody');
  if (!tbody) return;
  try {
    const data = await api(`/admin/projects/${detailProjectId}/members`);
    const members = data.members || [];
    if (members.length === 0) {
      tbody.innerHTML = '<tr><td colspan="4" class="text-muted" style="text-align:center;">No team members yet</td></tr>';
      return;
    }
    tbody.innerHTML = members.map(m => {
      const joined = formatDate(m.joined_at);
      const userId = m.user_id || '';
      const role = m.role || 'developer';
      const shortId = userId.substring(0, 8) + '...';
      const roleBadgeClass = role === 'owner' ? 'badge-suspended' : role === 'admin' ? 'badge-active' : 'badge-active';
      return `<tr>
        <td class="td-id" title="${escapeHtml(userId)}">${escapeHtml(shortId)}</td>
        <td>
          <select class="role-select" data-user-id="${escapeHtml(userId)}" onchange="changeMemberRole('${escapeHtml(userId)}', this.value)" style="padding:3px 6px;font-size:11px;background:var(--bg-input);border:1px solid var(--border);border-radius:4px;color:var(--text-primary);">
            <option value="owner" ${role === 'owner' ? 'selected' : ''}>Owner</option>
            <option value="admin" ${role === 'admin' ? 'selected' : ''}>Admin</option>
            <option value="developer" ${role === 'developer' ? 'selected' : ''}>Developer</option>
            <option value="viewer" ${role === 'viewer' ? 'selected' : ''}>Viewer</option>
          </select>
        </td>
        <td class="td-date">${joined}</td>
        <td class="td-actions">
          <button class="btn btn-xs btn-danger" onclick="removeMember('${escapeHtml(userId)}')">Remove</button>
        </td>
      </tr>`;
    }).join('');
  } catch (err) {
    tbody.innerHTML = `<tr><td colspan="4" class="text-danger" style="text-align:center;">Failed to load: ${escapeHtml(err.message)}</td></tr>`;
    showToast('Failed to load team members: ' + err.message, 'error');
  }
}

async function inviteMember() {
  const email = document.getElementById('invite-email').value.trim();
  const role = document.getElementById('invite-role').value;
  if (!email) { showToast('Please enter an email address', 'error'); return; }
  const btn = document.getElementById('btn-invite-submit');
  btn.disabled = true;
  btn.textContent = 'Sending...';
  try {
    const data = await api(`/admin/projects/${detailProjectId}/invite`, {
      method: 'POST',
      body: JSON.stringify({ email, role })
    });
    // Show invite link
    const linkContainer = document.getElementById('invite-result-link');
    const linkValue = document.getElementById('invite-link-value');
    linkContainer.style.display = 'block';
    linkValue.textContent = data.invite_link || 'Link generated';
    showToast('Invitation sent to ' + email, 'success');
    await loadTeam();
  } catch (err) {
    showToast(err.message, 'error');
  } finally {
    btn.disabled = false;
    btn.textContent = 'Send Invitation';
  }
}

async function removeMember(userId) {
  if (!await showConfirm('Remove Member', 'Are you sure you want to remove this team member?', 'Remove', true)) return;
  try {
    await api(`/admin/projects/${detailProjectId}/members/${userId}`, {
      method: 'DELETE'
    });
    showToast('Member removed');
    await loadTeam();
  } catch (err) {
    showToast(err.message, 'error');
  }
}

async function changeMemberRole(userId, newRole) {
  try {
    await api(`/admin/projects/${detailProjectId}/members/${userId}/role`, {
      method: 'PUT',
      body: JSON.stringify({ role: newRole })
    });
    showToast('Role updated to ' + newRole);
  } catch (err) {
    showToast(err.message, 'error');
    // Reload to reset the select
    await loadTeam();
  }
}

function openInviteMemberModal() {
  document.getElementById('invite-email').value = '';
  document.getElementById('invite-role').value = 'developer';
  document.getElementById('invite-result-link').style.display = 'none';
  document.getElementById('modal-invite-member').style.display = 'flex';
}

function closeInviteMemberModal() {
  const overlay = document.getElementById('modal-invite-member');
  const inner = document.getElementById('modal-invite-member-inner');
  inner.classList.add('closing');
  setTimeout(() => {
    overlay.style.display = 'none';
    inner.classList.remove('closing');
  }, 180);
}

function copyInviteLink() {
  const el = document.getElementById('invite-link-value');
  if (!el) return;
  navigator.clipboard.writeText(el.textContent).then(() => {
    showToast('Invite link copied');
  }).catch(() => {
    showToast('Failed to copy', 'error');
  });
}

async function loadProjectLogs() {
  const tbody = document.getElementById('logs-tbody');
  if (!tbody) return;

  // 1. Fetch usage metrics first (to always display even if requests.log is empty)
  let usage = { db_reads_total: 0, db_writes_total: 0, database_bytes: 0, file_storage_bytes: 0, total_storage_bytes: 0, realtime_connections: 0 };
  try {
    usage = await api(`/admin/projects/${detailProjectId}/usage`);
  } catch (e) {
    console.warn("Failed to load project usage", e);
  }

  // Populate usage elements
  const rtConnEl = document.getElementById('metrics-realtime-connections');
  if (rtConnEl) rtConnEl.textContent = usage.realtime_connections !== undefined ? usage.realtime_connections : '0';
  
  const dbReadsEl = document.getElementById('metrics-db-reads');
  if (dbReadsEl) dbReadsEl.textContent = usage.db_reads_total !== undefined ? usage.db_reads_total.toLocaleString() : '0';
  
  const dbWritesEl = document.getElementById('metrics-db-writes');
  if (dbWritesEl) dbWritesEl.textContent = usage.db_writes_total !== undefined ? usage.db_writes_total.toLocaleString() : '0';
  
  const dbSizeEl = document.getElementById('metrics-database-size');
  if (dbSizeEl) dbSizeEl.textContent = formatBytes(usage.database_bytes || 0);
  
  const fileStorageEl = document.getElementById('metrics-file-storage-size');
  if (fileStorageEl) fileStorageEl.textContent = formatBytes(usage.file_storage_bytes || 0);
  
  const totalStorageEl = document.getElementById('metrics-total-storage');
  if (totalStorageEl) totalStorageEl.textContent = formatBytes(usage.total_storage_bytes || 0);

  // 2. Fetch and render logs
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
    document.getElementById('cfg-backup-interval').value = cfg.backup_interval || '1h';

    // S3 fields
    const s3Enabled = cfg.s3_enabled === true;
    document.getElementById('cfg-s3-enabled').checked = s3Enabled;
    document.getElementById('s3-form-fields').style.display = s3Enabled ? 'grid' : 'none';
    document.getElementById('cfg-s3-endpoint').value = cfg.s3_endpoint || '';
    document.getElementById('cfg-s3-prefix').value = cfg.s3_bucket_prefix || '';
    document.getElementById('cfg-s3-access-key').value = cfg.s3_access_key || '';
    document.getElementById('cfg-s3-secret-key').value = cfg.s3_secret_key || '';
    document.getElementById('cfg-s3-ssl').checked = cfg.s3_use_ssl !== false;

    // SMTP / Email Verification fields
    const emailVerify = cfg.email_verification === true;
    document.getElementById('cfg-email-verification').checked = emailVerify;
    document.getElementById('smtp-form-fields').style.display = emailVerify ? 'grid' : 'none';
    document.getElementById('cfg-smtp-host').value = cfg.smtp_host || '';
    document.getElementById('cfg-smtp-port').value = cfg.smtp_port || 587;
    document.getElementById('cfg-smtp-sender').value = cfg.smtp_sender || '';
    document.getElementById('cfg-smtp-user').value = cfg.smtp_user || '';
    document.getElementById('cfg-smtp-password').value = '';

    // Replication fields
    const roleEl = document.getElementById('cfg-role');
    roleEl.value = cfg.role || '';
    document.getElementById('cfg-node-id').value = cfg.node_id || '';
    document.getElementById('cfg-repl-addr').value = cfg.repl_addr || '';
    document.getElementById('cfg-peers').value = (cfg.peers || []).join('\n');

    // Security & HTTPS fields
    const envEl = document.getElementById('cfg-env');
    if (envEl) envEl.value = cfg.env || 'development';
    document.getElementById('cfg-jwt-secret').value = cfg.jwt_secret || '';
    document.getElementById('cfg-cert-file').value = cfg.cert_file || '';
    document.getElementById('cfg-key-file').value = cfg.key_file || '';

    // Load backups
    loadBackups();

  } catch (e) {
    showToast('Failed to load config: ' + e.message, 'error');
  }
}

function toggleS3Form() {
  const enabled = document.getElementById('cfg-s3-enabled').checked;
  document.getElementById('s3-form-fields').style.display = enabled ? 'grid' : 'none';
}

function toggleSMTPForm() {
  const enabled = document.getElementById('cfg-email-verification').checked;
  document.getElementById('smtp-form-fields').style.display = enabled ? 'grid' : 'none';
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
    const smtpPwd = document.getElementById('cfg-smtp-password').value;
    await api('/admin/config', {
      method: 'POST',
      body: JSON.stringify({
        admin_email:        document.getElementById('cfg-admin-email').value,
        admin_password:     newPwd || '••••••••',
        env:               document.getElementById('cfg-env').value,
        jwt_secret:        document.getElementById('cfg-jwt-secret').value || '••••••••',
        cert_file:         document.getElementById('cfg-cert-file').value,
        key_file:          document.getElementById('cfg-key-file').value,
        session_duration:   document.getElementById('cfg-session-duration').value || '24h',
        backup_interval:    document.getElementById('cfg-backup-interval').value || '1h',
        s3_enabled:         document.getElementById('cfg-s3-enabled').checked,
        s3_endpoint:        document.getElementById('cfg-s3-endpoint').value,
        s3_access_key:      document.getElementById('cfg-s3-access-key').value,
        s3_secret_key:      secretKey || '••••••••',
        s3_bucket_prefix:   document.getElementById('cfg-s3-prefix').value,
        s3_use_ssl:         document.getElementById('cfg-s3-ssl').checked,
        email_verification: document.getElementById('cfg-email-verification').checked,
        smtp_host:          document.getElementById('cfg-smtp-host').value,
        smtp_port:          parseInt(document.getElementById('cfg-smtp-port').value || '587', 10),
        smtp_sender:        document.getElementById('cfg-smtp-sender').value,
        smtp_user:          document.getElementById('cfg-smtp-user').value,
        smtp_password:      smtpPwd || '••••••••',
        role:               document.getElementById('cfg-role').value,
        node_id:            document.getElementById('cfg-node-id').value,
        repl_addr:          document.getElementById('cfg-repl-addr').value,
        peers:              peers,
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
    if (e.target.id === 'modal-invite-member') closeInviteMemberModal();
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
    if (document.getElementById('modal-invite-member').style.display === 'flex') closeInviteMemberModal();
  }
  // Enter in create modal
  if (e.key === 'Enter' && document.getElementById('modal-create').style.display === 'flex') {
    const nameEl = document.getElementById('new-project-name');
    if (document.activeElement === nameEl) {
      createProject();
    }
  }
});

// ===== SETTINGS TABS =====
let auditPage = 0;
const auditPageSize = 25;

function switchSettingsTab(name) {
  document.querySelectorAll('#settings-tabs .qs-tab').forEach(el => el.classList.remove('active'));
  const tab = document.querySelector(`#settings-tabs .qs-tab[data-stab="${name}"]`);
  if (tab) tab.classList.add('active');
  document.querySelectorAll('.settings-tab-panel').forEach(el => el.classList.remove('active'));
  const panel = document.getElementById('stab-' + name);
  if (panel) panel.classList.add('active');
  if (name === 'admins') loadAdmins();
  if (name === 'audit') loadAuditLogs();
  if (name === 'backups') loadBackups();
}

// ===== ADMIN MANAGEMENT =====
async function loadAdmins() {
  const tbody = document.getElementById('admins-tbody');
  if (!tbody) return;
  try {
    const data = await api('/admin/admins');
    const admins = data.admins || [];
    if (admins.length === 0) {
      tbody.innerHTML = '<tr><td colspan="5" class="text-muted" style="text-align:center;">No admin users found</td></tr>';
      return;
    }
    tbody.innerHTML = admins.map(a => {
      const lastLogin = a.last_login ? new Date(a.last_login).toLocaleString() : '—';
      const roleBadge = `<span class="badge badge-${a.role === 'super_admin' ? 'active' : a.role === 'admin' ? 'badge-suspended' : ''}" style="text-transform:capitalize">${escapeHtml(a.role)}</span>`;
      return `<tr>
        <td>${escapeHtml(a.email)}</td>
        <td>${roleBadge}</td>
        <td>${escapeHtml(a.name || '—')}</td>
        <td class="td-date">${lastLogin}</td>
        <td class="td-actions">
          <button class="btn btn-xs btn-danger" onclick="deleteAdmin('${escapeHtml(a.id)}')">Remove</button>
        </td>
      </tr>`;
    }).join('');
  } catch (e) {
    tbody.innerHTML = '<tr><td colspan="5" class="text-danger" style="text-align:center;">Failed to load: ' + escapeHtml(e.message) + '</td></tr>';
  }
}

function openCreateAdminModal() {
  document.getElementById('modal-create-admin').style.display = 'flex';
  document.getElementById('new-admin-email').value = '';
  document.getElementById('new-admin-password').value = '';
  document.getElementById('new-admin-name').value = '';
  document.getElementById('new-admin-role').value = 'admin';
  setTimeout(() => document.getElementById('new-admin-email').focus(), 100);
}

function closeCreateAdminModal() {
  const overlay = document.getElementById('modal-create-admin');
  const inner = document.getElementById('modal-create-admin-inner');
  inner.classList.add('closing');
  setTimeout(() => {
    overlay.style.display = 'none';
    inner.classList.remove('closing');
  }, 180);
}

async function createAdmin() {
  const email = document.getElementById('new-admin-email').value.trim();
  const password = document.getElementById('new-admin-password').value;
  const name = document.getElementById('new-admin-name').value.trim();
  const role = document.getElementById('new-admin-role').value;

  if (!email || !password) {
    showToast('Email and password are required', 'error');
    return;
  }
  if (password.length < 8) {
    showToast('Password must be at least 8 characters', 'error');
    return;
  }

  const btn = document.getElementById('btn-create-admin');
  btn.disabled = true;
  btn.textContent = 'Creating...';

  try {
    await api('/admin/admins', {
      method: 'POST',
      body: JSON.stringify({ email, password, name, role })
    });
    showToast('Admin user created successfully', 'success');
    closeCreateAdminModal();
    loadAdmins();
  } catch (e) {
    showToast('Failed to create admin: ' + e.message, 'error');
  } finally {
    btn.disabled = false;
    btn.textContent = 'Create Admin';
  }
}

async function deleteAdmin(id) {
  if (!await showConfirm('Remove Admin', 'Remove this admin user? They will lose access immediately.', 'Remove', true)) return;
  try {
    await api('/admin/admins/' + encodeURIComponent(id), { method: 'DELETE' });
    showToast('Admin removed', 'success');
    loadAdmins();
  } catch (e) {
    showToast('Failed to remove admin: ' + e.message, 'error');
  }
}

// ===== AUDIT LOGS =====
async function loadAuditLogs() {
  const tbody = document.getElementById('audit-tbody');
  if (!tbody) return;

  const actionFilter = document.getElementById('audit-filter-action').value.trim();
  const targetFilter = document.getElementById('audit-filter-target').value.trim();

  let url = '/admin/audit-logs?limit=' + auditPageSize + '&offset=' + (auditPage * auditPageSize);
  if (actionFilter) url += '&action=' + encodeURIComponent(actionFilter);
  if (targetFilter) url += '&target_type=' + encodeURIComponent(targetFilter);

  try {
    const data = await api(url);
    const entries = data.entries || [];
    const total = data.total || 0;

    const startEl = document.getElementById('audit-start');
    const endEl = document.getElementById('audit-end');
    const totalEl = document.getElementById('audit-total');
    const prevBtn = document.getElementById('audit-prev-btn');
    const nextBtn = document.getElementById('audit-next-btn');

    if (startEl) startEl.textContent = total === 0 ? '0' : (auditPage * auditPageSize + 1);
    if (endEl) endEl.textContent = Math.min((auditPage + 1) * auditPageSize, total);
    if (totalEl) totalEl.textContent = total;

    if (prevBtn) prevBtn.disabled = auditPage <= 0;
    if (nextBtn) nextBtn.disabled = (auditPage + 1) * auditPageSize >= total;

    if (entries.length === 0) {
      tbody.innerHTML = '<tr><td colspan="5" class="text-muted" style="text-align:center;">No audit log entries found</td></tr>';
      return;
    }

    tbody.innerHTML = entries.map(e => {
      const ts = e.timestamp ? new Date(e.timestamp).toLocaleString() : '—';
      const details = e.details ? JSON.stringify(e.details).substring(0, 80) : '—';
      return `<tr>
        <td class="td-date" style="font-size:11px;">${escapeHtml(ts)}</td>
        <td style="font-size:12px;">${escapeHtml(e.admin_email || '—')}</td>
        <td style="font-size:12px;font-family:var(--font-mono);">${escapeHtml(e.action || '—')}</td>
        <td style="font-size:12px;">${escapeHtml(e.target_type || '')}${e.target_id ? ': ' + escapeHtml(e.target_id).substring(0, 12) + '...' : ''}</td>
        <td style="font-size:11px;color:var(--text-muted);max-width:200px;overflow:hidden;text-overflow:ellipsis;white-space:nowrap;" title="${escapeHtml(details)}">${escapeHtml(details)}</td>
      </tr>`;
    }).join('');
  } catch (e) {
    tbody.innerHTML = '<tr><td colspan="5" class="text-danger" style="text-align:center;">Failed to load: ' + escapeHtml(e.message) + '</td></tr>';
  }
}

function auditPagePrev() {
  if (auditPage > 0) {
    auditPage--;
    loadAuditLogs();
  }
}

function auditPageNext() {
  auditPage++;
  loadAuditLogs();
}

// ===== BACKUPS =====
async function loadBackups() {
  try {
    const data = await api('/admin/backups');
    const list = document.getElementById('backups-list');
    if (!list) return;
    
    const backups = data.backups || [];
    if (backups.length === 0) {
      list.innerHTML = '<div style="text-align:center;padding:12px;color:var(--text-muted);font-size:12px;">No backups yet. Create one above.</div>';
      return;
    }
    
    list.innerHTML = backups.map(b => {
      const date = b.modified ? new Date(b.modified).toLocaleString() : '—';
      return `<div style="display:flex;align-items:center;justify-content:space-between;padding:10px 14px;background:var(--bg-card-elevated);border:1px solid var(--border);border-radius:var(--radius);">
        <div style="display:flex;align-items:center;gap:10px;">
          <span style="font-size:18px;">📦</span>
          <div>
            <div style="font-size:13px;font-weight:600;">${escapeHtml(b.name)}</div>
            <div style="font-size:11px;color:var(--text-muted);">${date}</div>
          </div>
        </div>
        <div style="display:flex;gap:6px;">
          <button class="btn btn-xs btn-secondary" onclick="downloadBackup('${escapeHtml(b.name)}')">⬇ Download</button>
          <button class="btn btn-xs btn-danger" onclick="deleteBackup('${escapeHtml(b.name)}')">Delete</button>
        </div>
      </div>`;
    }).join('');
  } catch (e) {
    const list = document.getElementById('backups-list');
    if (list) list.innerHTML = '<div style="text-align:center;padding:12px;color:var(--danger);font-size:12px;">Failed to load backups</div>';
  }
}

async function createBackup() {
  const btn = document.getElementById('btn-create-backup');
  btn.disabled = true;
  btn.textContent = 'Creating...';
  try {
    await api('/admin/backups', { method: 'POST' });
    showToast('Backup created successfully', 'success');
    await loadBackups();
  } catch (e) {
    showToast('Backup failed: ' + e.message, 'error');
  } finally {
    btn.disabled = false;
    btn.textContent = '+ Create Backup';
  }
}

async function downloadBackup(name) {
  try {
    const token = localStorage.getItem('sovrabase_admin_token');
    const res = await fetch('/admin/backups/' + encodeURIComponent(name) + '/download', {
      headers: { 'Authorization': 'Bearer ' + token }
    });
    if (!res.ok) {
      const err = await res.json().catch(() => ({}));
      throw new Error(err.error || 'Download failed');
    }
    const blob = await res.blob();
    const url = window.URL.createObjectURL(blob);
    const a = document.createElement('a');
    a.href = url;
    a.download = name + '.zip';
    a.click();
    window.URL.revokeObjectURL(url);
  } catch (e) {
    showToast('Download failed: ' + e.message, 'error');
  }
}

async function deleteBackup(name) {
  if (!await showConfirm('Delete Backup', 'Delete backup "' + name + '"? This cannot be undone.', 'Delete', true)) return;
  try {
    await api('/admin/backups/' + encodeURIComponent(name), { method: 'DELETE' });
    showToast('Backup deleted', 'success');
    await loadBackups();
  } catch (e) {
    showToast('Delete failed: ' + e.message, 'error');
  }
}

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

async function clearAuditLogs() {
  if (!confirm("Are you sure you want to clear all audit logs? This action cannot be undone.")) {
    return;
  }
  try {
    await api('/admin/audit-logs', { method: 'DELETE' });
    showToast('Audit logs cleared successfully');
    auditPage = 0;
    loadAuditLogs();
  } catch (e) {
    showToast('Failed to clear audit logs: ' + e.message, 'error');
  }
}

async function runDeepDbAnalysis() {
  const resultDiv = document.getElementById('db-analysis-result');
  if (!resultDiv) return;
  resultDiv.style.display = 'block';
  resultDiv.innerHTML = '<div style="text-align:center;padding:12px;color:var(--text-secondary);">Analyzing storage...</div>';

  try {
    const analysis = await api(`/admin/projects/${detailProjectId}/db-analysis`);
    
    let collectionsHtml = '';
    const colls = analysis.collections || {};
    const collNames = Object.keys(colls);
    
    if (collNames.length === 0) {
      collectionsHtml = '<div style="font-size:12px;color:var(--text-muted);text-align:center;padding:8px 0;">No collections found</div>';
    } else {
      collectionsHtml = `
        <table style="width:100%;font-size:12px;margin-top:8px;border-collapse:collapse;">
          <thead>
            <tr style="border-bottom:1px solid var(--border);color:var(--text-muted);">
              <th style="text-align:left;padding:6px 0;">Collection</th>
              <th style="text-align:right;padding:6px 0;">Docs Count</th>
              <th style="text-align:right;padding:6px 0;">Docs Size</th>
              <th style="text-align:right;padding:6px 0;">Index Size</th>
              <th style="text-align:right;padding:6px 0;">Total Size</th>
            </tr>
          </thead>
          <tbody>
            ${collNames.map(name => {
              const c = colls[name];
              const docSize = c.document_size || 0;
              const idxSize = c.index_size || 0;
              const total = docSize + idxSize;
              return `
                <tr style="border-bottom:1px solid rgba(255,255,255,0.05);">
                  <td style="padding:6px 0;font-family:var(--font-mono);">${escapeHtml(name)}</td>
                  <td style="text-align:right;padding:6px 0;">${c.document_count || 0}</td>
                  <td style="text-align:right;padding:6px 0;">${formatBytes(docSize)}</td>
                  <td style="text-align:right;padding:6px 0;">${formatBytes(idxSize)}</td>
                  <td style="text-align:right;padding:6px 0;font-weight:bold;">${formatBytes(total)}</td>
                </tr>
              `;
            }).join('')}
          </tbody>
        </table>
      `;
    }

    resultDiv.innerHTML = `
      <div style="display:flex;justify-content:space-between;align-items:center;margin-bottom:12px;border-bottom:1px solid var(--border);padding-bottom:8px;">
        <span style="font-weight:bold;font-size:13px;text-transform:uppercase;color:var(--accent);">Storage Breakdown</span>
        <button class="btn btn-xs btn-secondary" onclick="runDeepDbAnalysis()">Re-run</button>
      </div>
      <div style="display:grid;grid-template-columns:repeat(auto-fit, minmax(110px, 1fr));gap:12px;margin-bottom:16px;">
        <div style="padding:10px;background:rgba(255,255,255,0.03);border-radius:4px;border:1px solid rgba(255,255,255,0.05);">
          <div style="font-size:10px;color:var(--text-muted);text-transform:uppercase;">Total Size</div>
          <div style="font-size:16px;font-weight:bold;margin-top:4px;color:var(--text-primary);">${formatBytes(analysis.total_size || 0)}</div>
        </div>
        <div style="padding:10px;background:rgba(255,255,255,0.03);border-radius:4px;border:1px solid rgba(255,255,255,0.05);">
          <div style="font-size:10px;color:var(--text-muted);text-transform:uppercase;">Metadata Size</div>
          <div style="font-size:16px;font-weight:bold;margin-top:4px;color:var(--text-primary);">${formatBytes(analysis.metadata_size || 0)}</div>
        </div>
        <div style="padding:10px;background:rgba(255,255,255,0.03);border-radius:4px;border:1px solid rgba(255,255,255,0.05);">
          <div style="font-size:10px;color:var(--text-muted);text-transform:uppercase;">Index Size</div>
          <div style="font-size:16px;font-weight:bold;margin-top:4px;color:var(--text-primary);">${formatBytes(analysis.index_size || 0)}</div>
        </div>
      </div>
      <div>
        <span style="font-weight:bold;font-size:12px;color:var(--text-secondary);">Collections Storage</span>
        ${collectionsHtml}
      </div>
    `;
  } catch (e) {
    resultDiv.innerHTML = `<div style="color:var(--danger);text-align:center;padding:12px;">Analysis failed: ${escapeHtml(e.message)}</div>`;
  }
}

// ===== REMOTE CONFIG =====

async function loadRemoteConfig() {
  const tbody = document.getElementById('config-entries-tbody');
  if (!tbody) return;
  try {
    const data = await api(`/admin/projects/${detailProjectId}/config`);
    const entries = data.data || [];
    if (entries.length === 0) {
      tbody.innerHTML = '<tr><td colspan="5" class="text-muted" style="text-align:center;">No config entries yet. Click "Add Config" to create one.</td></tr>';
      return;
    }
    tbody.innerHTML = entries.map(e => {
      const valStr = typeof e.value === 'object' ? JSON.stringify(e.value) : String(e.value);
      const visBadge = e.public
        ? '<span style="color:var(--success);font-size:11px;">🌐 Public</span>'
        : '<span style="color:var(--text-muted);font-size:11px;">🔒 Private</span>';
      return `
        <tr>
          <td style="font-family:var(--font-mono,monospace);font-size:12px;font-weight:600;">${escapeHtml(e.key)}</td>
          <td style="font-size:12px;max-width:200px;overflow:hidden;text-overflow:ellipsis;" title="${escapeHtml(valStr)}">${escapeHtml(valStr)}</td>
          <td><span style="font-size:11px;background:rgba(255,255,255,0.06);padding:2px 8px;border-radius:4px;">${escapeHtml(e.type || 'string')}</span></td>
          <td>${visBadge}</td>
          <td>
            <button class="btn btn-sm btn-danger" onclick="deleteConfigEntry('${escapeHtml(e.key)}')" title="Delete">🗑</button>
          </td>
        </tr>`;
    }).join('');
  } catch (err) {
    tbody.innerHTML = `<tr><td colspan="5" class="text-muted" style="text-align:center;">Failed to load: ${escapeHtml(err.message)}</td></tr>`;
  }
}

function showAddConfigModal() {
  document.getElementById('modal-add-config').style.display = 'flex';
  // Reset fields
  document.getElementById('cfg-config-key').value = '';
  document.getElementById('cfg-config-value').value = '';
  document.getElementById('cfg-config-type').value = '';
  document.getElementById('cfg-config-public').value = 'false';
  document.getElementById('cfg-config-desc').value = '';
  document.getElementById('config-modal-title').textContent = 'Add Config Entry';
  setTimeout(() => document.getElementById('cfg-config-key').focus(), 100);
}

function closeAddConfigModal() {
  const overlay = document.getElementById('modal-add-config');
  const inner = overlay.querySelector('.modal');
  if (inner) {
    inner.classList.add('closing');
    setTimeout(() => {
      overlay.style.display = 'none';
      inner.classList.remove('closing');
    }, 180);
  } else {
    overlay.style.display = 'none';
  }
}

async function saveConfigEntry() {
  const key = document.getElementById('cfg-config-key').value.trim();
  const valueStr = document.getElementById('cfg-config-value').value.trim();
  const type = document.getElementById('cfg-config-type').value;
  const isPublic = document.getElementById('cfg-config-public').value === 'true';
  const desc = document.getElementById('cfg-config-desc').value.trim();

  if (!key) { showToast('Key is required', 'error'); return; }
  if (!valueStr) { showToast('Value is required', 'error'); return; }

  // Parse value based on type.
  let value;
  const effectiveType = type || 'auto';
  try {
    if (effectiveType === 'boolean' || (effectiveType === 'auto' && (valueStr === 'true' || valueStr === 'false'))) {
      value = valueStr === 'true';
    } else if (effectiveType === 'number' || (effectiveType === 'auto' && /^-?\d+(\.\d+)?$/.test(valueStr))) {
      value = parseFloat(valueStr);
    } else if (effectiveType === 'json') {
      value = JSON.parse(valueStr);
    } else {
      value = valueStr;
    }
  } catch (e) {
    showToast('Invalid JSON value: ' + e.message, 'error');
    return;
  }

  const btn = document.getElementById('btn-save-config');
  btn.disabled = true;
  btn.textContent = 'Saving...';

  try {
    await api(`/admin/projects/${detailProjectId}/config`, {
      method: 'POST',
      body: JSON.stringify({ key, value, type, description: desc, public: isPublic })
    });
    showToast('Config entry saved');
    closeAddConfigModal();
    await loadRemoteConfig();
  } catch (err) {
    showToast('Failed to save: ' + err.message, 'error');
  } finally {
    btn.disabled = false;
    btn.textContent = 'Save';
  }
}

async function deleteConfigEntry(key) {
  if (!confirmCustom) {
    // Fallback to native confirm if custom confirm not available.
    if (!confirm(`Delete config entry "${key}"?`)) return;
    try {
      await api(`/admin/projects/${detailProjectId}/config/${encodeURIComponent(key)}`, { method: 'DELETE' });
      showToast('Config entry deleted');
      await loadRemoteConfig();
    } catch (err) {
      showToast('Failed to delete: ' + err.message, 'error');
    }
    return;
  }
  confirmCustom(`Delete config entry "${key}"?`, async () => {
    try {
      await api(`/admin/projects/${detailProjectId}/config/${encodeURIComponent(key)}`, { method: 'DELETE' });
      showToast('Config entry deleted');
      await loadRemoteConfig();
    } catch (err) {
      showToast('Failed to delete: ' + err.message, 'error');
    }
  });
}

// ===== CRON JOBS =====

async function loadCronJobs() {
  const tbody = document.getElementById('cron-jobs-tbody');
  if (!tbody) return;
  try {
    const data = await api(`/admin/projects/${detailProjectId}/cron`);
    const jobs = data.data || [];
    if (jobs.length === 0) {
      tbody.innerHTML = '<tr><td colspan="5" class="text-muted" style="text-align:center;">No scheduled jobs yet. Click "Add Job" to create one.</td></tr>';
      return;
    }
    tbody.innerHTML = jobs.map(j => {
      const statusBadge = j.enabled
        ? '<span style="color:var(--success);font-size:11px;">● Active</span>'
        : '<span style="color:var(--text-muted);font-size:11px;">○ Paused</span>';
      const shortUrl = (j.url || '').length > 40 ? j.url.substring(0, 37) + '...' : (j.url || '');
      return `
        <tr>
          <td style="font-size:12px;font-weight:600;">${escapeHtml(j.name)}</td>
          <td style="font-family:var(--font-mono,monospace);font-size:11px;background:rgba(255,255,255,0.06);padding:2px 6px;border-radius:3px;">${escapeHtml(j.schedule)}</td>
          <td style="font-size:12px;max-width:250px;overflow:hidden;text-overflow:ellipsis;" title="${escapeHtml(j.url)}">${escapeHtml(shortUrl)}</td>
          <td>${statusBadge}</td>
          <td>
            <button class="btn btn-sm btn-danger" onclick="deleteCronJob('${escapeHtml(j.id)}','${escapeHtml(j.name)}')" title="Delete">🗑</button>
          </td>
        </tr>`;
    }).join('');
  } catch (err) {
    tbody.innerHTML = `<tr><td colspan="5" class="text-muted" style="text-align:center;">Failed to load: ${escapeHtml(err.message)}</td></tr>`;
  }
}

function showAddCronModal() {
  document.getElementById('modal-add-cron').style.display = 'flex';
  document.getElementById('cron-name').value = '';
  document.getElementById('cron-schedule').value = '*/5 * * * *';
  document.getElementById('cron-method').value = 'POST';
  document.getElementById('cron-enabled').value = 'true';
  document.getElementById('cron-url').value = '';
  document.getElementById('cron-body').value = '';
  setTimeout(() => document.getElementById('cron-name').focus(), 100);
}

function closeAddCronModal() {
  const overlay = document.getElementById('modal-add-cron');
  const inner = overlay.querySelector('.modal');
  if (inner) {
    inner.classList.add('closing');
    setTimeout(() => { overlay.style.display = 'none'; inner.classList.remove('closing'); }, 180);
  } else {
    overlay.style.display = 'none';
  }
}

async function saveCronJob() {
  const name = document.getElementById('cron-name').value.trim();
  const schedule = document.getElementById('cron-schedule').value.trim();
  const method = document.getElementById('cron-method').value;
  const enabled = document.getElementById('cron-enabled').value === 'true';
  const url = document.getElementById('cron-url').value.trim();
  const body = document.getElementById('cron-body').value.trim();

  if (!name) { showToast('Job name is required', 'error'); return; }
  if (!schedule) { showToast('Schedule is required', 'error'); return; }
  if (!url) { showToast('Callback URL is required', 'error'); return; }

  const payload = { name, schedule, method, enabled, url };
  if (body) {
    try { JSON.parse(body); payload.body = body; }
    catch (e) { showToast('Invalid JSON in body', 'error'); return; }
  }

  const btn = document.getElementById('btn-save-cron');
  btn.disabled = true;
  btn.textContent = 'Creating...';

  try {
    await api(`/admin/projects/${detailProjectId}/cron`, {
      method: 'POST',
      body: JSON.stringify(payload)
    });
    showToast('Cron job created');
    closeAddCronModal();
    await loadCronJobs();
  } catch (err) {
    showToast('Failed to create: ' + err.message, 'error');
  } finally {
    btn.disabled = false;
    btn.textContent = 'Create Job';
  }
}

async function deleteCronJob(jobId, jobName) {
  if (!confirm(`Delete cron job "${jobName}"?`)) return;
  try {
    await api(`/admin/projects/${detailProjectId}/cron/${jobId}`, { method: 'DELETE' });
    showToast('Cron job deleted');
    await loadCronJobs();
  } catch (err) {
    showToast('Failed to delete: ' + err.message, 'error');
  }
}

// ===== WEBHOOKS =====

async function loadWebhooks() {
  const tbody = document.getElementById('webhooks-tbody');
  if (!tbody) return;
  try {
    const data = await api(`/admin/projects/${detailProjectId}/webhooks`);
    const hooks = data.data || [];
    if (hooks.length === 0) {
      tbody.innerHTML = '<tr><td colspan="4" class="text-muted" style="text-align:center;">No webhooks configured. Add one to receive HTTP callbacks on data events.</td></tr>';
      return;
    }
    tbody.innerHTML = hooks.map(h => {
      const events = h.events || 'all';
      const statusBadge = h.enabled
        ? '<span style="color:var(--success);font-size:11px;">● Active</span>'
        : '<span style="color:var(--text-muted);font-size:11px;">○ Paused</span>';
      const shortUrl = (h.url || '').length > 50 ? h.url.substring(0, 47) + '...' : (h.url || '');
      return `
        <tr>
          <td style="font-size:12px;max-width:300px;overflow:hidden;text-overflow:ellipsis;" title="${escapeHtml(h.url)}">${escapeHtml(shortUrl)}</td>
          <td style="font-size:11px;">${escapeHtml(events)}</td>
          <td>${statusBadge}</td>
          <td>
            <button class="btn btn-sm btn-danger" onclick="deleteWebhook('${escapeHtml(h.id)}')" title="Delete">🗑</button>
          </td>
        </tr>`;
    }).join('');
  } catch (err) {
    tbody.innerHTML = `<tr><td colspan="4" class="text-muted" style="text-align:center;">Failed to load: ${escapeHtml(err.message)}</td></tr>`;
  }
}

function showAddWebhookModal() {
  document.getElementById('modal-add-webhook').style.display = 'flex';
  document.getElementById('wh-url').value = '';
  document.getElementById('wh-events').value = '';
  document.getElementById('wh-enabled').value = 'true';
  setTimeout(() => document.getElementById('wh-url').focus(), 100);
}

function closeAddWebhookModal() {
  const overlay = document.getElementById('modal-add-webhook');
  const inner = overlay.querySelector('.modal');
  if (inner) {
    inner.classList.add('closing');
    setTimeout(() => { overlay.style.display = 'none'; inner.classList.remove('closing'); }, 180);
  } else {
    overlay.style.display = 'none';
  }
}

async function saveWebhook() {
  const url = document.getElementById('wh-url').value.trim();
  const events = document.getElementById('wh-events').value.trim();
  const enabled = document.getElementById('wh-enabled').value === 'true';

  if (!url) { showToast('URL is required', 'error'); return; }

  const btn = document.getElementById('btn-save-webhook');
  btn.disabled = true;
  btn.textContent = 'Creating...';

  try {
    await api(`/admin/projects/${detailProjectId}/webhooks`, {
      method: 'POST',
      body: JSON.stringify({ url, events, enabled })
    });
    showToast('Webhook created');
    closeAddWebhookModal();
    await loadWebhooks();
  } catch (err) {
    showToast('Failed to create: ' + err.message, 'error');
  } finally {
    btn.disabled = false;
    btn.textContent = 'Create Webhook';
  }
}

async function deleteWebhook(webhookId) {
  if (!confirm('Delete this webhook?')) return;
  try {
    await api(`/admin/projects/${detailProjectId}/webhooks/${webhookId}`, { method: 'DELETE' });
    showToast('Webhook deleted');
    await loadWebhooks();
  } catch (err) {
    showToast('Failed to delete: ' + err.message, 'error');
  }
}

// ===== ANALYTICS =====

async function loadAnalytics() {
  try {
    const data = await api(`/admin/projects/${detailProjectId}/analytics`);

    // Summary cards.
    document.getElementById('analytics-total').textContent = (data.total_events || 0).toLocaleString();

    const topEvents = data.top_events || [];
    document.getElementById('analytics-top').textContent = topEvents.length > 0 ? topEvents[0].name : '—';

    const hourlyCounts = data.hourly_counts || [];
    document.getElementById('analytics-hours').textContent = hourlyCounts.length;

    // Top events table.
    const topTbody = document.getElementById('analytics-top-tbody');
    if (topEvents.length === 0) {
      topTbody.innerHTML = '<tr><td colspan="2" class="text-muted" style="text-align:center;">No events yet. Start sending events via POST /api/v1/events</td></tr>';
    } else {
      topTbody.innerHTML = topEvents.map(e => `
        <tr>
          <td style="font-size:12px;font-weight:600;">${escapeHtml(e.name)}</td>
          <td>${e.count.toLocaleString()}</td>
        </tr>`).join('');
    }

    // Hourly bar chart.
    const chart = document.getElementById('analytics-chart');
    if (!chart) return;

    if (hourlyCounts.length === 0) {
      chart.innerHTML = '<span style="color:var(--text-muted);font-size:12px;align-self:center;width:100%;text-align:center;">No data yet</span>';
      return;
    }

    const maxCount = Math.max(...hourlyCounts.map(h => h.count), 1);
    chart.innerHTML = hourlyCounts.map(h => {
      const pct = Math.round((h.count / maxCount) * 100);
      const hour = new Date(h.hour).getHours();
      const hourLabel = String(hour).padStart(2, '0') + ':00';
      return `
        <div style="flex:1;display:flex;flex-direction:column;align-items:center;gap:4px;min-width:20px;" title="${hourLabel}: ${h.count} events">
          <span style="font-size:9px;color:var(--text-muted);">${h.count}</span>
          <div style="width:100%;height:${Math.max(pct, 2)}%;background:linear-gradient(180deg, #5b5bff 0%, #8b5cf6 100%);border-radius:3px 3px 0 0;min-height:2px;"></div>
          <span style="font-size:9px;color:var(--text-muted);">${hourLabel}</span>
        </div>`;
    }).join('');
  } catch (err) {
    document.getElementById('analytics-total').textContent = '—';
    document.getElementById('analytics-top').textContent = '—';
    document.getElementById('analytics-hours').textContent = '—';
    const topTbody = document.getElementById('analytics-top-tbody');
    if (topTbody) topTbody.innerHTML = `<tr><td colspan="2" class="text-muted" style="text-align:center;">Failed to load: ${escapeHtml(err.message)}</td></tr>`;
    const chart = document.getElementById('analytics-chart');
    if (chart) chart.innerHTML = '<span style="color:var(--danger);font-size:12px;align-self:center;width:100%;text-align:center;">Load failed</span>';
  }
}

// ===== QUEUES =====

let queuesData = [];

async function loadQueues() {
  const tbody = document.getElementById('queues-tbody');
  if (!tbody) return;
  try {
    const data = await api(`/admin/projects/${detailProjectId}/queues`);
    queuesData = data.data || [];
    if (queuesData.length === 0) {
      tbody.innerHTML = '<tr><td colspan="5" class="text-muted" style="text-align:center;">No queues yet. Send a message via POST /api/v1/queues/send to create one.</td></tr>';
      return;
    }
    tbody.innerHTML = queuesData.map(q => `
      <tr>
        <td style="font-size:13px;font-weight:600;font-family:var(--font-mono,monospace);">${escapeHtml(q.name)}</td>
        <td><span style="color:var(--success);">${q.visible || 0}</span></td>
        <td><span style="color:var(--warning);">${q.in_flight || 0}</span></td>
        <td>${(q.total || 0).toLocaleString()}</td>
        <td>
          <button class="btn btn-sm btn-warning" onclick="unstickQueue('${escapeHtml(q.name)}')" title="Make in-flight messages visible again">🔓</button>
          <button class="btn btn-sm btn-danger" onclick="purgeQueueConfirm('${escapeHtml(q.name)}')" title="Delete all messages">🗑</button>
        </td>
      </tr>`).join('');
  } catch (err) {
    tbody.innerHTML = `<tr><td colspan="5" class="text-muted" style="text-align:center;">Failed to load: ${escapeHtml(err.message)}</td></tr>`;
  }
}

async function unstickQueue(queueName) {
  if (!confirm(`Make ALL in-flight messages in "${queueName}" visible again?`)) return;
  try {
    await api(`/admin/projects/${detailProjectId}/queues/${encodeURIComponent(queueName)}/make-visible`, { method: 'POST', body: '{}' });
    showToast(`Queue "${queueName}" unstuck`);
    await loadQueues();
  } catch (err) {
    showToast('Failed: ' + err.message, 'error');
  }
}

async function makeAllVisible() {
  if (queuesData.length === 0) { showToast('No queues to unstick', 'error'); return; }
  const name = prompt('Queue name to unstick:');
  if (!name) return;
  await unstickQueue(name);
}

async function purgeQueueConfirm(queueName) {
  if (!confirm(`PERMANENTLY DELETE all messages from "${queueName}"?`)) return;
  try {
    await api(`/admin/projects/${detailProjectId}/queues/purge`, {
      method: 'POST',
      body: JSON.stringify({ queue: queueName })
    });
    showToast(`Queue "${queueName}" purged`);
    await loadQueues();
  } catch (err) {
    showToast('Failed: ' + err.message, 'error');
  }
}

async function purgeQueue() {
  if (queuesData.length === 0) { showToast('No queues to purge', 'error'); return; }
  const name = prompt('Queue name to purge (this DELETES all messages):');
  if (!name) return;
  await purgeQueueConfirm(name);
}
