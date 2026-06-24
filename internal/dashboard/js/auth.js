// === AUTH USER MANAGEMENT ===

async function loadUsers() {
  try {
    const users = await api(`/admin/projects/${detailProjectId}/users`);
    activeProjectEnv.users = users || [];
    renderUsersTable();
  } catch (err) {
    showToast('Failed to load users: ' + err.message, 'error');
  }
}

function renderUsersTable() {
  const tbody = document.getElementById('auth-users-tbody');
  if (!tbody) return;
  
  if (activeProjectEnv.users.length === 0) {
    tbody.innerHTML = '<tr><td colspan="6" class="text-muted" style="text-align:center;">No users registered yet</td></tr>';
    return;
  }
  tbody.innerHTML = activeProjectEnv.users.map(u => {
    const id = u.id || '';
    const email = u.email || '';
    const role = u.role || 'user';
    const name = u.name || '';
    const avatar = u.avatar_url || '';
    const providers = u._metadata || [];
    const created = formatDate(u.created_at);
    const shortId = id.substring(0, 8) + '...';
    
    const avatarHtml = avatar 
      ? `<img src="${escapeHtml(avatar)}" class="user-avatar" width="24" height="24" style="border-radius:50%;margin-right:8px;vertical-align:middle;" onerror="this.style.display='none'" alt="">`
      : '';
    const providerTags = providers.map(p => {
      const meta = getProviderMeta(p.provider);
      return `<span class="badge" style="margin-left:4px;background:${meta.bg};color:${meta.color};border:1px solid ${meta.color}30;">${escapeHtml(p.provider)}</span>`;
    }).join('');
    const displayName = name || email;
    
    return `<tr>
      <td class="td-id" title="${escapeHtml(id)}">${escapeHtml(shortId)}</td>
      <td class="td-name">${avatarHtml}${escapeHtml(displayName)}${providerTags}</td>
      <td><span class="badge ${role === 'admin' ? 'badge-suspended' : 'badge-active'}"><span class="badge-dot"></span>${escapeHtml(role)}</span></td>
      <td class="td-date">${created}</td>
      <td class="td-actions">
        <button class="btn btn-xs btn-danger" onclick="deleteUser('${escapeHtml(id)}')">Delete</button>
      </td>
    </tr>`;
  }).join('');
}

function openCreateUserModal() {
  document.getElementById('new-user-email').value = '';
  document.getElementById('new-user-password').value = '';
  document.getElementById('new-user-role').value = 'user';
  openSubModal('modal-create-user');
}

async function submitCreateUser() {
  const email = document.getElementById('new-user-email').value.trim();
  const password = document.getElementById('new-user-password').value;
  const role = document.getElementById('new-user-role').value;
  
  if (!email || !password) {
    showToast('Email and password are required', 'error');
    return;
  }
  if (password.length < 8) {
    showToast('Password must be at least 8 characters', 'error');
    return;
  }
  
  try {
    await api(`/admin/projects/${detailProjectId}/users`, {
      method: 'POST',
      body: JSON.stringify({ email, password, role })
    });
    showToast('User created successfully');
    closeSubModal('modal-create-user');
    await loadUsers();
  } catch (err) {
    showToast(err.message, 'error');
  }
}

async function deleteUser(id) {
  if (!await showConfirm('Delete User', 'Are you sure you want to delete this user?', 'Delete', true)) return;
  try {
    await api(`/admin/projects/${detailProjectId}/users/${id}`, {
      method: 'DELETE'
    });
    showToast('User deleted');
    await loadUsers();
  } catch (err) {
    showToast(err.message, 'error');
  }
}

// === OAUTH PROVIDER MANAGEMENT ===

