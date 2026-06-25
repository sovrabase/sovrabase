const API_BASE = '';

// ===== API REQUESTER =====
async function api(path, opts = {}) {
  const token = localStorage.getItem('sovrabase_admin_token');
  const headers = { 
    'Content-Type': 'application/json',
    ...opts.headers
  };
  if (token) {
    headers['Authorization'] = 'Bearer ' + token;
  }
  
  const res = await fetch(API_BASE + path, {
    headers,
    ...opts
  });
  
  if (res.status === 401 && path !== '/admin/login') {
    localStorage.removeItem('sovrabase_admin_token');
    if (typeof showLogin === 'function') {
      showLogin();
    }
    throw new Error('Unauthorized');
  }
  
  let data;
  const ct = res.headers.get('content-type') || '';
  if (ct.includes('application/json')) {
    data = await res.json();
  } else {
    data = await res.text();
  }
  if (!res.ok) throw new Error(data.error || data.message || `HTTP ${res.status}`);
  return data;
}

// ===== TOASTS =====
function showToast(msg, type) {
  type = type || 'success';
  const container = document.getElementById('toast-container');
  const icons = { success: '✓', error: '✕', info: 'ℹ' };
  const toast = document.createElement('div');
  toast.className = 'toast toast-' + type;
  toast.innerHTML = `<span class="toast-icon">${icons[type] || ''}</span>${escapeHtml(msg)}`;
  container.appendChild(toast);
  setTimeout(() => {
    toast.classList.add('toast-exit');
    setTimeout(() => toast.remove(), 260);
  }, 4000);
}

// ===== HTML ESCAPING =====
function escapeHtml(str) {
  const div = document.createElement('div');
  div.textContent = str || '';
  return div.innerHTML;
}

// ===== DATE FORMATTER =====
function formatDate(ts) {
  if (!ts) return '—';
  return new Date(ts).toLocaleDateString('en-US', {
    year: 'numeric', month: 'short', day: 'numeric',
    hour: '2-digit', minute: '2-digit'
  });
}

// ===== BYTES FORMATTER =====
function formatBytes(bytes, decimals = 2) {
  if (bytes === 0) return '0 Bytes';
  const k = 1024;
  const dm = decimals < 0 ? 0 : decimals;
  const sizes = ['Bytes', 'KB', 'MB', 'GB', 'TB'];
  const i = Math.floor(Math.log(bytes) / Math.log(k));
  return parseFloat((bytes / Math.pow(k, i)).toFixed(dm)) + ' ' + sizes[i];
}

// ===== HUMAN-READABLE BYTES (for form inputs) =====
function humanizeBytes(bytes) {
  if (bytes <= 0) return { val: 0, unit: 'MB' };
  const mb = bytes / (1024 * 1024);
  if (mb < 1024) return { val: parseFloat(mb.toFixed(1)), unit: 'MB' };
  const gb = bytes / (1024 * 1024 * 1024);
  if (gb < 1024) return { val: parseFloat(gb.toFixed(2)), unit: 'GB' };
  return { val: parseFloat((bytes / (1024 * 1024 * 1024 * 1024)).toFixed(2)), unit: 'TB' };
}

// ===== QUOTA UNIT CONVERTER =====
function quotaToBytes(val, unit) {
  const m = { 'MB': 1024 * 1024, 'GB': 1024 * 1024 * 1024, 'TB': 1024 * 1024 * 1024 * 1024 };
  return Math.round(val * (m[unit] || 1));
}

// ===== COPY TEXT =====
function copyText(elId) {
  const el = document.getElementById(elId);
  if (!el) return;
  const text = el.textContent || el.innerText;
  navigator.clipboard.writeText(text).then(() => {
    showToast('Copied to clipboard');
  }).catch(() => {
    showToast('Failed to copy', 'error');
  });
}

// ===== CUSTOM CONFIRMATION MODAL =====
let confirmResolver = null;

function showConfirm(title, message, confirmText = 'Confirm', isDanger = false) {
  return new Promise((resolve) => {
    document.getElementById('confirm-title').textContent = title;
    document.getElementById('confirm-message').textContent = message;
    
    const confirmBtn = document.getElementById('confirm-btn-ok');
    confirmBtn.textContent = confirmText;
    
    if (isDanger) {
      confirmBtn.className = 'btn btn-danger';
    } else {
      confirmBtn.className = 'btn btn-primary';
    }
    
    document.getElementById('modal-confirm-custom').style.display = 'flex';
    confirmResolver = resolve;
  });
}

function handleConfirmResult(result) {
  document.getElementById('modal-confirm-custom').style.display = 'none';
  if (confirmResolver) {
    confirmResolver(result);
    confirmResolver = null;
  }
}
