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
    tbody.innerHTML = '<tr><td colspan="5" class="text-muted" style="text-align:center;">No users registered yet</td></tr>';
    return;
  }
  tbody.innerHTML = activeProjectEnv.users.map(u => {
    const id = u.id || '';
    const email = u.email || '';
    const role = u.role || 'user';
    const created = formatDate(u.created_at);
    const shortId = id.substring(0, 8) + '...';
    
    return `<tr>
      <td class="td-id" title="${escapeHtml(id)}">${escapeHtml(shortId)}</td>
      <td class="td-name">${escapeHtml(email)}</td>
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
