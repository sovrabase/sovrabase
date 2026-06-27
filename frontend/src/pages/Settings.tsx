import { useEffect, useState, useCallback } from 'react';
import { Shield, Users, Cloud, Mail, GitBranch, ScrollText, Database, Loader2, Plus, RefreshCw, Save, Zap, Trash2 } from 'lucide-react';
import { api, formatBytes, formatDate } from '../api';
import { useToast } from '../components/Toast';
import Modal from '../components/Modal';
import type { AdminUser, Backup } from '../types';

type TabKey = 'account' | 'admins' | 'security' | 'captcha' | 's3' | 'smtp' | 'replication' | 'audit' | 'backups';

interface TabDef { key: TabKey; label: string; icon: typeof Shield; }

interface AuditEntry { timestamp: string; admin: string; action: string; target: string; details: string; }

const tabs: TabDef[] = [
  { key: 'account', label: 'Admin Account', icon: Shield },
  { key: 'admins', label: 'Admins', icon: Users },
  { key: 'security', label: 'Security & Rate Limits', icon: Shield },
  { key: 'captcha', label: 'Captcha', icon: Shield },
  { key: 's3', label: 'S3 Storage', icon: Cloud },
  { key: 'smtp', label: 'SMTP', icon: Mail },
  { key: 'replication', label: 'Replication', icon: GitBranch },
  { key: 'audit', label: 'Audit Log', icon: ScrollText },
  { key: 'backups', label: 'Backups', icon: Database },
];

const inputCls = 'w-full px-3 py-2 rounded-lg bg-bg-input border border-border text-text-primary placeholder:text-text-muted focus:outline-none focus:border-accent transition-colors text-sm';
const labelCls = 'text-text-secondary text-sm font-medium';

const toggleBtn = (on: boolean) =>
  `w-10 h-5 rounded-full transition-colors ${on ? 'bg-accent' : 'bg-bg-input border border-border'}`;
const toggleDot = (on: boolean) =>
  `block w-4 h-4 rounded-full bg-white transition-transform ${on ? 'translate-x-5' : 'translate-x-0.5'}`;

