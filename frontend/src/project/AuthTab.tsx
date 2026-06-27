import { useState, useEffect } from 'react';
import { Search, Shield, Users, ToggleLeft, ToggleRight, ExternalLink } from 'lucide-react';
import { api, formatDate } from '../api';
import type { User, OAuthProvider } from '../types';

interface Props {
  projectId: string;
  apiKey?: string;
}

export default function AuthTab({ projectId }: Props) {
  const [users, setUsers] = useState<User[]>([]);
  const [providers, setProviders] = useState<OAuthProvider[]>([]);
  const [loading, setLoading] = useState(true);
  const [search, setSearch] = useState('');

  useEffect(() => {
    setLoading(true);
    Promise.all([
      api<{ users: User[] }>(`/admin/projects/${encodeURIComponent(projectId)}/users`).then((d) => setUsers(d.users || [])).catch(() => {}),
      api<{ providers: OAuthProvider[] }>(`/admin/projects/${encodeURIComponent(projectId)}/auth`).then((d) => setProviders(d.providers || [])).catch(() => {}),
    ]).finally(() => setLoading(false));
  }, [projectId]);

  const filtered = users.filter((u) =>
    u.email?.toLowerCase().includes(search.toLowerCase()) ||
    u.name?.toLowerCase().includes(search.toLowerCase())
  );

  if (loading) return <div className="py-10 text-text-muted">Loading auth data...</div>;

  return (
    <div className="space-y-8">
      {/* OAuth Providers */}
      <section>
        <h2 className="text-lg font-semibold text-text-primary mb-3 flex items-center gap-2">
          <Shield className="w-5 h-5 text-accent" /> OAuth Providers
        </h2>
        {providers.length === 0 ? (
          <p className="text-text-muted text-sm">No OAuth providers configured</p>
        ) : (
          <div className="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-3 gap-3">
            {providers.map((p) => (
              <div key={p.name} className="bg-bg-card border border-border rounded-lg p-4 flex items-center justify-between">
                <div>
                  <p className="text-text-primary font-medium text-sm">{p.name}</p>
                  {p.client_id && <p className="text-text-muted text-xs font-mono mt-1 truncate max-w-[180px]">{p.client_id}</p>}
                </div>
                <div className="flex items-center gap-2">
                  {p.enabled ? (
                    <ToggleRight className="w-6 h-6 text-success" />
                  ) : (
                    <ToggleLeft className="w-6 h-6 text-text-muted" />
                  )}
                  {p.redirect_url && (
                    <a href={p.redirect_url} target="_blank" rel="noopener noreferrer" className="text-text-muted hover:text-accent transition-colors">
                      <ExternalLink className="w-4 h-4" />
                    </a>
                  )}
                </div>
              </div>
            ))}
          </div>
        )}
      </section>

      {/* Users */}
      <section>
        <div className="flex items-center justify-between mb-3">
          <h2 className="text-lg font-semibold text-text-primary flex items-center gap-2">
            <Users className="w-5 h-5 text-accent" /> Users ({users.length})
          </h2>
          <div className="relative max-w-xs">
            <Search className="absolute left-3 top-1/2 -translate-y-1/2 w-4 h-4 text-text-muted" />
            <input
              type="text"
              placeholder="Search users..."
              value={search}
              onChange={(e) => setSearch(e.target.value)}
              className="w-full bg-bg-input border border-border rounded-md pl-9 pr-3 py-2 text-text-primary text-sm placeholder:text-text-muted focus:outline-none focus:border-accent"
            />
          </div>
        </div>

        {filtered.length === 0 ? (
          <div className="flex flex-col items-center py-16 text-text-muted gap-3">
            <Users className="w-10 h-10" />
            <p>{users.length === 0 ? 'No users' : 'No users match search'}</p>
          </div>
        ) : (
          <div className="border border-border rounded-lg overflow-hidden">
            <table className="w-full text-sm">
              <thead className="bg-bg-input text-text-muted text-xs uppercase">
                <tr>
                  <th className="text-left px-4 py-3 font-medium">ID</th>
                  <th className="text-left px-4 py-3 font-medium">Email</th>
                  <th className="text-left px-4 py-3 font-medium">Name</th>
                  <th className="text-left px-4 py-3 font-medium">Providers</th>
                  <th className="text-left px-4 py-3 font-medium">Role</th>
                  <th className="text-left px-4 py-3 font-medium">Created</th>
                </tr>
              </thead>
              <tbody className="divide-y divide-border">
                {filtered.map((u) => (
                  <tr key={u.id} className="hover:bg-bg-input/50 transition-colors">
                    <td className="px-4 py-3 font-mono text-text-primary text-xs">{u.id.slice(0, 12)}...</td>
                    <td className="px-4 py-3 text-text-primary">{u.email || '—'}</td>
                    <td className="px-4 py-3 text-text-secondary">{u.name || '—'}</td>
                    <td className="px-4 py-3">
                      <div className="flex gap-1 flex-wrap">
                        {u._metadata && u._metadata.length > 0 ? (
                          u._metadata.map((m, i) => (
                            <span key={i} className="inline-flex px-2 py-0.5 bg-bg-input border border-border rounded-full text-xs text-text-secondary font-mono">
                              {m.provider}
                            </span>
                          ))
                        ) : (
                          <span className="text-text-muted text-xs">email</span>
                        )}
                      </div>
                    </td>
                    <td className="px-4 py-3">
                      <span className="inline-flex px-2 py-0.5 bg-accent/10 text-accent rounded-full text-xs font-medium">
                        {u.role || 'user'}
                      </span>
                    </td>
                    <td className="px-4 py-3 text-text-secondary text-xs">{formatDate(u.created_at)}</td>
                  </tr>
                ))}
              </tbody>
            </table>
          </div>
        )}
      </section>
    </div>
  );
}
