import { useEffect, useState } from 'react';
import { Shield, Users, Cloud, Mail, GitBranch, ScrollText, Database as Database2, Loader2, Check, Plus, RefreshCw } from 'lucide-react';
import { api, formatDate } from '../api';
import { useToast } from '../components/Toast';
import Modal from '../components/Modal';
import type { AdminUser, Backup, LogEntry } from '../types';

type TabKey = 'account' | 'admins' | 'security' | 's3' | 'smtp' | 'replication' | 'audit' | 'backups';

interface TabDef { key: TabKey; label: string; icon: typeof Shield; }

const tabs: TabDef[] = [
  { key: 'account', label: 'Admin Account', icon: Shield },
  { key: 'admins', label: 'Admins', icon: Users },
  { key: 'security', label: 'Security & HTTPS', icon: Shield },
  { key: 's3', label: 'S3 Storage', icon: Cloud },
  { key: 'smtp', label: 'SMTP', icon: Mail },
  { key: 'replication', label: 'Replication', icon: GitBranch },
  { key: 'audit', label: 'Audit Log', icon: ScrollText },
  { key: 'backups', label: 'Backups', icon: Database2 },
];

const inputCls = 'w-full px-3 py-2 rounded-lg bg-bg-input border border-border text-text-primary placeholder:text-text-muted focus:outline-none focus:border-accent transition-colors text-sm';

