import { useCallback, useEffect, useMemo, useState, type ReactNode } from 'react';
import { Activity, RefreshCw, Trash2 } from 'lucide-react';
import {
  Badge,
  Banner,
  Button,
  ButtonLink,
  Card,
  CodeBlock,
  ConfirmDialog,
  CopyButton,
  EmptyState,
  Page,
  PageHeader,
  SearchInput,
  Segmented,
  Sheet,
  Skeleton,
  Stat,
  Switch,
  cx,
  useToast,
  type Tone,
} from '../../ui';
import { api, adminProjectPath, formatDate } from '../../api';
import { useProject } from '../../shell/ProjectContext';
import { projectPath } from '../../shell/nav';
import { useProjectUsage } from '../../hooks/useProjectUsage';
import { SOURCES, clock, dayOf, levelOf, sourceLabel, sourceOf, summary, type Level, type RequestLog, type Source } from './log-view/model';

const PAGE = 200;
const LIVE_MS = 5000;

const LEVEL_TONE: Record<Level, Tone> = { info: 'neutral', warn: 'warn', error: 'danger' };
const LEVEL_TEXT: Record<Level, string> = { info: 'INFO', warn: 'WARN', error: 'ERROR' };

export default function ProjectLogs() {
  const { projectId } = useProject();
  const { showToast } = useToast();
  const { usage } = useProjectUsage(projectId);

  const [entries, setEntries] = useState<RequestLog[] | null>(null);
  const [total, setTotal] = useState(0);
  const [error, setError] = useState<string | null>(null);
  const [loadingMore, setLoadingMore] = useState(false);
  const [refreshing, setRefreshing] = useState(false);
  const [live, setLive] = useState(false);

  const [q, setQ] = useState('');
  const [level, setLevel] = useState<'all' | Level>('all');
  const [source, setSource] = useState<'all' | Source>('all');
  const [open, setOpen] = useState<RequestLog | null>(null);
  const [confirmFlush, setConfirmFlush] = useState(false);

  const fetchPage = useCallback(
    (offset: number) =>
      api<{ entries: RequestLog[]; total: number }>(adminProjectPath(projectId, `/logs?limit=${PAGE}&offset=${offset}`)),
    [projectId],
  );

  const load = useCallback(async () => {
    setRefreshing(true);
    try {
      const r = await fetchPage(0);
      setEntries(r.entries || []);
      setTotal(r.total || 0);
      setError(null);
    } catch (e) {
      setError((e as Error).message);
      setEntries((x) => x ?? []);
    } finally {
      setRefreshing(false);
    }
  }, [fetchPage]);

  useEffect(() => {
    load();
  }, [load]);

  useEffect(() => {
    if (!live) return;
    const t = setInterval(load, LIVE_MS);
    return () => clearInterval(t);
  }, [live, load]);

  const loadMore = async () => {
    if (!entries) return;
    setLoadingMore(true);
    try {
      const r = await fetchPage(entries.length);
      setEntries([...entries, ...(r.entries || [])]);
      setTotal(r.total || 0);
    } catch (e) {
      showToast((e as Error).message, 'error');
    } finally {
      setLoadingMore(false);
    }
  };

  const flush = async () => {
    try {
      await api(adminProjectPath(projectId, '/logs'), { method: 'DELETE' });
      setEntries([]);
      setTotal(0);
      showToast('Logs flushed', 'success');
    } catch (e) {
      showToast((e as Error).message, 'error');
    }
  };

  const rows = useMemo(() => {
    const needle = q.trim().toLowerCase();
    return (entries ?? []).filter((e) => {
      if (level !== 'all' && levelOf(e) !== level) return false;
      if (source !== 'all' && sourceOf(e) !== source) return false;
      if (needle) {
        const hay = `${summary(e)} ${e.status ?? ''} ${e.email ?? ''} ${e.ip ?? ''} ${e.collection ?? ''}`.toLowerCase();
        if (!hay.includes(needle)) return false;
      }
      return true;
    });
  }, [entries, q, level, source]);

  const loaded = entries?.length ?? 0;
  const errors = (entries ?? []).filter((e) => (e.status ?? 0) >= 400).length;
  const errorRate = loaded ? Math.round((errors / loaded) * 100) : 0;

  const levelCounts = useMemo(() => {
    const c: Record<Level, number> = { info: 0, warn: 0, error: 0 };
    entries?.forEach((e) => c[levelOf(e)]++);
    return c;
  }, [entries]);

  return (
    <Page wide>
      <PageHeader
        title="Logs"
        description="Every request to this project's API, newest first, with its status and timing."
        actions={
          <>
            <Switch checked={live} onChange={setLive} label="Live" />
            <Button icon={<RefreshCw className={cx('w-4 h-4', refreshing && 'animate-spin')} />} onClick={load}>
              Refresh
            </Button>
            <Button variant="ghost" className="text-danger" icon={<Trash2 className="w-4 h-4" />} onClick={() => setConfirmFlush(true)}>
              Flush
            </Button>
          </>
        }
      />

      {error && (
        <div className="mb-4">
          <Banner tone="danger" title="Logs could not be loaded">
            {error}
          </Banner>
        </div>
      )}

      <Card className="mb-5">
        <div className="grid grid-cols-2 md:grid-cols-4 gap-4">
          <Stat label="Requests logged" value={total.toLocaleString()} />
          <Stat label="Success rate" value={`${100 - errorRate}%`} sub={loaded ? `last ${loaded.toLocaleString()} requests` : undefined} />
          <Stat label="Errors" value={`${errorRate}%`} sub={`${errors.toLocaleString()} with status 4xx or 5xx`} />
          <Stat label="Realtime connections" value={(usage?.realtime_connections ?? 0).toLocaleString()} sub="open now" />
        </div>
      </Card>

      <div className="flex flex-wrap items-center gap-3 mb-3">
        <SearchInput value={q} onChange={setQ} placeholder="Search path, status, email, IP" className="flex-1 min-w-[220px]" />
        <Segmented
          size="sm"
          aria-label="Level"
          value={level}
          onChange={setLevel}
          options={[
            { value: 'all', label: 'All levels' },
            { value: 'info', label: 'Info', count: levelCounts.info },
            { value: 'warn', label: 'Warn', count: levelCounts.warn },
            { value: 'error', label: 'Error', count: levelCounts.error },
          ]}
        />
      </div>
      <div className="mb-4 overflow-x-auto no-scrollbar">
        <Segmented
          size="sm"
          aria-label="Source"
          value={source}
          onChange={setSource}
          options={[{ value: 'all' as const, label: 'All sources' }, ...SOURCES]}
        />
      </div>

      <Card padded={false}>
        {entries === null ? (
          <div className="p-5 flex flex-col gap-2">
            {Array.from({ length: 6 }, (_, i) => (
              <Skeleton key={i} className="h-7" />
            ))}
          </div>
        ) : rows.length === 0 ? (
          <EmptyState
            icon={<Activity className="w-5 h-5" />}
            title={entries.length ? 'No entries match' : 'No requests logged yet'}
            description={entries.length ? 'Change the filters or search.' : 'Requests to this project show up here as they happen.'}
          />
        ) : (
          <>
            <div className="hidden md:grid grid-cols-[150px_70px_96px_minmax(0,1fr)_56px_90px] gap-3 px-5 h-9 items-center border-b border-line text-[12px] font-semibold uppercase tracking-[.05em] text-ink-3">
              <span>Time</span>
              <span>Level</span>
              <span>Source</span>
              <span>Request</span>
              <span>Status</span>
              <span className="text-right">Duration</span>
            </div>
            <ul className="m-0 p-0 list-none font-mono text-[12.5px]">
              {rows.map((e, i) => {
                const lv = levelOf(e);
                const status = e.status ?? 0;
                return (
                  <li key={`${e.timestamp}-${i}`} className="border-b border-line last:border-b-0">
                    <button
                      type="button"
                      onClick={() => setOpen(e)}
                      className={cx(
                        'w-full text-left px-4 md:px-5 py-2 hover:bg-hover focus-visible:outline-none focus-visible:bg-hover',
                        'grid gap-x-3 gap-y-0.5 items-center grid-cols-[auto_auto_minmax(0,1fr)] md:grid-cols-[150px_70px_96px_minmax(0,1fr)_56px_90px]',
                        lv === 'error' && 'bg-danger-soft/40',
                      )}
                    >
                      <span className="text-ink-3 whitespace-nowrap" title={formatDate(e.timestamp)}>
                        <span className="hidden md:inline">{dayOf(e.timestamp)} </span>
                        {clock(e.timestamp)}
                      </span>
                      <span>
                        <Badge tone={LEVEL_TONE[lv]} className="font-mono text-[11px] h-5">
                          {LEVEL_TEXT[lv]}
                        </Badge>
                      </span>
                      <span className="text-ink-3 truncate text-right md:text-left">{sourceLabel(sourceOf(e))}</span>
                      <span className="col-span-3 md:col-span-1 text-ink truncate" title={summary(e)}>
                        {e.method && <b className="font-semibold text-ink-2 mr-2">{e.method}</b>}
                        {e.message ?? e.path}
                      </span>
                      <span
                        className={cx(
                          'hidden md:inline font-semibold',
                          status >= 500 ? 'text-danger' : status >= 400 ? 'text-warn' : 'text-ink-2',
                        )}
                      >
                        {status || ''}
                      </span>
                      <span className="hidden md:inline text-ink-3 text-right">{e.duration}</span>
                    </button>
                  </li>
                );
              })}
            </ul>
          </>
        )}
      </Card>

      {entries && entries.length > 0 && (
        <div className="flex flex-wrap items-center justify-between gap-3 mt-3">
          <span className="text-[12.5px] text-ink-3">
            Showing {rows.length.toLocaleString()} of {loaded.toLocaleString()} loaded · {total.toLocaleString()} in total
          </span>
          {loaded < total && (
            <Button size="sm" loading={loadingMore} onClick={loadMore}>
              Load older
            </Button>
          )}
        </div>
      )}

      <LogSheet entry={open} onClose={() => setOpen(null)} projectId={projectId} />

      <ConfirmDialog open={confirmFlush} onClose={() => setConfirmFlush(false)} onConfirm={flush} title="Flush all logs?" confirmLabel="Flush logs">
        Every request log of this project is deleted. Usage counters are not affected.
      </ConfirmDialog>
    </Page>
  );
}

