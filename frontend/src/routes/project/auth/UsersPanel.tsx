import { useEffect, useMemo, useState } from 'react';
import { Check, ShieldCheck, Users } from 'lucide-react';
import { api, adminProjectPath, formatDate } from '../../../api';
import {
  Badge,
  Button,
  Card,
  ConfirmDialog,
  DataTable,
  EmptyState,
  Field,
  Input,
  SearchInput,
  Segmented,
  Select,
  Sheet,
  useToast,
  type Column,
} from '../../../ui';
import { providerLabel } from './providers';
import { ROLES, displayName, initials, userMethods, type UserRow } from './types';

const emptyNew = { username: '', email: '', password: '', role: 'user' };

function Avatar({ u }: { u: UserRow }) {
  if (u.avatar_url)
    return <img src={u.avatar_url} alt="" className="w-8 h-8 rounded-full object-cover shrink-0 bg-sunken" referrerPolicy="no-referrer" />;
  return (
    <span className="w-8 h-8 rounded-full shrink-0 bg-accent-soft text-accent-2 text-[12px] font-semibold inline-flex items-center justify-center" aria-hidden>
      {initials(u)}
    </span>
  );
}

function methodLabel(m: string) {
  return m === 'email' ? 'Email' : providerLabel(m);
}

export function UsersPanel({
  projectId,
  users,
  setUsers,
  reload,
  createOpen,
  setCreateOpen,
}: {
  projectId: string;
  users: UserRow[];
  setUsers: (fn: (u: UserRow[]) => UserRow[]) => void;
  reload: () => Promise<void>;
  createOpen: boolean;
  setCreateOpen: (v: boolean) => void;
}) {
  const { showToast } = useToast();
  const [search, setSearch] = useState('');
  const [role, setRole] = useState('all');
  const [method, setMethod] = useState('');

  const methods = useMemo(() => Array.from(new Set(users.flatMap(userMethods))).sort(), [users]);
  const filtered = users.filter((u) => {
    const q = search.trim().toLowerCase();
    if (q && ![u.email, u.username, u.name, u.id].some((s) => (s || '').toLowerCase().includes(q))) return false;
    if (role !== 'all' && (u.role || 'user') !== role) return false;
    if (method && !userMethods(u).includes(method)) return false;
    return true;
  });

  // Create
  const [nf, setNf] = useState(emptyNew);
  const [creating, setCreating] = useState(false);
  useEffect(() => {
    if (createOpen) setNf(emptyNew);
  }, [createOpen]);
  const createValid = nf.username.trim() && /^\S+@\S+$/.test(nf.email.trim()) && nf.password.length > 0;
  const create = async () => {
    if (!createValid) return;
    setCreating(true);
    try {
      await api(adminProjectPath(projectId, '/users'), { method: 'POST', body: JSON.stringify({ ...nf, name: '', avatar_url: '' }) });
      showToast('User added', 'success');
      setCreateOpen(false);
      await reload();
    } catch (e) {
      showToast((e as Error).message || 'Could not add the user', 'error');
    } finally {
      setCreating(false);
    }
  };

  // Edit
  const [selected, setSelected] = useState<UserRow | null>(null);
  const [ef, setEf] = useState({ name: '', email: '', avatar_url: '', role: 'user' });
  const [saving, setSaving] = useState(false);
  const [newPassword, setNewPassword] = useState('');
  const [settingPw, setSettingPw] = useState(false);
  const [confirmDelete, setConfirmDelete] = useState(false);

  const openUser = (u: UserRow) => {
    setSelected(u);
    setEf({ name: u.name || u.username || '', email: u.email || '', avatar_url: u.avatar_url || '', role: u.role || 'user' });
    setNewPassword('');
  };

  const save = async () => {
    if (!selected) return;
    setSaving(true);
    try {
      await api(adminProjectPath(projectId, `/users/${encodeURIComponent(selected.id)}`), {
        method: 'PATCH',
        body: JSON.stringify({ username: ef.name, email: ef.email, name: ef.name, avatar_url: ef.avatar_url, role: ef.role }),
      });
      showToast('User updated', 'success');
      setSelected(null);
      await reload();
    } catch (e) {
      showToast((e as Error).message || 'Could not save', 'error');
    } finally {
      setSaving(false);
    }
  };

  const setPassword = async () => {
    if (!selected || !newPassword.trim()) return;
    setSettingPw(true);
    try {
      await api(adminProjectPath(projectId, `/users/${encodeURIComponent(selected.id)}/password`), {
        method: 'POST',
        body: JSON.stringify({ password: newPassword.trim() }),
      });
      showToast('Password changed', 'success');
      setNewPassword('');
    } catch (e) {
      showToast((e as Error).message || 'Could not change the password', 'error');
    } finally {
      setSettingPw(false);
    }
  };

  const remove = async () => {
    if (!selected) return;
    try {
      await api(adminProjectPath(projectId, `/users/${encodeURIComponent(selected.id)}`), { method: 'DELETE' });
      showToast('User deleted', 'success');
      const id = selected.id;
      setUsers((p) => p.filter((u) => u.id !== id));
      setSelected(null);
    } catch (e) {
      showToast((e as Error).message || 'Could not delete the user', 'error');
      throw e;
    }
  };

  const columns: Column<UserRow>[] = [
    {
      key: 'user',
      header: 'User',
      width: 'minmax(0,2.2fr)',
      cell: (u) => (
        <div className="flex items-center gap-3 min-w-0">
          <Avatar u={u} />
          <div className="min-w-0">
            <div className="text-sm font-medium text-ink truncate">{displayName(u)}</div>
            <div className="text-[12.5px] text-ink-3 truncate">{u.email}</div>
          </div>
        </div>
      ),
    },
    {
      key: 'role',
      header: 'Role',
      width: '110px',
      cell: (u) => <Badge tone={(u.role || 'user') === 'admin' ? 'accent' : 'neutral'}>{u.role || 'user'}</Badge>,
    },
    {
      key: 'method',
      header: 'Method',
      width: 'minmax(0,1fr)',
      cell: (u) => <span className="text-[13px] text-ink-2 truncate">{userMethods(u).map(methodLabel).join(', ')}</span>,
    },
    {
      key: 'status',
      header: 'Status',
      width: 'minmax(0,1fr)',
      cell: (u) => (
        <span className="inline-flex items-center gap-2 text-[12.5px]">
          {u.is_verified ? (
            <span className="inline-flex items-center gap-1 text-ok">
              <Check className="w-3.5 h-3.5" aria-hidden /> Verified
            </span>
          ) : (
            <span className="text-warn">Unverified</span>
          )}
          {u.mfa_enabled && (
            <span className="inline-flex items-center gap-1 text-ink-2" title="Two-factor on">
              <ShieldCheck className="w-3.5 h-3.5" aria-hidden /> 2FA
            </span>
          )}
        </span>
      ),
    },
    {
      key: 'created',
      header: 'Created',
      width: '150px',
      hideOnMobile: true,
      cell: (u) => <span className="text-[12.5px] text-ink-3">{formatDate(u.created_at)}</span>,
    },
  ];

  return (
    <>
      <div className="flex flex-wrap items-center gap-2 mb-3">
        <SearchInput value={search} onChange={setSearch} placeholder="Search by name, email or ID" className="flex-[1_1_240px] min-w-0" />
        <Segmented
          aria-label="Role"
          size="sm"
          value={role}
          onChange={setRole}
          options={[{ value: 'all', label: 'All roles' }, ...ROLES.map((r) => ({ value: r, label: r }))]}
        />
        {methods.length > 1 && (
          <div className="w-[160px]">
            <Select aria-label="Sign-in method" value={method} onChange={(e) => setMethod(e.target.value)}>
              <option value="">All methods</option>
              {methods.map((m) => (
                <option key={m} value={m}>
                  {methodLabel(m)}
                </option>
              ))}
            </Select>
          </div>
        )}
      </div>

      <Card padded={false}>
        <DataTable
          columns={columns}
          rows={filtered}
          rowKey={(u) => u.id}
          onRowClick={openUser}
          selectedKey={selected?.id}
          empty={
            users.length === 0 ? (
              <EmptyState
                icon={<Users className="w-5 h-5" />}
                title="No users yet"
                description="People appear here when they sign up through your app. You can also add one by hand."
                action={
                  <Button variant="primary" onClick={() => setCreateOpen(true)}>
                    Add user
                  </Button>
                }
              />
            ) : (
              <EmptyState title="No users match" description="Try another search or clear the filters." />
            )
          }
        />
      </Card>
      {filtered.length > 0 && (
        <p className="mt-2 text-[12.5px] text-ink-3">
          {filtered.length === users.length ? `${users.length} users` : `${filtered.length} of ${users.length} users`}
        </p>
      )}

      {/* Create */}
      <Sheet
        open={createOpen}
        onClose={() => setCreateOpen(false)}
        title="Add user"
        description="The user can sign in with this email and password right away."
        footer={
          <>
            <Button variant="ghost" onClick={() => setCreateOpen(false)}>
              Cancel
            </Button>
            <Button variant="primary" loading={creating} disabled={!createValid} onClick={create}>
              Add user
            </Button>
          </>
        }
      >
        <Field label="Username">
          <Input value={nf.username} onChange={(e) => setNf({ ...nf, username: e.target.value })} autoFocus />
        </Field>
        <Field label="Email">
          <Input type="email" value={nf.email} onChange={(e) => setNf({ ...nf, email: e.target.value })} />
        </Field>
        <Field label="Password">
          <Input type="password" autoComplete="new-password" value={nf.password} onChange={(e) => setNf({ ...nf, password: e.target.value })} />
        </Field>
        <Field label="Role" hint="Roles can be used in access rules.">
          <Segmented value={nf.role} onChange={(r) => setNf({ ...nf, role: r })} options={ROLES.map((r) => ({ value: r, label: r }))} aria-label="Role" />
        </Field>
      </Sheet>

      {/* Detail / edit */}
      <Sheet
        open={!!selected}
        onClose={() => setSelected(null)}
        title={selected ? displayName(selected) : ''}
        description={
          selected && (
            <span className="font-mono text-[12px]">
              {selected.id}
            </span>
          )
        }
        footer={
          <>
            <Button variant="ghost" onClick={() => setSelected(null)}>
              Cancel
            </Button>
            <Button variant="primary" loading={saving} onClick={save}>
              Save
            </Button>
          </>
        }
      >
        {selected && (
          <>
            <div className="flex flex-wrap gap-1.5">
              {selected.is_verified ? <Badge tone="ok" dot>Email verified</Badge> : <Badge tone="warn" dot>Not verified</Badge>}
              {selected.mfa_enabled && <Badge tone="accent">Two-factor on</Badge>}
              {userMethods(selected).map((m) => (
                <Badge key={m}>{methodLabel(m)}</Badge>
              ))}
              <span className="text-[12.5px] text-ink-3 self-center">Joined {formatDate(selected.created_at)}</span>
            </div>
            <Field label="Display name">
              <Input value={ef.name} onChange={(e) => setEf({ ...ef, name: e.target.value })} />
            </Field>
            <Field label="Email">
              <Input type="email" value={ef.email} onChange={(e) => setEf({ ...ef, email: e.target.value })} />
            </Field>
            <Field label="Avatar URL" optional>
              <Input mono type="url" value={ef.avatar_url} onChange={(e) => setEf({ ...ef, avatar_url: e.target.value })} placeholder="https://" />
            </Field>
            <Field label="Role" hint="Roles can be used in access rules.">
              <Segmented value={ef.role} onChange={(r) => setEf({ ...ef, role: r })} options={ROLES.map((r) => ({ value: r, label: r }))} aria-label="Role" />
            </Field>

            <div className="border-t border-line pt-5 flex flex-col gap-3">
              <h3 className="m-0 text-[15px] font-semibold text-ink">Password</h3>
              <Field label="New password" hint="Sets the password now. The user is not notified.">
                <div className="flex gap-2">
                  <Input type="password" autoComplete="new-password" value={newPassword} onChange={(e) => setNewPassword(e.target.value)} />
                  <Button onClick={setPassword} loading={settingPw} disabled={!newPassword.trim()}>
                    Set
                  </Button>
                </div>
              </Field>
            </div>

            <div className="border-t border-line pt-5 flex flex-wrap items-center gap-3">
              <div className="flex-1 min-w-[200px]">
                <h3 className="m-0 text-[15px] font-semibold text-ink">Delete user</h3>
                <p className="m-0 text-[12.5px] text-ink-3">Removes the account. This can't be undone.</p>
              </div>
              <Button variant="danger" onClick={() => setConfirmDelete(true)}>
                Delete…
              </Button>
            </div>
          </>
        )}
      </Sheet>

      <ConfirmDialog
        open={confirmDelete}
        onClose={() => setConfirmDelete(false)}
        onConfirm={remove}
        title="Delete this user?"
        confirmLabel="Delete user"
        confirmText={selected?.email || selected?.id}
      >
        <p className="m-0">The account and its linked sign-in methods are removed for good.</p>
      </ConfirmDialog>
    </>
  );
}
