import { useCallback, useEffect, useMemo, useState } from 'react';
import { BarChart3, Download, RefreshCw } from 'lucide-react';
import {
  Banner,
  Button,
  Card,
  CardHeader,
  CodeBlock,
  EmptyState,
  Page,
  PageHeader,
  Segmented,
  Skeleton,
  Stat,
  cx,
} from '../../ui';
import { api, adminProjectPath, formatBytes } from '../../api';
import { useProject } from '../../shell/ProjectContext';
import { useProjectUsage } from '../../hooks/useProjectUsage';
import { SingleBarChart, type Point } from './analytics/charts';

interface AnalyticsResponse {
  total_events?: number;
  top_events?: Array<{ name: string; count: number }>;
  /** Current API: one bucket per hour, RFC3339. */
  hourly_counts?: Array<{ hour: string; count: number }>;
  /** Older shape: hour of day 0-23. */
  hourly?: Array<{ hour: number; count: number }>;
}

const pad = (n: number) => String(n).padStart(2, '0');

/** Normalise both server shapes into 24 hourly points, oldest first. */
function hourlyPoints(d: AnalyticsResponse): Point[] {
  if (d.hourly_counts?.length) {
    const sorted = [...d.hourly_counts].sort((a, b) => +new Date(a.hour) - +new Date(b.hour));
    return sorted.map((h) => {
      const t = new Date(h.hour);
      const ok = !Number.isNaN(t.getTime());
      return {
        label: ok ? `${pad(t.getHours())}h` : h.hour,
        title: ok ? t.toLocaleString(undefined, { weekday: 'short', hour: '2-digit', minute: '2-digit' }) : h.hour,
        value: h.count,
      };
    });
  }
  if (d.hourly?.length) {
    const byHour = new Map(d.hourly.map((h) => [h.hour, h.count]));
    return Array.from({ length: 24 }, (_, h) => ({ label: `${pad(h)}h`, title: `${pad(h)}:00 – ${pad(h)}:59`, value: byHour.get(h) ?? 0 }));
  }
  return [];
}