function LogSheet({ entry, onClose, projectId }: { entry: RequestLog | null; onClose: () => void; projectId: string }) {
  if (!entry) return null;
  const lv = levelOf(entry);
  const json = JSON.stringify(entry, null, 2);
  const facts: Array<{ k: string; v: ReactNode }> = [
    { k: 'Time', v: formatDate(entry.timestamp) },
    { k: 'Level', v: <Badge tone={LEVEL_TONE[lv]}>{LEVEL_TEXT[lv]}</Badge> },
    { k: 'Source', v: sourceLabel(sourceOf(entry)) },
  ];
  if (entry.method) facts.push({ k: 'Method', v: <code className="font-mono">{entry.method}</code> });
  if (entry.path) facts.push({ k: 'Path', v: <code className="font-mono break-all">{entry.path}</code> });
  if (entry.status) facts.push({ k: 'Status', v: <code className="font-mono">{entry.status}</code> });
  if (entry.duration) facts.push({ k: 'Duration', v: <code className="font-mono">{entry.duration}</code> });
  if (entry.email) facts.push({ k: 'User', v: entry.email });
  if (entry.ip) facts.push({ k: 'IP', v: <code className="font-mono">{entry.ip}</code> });
  if (entry.collection) facts.push({ k: 'Collection', v: <code className="font-mono">{entry.collection}</code> });
  if (entry.doc_id) facts.push({ k: 'Record', v: <code className="font-mono break-all">{entry.doc_id}</code> });

  return (
    <Sheet
      open
      onClose={onClose}
      title={<span className="font-mono text-[15px] break-all">{summary(entry)}</span>}
      description={formatDate(entry.timestamp)}
      footer={
        <>
          <CopyButton text={json} label="Copy as JSON" />
          {entry.collection && (
            <ButtonLink size="sm" to={projectPath(projectId, `database/${encodeURIComponent(entry.collection)}`)}>
              Open collection
            </ButtonLink>
          )}
        </>
      }
    >
      <dl className="m-0 grid grid-cols-[110px_minmax(0,1fr)] gap-x-4 gap-y-2.5 text-[13.5px]">
        {facts.map(({ k, v }) => (
          <div key={k} className="contents">
            <dt className="text-ink-3">{k}</dt>
            <dd className="m-0 text-ink min-w-0">{v}</dd>
          </div>
        ))}
      </dl>
      <div className="flex flex-col gap-2">
        <h3 className="m-0 text-[13px] font-semibold text-ink">Raw entry</h3>
        <CodeBlock className="whitespace-pre-wrap break-words">{json}</CodeBlock>
      </div>
    </Sheet>
  );
}
