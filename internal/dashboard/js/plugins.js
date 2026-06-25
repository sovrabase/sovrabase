
async function loadPlugins() {
  try {
    const data = await api('/admin/plugins');
    
    // Plugins list
    const pluginsList = document.getElementById('plugins-list');
    if (data.plugins && data.plugins.length > 0) {
      pluginsList.innerHTML = data.plugins.map(p => 
        `<div style="display:inline-flex;align-items:center;gap:8px;background:var(--bg-input);border:1px solid var(--border);border-radius:var(--radius);padding:6px 14px;margin:4px;font-size:13px;">
          <span>🧩</span> <strong>${escapeHtml(p)}</strong>
        </div>`
      ).join('');
    } else {
      pluginsList.innerHTML = '<span class="text-muted">No plugins registered. <a href="https://github.com/ketsuna-org/sovrabase" target="_blank" style="color:var(--accent);">Learn how to create one →</a></span>';
    }

    // Hooks table
    const hooksTbody = document.getElementById('hooks-tbody');
    if (data.hooks && data.hooks.length > 0) {
      hooksTbody.innerHTML = data.hooks.map(h => `
        <tr>
          <td><span class="badge" style="background:${hookColor(h.type)};color:#fff;font-size:11px;">${escapeHtml(h.type)}</span></td>
          <td>${escapeHtml(h.action || '—')}</td>
          <td>${escapeHtml(h.collection || '—')}</td>
          <td>${h.count}</td>
        </tr>
      `).join('');
    } else {
      hooksTbody.innerHTML = '<tr><td colspan="4" class="text-muted" style="text-align:center;">No hooks registered</td></tr>';
    }

    // Routes table
    const routesTbody = document.getElementById('routes-tbody');
    if (data.routes && data.routes.length > 0) {
      routesTbody.innerHTML = data.routes.map(r => `
        <tr>
          <td><span class="badge" style="background:${methodColor(r.method)};color:#fff;font-size:11px;">${escapeHtml(r.method)}</span></td>
          <td style="font-family:var(--font-mono);font-size:12px;">${escapeHtml(r.path)}</td>
        </tr>
      `).join('');
    } else {
      routesTbody.innerHTML = '<tr><td colspan="2" class="text-muted" style="text-align:center;">No custom routes registered</td></tr>';
    }
  } catch (e) {
    console.error('Failed to load plugins', e);
    document.getElementById('plugins-list').innerHTML = '<span class="text-muted">Failed to load plugin data.</span>';
  }
}

function hookColor(type) {
  var colors = { record: '#49cc90', auth: '#fca130', storage: '#61affe', realtime: '#f93e3e', email: '#8b5cf6', serve: '#5b5bff', terminate: '#ff5252', log: '#5c5c66', collection: '#00c853' };
  return colors[type] || '#5c5c66';
}
function methodColor(method) {
  var colors = { GET: '#61affe', POST: '#49cc90', PUT: '#fca130', PATCH: '#50e3c2', DELETE: '#f93e3e' };
  return colors[method] || '#5c5c66';
}
