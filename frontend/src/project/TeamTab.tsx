import { useState, useEffect } from 'react';
import { UserPlus, Trash2, Loader2, Users } from 'lucide-react';
import { api, formatDate } from '../api';
import type { TeamMember } from '../types';
import { Modal } from '../components/Modal';
import { ConfirmDialog } from '../components/ConfirmDialog';
import { useToast } from '../components/Toast';

interface Props {
  projectId: string;
  apiKey?: string;
}

const ROLES = ['owner', 'admin', 'developer', 'viewer'] as const;
type Role = (typeof ROLES)[number];

export default function TeamTab({ projectId }: Props) {
  const { show } = useToast();
  const [members, setMembers] = useState<TeamMember[]>([]);
  const [loading, setLoading] = useState(true);
  const [inviteOpen, setInviteOpen] = useState(false);
  const [removeTarget, setRemoveTarget] = useState<string | null>(null);
  const [submitting, setSubmitting] = useState(false);

  const loadMembers = () => {
    setLoading(true);
    api<TeamMember[]>(`/admin/projects/${encodeURIComponent(projectId)}/members`)
      .then(setMembers)
      .finally(() => setLoading(false));
  };

  useEffect(() => { loadMembers(); }, [projectId]);

  const changeRole = async (userId: string, role: Role) => {
    try {
      await api(`/admin/projects/${encodeURIComponent(projectId)}/members/${encodeURIComponent(userId)}`, {
        method: 'PATCH',
        body: JSON.stringify({ role }),
      });
      setMembers((prev) => prev.map((m) => (m.user_id === userId ? { ...m, role } : m)));
      show('Role updated', 'success');
    } catch (e) {
      show((e as Error).message, 'error');
    }
  };

  const removeMember = async () => {
    if (!removeTarget) return;
    setSubmitting(true);
    try {
      await api(`/admin/projects/${encodeURIComponent(projectId)}/members/${encodeURIComponent(removeTarget)}`, { method: 'DELETE' });
      setMembers((prev) => prev.filter((m) => m.user_id !== removeTarget));
      show('Member removed', 'success');
    } catch (e) {
      show((e as Error).message, 'error');
    } finally {
      setSubmitting(false);
      setRemoveTarget(null);
    }
  };

  const handleInvite = async (e: React.FormEvent<HTMLFormElement>) => {
    e.preventDefault();
    const form = e.currentTarget;
    const email = (form.elements.namedItem('email') as HTMLInputElement).value.trim();
    const role = (form.elements.namedItem('role') as HTMLSelectElement).value as Role;
    if (!email) return;
    setSubmitting(true);
    try {
      await api(`/admin/projects/${encodeURIComponent(projectId)}/invite`, {
        method: 'POST',
        body: JSON.stringify({ email, role }),
      });
      show('Invitation sent', 'success');
      setInviteOpen(false);
      loadMembers();
    } catch (e) {
      show((e as Error).message, 'error');
    } finally {
      setSubmitting(false);
    }
  };

  if (loading) {
    return <div className="flex items-center gap-2 py-10 text-text-muted"><Loader2 className="w-4 h-4 animate-spin" /> Loading team...</div>;
  }

  return (
    <div className="space-y-4">
      <div className="flex items-center justify-between">
        <h2 className="text-lg font-semibold text-text-primary">Team Members ({members.length})</h2>
        <button onClick={() => setInviteOpen(true)} className="flex items-center gap-2 px-4 py-2 bg-accent text-white rounded-md hover:bg-accent-hover transition-colors text-sm font-medium">
          <UserPlus className="w-4 h-4" /> Invite Member
        </button>
      </div>

      {members.length === 0 ? (
        <div className="flex flex-col items-center py-16 text-text-muted gap-3">
          <Users className="w-10 h-10" />
          <p>No team members yet</p>
        </div>
      ) : (
        <div className="border border-border rounded-lg overflow-hidden">
          <table className="w-full text-sm">
            <thead className="bg-bg-input text-text-muted text-xs uppercase">
              <tr>
                <th className="text-left px-4 py-3 font-medium">User ID</th>
                <th className="text-left px-4 py-3 font-medium">Role</th>
                <th className="text-left px-4 py-3 font-medium">Joined</th>
                <th className="text-right px-4 py-3 font-medium">Actions</th>
              </tr>
            </thead>
            <tbody className="divide-y divide-border">
              {members.map((m) => (
                <tr key={m.user_id} className="hover:bg-bg-input/50 transition-colors">
                  <td className="px-4 py-3 font-mono text-text-primary text-xs">{m.user_id.slice(0, 12)}...</td>
                  <td className="px-4 py-3">
                    <select
                      value={m.role}
                      onChange={(e) => changeRole(m.user_id, e.target.value as Role)}
                      className="bg-bg-input border border-border rounded px-2 py-1 text-text-primary text-xs"
                    >
                      {ROLES.map((r) => (
                        <option key={r} value={r}>{r}</option>
                      ))}
                    </select>
                  </td>
                  <td className="px-4 py-3 text-text-secondary text-xs">{formatDate(m.joined_at)}</td>
                  <td className="px-4 py-3 text-right">
                    <button onClick={() => setRemoveTarget(m.user_id)} className="p-1.5 rounded hover:bg-danger/10 text-text-secondary hover:text-danger transition-colors" title="Remove">
                      <Trash2 className="w-4 h-4" />
                    </button>
                  </td>
                </tr>
              ))}
            </tbody>
          </table>
        </div>
      )}

      {/* Invite Modal */}
      <Modal open={inviteOpen} onClose={() => setInviteOpen(false)} title="Invite Team Member">
        <form onSubmit={handleInvite} className="space-y-4">
          <div>
            <label className="block text-sm text-text-secondary mb-1">Email</label>
            <input name="email" type="email" required placeholder="user@example.com" className="w-full bg-bg-input border border-border rounded-md px-3 py-2 text-text-primary text-sm placeholder:text-text-muted focus:outline-none focus:border-accent" />
          </div>
          <div>
            <label className="block text-sm text-text-secondary mb-1">Role</label>
            <select name="role" defaultValue="developer" className="w-full bg-bg-input border border-border rounded-md px-3 py-2 text-text-primary text-sm focus:outline-none focus:border-accent">
              {ROLES.map((r) => (<option key={r} value={r}>{r}</option>))}
            </select>
          </div>
          <div className="flex justify-end gap-3 pt-2">
            <button type="button" onClick={() => setInviteOpen(false)} className="px-4 py-2 text-sm text-text-secondary hover:text-text-primary transition-colors">Cancel</button>
            <button type="submit" disabled={submitting} className="px-4 py-2 bg-accent text-white rounded-md text-sm font-medium hover:bg-accent-hover transition-colors disabled:opacity-50">
              {submitting ? 'Sending...' : 'Send Invite'}
            </button>
          </div>
        </form>
      </Modal>

      <ConfirmDialog
        open={!!removeTarget}
        onClose={() => setRemoveTarget(null)}
        onConfirm={removeMember}
        title="Remove Member"
        message="Are you sure you want to remove this team member? This action cannot be undone."
        loading={submitting}
      />
    </div>
  );
}
