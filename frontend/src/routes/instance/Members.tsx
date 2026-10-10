import { useCallback, useEffect, useMemo, useState } from 'react';
import { Link } from 'react-router-dom';
import { Plus, Shield, Trash2, Users } from 'lucide-react';
import { api } from '../../api';
import { useAuth, useProjects } from '../../store';
import type { AdminUser } from '../../types';
import { projectPath } from '../../shell/nav';
import {
  Badge,
  Banner,
  Button,
  Card,
  ConfirmDialog,
  DataTable,
  EmptyState,
  IconButton,
  Page,
  PageHeader,
  SearchInput,
  Segmented,
  Select,
  Skeleton,
  useToast,
  type Column,
} from '../../ui';
import { ADMIN_ROLES, AddAdminSheet, roleLabel, type NewAdmin } from './members/AdminSheet';

interface GlobalMember {
  user_id: string;
  email: string;
  name?: string;
  is_admin?: boolean;
  admin_role?: string;
  project_ids?: string[];
  project_count?: number;
}

type Admin = AdminUser & { name?: string };
type View = 'admins' | 'people';

function shortDate(s?: string) {
  if (!s) return '—';
  const d = new Date(s);
  return Number.isNaN(d.getTime()) ? s : d.toLocaleDateString(undefined, { year: 'numeric', month: 'short', day: 'numeric' });
}