const OAUTH_PRESETS = {
  google: {
    auth_url: 'https://accounts.google.com/o/oauth2/auth',
    token_url: 'https://oauth2.googleapis.com/token',
    userinfo_url: 'https://www.googleapis.com/oauth2/v3/userinfo',
    scopes: 'email, profile',
    email_field: 'email',
    name_field: 'name',
    avatar_field: 'picture',
    id_field: 'sub'
  },
  github: {
    auth_url: 'https://github.com/login/oauth/authorize',
    token_url: 'https://github.com/login/oauth/access_token',
    userinfo_url: 'https://api.github.com/user',
    scopes: 'user:email',
    email_field: 'email',
    name_field: 'login',
    avatar_field: 'avatar_url',
    id_field: 'id'
  },
  discord: {
    auth_url: 'https://discord.com/api/oauth2/authorize',
    token_url: 'https://discord.com/api/oauth2/token',
    userinfo_url: 'https://discord.com/api/users/@me',
    scopes: 'identify, email',
    email_field: 'email',
    name_field: 'username',
    avatar_field: 'avatar',
    id_field: 'id'
  },
  gitlab: {
    auth_url: 'https://gitlab.com/oauth/authorize',
    token_url: 'https://gitlab.com/oauth/token',
    userinfo_url: 'https://gitlab.com/api/v4/user',
    scopes: 'read_user',
    email_field: 'email',
    name_field: 'name',
    avatar_field: 'avatar_url',
    id_field: 'id'
  },
  apple: {
    auth_url: 'https://appleid.apple.com/auth/authorize',
    token_url: 'https://appleid.apple.com/auth/token',
    userinfo_url: 'https://appleid.apple.com/auth/userinfo',
    scopes: 'name, email',
    email_field: 'email',
    name_field: 'name',
    avatar_field: '',
    id_field: 'sub'
  }
};

