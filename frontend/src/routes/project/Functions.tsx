import { useCallback, useEffect, useState } from 'react';
import { Link, useNavigate, useParams } from 'react-router-dom';
import { ArrowLeft, Plus, SquareFunction } from 'lucide-react';
import { api, adminProjectPath, formatDate } from '../../api';
import type { ServerlessFunction } from '../../types';
import { useProject } from '../../shell/ProjectContext';
import { projectPath } from '../../shell/nav';
import { Badge, Banner, Button, Card, DataTable, EmptyState, Page, PageHeader, SearchInput, Spinner, cx, type Column } from '../../ui';
import { FunctionDetail } from './functions/FunctionDetail';
import { CreateFunctionSheet } from './functions/CreateFunctionSheet';
import { languageLabel, triggerSummary } from './functions/shared';

export default function ProjectFunctions() {
  const { project, projectId } = useProject();
  const { fn: fnParam } = useParams<{ fn?: string }>();
  const navigate = useNavigate();
  const [fns, setFns] = useState<ServerlessFunction[] | null>(null);
  const [error, setError] = useState<string | null>(null);
  const [creating, setCreating] = useState(false);
  const [search, setSearch] = useState('');

  const load = useCallback(async () => {
    try {
      const res = await api<{ data: ServerlessFunction[] }>(adminProjectPath(projectId, '/functions'));
      setFns(res.data || []);
      setError(null);
    } catch (e) {
      setError((e as Error).message);
      setFns((f) => f ?? []);
    }
  }, [projectId]);

  useEffect(() => {
    load();
  }, [load]);

  const fnUrl = (name: string) => projectPath(projectId, `functions/${encodeURIComponent(name)}`);
  const selected = fnParam && fns ? fns.find((f) => f.name === fnParam || f.id === fnParam) : undefined;

  const createSheet = (
    <CreateFunctionSheet
      open={creating}
      onClose={() => setCreating(false)}
      projectId={projectId}
      existing={fns ?? []}
      onCreated={async (name) => {
        setCreating(false);
        await load();
        navigate(fnUrl(name));
      }}
    />
  );

  if (fns === null) return <Spinner label="Loading functions" />;

  /* ---------- Detail ---------- */
  if (fnParam) {
    if (!selected) {
      return (
        <Page>
          <EmptyState
            icon={<SquareFunction className="w-5 h-5" />}
            title="Function not found"
            description={`There is no function called "${fnParam}" in this project.`}
            action={<Button onClick={() => navigate(projectPath(projectId, 'functions'))}>All functions</Button>}
          />
        </Page>
      );
    }
    return (
      <Page wide>
        <div className="grid grid-cols-1 lg:grid-cols-[220px_minmax(0,1fr)] gap-6">
          <nav aria-label="Functions" className="hidden lg:flex flex-col gap-1 self-start sticky top-4">
            <div className="flex items-center justify-between px-2 mb-1">
              <span className="text-[12px] font-semibold uppercase tracking-[.05em] text-ink-3">Functions</span>
              <button
                type="button"
                aria-label="New function"
                onClick={() => setCreating(true)}
                className="w-7 h-7 inline-flex items-center justify-center rounded-md text-ink-3 hover:text-ink hover:bg-sunken"
              >
                <Plus className="w-4 h-4" />
              </button>
            </div>
            {fns.map((f) => (
              <Link
                key={f.id}
                to={fnUrl(f.name)}
                aria-current={f.id === selected.id ? 'page' : undefined}
                className={cx(
                  'flex flex-col px-2.5 py-2 rounded-md min-w-0',
                  f.id === selected.id ? 'bg-accent-soft text-accent-2' : 'hover:bg-sunken text-ink',
                )}
              >
                <span className="font-mono text-[13px] truncate flex items-center gap-1.5">
                  <span className={cx('w-1.5 h-1.5 rounded-full shrink-0', f.enabled ? 'bg-ok' : 'bg-line-2')} aria-hidden />
                  {f.name}
                </span>
                <span className="text-[12px] text-ink-3 truncate pl-3">
                  {triggerSummary(f)} · {languageLabel(f.language)}
                </span>
              </Link>
            ))}
          </nav>
          <div className="min-w-0">
            <Link to={projectPath(projectId, 'functions')} className="lg:hidden inline-flex items-center gap-1.5 mb-3 text-[13px] text-ink-3 hover:text-ink">
              <ArrowLeft className="w-4 h-4" aria-hidden /> All functions
            </Link>
            {error && (
              <div className="mb-4">
                <Banner tone="danger">{error}</Banner>
              </div>
            )}
            <FunctionDetail
              key={selected.id}
              projectId={projectId}
              fn={selected}
              apiKey={project.api_key}
              onChanged={load}
              onDeleted={() => {
                navigate(projectPath(projectId, 'functions'));
                load();
              }}
            />
          </div>
        </div>
        {createSheet}
      </Page>
    );
  }

  /* ---------- List ---------- */
  const q = search.trim().toLowerCase();
  const rows = q ? fns.filter((f) => f.name.toLowerCase().includes(q) || f.trigger.includes(q)) : fns;
  const columns: Column<ServerlessFunction>[] = [
    { key: 'name', header: 'Name', width: 'minmax(0,1.4fr)', cell: (f) => <span className="font-mono text-[13.5px] text-ink truncate block">{f.name}</span> },
    {
      key: 'state',
      header: 'State',
      width: '90px',
      cell: (f) => (f.enabled ? <Badge tone="ok" dot>On</Badge> : <Badge dot>Off</Badge>),
    },
    { key: 'trigger', header: 'Runs when', width: 'minmax(0,1.4fr)', cell: (f) => <span className="text-[13px] text-ink-2 truncate block">{triggerSummary(f)}</span> },
    { key: 'lang', header: 'Language', width: '110px', cell: (f) => <span className="text-[13px] text-ink-2">{languageLabel(f.language)}</span> },
    { key: 'access', header: 'Access', width: '100px', hideOnMobile: true, cell: (f) => (f.trigger === 'http' ? <span className="text-[13px] text-ink-2">{f.public ? 'Public' : 'Signed in'}</span> : <span className="text-ink-3">—</span>) },
    { key: 'updated', header: 'Updated', width: '160px', hideOnMobile: true, cell: (f) => <span className="text-[12.5px] text-ink-3">{formatDate(f.updated_at || f.created_at)}</span> },
  ];

  return (
    <Page>
      <PageHeader
        title="Functions"
        description="Server-side code that runs on a request, a data change, a sign-up or a schedule."
        actions={
          <Button variant="primary" icon={<Plus className="w-4 h-4" />} onClick={() => setCreating(true)}>
            New function
          </Button>
        }
      />
      {error && (
        <div className="mb-4">
          <Banner tone="danger" title="Functions could not be loaded" action={<Button size="sm" onClick={load}>Try again</Button>}>
            {error}
          </Banner>
        </div>
      )}
      {fns.length > 0 && (
        <div className="flex flex-wrap items-center gap-2 mb-3">
          <SearchInput value={search} onChange={setSearch} placeholder="Search functions" className="flex-[1_1_240px] max-w-[360px]" />
        </div>
      )}
      <Card padded={false}>
        <DataTable
          columns={columns}
          rows={rows}
          rowKey={(f) => f.id}
          onRowClick={(f) => navigate(fnUrl(f.name))}
          empty={
            fns.length === 0 ? (
              <EmptyState
                icon={<SquareFunction className="w-5 h-5" />}
                title="No functions yet"
                description="Write a few lines of JavaScript or Go and call them over HTTP, on a schedule or when data changes."
                action={
                  <Button variant="primary" onClick={() => setCreating(true)}>
                    New function
                  </Button>
                }
              />
            ) : (
              <EmptyState title="No functions match" description="Try another search." />
            )
          }
        />
      </Card>
      {createSheet}
    </Page>
  );
}
