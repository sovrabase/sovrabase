import { useEffect, useState } from 'react';
import { Link } from 'react-router-dom';
import { ArrowRight, Check, KeyRound } from 'lucide-react';
import { api, adminProjectPath, formatBytes, formatDate } from '../../api';
import { useProject } from '../../shell/ProjectContext';
import { projectPath } from '../../shell/nav';
import { Badge, Banner, Button, ButtonLink, Card, CardHeader, CopyButton, DataTable, EmptyState, Page, PageHeader, Skeleton, Stat, cx, useToast, type Column } from '../../ui';
import { ConnectSnippet } from './home/ConnectSnippet';
import { apiBase, timeAgo, useProjectDetail, useProjectUsage } from './home/data';

interface LogEntry {
  timestamp: string;
  method: string;
  path: string;
  status: number;
  duration: string;
  ip?: string;
}

interface DbAnalysis {
  total_size: number;
  metadata_size: number;
  index_size: number;
  collections: Record<string, { document_count: number; document_size: number; index_size: number }>;
}

interface Setup {
  collections: number | null;
  buckets: number | null;
  providers: number | null;
  plugins: number | null;
  mcp: boolean | null;
  members: number | null;
}

const quiet = <T,>(p: Promise<T>) => p.catch(() => null);

function useSetup(projectId: string): Setup | null {
  const [s, setS] = useState<Setup | null>(null);
  useEffect(() => {
    const p = (x: string) => adminProjectPath(projectId, x);
    Promise.all([
      quiet(api<{ collections: unknown[] }>(p('/collections'))),
      quiet(api<{ buckets: string[] }>(p('/storage/buckets'))),
      quiet(api<{ providers: unknown[] }>(p('/auth/providers'))),
      quiet(api<{ integrations: unknown[] }>(p('/integrations'))),
      quiet(api<{ enabled?: boolean }>(p('/mcp'))),
      quiet(api<{ members: unknown[] }>(p('/members'))),
    ]).then(([c, b, pr, i, m, mem]) =>
      setS({
        // Internal collections (leading underscore) do not count as the user's own.
        collections: c ? (c.collections || []).filter((x) => !String(typeof x === 'string' ? x : (x as { name: string }).name).startsWith('_')).length : null,
        buckets: b ? (b.buckets || []).length : null,
        providers: pr ? (pr.providers || []).length : null,
        plugins: i ? (i.integrations || []).length : null,
        mcp: m ? !!m.enabled : null,
        members: mem ? (mem.members || []).length : null,
      }),
    );
  }, [projectId]);
  return s;
}