function csvCell(v: string | number) {
  const s = String(v);
  return /[",\n]/.test(s) ? `"${s.replace(/"/g, '""')}"` : s;
}

export default function ProjectAnalytics() {
  const { projectId, project } = useProject();
  const { usage, refresh: refreshUsage } = useProjectUsage(projectId, false);
  const [data, setData] = useState<AnalyticsResponse | null>(null);
  const [error, setError] = useState<string | null>(null);
  const [loading, setLoading] = useState(true);
  const [hourlyView, setHourlyView] = useState<'chart' | 'table'>('chart');

  const load = useCallback(async () => {
    setLoading(true);
    try {
      const [d] = await Promise.all([api<AnalyticsResponse>(adminProjectPath(projectId, '/analytics')), refreshUsage()]);
      setData(d);
      setError(null);
    } catch (e) {
      setError((e as Error).message);
    } finally {
      setLoading(false);
    }
  }, [projectId, refreshUsage]);

  useEffect(() => {
    load();
  }, [load]);

  const hourly = useMemo(() => (data ? hourlyPoints(data) : []), [data]);
  const top = data?.top_events ?? [];
  const topMax = Math.max(1, ...top.map((t) => t.count));
  const activeHours = hourly.filter((h) => h.value > 0).length;
  const busiest = hourly.reduce<Point | null>((m, h) => (!m || h.value > m.value ? h : m), null);

  const methods: Point[] = useMemo(
    () =>
      Object.entries(usage?.requests_by_method ?? {})
        .map(([label, value]) => ({ label, value }))
        .sort((a, b) => b.value - a.value),
    [usage],
  );

  const exportCsv = () => {
    const lines = ['section,key,count'];
    hourly.forEach((h) => lines.push(['hourly', h.title ?? h.label, h.value].map(csvCell).join(',')));
    top.forEach((t) => lines.push(['top_event', t.name, t.count].map(csvCell).join(',')));
    methods.forEach((m) => lines.push(['requests_by_method', m.label, m.value].map(csvCell).join(',')));
    const blob = new Blob([lines.join('\n')], { type: 'text/csv' });
    const a = document.createElement('a');
    a.href = URL.createObjectURL(blob);
    a.download = `${project.name || projectId}-analytics.csv`;
    a.click();
    setTimeout(() => URL.revokeObjectURL(a.href), 1000);
  };

  const noEvents = !!data && !data.total_events && !top.length && hourly.every((h) => h.value === 0);

  return (
    <Page>
      <PageHeader
        title="Analytics"
        description="Usage of this project and the custom events your app sends."
        actions={
          <>
            <Button icon={<RefreshCw className={cx('w-4 h-4', loading && 'animate-spin')} />} onClick={load}>
              Refresh
            </Button>
            <Button icon={<Download className="w-4 h-4" />} onClick={exportCsv} disabled={!data}>
              Export CSV
            </Button>
          </>
        }
      />

      {error && (
        <div className="mb-5">
          <Banner tone="danger" title="Analytics could not be loaded" action={<Button size="sm" onClick={load}>Retry</Button>}>
            {error}
          </Banner>
        </div>
      )}

      {/* Usage tiles */}
      <Card className="mb-5">
        {usage === null && loading ? (
          <div className="grid grid-cols-2 md:grid-cols-4 gap-4">
            {Array.from({ length: 4 }, (_, i) => (
              <Skeleton key={i} className="h-14" />
            ))}
          </div>
        ) : (
          <div className="grid grid-cols-2 md:grid-cols-4 gap-x-4 gap-y-5">
            <Stat label="API requests" value={(usage?.api_requests ?? 0).toLocaleString()} sub="since the project started" />
            <Stat
              label="Database reads / writes"
              value={`${(usage?.db_reads_total ?? 0).toLocaleString()} / ${(usage?.db_writes_total ?? 0).toLocaleString()}`}
            />
            <Stat
              label="Storage used"
              value={formatBytes(usage?.total_storage_bytes || 0)}
              sub={`${formatBytes(usage?.database_bytes || 0)} database · ${formatBytes(usage?.file_storage_bytes || 0)} files`}
            />
            <Stat
              label="Bandwidth out"
              value={formatBytes(usage?.bandwidth_down || 0)}
              sub={`${formatBytes(usage?.bandwidth_up || 0)} in · ${(usage?.realtime_connections ?? 0).toLocaleString()} realtime now`}
            />
          </div>
        )}
      </Card>

      <div className="grid gap-5 lg:grid-cols-[minmax(0,1.6fr)_minmax(0,1fr)] mb-5">
        <Card padded={false}>
          <CardHeader
            title="Events per hour"
            description={
              data
                ? `${(data.total_events ?? 0).toLocaleString()} events in the last 24 h · active ${activeHours} of 24 hours${
                    busiest && busiest.value ? ` · busiest ${busiest.title ?? busiest.label}` : ''
                  }`
                : undefined
            }
            actions={
              <Segmented
                size="sm"
                aria-label="Hourly view"
                value={hourlyView}
                onChange={setHourlyView}
                options={[
                  { value: 'chart', label: 'Chart' },
                  { value: 'table', label: 'Table' },
                ]}
              />
            }
          />
          <div className="px-3 pb-4">
            {loading && !data ? (
              <Skeleton className="h-[220px] mx-2" />
            ) : noEvents || !hourly.length ? (
              <EmptyState
                icon={<BarChart3 className="w-5 h-5" />}
                title="No events in the last 24 hours"
                description="Send custom events from your app to see them here."
              />
            ) : hourlyView === 'chart' ? (
              <SingleBarChart data={hourly} unit="events" ariaLabel="Events per hour, last 24 hours" tickInterval="preserveStartEnd" />
            ) : (
              <div className="px-2 max-h-[260px] overflow-y-auto">
                <table className="w-full text-[13px]">
                  <thead>
                    <tr className="text-ink-3 text-[12px] text-left">
                      <th className="font-semibold py-1.5">Hour</th>
                      <th className="font-semibold py-1.5 text-right">Events</th>
                    </tr>
                  </thead>
                  <tbody>
                    {hourly.map((h, i) => (
                      <tr key={i} className="border-t border-line">
                        <td className="py-1.5 text-ink-2">{h.title ?? h.label}</td>
                        <td className="py-1.5 text-right tabular-nums text-ink">{h.value.toLocaleString()}</td>
                      </tr>
                    ))}
                  </tbody>
                </table>
              </div>
            )}
          </div>
        </Card>

        <Card padded={false}>
          <CardHeader
            title="Top events"
            description={top[0] ? `${top[0].name} leads with ${top[0].count.toLocaleString()}` : 'Last 24 hours'}
          />
          {top.length === 0 ? (
            <p className="m-0 px-5 pb-5 text-[13px] text-ink-3">{loading ? 'Loading…' : 'No custom events yet.'}</p>
          ) : (
            <ul className="m-0 px-5 pb-5 list-none flex flex-col gap-2.5">
              {top.map((t) => (
                <li key={t.name} className="flex flex-col gap-1" title={`${t.name}: ${t.count.toLocaleString()}`}>
                  <div className="flex items-center justify-between gap-3 text-[13px]">
                    <code className="font-mono text-[12.5px] text-ink truncate">{t.name}</code>
                    <span className="tabular-nums text-ink-2">{t.count.toLocaleString()}</span>
                  </div>
                  <div className="h-1.5 rounded-full bg-sunken overflow-hidden" aria-hidden>
                    <div className="h-full rounded-full bg-accent" style={{ width: `${Math.max(2, (t.count / topMax) * 100)}%` }} />
                  </div>
                </li>
              ))}
            </ul>
          )}
        </Card>
      </div>

      <div className="grid gap-5 lg:grid-cols-[minmax(0,1fr)_minmax(0,1fr)]">
        <Card padded={false}>
          <CardHeader title="Requests by method" description="All requests since the project started." />
          <div className="px-3 pb-4">
            {methods.length ? (
              <SingleBarChart data={methods} unit="requests" height={200} ariaLabel="Requests by HTTP method" />
            ) : (
              <p className="m-0 px-2 pb-1 text-[13px] text-ink-3">No requests counted yet.</p>
            )}
          </div>
        </Card>
        <Card padded={false}>
          <CardHeader title="Send custom events" description="Events from your app show up above within a few seconds." />
          <div className="px-5 pb-5">
            <CodeBlock>{`POST /api/v1/events
X-Project-Key: <your project key>

{ "events": [
  { "name": "checkout_started",
    "properties": { "cart": 3 } }
] }`}</CodeBlock>
          </div>
        </Card>
      </div>
    </Page>
  );
}
