import { useState, useEffect } from 'react';
import { Trash2, Loader2, Users, Shield, AlertTriangle } from 'lucide-react';
import { api } from '../api';
import { useToast } from '../components/Toast';
import { useAuth } from '../store';

interface GlobalMember {
  user_id: string;
  email: string;
  name?: string;
  is_admin?: boolean;
  admin_role?: string;
  project_ids?: string[];
  project_count?: number;
}

export default function GlobalMembers() {
  const { showToast } = useToast();
  const { role: adminRole } = useAuth();
  const [members, setMembers] = useState<GlobalMember[]>([]);
  const [loading, setLoading] = useState(true);
  const [removeTarget, setRemoveTarget] = useState<GlobalMember | null>(null);

  const loadMembers = async () => {
    setLoading(true);
    try {
      const res = await api<{ members: GlobalMember[]; count: number }>('/admin/members');
      setMembers(res.members || []);
    } catch (e) {
      showToast((e as Error).message, 'error');
    } finally {
      setLoading(false);
    }
  };

  useEffect(() => { loadMembers(); }, []);

  const removeMember = async () => {
    if (!removeTarget) return;
    setRemoveTarget(null);
    try {
      await api(`/admin/members/${encodeURIComponent(removeTarget.email)}`, { method: 'DELETE' });
      showToast('Member removed globally', 'success');
      loadMembers();
    } catch (e) {
      showToast((e as Error).message, 'error');
    }
  };

  if (adminRole !== 'super_admin') {
    return (
      <div className="flex flex-col items-center justify-center h-full">
        <Shield className="w-16 h-16 text-text-muted mb-4" />
        <h2 className="text-xl font-semibold text-text-primary mb-2">Access Denied</h2>
        <p className="text-text-muted">Only super admins can manage global members.</p>
      </div>
    );
  }

  return (
    <div className="space-y-4">
      <div className="flex items-center justify-between">
        <h2 className="text-lg font-semibold text-text-primary">Global Users ({members.length})</h2>
        <div className="text-sm text-text-muted">
          All team members across all projects
        </div>
      </div>

      {loading ? (
        <div className="flex items-center gap-2 py-10 text-text-muted">
          <Loader2 className="w-4 h-4 animate-spin" /> Loading users...
        </div>
      ) : members.length === 0 ? (
        <div className="flex flex-col items-center py-16 text-text-muted gap-3">
          <Users className="w-10 h-10" />
          <p>No team members found</p>
        </div>
      ) : (
        <div className="border border-border rounded-lg overflow-hidden">
          <table className="w-full text-sm">
            <thead className="bg-bg-input text-text-muted text-xs uppercase">
              <tr>
                <th className="text-left px-4 py-3 font-medium">User</th>
                <th className="text-left px-4 py-3 font-medium">User ID</th>
                <th className="text-left px-4 py-3 font-medium">Projects</th>
                <th className="text-right px-4 py-3 font-medium">Actions</th>
              </tr>
            </thead>
            <tbody className="divide-y divide-border">
              {members.map((m) => (
                <tr key={m.user_id} className="hover:bg-bg-input/50 transition-colors">
                  <td className="px-4 py-3 text-text-primary text-sm font-medium">
                    <div className="flex items-center gap-2">
                      {m.email || m.user_id}
                      {m.is_admin && (
                        <span className="inline-flex items-center gap-1 px-2 py-0.5 bg-accent/10 text-accent text-xs font-medium rounded-full">
                          <Shield className="w-3 h-3" />
                          {m.admin_role === 'super_admin' ? 'Super Admin' : 'Admin'}
                        </span>
                      )}
                    </div>
                  </td>
                  <td className="px-4 py-3 text-text-secondary text-xs font-mono">
                    {m.user_id}
                  </td>
                  <td className="px-4 py-3 text-text-secondary text-xs">
                    {m.project_ids && m.project_ids.length > 0 ? (
                      <div className="flex items-center gap-2">
                        <span className="font-medium text-text-primary">{m.project_count || m.project_ids.length}</span>
                        <div className="flex flex-wrap gap-1">
                          {m.project_ids.slice(0, 3).map((pid) => (
                            <span key={pid} className="px-2 py-0.5 bg-accent/10 text-accent text-xs rounded">
                              {pid.slice(0, 8)}…
                            </span>
                          ))}
                          {m.project_ids.length > 3 && (
                            <span className="text-text-muted text-xs">+{m.project_ids.length - 3} more</span>
                          )}
                        </div>
                      </div>
                    ) : (
                      <span className="text-text-muted">None</span>
                    )}
                  </td>
                  <td className="px-4 py-3 text-right">
                    <button
                      onClick={() => setRemoveTarget(m)}
                      className="p-1.5 rounded transition-colors hover:bg-danger/10 text-text-secondary hover:text-danger"
                      title="Remove globally"
                    >
                      <Trash2 className="w-4 h-4" />
                    </button>
                  </td>
                </tr>
              ))}
            </tbody>
          </table>
        </div>
      )}

      {removeTarget && (
        <div className="fixed inset-0 bg-black/50 flex items-center justify-center z-50">
          <div className="bg-bg-card border border-border rounded-lg p-6 w-full max-w-md shadow-xl">
            <div className="flex items-center gap-3 mb-4">
              <div className="p-2 bg-danger/10 rounded-lg">
                <AlertTriangle className="w-5 h-5 text-danger" />
              </div>
              <h3 className="text-lg font-semibold text-text-primary">Remove Global User</h3>
            </div>
            <p className="text-sm text-text-muted mb-6">
              This will remove <span className="font-mono text-text-primary">{removeTarget.email}</span> from ALL projects and delete their credentials. This cannot be undone.
            </p>
            <div className="flex justify-end gap-2">
              <button
                onClick={() => setRemoveTarget(null)}
                className="px-4 py-2 rounded-lg border border-border text-text-primary hover:bg-bg-input transition-colors"
              >
                Cancel
              </button>
              <button
                onClick={removeMember}
                className="px-4 py-2 rounded-lg bg-danger text-white hover:bg-danger/90 transition-colors"
              >
                Remove
              </button>
            </div>
          </div>
        </div>
      )}
    </div>
  );
}