export default function ProjectHome() {
  const { project, projectId } = useProject();
  const { detail } = useProjectDetail(projectId);
  const { usage } = useProjectUsage(projectId);
  const setup = useSetup(projectId);
  const [logs, setLogs] = useState<LogEntry[] | null>(null);

  useEffect(() => {
    api<{ entries: LogEntry[] }>(adminProjectPath(projectId, '/logs?limit=50'))
      .then((d) => setLogs([...(d.entries || [])].sort((a, b) => b.timestamp.localeCompare(a.timestamp)).slice(0, 6)))
      .catch(() => setLogs([]));
  }, [projectId]);

  const isOnline = project.is_online ?? (project.status === 'active' || !project.status);
  const apiKey = detail?.api_key ?? project.api_key;
  const url = `${apiBase()}/api/v1`;

  return (
    <Page className="flex flex-col gap-5">
      <div>
        <PageHeader
          title={<span className="break-all">{project.name}</span>}
          meta={
            <Badge tone={isOnline ? 'ok' : 'neutral'} dot>
              {isOnline ? 'Live' : 'Offline'}
            </Badge>
          }
          description="What to set up next, how to connect your app, and how the project is running."
          actions={
            <ButtonLink to={projectPath(projectId, 'settings/api-keys')} icon={<KeyRound className="w-4 h-4" />}>
              API keys
            </ButtonLink>
          }
        />
        <div className="-mt-3 flex items-center gap-2 flex-wrap text-[12.5px] text-ink-3">
          <span>API URL</span>
          <code className="font-mono text-ink-2 break-all">{url}</code>
          <CopyButton text={url} label="Copy" />
        </div>
      </div>

      <div className="grid grid-cols-1 lg:grid-cols-[minmax(0,1fr)_minmax(0,1.1fr)] gap-5 items-start">
        <Checklist projectId={projectId} setup={setup} requests={usage?.api_requests} name={project.name} />
        <Card>
          <div className="flex flex-col gap-1 mb-4">
            <h2 className="m-0 text-[15px] font-semibold text-ink">Connect your app</h2>
            <p className="m-0 text-[12.5px] text-ink-3">
              Paste this where your app starts. Project id <code className="font-mono">{projectId}</code>.
            </p>
          </div>
          <ConnectSnippet apiKey={apiKey} projectId={projectId} />
          <p className="m-0 mt-3 text-[12.5px] text-ink-3">
            Keep the key out of public repositories. Manage it in{' '}
            <Link className="text-accent hover:text-accent-2" to={projectPath(projectId, 'settings/api-keys')}>
              Settings · API keys
            </Link>
            .
          </p>
        </Card>
      </div>

      <Metrics usage={usage} />

      <Card padded={false}>
        <CardHeader
          title="Recent requests"
          description="The latest calls your apps made to this project."
          actions={
            <ButtonLink size="sm" variant="ghost" to={projectPath(projectId, 'logs')}>
              All logs
            </ButtonLink>
          }
        />
        <RecentRequests logs={logs} />
      </Card>

      <DbSize projectId={projectId} />
    </Page>
  );
}

/* ---------- Get started ---------- */