export default function Settings() {
  const { showToast } = useToast();
  const [tab, setTab] = useState<TabKey>('account');
  const [form, setForm] = useState<Record<string, string | boolean>>({});
  const [loading, setLoading] = useState(false);
  const [saving, setSaving] = useState(false);
  const [admins, setAdmins] = useState<AdminUser[]>([]);
  const [backups, setBackups] = useState<Backup[]>([]);
  const [audit, setAudit] = useState<LogEntry[]>([]);
  const [showAddAdmin, setShowAddAdmin] = useState(false);
  const [newAdmin, setNewAdmin] = useState({ email: '', password: '' });

  const loadTab = async (t: TabKey) => {
    setLoading(true);
    try {
      if (t === 'admins') {
        const d = await api<{ admins: AdminUser[] }>('/admin/admins');
        setAdmins(d.admins || []);
      } else if (t === 'audit') {
        const d = await api<{ entries: LogEntry[] }>('/admin/audit');
        setAudit(d.entries || []);
      } else if (t === 'backups') {
        const d = await api<{ backups: Backup[] }>('/admin/backups');
        setBackups(d.backups || []);
      } else {
        const data = await api<Record<string, unknown>>('/admin/config');
        const flat: Record<string, string | boolean> = {};
        for (const [k, v] of Object.entries(data)) {
          if (typeof v === 'boolean' || typeof v === 'string') flat[k] = v;
          else if (Array.isArray(v)) flat[k] = v.join(', ');
        }
        setForm(flat);
      }
    } catch {
      // empty tab — no config yet
    } finally {
      setLoading(false);
    }
  };

  useEffect(() => { loadTab(tab); }, [tab]);

  const handleSave = async () => {
    setSaving(true);
    try {
      const body: Record<string, unknown> = { ...form };
      if (body.peers && typeof body.peers === 'string') {
        body.peers = (body.peers as string).split(',').map((s: string) => s.trim()).filter(Boolean);
      }
      await api('/admin/config', { method: 'PATCH', body: JSON.stringify(body) });
      showToast('Settings saved', 'success');
    } catch (err) {
      showToast((err as Error).message, 'error');
    } finally {
      setSaving(false);
    }
  };

  const handleAddAdmin = async () => {
    try {
      await api('/admin/admins', { method: 'POST', body: JSON.stringify(newAdmin) });
      showToast('Admin added', 'success');
      setShowAddAdmin(false);
      setNewAdmin({ email: '', password: '' });
      loadTab('admins');
    } catch (err) {
      showToast((err as Error).message, 'error');
    }
  };

  const createBackup = async () => {
    try {
      await api('/admin/backups', { method: 'POST' });
      showToast('Backup created', 'success');
      loadTab('backups');
    } catch (err) {
      showToast((err as Error).message, 'error');
    }
  };

  const restoreBackup = async (backupId: string) => {
    try {
      await api(`/admin/backups/${encodeURIComponent(backupId)}/restore`, { method: 'POST' });
      showToast('Restore initiated', 'success');
    } catch (err) {
      showToast((err as Error).message, 'error');
    }
  };

  const renderField = (key: string, label: string, opts?: { type?: string; placeholder?: string; toggle?: boolean }) => {
    if (opts?.toggle) {
      return (
        <div key={key} className="flex items-center justify-between py-2">
          <span className="text-text-secondary text-sm">{label}</span>
          <button
            onClick={() => setForm((fv) => ({ ...fv, [key]: !fv[key] }))}
            className={`w-10 h-5 rounded-full transition-colors ${form[key] ? 'bg-accent' : 'bg-bg-input border border-border'}`}
          >
            <span className={`block w-4 h-4 rounded-full bg-white transition-transform ${form[key] ? 'translate-x-5' : 'translate-x-0.5'}`} />
          </button>
        </div>
      );
    }
    return (
      <div key={key} className="space-y-1">
        <label className="text-text-secondary text-sm font-medium">{label}</label>
        <input
          type={opts?.type || 'text'}
          value={(form[key] as string) ?? ''}
          onChange={(e) => setForm((fv) => ({ ...fv, [key]: e.target.value }))}
          className={inputCls}
          placeholder={opts?.placeholder}
        />
      </div>
    );
  };

  const renderContent = () => {
    if (loading) {
      return <div className="flex justify-center py-16"><Loader2 className="w-6 h-6 text-accent animate-spin" /></div>;
    }

    switch (tab) {
      case 'account':
        return (
          <div className="space-y-4 max-w-md">
            {renderField('admin_email', 'Email', { type: 'email' })}
            {renderField('admin_password', 'New Password', { type: 'password', placeholder: 'Leave blank to keep current' })}
            <button onClick={handleSave} disabled={saving} className="flex items-center gap-2 px-4 py-2 rounded-lg bg-accent text-white text-sm font-medium hover:bg-accent-hover disabled:opacity-50 transition-colors">
              <Check className="w-4 h-4" />{saving ? 'Saving...' : 'Save'}
            </button>
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
            {renderField('jwt_secret', 'JWT Secret', { type: 'password', placeholder: 'Leave blank to use current' })}
            {renderField('session_duration', 'Session Duration', { placeholder: '24h' })}
            {renderField('allow_origins', 'Allowed Origins', { placeholder: '*' })}
            {renderField('cert_file', 'TLS Cert File')}
            {renderField('key_file', 'TLS Key File')}
            <button onClick={handleSave} disabled={saving} className="flex items-center gap-2 px-4 py-2 rounded-lg bg-accent text-white text-sm font-medium hover:bg-accent-hover disabled:opacity-50 transition-colors">
              <Check className="w-4 h-4" />{saving ? 'Saving...' : 'Save'}
            </button>
          </div>
        );

      case 's3':
        return (
          <div className="space-y-4 max-w-md">
            {renderField('s3_enabled', 'Enable S3 Storage', { toggle: true })}
            {renderField('s3_endpoint', 'Endpoint', { placeholder: 'https://s3.amazonaws.com' })}
            {renderField('s3_access_key', 'Access Key')}
            {renderField('s3_secret_key', 'Secret Key', { type: 'password', placeholder: 'Leave blank to keep current' })}
            {renderField('s3_bucket_prefix', 'Bucket Prefix', { placeholder: 'sovrabase-' })}
            {renderField('s3_use_ssl', 'Use SSL', { toggle: true })}
            <button onClick={handleSave} disabled={saving} className="flex items-center gap-2 px-4 py-2 rounded-lg bg-accent text-white text-sm font-medium hover:bg-accent-hover disabled:opacity-50 transition-colors">
              <Check className="w-4 h-4" />{saving ? 'Saving...' : 'Save'}
            </button>
          </div>
        );

      case 'smtp':
        return (
          <div className="space-y-4 max-w-md">
            {renderField('smtp_host', 'Host')}
            {renderField('smtp_port', 'Port', { type: 'number' })}
            {renderField('smtp_user', 'Username')}
            {renderField('smtp_password', 'Password', { type: 'password', placeholder: 'Leave blank to keep current' })}
            {renderField('smtp_sender', 'Sender Address', { type: 'email' })}
            <button onClick={handleSave} disabled={saving} className="flex items-center gap-2 px-4 py-2 rounded-lg bg-accent text-white text-sm font-medium hover:bg-accent-hover disabled:opacity-50 transition-colors">
              <Check className="w-4 h-4" />{saving ? 'Saving...' : 'Save'}
            </button>
          </div>
        );

      case 'replication':
        return (
          <div className="space-y-4 max-w-md">
            {renderField('role', 'Role', { placeholder: 'primary / secondary' })}
            {renderField('node_id', 'Node ID')}
            {renderField('repl_addr', 'Replication Address', { placeholder: '0.0.0.0:9000' })}
            {renderField('peers', 'Peers', { placeholder: 'node1:9000, node2:9000' })}
            {renderField('lease_ttl', 'Lease TTL', { placeholder: '10s' })}
            <button onClick={handleSave} disabled={saving} className="flex items-center gap-2 px-4 py-2 rounded-lg bg-accent text-white text-sm font-medium hover:bg-accent-hover disabled:opacity-50 transition-colors">
              <Check className="w-4 h-4" />{saving ? 'Saving...' : 'Save'}
            </button>
          </div>
        );

      case 'audit':
        return (
          <div className="bg-bg-card border border-border rounded-xl overflow-hidden">
            <table className="w-full">
              <thead>
                <tr className="border-b border-border">
                  {['Timestamp', 'Level', 'Message', 'Source'].map((h) => (
                    <th key={h} className="text-left text-text-muted text-xs font-medium uppercase tracking-wider px-6 py-3">{h}</th>
                  ))}
                </tr>
              </thead>
              <tbody>
                {audit.length === 0 ? (
                  <tr><td colSpan={4} className="px-6 py-8 text-center text-text-muted text-sm">No audit entries</td></tr>
                ) : audit.map((e, i) => (
                  <tr key={i} className="border-b border-border/50">
                    <td className="px-6 py-3 text-text-secondary text-sm whitespace-nowrap">{formatDate(e.timestamp)}</td>
                    <td className="px-6 py-3">
                      <span className={`text-xs font-medium px-2 py-0.5 rounded ${
                        e.level === 'error' ? 'bg-danger/15 text-danger' :
                        e.level === 'warn' ? 'bg-yellow-500/15 text-yellow-500' :
                        'bg-accent/15 text-accent'
                      }`}>{e.level}</span>
                    </td>
                    <td className="px-6 py-3 text-text-primary text-sm max-w-md truncate">{e.message}</td>
                    <td className="px-6 py-3 text-text-secondary text-sm">{e.source || '—'}</td>
                  </tr>
                ))}
              </tbody>
            </table>
          </div>
        );

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
                  <div key={b.id} className="flex items-center justify-between bg-bg-input border border-border rounded-lg px-5 py-3">
                    <div>
                      <p className="text-text-primary text-sm font-mono">{b.id}</p>
                      <p className="text-text-muted text-xs mt-0.5">
                        {formatDate(b.created_at)}{b.size != null ? ` · ${b.size} bytes` : ''}
                      </p>
                    </div>
                    <button
                      onClick={() => restoreBackup(b.id)}
                      className="flex items-center gap-1.5 px-3 py-1.5 rounded-lg text-accent text-sm font-medium hover:bg-accent/10"
                    >
                      <RefreshCw className="w-3.5 h-3.5" />Restore
                    </button>
                  </div>
                ))}
              </div>
            )}
          </div>
        );

      default:
        return null;
    }
  };

  return (
    <div className="space-y-6">
      <h1 className="text-2xl font-bold text-text-primary">Settings</h1>

      {/* Tab bar */}
      <div className="flex gap-1 overflow-x-auto pb-1">
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

      {/* Content */}
      <div className="bg-bg-card border border-border rounded-xl p-6">
        {renderContent()}
      </div>

      {/* Add Admin Modal */}
      <Modal isOpen={showAddAdmin} onClose={() => setShowAddAdmin(false)} title="Add Admin">
        <div className="space-y-4">
          <div>
            <label className="text-text-secondary text-sm font-medium">Email</label>
            <input
              type="email"
              value={newAdmin.email}
              onChange={(e) => setNewAdmin((a) => ({ ...a, email: e.target.value }))}
              className="w-full mt-1 px-3 py-2 rounded-lg bg-bg-input border border-border text-text-primary focus:outline-none focus:border-accent text-sm"
              placeholder="admin@example.com"
            />
          </div>
          <div>
            <label className="text-text-secondary text-sm font-medium">Password</label>
            <input
              type="password"
              value={newAdmin.password}
              onChange={(e) => setNewAdmin((a) => ({ ...a, password: e.target.value }))}
              className="w-full mt-1 px-3 py-2 rounded-lg bg-bg-input border border-border text-text-primary focus:outline-none focus:border-accent text-sm"
            />
          </div>
          <div className="flex justify-end gap-3">
            <button onClick={() => setShowAddAdmin(false)} className="px-4 py-2 rounded-lg text-text-secondary text-sm hover:bg-bg-input transition-colors">Cancel</button>
            <button onClick={handleAddAdmin} className="px-4 py-2 rounded-lg bg-accent text-white text-sm font-medium hover:bg-accent-hover transition-colors">Add</button>
          </div>
        </div>
      </Modal>
    </div>
  );
}