export default function Members() {
  const { role } = useAuth();
  const isSuper = role === 'super_admin';
  const { showToast } = useToast();
  const { projects, loadProjects } = useProjects();
  const myId = localStorage.getItem('sovrabase_admin_user_id');

  const [view, setView] = useState<View>(isSuper ? 'people' : 'admins');
  const [query, setQuery] = useState('');

  const [admins, setAdmins] = useState<Admin[]>([]);
  const [adminsLoading, setAdminsLoading] = useState(true);
  const [adminsError, setAdminsError] = useState<string | null>(null);
  const [members, setMembers] = useState<GlobalMember[]>([]);
  const [membersLoading, setMembersLoading] = useState(isSuper);
  const [membersError, setMembersError] = useState<string | null>(null);

  const [addOpen, setAddOpen] = useState(false);
  const [removeAdmin, setRemoveAdmin] = useState<Admin | null>(null);
  const [removeMember, setRemoveMember] = useState<GlobalMember | null>(null);

  const loadAdmins = useCallback(async () => {
    setAdminsLoading(true);
    setAdminsError(null);
    try {
      const d = await api<{ admins: Admin[] }>('/admin/admins');
      setAdmins(d.admins || []);
    } catch (e) {
      setAdminsError((e as Error).message);
    } finally {
      setAdminsLoading(false);
    }
  }, []);

  const loadMembers = useCallback(async () => {
    if (!isSuper) return;
    setMembersLoading(true);
    setMembersError(null);
    try {
      const d = await api<{ members: GlobalMember[] }>('/admin/members');
      setMembers(d.members || []);
    } catch (e) {
      setMembersError((e as Error).message);
    } finally {
      setMembersLoading(false);
    }
  }, [isSuper]);

  useEffect(() => {
    loadAdmins();
    loadMembers();
  }, [loadAdmins, loadMembers]);

  const noProjects = projects.length === 0;
  useEffect(() => {
    if (noProjects) loadProjects();
  }, [noProjects, loadProjects]);

  const projectName = useMemo(() => Object.fromEntries(projects.map((p) => [p.id, p.name])), [projects]);

  const addAdmin = async (a: NewAdmin) => {
    await api('/admin/admins', {
      method: 'POST',
      body: JSON.stringify({ email: a.email.trim(), password: a.password, role: a.role, name: a.name.trim() || undefined }),
    });
    showToast('Admin added', 'success');
    setAddOpen(false);
    loadAdmins();
  };

  const changeRole = async (a: Admin, next: string) => {
    try {
      await api(`/admin/admins/${encodeURIComponent(a.id)}/role`, { method: 'PUT', body: JSON.stringify({ role: next }) });
      setAdmins((list) => list.map((x) => (x.id === a.id ? { ...x, role: next } : x)));
      showToast(`${a.email} is now ${roleLabel(next).toLowerCase()}`, 'success');
    } catch (e) {
      showToast((e as Error).message, 'error');
    }
  };

  const doRemoveAdmin = async () => {
    if (!removeAdmin) return;
    try {
      await api(`/admin/admins/${encodeURIComponent(removeAdmin.id)}`, { method: 'DELETE' });
      showToast('Admin removed', 'success');
      loadAdmins();
    } catch (e) {
      showToast((e as Error).message, 'error');
    }
  };

  const doRemoveMember = async () => {
    if (!removeMember) return;
    try {
      await api(`/admin/members/${encodeURIComponent(removeMember.email)}`, { method: 'DELETE' });
      showToast('Member removed from every project', 'success');
      loadMembers();
    } catch (e) {
      showToast((e as Error).message, 'error');
    }
  };

  const q = query.trim().toLowerCase();
  const shownAdmins = admins.filter((a) => !q || a.email.toLowerCase().includes(q) || (a.name ?? '').toLowerCase().includes(q));
  const shownMembers = members.filter(
    (m) => !q || (m.email ?? '').toLowerCase().includes(q) || (m.name ?? '').toLowerCase().includes(q) || m.user_id.includes(q),
  );

  const adminCols: Column<Admin>[] = [
    {
      key: 'person',
      header: 'Person',
      width: 'minmax(0,2fr)',
      cell: (a) => (
        <div className="flex flex-col min-w-0">
          <span className="font-medium text-ink truncate">
            {a.name || a.email}
            {a.id === myId && <span className="ml-2 text-[12px] font-normal text-ink-3">you</span>}
          </span>
          {a.name && <span className="text-[12.5px] text-ink-3 truncate">{a.email}</span>}
        </div>
      ),
    },
    {
      key: 'role',
      header: 'Instance role',
      width: '180px',
      cell: (a) =>
        isSuper && a.id !== myId ? (
          <Select
            aria-label={`Role of ${a.email}`}
            value={a.role}
            onChange={(e) => changeRole(a, e.target.value)}
            options={ADMIN_ROLES}
          />
        ) : (
          <Badge tone={a.role === 'super_admin' ? 'accent' : 'neutral'}>{roleLabel(a.role)}</Badge>
        ),
    },
    { key: 'id', header: 'ID', hideOnMobile: true, cell: (a) => <span className="font-mono text-[12px] text-ink-3 truncate block">{a.id}</span> },
    { key: 'created', header: 'Added', width: '120px', cell: (a) => <span className="text-[13px] text-ink-3">{shortDate(a.created_at)}</span> },
    ...(isSuper
      ? [
          {
            key: 'actions',
            header: '',
            width: '48px',
            align: 'right' as const,
            cell: (a: Admin) =>
              a.id !== myId ? (
                <IconButton label={`Remove ${a.email}`} size="sm" onClick={() => setRemoveAdmin(a)} className="hover:!text-danger">
                  <Trash2 className="w-4 h-4" />
                </IconButton>
              ) : null,
          },
        ]
      : []),
  ];

  const memberCols: Column<GlobalMember>[] = [
    {
      key: 'person',
      header: 'Person',
      width: 'minmax(0,2fr)',
      cell: (m) => (
        <div className="flex flex-col min-w-0">
          <span className="font-medium text-ink truncate inline-flex items-center gap-2">
            <span className="truncate">{m.name || m.email || m.user_id}</span>
            {m.is_admin && (
              <Badge tone="accent">
                <Shield className="w-3 h-3" aria-hidden />
                {roleLabel(m.admin_role)}
              </Badge>
            )}
          </span>
          {m.name && <span className="text-[12.5px] text-ink-3 truncate">{m.email}</span>}
        </div>
      ),
    },
    {
      key: 'projects',
      header: 'Projects',
      width: 'minmax(0,2fr)',
      cell: (m) => {
        const ids = m.project_ids ?? [];
        if (!ids.length) return <span className="text-ink-3 text-[13px]">None</span>;
        return (
          <span className="flex flex-wrap items-center gap-1.5">
            {ids.slice(0, 3).map((id) => (
              <Link
                key={id}
                to={projectPath(id)}
                onClick={(e) => e.stopPropagation()}
                className="inline-flex h-[22px] items-center px-2 rounded-full bg-sunken text-[12px] text-ink-2 hover:text-accent"
              >
                {projectName[id] ?? <span className="font-mono">{id.slice(0, 8)}</span>}
              </Link>
            ))}
            {ids.length > 3 && <span className="text-[12px] text-ink-3">+{ids.length - 3} more</span>}
          </span>
        );
      },
    },
    { key: 'id', header: 'User ID', hideOnMobile: true, cell: (m) => <span className="font-mono text-[12px] text-ink-3 truncate block">{m.user_id}</span> },
    {
      key: 'actions',
      header: '',
      width: '48px',
      align: 'right',
      cell: (m) => (
        <IconButton label={`Remove ${m.email || m.user_id} everywhere`} size="sm" onClick={() => setRemoveMember(m)} className="hover:!text-danger">
          <Trash2 className="w-4 h-4" />
        </IconButton>
      ),
    },
  ];

  const loadingNow = view === 'admins' ? adminsLoading : membersLoading;
  const errorNow = view === 'admins' ? adminsError : membersError;

  return (
    <Page>
      <PageHeader
        title="Members"
        description="People who can sign in to this console and the projects they belong to."
        actions={
          isSuper && (
            <Button variant="primary" icon={<Plus className="w-4 h-4" />} onClick={() => setAddOpen(true)}>
              Add admin
            </Button>
          )
        }
      />

      <div className="flex flex-wrap items-center gap-3 mb-4">
        {isSuper && (
          <Segmented<View>
            aria-label="Show"
            value={view}
            onChange={setView}
            options={[
              { value: 'people', label: 'Everyone', count: members.length },
              { value: 'admins', label: 'Console admins', count: admins.length },
            ]}
          />
        )}
        <SearchInput value={query} onChange={setQuery} placeholder="Search by name or email" className="w-full sm:w-[280px] sm:ml-auto" />
      </div>

      {errorNow && (
        <div className="mb-4">
          <Banner
            tone="danger"
            title="Could not load the list"
            action={
              <Button size="sm" onClick={view === 'admins' ? loadAdmins : loadMembers}>
                Retry
              </Button>
            }
          >
            {errorNow}
          </Banner>
        </div>
      )}

      <Card padded={false}>
        {loadingNow ? (
          <div className="flex flex-col gap-3 p-5">
            {Array.from({ length: 3 }, (_, i) => (
              <Skeleton key={i} className="h-10 w-full" />
            ))}
          </div>
        ) : view === 'admins' ? (
          <DataTable
            columns={adminCols}
            rows={shownAdmins}
            rowKey={(a) => a.id}
            empty={<EmptyState icon={<Shield className="w-5 h-5" />} title={q ? 'No matches' : 'No admins'} />}
          />
        ) : (
          <DataTable
            columns={memberCols}
            rows={shownMembers}
            rowKey={(m) => m.user_id}
            empty={
              <EmptyState
                icon={<Users className="w-5 h-5" />}
                title={q ? 'No matches' : 'No members yet'}
                description={q ? undefined : 'People invited to a project from its Settings > Team appear here.'}
              />
            }
          />
        )}
      </Card>

      {!isSuper && (
        <p className="mt-3 text-[12.5px] text-ink-3">Only super admins can add or remove admins and see project members.</p>
      )}

      <AddAdminSheet open={addOpen} onClose={() => setAddOpen(false)} onSubmit={addAdmin} />

      <ConfirmDialog
        open={!!removeAdmin}
        onClose={() => setRemoveAdmin(null)}
        onConfirm={doRemoveAdmin}
        title="Remove admin"
        confirmLabel="Remove admin"
        confirmText={removeAdmin?.email}
      >
        <p className="m-0">
          <b className="text-ink">{removeAdmin?.email}</b> will no longer be able to sign in to this console.
        </p>
      </ConfirmDialog>

      <ConfirmDialog
        open={!!removeMember}
        onClose={() => setRemoveMember(null)}
        onConfirm={doRemoveMember}
        title="Remove from every project"
        confirmLabel="Remove"
        confirmText={removeMember?.email || removeMember?.user_id}
      >
        <p className="m-0">
          This removes <b className="text-ink">{removeMember?.email}</b> from all projects and deletes their console credentials. It can't be undone.
        </p>
      </ConfirmDialog>
    </Page>
  );
}
