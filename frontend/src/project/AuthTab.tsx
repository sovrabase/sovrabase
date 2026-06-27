import { useState, useEffect } from 'react';
import { Plus, Trash2, Search, Users, Shield, Mail, Key, Loader2, Check, X, Globe, Zap } from 'lucide-react';
import { api, formatDate } from '../api';
import Modal from '../components/Modal';
import { useToast } from '../components/Toast';
import type { OAuthProvider } from '../types';

interface UserRow { id: string; username?: string; email: string; name?: string; avatar_url?: string; role?: string; created_at?: string; }
interface Props { projectId: string; }
const ROLES = ['user', 'admin', 'moderator'];

const emptyUser = { username: '', email: '', password: '', role: 'user', name: '', avatar_url: '' };
const emptyProvider = { name: '', enabled: false, client_id: '', client_secret: '' };

export default function AuthTab({ projectId }: Props) {
  const { showToast } = useToast();
  const [users, setUsers] = useState<UserRow[]>([]);
  const [providers, setProviders] = useState<OAuthProvider[]>([]);
  const [loading, setLoading] = useState(true);
  const [search, setSearch] = useState('');
  const [showAddUser, setShowAddUser] = useState(false);
  const [editUser, setEditUser] = useState<UserRow | null>(null);
  const [uf, setUf] = useState(emptyUser);
  const [savingU, setSavingU] = useState(false);
  const [showAddProv, setShowAddProv] = useState(false);
  const [pf, setPf] = useState(emptyProvider);
  const [savingP, setSavingP] = useState(false);

  useEffect(() => {
    setLoading(true);
    Promise.all([
      api<{ users: UserRow[] }>(`/admin/projects/${encodeURIComponent(projectId)}/users`).then((d) => setUsers(d.users || [])).catch(() => {}),
      api<{ providers: OAuthProvider[] }>(`/admin/projects/${encodeURIComponent(projectId)}/oauth-providers`).then((d) => setProviders(d.providers || [])).catch(() => {}),
    ]).finally(() => setLoading(false));
  }, [projectId]);

  const filtered = users.filter((u) => (u.email || '').toLowerCase().includes(search.toLowerCase()) || (u.username || '').toLowerCase().includes(search.toLowerCase()) || (u.name || '').toLowerCase().includes(search.toLowerCase()));

  const refreshUsers = async () => { const d = await api<{ users: UserRow[] }>(`/admin/projects/${encodeURIComponent(projectId)}/users`); setUsers(d.users || []); };
  const refreshProviders = async () => { const d = await api<{ providers: OAuthProvider[] }>(`/admin/projects/${encodeURIComponent(projectId)}/oauth-providers`); setProviders(d.providers || []); };

  const addUser = async () => {
    if (!uf.username.trim() || !uf.email.trim() || !uf.password.trim()) return;
    setSavingU(true);
    try { await api(`/admin/projects/${encodeURIComponent(projectId)}/users`, { method: 'POST', body: JSON.stringify(uf) }); showToast('User added', 'success'); setShowAddUser(false); refreshUsers(); } catch (e: unknown) { showToast((e as Error).message || 'Failed', 'error'); }
    setSavingU(false);
  };

  const updateUser = async () => {
    if (!editUser) return;
    setSavingU(true);
    try { await api(`/admin/projects/${encodeURIComponent(projectId)}/users/${encodeURIComponent(editUser.id)}`, { method: 'PUT', body: JSON.stringify({ username: uf.username, email: uf.email, name: uf.name, avatar_url: uf.avatar_url, role: uf.role }) }); showToast('User updated', 'success'); setEditUser(null); refreshUsers(); } catch (e: unknown) { showToast((e as Error).message || 'Failed', 'error'); }
    setSavingU(false);
  };

  const deleteUser = async (id: string) => {
    if (!confirm('Delete user?')) return;
    try { await api(`/admin/projects/${encodeURIComponent(projectId)}/users/${encodeURIComponent(id)}`, { method: 'DELETE' }); showToast('User deleted', 'success'); setUsers((p) => p.filter((u) => u.id !== id)); } catch (e: unknown) { showToast((e as Error).message || 'Failed', 'error'); }
  };

  const addProvider = async () => {
    if (!pf.name.trim()) return;
    setSavingP(true);
    try { await api(`/admin/projects/${encodeURIComponent(projectId)}/oauth-providers`, { method: 'POST', body: JSON.stringify(pf) }); showToast('Provider added', 'success'); setShowAddProv(false); refreshProviders(); } catch (e: unknown) { showToast((e as Error).message || 'Failed', 'error'); }
    setSavingP(false);
  };

  const openAdd = () => { setUf(emptyUser); setShowAddUser(true); };
  const openEdit = (u: UserRow) => { setEditUser(u); setUf({ username: u.username || '', email: u.email || '', password: '', role: u.role || 'user', name: u.name || '', avatar_url: u.avatar_url || '' }); };
  const openProvider = () => { setPf(emptyProvider); setShowAddProv(true); };

  if (loading) return <div className="flex items-center gap-2 py-10 text-text-muted"><Loader2 className="w-4 h-4 animate-spin" /> Loading auth data...</div>;

  return (
    <div className="space-y-8">
      {/* Users */}
      <section>
        <div className="flex items-center justify-between mb-4">
          <h2 className="text-lg font-semibold text-text-primary flex items-center gap-2"><Users className="w-5 h-5 text-accent" /> Users ({users.length})</h2>
          <div className="flex items-center gap-3">
            <div className="relative"><Search className="absolute left-3 top-1/2 -translate-y-1/2 w-4 h-4 text-text-muted" /><input type="text" placeholder="Search users..." value={search} onChange={(e) => setSearch(e.target.value)} className="bg-bg-input border border-border rounded-md pl-9 pr-3 py-2 text-text-primary text-sm placeholder:text-text-muted focus:outline-none focus:border-accent w-56" /></div>
            <button onClick={openAdd} className="flex items-center gap-1 px-3 py-2 bg-accent text-white rounded-lg text-sm font-medium hover:opacity-90"><Plus className="w-4 h-4" /> Add User</button>
          </div>
        </div>
        {filtered.length === 0 ? (
          <div className="flex flex-col items-center py-16 text-text-muted gap-3"><Users className="w-10 h-10" /><p>{users.length === 0 ? 'No users' : 'No users match search'}</p></div>
        ) : (
          <div className="border border-border rounded-lg overflow-hidden">
            <table className="w-full text-sm">
              <thead className="bg-bg-input text-text-muted text-xs uppercase"><tr><th className="text-left px-4 py-3 font-medium">ID</th><th className="text-left px-4 py-3 font-medium">Username</th><th className="text-left px-4 py-3 font-medium">Email</th><th className="text-left px-4 py-3 font-medium">Role</th><th className="text-left px-4 py-3 font-medium">Created</th><th className="text-right px-4 py-3 font-medium">Actions</th></tr></thead>
              <tbody className="divide-y divide-border">
                {filtered.map((u) => (
                  <tr key={u.id} className="hover:bg-bg-input/50 transition-colors">
                    <td className="px-4 py-3 font-mono text-text-primary text-xs">{u.id.slice(0, 8)}...</td>
                    <td className="px-4 py-3 text-text-primary text-sm">{u.username || '—'}</td>
                    <td className="px-4 py-3 text-text-secondary text-sm">{u.email}</td>
                    <td className="px-4 py-3"><span className="inline-flex px-2 py-0.5 bg-accent/10 text-accent rounded-full text-xs font-medium">{u.role || 'user'}</span></td>
                    <td className="px-4 py-3 text-text-secondary text-xs">{formatDate(u.created_at)}</td>
                    <td className="px-4 py-3 text-right"><div className="flex items-center justify-end gap-2"><button onClick={() => openEdit(u)} className="px-2 py-1 bg-bg-input border border-border rounded text-text-secondary text-xs hover:text-text-primary">Edit</button><button onClick={() => deleteUser(u.id)} className="px-2 py-1 bg-bg-input border border-border rounded text-text-muted text-xs hover:text-danger"><Trash2 className="w-3.5 h-3.5" /></button></div></td>
                  </tr>
                ))}
              </tbody>
            </table>
          </div>
        )}
      </section>

      {/* OAuth Providers */}
      <section>
        <div className="flex items-center justify-between mb-4">
          <h2 className="text-lg font-semibold text-text-primary flex items-center gap-2"><Shield className="w-5 h-5 text-accent" /> OAuth Providers</h2>
          <button onClick={openProvider} className="flex items-center gap-1 px-3 py-2 bg-accent text-white rounded-lg text-sm font-medium hover:opacity-90"><Plus className="w-4 h-4" /> Add Provider</button>
        </div>
        {providers.length === 0 ? (
          <div className="flex flex-col items-center py-12 text-text-muted gap-3 bg-bg-card border border-border rounded-lg"><Globe className="w-8 h-8" /><p className="text-sm">No OAuth providers configured</p></div>
        ) : (
          <div className="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-3 gap-3">
            {providers.map((p) => (
              <div key={p.name} className="bg-bg-card border border-border rounded-lg p-4">
                <div className="flex items-center justify-between mb-2"><h3 className="text-text-primary font-medium text-sm capitalize">{p.name}</h3><span className={`inline-flex items-center gap-1 px-2 py-0.5 rounded-full text-xs font-medium ${p.enabled ? 'bg-success/10 text-success' : 'bg-bg-input text-text-muted'}`}>{p.enabled ? <Check className="w-3 h-3" /> : <X className="w-3 h-3" />}{p.enabled ? 'Enabled' : 'Disabled'}</span></div>
                {p.client_id && <p className="text-text-muted text-xs font-mono truncate">Client: {p.client_id.slice(0, 24)}...</p>}
                {p.redirect_url && <p className="text-text-muted text-xs font-mono truncate mt-1">{p.redirect_url}</p>}
              </div>
            ))}
          </div>
        )}
      </section>

      {/* Auth Features */}
      <section>
        <h2 className="text-lg font-semibold text-text-primary flex items-center gap-2 mb-4"><Zap className="w-5 h-5 text-accent" /> Auth Features</h2>
        <div className="grid grid-cols-1 md:grid-cols-3 gap-4">
          {[{ icon: Mail, label: 'Magic Links', status: 'Available' }, { icon: Key, label: 'MFA (TOTP)', status: 'Available' }, { icon: Shield, label: 'Email Verification', status: 'Enabled' }].map((f) => (
            <div key={f.label} className="bg-bg-card border border-border rounded-lg p-4 flex items-center justify-between">
              <div className="flex items-center gap-2"><f.icon className="w-4 h-4 text-accent" /><span className="text-text-primary text-sm">{f.label}</span></div>
              <span className="inline-flex px-2 py-0.5 bg-success/10 text-success rounded-full text-xs font-medium">{f.status}</span>
            </div>
          ))}
        </div>
      </section>

      {/* Add User Modal */}
      <Modal isOpen={showAddUser} onClose={() => setShowAddUser(false)} title="Add User">
        <div className="space-y-4">
          <div><label className="block text-text-secondary text-sm mb-1">Username</label><input type="text" value={uf.username} onChange={(e) => setUf((f) => ({ ...f, username: e.target.value }))} className="w-full bg-bg-input border border-border rounded-md px-3 py-2 text-text-primary text-sm focus:outline-none focus:border-accent" autoFocus /></div>
          <div><label className="block text-text-secondary text-sm mb-1">Email</label><input type="email" value={uf.email} onChange={(e) => setUf((f) => ({ ...f, email: e.target.value }))} className="w-full bg-bg-input border border-border rounded-md px-3 py-2 text-text-primary text-sm focus:outline-none focus:border-accent" /></div>
          <div><label className="block text-text-secondary text-sm mb-1">Password</label><input type="password" value={uf.password} onChange={(e) => setUf((f) => ({ ...f, password: e.target.value }))} className="w-full bg-bg-input border border-border rounded-md px-3 py-2 text-text-primary text-sm focus:outline-none focus:border-accent" /></div>
          <div><label className="block text-text-secondary text-sm mb-1">Role</label><select value={uf.role} onChange={(e) => setUf((f) => ({ ...f, role: e.target.value }))} className="w-full bg-bg-input border border-border rounded-md px-3 py-2 text-text-primary text-sm focus:outline-none focus:border-accent">{ROLES.map((r) => <option key={r} value={r}>{r}</option>)}</select></div>
          <div className="flex justify-end gap-2 pt-2"><button onClick={() => setShowAddUser(false)} className="px-4 py-2 border border-border rounded-lg text-text-secondary text-sm hover:text-text-primary">Cancel</button><button onClick={addUser} disabled={savingU} className="flex items-center gap-1 px-4 py-2 bg-accent text-white rounded-lg text-sm font-medium hover:opacity-90 disabled:opacity-50">{savingU ? <Loader2 className="w-4 h-4 animate-spin" /> : <Plus className="w-4 h-4" />} Add</button></div>
        </div>
      </Modal>

      {/* Edit User Modal */}
      <Modal isOpen={!!editUser} onClose={() => setEditUser(null)} title="Edit User">
        <div className="space-y-4">
          <div><label className="block text-text-secondary text-sm mb-1">Username</label><input type="text" value={uf.username} onChange={(e) => setUf((f) => ({ ...f, username: e.target.value }))} className="w-full bg-bg-input border border-border rounded-md px-3 py-2 text-text-primary text-sm focus:outline-none focus:border-accent" /></div>
          <div><label className="block text-text-secondary text-sm mb-1">Email</label><input type="email" value={uf.email} onChange={(e) => setUf((f) => ({ ...f, email: e.target.value }))} className="w-full bg-bg-input border border-border rounded-md px-3 py-2 text-text-primary text-sm focus:outline-none focus:border-accent" /></div>
          <div><label className="block text-text-secondary text-sm mb-1">Name</label><input type="text" value={uf.name} onChange={(e) => setUf((f) => ({ ...f, name: e.target.value }))} className="w-full bg-bg-input border border-border rounded-md px-3 py-2 text-text-primary text-sm focus:outline-none focus:border-accent" /></div>
          <div><label className="block text-text-secondary text-sm mb-1">Avatar URL</label><input type="text" value={uf.avatar_url} onChange={(e) => setUf((f) => ({ ...f, avatar_url: e.target.value }))} className="w-full bg-bg-input border border-border rounded-md px-3 py-2 text-text-primary text-sm focus:outline-none focus:border-accent" /></div>
          <div><label className="block text-text-secondary text-sm mb-1">Role</label><select value={uf.role} onChange={(e) => setUf((f) => ({ ...f, role: e.target.value }))} className="w-full bg-bg-input border border-border rounded-md px-3 py-2 text-text-primary text-sm focus:outline-none focus:border-accent">{ROLES.map((r) => <option key={r} value={r}>{r}</option>)}</select></div>
          <div className="flex justify-end gap-2 pt-2"><button onClick={() => setEditUser(null)} className="px-4 py-2 border border-border rounded-lg text-text-secondary text-sm hover:text-text-primary">Cancel</button><button onClick={updateUser} disabled={savingU} className="flex items-center gap-1 px-4 py-2 bg-accent text-white rounded-lg text-sm font-medium hover:opacity-90 disabled:opacity-50">{savingU ? <Loader2 className="w-4 h-4 animate-spin" /> : <Check className="w-4 h-4" />} Save</button></div>
        </div>
      </Modal>

      {/* Add Provider Modal */}
      <Modal isOpen={showAddProv} onClose={() => setShowAddProv(false)} title="Add OAuth Provider">
        <div className="space-y-4">
          <div><label className="block text-text-secondary text-sm mb-1">Name</label><input type="text" value={pf.name} onChange={(e) => setPf((f) => ({ ...f, name: e.target.value }))} placeholder="e.g. google, github" className="w-full bg-bg-input border border-border rounded-md px-3 py-2 text-text-primary text-sm focus:outline-none focus:border-accent" autoFocus /></div>
          <div className="flex items-center gap-3"><label className="text-text-secondary text-sm">Enabled</label><input type="checkbox" checked={pf.enabled} onChange={(e) => setPf((f) => ({ ...f, enabled: e.target.checked }))} className="w-4 h-4 rounded border-border bg-bg-input accent-accent" /></div>
          <div><label className="block text-text-secondary text-sm mb-1">Client ID</label><input type="text" value={pf.client_id} onChange={(e) => setPf((f) => ({ ...f, client_id: e.target.value }))} className="w-full bg-bg-input border border-border rounded-md px-3 py-2 text-text-primary text-sm focus:outline-none focus:border-accent font-mono" /></div>
          <div><label className="block text-text-secondary text-sm mb-1">Client Secret</label><input type="password" value={pf.client_secret} onChange={(e) => setPf((f) => ({ ...f, client_secret: e.target.value }))} className="w-full bg-bg-input border border-border rounded-md px-3 py-2 text-text-primary text-sm focus:outline-none focus:border-accent font-mono" /></div>
          <div className="flex justify-end gap-2 pt-2"><button onClick={() => setShowAddProv(false)} className="px-4 py-2 border border-border rounded-lg text-text-secondary text-sm hover:text-text-primary">Cancel</button><button onClick={addProvider} disabled={savingP} className="flex items-center gap-1 px-4 py-2 bg-accent text-white rounded-lg text-sm font-medium hover:opacity-90 disabled:opacity-50">{savingP ? <Loader2 className="w-4 h-4 animate-spin" /> : <Plus className="w-4 h-4" />} Add</button></div>
        </div>
      </Modal>
    </div>
  );
}
