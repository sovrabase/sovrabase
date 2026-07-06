import { useCallback, useEffect, useState } from 'react';
import {
  ComposedChart,
  Line,
  Bar,
  XAxis,
  YAxis,
  CartesianGrid,
  Tooltip,
  Legend,
  ResponsiveContainer,
  AreaChart,
  Area,
} from 'recharts';
import { Loader2 } from 'lucide-react';
import { api, formatBytes, formatDate } from '../api';
import type { FunctionExecution, MetricsBucket } from '../types';
import Modal from '../components/Modal';

interface Props {
  projectId: string;
  functionId: string | null;
  refreshKey?: number;
}

const STATUS_FILTERS = ['all', 'success', 'error', 'timeout'] as const;
type StatusFilter = (typeof STATUS_FILTERS)[number];

function formatHour(iso: string) {
  try {
    return new Date(iso).toLocaleTimeString([], { hour: '2-digit', minute: '2-digit' });
  } catch {
    return iso;
  }
}

export default function FunctionMetricsChart({ projectId, functionId, refreshKey }: Props) {
  const [range, setRange] = useState<'24h' | '7d'>('24h');
  const [buckets, setBuckets] = useState<MetricsBucket[]>([]);
  const [executions, setExecutions] = useState<FunctionExecution[]>([]);
  const [loading, setLoading] = useState(false);
  const [statusFilter, setStatusFilter] = useState<StatusFilter>('all');
  const [selectedExec, setSelectedExec] = useState<FunctionExecution | null>(null);
  const [execDetail, setExecDetail] = useState<FunctionExecution | null>(null);

  const load = useCallback(() => {
    if (!functionId) return;
    setLoading(true);
    Promise.all([
      api<{ buckets: MetricsBucket[] }>(
        `/admin/projects/${encodeURIComponent(projectId)}/functions/${encodeURIComponent(functionId)}/metrics?range=${range}`,
      ).catch(() => ({ buckets: [] })),
      api<{ data: FunctionExecution[] }>(
        `/admin/projects/${encodeURIComponent(projectId)}/functions/${encodeURIComponent(functionId)}/executions`,
      ).catch(() => ({ data: [] })),
    ])
      .then(([metrics, execs]) => {
        setBuckets(metrics.buckets || []);
        setExecutions(execs.data || []);
      })
      .finally(() => setLoading(false));
  }, [projectId, functionId, range, refreshKey]);

  useEffect(() => {
    load();
  }, [load]);

  const openExec = async (exec: FunctionExecution) => {
    setSelectedExec(exec);
    try {
      const detail = await api<FunctionExecution>(
        `/admin/projects/${encodeURIComponent(projectId)}/functions/${encodeURIComponent(functionId!)}/executions/${encodeURIComponent(exec.id)}`,
      );
      setExecDetail(detail);
    } catch {
      setExecDetail(exec);
    }
  };

  if (!functionId) {
    return (
      <p className="text-text-muted text-sm py-6 text-center">
        Select a function to view metrics
      </p>
    );
  }

  if (loading && buckets.length === 0) {
    return (
      <div className="flex items-center gap-2 py-8 text-text-muted justify-center">
        <Loader2 className="w-4 h-4 animate-spin" /> Loading metrics...
      </div>
    );
  }

  const chartData = buckets.map((b) => ({
    hour: formatHour(b.hour),
    durationP95: b.duration_p95_ms,
    memPeak: b.mem_peak_max_bytes,
    cpuAvg: b.cpu_avg_ms,
    execCount: b.exec_count,
  }));

  const peakMem = buckets.reduce((m, b) => Math.max(m, b.mem_peak_max_bytes), 0);
  const p95Duration = buckets.reduce((m, b) => Math.max(m, b.duration_p95_ms), 0);
  const avgCpu =
    buckets.length > 0
      ? Math.round(buckets.reduce((s, b) => s + b.cpu_avg_ms, 0) / buckets.length)
      : 0;

  const samples = execDetail?.metrics?.samples || [];

  const filteredExecutions =
    statusFilter === 'all' ? executions : executions.filter((ex) => ex.status === statusFilter);

  return (
    <div className="space-y-4">
      <div className="flex items-center justify-between gap-2 flex-wrap">
        <h3 className="text-sm font-semibold text-text-primary">Execution Metrics</h3>
        <div className="flex gap-1">
          {(['24h', '7d'] as const).map((r) => (
            <button
              key={r}
              onClick={() => setRange(r)}
              className={`px-2.5 py-1 rounded text-xs border transition-colors ${
                range === r
                  ? 'bg-accent text-white border-accent'
                  : 'bg-bg-input border-border text-text-secondary hover:text-text-primary'
              }`}
            >
              {r}
            </button>
          ))}
        </div>
      </div>

      <div className="grid grid-cols-3 gap-3">
        <div className="bg-bg-card border border-border rounded-lg p-3">
          <p className="text-text-muted text-xs">Durée P95</p>
          <p className="text-lg font-bold text-text-primary">{p95Duration} ms</p>
        </div>
        <div className="bg-bg-card border border-border rounded-lg p-3">
          <p className="text-text-muted text-xs">Pic RAM</p>
          <p className="text-lg font-bold text-text-primary">{formatBytes(peakMem)}</p>
        </div>
        <div className="bg-bg-card border border-border rounded-lg p-3">
          <p className="text-text-muted text-xs">CPU moyen</p>
          <p className="text-lg font-bold text-text-primary">{avgCpu} ms</p>
        </div>
      </div>

      {chartData.length > 0 ? (
        <div className="bg-bg-card border border-border rounded-lg p-4 h-64">
          <ResponsiveContainer width="100%" height="100%">
            <ComposedChart data={chartData}>
              <CartesianGrid strokeDasharray="3 3" stroke="var(--color-border)" />
              <XAxis dataKey="hour" tick={{ fontSize: 10 }} stroke="var(--color-text-muted)" />
              <YAxis yAxisId="left" tick={{ fontSize: 10 }} stroke="var(--color-text-muted)" />
              <YAxis yAxisId="right" orientation="right" tick={{ fontSize: 10 }} stroke="var(--color-text-muted)" />
              <Tooltip
                contentStyle={{
                  background: 'var(--color-bg-card)',
                  border: '1px solid var(--color-border)',
                  borderRadius: 8,
                  fontSize: 12,
                }}
              />
              <Legend wrapperStyle={{ fontSize: 11 }} />
              <Bar yAxisId="left" dataKey="durationP95" name="Durée P95 (ms)" fill="var(--color-accent)" opacity={0.7} />
              <Line yAxisId="right" type="monotone" dataKey="memPeak" name="Pic RAM" stroke="#22c55e" dot={false} />
              <Line yAxisId="right" type="monotone" dataKey="cpuAvg" name="CPU moy (ms)" stroke="#f59e0b" dot={false} />
            </ComposedChart>
          </ResponsiveContainer>
        </div>
      ) : (
        <p className="text-text-muted text-sm text-center py-4">No metrics yet — run or test the function.</p>
      )}

      {executions.length > 0 && (
        <div className="space-y-2">
          <div className="flex gap-1">
            {STATUS_FILTERS.map((s) => (
              <button
                key={s}
                onClick={() => setStatusFilter(s)}
                className={`px-2.5 py-1 rounded text-xs border transition-colors ${
                  statusFilter === s
                    ? 'bg-accent text-white border-accent'
                    : 'bg-bg-input border-border text-text-secondary hover:text-text-primary'
                }`}
              >
                {s === 'all' ? 'All' : s.charAt(0).toUpperCase() + s.slice(1)}
              </button>
            ))}
          </div>
          <div className="border border-border rounded-lg overflow-hidden">
            <table className="w-full text-xs">
              <thead className="bg-bg-input text-text-muted uppercase">
                <tr>
                  <th className="text-left px-3 py-2">Time</th>
                  <th className="text-left px-3 py-2">Trigger</th>
                  <th className="text-left px-3 py-2">Status</th>
                  <th className="text-left px-3 py-2">Duration</th>
                  <th className="text-left px-3 py-2">RAM peak</th>
                  <th className="text-left px-3 py-2">CPU</th>
                </tr>
              </thead>
              <tbody className="divide-y divide-border">
                {filteredExecutions.length === 0 && (
                  <tr>
                    <td colSpan={6} className="px-3 py-4 text-center text-text-muted">
                      No executions match this filter
                    </td>
                  </tr>
                )}
                {filteredExecutions.map((ex) => (
                  <tr
                    key={ex.id}
                    className="hover:bg-bg-input/50 cursor-pointer"
                    onClick={() => openExec(ex)}
                  >
                    <td className="px-3 py-2 text-text-secondary">{formatDate(ex.created_at)}</td>
                    <td className="px-3 py-2 font-mono text-accent">{ex.trigger}</td>
                    <td className="px-3 py-2">
                      <span
                        className={
                          ex.status === 'success'
                            ? 'text-success'
                            : ex.status === 'timeout'
                              ? 'text-yellow-500'
                              : 'text-danger'
                        }
                      >
                        {ex.status}
                      </span>
                    </td>
                    <td className="px-3 py-2 font-mono">{ex.duration_ms} ms</td>
                    <td className="px-3 py-2 font-mono">
                      {formatBytes(ex.metrics?.mem_peak_bytes || 0)}
                    </td>
                    <td className="px-3 py-2 font-mono">
                      {(ex.metrics?.cpu_user_ms || 0) + (ex.metrics?.cpu_sys_ms || 0)} ms
                    </td>
                  </tr>
                ))}
              </tbody>
            </table>
          </div>
        </div>
      )}

      <Modal
        isOpen={!!selectedExec}
        onClose={() => {
          setSelectedExec(null);
          setExecDetail(null);
        }}
        title="Execution detail"
        size="lg"
      >
        {execDetail && (
          <div className="space-y-4">
            <div className="grid grid-cols-2 gap-2 text-sm">
              <div>
                <span className="text-text-muted">Status: </span>
                <span className="text-text-primary">{execDetail.status}</span>
              </div>
              <div>
                <span className="text-text-muted">Duration: </span>
                <span className="font-mono">{execDetail.duration_ms} ms</span>
              </div>
              {execDetail.metrics?.isolated && (
                <div className="col-span-2">
                  <span className="text-xs px-2 py-0.5 rounded bg-success/10 text-success">
                    Isolated worker metrics (RSS/CPU)
                  </span>
                </div>
              )}
            </div>
            {samples.length > 0 && (
              <div className="h-48">
                <ResponsiveContainer width="100%" height="100%">
                  <AreaChart data={samples}>
                    <CartesianGrid strokeDasharray="3 3" stroke="var(--color-border)" />
                    <XAxis
                      dataKey="offset_ms"
                      tick={{ fontSize: 10 }}
                      label={{ value: 'ms', position: 'insideBottomRight', offset: -5, fontSize: 10 }}
                    />
                    <YAxis tick={{ fontSize: 10 }} />
                    <Tooltip />
                    <Legend wrapperStyle={{ fontSize: 11 }} />
                    <Area
                      type="monotone"
                      dataKey="heap_bytes"
                      name={execDetail.metrics?.isolated ? 'RSS' : 'Heap'}
                      stroke="#22c55e"
                      fill="#22c55e33"
                    />
                    <Line type="monotone" dataKey="cpu_ms" name="CPU (ms)" stroke="#f59e0b" dot={false} />
                  </AreaChart>
                </ResponsiveContainer>
              </div>
            )}
            {execDetail.output && (
              <pre className="text-xs bg-bg-input border border-border rounded p-3 overflow-auto max-h-32 font-mono">
                {execDetail.output}
              </pre>
            )}
            {execDetail.error && (
              <p className="text-danger text-sm">{execDetail.error}</p>
            )}
          </div>
        )}
      </Modal>
    </div>
  );
}
