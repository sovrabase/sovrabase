import { useCallback, useEffect, useState } from 'react';
import { UserPlus } from 'lucide-react';
import { api, adminProjectPath, formatDate } from '../../../api';
import type { TeamMember } from '../../../types';
import { Badge, Banner, Button, Card, CardHeader, ConfirmDialog, CopyButton, EmptyState, Field, Input, Row, Select, Sheet, Spinner, useToast } from '../../../ui';

const ROLES: Array<{ value: TeamMember['role']; label: string }> = [
  { value: 'owner', label: 'Owner' },
  { value: 'admin', label: 'Admin' },
  { value: 'developer', label: 'Developer' },
  { value: 'viewer', label: 'Viewer' },
];

function initials(s: string) {
  const base = s.split('@')[0];
  const parts = base.split(/[._\-\s]+/).filter(Boolean);
  return ((parts[0]?.[0] ?? '?') + (parts[1]?.[0] ?? '')).toUpperCase();
}

export function TeamSection({ projectId }: { projectId: string }) {
  const { showToast } = useToast();
  const [members, setMembers] = useState<TeamMember[] | null>(null);
  const [error, setError] = useState<string | null>(null);
  const [inviteOpen, setInviteOpen] = useState(false);
  const [email, setEmail] = useState('');
  const [role, setRole] = useState<TeamMember['role']>('developer');
  const [sending, setSending] = useState(false);
  const [invite, setInvite] = useState<{ link: string; emailSent: boolean } | null>(null);
  const [removeTarget, setRemoveTarget] = useState<TeamMember | null>(null);

  const myId = localStorage.getItem('sovrabase_admin_user_id');
  const adminRole = localStorage.getItem('sovrabase_admin_role');

  const load = useCallback(async () => {
    try {
      const d = await api<{ members: TeamMember[] }>(adminProjectPath(projectId, '/members'));
      setMembers(d.members || []);
      setError(null);
    } catch (e) {
      setError((e as Error).message);
    }
  }, [projectId]);

  useEffect(() => {
    load();
  }, [load]);

  if (error) return <Banner tone="danger" title="The team could not be loaded">{error}</Banner>;
  if (!members) return <Spinner label="Loading team" />;

  const me = members.find((m) => m.user_id === myId);
  // Instance admins manage every project, even without a membership.
  const canManage = !!(me && (me.is_owner || me.role === 'owner')) || (!me && (adminRole === 'admin' || adminRole === 'super_admin'));

  const changeRole = async (m: TeamMember, r: TeamMember['role']) => {
    try {
      await api(adminProjectPath(projectId, `/members/${encodeURIComponent(m.user_id)}/role`), { method: 'PUT', body: JSON.stringify({ role: r }) });
      setMembers((ms) => (ms ?? []).map((x) => (x.user_id === m.user_id ? { ...x, role: r } : x)));
      showToast('Role updated', 'success');
    } catch (e) {
      showToast((e as Error).message, 'error');
    }
  };

  const remove = async () => {
    if (!removeTarget) return;
    try {
      await api(adminProjectPath(projectId, `/members/${encodeURIComponent(removeTarget.user_id)}`), { method: 'DELETE' });
      setMembers((ms) => (ms ?? []).filter((x) => x.user_id !== removeTarget.user_id));
      showToast('Member removed', 'success');
    } catch (e) {
      showToast((e as Error).message, 'error');
    }
  };

  const sendInvite = async () => {
    if (!email.trim()) return;
    setSending(true);
    try {
      const res = await api<{ invite_link: string; email_sent?: boolean }>(adminProjectPath(projectId, '/invite'), {
        method: 'POST',
        body: JSON.stringify({ email: email.trim(), role }),
      });
      setInvite({ link: res.invite_link, emailSent: !!res.email_sent });
      showToast('Invitation created', 'success');
      load();
    } catch (e) {
      showToast((e as Error).message, 'error');
    } finally {
      setSending(false);
    }
  };

  const closeInvite = () => {
    setInviteOpen(false);
    setInvite(null);
    setEmail('');
    setRole('developer');
  };

  return (
    <Card padded={false}>
      <CardHeader
        title="Team"
        description="Who can open this project in the console. Instance admins see every project."
        actions={
          canManage && (
            <Button size="sm" variant="primary" icon={<UserPlus className="w-3.5 h-3.5" />} onClick={() => setInviteOpen(true)}>
              Invite
            </Button>
          )
        }
      />
      <div className="border-t border-line">
        {members.length === 0 ? (
          <EmptyState title="No members yet" description="Invite someone by email to work on this project with you." />
        ) : (
          members.map((m) => {
            const label = m.email || `${m.user_id.slice(0, 12)}…`;
            const locked = !canManage || !!m.is_owner;
            return (
              <Row key={m.user_id}>
                <span className="w-8 h-8 rounded-full bg-accent-soft text-accent-2 text-[12px] font-semibold inline-flex items-center justify-center shrink-0" aria-hidden>
                  {initials(label)}
                </span>
                <div className="flex-1 min-w-[160px]">
                  <p className="m-0 text-sm text-ink font-medium truncate">
                    {label}
                    {m.user_id === myId && <span className="text-ink-3 font-normal"> (you)</span>}
                  </p>
                  <p className="m-0 text-[12.5px] text-ink-3">Joined {formatDate(m.joined_at)}</p>
                </div>
                {m.is_owner ? (
                  <Badge tone="accent">Owner</Badge>
                ) : (
                  <div className="flex items-center gap-2">
                    <Select
                      aria-label={`Role for ${label}`}
                      value={m.role}
                      disabled={locked}
                      onChange={(e) => changeRole(m, e.target.value as TeamMember['role'])}
                      options={ROLES.map((r) => ({ value: r.value, label: r.label }))}
                      className="w-[140px] h-[30px] text-[13px]"
                    />
                    {canManage && m.user_id !== myId && (
                      <Button size="sm" variant="ghost" onClick={() => setRemoveTarget(m)}>
                        Remove
                      </Button>
                    )}
                  </div>
                )}
              </Row>
            );
          })
        )}
      </div>
      {!canManage && me && (
        <p className="m-0 px-5 py-3 border-t border-line text-[12.5px] text-ink-3">
          You are a {me.role} on this project. Only owners can change the team.
        </p>
      )}

      <Sheet
        open={inviteOpen}
        onClose={closeInvite}
        title={invite ? 'Invitation ready' : 'Invite to the project'}
        footer={
          invite ? (
            <Button variant="primary" onClick={closeInvite}>
              Done
            </Button>
          ) : (
            <>
              <Button variant="ghost" onClick={closeInvite}>
                Cancel
              </Button>
              <Button variant="primary" loading={sending} disabled={!email.trim()} onClick={sendInvite}>
                Create invitation
              </Button>
            </>
          )
        }
      >
        {invite ? (
          <>
            <Banner tone={invite.emailSent ? 'ok' : 'neutral'}>
              {invite.emailSent ? `An email is on its way to ${email}.` : 'No email was sent (email is not set up). Share this link; they need to sign in to accept.'}
            </Banner>
            <Field label="Invitation link">
              <div className="flex items-center gap-2">
                <Input mono readOnly value={invite.link} className="bg-sunken" />
                <CopyButton text={invite.link} />
              </div>
            </Field>
          </>
        ) : (
          <>
            <Field label="Email">
              <Input type="email" autoComplete="off" value={email} onChange={(e) => setEmail(e.target.value)} placeholder="name@example.com" onKeyDown={(e) => e.key === 'Enter' && sendInvite()} />
            </Field>
            <Field label="Role" hint="Owners manage the team. Viewers can only look.">
              <Select value={role} onChange={(e) => setRole(e.target.value as TeamMember['role'])} options={ROLES.map((r) => ({ value: r.value, label: r.label }))} />
            </Field>
          </>
        )}
      </Sheet>

      <ConfirmDialog
        open={!!removeTarget}
        onClose={() => setRemoveTarget(null)}
        onConfirm={remove}
        title={`Remove ${removeTarget?.email || 'this member'}?`}
        confirmLabel="Remove"
      >
        They lose access to this project right away. You can invite them again later.
      </ConfirmDialog>
    </Card>
  );
}
