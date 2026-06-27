import { useState, useEffect, useCallback } from 'react';
import { RefreshCw, Trash2, Loader2, Activity, Cloud, HardDrive, Database } from 'lucide-react';
import { api, formatBytes, formatDate } from '../api';

interface LogEntry { timestamp: string; method: string; path: string; status: number; duration: string; ip?: string; }
interface UsageMetrics {
  db_reads_total?: number;
  db_writes_total?: number;
  database_bytes?: number;
  file_storage_bytes?: number;
  total_storage_bytes?: number;
  realtime_connections?: number;
}

interface Props { projectId: string; }

const METHOD_COLORS: Record<string, string> = {
  GET: 'bg-success/10 text-success', POST: 'bg-accent/10 text-accent',
  PUT: 'bg-yellow-500/10 text-yellow-500', PATCH: 'bg-yellow-500/10 text-yellow-500',
  DELETE: 'bg-danger/10 text-danger',
};

export default function LogsTab({ projectId }: Props) {
  const [entries, setEntries] = useState<LogEntry[]>([]);
  const [usage, setUsage] = useState<UsageMetrics>({});
  const [loading, setLoading] = useState(true);
  const [logTotal, setLogTotal] = useState(0);
  const LOG_PAGE_SIZE = 200;

  const load = useCallback(() => {
    setLoading(true);
    Promise.all([
      api<{ entries: LogEntry[]; total: number; limit: number; offset: number }>(
        `/admin/projects/${encodeURIComponent(projectId)}/logs?limit=${LOG_PAGE_SIZE}`
      ).catch(() => ({ entries: [], total: 0, limit: LOG_PAGE_SIZE, offset: 0 })),
      api<UsageMetrics>(`/admin/projects/${encodeURIComponent(projectId)}/usage`).catch(() => ({})),
    ]).then(([logsResp, usageData]) => {
      setEntries(logsResp.entries || []);
      setLogTotal(logsResp.total || 0);
      setUsage(usageData || {});
    }).finally(() => setLoading(false));
  }, [projectId]);

  useEffect(() => { load(); }, [load]);

  const clearLogs = async () => {
    try { await api(`/admin/projects/${encodeURIComponent(projectId)}/logs`, { method: 'DELETE' }); setEntries([]); setLogTotal(0); } catch {}
  };

  const methodColor = (m: string) => METHOD_COLORS[m] || 'bg-bg-input text-text-secondary';
  const statusColor = (s: number) => s < 300 ? 'text-success' : s < 400 ? 'text-yellow-500' : 'text-danger';

  const totalRequests = logTotal;
  const displayedRequests = entries.length;
  const successRate = displayedRequests > 0 ? Math.round((entries.filter((e) => e.status < 400).length / displayedRequests) * 100) : 100;
  const errorRate = 100 - successRate;

  if (loading) {
    return <div className="flex items-center gap-2 py-10 text-text-muted"><Loader2 className="w-4 h-4 animate-spin" /> Loading logs...</div>;
  }

  return (
    <div className="space-y-6">
      <div className="flex items-center justify-between">
        <h2 className="text-lg font-semibold text-text-primary">Logs & Metrics</h2>
        <div className="flex gap-2">
          <button onClick={load} className="flex items-center gap-1.5 px-3 py-1.5 rounded-lg bg-bg-input border border-border text-text-secondary text-xs hover:text-text-primary transition-colors">
            <RefreshCw className="w-3.5 h-3.5" /> Refresh
          </button>
          <button onClick={clearLogs} className="flex items-center gap-1.5 px-3 py-1.5 rounded-lg bg-danger/10 border border-danger/20 text-danger text-xs hover:bg-danger/20 transition-colors">
            <Trash2 className="w-3.5 h-3.5" /> Clear
          </button>
        </div>
      </div>

      {/* Traffic & Connections */}
      <div>
        <h3 className="text-xs font-bold text-accent uppercase tracking-wider mb-3">Traffic & Connections</h3>
        <div className="grid grid-cols-2 md:grid-cols-4 gap-3">
          {[
            { label: 'Total Requests', value: totalRequests.toLocaleString(), icon: Activity },
            { label: 'Realtime Connections', value: (usage.realtime_connections ?? 0).toLocaleString(), icon: Cloud },
            { label: 'Success Rate', value: `${successRate}%`, color: 'text-success', icon: Activity },
            { label: 'Error Rate', value: `${errorRate}%`, color: 'text-danger', icon: Activity },
          ].map((m) => {
            const Icon = m.icon;
            return (
              <div key={m.label} className="bg-bg-card border border-border rounded-lg p-4">
                <div className="flex items-center gap-2 text-text-muted text-xs mb-1">
                  <Icon className="w-3.5 h-3.5 text-accent" /> {m.label}
                </div>
                <p className={`text-xl font-bold ${m.color || 'text-text-primary'}`}>{m.value}</p>
              </div>
            );
          })}
        </div>
      </div>

      {/* Database & Storage */}
      <div>
        <h3 className="text-xs font-bold text-accent uppercase tracking-wider mb-3">Database & Storage</h3>
        <div className="grid grid-cols-2 md:grid-cols-3 lg:grid-cols-5 gap-3">
          {[
            { label: 'DB Reads', value: (usage.db_reads_total ?? 0).toLocaleString(), icon: Database },
            { label: 'DB Writes', value: (usage.db_writes_total ?? 0).toLocaleString(), icon: Database },
            { label: 'Database Size', value: formatBytes(usage.database_bytes || 0), icon: HardDrive },
            { label: 'File Storage', value: formatBytes(usage.file_storage_bytes || 0), icon: HardDrive },
            { label: 'Total Storage', value: formatBytes(usage.total_storage_bytes || 0), color: 'text-accent', icon: HardDrive },
          ].map((m) => {
            const Icon = m.icon;
            return (
              <div key={m.label} className="bg-bg-card border border-border rounded-lg p-4">
                <div className="flex items-center gap-2 text-text-muted text-xs mb-1">
                  <Icon className="w-3.5 h-3.5 text-accent" /> {m.label}
                </div>
                <p className={`text-xl font-bold ${m.color || 'text-text-primary'}`}>{m.value}</p>
              </div>
            );
          })}
        </div>
      </div>

      {/* Logs Table */}
      {entries.length === 0 ? (
        <div className="flex flex-col items-center py-12 text-text-muted gap-2">
          <p className="text-sm">No request logs recorded yet</p>
        </div>
      ) : (
        <div className="space-y-2">
          {logTotal > entries.length && (
            <p className="text-text-muted text-xs text-center">
              Showing latest {entries.length} of {logTotal.toLocaleString()} log entries
            </p>
          )}
          <div className="border border-border rounded-lg overflow-hidden">
          <table className="w-full text-sm">
            <thead className="bg-bg-input text-text-muted text-xs uppercase">
              <tr>
                <th className="text-left px-4 py-3 font-medium">Timestamp</th>
                <th className="text-left px-4 py-3 font-medium w-20">Method</th>
                <th className="text-left px-4 py-3 font-medium">Path</th>
                <th className="text-left px-4 py-3 font-medium w-20">Status</th>
                <th className="text-right px-4 py-3 font-medium w-24">Duration</th>
              </tr>
            </thead>
            <tbody className="divide-y divide-border font-mono">
              {entries.map((e, i) => (
                <tr key={i} className="hover:bg-bg-input/50 transition-colors">
                  <td className="px-4 py-2 text-text-secondary text-xs whitespace-nowrap">{formatDate(e.timestamp)}</td>
                  <td className="px-4 py-2"><span className={`inline-flex px-2 py-0.5 rounded text-xs font-bold ${methodColor(e.method)}`}>{e.method}</span></td>
                  <td className="px-4 py-2 text-text-primary text-xs truncate max-w-md" title={e.path}>{e.path}</td>
                  <td className={`px-4 py-2 text-xs font-bold ${statusColor(e.status)}`}>{e.status}</td>
                  <td className="px-4 py-2 text-text-secondary text-xs text-right">{e.duration}</td>
                </tr>
              ))}
            </tbody>
          </table>
          </div>
        </div>
      )}
    </div>
  );
}