const OAUTH_SVG_ICONS = {
  google: `<svg viewBox="0 0 24 24" width="20" height="20" xmlns="http://www.w3.org/2000/svg"><path d="M22.56 12.25c0-.78-.07-1.53-.2-2.25H12v4.26h5.92c-.26 1.37-1.04 2.53-2.21 3.31v2.77h3.57c2.08-1.92 3.28-4.74 3.28-8.09z" fill="#4285F4"/><path d="M12 23c2.97 0 5.46-.98 7.28-2.66l-3.57-2.77c-.98.66-2.23 1.06-3.71 1.06-2.86 0-5.29-1.93-6.16-4.53H2.18v2.84C3.99 20.53 7.7 23 12 23z" fill="#34A853"/><path d="M5.84 14.09c-.22-.66-.35-1.36-.35-2.09s.13-1.43.35-2.09V7.07H2.18C1.43 8.55 1 10.22 1 12s.43 3.45 1.18 4.93l3.66-2.84z" fill="#FBBC05"/><path d="M12 5.38c1.62 0 3.06.56 4.21 1.64l3.15-3.15C17.45 2.09 14.97 1 12 1 7.7 1 3.99 3.47 2.18 7.07l3.66 2.84c.87-2.6 3.3-4.53 6.16-4.53z" fill="#EA4335"/></svg>`,
  github: `<svg viewBox="0 0 24 24" width="20" height="20" xmlns="http://www.w3.org/2000/svg"><path fill="#e6edf3" d="M12 .297c-6.63 0-12 5.373-12 12 0 5.303 3.438 9.8 8.205 11.385.6.113.82-.258.82-.577 0-.285-.01-1.04-.015-2.04-3.338.724-4.042-1.61-4.042-1.61C4.422 18.07 3.633 17.7 3.633 17.7c-1.087-.744.084-.729.084-.729 1.205.084 1.838 1.236 1.838 1.236 1.07 1.835 2.809 1.305 3.495.998.108-.776.417-1.305.76-1.605-2.665-.3-5.466-1.332-5.466-5.93 0-1.31.465-2.38 1.235-3.22-.135-.303-.54-1.523.105-3.176 0 0 1.005-.322 3.3 1.23.96-.267 1.98-.399 3-.405 1.02.006 2.04.138 3 .405 2.28-1.552 3.285-1.23 3.285-1.23.645 1.653.24 2.873.12 3.176.765.84 1.23 1.91 1.23 3.22 0 4.61-2.805 5.625-5.475 5.92.42.36.81 1.096.81 2.22 0 1.606-.015 2.896-.015 3.286 0 .315.21.69.825.57C20.565 22.092 24 17.592 24 12.297c0-6.627-5.373-12-12-12"/></svg>`,
  discord: `<svg viewBox="0 0 24 24" width="20" height="20" xmlns="http://www.w3.org/2000/svg"><path fill="#5865F2" d="M20.317 4.37a19.791 19.791 0 0 0-4.885-1.515.074.074 0 0 0-.079.037c-.21.375-.444.864-.608 1.25a18.27 18.27 0 0 0-5.487 0 12.64 12.64 0 0 0-.617-1.25.077.077 0 0 0-.079-.037A19.736 19.736 0 0 0 3.677 4.37a.07.07 0 0 0-.032.027C.533 9.046-.32 13.58.099 18.057a.082.082 0 0 0 .031.057 19.9 19.9 0 0 0 5.993 3.03.078.078 0 0 0 .084-.028 14.09 14.09 0 0 0 1.226-1.994.076.076 0 0 0-.041-.106 13.107 13.107 0 0 1-1.872-.892.077.077 0 0 1-.008-.128 10.2 10.2 0 0 0 .372-.292.074.074 0 0 1 .077-.01c3.928 1.793 8.18 1.793 12.062 0a.074.074 0 0 1 .078.01c.12.098.246.198.373.292a.077.077 0 0 1-.006.127 12.299 12.299 0 0 1-1.873.892.077.077 0 0 0-.041.107c.36.698.772 1.362 1.225 1.993a.076.076 0 0 0 .084.028 19.839 19.839 0 0 0 6.002-3.03.077.077 0 0 0 .032-.054c.5-5.177-.838-9.674-3.549-13.66a.061.061 0 0 0-.031-.03zM8.02 15.33c-1.183 0-2.157-1.085-2.157-2.419 0-1.333.956-2.419 2.157-2.419 1.21 0 2.176 1.096 2.157 2.42 0 1.333-.956 2.418-2.157 2.418zm7.975 0c-1.183 0-2.157-1.085-2.157-2.419 0-1.333.955-2.419 2.157-2.419 1.21 0 2.176 1.096 2.157 2.42 0 1.333-.946 2.418-2.157 2.418z"/></svg>`,
  gitlab: `<svg viewBox="0 0 24 24" width="20" height="20" xmlns="http://www.w3.org/2000/svg"><path fill="#FC6D26" d="M4.845.904C4.371.904 4 1.275 4 1.749v20.502c0 .474.371.845.845.845h14.31c.474 0 .845-.371.845-.845V1.749c0-.474-.371-.845-.845-.845H4.845zm7.155 3.967l3.14 9.652H8.861L12 4.871zm-5.44 0l2.27 6.98-1.18 2.672H5.5L6.56 4.871zm10.88 0l1.06 9.652h-2.151l-1.18-2.672 2.27-6.98z"/></svg>`,
  apple: `<svg viewBox="0 0 24 24" width="20" height="20" xmlns="http://www.w3.org/2000/svg"><path fill="#f5f5f7" d="M12.152 6.896c-.948 0-2.415-1.078-3.96-1.04-2.04.027-3.91 1.183-4.961 3.014-2.117 3.675-.546 9.103 1.519 12.09 1.013 1.454 2.208 3.09 3.792 3.039 1.52-.065 2.09-.987 3.935-.987 1.831 0 2.35.987 3.96.948 1.637-.026 2.676-1.48 3.676-2.948 1.156-1.688 1.636-3.325 1.662-3.415-.039-.013-3.182-1.221-3.22-4.857-.026-3.04 2.48-4.494 2.597-4.559-1.429-2.09-3.623-2.324-4.39-2.376-2-.156-3.675 1.09-4.61 1.09zM15.53 3.83c.843-1.012 1.4-2.427 1.245-3.83-1.207.052-2.662.805-3.532 1.818-.78.896-1.454 2.338-1.273 3.714 1.338.104 2.715-.688 3.559-1.701z"/></svg>`,
  custom: `<svg viewBox="0 0 24 24" width="20" height="20" xmlns="http://www.w3.org/2000/svg"><path fill="#8b8b96" d="M12 15.5A3.5 3.5 0 018.5 12 3.5 3.5 0 0112 8.5a3.5 3.5 0 013.5 3.5 3.5 3.5 0 01-3.5 3.5m7.43-2.92c.04-.34.07-.69.07-1.08s-.03-.73-.07-1.08l2.33-1.82c.21-.16.27-.45.13-.68l-2.21-3.82c-.13-.23-.42-.31-.65-.23l-2.75 1.11c-.57-.44-1.18-.81-1.86-1.08L14.1 2.08A.52.52 0 0013.6 2h-3.2a.52.52 0 00-.52.42L9.4 4.91C8.72 5.18 8.11 5.55 7.54 6L4.79 4.89c-.23-.09-.52 0-.65.23L1.93 8.94c-.14.23-.08.52.13.68l2.33 1.82c-.04.35-.07.7-.07 1.08s.03.73.07 1.08L1.06 15.44c-.21.16-.27.45-.13.68l2.21 3.82c.13.23.42.31.65.23l2.75-1.11c.57.44 1.18.81 1.86 1.08l.48 2.83c.07.23.28.42.52.42h3.2c.24 0 .46-.19.52-.42l.48-2.83c.68-.27 1.29-.64 1.86-1.08l2.75 1.11c.23.09.52 0 .65-.23l2.21-3.82c.14-.23.08-.52-.13-.68l-2.33-1.82z"/></svg>`,
};

