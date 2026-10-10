import { useEffect, useMemo, useState } from 'react';
import { useNavigate, useSearchParams } from 'react-router-dom';
import { FolderKanban, Plus } from 'lucide-react';
import { adminProjectPath, api, formatBytes } from '../../api';
import { useAuth, useDashboard, useProjects } from '../../store';
import type { Project } from '../../types';
import { ProjectAvatar } from '../../shell/AppShell';
import { projectPath } from '../../shell/nav';
import {
  Banner,
  Button,
  Card,
  DataTable,
  EmptyState,
  Page,
  PageHeader,
  SearchInput,
  Skeleton,
  useToast,
  type Column,
} from '../../ui';
import { HealthStrip, type InstanceStats } from './projects/HealthStrip';
import { NewProjectSheet } from './projects/NewProjectSheet';

interface ProjectUsage {
  enabled?: boolean;
  api_requests?: number;
  total_storage_bytes?: number;
}

function shortDate(s?: string) {
  if (!s) return '—';
  const d = new Date(s);
  return Number.isNaN(d.getTime()) ? s : d.toLocaleDateString(undefined, { year: 'numeric', month: 'short', day: 'numeric' });
}

export default function Projects() {
  const { projects, loading, error, loadProjects, createProject } = useProjects();
  const { stats, usage, replication, loading: statsLoading, error: statsError, loadDashboard } = useDashboard();
  const { role } = useAuth();
  const { showToast } = useToast();
  const navigate = useNavigate();
  const [params, setParams] = useSearchParams();
  const [query, setQuery] = useState('');
  const [usageById, setUsageById] = useState<Record<string, ProjectUsage>>({});

  const canCreate = role !== 'support' && role !== 'member';
  const sheetOpen = params.get('new') === '1';
  const setSheetOpen = (open: boolean) => {
    const next = new URLSearchParams(params);
    if (open) next.set('new', '1');
    else next.delete('new');
    setParams(next, { replace: true });
  };

  useEffect(() => {
    loadProjects();
    loadDashboard();
  }, [loadProjects, loadDashboard]);

  // Per-project usage, best effort (metering may be off).
  const ids = projects.map((p) => p.id).join(',');
  useEffect(() => {
    if (!ids) return;
    let alive = true;
    const list = ids.split(',').slice(0, 50);
    Promise.all(
      list.map((id) =>
        api<ProjectUsage>(adminProjectPath(id, '/usage'))
          .then((u) => [id, u] as const)
          .catch(() => [id, {} as ProjectUsage] as const),
      ),
    ).then((rows) => alive && setUsageById(Object.fromEntries(rows)));
    return () => {
      alive = false;
    };
  }, [ids]);

  const filtered = useMemo(() => {
    const q = query.trim().toLowerCase();
    const list = [...projects].sort((a, b) => a.name.localeCompare(b.name));
    if (!q) return list;
    return list.filter((p) => p.name.toLowerCase().includes(q) || p.id.toLowerCase().includes(q));
  }, [projects, query]);

  const hasUsage = Object.values(usageById).some((u) => u.enabled);

  const create = async (name: string) => {
    const p = await createProject(name);
    showToast('Project created', 'success');
    loadDashboard();
    navigate(projectPath(p.id));
  };

  const columns: Column<Project>[] = [
    {
      key: 'name',
      header: 'Project',
      width: 'minmax(0,2.2fr)',
      cell: (p) => (
        <div className="flex items-center gap-3 min-w-0">
          <ProjectAvatar name={p.name} size={28} />
          <div className="flex flex-col min-w-0">
            <span className="font-medium text-ink truncate">{p.name}</span>
            <span className="font-mono text-[12px] text-ink-3 truncate">{p.id}</span>
          </div>
        </div>
      ),
    },
    ...(hasUsage
      ? [
          {
            key: 'requests',
            header: 'Requests',
            align: 'right' as const,
            cell: (p: Project) => {
              const u = usageById[p.id];
              return <span className="tabular-nums text-ink-2">{u?.enabled ? (u.api_requests ?? 0).toLocaleString() : '—'}</span>;
            },
          },
          {
            key: 'storage',
            header: 'Storage',
            align: 'right' as const,
            cell: (p: Project) => {
              const u = usageById[p.id];
              return <span className="tabular-nums text-ink-2">{u?.enabled ? formatBytes(u.total_storage_bytes ?? 0) : '—'}</span>;
            },
          },
        ]
      : []),
    {
      key: 'collections',
      header: 'Collections',
      align: 'right',
      hideOnMobile: true,
      cell: (p) => <span className="tabular-nums text-ink-2">{p.collection_count ?? 0}</span>,
    },
    {
      key: 'members',
      header: 'Members',
      align: 'right',
      hideOnMobile: true,
      cell: (p) => <span className="tabular-nums text-ink-2">{p.member_count ?? 0}</span>,
    },
    {
      key: 'created',
      header: 'Created',
      width: '130px',
      cell: (p) => <span className="text-ink-3 text-[13px]">{shortDate(p.created_at)}</span>,
    },
  ];

  return (
    <Page>
      <PageHeader
        title="Projects"
        description="Each project has its own database, users, files and keys."
        actions={
          canCreate && (
            <Button variant="primary" icon={<Plus className="w-4 h-4" />} onClick={() => setSheetOpen(true)}>
              New project
            </Button>
          )
        }
      />

      <HealthStrip
        stats={stats as InstanceStats | null}
        usage={usage}
        replication={replication}
        loading={statsLoading}
        error={statsError}
      />

      {error && (
        <div className="mb-4">
          <Banner tone="danger" title="Could not load projects" action={<Button size="sm" onClick={loadProjects}>Retry</Button>}>
            {error}
          </Banner>
        </div>
      )}

      <Card padded={false}>
        <div className="flex flex-wrap items-center gap-3 px-5 py-4 border-b border-line">
          <h2 className="m-0 text-[15px] font-semibold text-ink mr-auto">
            All projects {projects.length > 0 && <span className="text-ink-3 font-normal">{projects.length}</span>}
          </h2>
          {projects.length > 0 && (
            <SearchInput value={query} onChange={setQuery} placeholder="Search by name or id" className="w-full sm:w-[260px]" />
          )}
        </div>
        {loading && !projects.length ? (
          <div className="flex flex-col gap-3 p-5">
            {Array.from({ length: 3 }, (_, i) => (
              <Skeleton key={i} className="h-10 w-full" />
            ))}
          </div>
        ) : (
          <DataTable
            columns={columns}
            rows={filtered}
            rowKey={(p) => p.id}
            onRowClick={(p) => navigate(projectPath(p.id))}
            empty={
              projects.length === 0 ? (
                <EmptyState
                  icon={<FolderKanban className="w-5 h-5" />}
                  title="No projects yet"
                  description="Create a project to get a database, sign-in, file storage and API keys."
                  action={
                    canCreate && (
                      <Button variant="primary" icon={<Plus className="w-4 h-4" />} onClick={() => setSheetOpen(true)}>
                        New project
                      </Button>
                    )
                  }
                />
              ) : (
                <EmptyState title="No matches" description={`No project name or id contains "${query}".`} />
              )
            }
          />
        )}
      </Card>

      <NewProjectSheet open={sheetOpen} onClose={() => setSheetOpen(false)} onCreate={create} />
    </Page>
  );
}