export default function Settings() {
  const { showToast } = useToast();
  const [tab, setTab] = useState<TabKey>('account');
  const [form, setForm] = useState<Record<string, string | boolean>>({});
  const [confirmPw, setConfirmPw] = useState('');
  const [systemInfo, setSystemInfo] = useState<Record<string, string> | null>(null);
  const [loading, setLoading] = useState(false);
  const [saving, setSaving] = useState(false);
  const [admins, setAdmins] = useState<AdminUser[]>([]);
  const [backups, setBackups] = useState<Backup[]>([]);
  const [audit, setAudit] = useState<AuditEntry[]>([]);
  const [auditOffset, setAuditOffset] = useState(0);
  const [auditTotal, setAuditTotal] = useState(0);
  const [auditAction, setAuditAction] = useState('');
  const [auditTarget, setAuditTarget] = useState('');
  const [showAddAdmin, setShowAddAdmin] = useState(false);
  const [showRestartConfirm, setShowRestartConfirm] = useState(false);
  const [newAdmin, setNewAdmin] = useState({ email: '', password: '' });

  const auditLimit = 50;

  // ----- data loading -----
  const loadConfig = useCallback(async () => {
    try {
      const data = await api<Record<string, unknown>>('/admin/config');
      const flat: Record<string, string | boolean> = {};
      for (const [k, v] of Object.entries(data)) {
        if (typeof v === 'boolean' || typeof v === 'string') flat[k] = v;
        else if (typeof v === 'number') flat[k] = String(v);
        else if (Array.isArray(v)) flat[k] = v.join('\n');
      }
      setForm(flat);
    } catch { /* no config yet */ }
  }, []);

  const loadSystemInfo = useCallback(async () => {
    try {
      const data = await api<Record<string, unknown>>('/admin/stats');
      const info: Record<string, string> = {};
      for (const [k, v] of Object.entries(data)) {
        if (typeof v === 'string' || typeof v === 'number' || typeof v === 'boolean') info[k] = String(v);
      }
      setSystemInfo(info);
    } catch { /* stats unavailable */ }
  }, []);

  const loadAdmins = useCallback(async () => {
    try { const d = await api<{ admins: AdminUser[] }>('/admin/admins'); setAdmins(d.admins || []); } catch { /* */ }
  }, []);

  const loadAudit = useCallback(async (offset = 0, action = '', target = '') => {
    setLoading(true);
    try {
      const params = new URLSearchParams({ offset: String(offset), limit: String(auditLimit) });
      if (action) params.set('action', action);
      if (target) params.set('target', target);
      const d = await api<{ entries: AuditEntry[]; total?: number }>(`/admin/audit-logs?${params}`);
      setAudit(d.entries || []);
      setAuditTotal(d.total ?? 0);
      setAuditOffset(offset);
    } catch { setAudit([]); setAuditTotal(0); }
    finally { setLoading(false); }
  }, []);

  const loadBackups = useCallback(async () => {
    try { const d = await api<{ backups: Backup[] }>('/admin/backups'); setBackups(d.backups || []); } catch { /* */ }
  }, []);

  useEffect(() => { loadConfig(); loadSystemInfo(); }, [loadConfig, loadSystemInfo]);

  useEffect(() => {
    if (tab === 'admins') loadAdmins();
    else if (tab === 'audit') { setAuditOffset(0); loadAudit(0, auditAction, auditTarget); }
    else if (tab === 'backups') loadBackups();
    else setLoading(false);
  }, [tab]);

  // ----- actions -----
  const saveConfig = useCallback(async (): Promise<boolean> => {
    if (form.admin_password && form.admin_password !== confirmPw) {
      showToast('Passwords do not match', 'error');
      return false;
    }
    setSaving(true);
    try {
      const body: Record<string, unknown> = { ...form };
      if (body.peers && typeof body.peers === 'string') {
        body.peers = (body.peers as string).split('\n').map((s: string) => s.trim()).filter(Boolean);
      }
      // Convert numeric fields from string to number for the backend
      for (const numKey of ['rate_limit_per_minute', 'rate_limit_burst', 'smtp_port']) {
        if (body[numKey] != null && body[numKey] !== '') {
          body[numKey] = Number(body[numKey]);
        }
      }
      if (body.admin_password === '') delete body.admin_password;
      await api('/admin/config', { method: 'POST', body: JSON.stringify(body) });
      showToast('Settings saved', 'success');
      return true;
    } catch (err) {
      showToast((err as Error).message, 'error');
      return false;
    } finally { setSaving(false); }
  }, [form, confirmPw, showToast]);

  const handleRestart = useCallback(async () => {
    const ok = await saveConfig();
    if (!ok) return;
    setShowRestartConfirm(true);
  }, [saveConfig]);

  const confirmRestart = useCallback(async () => {
    setShowRestartConfirm(false);
    try {
      await api('/admin/restart', { method: 'POST' });
      showToast('Server restarting...', 'info');
    } catch (err) {
      showToast((err as Error).message, 'error');
    }
  }, [showToast]);

  const handleAddAdmin = async () => {
    try {
      await api('/admin/admins', { method: 'POST', body: JSON.stringify(newAdmin) });
      showToast('Admin added', 'success');
      setShowAddAdmin(false);
      setNewAdmin({ email: '', password: '' });
      loadAdmins();
    } catch (err) { showToast((err as Error).message, 'error'); }
  };

  const createBackup = async () => {
    try {
      await api('/admin/backups', { method: 'POST' });
      showToast('Backup created', 'success');
      loadBackups();
    } catch (err) { showToast((err as Error).message, 'error'); }
  };


  const clearAudit = async () => {
    try {
      await api('/admin/audit-logs', { method: 'DELETE' });
      showToast('Audit log cleared', 'success');
      loadAudit(0, auditAction, auditTarget);
    } catch (err) { showToast((err as Error).message, 'error'); }
  };

  const auditPage = () => {
    const start = auditTotal === 0 ? 0 : auditOffset + 1;
    const end = Math.min(auditOffset + audit.length, auditTotal);
    return { start, end, total: auditTotal };
  };

  // ----- field helpers -----
  const f = (key: string, label: string, opts?: { type?: string; ph?: string; toggle?: boolean }) => {
    if (opts?.toggle) {
      return (
        <div key={key} className="flex items-center justify-between py-2">
          <span className={labelCls}>{label}</span>
          <button onClick={() => setForm((fv) => ({ ...fv, [key]: !fv[key] }))} className={toggleBtn(!!form[key])}>
            <span className={toggleDot(!!form[key])} />
          </button>
        </div>
      );
    }
    return (
      <div key={key} className="space-y-1">
        <label className={labelCls}>{label}</label>
        <input
          type={opts?.type || 'text'}
          value={(form[key] as string) ?? ''}
          onChange={(e) => setForm((fv) => ({ ...fv, [key]: e.target.value }))}
          className={inputCls}
          placeholder={opts?.ph}
        />
      </div>
    );
  };

  const sel = (key: string, label: string, options: string[]) => (
    <div key={key} className="space-y-1">
      <label className={labelCls}>{label}</label>
      <select
        value={(form[key] as string) ?? ''}
        onChange={(e) => setForm((fv) => ({ ...fv, [key]: e.target.value }))}
        className={inputCls}
      >
        {options.map((o) => <option key={o} value={o}>{o}</option>)}
      </select>
    </div>
  );

  // ----- tab content -----
  const renderContent = () => {
    if (loading && tab !== 'audit') {
      return <div className="flex justify-center py-16"><Loader2 className="w-6 h-6 text-accent animate-spin" /></div>;
    }

    switch (tab) {
      case 'account':
        return (
          <div className="space-y-4 max-w-md">
            {f('admin_email', 'Email', { type: 'email' })}
            {f('admin_password', 'New Password', { type: 'password', ph: 'Leave blank to keep current' })}
            <div className="space-y-1">
              <label className={labelCls}>Confirm Password</label>
              <input type="password" value={confirmPw} onChange={(e) => setConfirmPw(e.target.value)} className={inputCls} placeholder="Re-enter new password" />
            </div>
            {f('session_duration', 'Session Duration', { ph: '24h' })}
            {f('backup_interval', 'Backup Interval', { ph: '24h' })}
          </div>
        );

      case 'admins':
        return (
          <div className="space-y-4">
            <button onClick={() => setShowAddAdmin(true)} className="flex items-center gap-2 px-4 py-2 rounded-lg bg-accent text-white text-sm font-medium hover:bg-accent-hover">
              <Plus className="w-4 h-4" />Add Admin
            </button>
            <div className="bg-bg-card border border-border rounded-xl overflow-hidden">
              <table className="w-full">
                <thead>
                  <tr className="border-b border-border">
                    <th className="text-left text-text-muted text-xs font-medium uppercase tracking-wider px-6 py-3">ID</th>
                    <th className="text-left text-text-muted text-xs font-medium uppercase tracking-wider px-6 py-3">Email</th>
                    <th className="text-left text-text-muted text-xs font-medium uppercase tracking-wider px-6 py-3">Role</th>
                    <th className="text-left text-text-muted text-xs font-medium uppercase tracking-wider px-6 py-3">Created</th>
                  </tr>
                </thead>
                <tbody>
                  {admins.length === 0 ? (
                    <tr><td colSpan={4} className="px-6 py-8 text-center text-text-muted text-sm">No admins found</td></tr>
                  ) : admins.map((a) => (
                    <tr key={a.id} className="border-b border-border/50">
                      <td className="px-6 py-3 text-text-secondary text-sm font-mono">{a.id.slice(0, 12)}...</td>
                      <td className="px-6 py-3 text-text-primary text-sm">{a.email}</td>
                      <td className="px-6 py-3 text-text-secondary text-sm capitalize">{a.role}</td>
                      <td className="px-6 py-3 text-text-secondary text-sm">{formatDate(a.created_at)}</td>
                    </tr>
                  ))}
                </tbody>
              </table>
            </div>
          </div>
        );

      case 'security':
        return (
          <div className="space-y-4 max-w-md">
            {sel('env', 'Environment', ['development', 'production'])}
            {f('jwt_secret', 'JWT Secret', { type: 'password', ph: 'Leave blank to use current' })}
            {f('cert_file', 'TLS Cert File Path')}
            {f('key_file', 'TLS Key File Path')}
            {f('allow_origins', 'Allowed Origins (CORS)', { ph: '*' })}
            <div className="border-t border-border pt-4">
              <h4 className="text-text-primary text-sm font-semibold mb-3">Rate Limiting</h4>
              {f('rate_limit_per_minute', 'Requests per Minute', { type: 'number', ph: '100' })}
              {f('rate_limit_burst', 'Burst Size', { type: 'number', ph: '20' })}
            </div>
          </div>
        );

      case 'captcha':
        return (
          <div className="space-y-4 max-w-md">
            <p className="text-text-muted text-sm">Protect sign-up and sign-in endpoints with a CAPTCHA challenge.</p>
            {f('captcha_enabled', 'Enable Captcha', { toggle: true })}
            {sel('captcha_provider', 'Provider', ['', 'hcaptcha', 'turnstile'])}
            {f('captcha_site_key', 'Site Key', { ph: 'your-site-key' })}
            {f('captcha_secret', 'Secret Key', { type: 'password', ph: 'Leave blank to keep current' })}
          </div>
        );

      case 's3':
        return (
          <div className="space-y-4 max-w-md">
            {f('s3_enabled', 'Enable S3 Storage', { toggle: true })}
            {f('s3_endpoint', 'Endpoint', { ph: 'https://s3.amazonaws.com' })}
            {f('s3_bucket_prefix', 'Bucket Prefix', { ph: 'sovrabase-' })}
            {f('s3_access_key', 'Access Key')}
            {f('s3_secret_key', 'Secret Key', { type: 'password', ph: 'Leave blank to keep current' })}
            {f('s3_use_ssl', 'Use SSL', { toggle: true })}
          </div>
        );

      case 'smtp':
        return (
          <div className="space-y-4 max-w-md">
            {f('email_verification', 'Enable Email Verification', { toggle: true })}
            {f('smtp_host', 'SMTP Host')}
            {f('smtp_port', 'Port', { type: 'number' })}
            {f('smtp_sender', 'Sender Email', { type: 'email' })}
            {f('smtp_user', 'Username')}
            {f('smtp_password', 'Password', { type: 'password', ph: 'Leave blank to keep current' })}
          </div>
        );

      case 'replication':
        return (
          <div className="space-y-4 max-w-md">
            {sel('role', 'Node Role', ['Standalone', 'Master', 'Heir', 'Reader'])}
            {f('node_id', 'Node ID')}
            {f('repl_addr', 'Replication Listen Address', { ph: '0.0.0.0:9000' })}
            <div className="space-y-1">
              <label className={labelCls}>Peers</label>
              <textarea
                value={(form.peers as string) ?? ''}
                onChange={(e) => setForm((fv) => ({ ...fv, peers: e.target.value }))}
                className={inputCls + ' h-24 resize-y'}
                placeholder="One peer per line"
              />
            </div>
          </div>
        );

      case 'audit': {
        const pg = auditPage();
        return (
          <div className="space-y-4">
            <div className="flex flex-wrap items-center gap-3">
              <input
                type="text" placeholder="Filter by Action..." value={auditAction}
                onChange={(e) => setAuditAction(e.target.value)}
                className={inputCls + ' max-w-[180px]'}
              />
              <input
                type="text" placeholder="Filter by Target..." value={auditTarget}
                onChange={(e) => setAuditTarget(e.target.value)}
                className={inputCls + ' max-w-[180px]'}
              />
              <button onClick={() => loadAudit(0, auditAction, auditTarget)} className="px-3 py-2 rounded-lg bg-accent text-white text-sm">Apply</button>
              <button onClick={() => { setAuditAction(''); setAuditTarget(''); loadAudit(0, '', ''); }} className="px-3 py-2 rounded-lg text-text-secondary text-sm hover:bg-bg-input">Clear</button>
              <div className="flex-1" />
              <button onClick={clearAudit} className="px-3 py-2 rounded-lg text-danger text-sm hover:bg-danger/10">Clear Logs</button>
              <button onClick={() => loadAudit(auditOffset, auditAction, auditTarget)} className="p-2 rounded-lg text-text-secondary hover:bg-bg-input"><RefreshCw className="w-4 h-4" /></button>
            </div>
            {loading ? <div className="flex justify-center py-12"><Loader2 className="w-5 h-5 text-accent animate-spin" /></div> : audit.length === 0 ? (
              <p className="text-text-muted text-sm py-8 text-center">No audit entries</p>
            ) : (
              <>
                <div className="bg-bg-card border border-border rounded-xl overflow-hidden">
                  <table className="w-full">
                    <thead>
                      <tr className="border-b border-border">
                        {['Timestamp', 'Admin', 'Action', 'Target', 'Details'].map((h) => (
                          <th key={h} className="text-left text-text-muted text-xs font-medium uppercase tracking-wider px-4 py-3">{h}</th>
                        ))}
                      </tr>
                    </thead>
                    <tbody>
                      {audit.map((e, i) => (
                        <tr key={i} className="border-b border-border/50">
                          <td className="px-4 py-3 text-text-secondary text-sm whitespace-nowrap">{formatDate(e.timestamp)}</td>
                          <td className="px-4 py-3 text-text-primary text-sm">{e.admin || '—'}</td>
                          <td className="px-4 py-3 text-text-primary text-sm">{e.action || '—'}</td>
                          <td className="px-4 py-3 text-text-secondary text-sm font-mono text-xs">{e.target || '—'}</td>
                          <td className="px-4 py-3 text-text-secondary text-sm max-w-xs truncate">{e.details || '—'}</td>
                        </tr>
                      ))}
                    </tbody>
                  </table>
                </div>
                <div className="flex items-center justify-between text-sm text-text-muted">
                  <span>{pg.total > 0 ? `${pg.start}-${pg.end} of ${pg.total}` : '0 results'}</span>
                  <div className="flex gap-2">
                    <button disabled={auditOffset === 0} onClick={() => loadAudit(Math.max(0, auditOffset - auditLimit), auditAction, auditTarget)} className="px-3 py-1.5 rounded-lg border border-border hover:bg-bg-input disabled:opacity-40 transition-colors">Previous</button>
                    <button disabled={auditOffset + auditLimit >= auditTotal} onClick={() => loadAudit(auditOffset + auditLimit, auditAction, auditTarget)} className="px-3 py-1.5 rounded-lg border border-border hover:bg-bg-input disabled:opacity-40 transition-colors">Next</button>
                  </div>
                </div>
              </>
            )}
          </div>
        );
      }

      case 'backups':
        return (
          <div className="space-y-4">
            <button onClick={createBackup} className="flex items-center gap-2 px-4 py-2 rounded-lg bg-accent text-white text-sm font-medium hover:bg-accent-hover">
              <Plus className="w-4 h-4" />Create Backup
            </button>
            {backups.length === 0 ? (
              <p className="text-text-muted text-sm py-8 text-center">No backups yet</p>
            ) : (
              <div className="grid gap-3">
                {backups.map((b) => (
                  <div key={b.name} className="flex items-center justify-between bg-bg-input border border-border rounded-lg px-5 py-3">
                    <div>
                      <p className="text-text-primary text-sm font-mono">{b.name}</p>
                      <p className="text-text-muted text-xs mt-0.5">
                        {formatDate(b.modified)}{b.size != null ? ` · ${formatBytes(b.size)}` : ''}
                      </p>
                    </div>
                    <div className="flex items-center gap-2 ml-auto">
                      <button
                        onClick={() => { window.open(`/admin/backups/${encodeURIComponent(b.name)}/download`, '_blank'); }}
                        className="px-3 py-1.5 rounded-lg border border-border text-text-secondary text-xs hover:text-text-primary hover:bg-bg-card transition-colors"
                      >Download</button>
                      <button
                        onClick={async () => { if (!confirm(`Delete backup "${b.name}"?`)) return; try { await api(`/admin/backups/${encodeURIComponent(b.name)}`, { method: 'DELETE' }); showToast('Backup deleted', 'success'); loadBackups(); } catch (e) { showToast((e as Error).message, 'error'); } }}
                        className="flex items-center gap-1 px-3 py-1.5 rounded-lg text-danger text-xs hover:bg-danger/10 transition-colors"
                      ><Trash2 className="w-3.5 h-3.5" /> Delete</button>
                    </div>
                  </div>
                ))}
              </div>
            )}
          </div>
        );

      default: return null;
    }
  };

  // ----- system info labels -----
  const infoLabels: Record<string, string> = {
    version: 'Version', go_version: 'Go Version', listen_addr: 'Listen Address',
    data_dir: 'Data Directory', storage_driver: 'Storage Driver',
    replication_role: 'Replication Role', region: 'Region',
  };

  return (
    <div className="space-y-6">
      <h1 className="text-2xl font-bold text-text-primary">Settings</h1>

      {/* System Information */}
      {systemInfo && Object.keys(systemInfo).length > 0 && (
        <div className="bg-bg-card border border-border rounded-xl p-6">
          <h2 className="text-lg font-semibold text-text-primary mb-4">System Information</h2>
          <div className="grid grid-cols-1 sm:grid-cols-2 lg:grid-cols-3 gap-x-8 gap-y-2">
            {Object.entries(systemInfo).map(([k, v]) => (
              <div key={k} className="flex justify-between py-1.5 border-b border-border/30 last:border-0">
                <span className="text-text-muted text-sm capitalize">{infoLabels[k] || k.replace(/_/g, ' ')}</span>
                <span className="text-text-primary text-sm font-medium">{v}</span>
              </div>
            ))}
          </div>
        </div>
      )}

      {/* Tab bar */}
      <div className="flex gap-1 overflow-x-auto overflow-y-hidden pb-1 tabbar-scroll">
        {tabs.map((t) => {
          const Icon = t.icon;
          const active = tab === t.key;
          return (
            <button
              key={t.key}
              onClick={() => setTab(t.key)}
              className={`flex items-center gap-2 px-4 py-2.5 rounded-lg text-sm font-medium whitespace-nowrap transition-colors ${
                active ? 'bg-accent text-white' : 'text-text-secondary hover:text-text-primary hover:bg-bg-card'
              }`}
            >
              <Icon className="w-4 h-4" />{t.label}
            </button>
          );
        })}
      </div>

      {/* Tab content */}
      <div className="bg-bg-card border border-border rounded-xl p-6">{renderContent()}</div>

      {/* Apply Changes */}
      <div className="bg-bg-card border border-border rounded-xl p-6">
        <div className="flex flex-wrap items-center justify-between gap-4">
          <div>
            <h3 className="text-text-primary font-semibold">Apply Changes</h3>
            <p className="text-text-muted text-sm">Config is saved to config.yaml</p>
          </div>
          <div className="flex items-center gap-3">
            <button onClick={() => { loadConfig(); setConfirmPw(''); }} className="px-4 py-2 rounded-lg border border-border text-text-secondary text-sm hover:bg-bg-input transition-colors">Reset</button>
            <button onClick={saveConfig} disabled={saving} className="flex items-center gap-2 px-4 py-2 rounded-lg bg-accent text-white text-sm font-medium hover:bg-accent-hover disabled:opacity-50 transition-colors">
              <Save className="w-4 h-4" />{saving ? 'Saving...' : 'Save Config'}
            </button>
            <button onClick={handleRestart} disabled={saving} className="flex items-center gap-2 px-4 py-2 rounded-lg bg-warning text-white text-sm font-medium hover:bg-yellow-600 disabled:opacity-50 transition-colors">
              <Zap className="w-4 h-4" />Save &amp; Restart
            </button>
          </div>
        </div>
      </div>

      {/* Add Admin Modal */}
      <Modal isOpen={showAddAdmin} onClose={() => setShowAddAdmin(false)} title="Add Admin">
        <div className="space-y-4">
          <div>
            <label className={labelCls}>Email</label>
            <input type="email" value={newAdmin.email} onChange={(e) => setNewAdmin((a) => ({ ...a, email: e.target.value }))} className="w-full mt-1 px-3 py-2 rounded-lg bg-bg-input border border-border text-text-primary focus:outline-none focus:border-accent text-sm" placeholder="admin@example.com" />
          </div>
          <div>
            <label className={labelCls}>Password</label>
            <input type="password" value={newAdmin.password} onChange={(e) => setNewAdmin((a) => ({ ...a, password: e.target.value }))} className="w-full mt-1 px-3 py-2 rounded-lg bg-bg-input border border-border text-text-primary focus:outline-none focus:border-accent text-sm" />
          </div>
          <div className="flex justify-end gap-3">
            <button onClick={() => setShowAddAdmin(false)} className="px-4 py-2 rounded-lg text-text-secondary text-sm hover:bg-bg-input transition-colors">Cancel</button>
            <button onClick={handleAddAdmin} className="px-4 py-2 rounded-lg bg-accent text-white text-sm font-medium hover:bg-accent-hover transition-colors">Add</button>
          </div>
        </div>
      </Modal>

      {/* Restart Confirm Modal */}
      <Modal isOpen={showRestartConfirm} onClose={() => setShowRestartConfirm(false)} title="Restart Server" size="sm">
        <div className="space-y-4">
          <p className="text-text-secondary text-sm">Restart the server? This will briefly interrupt service.</p>
          <div className="flex justify-end gap-3">
            <button onClick={() => setShowRestartConfirm(false)} className="px-4 py-2 rounded-lg text-text-secondary text-sm hover:bg-bg-input transition-colors">Cancel</button>
            <button onClick={confirmRestart} className="px-4 py-2 rounded-lg bg-warning text-white text-sm font-medium hover:bg-yellow-600 transition-colors">Restart</button>
          </div>
        </div>
      </Modal>

      {/* Danger Zone */}
      <div className="mt-8 p-6 bg-danger/5 border border-danger/20 rounded-xl">
        <h3 className="text-base font-bold text-danger mb-2">⚠ Danger Zone</h3>
        <p className="text-text-muted text-sm mb-4">Irreversible actions. Please proceed with caution.</p>
        <button disabled className="px-4 py-2 rounded-lg bg-danger/30 text-danger/60 text-sm font-medium cursor-not-allowed">
          Reset All Data (coming soon)
        </button>
      </div>
    </div>
  );
}
