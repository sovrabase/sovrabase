import { useState, useEffect } from 'react';
import { Plus, Trash2, Search, Users, Shield, Mail, Key, Loader2, Check, Globe, Zap, Pencil, ChevronDown, ChevronUp, SlidersHorizontal, Eye, EyeOff } from 'lucide-react';
import { api, formatDate } from '../api';
import Modal from '../components/Modal';
import { useToast } from '../components/Toast';
import type { OAuthProvider } from '../types';

interface UserRow { id: string; email: string; username?: string; name?: string; avatar_url?: string; role?: string; created_at?: string; updated_at?: string; is_verified?: boolean; mfa_enabled?: boolean; _metadata?: { provider: string; provider_id: string }[]; }
interface Props { projectId: string; }
const ROLES = ['user', 'admin', 'moderator'];

// ===== OAuth Provider Templates =====
// Predefined templates pre-fill auth/token/userinfo URLs, scopes, and field
// mappings. The user still must supply client_id, client_secret, and redirect_url.
interface ProviderTemplate {
  label: string;
  color: string;
  fields: Partial<OAuthProvider>;
}

const PROVIDER_TEMPLATES: Record<string, ProviderTemplate> = {
  google: {
    label: 'Google',
    color: '#4285F4',
    fields: {
      auth_url: 'https://accounts.google.com/o/oauth2/v2/auth',
      token_url: 'https://oauth2.googleapis.com/token',
      userinfo_url: 'https://www.googleapis.com/oauth2/v3/userinfo',
      scopes: ['openid', 'email', 'profile'],
      email_field: 'email', name_field: 'name', avatar_field: 'picture', id_field: 'sub',
    },
  },
  github: {
    label: 'GitHub',
    color: '#f0f0f5',
    fields: {
      auth_url: 'https://github.com/login/oauth/authorize',
      token_url: 'https://github.com/login/oauth/access_token',
      userinfo_url: 'https://api.github.com/user',
      scopes: ['user:email'],
      email_field: 'email', name_field: 'name', avatar_field: 'avatar_url', id_field: 'id',
    },
  },
  discord: {
    label: 'Discord',
    color: '#5865F2',
    fields: {
      auth_url: 'https://discord.com/api/oauth2/authorize',
      token_url: 'https://discord.com/api/oauth2/token',
      userinfo_url: 'https://discord.com/api/users/@me',
      scopes: ['identify', 'email'],
      email_field: 'email', name_field: 'username', avatar_field: 'avatar', id_field: 'id',
    },
  },
  gitlab: {
    label: 'GitLab',
    color: '#FC6D26',
    fields: {
      auth_url: 'https://gitlab.com/oauth/authorize',
      token_url: 'https://gitlab.com/oauth/token',
      userinfo_url: 'https://gitlab.com/api/v4/user',
      scopes: ['read_user'],
      email_field: 'email', name_field: 'name', avatar_field: 'avatar_url', id_field: 'id',
    },
  },
  microsoft: {
    label: 'Microsoft',
    color: '#00A4EF',
    fields: {
      auth_url: 'https://login.microsoftonline.com/common/oauth2/v2.0/authorize',
      token_url: 'https://login.microsoftonline.com/common/oauth2/v2.0/token',
      userinfo_url: 'https://graph.microsoft.com/oidc/userinfo',
      scopes: ['openid', 'email', 'profile'],
      email_field: 'email', name_field: 'name', avatar_field: 'picture', id_field: 'sub',
    },
  },
  facebook: {
    label: 'Facebook',
    color: '#1877F2',
    fields: {
      auth_url: 'https://www.facebook.com/v18.0/dialog/oauth',
      token_url: 'https://graph.facebook.com/v18.0/oauth/access_token',
      userinfo_url: 'https://graph.facebook.com/v19.0/me?fields=id,name,email,picture',
      scopes: ['email'],
      email_field: 'email', name_field: 'name', avatar_field: 'picture', id_field: 'id',
    },
  },
  x: {
    label: 'X',
    color: '#1d9bf0',
    fields: {
      auth_url: 'https://twitter.com/i/oauth2/2',
      token_url: 'https://api.twitter.com/2/oauth2/token',
      userinfo_url: 'https://api.twitter.com/2/users/me',
      scopes: ['users.read', 'tweet.read'],
      email_field: 'email', name_field: 'name', avatar_field: 'profile_image_url', id_field: 'id',
    },
  },
  custom: {
    label: 'Custom',
    color: '#8b8b96',
    fields: {},
  },
};