const OAUTH_PROVIDER_META = {
  google:  { icon: OAUTH_SVG_ICONS.google,  color: '#4285F4', bg: 'rgba(66,133,244,0.12)' },
  github:  { icon: OAUTH_SVG_ICONS.github,  color: '#e6edf3', bg: 'rgba(230,237,243,0.08)' },
  discord: { icon: OAUTH_SVG_ICONS.discord, color: '#5865F2', bg: 'rgba(88,101,242,0.12)' },
  gitlab:  { icon: OAUTH_SVG_ICONS.gitlab,  color: '#FC6D26', bg: 'rgba(252,109,38,0.12)' },
  apple:   { icon: OAUTH_SVG_ICONS.apple,   color: '#f5f5f5', bg: 'rgba(245,245,245,0.07)' },
  slack:   { icon: OAUTH_SVG_ICONS.custom,  color: '#4A154B', bg: 'rgba(74,21,75,0.18)' },
  twitter: { icon: OAUTH_SVG_ICONS.custom,  color: '#1DA1F2', bg: 'rgba(29,161,242,0.12)' },
  custom:  { icon: OAUTH_SVG_ICONS.custom,  color: '#5b5bff', bg: 'rgba(91,91,255,0.12)' },
};

function getProviderMeta(name) {
  return OAUTH_PROVIDER_META[(name || '').toLowerCase()] || OAUTH_PROVIDER_META.custom;
}

async function loadOAuthProviders() {
  try {
    const data = await api(`/admin/projects/${detailProjectId}/auth/providers`);
    activeProjectEnv.oauthProviders = data.providers || [];
    renderOAuthProviders();
  } catch (err) {
    showToast('Failed to load OAuth providers: ' + err.message, 'error');
  }
}

function renderOAuthProviders() {
  const container = document.getElementById('oauth-providers-list');
  if (!container) return;

  const providers = activeProjectEnv.oauthProviders;
  if (providers.length === 0) {
    container.innerHTML = `<div class="oauth-empty-state">
      <div class="oauth-empty-icon">🔐</div>
      <div class="oauth-empty-text">No OAuth providers yet. Add one to enable social login for your users.</div>
    </div>`;
    return;
  }

  container.innerHTML = providers.map((p, i) => {
    const clientId = p.client_id || '';
    const scopes = (p.scopes || []).join(', ');
    const meta = getProviderMeta(p.name);
    const shortId = clientId.substring(0, 14) + (clientId.length > 14 ? '…' : '');
    return `<div class="oauth-provider-card-v2" style="--provider-color:${meta.color};--provider-bg:${meta.bg};">
      <div class="oauth-provider-icon-wrap">${meta.icon}</div>
      <div class="oauth-provider-info">
        <div class="oauth-provider-name">${escapeHtml(p.name)}</div>
        <div class="oauth-provider-meta">
          <span class="oauth-provider-meta-chip">ID: ${escapeHtml(shortId)}</span>
          ${scopes ? `<span class="oauth-provider-meta-chip">${escapeHtml(scopes)}</span>` : ''}
        </div>
      </div>
      <div class="oauth-provider-actions">
        <button class="btn btn-xs btn-secondary" onclick="testOAuthProvider('${escapeHtml(p.name)}')" title="Test login flow">▶ Test</button>
        <button class="btn btn-xs btn-secondary" onclick="openEditOAuthProviderModal(${i})" title="Edit">✎ Edit</button>
        <button class="btn btn-xs btn-danger" onclick="deleteOAuthProvider(${i})" title="Delete">✕</button>
      </div>
    </div>`;
  }).join('');
}

