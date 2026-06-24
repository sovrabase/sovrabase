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

  const token = localStorage.getItem('sovrabase_admin_token');
  const bucket = activeProjectEnv.selectedBucket;

  tbody.innerHTML = activeProjectEnv.files.map(f => {
    const name = f.path || '';
    const size = formatBytes(f.size || 0);
    const type = f.content_type || 'binary';
    const downloadUrl = f.url || `/admin/projects/${detailProjectId}/storage/buckets/${bucket}/files/${encodeURIComponent(name)}?token=${token}`;
    
    return `<tr>
      <td class="td-name">${escapeHtml(name)}</td>
      <td class="mono" style="font-size:12px;">${size}</td>
      <td style="color:var(--text-secondary); font-size:12px;">${escapeHtml(type)}</td>
      <td class="td-actions">
        <button class="btn btn-xs btn-secondary" onclick="previewFile('${escapeHtml(name)}', '${escapeHtml(type)}', '${escapeHtml(downloadUrl)}')">Preview</button>
        <a href="${escapeHtml(downloadUrl)}" target="_blank" class="btn btn-xs btn-secondary" style="text-decoration:none;">Download</a>
        <button class="btn btn-xs btn-danger" onclick="deleteFile('${escapeHtml(name)}')">Delete</button>
      </td>
    </tr>`;
  }).join('');
}

async function previewFile(name, contentType, url) {
  openSubModal('modal-preview-file');
  document.getElementById('preview-file-title').textContent = name;
  const body = document.getElementById('preview-file-body');
  body.style.alignItems = 'center';
  body.innerHTML = '<div style="display:flex; flex-direction:column; align-items:center; gap:8px;"><span class="spinner"></span><span style="font-size:12px; color:var(--text-secondary);">Loading preview...</span></div>';

  const type = (contentType || 'application/octet-stream').toLowerCase();
  
  if (type.startsWith('image/')) {
    body.innerHTML = `<img src="${url}" style="max-width: 100%; max-height: 60vh; object-fit: contain; border-radius: var(--radius); border: 1px solid var(--border);" onerror="this.onerror=null; showPreviewFallback('${escapeHtml(name)}', '${escapeHtml(contentType)}', '${escapeHtml(url)}');" />`;
  } else if (type === 'application/pdf') {
    body.innerHTML = `<iframe src="${url}" style="width: 100%; height: 60vh; border: none; background: white; border-radius: var(--radius);" onerror="showPreviewFallback('${escapeHtml(name)}', '${escapeHtml(contentType)}', '${escapeHtml(url)}');"></iframe>`;
  } else if (
    type.startsWith('text/') ||
    type === 'application/json' ||
    type === 'application/javascript' ||
    type === 'application/xml' ||
    type === 'application/x-javascript' ||
    type === 'text/javascript'
  ) {
    try {
      const res = await fetch(url);
      if (!res.ok) throw new Error(`HTTP ${res.status}`);
      let text = await res.text();
      if (text.length > 200000) {
        text = text.slice(0, 200000) + '\n\n... [Content truncated, file too large to preview completely]';
      }
      body.style.alignItems = 'flex-start';
      body.innerHTML = `<pre style="width: 100%; margin: 0; text-align: left;"><code style="font-family: var(--font-mono); font-size: 12.5px; white-space: pre-wrap; word-break: break-all; color: var(--text-primary);">${escapeHtml(text)}</code></pre>`;
    } catch (err) {
      body.style.alignItems = 'center';
      body.innerHTML = `<div class="text-danger" style="font-size:13px; text-align:center;">Failed to load text content: ${escapeHtml(err.message)}</div>`;
    }
  } else {
    showPreviewFallback(name, contentType, url);
  }
}

function showPreviewFallback(name, contentType, url) {
  const body = document.getElementById('preview-file-body');
  body.style.alignItems = 'center';
  body.innerHTML = `
    <div style="text-align: center; padding: 24px 16px; display: flex; flex-direction: column; align-items: center; gap: 16px; width: 100%;">
      <span style="font-size: 48px;">📄</span>
      <div style="display: flex; flex-direction: column; gap: 4px;">
        <div style="font-size: 14px; font-weight: 500; color: var(--text-primary);">${escapeHtml(name)}</div>
        <div style="font-size: 12px; color: var(--text-secondary);">${escapeHtml(contentType || 'unknown type')}</div>
      </div>
      <div style="font-size: 13px; color: var(--text-muted); max-width: 320px;">No inline preview available for this file format.</div>
      <a href="${url}" target="_blank" class="btn btn-primary" style="text-decoration: none; display: inline-flex; align-items: center; gap: 8px; margin-top: 8px;">
        <span>📥</span> Download File
      </a>
    </div>
  `;
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