function Checklist({ projectId, setup, requests, name }: { projectId: string; setup: Setup | null; requests?: number; name: string }) {
  if (!setup) {
    return (
      <Card>
        <Skeleton className="h-5 w-48 mb-4" />
        {[0, 1, 2, 3].map((i) => (
          <Skeleton key={i} className="h-9 w-full mb-2" />
        ))}
      </Card>
    );
  }
  const items: Array<{ done: boolean | null; title: string; hint: string; to: string; cta: string }> = [
    { done: setup.collections == null ? null : setup.collections > 0, title: 'Create a collection', hint: 'Collections hold your records.', to: 'database', cta: 'Open database' },
    { done: setup.providers == null ? null : setup.providers > 0, title: 'Add a sign-in method', hint: 'Email works already. Add Google or GitHub for one-click sign-in.', to: 'auth', cta: 'Open auth' },
    { done: setup.buckets == null ? null : setup.buckets > 0, title: 'Create a storage bucket', hint: 'Buckets hold uploaded files.', to: 'storage', cta: 'Open storage' },
    { done: setup.plugins == null ? null : setup.plugins > 0, title: 'Install a plugin', hint: 'Payments, chat alerts and more.', to: 'plugins', cta: 'Browse' },
    { done: setup.mcp, title: 'Connect an AI assistant', hint: 'Turn on the MCP server so Claude or Cursor can work with this project.', to: 'mcp', cta: 'Set up MCP' },
    { done: setup.members == null ? null : setup.members > 1, title: 'Invite a teammate', hint: 'Give someone else access to this project.', to: 'settings/team', cta: 'Invite' },
    { done: requests == null ? null : requests > 0, title: 'Make the first request from your app', hint: 'Detected automatically.', to: '', cta: '' },
  ];
  const known = items.filter((i) => i.done !== null);
  const done = known.filter((i) => i.done).length;
  if (known.length && done === known.length) {
    return (
      <Card>
        <div className="flex items-center gap-3">
          <span className="w-8 h-8 rounded-full bg-ok-soft text-ok inline-flex items-center justify-center">
            <Check className="w-4 h-4" />
          </span>
          <div>
            <h2 className="m-0 text-[15px] font-semibold text-ink">{name} is set up</h2>
            <p className="m-0 text-[12.5px] text-ink-3">Every step of the checklist is done.</p>
          </div>
        </div>
      </Card>
    );
  }
  const firstOpen = items.findIndex((i) => i.done === false);
  return (
    <Card padded={false}>
      <div className="flex items-center justify-between gap-3 px-5 pt-4 pb-3">
        <h2 className="m-0 text-[15px] font-semibold text-ink">Get {name} ready</h2>
        <span className="text-[12.5px] text-ink-3 tabular-nums">
          {done} of {known.length} done
        </span>
      </div>
      <div className="mx-5 mb-2 h-1.5 rounded-full bg-sunken overflow-hidden" aria-hidden>
        <div className="h-full bg-accent rounded-full transition-[width]" style={{ width: `${known.length ? (done / known.length) * 100 : 0}%` }} />
      </div>
      <ol className="m-0 p-0 list-none">
        {items.map((it, i) =>
          it.done === null ? null : (
            <li key={it.title} className="flex items-start gap-3 px-5 py-3 border-t border-line first:border-t-0">
              <span
                className={cx(
                  'mt-0.5 w-5 h-5 rounded-full shrink-0 inline-flex items-center justify-center',
                  it.done ? 'bg-ok-soft text-ok' : 'border border-line-2',
                )}
                aria-hidden
              >
                {it.done && <Check className="w-3 h-3" />}
              </span>
              <div className="flex-1 min-w-0">
                <p className={cx('m-0 text-sm', it.done ? 'text-ink-3 line-through' : 'text-ink font-medium')}>
                  {it.title}
                  <span className="sr-only">{it.done ? ' (done)' : ' (to do)'}</span>
                </p>
                {!it.done && i === firstOpen && <p className="m-0 text-[12.5px] text-ink-3">{it.hint}</p>}
              </div>
              {!it.done && it.cta && (
                <ButtonLink size="sm" variant={i === firstOpen ? 'primary' : 'ghost'} to={projectPath(projectId, it.to)}>
                  {it.cta}
                </ButtonLink>
              )}
            </li>
          ),
        )}
      </ol>
    </Card>
  );
}

/* ---------- Metrics ---------- */

function Metrics({ usage }: { usage: ReturnType<typeof useProjectUsage>['usage'] }) {
  const n = (v?: number) => (v == null ? '—' : v.toLocaleString());
  const b = (v?: number) => (v == null ? '—' : formatBytes(v));
  const tiles = [
    { label: 'API requests', value: n(usage?.api_requests), sub: 'Since the project was created' },
    { label: 'Database reads', value: n(usage?.db_reads_total), sub: `${n(usage?.db_writes_total)} writes` },
    { label: 'Realtime', value: n(usage?.realtime_connections), sub: 'Open connections' },
    { label: 'Storage used', value: b(usage?.total_storage_bytes), sub: usage?.bandwidth_down != null ? `${b(usage?.bandwidth_up)} up · ${b(usage?.bandwidth_down)} down` : undefined },
  ];
  return (
    <section aria-label="Usage">
      <Card padded={false} className="grid grid-cols-2 lg:grid-cols-4">
        {tiles.map((t, i) => (
          <Stat
            key={t.label}
            label={t.label}
            value={usage ? t.value : <Skeleton className="h-7 w-20 mt-0.5" />}
            sub={t.sub}
            className={cx('p-5 min-w-0', i % 2 === 1 && 'border-l border-line', i >= 2 && 'border-t lg:border-t-0 border-line', i === 2 && 'lg:border-l')}
          />
        ))}
      </Card>
      {usage && usage.enabled === false && <p className="m-0 mt-2 text-[12.5px] text-ink-3">Usage tracking is off on this server, so some numbers stay empty.</p>}
    </section>
  );
}

/* ---------- Activity ---------- */

