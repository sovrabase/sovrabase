// === STORAGE EXPLORER ===

async function loadBuckets() {
  try {
    const data = await api(`/admin/projects/${detailProjectId}/storage/buckets`);
    activeProjectEnv.buckets = data.buckets || [];
    renderBucketsList();
  } catch (err) {
    showToast('Failed to load buckets: ' + err.message, 'error');
  }
}

function renderBucketsList() {
  const container = document.getElementById('storage-buckets-list');
  if (!container) return;
  
  if (activeProjectEnv.buckets.length === 0) {
    container.innerHTML = '<span class="text-muted" style="font-size:12px;">No buckets</span>';
    selectBucket(null);
    return;
  }
  container.innerHTML = activeProjectEnv.buckets.map(b => {
    const activeClass = activeProjectEnv.selectedBucket === b ? 'active' : '';
    return `<div class="nav-item ${activeClass}" style="padding:6px 10px; font-size:12.5px; border:1px solid transparent;" onclick="selectBucket('${escapeHtml(b)}')">
      <span class="nav-icon">📁</span> ${escapeHtml(b)}
    </div>`;
  }).join('');
  
  if (activeProjectEnv.selectedBucket && !activeProjectEnv.buckets.includes(activeProjectEnv.selectedBucket)) {
    selectBucket(null);
  }
}

function selectBucket(name) {
  activeProjectEnv.selectedBucket = name;
  document.querySelectorAll('#storage-buckets-list .nav-item').forEach(el => {
    if (el.textContent.includes(name)) el.classList.add('active');
    else el.classList.remove('active');
  });
  
  if (!name) {
    document.getElementById('storage-no-bucket-selected').style.display = 'block';
    document.getElementById('storage-bucket-content').style.display = 'none';
    return;
  }
  document.getElementById('storage-no-bucket-selected').style.display = 'none';
  document.getElementById('storage-bucket-content').style.display = 'flex';
  document.getElementById('selected-bucket-title').textContent = name;
  loadFiles();
}

async function loadFiles() {
  const bucket = activeProjectEnv.selectedBucket;
  if (!bucket) return;
  try {
    const files = await api(`/admin/projects/${detailProjectId}/storage/buckets/${bucket}/files`);
    activeProjectEnv.files = files || [];
    renderFilesTable();
  } catch (err) {
    showToast('Failed to load files: ' + err.message, 'error');
  }
}

function renderFilesTable() {
  const tbody = document.getElementById('storage-files-tbody');
  if (!tbody) return;
  
  if (activeProjectEnv.files.length === 0) {
    tbody.innerHTML = '<tr><td colspan="4" class="text-muted" style="text-align:center;">No files in this bucket</td></tr>';
    return;
  }
  tbody.innerHTML = activeProjectEnv.files.map(f => {
    const name = f.path || '';
    const size = formatBytes(f.size || 0);
    const type = f.content_type || 'binary';
    const url = f.url || '';
    
    return `<tr>
      <td class="td-name">${escapeHtml(name)}</td>
      <td class="mono" style="font-size:12px;">${size}</td>
      <td style="color:var(--text-secondary); font-size:12px;">${escapeHtml(type)}</td>
      <td class="td-actions">
        ${url ? `<a href="${escapeHtml(url)}" target="_blank" class="btn btn-xs btn-secondary" style="text-decoration:none;">Download</a>` : ''}
        <button class="btn btn-xs btn-danger" onclick="deleteFile('${escapeHtml(name)}')">Delete</button>
      </td>
    </tr>`;
  }).join('');
}

function openCreateBucketModal() {
  document.getElementById('new-bucket-name').value = '';
  openSubModal('modal-create-bucket');
}

async function submitCreateBucket() {
  const name = document.getElementById('new-bucket-name').value.trim();
  if (!name) { showToast('Please enter a bucket name', 'error'); return; }
  try {
    await api(`/admin/projects/${detailProjectId}/storage/buckets`, {
      method: 'POST',
      body: JSON.stringify({ name })
    });
    showToast(`Bucket "${name}" created`);
    closeSubModal('modal-create-bucket');
    await loadBuckets();
    selectBucket(name);
  } catch (err) {
    showToast(err.message, 'error');
  }
}

async function deleteSelectedBucket() {
  const name = activeProjectEnv.selectedBucket;
  if (!name) return;
  if (!await showConfirm('Delete Bucket', `Are you sure you want to delete bucket "${name}"? It must be empty first.`, 'Delete', true)) return;
  try {
    await api(`/admin/projects/${detailProjectId}/storage/buckets/${name}`, {
      method: 'DELETE'
    });
    showToast(`Bucket "${name}" deleted`);
    activeProjectEnv.selectedBucket = null;
    await loadBuckets();
  } catch (err) {
    showToast(err.message, 'error');
  }
}

function triggerFileUpload() {
  document.getElementById('storage-hidden-file-input').click();
}

async function handleFileSelected(e) {
  const file = e.target.files[0];
  if (!file) return;
  
  const bucket = activeProjectEnv.selectedBucket;
  const formData = new FormData();
  formData.append('file', file);
  formData.append('path', file.name);
  
  const token = localStorage.getItem('sovrabase_admin_token');
  try {
    showToast('Uploading file...');
    const res = await fetch(`/admin/projects/${detailProjectId}/storage/buckets/${bucket}/files`, {
      method: 'POST',
      headers: {
        'Authorization': 'Bearer ' + token
      },
      body: formData
    });
    if (!res.ok) {
      const data = await res.json();
      throw new Error(data.error || 'Upload failed');
    }
    showToast(`Uploaded "${file.name}" successfully`);
    await loadFiles();
  } catch (err) {
    showToast(err.message, 'error');
  } finally {
    e.target.value = '';
  }
}

async function deleteFile(path) {
  if (!await showConfirm('Delete File', `Are you sure you want to delete file "${path}"?`, 'Delete', true)) return;
  const bucket = activeProjectEnv.selectedBucket;
  try {
    await api(`/admin/projects/${detailProjectId}/storage/buckets/${bucket}/files/${encodeURIComponent(path)}`, {
      method: 'DELETE'
    });
    showToast('File deleted');
    await loadFiles();
  } catch (err) {
    showToast(err.message, 'error');
  }
}