// NOTE (discord avatar): the "avatar" field from Discord's userinfo is just a hash.
// The full CDN URL requires the user id (https://cdn.discordapp.com/avatars/{id}/{avatar}.png).
// We keep avatar_field: "avatar" for simplicity; full-URL construction is a known limitation.

// Detect which template (if any) a stored provider matches, for badge rendering.
function detectTemplate(p: OAuthProvider): string {
  for (const key of Object.keys(PROVIDER_TEMPLATES)) {
    if (key === 'custom') continue;
    const f = PROVIDER_TEMPLATES[key].fields;
    if (f.auth_url && p.auth_url && p.auth_url === f.auth_url) return key;
  }
  if (p.name && p.name !== 'custom' && PROVIDER_TEMPLATES[p.name]) return p.name;
  return 'custom';
}

// Colored letter badge. GitHub's light badge gets dark text for contrast.
function ProviderBadge({ templateKey, name }: { templateKey: string; name: string }) {
  const color = PROVIDER_TEMPLATES[templateKey]?.color ?? PROVIDER_TEMPLATES.custom.color;
  const dark = color === '#f0f0f5';
  return (
    <span
      className="inline-flex items-center justify-center w-8 h-8 rounded-full text-sm font-bold shrink-0 select-none"
      style={{ backgroundColor: color, color: dark ? '#18181b' : '#ffffff' }}
    >
      {(name || templateKey).charAt(0).toUpperCase()}
    </span>
  );
}

// Local form-state type. `template` is UI-only (tracks the chosen template) and
// is stripped before sending to the backend.
interface ProviderFormState extends OAuthProvider {
  template: string;
}