function RecentRequests({ logs }: { logs: LogEntry[] | null }) {
  if (!logs) return <div className="px-5 pb-5 flex flex-col gap-2">{[0, 1, 2].map((i) => <Skeleton key={i} className="h-8 w-full" />)}</div>;
  const cols: Column<LogEntry>[] = [
    {
      key: 'path',
      header: 'Request',
      cell: (l) => (
        <span className="flex items-center gap-2 min-w-0">
          <span className="font-mono text-[12px] text-ink-3 w-12 shrink-0">{l.method}</span>
          <span className="font-mono text-[13px] text-ink truncate">{l.path}</span>
        </span>
      ),
    },
    {
      key: 'status',
      header: 'Status',
      width: '90px',
      cell: (l) => <Badge tone={l.status >= 500 ? 'danger' : l.status >= 400 ? 'warn' : 'ok'}>{l.status}</Badge>,
    },
    { key: 'dur', header: 'Time', width: '90px', cell: (l) => <span className="text-ink-3 tabular-nums">{l.duration}</span> },
    { key: 'at', header: 'When', width: '120px', align: 'right', cell: (l) => <span className="text-ink-3" title={formatDate(l.timestamp)}>{timeAgo(l.timestamp)}</span> },
  ];
  return (
    <DataTable
      columns={cols}
      rows={logs}
      rowKey={(l) => `${l.timestamp}-${l.path}-${l.method}`}
      empty={<EmptyState title="No requests yet" description="Calls from your apps show up here as they happen." className="py-10" />}
    />
  );
}

/* ---------- Database size (on demand) ---------- */

function DbSize({ projectId }: { projectId: string }) {
  const { showToast } = useToast();
  const [data, setData] = useState<DbAnalysis | null>(null);
  const [busy, setBusy] = useState(false);
  const run = async () => {
    setBusy(true);
    try {
      setData(await api<DbAnalysis>(adminProjectPath(projectId, '/db-analysis')));
    } catch (e) {
      showToast((e as Error).message, 'error');
    } finally {
      setBusy(false);
    }
  };
  const rows = Object.entries(data?.collections || {}).map(([name, c]) => ({ name, ...c }));
  return (
    <Card padded={false}>
      <CardHeader
        title="Database size"
        description="Scan collection sizes, metadata and index usage. Takes a moment on large projects."
        actions={
          <Button size="sm" loading={busy} onClick={run} icon={!busy ? <ArrowRight className="w-3.5 h-3.5" /> : undefined}>
            {data ? 'Scan again' : 'Scan now'}
          </Button>
        }
      />
      {data && (
        <>
          <div className="flex flex-wrap gap-x-10 gap-y-3 px-5 pb-4">
            <Stat label="Total" value={formatBytes(data.total_size)} />
            <Stat label="Metadata" value={formatBytes(data.metadata_size)} />
            <Stat label="Indexes" value={formatBytes(data.index_size)} />
          </div>
          {rows.length > 0 && (
            <div className="border-t border-line">
              <DataTable
                rows={rows}
                rowKey={(r) => r.name}
                columns={[
                  { key: 'name', header: 'Collection', cell: (r) => <span className="font-mono text-[13px] text-ink">{r.name}</span> },
                  { key: 'docs', header: 'Records', align: 'right', width: '120px', cell: (r) => <span className="tabular-nums">{r.document_count.toLocaleString()}</span> },
                  { key: 'size', header: 'Data', align: 'right', width: '120px', cell: (r) => <span className="tabular-nums text-ink-2">{formatBytes(r.document_size)}</span> },
                  { key: 'idx', header: 'Indexes', align: 'right', width: '120px', cell: (r) => <span className="tabular-nums text-ink-3">{formatBytes(r.index_size)}</span> },
                ]}
              />
            </div>
          )}
        </>
      )}
      {!data && !busy && (
        <div className="px-5 pb-5">
          <Banner>Nothing scanned yet.</Banner>
        </div>
      )}
    </Card>
  );
}
