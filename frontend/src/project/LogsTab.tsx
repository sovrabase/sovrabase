import { useState, useEffect, useCallback } from 'react';
import { Activity, Cloud, CheckCircle, XCircle, Database, HardDrive, FileText, RefreshCw, Trash2, Loader2 } from 'lucide-react';
import { api, formatBytes, formatDate } from '../api';

interface LogMetrics {
  total_requests: number;
  realtime_connections: number;
  success_rate: number;
  error_rate: number;
  db_reads: number;
  db_writes: number;
  database_size: number;
  file_storage_size: number;
  total_storage: number;
}

interface LogEntry {
  timestamp: string;
  method: string;
  path: string;
  status: number;
  duration: number;
}

interface LogsData {
  metrics: LogMetrics;
  entries: LogEntry[];
}

interface Props {
  projectId: string;
}

const METHOD_COLORS: Record<string, string> = {
  GET: 'bg-success/10 text-success',
  POST: 'bg-accent/10 text-accent',
  PUT: 'bg-yellow-500/10 text-yellow-500',
  PATCH: 'bg-yellow-500/10 text-yellow-500',
  DELETE: 'bg-danger/10 text-danger',
};

export default function LogsTab({ projectId }: Props) {
  const [data, setData] = useState<LogsData | null>(null);
  const [loading, setLoading] = useState(true);

  const load = useCallback(() => {
    setLoading(true);
    api<LogsData>(`/admin/projects/${encodeURIComponent(projectId)}/logs`)
      .then(setData)
      .catch(() => {})
      .finally(() => setLoading(false));
  }, [projectId]);

  useEffect(() => { load(); }, [load]);

  const clearLogs = async () => {
    try {
      await api(`/admin/projects/${encodeURIComponent(projectId)}/logs`, { method: 'DELETE' });
      setData(null);
    } catch { /* ignore */ }
  };

  const methodColor = (m: string) => METHOD_COLORS[m] || 'bg-bg-input text-text-secondary';
  const statusColor = (s: number) =>
    s >= 200 && s < 300 ? 'text-success' : s >= 400 ? 'text-danger' : 'text-text-secondary';

  if (loading) {
    return (
      <div className="flex items-center gap-2 py-10 text-text-muted">
        <Loader2 className="w-4 h-4 animate-spin" /> Loading logs...
      </div>
    );
  }

  if (!data) {
    return (
      <div className="flex flex-col items-center py-16 text-text-muted gap-3">
        <FileText className="w-10 h-10" />
        <p>No logs data available</p>
        <button onClick={load} className="flex items-center gap-1 px-3 py-1.5 bg-bg-input border border-border rounded text-text-secondary text-xs hover:text-text-primary transition-colors">
          <RefreshCw className="w-3 h-3" /> Retry
        </button>
      </div>
    );
  }

  const m = data.metrics || {} as LogMetrics;

  return (
    <div className="space-y-6">
      {/* Header */}
      <div className="flex items-center justify-between">
        <h2 className="text-lg font-semibold text-text-primary">Logs ({data.entries?.length ?? 0})</h2>
        <div className="flex items-center gap-2">
          <button
            onClick={load}
            className="flex items-center gap-1 px-3 py-1.5 bg-bg-input border border-border rounded text-text-secondary text-xs hover:text-text-primary transition-colors"
          >
            <RefreshCw className="w-3 h-3" /> Refresh
          </button>
          <button
            onClick={clearLogs}
            className="flex items-center gap-1 px-3 py-1.5 bg-bg-input border border-border rounded text-text-secondary text-xs hover:text-danger transition-colors"
          >
            <Trash2 className="w-3 h-3" /> Clear Logs
          </button>
        </div>
      </div>

      {/* Traffic & Connections */}
      <div className="grid grid-cols-2 md:grid-cols-4 gap-4">
        <div className="bg-bg-card border border-border rounded-lg p-4">
          <div className="flex items-center gap-2 text-text-muted text-xs mb-1">
            <Activity className="w-3.5 h-3.5 text-accent" /> Total Requests
          </div>
          <p className="text-xl font-bold text-text-primary">{m.total_requests?.toLocaleString() ?? '0'}</p>
        </div>
        <div className="bg-bg-card border border-border rounded-lg p-4">
          <div className="flex items-center gap-2 text-text-muted text-xs mb-1">
            <Cloud className="w-3.5 h-3.5 text-accent" /> Realtime Connections
          </div>
          <p className="text-xl font-bold text-text-primary">{m.realtime_connections?.toLocaleString() ?? '0'}</p>
        </div>
        <div className="bg-bg-card border border-border rounded-lg p-4">
          <div className="flex items-center gap-2 text-text-muted text-xs mb-1">
            <CheckCircle className="w-3.5 h-3.5 text-success" /> Success Rate
          </div>
          <p className="text-xl font-bold text-success">{m.success_rate != null ? `${m.success_rate}%` : '—'}</p>
        </div>
        <div className="bg-bg-card border border-border rounded-lg p-4">
          <div className="flex items-center gap-2 text-text-muted text-xs mb-1">
            <XCircle className="w-3.5 h-3.5 text-danger" /> Error Rate
          </div>
          <p className="text-xl font-bold text-danger">{m.error_rate != null ? `${m.error_rate}%` : '—'}</p>
        </div>
      </div>

      {/* Database & Storage */}
      <div className="grid grid-cols-2 md:grid-cols-5 gap-4">
        <div className="bg-bg-card border border-border rounded-lg p-4">
          <div className="flex items-center gap-2 text-text-muted text-xs mb-1">
            <Database className="w-3.5 h-3.5" /> DB Reads
          </div>
          <p className="text-xl font-bold text-text-primary">{m.db_reads?.toLocaleString() ?? '0'}</p>
        </div>
        <div className="bg-bg-card border border-border rounded-lg p-4">
          <div className="flex items-center gap-2 text-text-muted text-xs mb-1">
            <Database className="w-3.5 h-3.5" /> DB Writes
          </div>
          <p className="text-xl font-bold text-text-primary">{m.db_writes?.toLocaleString() ?? '0'}</p>
        </div>
        <div className="bg-bg-card border border-border rounded-lg p-4">
          <div className="flex items-center gap-2 text-text-muted text-xs mb-1">
            <HardDrive className="w-3.5 h-3.5" /> Database Size
          </div>
          <p className="text-xl font-bold text-text-primary">{m.database_size != null ? formatBytes(m.database_size) : '—'}</p>
        </div>
        <div className="bg-bg-card border border-border rounded-lg p-4">
          <div className="flex items-center gap-2 text-text-muted text-xs mb-1">
            <HardDrive className="w-3.5 h-3.5" /> File Storage
          </div>
          <p className="text-xl font-bold text-text-primary">{m.file_storage_size != null ? formatBytes(m.file_storage_size) : '—'}</p>
        </div>
        <div className="bg-bg-card border border-accent/30 rounded-lg p-4">
          <div className="flex items-center gap-2 text-text-muted text-xs mb-1">
            <HardDrive className="w-3.5 h-3.5 text-accent" /> Total Storage
          </div>
          <p className="text-xl font-bold text-accent">{m.total_storage != null ? formatBytes(m.total_storage) : '—'}</p>
        </div>
      </div>

      {/* Logs Table */}
      {data.entries && data.entries.length > 0 ? (
        <div className="border border-border rounded-lg overflow-hidden">
          <table className="w-full text-sm">
            <thead className="bg-bg-input text-text-muted text-xs uppercase">
              <tr>
                <th className="text-left px-4 py-3 font-medium w-44">Timestamp</th>
                <th className="text-left px-4 py-3 font-medium w-20">Method</th>
                <th className="text-left px-4 py-3 font-medium">Path</th>
                <th className="text-left px-4 py-3 font-medium w-20">Status</th>
                <th className="text-right px-4 py-3 font-medium w-24">Duration</th>
              </tr>
            </thead>
            <tbody className="divide-y divide-border font-mono">
              {data.entries.map((e, i) => (
                <tr key={i} className="hover:bg-bg-input/50 transition-colors">
                  <td className="px-4 py-2 text-text-secondary text-xs whitespace-nowrap">
                    {formatDate(e.timestamp)}
                  </td>
                  <td className="px-4 py-2">
                    <span className={`inline-flex px-2 py-0.5 rounded text-xs font-bold ${methodColor(e.method)}`}>
                      {e.method}
                    </span>
                  </td>
                  <td className="px-4 py-2 text-text-primary text-xs break-all">{e.path}</td>
                  <td className="px-4 py-2">
                    <span className={`text-xs font-bold ${statusColor(e.status)}`}>{e.status}</span>
                  </td>
                  <td className="px-4 py-2 text-text-secondary text-xs text-right">
                    {e.duration != null ? `${e.duration} ms` : '—'}
                  </td>
                </tr>
              ))}
            </tbody>
          </table>
        </div>
      ) : (
        <div className="flex flex-col items-center py-16 text-text-muted gap-3">
          <FileText className="w-10 h-10" />
          <p>No log entries</p>
        </div>
      )}
    </div>
  );
}