const emptyUser = { username: '', email: '', password: '', role: 'user', name: '', avatar_url: '' };
const emptyProvider: ProviderFormState = {
  name: '', template: '',
  client_id: '', client_secret: '', redirect_url: '',
  auth_url: '', token_url: '', userinfo_url: '', scopes: [],
  email_field: '', name_field: '', avatar_field: '', id_field: '',
};

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
  const [newPassword, setNewPassword] = useState('');
  // Provider modal state
  const [showProvModal, setShowProvModal] = useState(false);
  const [editProvider, setEditProvider] = useState<OAuthProvider | null>(null);
  const [pf, setPf] = useState<ProviderFormState>(emptyProvider);
  const [scopeText, setScopeText] = useState('');
  // When editing, holds the masked secret returned by GET. If the user leaves the
  // secret field empty, this masked value is sent back verbatim so the backend
  // keeps the existing secret. KNOWN LIMITATION: the backend stores whatever is
  // sent, so editing other fields without re-typing the secret will store the
  // masked string (e.g. "abcd••••") and effectively lose the real secret.
  const [maskedSecret, setMaskedSecret] = useState('');
  const [showAdvanced, setShowAdvanced] = useState(false);
  const [showSecret, setShowSecret] = useState(false);
  const [savingP, setSavingP] = useState(false);

  useEffect(() => {
    setLoading(true);
    Promise.all([
      api<UserRow[]>(`/admin/projects/${encodeURIComponent(projectId)}/users`).then((d) => setUsers(d || [])).catch(() => {}),
      api<{ providers: OAuthProvider[] }>(`/admin/projects/${encodeURIComponent(projectId)}/auth/providers`).then((d) => setProviders(d.providers || [])).catch(() => {}),
    ]).finally(() => setLoading(false));
  }, [projectId]);

  const filtered = users.filter((u) => (u.email || '').toLowerCase().includes(search.toLowerCase()) || (u.username || '').toLowerCase().includes(search.toLowerCase()) || (u.name || '').toLowerCase().includes(search.toLowerCase()));

  const refreshUsers = async () => { const d = await api<UserRow[]>(`/admin/projects/${encodeURIComponent(projectId)}/users`); setUsers(d || []); };
  const refreshProviders = async () => { const d = await api<{ providers: OAuthProvider[] }>(`/admin/projects/${encodeURIComponent(projectId)}/auth/providers`); setProviders(d.providers || []); };

  const addUser = async () => {
    if (!uf.username.trim() || !uf.email.trim() || !uf.password.trim()) return;
    setSavingU(true);
    try { await api(`/admin/projects/${encodeURIComponent(projectId)}/users`, { method: 'POST', body: JSON.stringify(uf) }); showToast('User added', 'success'); setShowAddUser(false); refreshUsers(); } catch (e: unknown) { showToast((e as Error).message || 'Failed', 'error'); }
    setSavingU(false);
  };

  const updateUser = async () => {
    if (!editUser) return;
    setSavingU(true);
    try { await api(`/admin/projects/${encodeURIComponent(projectId)}/users/${encodeURIComponent(editUser.id)}`, { method: 'PATCH', body: JSON.stringify({ username: uf.name || uf.username, email: uf.email, name: uf.name, avatar_url: uf.avatar_url, role: uf.role }) }); showToast('User updated', 'success'); setEditUser(null); refreshUsers(); } catch (e: unknown) { showToast((e as Error).message || 'Failed', 'error'); }
    setSavingU(false);
  };

  const deleteUser = async (id: string) => {
    if (!confirm('Delete user?')) return;
    try { await api(`/admin/projects/${encodeURIComponent(projectId)}/users/${encodeURIComponent(id)}`, { method: 'DELETE' }); showToast('User deleted', 'success'); setUsers((p) => p.filter((u) => u.id !== id)); } catch (e: unknown) { showToast((e as Error).message || 'Failed', 'error'); }
  };

  const changeUserPassword = async () => {
    if (!editUser || !newPassword.trim()) return;
    setSavingU(true);
    try {
      await api(`/admin/projects/${encodeURIComponent(projectId)}/users/${encodeURIComponent(editUser.id)}/password`, {
        method: 'POST', body: JSON.stringify({ password: newPassword.trim() }),
      });
      showToast('Password changed', 'success');
      setNewPassword('');
    } catch (e: unknown) { showToast((e as Error).message || 'Failed', 'error'); }
    setSavingU(false);
  };

  // ===== Provider handlers =====
  // PUT replaces the entire provider list atomically. We mutate a local copy,
  // PUT the full list back, then re-GET to refresh masked secrets.
  const putProviders = async (newList: OAuthProvider[]) => {
    await api<{ providers: number }>(
      `/admin/projects/${encodeURIComponent(projectId)}/auth/providers`,
      { method: 'PUT', body: JSON.stringify({ providers: newList }) },
    );
  };

  const saveProvider = async () => {
    if (!pf.name.trim() || !pf.client_id.trim()) {
      showToast('Name and Client ID are required', 'error');
      return;
    }
    setSavingP(true);
    try {
      // Preserve masked secret when the user did not type a new one.
      const secretToSend = pf.client_secret.trim() !== '' ? pf.client_secret : maskedSecret;
      const provider: OAuthProvider = {
        name: pf.name.trim(),
        client_id: pf.client_id.trim(),
        client_secret: secretToSend,
        redirect_url: pf.redirect_url.trim(),
        auth_url: pf.auth_url.trim(),
        token_url: pf.token_url.trim(),
        userinfo_url: pf.userinfo_url.trim(),
        scopes: scopeText.split(',').map((s) => s.trim()).filter(Boolean),
        email_field: pf.email_field.trim(),
        name_field: pf.name_field.trim(),
        avatar_field: pf.avatar_field.trim(),
        id_field: pf.id_field.trim(),
      };
      let newList: OAuthProvider[];
      if (editProvider) {
        newList = providers.map((p) => (p.name === editProvider.name ? provider : p));
      } else {
        if (providers.some((p) => p.name === provider.name)) {
          showToast(`Provider "${provider.name}" already exists`, 'error');
          setSavingP(false);
          return;
        }
        newList = [...providers, provider];
      }
      await putProviders(newList);
      showToast(editProvider ? 'Provider updated' : 'Provider added', 'success');
      closeProvModal();
      await refreshProviders();
    } catch (e: unknown) {
      showToast((e as Error).message || 'Failed', 'error');
    }
    setSavingP(false);
  };

  const deleteProvider = async (name: string) => {
    if (!confirm(`Delete provider "${name}"?`)) return;
    try {
      await putProviders(providers.filter((p) => p.name !== name));
      showToast('Provider deleted', 'success');
      await refreshProviders();
    } catch (e: unknown) {
      showToast((e as Error).message || 'Failed', 'error');
    }
  };

  const pickTemplate = (key: string) => {
    const tpl = PROVIDER_TEMPLATES[key];
    if (!tpl) return;
    const name = key === 'custom' ? pf.name : key;
    setPf((f) => ({
      ...f,
      template: key,
      name,
      auth_url: tpl.fields.auth_url ?? '',
      token_url: tpl.fields.token_url ?? '',
      userinfo_url: tpl.fields.userinfo_url ?? '',
      email_field: tpl.fields.email_field ?? '',
      name_field: tpl.fields.name_field ?? '',
      avatar_field: tpl.fields.avatar_field ?? '',
      id_field: tpl.fields.id_field ?? '',
    }));
    setScopeText(tpl.fields.scopes ? tpl.fields.scopes.join(', ') : '');
    setShowAdvanced(key === 'custom');
  };

  const openAddProvider = () => {
    setEditProvider(null);
    setMaskedSecret('');
    setPf(emptyProvider);
    setScopeText('');
    setShowAdvanced(false);
    setShowSecret(false);
    setShowProvModal(true);
  };

  const openEditProvider = (p: OAuthProvider) => {
    setEditProvider(p);
    setMaskedSecret(p.client_secret);
    setPf({
      ...p,
      template: detectTemplate(p),
      // Clear the secret field; the masked value is shown as a placeholder only.
      client_secret: '',
    });
    setScopeText((p.scopes || []).join(', '));
    setShowAdvanced(false);
    setShowSecret(false);
    setShowProvModal(true);
  };

  const closeProvModal = () => {
    setShowProvModal(false);
    setEditProvider(null);
    setMaskedSecret('');
  };

  const openAdd = () => { setUf(emptyUser); setShowAddUser(true); };
  const openEdit = (u: UserRow) => { const displayName = u.name || u.username || ''; setEditUser(u); setNewPassword(''); setUf({ username: displayName, email: u.email || '', password: '', role: u.role || 'user', name: displayName, avatar_url: u.avatar_url || '' }); };

  if (loading) return <div className="flex items-center gap-2 py-10 text-text-muted"><Loader2 className="w-4 h-4 animate-spin" /> Loading auth data...</div>;

  const showPicker = !editProvider && pf.template === '';
  const valid = pf.name.trim() !== '' && pf.client_id.trim() !== '';

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
          <div className="border border-border rounded-lg overflow-x-auto">
            <table className="w-full text-sm">
              <thead className="bg-bg-input text-text-muted text-xs uppercase"><tr><th className="text-left px-4 py-3 font-medium">ID</th><th className="text-left px-4 py-3 font-medium">Name</th><th className="text-left px-4 py-3 font-medium">Email</th><th className="text-left px-4 py-3 font-medium">Role</th><th className="text-left px-4 py-3 font-medium">OAuth</th><th className="text-left px-4 py-3 font-medium">Status</th><th className="text-left px-4 py-3 font-medium">Created</th><th className="text-right px-4 py-3 font-medium">Actions</th></tr></thead>
              <tbody className="divide-y divide-border">
                {filtered.map((u) => (
                  <tr key={u.id} className="hover:bg-bg-input/50 transition-colors">
                    <td className="px-4 py-3 font-mono text-text-primary text-xs">{u.id.slice(0, 8)}…</td>
                    <td className="px-4 py-3 text-text-primary text-sm">{u.name || u.username || '—'}</td>
                    <td className="px-4 py-3 text-text-secondary text-sm">{u.email}</td>
                    <td className="px-4 py-3"><span className={`inline-flex px-2 py-0.5 rounded-full text-xs font-medium ${(u.role || 'user') === 'admin' ? 'bg-success/10 text-success' : 'bg-accent/10 text-accent'}`}>{u.role || 'user'}</span></td>
                    <td className="px-4 py-3">{u._metadata && u._metadata.length > 0 ? <div className="flex flex-wrap gap-1">{u._metadata.map((m) => { const tplKey = PROVIDER_TEMPLATES[m.provider] ? m.provider : 'custom'; const color = PROVIDER_TEMPLATES[tplKey]?.color ?? '#8b8b96'; return <span key={m.provider} className="inline-flex items-center gap-1 px-1.5 py-0.5 rounded text-[10px] font-medium" style={{ backgroundColor: color + '22', color }}>{m.provider}</span>; })}</div> : <span className="text-text-muted text-xs">—</span>}</td>
                    <td className="px-4 py-3"><div className="flex items-center gap-1.5">{u.is_verified && <span title="Verified" className="text-success"><Check className="w-3.5 h-3.5" /></span>}{u.mfa_enabled && <span title="MFA enabled" className="text-accent"><Shield className="w-3.5 h-3.5" /></span>}{!u.is_verified && !u.mfa_enabled && <span className="text-text-muted text-xs">—</span>}</div></td>
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
          <h2 className="text-lg font-semibold text-text-primary flex items-center gap-2"><Shield className="w-5 h-5 text-accent" /> OAuth Providers ({providers.length})</h2>
          <button onClick={openAddProvider} className="flex items-center gap-1 px-3 py-2 bg-accent text-white rounded-lg text-sm font-medium hover:opacity-90"><Plus className="w-4 h-4" /> Add Provider</button>
        </div>
        {providers.length === 0 ? (
          <div className="flex flex-col items-center py-12 text-text-muted gap-3 bg-bg-card border border-border rounded-lg"><Globe className="w-8 h-8" /><p className="text-sm">No OAuth providers configured</p></div>
        ) : (
          <div className="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-3 gap-3">
            {providers.map((p) => {
              const tplKey = detectTemplate(p);
              const cid = p.client_id || '';
              return (
                <div key={p.name} className="bg-bg-card border border-border rounded-lg p-4 flex flex-col gap-3">
                  <div className="flex items-start justify-between gap-2">
                    <div className="flex items-center gap-2 min-w-0">
                      <ProviderBadge templateKey={tplKey} name={p.name} />
                      <div className="min-w-0">
                        <h3 className="text-text-primary font-medium text-sm truncate">{p.name}</h3>
                        {cid && <p className="text-text-muted text-xs font-mono truncate">Client: {cid.length > 24 ? cid.slice(0, 24) + '…' : cid}</p>}
                      </div>
                    </div>
                    <div className="flex items-center gap-1 shrink-0">
                      <button onClick={() => openEditProvider(p)} title="Edit" className="p-1.5 bg-bg-input border border-border rounded text-text-secondary hover:text-text-primary"><Pencil className="w-3.5 h-3.5" /></button>
                      <button onClick={() => deleteProvider(p.name)} title="Delete" className="p-1.5 bg-bg-input border border-border rounded text-text-muted hover:text-danger"><Trash2 className="w-3.5 h-3.5" /></button>
                    </div>
                  </div>
                  {p.redirect_url && <p className="text-text-muted text-xs font-mono truncate">{p.redirect_url}</p>}
                  {p.scopes && p.scopes.length > 0 && (
                    <div className="flex flex-wrap gap-1">
                      {p.scopes.map((s) => <span key={s} className="px-1.5 py-0.5 bg-bg-input text-text-muted rounded text-[10px] font-mono">{s}</span>)}
                    </div>
                  )}
                </div>
              );
            })}
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
          <div><label className="block text-text-secondary text-sm mb-1">Display Name</label><input type="text" value={uf.name || uf.username} onChange={(e) => setUf((f) => ({ ...f, name: e.target.value }))} className="w-full bg-bg-input border border-border rounded-md px-3 py-2 text-text-primary text-sm focus:outline-none focus:border-accent" /></div>
          <div><label className="block text-text-secondary text-sm mb-1">Email</label><input type="email" value={uf.email} onChange={(e) => setUf((f) => ({ ...f, email: e.target.value }))} className="w-full bg-bg-input border border-border rounded-md px-3 py-2 text-text-primary text-sm focus:outline-none focus:border-accent" /></div>
          <div><label className="block text-text-secondary text-sm mb-1">Avatar URL</label><input type="text" value={uf.avatar_url} onChange={(e) => setUf((f) => ({ ...f, avatar_url: e.target.value }))} className="w-full bg-bg-input border border-border rounded-md px-3 py-2 text-text-primary text-sm focus:outline-none focus:border-accent" /></div>
          <div><label className="block text-text-secondary text-sm mb-1">Role</label><select value={uf.role} onChange={(e) => setUf((f) => ({ ...f, role: e.target.value }))} className="w-full bg-bg-input border border-border rounded-md px-3 py-2 text-text-primary text-sm focus:outline-none focus:border-accent">{ROLES.map((r) => <option key={r} value={r}>{r}</option>)}</select></div>
          <div className="border-t border-border pt-3">
            <label className="block text-text-secondary text-sm mb-1">New Password</label>
            <div className="flex gap-2">
              <input type="text" value={newPassword} onChange={(e) => setNewPassword(e.target.value)} placeholder="Leave empty to keep current" className="flex-1 bg-bg-input border border-border rounded-md px-3 py-2 text-text-primary text-sm placeholder:text-text-muted focus:outline-none focus:border-accent" />
              <button onClick={changeUserPassword} disabled={!newPassword.trim() || savingU} className="px-3 py-2 bg-accent text-white rounded-md text-sm font-medium hover:opacity-90 disabled:opacity-50 shrink-0">{savingU ? <Loader2 className="w-4 h-4 animate-spin" /> : 'Change'}</button>
            </div>
          </div>
          <div className="flex justify-end gap-2 pt-2"><button onClick={() => setEditUser(null)} className="px-4 py-2 border border-border rounded-lg text-text-secondary text-sm hover:text-text-primary">Cancel</button><button onClick={updateUser} disabled={savingU} className="flex items-center gap-1 px-4 py-2 bg-accent text-white rounded-lg text-sm font-medium hover:opacity-90 disabled:opacity-50">{savingU ? <Loader2 className="w-4 h-4 animate-spin" /> : <Check className="w-4 h-4" />} Save</button></div>
        </div>
      </Modal>

      {/* Provider Modal (Add / Edit) */}
      <Modal isOpen={showProvModal} onClose={closeProvModal} title={editProvider ? 'Edit OAuth Provider' : 'Add OAuth Provider'} size="lg">
        <div className="space-y-4">
          {showPicker && (
            <div>
              <p className="text-text-secondary text-sm mb-3">Choose a provider template to pre-fill its settings. You can edit any field afterward.</p>
              <div className="grid grid-cols-2 sm:grid-cols-4 gap-2">
                {Object.keys(PROVIDER_TEMPLATES).map((key) => {
                  const tpl = PROVIDER_TEMPLATES[key];
                  return (
                    <button key={key} onClick={() => pickTemplate(key)} className="flex flex-col items-center gap-2 p-3 bg-bg-input border border-border rounded-lg hover:border-accent transition-colors">
                      <ProviderBadge templateKey={key} name={tpl.label} />
                      <span className="text-text-primary text-xs font-medium">{tpl.label}</span>
                    </button>
                  );
                })}
              </div>
            </div>
          )}

          {!showPicker && (
            <>
              <div className="grid grid-cols-1 sm:grid-cols-2 gap-4">
                <div>
                  <label className="block text-text-secondary text-sm mb-1">Name</label>
                  <input type="text" value={pf.name} onChange={(e) => setPf((f) => ({ ...f, name: e.target.value }))} disabled={!!editProvider} placeholder="e.g. google, my-sso" className="w-full bg-bg-input border border-border rounded-md px-3 py-2 text-text-primary text-sm focus:outline-none focus:border-accent disabled:opacity-60 disabled:cursor-not-allowed" autoFocus />
                </div>
                <div>
                  <label className="block text-text-secondary text-sm mb-1">Client ID</label>
                  <input type="text" value={pf.client_id} onChange={(e) => setPf((f) => ({ ...f, client_id: e.target.value }))} className="w-full bg-bg-input border border-border rounded-md px-3 py-2 text-text-primary text-sm focus:outline-none focus:border-accent font-mono" />
                </div>
              </div>

              <div>
                <label className="block text-text-secondary text-sm mb-1">Client Secret</label>
                <div className="relative">
                  <input type={showSecret ? 'text' : 'password'} value={pf.client_secret} onChange={(e) => setPf((f) => ({ ...f, client_secret: e.target.value }))} placeholder={editProvider ? (maskedSecret || 'Leave empty to keep current secret') : ''} className="w-full bg-bg-input border border-border rounded-md px-3 py-2 pr-9 text-text-primary text-sm focus:outline-none focus:border-accent font-mono placeholder:text-text-muted" />
                  <button type="button" onClick={() => setShowSecret((s) => !s)} className="absolute right-2 top-1/2 -translate-y-1/2 text-text-muted hover:text-text-secondary">{showSecret ? <EyeOff className="w-4 h-4" /> : <Eye className="w-4 h-4" />}</button>
                </div>
                {editProvider && <p className="text-text-muted text-xs mt-1">Leave empty to keep current secret{maskedSecret ? ` (${maskedSecret})` : ''}.</p>}
              </div>

              <div>
                <label className="block text-text-secondary text-sm mb-1">Redirect URL</label>
                <input type="text" value={pf.redirect_url} onChange={(e) => setPf((f) => ({ ...f, redirect_url: e.target.value }))} placeholder="https://your-app.example.com/auth/callback" className="w-full bg-bg-input border border-border rounded-md px-3 py-2 text-text-primary text-sm focus:outline-none focus:border-accent font-mono" />
                <p className="text-text-muted text-xs mt-1">Set this exact URL in your OAuth provider's redirect URIs.</p>
              </div>

              <div>
                <label className="block text-text-secondary text-sm mb-1">Scopes <span className="text-text-muted font-normal">(comma-separated)</span></label>
                <input type="text" value={scopeText} onChange={(e) => setScopeText(e.target.value)} placeholder="openid, email, profile" className="w-full bg-bg-input border border-border rounded-md px-3 py-2 text-text-primary text-sm focus:outline-none focus:border-accent font-mono" />
              </div>

              {/* Advanced (collapsible) */}
              <div className="border border-border rounded-lg">
                <button type="button" onClick={() => setShowAdvanced((s) => !s)} className="w-full flex items-center justify-between px-3 py-2 text-text-secondary text-sm hover:text-text-primary">
                  <span className="flex items-center gap-2"><SlidersHorizontal className="w-4 h-4" /> Advanced (OAuth endpoints &amp; field mappings)</span>
                  {showAdvanced ? <ChevronUp className="w-4 h-4" /> : <ChevronDown className="w-4 h-4" />}
                </button>
                {showAdvanced && (
                  <div className="px-3 pb-3 pt-1 space-y-3 border-t border-border">
                    <div>
                      <label className="block text-text-secondary text-sm mb-1">Auth URL</label>
                      <input type="text" value={pf.auth_url} onChange={(e) => setPf((f) => ({ ...f, auth_url: e.target.value }))} className="w-full bg-bg-input border border-border rounded-md px-3 py-2 text-text-primary text-sm focus:outline-none focus:border-accent font-mono" />
                    </div>
                    <div>
                      <label className="block text-text-secondary text-sm mb-1">Token URL</label>
                      <input type="text" value={pf.token_url} onChange={(e) => setPf((f) => ({ ...f, token_url: e.target.value }))} className="w-full bg-bg-input border border-border rounded-md px-3 py-2 text-text-primary text-sm focus:outline-none focus:border-accent font-mono" />
                    </div>
                    <div>
                      <label className="block text-text-secondary text-sm mb-1">Userinfo URL</label>
                      <input type="text" value={pf.userinfo_url} onChange={(e) => setPf((f) => ({ ...f, userinfo_url: e.target.value }))} className="w-full bg-bg-input border border-border rounded-md px-3 py-2 text-text-primary text-sm focus:outline-none focus:border-accent font-mono" />
                    </div>
                    <div className="grid grid-cols-2 gap-3">
                      <div>
                        <label className="block text-text-secondary text-xs mb-1">Email field</label>
                        <input type="text" value={pf.email_field} onChange={(e) => setPf((f) => ({ ...f, email_field: e.target.value }))} className="w-full bg-bg-input border border-border rounded-md px-2 py-1.5 text-text-primary text-xs focus:outline-none focus:border-accent font-mono" />
                      </div>
                      <div>
                        <label className="block text-text-secondary text-xs mb-1">Name field</label>
                        <input type="text" value={pf.name_field} onChange={(e) => setPf((f) => ({ ...f, name_field: e.target.value }))} className="w-full bg-bg-input border border-border rounded-md px-2 py-1.5 text-text-primary text-xs focus:outline-none focus:border-accent font-mono" />
                      </div>
                      <div>
                        <label className="block text-text-secondary text-xs mb-1">Avatar field</label>
                        <input type="text" value={pf.avatar_field} onChange={(e) => setPf((f) => ({ ...f, avatar_field: e.target.value }))} className="w-full bg-bg-input border border-border rounded-md px-2 py-1.5 text-text-primary text-xs focus:outline-none focus:border-accent font-mono" />
                      </div>
                      <div>
                        <label className="block text-text-secondary text-xs mb-1">ID field</label>
                        <input type="text" value={pf.id_field} onChange={(e) => setPf((f) => ({ ...f, id_field: e.target.value }))} className="w-full bg-bg-input border border-border rounded-md px-2 py-1.5 text-text-primary text-xs focus:outline-none focus:border-accent font-mono" />
                      </div>
                    </div>
                    <p className="text-text-muted text-xs">JSON key path in the userinfo response (dot-notation supported). Leave empty to use backend defaults (email / name / avatar_url / id).</p>
                  </div>
                )}
              </div>

              <div className="flex justify-end gap-2 pt-1">
                <button onClick={closeProvModal} className="px-4 py-2 border border-border rounded-lg text-text-secondary text-sm hover:text-text-primary">Cancel</button>
                <button onClick={saveProvider} disabled={!valid || savingP} className="flex items-center gap-1 px-4 py-2 bg-accent text-white rounded-lg text-sm font-medium hover:opacity-90 disabled:opacity-50 disabled:cursor-not-allowed">{savingP ? <Loader2 className="w-4 h-4 animate-spin" /> : editProvider ? <Check className="w-4 h-4" /> : <Plus className="w-4 h-4" />} {editProvider ? 'Save' : 'Add'}</button>
              </div>
            </>
          )}
        </div>
      </Modal>
    </div>
  );
}