function testOAuthProvider(providerName) {
  if (!detailApiKey) {
    showToast('Project key not available — try reopening the project', 'error');
    return;
  }
  const finalRedirect = encodeURIComponent(window.location.pathname + window.location.search);
  const url = `/auth/v1/oauth/${encodeURIComponent(providerName)}?project_key=${encodeURIComponent(detailApiKey)}&redirect=true&final_redirect=${finalRedirect}`;
  window.open(url, '_blank', 'noopener');
}

function openAddOAuthProviderModal() {
  openOAuthProviderForm('Add OAuth Provider', null, -1);
}

function openEditOAuthProviderModal(index) {
  const p = activeProjectEnv.oauthProviders[index];
  if (!p) return;
  openOAuthProviderForm('Edit OAuth Provider', p, index);
}

function applyOAuthPreset(name) {
  // Highlight selected preset button
  document.querySelectorAll('.oauth-preset-btn').forEach(b => b.classList.remove('selected'));
  const activeBtn = document.querySelector(`.oauth-preset-btn[data-preset="${name}"]`);
  if (activeBtn) activeBtn.classList.add('selected');

  const preset = OAUTH_PRESETS[name];
  if (!preset) {
    // Custom provider: reset everything, open advanced so the user can fill URLs
    document.getElementById('oauth-name').value = '';
    document.getElementById('oauth-redirect-url').value = '';
    clearOAuthAdvancedFields();
    const adv = document.getElementById('oauth-advanced');
    const toggle = document.getElementById('oauth-advanced-toggle');
    if (adv && toggle) {
      adv.style.display = 'flex';
      toggle.classList.add('open');
    }
    document.getElementById('oauth-name').focus();
    return;
  }

  document.getElementById('oauth-name').value = name;
  document.getElementById('oauth-auth-url').value = preset.auth_url;
  document.getElementById('oauth-token-url').value = preset.token_url;
  document.getElementById('oauth-userinfo-url').value = preset.userinfo_url;
  document.getElementById('oauth-scopes').value = preset.scopes;
  document.getElementById('oauth-email-field').value = preset.email_field;
  document.getElementById('oauth-name-field').value = preset.name_field;
  document.getElementById('oauth-avatar-field').value = preset.avatar_field;
  document.getElementById('oauth-id-field').value = preset.id_field;

  // Auto-generate redirect URL
  autoFillRedirect();

  // Expand advanced section
  const adv = document.getElementById('oauth-advanced');
  const toggle = document.getElementById('oauth-advanced-toggle');
  if (adv && toggle) {
    adv.style.display = 'flex';
    toggle.classList.add('open');
  }
}


function clearOAuthAdvancedFields() {
  document.getElementById('oauth-auth-url').value = '';
  document.getElementById('oauth-token-url').value = '';
  document.getElementById('oauth-userinfo-url').value = '';
  document.getElementById('oauth-scopes').value = '';
  document.getElementById('oauth-email-field').value = 'email';
  document.getElementById('oauth-name-field').value = 'name';
  document.getElementById('oauth-avatar-field').value = 'avatar_url';
  document.getElementById('oauth-id-field').value = 'id';
}

function autoFillRedirect() {
  const name = document.getElementById('oauth-name').value.trim();
  const redirectInput = document.getElementById('oauth-redirect-url');
  if (name) {
    redirectInput.value = window.location.origin + '/auth/v1/oauth/' + name + '/callback';
  } else {
    redirectInput.value = '';
  }
}


function toggleOAuthAdvanced() {
  const adv = document.getElementById('oauth-advanced');
  const toggle = document.getElementById('oauth-advanced-toggle');
  if (!adv || !toggle) return;
  if (adv.style.display === 'flex') {
    adv.style.display = 'none';
    toggle.classList.remove('open');
  } else {
    adv.style.display = 'flex';
    toggle.classList.add('open');
  }
}

function copyRedirectUrl() {
  const val = document.getElementById('oauth-redirect-url').value;
  if (!val) return;
  navigator.clipboard.writeText(val).then(() => showToast('Redirect URL copied'));
}

