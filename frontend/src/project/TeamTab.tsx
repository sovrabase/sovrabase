import { useState, useEffect } from 'react';
import { UserPlus, Trash2, Loader2, Users, Copy, Check, Link2, Shield } from 'lucide-react';
import { api, formatDate } from '../api';
import type { TeamMember } from '../types';
import Modal from '../components/Modal';
import { ConfirmDialog } from '../components/ConfirmDialog';
import { useToast } from '../components/Toast';

interface Props {
  projectId: string;
  apiKey?: string;
}

const ROLES = ['owner', 'admin', 'developer', 'viewer'] as const;
type Role = (typeof ROLES)[number];

export default function TeamTab({ projectId }: Props) {
  const { showToast } = useToast();
  const [members, setMembers] = useState<TeamMember[]>([]);
  const [loading, setLoading] = useState(true);
  const [inviteOpen, setInviteOpen] = useState(false);
  const [removeTarget, setRemoveTarget] = useState<string | null>(null);
  const [submitting, setSubmitting] = useState(false);
  const [inviteLink, setInviteLink] = useState<string | null>(null);
  const [copied, setCopied] = useState(false);
  const [currentUserRole, setCurrentUserRole] = useState<Role | null>(null);
  const [currentUserId, setCurrentUserId] = useState<string | null>(null);
  const [isCurrentUserOwner, setIsCurrentUserOwner] = useState(false);
  const currentAdminRole = localStorage.getItem('sovrabase_admin_role');

  const loadMembers = () => {
    setLoading(true);
    api<{ members: TeamMember[] }>(`/admin/projects/${encodeURIComponent(projectId)}/members`)
      .then((res) => {
        setMembers(res.members || []);
        // Find current user from member list
        const currentUserId = localStorage.getItem('sovrabase_admin_user_id');
        const currentUser = res.members?.find(m => m.user_id === currentUserId);
        if (currentUser) {
          setCurrentUserRole(currentUser.role);
          setCurrentUserId(currentUser.user_id);
          setIsCurrentUserOwner(currentUser.is_owner || currentUser.role === 'owner');
        }
        // Admins can manage all projects regardless of membership
        if (!currentUser && currentAdminRole === 'admin') {
          setIsCurrentUserOwner(true);
        }
      })
      .finally(() => setLoading(false));
  };

  useEffect(() => { loadMembers(); }, [projectId]);

  const changeRole = async (userId: string, role: Role) => {
    const member = members.find(m => m.user_id === userId);
    if (member?.is_owner) {
      showToast('Cannot change owner role', 'error');
      return;
    }
    // Only owners can change roles
    if (!isCurrentUserOwner) {
      showToast('Only owners can change roles', 'error');
      return;
    }
    try {
      await api(`/admin/projects/${encodeURIComponent(projectId)}/members/${encodeURIComponent(userId)}/role`, {
        method: 'PUT',
        body: JSON.stringify({ role }),
      });
      setMembers((prev) => prev.map((m) => (m.user_id === userId ? { ...m, role } : m)));
      showToast('Role updated', 'success');
    } catch (e) {
      showToast((e as Error).message, 'error');
    }
  };

  const removeMember = async () => {
    if (!removeTarget) return;

    // Check if trying to remove yourself
    if (removeTarget === currentUserId) {
      showToast('You cannot remove yourself', 'error');
      return;
    }

    // Only owners can remove members
    if (!isCurrentUserOwner) {
      showToast('Only owners can remove members', 'error');
      return;
    }

    setSubmitting(true);
    try {
      await api(`/admin/projects/${encodeURIComponent(projectId)}/members/${encodeURIComponent(removeTarget)}`, { method: 'DELETE' });
      setMembers((prev) => prev.filter((m) => m.user_id !== removeTarget));
      showToast('Member removed', 'success');
    } catch (e) {
      showToast((e as Error).message, 'error');
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
      const res = await api<{ invitation: unknown; invite_link: string }>(`/admin/projects/${encodeURIComponent(projectId)}/invite`, {
        method: 'POST',
        body: JSON.stringify({ email, role }),
      });
      setInviteLink(res.invite_link);
      showToast('Invitation created', 'success');
      loadMembers();
    } catch (e) {
      showToast((e as Error).message, 'error');
    } finally {
      setSubmitting(false);
    }
  };

  const copyLink = async () => {
    if (!inviteLink) return;
    try {
      await navigator.clipboard.writeText(inviteLink);
      setCopied(true);
      setTimeout(() => setCopied(false), 2000);
    } catch {
      showToast('Failed to copy', 'error');
    }
  };

  if (loading) {
    return <div className="flex items-center gap-2 py-10 text-text-muted"><Loader2 className="w-4 h-4 animate-spin" /> Loading team...</div>;
  }

  return (
    <div className="space-y-4">
      <div className="flex items-center justify-between">
        <h2 className="text-lg font-semibold text-text-primary">Team Members ({members.length})</h2>
        {isCurrentUserOwner && (
          <button onClick={() => setInviteOpen(true)} className="flex items-center gap-2 px-4 py-2 bg-accent text-white rounded-md hover:bg-accent-hover transition-colors text-sm font-medium">
            <UserPlus className="w-4 h-4" /> Invite Member
          </button>
        )}
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
                <th className="text-left px-4 py-3 font-medium">User</th>
                <th className="text-left px-4 py-3 font-medium">Role</th>
                <th className="text-left px-4 py-3 font-medium">Joined</th>
                <th className="text-right px-4 py-3 font-medium">Actions</th>
              </tr>
            </thead>
            <tbody className="divide-y divide-border">
              {members.map((m) => (
                <tr key={m.user_id} className="hover:bg-bg-input/50 transition-colors">
                  <td className="px-4 py-3 text-text-primary text-sm">
                    <div className="flex items-center gap-2">
                      {m.email || m.user_id.slice(0, 12) + '…'}
                      {m.is_owner && (
                        <span className="inline-flex items-center gap-1 px-2 py-0.5 bg-accent/10 text-accent text-xs font-medium rounded-full">
                          <Shield className="w-3 h-3" />
                          Owner
                        </span>
                      )}
                    </div>
                  </td>
                  <td className="px-4 py-3">
                    <select
                      value={m.role}
                      onChange={(e) => changeRole(m.user_id, e.target.value as Role)}
                      disabled={!isCurrentUserOwner || m.is_owner}
                      className={`bg-bg-input border border-border rounded px-2 py-1 text-text-primary text-xs ${
                        !isCurrentUserOwner || m.is_owner ? 'opacity-50 cursor-not-allowed' : ''
                      }`}
                    >
                      {ROLES.map((r) => (
                        <option key={r} value={r}>{r}</option>
                      ))}
                    </select>
                  </td>
                  <td className="px-4 py-3 text-text-secondary text-xs">{formatDate(m.joined_at)}</td>
                  <td className="px-4 py-3 text-right">
                    <button
                      onClick={() => setRemoveTarget(m.user_id)}
                      disabled={!isCurrentUserOwner || m.is_owner}
                      className={`p-1.5 rounded transition-colors ${
                        !isCurrentUserOwner || m.is_owner
                          ? 'opacity-30 cursor-not-allowed text-text-muted'
                          : 'hover:bg-danger/10 text-text-secondary hover:text-danger'
                      }`}
                      title={m.is_owner ? 'Cannot remove owner' : (!isCurrentUserOwner ? 'Only owners can remove members' : 'Remove')}
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

      {/* Invite Modal */}
      <Modal isOpen={inviteOpen} onClose={() => { setInviteOpen(false); setInviteLink(null); }} title={inviteLink ? 'Invite Created' : 'Invite Team Member'}>
        {inviteLink ? (
          <div className="space-y-4">
            <p className="text-text-muted text-sm">Share this link with the invitee. They must be logged in to accept the invitation.</p>
            <div className="flex items-center gap-2 bg-bg-input border border-border rounded-lg px-3 py-2.5">
              <Link2 className="w-4 h-4 text-text-muted shrink-0" />
              <code className="flex-1 text-text-primary text-xs font-mono break-all">{inviteLink}</code>
              <button onClick={copyLink} className="shrink-0 p-1.5 rounded text-text-muted hover:text-accent hover:bg-accent/10 transition-colors" title="Copy link">
                {copied ? <Check className="w-4 h-4 text-success" /> : <Copy className="w-4 h-4" />}
              </button>
            </div>
            <div className="flex justify-end">
              <button onClick={() => { setInviteOpen(false); setInviteLink(null); }} className="px-4 py-2 bg-accent text-white rounded-md text-sm font-medium hover:bg-accent-hover">Done</button>
            </div>
          </div>
        ) : (
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
                {submitting ? 'Sending...' : 'Create Invite'}
              </button>
            </div>
          </form>
        )}
      </Modal>

      <ConfirmDialog
        isOpen={!!removeTarget}
        onCancel={() => setRemoveTarget(null)}
        onConfirm={removeMember}
        title="Remove Team Member"
        message="Are you sure? This cannot be undone."
      />
      {/* Show message if not owner */}
      {!isCurrentUserOwner && currentUserId && (
        <div className="text-xs text-text-muted mt-2">
          You are viewing as <span className="font-medium text-text-primary">{currentUserRole}</span>. Only owners can manage team members.
        </div>
      )}
    </div>
  );
}
