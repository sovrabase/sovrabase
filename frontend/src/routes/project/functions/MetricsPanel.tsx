import { useCallback, useEffect, useState } from 'react';
import { api, adminProjectPath, formatBytes, formatDate } from '../../../api';
import type { FunctionExecution, MetricsBucket } from '../../../types';
import {
  Badge,
  Banner,
  Button,
  Card,
  CardHeader,
  CodeBlock,
  DataTable,
  EmptyState,
  Segmented,
  Sheet,
  Skeleton,
  Stat,
  type Column,
} from '../../../ui';
import { SeriesChart } from './charts';
import { statusTone, triggerLabel } from './shared';

type Range = '24h' | '7d';
type StatusFilter = 'all' | 'success' | 'error' | 'timeout';

function hourLabel(iso: string, range: Range) {
  try {
    const d = new Date(iso);
    return range === '24h'
      ? d.toLocaleTimeString([], { hour: '2-digit', minute: '2-digit' })
      : d.toLocaleString([], { weekday: 'short', hour: '2-digit' });
  } catch {
    return iso;
  }
}

const cpu = (e: FunctionExecution) => (e.metrics?.cpu_user_ms || 0) + (e.metrics?.cpu_sys_ms || 0);

export function MetricsPanel({ projectId, functionId, refreshKey }: { projectId: string; functionId: string; refreshKey: number }) {
  const [range, setRange] = useState<Range>('24h');
  const [buckets, setBuckets] = useState<MetricsBucket[] | null>(null);
  const [executions, setExecutions] = useState<FunctionExecution[]>([]);
  const [filter, setFilter] = useState<StatusFilter>('all');
  const [open, setOpen] = useState<FunctionExecution | null>(null);
  const [detail, setDetail] = useState<FunctionExecution | null>(null);

  const base = adminProjectPath(projectId, `/functions/${encodeURIComponent(functionId)}`);

  const load = useCallback(async () => {
    const [m, e] = await Promise.all([
      api<{ buckets: MetricsBucket[] }>(`${base}/metrics?range=${range}`).catch(() => ({ buckets: [] as MetricsBucket[] })),
      api<{ data: FunctionExecution[] }>(`${base}/executions`).catch(() => ({ data: [] as FunctionExecution[] })),
    ]);
    setBuckets(m.buckets || []);
    setExecutions(e.data || []);
  }, [base, range]);

  useEffect(() => {
    load();
  }, [load, refreshKey]);

  const openExec = async (ex: FunctionExecution) => {
    setOpen(ex);
    setDetail(null);
    try {
      setDetail(await api<FunctionExecution>(`${base}/executions/${encodeURIComponent(ex.id)}`));
    } catch {
      setDetail(ex);
    }
  };

  const b = buckets ?? [];
  const data = b.map((x) => ({
    t: hourLabel(x.hour, range),
    calls: x.exec_count,
    p95: x.duration_p95_ms,
    mem: x.mem_peak_max_bytes,
    errors: x.error_count,
  }));
  const calls = b.reduce((s, x) => s + x.exec_count, 0);
  const errors = b.reduce((s, x) => s + (x.error_count || 0), 0);
  const p95 = b.reduce((m, x) => Math.max(m, x.duration_p95_ms), 0);
  const peak = b.reduce((m, x) => Math.max(m, x.mem_peak_max_bytes), 0);
  const avgCpu = b.length ? Math.round(b.reduce((s, x) => s + x.cpu_avg_ms, 0) / b.length) : 0;

  const filtered = filter === 'all' ? executions : executions.filter((e) => e.status === filter);

  const columns: Column<FunctionExecution>[] = [
    { key: 'time', header: 'Time', width: 'minmax(0,1.4fr)', cell: (e) => <span className="text-[13px] text-ink-2">{formatDate(e.created_at)}</span> },
    {
      key: 'status',
      header: 'Status',
      width: '100px',
      cell: (e) => (
        <Badge tone={statusTone(e.status)} dot>
          {e.status}
        </Badge>
      ),
    },
    { key: 'trigger', header: 'Trigger', width: 'minmax(0,1fr)', cell: (e) => <span className="text-[13px] text-ink-2">{triggerLabel(e.trigger)}</span> },
    { key: 'dur', header: 'Duration', width: '90px', align: 'right', cell: (e) => <span className="font-mono text-[12.5px]">{e.duration_ms} ms</span> },
    { key: 'mem', header: 'Memory', width: '90px', align: 'right', hideOnMobile: true, cell: (e) => <span className="font-mono text-[12.5px]">{formatBytes(e.metrics?.mem_peak_bytes || 0)}</span> },
    { key: 'cpu', header: 'CPU', width: '80px', align: 'right', hideOnMobile: true, cell: (e) => <span className="font-mono text-[12.5px]">{cpu(e)} ms</span> },
  ];

  const samples = detail?.metrics?.samples ?? [];

  return (
    <div className="flex flex-col gap-5">
      <Card padded={false}>
        <CardHeader
          title="Metrics"
          description="Per hour, from real calls and test runs."
          actions={<Segmented size="sm" aria-label="Time range" value={range} onChange={setRange} options={[{ value: '24h', label: '24 h' }, { value: '7d', label: '7 days' }]} />}
        />
        <div className="border-t border-line p-5 flex flex-col gap-6">
          {buckets === null ? (
            <Skeleton className="h-[150px]" />
          ) : (
            <>
              <div className="grid grid-cols-2 sm:grid-cols-5 gap-5">
                <Stat label="Calls" value={calls.toLocaleString()} />
                <Stat label="Errors" value={errors.toLocaleString()} />
                <Stat label="p95" value={`${p95} ms`} />
                <Stat label="Memory peak" value={formatBytes(peak)} />
                <Stat label="Avg CPU" value={`${avgCpu} ms`} />
              </div>
              {data.length === 0 ? (
                <p className="m-0 text-[13px] text-ink-3">No calls in this range yet. Run a test to see numbers here.</p>
              ) : (
                <div className="grid grid-cols-1 lg:grid-cols-3 gap-5">
                  <div className="min-w-0">
                    <h4 className="m-0 mb-2 text-[13px] font-semibold text-ink">Calls</h4>
                    <SeriesChart data={data} x="t" y="calls" name="Calls" kind="bar" />
                  </div>
                  <div className="min-w-0">
                    <h4 className="m-0 mb-2 text-[13px] font-semibold text-ink">Duration p95</h4>
                    <SeriesChart data={data} x="t" y="p95" name="p95" format={(v) => `${v} ms`} />
                  </div>
                  <div className="min-w-0">
                    <h4 className="m-0 mb-2 text-[13px] font-semibold text-ink">Memory peak</h4>
                    <SeriesChart data={data} x="t" y="mem" name="Memory" format={(v) => formatBytes(v)} />
                  </div>
                </div>
              )}
            </>
          )}
        </div>
      </Card>

      <Card padded={false}>
        <CardHeader
          title="Executions"
          description="Most recent first. Open one to see its output."
          actions={
            <Segmented
              size="sm"
              aria-label="Status"
              value={filter}
              onChange={setFilter}
              options={[
                { value: 'all', label: 'All' },
                { value: 'success', label: 'Success' },
                { value: 'error', label: 'Error' },
                { value: 'timeout', label: 'Timeout' },
              ]}
            />
          }
        />
        <div className="border-t border-line">
          <DataTable
            columns={columns}
            rows={filtered}
            rowKey={(e) => e.id}
            onRowClick={openExec}
            selectedKey={open?.id}
            empty={<EmptyState title={executions.length ? 'No executions match' : 'No executions yet'} description={executions.length ? 'Try another status.' : 'Calls and test runs show up here.'} />}
          />
        </div>
      </Card>

      <Sheet
        open={!!open}
        onClose={() => setOpen(null)}
        title="Execution"
        description={open ? `${formatDate(open.created_at)} · ${triggerLabel(open.trigger)}` : undefined}
        width={600}
        footer={<Button onClick={() => setOpen(null)}>Close</Button>}
      >
        {!detail ? (
          <Skeleton className="h-40" />
        ) : (
          <>
            <div className="grid grid-cols-2 sm:grid-cols-4 gap-4">
              <Stat label="Status" value={<Badge tone={statusTone(detail.status)} dot>{detail.status}</Badge>} />
              <Stat label="Duration" value={`${detail.duration_ms} ms`} />
              <Stat label="Memory peak" value={formatBytes(detail.metrics?.mem_peak_bytes || 0)} />
              <Stat label="CPU" value={`${cpu(detail)} ms`} />
            </div>
            {detail.metrics?.isolated && <Badge tone="ok">Measured in an isolated worker (RSS and CPU)</Badge>}
            {detail.error && (
              <Banner tone="danger" title="Error">
                <span className="font-mono text-[12.5px] break-words">{detail.error}</span>
              </Banner>
            )}
            {samples.length > 0 && (
              <div className="flex flex-col gap-4">
                <div>
                  <h4 className="m-0 mb-2 text-[13px] font-semibold text-ink">{detail.metrics?.isolated ? 'Memory (RSS)' : 'Heap'} over the run</h4>
                  <SeriesChart data={samples} x="offset_ms" y="heap_bytes" name="Memory" kind="area" format={(v) => formatBytes(v)} height={140} />
                </div>
                <div>
                  <h4 className="m-0 mb-2 text-[13px] font-semibold text-ink">CPU time over the run</h4>
                  <SeriesChart data={samples} x="offset_ms" y="cpu_ms" name="CPU" format={(v) => `${v} ms`} height={120} />
                </div>
                <p className="m-0 text-[12px] text-ink-3">Horizontal axis: milliseconds since start.</p>
              </div>
            )}
            <div>
              <h4 className="m-0 mb-2 text-[13px] font-semibold text-ink">Console</h4>
              {detail.output ? <CodeBlock className="max-h-72">{detail.output}</CodeBlock> : <p className="m-0 text-[13px] text-ink-3">Nothing was logged.</p>}
            </div>
          </>
        )}
      </Sheet>
    </div>
  );
}