function openOAuthProviderForm(title, provider, editIndex) {
  let modal = document.getElementById('modal-oauth-provider');

  const PRESETS_CONFIG = [
    { key: 'google',  icon: OAUTH_SVG_ICONS.google,  label: 'Google',  color: '#4285F4' },
    { key: 'github',  icon: OAUTH_SVG_ICONS.github,  label: 'GitHub',  color: '#e6edf3' },
    { key: 'discord', icon: OAUTH_SVG_ICONS.discord, label: 'Discord', color: '#5865F2' },
    { key: 'gitlab',  icon: OAUTH_SVG_ICONS.gitlab,  label: 'GitLab',  color: '#FC6D26' },
    { key: 'apple',   icon: OAUTH_SVG_ICONS.apple,   label: 'Apple',   color: '#f5f5f5' },
    { key: 'custom',  icon: OAUTH_SVG_ICONS.custom,  label: 'Custom',  color: '#5b5bff' },
  ];

  if (!modal) {
    modal = document.createElement('div');
    modal.id = 'modal-oauth-provider';
    modal.className = 'modal-overlay';
    modal.innerHTML = `<div class="modal-content" id="modal-oauth-provider-inner" style="max-width:520px;max-height:88vh;overflow-y:auto;">
      <div class="modal-header">
        <div>
          <h3 id="modal-oauth-title" style="font-size:16px;font-weight:700;margin-bottom:2px;">${title}</h3>
          <p style="font-size:12px;color:var(--text-muted);margin:0;">Configure an OAuth2 social login provider</p>
        </div>
        <button class="modal-close" onclick="closeOAuthProviderModal()">✕</button>
      </div>
      <div class="modal-body" style="display:flex;flex-direction:column;gap:18px;">

        <!-- Preset grid -->
        <div>
          <div class="oauth-section-divider" style="margin-bottom:12px;"><span>Quick Preset</span></div>
          <div class="oauth-preset-grid">
            ${PRESETS_CONFIG.map(p =>
              `<button type="button" class="oauth-preset-btn" data-preset="${p.key}" onclick="applyOAuthPreset('${p.key}')" style="--preset-color:${p.color}">
                <span class="oauth-preset-icon">${p.icon}</span>
                <span class="oauth-preset-label">${p.label}</span>
              </button>`
            ).join('')}
          </div>
        </div>

        <!-- Required fields -->
        <div style="display:flex;flex-direction:column;gap:14px;">
          <div class="oauth-section-divider"><span>Credentials</span></div>
          <div class="form-group" style="margin-bottom:0;">
            <label for="oauth-name">Provider Name</label>
            <input type="text" id="oauth-name" placeholder="e.g. google, discord, github…" required oninput="autoFillRedirect()">
          </div>
          <div style="display:grid;grid-template-columns:1fr 1fr;gap:12px;">
            <div class="form-group" style="margin-bottom:0;">
              <label for="oauth-client-id">Client ID</label>
              <input type="text" id="oauth-client-id" placeholder="Client ID" required>
            </div>
            <div class="form-group" style="margin-bottom:0;">
              <label for="oauth-client-secret">Client Secret</label>
              <input type="password" id="oauth-client-secret" placeholder="Client secret" autocomplete="new-password">
            </div>
          </div>
          <p style="font-size:11px;color:var(--text-muted);margin-top:-6px;">🔒 Secrets are write-only — must be re-entered when editing.</p>
          <div class="form-group" style="margin-bottom:0;">
            <label for="oauth-redirect-url">Redirect URL</label>
            <div class="oauth-redirect-row">
              <input type="text" id="oauth-redirect-url" placeholder="Auto-filled from provider name" readonly>
              <button type="button" class="oauth-copy-btn" onclick="copyRedirectUrl()" title="Copy">⎘</button>
            </div>
          </div>
        </div>

        <!-- Advanced toggle -->
        <div>
          <div style="border-top:1px solid var(--border);margin-bottom:6px;"></div>
          <button type="button" id="oauth-advanced-toggle" class="oauth-advanced-toggle-btn" onclick="toggleOAuthAdvanced()">
            <span class="oauth-adv-arrow">▶</span>
            Advanced settings
            <span style="font-size:10px;color:var(--text-muted);font-weight:500;margin-left:auto;white-space:nowrap;">URLs &amp; field mapping</span>
          </button>
        </div>

        <div id="oauth-advanced" style="display:none;flex-direction:column;gap:14px;">
          <div style="display:flex;flex-direction:column;gap:12px;">
            <div class="form-group" style="margin-bottom:0;">
              <label for="oauth-auth-url">Authorization URL</label>
              <input type="text" id="oauth-auth-url" placeholder="https://provider.com/oauth/authorize">
            </div>
            <div class="form-group" style="margin-bottom:0;">
              <label for="oauth-token-url">Token URL</label>
              <input type="text" id="oauth-token-url" placeholder="https://provider.com/oauth/token">
            </div>
            <div class="form-group" style="margin-bottom:0;">
              <label for="oauth-userinfo-url">UserInfo URL</label>
              <input type="text" id="oauth-userinfo-url" placeholder="https://provider.com/userinfo">
            </div>
            <div class="form-group" style="margin-bottom:0;">
              <label for="oauth-scopes">Scopes</label>
              <input type="text" id="oauth-scopes" placeholder="email, profile">
            </div>
          </div>
          <div>
            <div class="oauth-section-divider" style="margin-bottom:12px;"><span>Field Mapping</span></div>
            <div class="oauth-field-grid">
              <div class="form-group" style="margin-bottom:0;">
                <label for="oauth-email-field">Email field</label>
                <input type="text" id="oauth-email-field" placeholder="email" value="email">
              </div>
              <div class="form-group" style="margin-bottom:0;">
                <label for="oauth-name-field">Name field</label>
                <input type="text" id="oauth-name-field" placeholder="name" value="name">
              </div>
              <div class="form-group" style="margin-bottom:0;">
                <label for="oauth-avatar-field">Avatar field</label>
                <input type="text" id="oauth-avatar-field" placeholder="avatar_url" value="avatar_url">
              </div>
              <div class="form-group" style="margin-bottom:0;">
                <label for="oauth-id-field">ID field</label>
                <input type="text" id="oauth-id-field" placeholder="id" value="id">
              </div>
            </div>
          </div>
        </div>

      </div>
      <div class="modal-actions" style="padding-top:8px;border-top:1px solid var(--border);margin-top:4px;">
        <button class="btn btn-secondary" onclick="closeOAuthProviderModal()">Cancel</button>
        <button class="btn btn-primary" id="btn-save-oauth-provider" onclick="saveOAuthProviderForm(${editIndex})" style="gap:8px;">💾 Save Provider</button>
      </div>
    </div>`;
    document.body.appendChild(modal);
  } else {
    document.getElementById('modal-oauth-title').textContent = title;
    document.getElementById('btn-save-oauth-provider').onclick = function() { saveOAuthProviderForm(editIndex); };
    // Reset all fields
    document.getElementById('oauth-name').value = '';
    document.getElementById('oauth-client-id').value = '';
    document.getElementById('oauth-client-secret').value = '';
    document.getElementById('oauth-client-secret').placeholder = 'Client secret';
    document.getElementById('oauth-redirect-url').value = '';
    clearOAuthAdvancedFields();
    // Deselect preset buttons
    document.querySelectorAll('.oauth-preset-btn').forEach(b => b.classList.remove('selected'));
    // Collapse advanced
    const adv = document.getElementById('oauth-advanced');
    const toggle = document.getElementById('oauth-advanced-toggle');
    if (adv) adv.style.display = 'none';
    if (toggle) toggle.classList.remove('open');
  }
  
  if (provider) {
    document.getElementById('oauth-name').value = provider.name || '';
    document.getElementById('oauth-client-id').value = provider.client_id || '';
    document.getElementById('oauth-client-secret').placeholder = 'Enter new secret (or leave blank to keep current)';
    document.getElementById('oauth-redirect-url').value = provider.redirect_url || '';
    document.getElementById('oauth-auth-url').value = provider.auth_url || '';
    document.getElementById('oauth-token-url').value = provider.token_url || '';
    document.getElementById('oauth-userinfo-url').value = provider.userinfo_url || '';
    document.getElementById('oauth-scopes').value = (provider.scopes || []).join(', ');
    document.getElementById('oauth-email-field').value = provider.email_field || 'email';
    document.getElementById('oauth-name-field').value = provider.name_field || 'name';
    document.getElementById('oauth-avatar-field').value = provider.avatar_field || 'avatar_url';
    document.getElementById('oauth-id-field').value = provider.id_field || 'id';
    // Highlight matching preset
    const matchingPreset = document.querySelector(`.oauth-preset-btn[data-preset="${provider.name}"]`);
    if (matchingPreset) matchingPreset.classList.add('selected');
    // Expand advanced if endpoints are configured
    if (provider.auth_url || provider.token_url || provider.userinfo_url) {
      const adv = document.getElementById('oauth-advanced');
      const toggle = document.getElementById('oauth-advanced-toggle');
      if (adv) adv.style.display = 'flex';
      if (toggle) toggle.classList.add('open');
    }
  }
  
  modal.style.display = 'flex';
}

function closeOAuthProviderModal() {
  const modal = document.getElementById('modal-oauth-provider');
  if (modal) modal.style.display = 'none';
}

function saveOAuthProviderForm(editIndex) {
  const name = document.getElementById('oauth-name').value.trim();
  const clientId = document.getElementById('oauth-client-id').value.trim();
  const clientSecret = document.getElementById('oauth-client-secret').value.trim();
  const redirectUrl = document.getElementById('oauth-redirect-url').value.trim();
  const authUrl = document.getElementById('oauth-auth-url').value.trim();
  const tokenUrl = document.getElementById('oauth-token-url').value.trim();
  const userinfoUrl = document.getElementById('oauth-userinfo-url').value.trim();
  const scopesStr = document.getElementById('oauth-scopes').value.trim();
  const emailField = document.getElementById('oauth-email-field').value.trim();
  const nameField = document.getElementById('oauth-name-field').value.trim();
  const avatarField = document.getElementById('oauth-avatar-field').value.trim();
  const idField = document.getElementById('oauth-id-field').value.trim();
  
  if (!name) { showToast('Provider name is required', 'error'); return; }
  if (!clientId) { showToast('Client ID is required', 'error'); return; }
  
  const scopes = scopesStr ? scopesStr.split(',').map(s => s.trim()).filter(s => s) : [];
  
  const provider = {
    name,
    client_id: clientId,
    redirect_url: redirectUrl,
    auth_url: authUrl,
    token_url: tokenUrl,
    userinfo_url: userinfoUrl,
    scopes,
    email_field: emailField || 'email',
    name_field: nameField || 'name',
    avatar_field: avatarField || 'avatar_url',
    id_field: idField || 'id',
  };
  
  if (clientSecret) {
    provider.client_secret = clientSecret;
  } else if (editIndex >= 0) {
    showToast('Client Secret is required when saving (cannot be retrieved)', 'error');
    return;
  }
  
  if (!clientSecret && editIndex < 0) {
    showToast('Client Secret is required for new providers', 'error');
    return;
  }
  
  const providers = [...activeProjectEnv.oauthProviders];
  if (editIndex >= 0) {
    providers[editIndex] = provider;
  } else {
    providers.push(provider);
  }
  
  saveOAuthProviders(providers);
}

async function saveOAuthProviders(providers) {
  const btn = document.getElementById('btn-save-oauth-provider');
  btn.disabled = true;
  btn.textContent = 'Saving...';
  
  try {
    await api(`/admin/projects/${detailProjectId}/auth/providers`, {
      method: 'PUT',
      body: JSON.stringify({ providers })
    });
    showToast('OAuth providers saved');
    closeOAuthProviderModal();
    await loadOAuthProviders();
  } catch (err) {
    showToast('Failed to save OAuth providers: ' + err.message, 'error');
  } finally {
    btn.disabled = false;
    btn.textContent = 'Save Provider';
  }
}

async function deleteOAuthProvider(index) {
  if (!await showConfirm('Delete Provider', 'Remove this OAuth provider? This cannot be undone.', 'Delete', true)) return;
  
  const providers = activeProjectEnv.oauthProviders.filter((_, i) => i !== index);
  try {
    await api(`/admin/projects/${detailProjectId}/auth/providers`, {
      method: 'PUT',
      body: JSON.stringify({ providers })
    });
    showToast('OAuth provider removed');
    await loadOAuthProviders();
  } catch (err) {
    showToast('Failed to remove OAuth provider: ' + err.message, 'error');
  }
}
